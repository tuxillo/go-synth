// Package pkg provides package metadata parsing and dependency resolution
// for BSD ports. It supports parsing port specifications, resolving complete
// dependency graphs with 6 dependency types (FETCH, EXTRACT, PATCH, BUILD,
// LIB, RUN), and computing topological build order using Kahn's algorithm.
//
// The package separates concerns between:
//   - Package metadata (Package struct) - immutable port information
//   - Build-time state (BuildStateRegistry) - mutable build flags and status
//
// # Basic Usage
//
// Parse port specifications, resolve dependencies, and compute build order:
//
//	cfg, _ := config.LoadConfig("", "default")
//	pkgRegistry := pkg.NewPackageRegistry()
//	stateRegistry := pkg.NewBuildStateRegistry()
//
//	// Parse port specifications
//	packages, err := pkg.ParsePortList([]string{"editors/vim"}, cfg, stateRegistry, pkgRegistry)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Resolve all dependencies recursively
//	err = pkg.ResolveDependencies(packages, cfg, stateRegistry, pkgRegistry)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get topological build order
//	buildOrder, err := pkg.GetBuildOrder(packages)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// buildOrder now contains packages in dependency order
//	for _, p := range buildOrder {
//	    fmt.Println(p.PortDir)
//	}
//
// # Error Handling
//
// The package defines structured error types for common failures:
//   - ErrCycleDetected: circular dependencies found during topological sort
//   - ErrPortNotFound: port doesn't exist in the ports tree
//   - ErrInvalidSpec: malformed port specification
//   - ErrNoValidPorts: no valid ports found in specification list
//
// Use errors.Is() and errors.As() to check error types programmatically.
package pkg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"dsynth/builddb"
	"dsynth/config"
)

// PackageFlags represents boolean attributes of a package using bitfield flags.
// Multiple flags can be combined using bitwise OR, allowing efficient storage
// of multiple boolean properties in a single integer.
//
// Use the Has(), Set(), and Clear() methods to manipulate flags in a type-safe way:
//
//	flags := PkgFManualSel | PkgFMeta
//	if flags.Has(PkgFMeta) {
//	    // handle meta port
//	}
//	flags = flags.Set(PkgFSuccess)
//	flags = flags.Clear(PkgFRunning)
type PackageFlags int

// Package flags represent build-time attributes and status. These flags are
// stored in BuildStateRegistry, not in the Package struct itself.
const (
	// PkgFManualSel indicates the package was manually selected by the user,
	// not pulled in as a dependency.
	PkgFManualSel PackageFlags = 0x00000001

	// PkgFMeta indicates a meta port that has no build phase (only dependencies).
	PkgFMeta PackageFlags = 0x00000002

	// PkgFDummy indicates a dummy package (placeholder for testing).
	PkgFDummy PackageFlags = 0x00000004

	// PkgFSuccess indicates the package built successfully.
	PkgFSuccess PackageFlags = 0x00000008

	// PkgFFailed indicates the package build failed.
	PkgFFailed PackageFlags = 0x00000010

	// PkgFSkipped indicates the package was skipped (dependency failed).
	PkgFSkipped PackageFlags = 0x00000020

	// PkgFIgnored indicates the package is ignored (IGNORE in Makefile).
	PkgFIgnored PackageFlags = 0x00000040

	// PkgFNoBuildIgnore indicates the package should not be built (ignored).
	PkgFNoBuildIgnore PackageFlags = 0x00000080

	// PkgFNotFound indicates the port was not found in the ports tree.
	PkgFNotFound PackageFlags = 0x00000100

	// PkgFCorrupt indicates the port has a corrupted or invalid Makefile.
	PkgFCorrupt PackageFlags = 0x00000200

	// PkgFPackaged indicates a package file already exists for this port.
	PkgFPackaged PackageFlags = 0x00000400

	// PkgFRunning indicates the package is currently being built.
	PkgFRunning PackageFlags = 0x00000800
)

// Has reports whether the flag f includes the specified flag.
func (f PackageFlags) Has(flag PackageFlags) bool {
	return f&flag != 0
}

// Set returns f with the specified flag set.
func (f PackageFlags) Set(flag PackageFlags) PackageFlags {
	return f | flag
}

// Clear returns f with the specified flag cleared.
func (f PackageFlags) Clear(flag PackageFlags) PackageFlags {
	return f &^ flag
}

// String returns a string representation of the flags.
func (f PackageFlags) String() string {
	if f == 0 {
		return "NONE"
	}

	var parts []string
	if f.Has(PkgFManualSel) {
		parts = append(parts, "MANUAL_SEL")
	}
	if f.Has(PkgFMeta) {
		parts = append(parts, "META")
	}
	if f.Has(PkgFDummy) {
		parts = append(parts, "DUMMY")
	}
	if f.Has(PkgFSuccess) {
		parts = append(parts, "SUCCESS")
	}
	if f.Has(PkgFFailed) {
		parts = append(parts, "FAILED")
	}
	if f.Has(PkgFSkipped) {
		parts = append(parts, "SKIPPED")
	}
	if f.Has(PkgFIgnored) {
		parts = append(parts, "IGNORED")
	}
	if f.Has(PkgFNoBuildIgnore) {
		parts = append(parts, "NO_BUILD_IGNORE")
	}
	if f.Has(PkgFNotFound) {
		parts = append(parts, "NOT_FOUND")
	}
	if f.Has(PkgFCorrupt) {
		parts = append(parts, "CORRUPT")
	}
	if f.Has(PkgFPackaged) {
		parts = append(parts, "PACKAGED")
	}
	if f.Has(PkgFRunning) {
		parts = append(parts, "RUNNING")
	}

	return strings.Join(parts, "|")
}

// DepType represents the type of dependency relationship between packages.
// BSD ports support six distinct dependency types, each controlling when
// a dependency is required during the build process.
//
// Values match the original C dsynth implementation for compatibility.
type DepType int

// Dependency types define when dependencies are required during the build process.
const (
	// DepTypeFetch indicates a dependency required during the fetch phase.
	// Used for tools needed to download distfiles (e.g., git, svn).
	DepTypeFetch DepType = 1

	// DepTypeExtract indicates a dependency required during the extract phase.
	// Used for tools needed to extract archives (e.g., unzip, bzip2).
	DepTypeExtract DepType = 2

	// DepTypePatch indicates a dependency required during the patch phase.
	// Used for tools needed to apply patches.
	DepTypePatch DepType = 3

	// DepTypeBuild indicates a dependency required during the build phase.
	// Used for build tools and compilers (e.g., cmake, autoconf).
	DepTypeBuild DepType = 4

	// DepTypeLib indicates a library dependency required at both build and runtime.
	// Used for shared libraries (e.g., libpng, libxml2).
	DepTypeLib DepType = 5

	// DepTypeRun indicates a runtime dependency not needed during build.
	// Used for programs/libraries only needed when the package runs.
	DepTypeRun DepType = 6
)

// String returns the string representation of the dependency type.
func (d DepType) String() string {
	switch d {
	case DepTypeFetch:
		return "FETCH"
	case DepTypeExtract:
		return "EXTRACT"
	case DepTypePatch:
		return "PATCH"
	case DepTypeBuild:
		return "BUILD"
	case DepTypeLib:
		return "LIB"
	case DepTypeRun:
		return "RUN"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", d)
	}
}

// Valid reports whether the dependency type is valid.
func (d DepType) Valid() bool {
	return d >= DepTypeFetch && d <= DepTypeRun
}

// Package represents immutable metadata about a BSD port. It contains only
// port information extracted from the ports tree Makefile, and does not
// include any build-time state (which is tracked separately in BuildStateRegistry).
//
// The struct is organized into several logical groups:
//
// # Identification
//
// PortDir uniquely identifies the package (e.g., "editors/vim"). Category,
// Name, and Flavor are parsed from PortDir. Version comes from the port's
// Makefile. PkgFile is the generated package filename.
//
// # Dependencies
//
// Six string fields (FetchDeps, ExtractDeps, PatchDeps, BuildDeps, LibDeps,
// RunDeps) contain raw dependency specifications as returned by the port's
// Makefile. These are parsed during dependency resolution.
//
// # Dependency Graph
//
// After resolution, IDependOn contains links to packages this package depends
// on (forward edges), and DependsOnMe contains reverse links from packages
// that depend on this one (backward edges). DepiCount and DepiDepth are
// computed during graph construction and used for topological sorting.
//
// Package instances are safe for concurrent read access after resolution
// is complete. During resolution, access is coordinated via PackageRegistry.
type Package struct {
	// Identification - uniquely identifies this package
	PortDir  string // e.g., "editors/vim" - unique identifier
	Category string // e.g., "editors"
	Name     string // e.g., "vim"
	Flavor   string // e.g., "" or "python" for flavored ports
	Version  string // e.g., "9.0.1234"
	PkgFile  string // e.g., "vim-9.0.1234.pkg" - package filename

	// Dependencies - raw dependency strings from Makefile
	FetchDeps   string // FETCH_DEPENDS
	ExtractDeps string // EXTRACT_DEPENDS
	PatchDeps   string // PATCH_DEPENDS
	BuildDeps   string // BUILD_DEPENDS
	LibDeps     string // LIB_DEPENDS
	RunDeps     string // RUN_DEPENDS

	// Dependency graph - populated during resolution
	IDependOn   []*PkgLink // Forward edges: packages I depend on
	DependsOnMe []*PkgLink // Backward edges: packages that depend on me
	DepiCount   int        // Number of direct dependents (for topological sort)
	DepiDepth   int        // Maximum dependency chain length (for ordering)

	// Status tracking (not build state)
	BuildUUID  string // UUID for current build attempt (generated at build start)
	LastStatus string // Last build status message
}

// PkgLink represents a directed dependency link in the dependency graph.
// It connects two packages with a specific dependency type (FETCH, EXTRACT,
// PATCH, BUILD, LIB, or RUN).
//
// Links are bidirectional: if package A depends on package B with type BUILD,
// then:
//   - A.IDependOn contains a PkgLink{Pkg: B, DepType: BUILD}
//   - B.DependsOnMe contains a PkgLink{Pkg: A, DepType: BUILD}
//
// This bidirectional structure allows efficient traversal of the dependency
// graph in both directions during topological sorting and build planning.
type PkgLink struct {
	Pkg     *Package // The package at the other end of this dependency link
	DepType DepType  // The type of dependency relationship
}

// GetPortDir implements builddb.Package interface
func (p *Package) GetPortDir() string {
	return p.PortDir
}

// GetCategory implements builddb.Package interface
func (p *Package) GetCategory() string {
	return p.Category
}

// GetName implements builddb.Package interface
func (p *Package) GetName() string {
	return p.Name
}

// GetVersion implements builddb.Package interface
func (p *Package) GetVersion() string {
	return p.Version
}

// GetPkgFile implements builddb.Package interface
func (p *Package) GetPkgFile() string {
	return p.PkgFile
}

// PackageRegistry maintains a global registry of all Package instances,
// ensuring each PortDir has exactly one Package object. This deduplication
// is essential for building an accurate dependency graph where all references
// to the same port point to the same Package instance.
//
// The registry is thread-safe and can be accessed concurrently during
// parallel dependency resolution. Use NewPackageRegistry() to create
// a new instance.
type PackageRegistry struct {
	mu       sync.RWMutex
	packages map[string]*Package
}

// NewPackageRegistry creates a new empty package registry. Each parsing
// session should use a dedicated registry instance for isolation.
//
// Returns a new PackageRegistry ready for use.
func NewPackageRegistry() *PackageRegistry {
	return &PackageRegistry{
		packages: make(map[string]*Package),
	}
}

// AllPackages returns a slice containing all packages in the registry.
// This is useful after dependency resolution to get the complete dependency graph.
//
// Returns a new slice containing all registered packages in no particular order.
func (r *PackageRegistry) AllPackages() []*Package {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Package, 0, len(r.packages))
	for _, pkg := range r.packages {
		result = append(result, pkg)
	}
	return result
}

// Enter adds a package to the registry or returns the existing package
// if one with the same PortDir already exists. This ensures package
// deduplication during dependency resolution.
//
// The method is thread-safe and can be called concurrently.
//
// Parameters:
//   - pkg: the Package to add to the registry
//
// Returns the registered Package, which may be the input pkg or an
// existing Package if one with the same PortDir was already registered.
func (r *PackageRegistry) Enter(pkg *Package) *Package {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.packages[pkg.PortDir]; ok {
		return existing
	}

	r.packages[pkg.PortDir] = pkg
	return pkg
}

// Find looks up a package by its PortDir. Returns nil if the package
// is not in the registry.
//
// The method is thread-safe and can be called concurrently.
//
// Parameters:
//   - portDir: the port directory to look up (e.g., "editors/vim")
//
// Returns the Package if found, nil otherwise.
func (r *PackageRegistry) Find(portDir string) *Package {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.packages[portDir]
}

// ParsePortList parses a list of port specifications and returns Package
// instances for each valid port. Port specifications use the format
// "category/name" or "category/name@flavor" (e.g., "editors/vim" or
// "lang/python@py39").
//
// The function queries each port's Makefile in parallel using cfg.MaxWorkers
// goroutines to extract metadata (version, dependencies, etc.). Invalid or
// not-found ports are logged as warnings but don't cause the function to fail.
//
// # Parameters
//
//   - portList: slice of port specifications like ["editors/vim", "shells/bash"]
//   - cfg: configuration containing DPortsPath and MaxWorkers
//   - registry: build state registry for tracking flags and ignore reasons
//   - pkgRegistry: package registry for deduplication
//
// # Returns
//
//   - slice of Package pointers for successfully parsed ports
//   - ErrNoValidPorts if all ports failed to parse
//   - nil error if at least one port was parsed successfully
//
// # Note
//
// This function does NOT resolve dependencies. Call ResolveDependencies()
// after ParsePortList() to build the complete dependency graph.
//
// # Example
//
//	pkgRegistry := pkg.NewPackageRegistry()
//	stateRegistry := pkg.NewBuildStateRegistry()
//	packages, err := pkg.ParsePortList([]string{"editors/vim"}, cfg, stateRegistry, pkgRegistry)
//	if err != nil {
//	    log.Fatal(err)
//	}
func ParsePortList(portList []string, cfg *config.Config, registry *BuildStateRegistry, pkgRegistry *PackageRegistry) ([]*Package, error) {
	packages := make([]*Package, 0)

	bq := newBulkQueue(cfg, cfg.MaxWorkers)
	defer bq.Close()

	// Queue all ports for parallel processing
	for _, portSpec := range portList {
		category, name, flavor := parsePortSpec(portSpec, cfg)
		if category == "" || name == "" {
			fmt.Printf("Warning: invalid port specification: %s\n", portSpec)
			continue
		}
		bq.Queue(category, name, flavor, "") // Empty flags means manually selected
	}

	// Collect results
	for bq.Pending() > 0 {
		pkg, initialFlags, parseFlags, ignoreReason, err := bq.GetResult()
		if err != nil {
			fmt.Printf("Warning: failed to get package info: %v\n", err)
			continue
		}

		// Store all flags in registry
		allFlags := initialFlags | parseFlags
		if allFlags != 0 {
			registry.AddFlags(pkg, allFlags)
		}
		if ignoreReason != "" {
			registry.SetIgnoreReason(pkg, ignoreReason)
		}

		// Add to slice
		packages = append(packages, pkg)

		// Register package
		pkgRegistry.Enter(pkg)
	}

	if len(packages) == 0 {
		return nil, ErrNoValidPorts
	}

	return packages, nil
}

// parsePortSpec parses a port specification into category/name/flavor
func parsePortSpec(spec string, cfg *config.Config) (category, name, flavor string) {
	// Handle absolute paths
	if strings.HasPrefix(spec, "/") {
		// Strip ports path prefix
		spec = strings.TrimPrefix(spec, cfg.DPortsPath)
		spec = strings.TrimPrefix(spec, "/")
	}

	// Split on @flavor
	parts := strings.SplitN(spec, "@", 2)
	portPath := parts[0]
	if len(parts) == 2 {
		flavor = parts[1]
	}

	// Split category/name
	parts = strings.Split(portPath, "/")
	if len(parts) >= 2 {
		category = parts[0]
		name = parts[1]
	}

	return
}

// getPackageInfo fetches package information from the port Makefile
// getPackageInfo returns a package and its flags/ignoreReason
// Returns: (pkg, flags, ignoreReason, error)
func getPackageInfo(category, name, flavor string, cfg *config.Config) (*Package, PackageFlags, string, error) {
	portDir := category + "/" + name
	if flavor != "" {
		portDir += "@" + flavor
	}

	portPath := filepath.Join(cfg.DPortsPath, category, name)

	// Check if port exists (skip in tests when using fixtures)
	if !skipPortDirCheck {
		if _, err := os.Stat(portPath); os.IsNotExist(err) {
			return nil, PkgFNotFound, "", &PortNotFoundError{
				PortSpec: portDir,
				Path:     portPath,
			}
		}
	}

	pkg := &Package{
		PortDir:  portDir,
		Category: category,
		Name:     name,
		Flavor:   flavor,
	}

	// Query port Makefile for metadata
	flags, ignoreReason, err := queryMakefile(pkg, portPath, cfg)
	if err != nil {
		return pkg, PkgFCorrupt, "", err
	}

	return pkg, flags, ignoreReason, nil
}

// queryMakefile extracts information from port Makefile using the configured querier.
// The querier can be swapped in tests to use fixtures instead of real make commands.
// Returns: flags to set, ignoreReason, error
func queryMakefile(pkg *Package, portPath string, cfg *config.Config) (PackageFlags, string, error) {
	return portsQuerier.QueryMakefile(pkg, portPath, cfg)
}

// ResolveDependencies builds the complete dependency graph for a set of packages
// using a two-pass algorithm that matches the behavior of the original C dsynth
// implementation.
//
// # Algorithm
//
// Pass 1 (Collection): Recursively discovers all dependencies by querying each
// package's six dependency types (FETCH, EXTRACT, PATCH, BUILD, LIB, RUN).
// Dependencies are parsed, queued for parallel fetching via worker pool, and
// added to the packages slice. The process continues iteratively until no new
// dependencies are found.
//
// Pass 2 (Linking): Builds bidirectional links between all packages in the
// dependency graph. For each dependency relationship found in Pass 1, creates
// PkgLink entries in both IDependOn (forward edges) and DependsOnMe (backward
// edges) fields. Also computes DepiCount (number of direct dependents) and
// DepiDepth (maximum dependency chain length) for use in topological sorting.
//
// The function uses parallel workers (cfg.MaxWorkers) to query port Makefiles,
// making dependency resolution efficient even for large graphs (hundreds or
// thousands of packages).
//
// # Parameters
//
//   - packages: initial slice of packages (will be modified with discovered dependencies)
//   - cfg: configuration containing DPortsPath and MaxWorkers
//   - registry: build state registry for tracking flags and ignore reasons
//   - pkgRegistry: package registry for deduplication
//
// # Returns
//
//   - nil on success
//   - error if a critical failure occurs during dependency resolution
//
// After successful completion, pkgRegistry contains the complete transitive
// closure of all dependencies. Each Package has its dependency graph fields
// (IDependOn, DependsOnMe, DepiCount, DepiDepth) fully populated.
//
// IMPORTANT: The complete dependency graph is stored in pkgRegistry, not in
// the input packages slice. To get all resolved packages (including transitive
// dependencies), call pkgRegistry.AllPackages() after resolution completes.
// This is required when passing packages to GetBuildOrder() or TopoOrderStrict().
//
// # Example
//
//	err := pkg.ResolveDependencies(packages, cfg, stateRegistry, pkgRegistry)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Get all resolved packages for build ordering
//	allPackages := pkgRegistry.AllPackages()
//	buildOrder := pkg.GetBuildOrder(allPackages)
func ResolveDependencies(packages []*Package, cfg *config.Config, registry *BuildStateRegistry, pkgRegistry *PackageRegistry) error {
	return resolveDependencies(packages, cfg, registry, pkgRegistry)
}

// MarkPackagesNeedingBuild analyzes which packages need rebuilding based on
// CRC comparisons with the build database. It marks packages that are already
// up-to-date with PkgFSuccess|PkgFPackaged flags so they can be skipped during
// the build phase.
//
// The function handles several special cases:
//   - Packages with errors (PkgFNotFound, PkgFCorrupt) are marked PkgFNoBuildIgnore
//   - Meta packages are marked PkgFSuccess (metaports have no build phase)
//   - Ignored packages are marked PkgFNoBuildIgnore
//   - Packages with unchanged CRCs are marked PkgFSuccess|PkgFPackaged
//   - Packages with CRC errors default to "needs rebuild" for safety
//
// This function is typically called after dependency resolution and before
// starting the build process:
//
//	ResolveDependencies(packages, cfg, stateRegistry, pkgRegistry)
//	needBuild, err := MarkPackagesNeedingBuild(packages, cfg, stateRegistry, buildDB)
//	DoBuild(packages, cfg, logger, buildDB)
//
// # Parameters
//   - packages: List of packages to check (typically from ParsePortList)
//   - cfg: Configuration containing build paths and settings
//   - registry: BuildStateRegistry for tracking package flags
//   - buildDB: Open BuildDB instance for CRC operations
//
// # Returns
//   - number of packages marked as needing rebuild
//   - error if CRC operations fail
//
// # Example
//
//	buildDB, _ := builddb.OpenDB(filepath.Join(cfg.BuildBase, "builds.db"))
//	defer buildDB.Close()
//	needBuild, err := pkg.MarkPackagesNeedingBuild(packages, cfg, stateRegistry, buildDB)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("%d packages need rebuilding\n", needBuild)
func MarkPackagesNeedingBuild(packages []*Package, cfg *config.Config, registry *BuildStateRegistry, buildDB *builddb.DB) (int, error) {

	fmt.Println("\nChecking which packages need rebuilding...")

	needBuild := 0
	checked := 0

	for _, pkg := range packages {
		checked++

		// Skip packages marked with errors
		if registry.HasAnyFlags(pkg, PkgFNotFound|PkgFCorrupt) {
			registry.AddFlags(pkg, PkgFNoBuildIgnore)
			continue
		}

		// Skip meta packages
		if registry.HasFlags(pkg, PkgFMeta) {
			registry.AddFlags(pkg, PkgFSuccess) // Don't build metaports
			continue
		}

		// Skip ignored packages
		if registry.HasFlags(pkg, PkgFIgnored) {
			registry.AddFlags(pkg, PkgFNoBuildIgnore)
			continue
		}

		// Check if build is needed
		portPath := filepath.Join(cfg.DPortsPath, pkg.Category, pkg.Name)
		currentCRC, err := builddb.ComputePortCRC(portPath)
		if err != nil {
			// On error computing CRC, rebuild to be safe
			fmt.Printf("  %s: needs rebuild (CRC computation error: %v)\n", pkg.PortDir, err)
			needBuild++
			continue
		}

		needsBuild, err := buildDB.NeedsBuild(pkg.PortDir, currentCRC)
		if err != nil {
			// On database error, rebuild to be safe
			fmt.Printf("  %s: needs rebuild (DB error: %v)\n", pkg.PortDir, err)
			needBuild++
			continue
		}

		if needsBuild {
			needBuild++
			fmt.Printf("  %s: needs rebuild\n", pkg.PortDir)
		} else {
			// Mark as already successful (no build needed)
			registry.AddFlags(pkg, PkgFSuccess|PkgFPackaged)
			fmt.Printf("  %s: up-to-date\n", pkg.PortDir)
		}

		if checked%100 == 0 {
			fmt.Printf("  Checked %d packages...\r", checked)
		}
	}

	fmt.Printf("  Checked %d packages\n", checked)
	fmt.Printf("  %d packages need building\n", needBuild)
	fmt.Printf("  %d packages are up-to-date\n", checked-needBuild)

	return needBuild, nil
}

// GetInstalledPackages queries the system's package database and returns
// a list of port origins for all currently installed packages.
//
// This is typically used for "dsynth upgrade-system" to rebuild all
// installed packages after ports tree updates.
//
// The function executes "pkg query %o" to retrieve the origin (category/name)
// of each installed package.
//
// # Parameters
//
//   - cfg: configuration (not currently used, included for API consistency)
//
// # Returns
//
//   - slice of port origins like ["editors/vim", "shells/bash"]
//   - error if pkg query command fails
//
// # Example
//
//	origins, err := pkg.GetInstalledPackages(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Found %d installed packages\n", len(origins))
func GetInstalledPackages(cfg *config.Config) ([]string, error) {
	cmd := exec.Command("pkg", "query", "%o")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pkg query failed: %w", err)
	}

	lines := strings.Split(out.String(), "\n")
	pkgs := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			pkgs = append(pkgs, line)
		}
	}

	return pkgs, nil
}

// GetAllPorts scans the entire ports tree and returns a list of all
// available port origins.
//
// # Warning
//
// This function is expensive and can take several minutes on a full ports
// tree (30,000+ ports). Only use for operations that genuinely need all
// ports, such as "dsynth status-all".
//
// The function walks the ports tree directory structure, skipping special
// directories (Mk, Templates, Tools, distfiles, packages) and hidden
// directories.
//
// # Parameters
//
//   - cfg: configuration containing DPortsPath (ports tree location)
//
// # Returns
//
//   - slice of port origins like ["editors/vim", "shells/bash", ...]
//   - error if ports tree directory cannot be read
//
// # Example
//
//	origins, err := pkg.GetAllPorts(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Found %d ports in tree\n", len(origins))
func GetAllPorts(cfg *config.Config) ([]string, error) {
	ports := make([]string, 0, 30000)

	// Walk the ports tree
	categories, err := os.ReadDir(cfg.DPortsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ports directory: %w", err)
	}

	for _, category := range categories {
		if !category.IsDir() {
			continue
		}

		catName := category.Name()

		// Skip special directories
		if strings.HasPrefix(catName, ".") || catName == "Mk" || catName == "Templates" || catName == "Tools" || catName == "distfiles" || catName == "packages" {
			continue
		}

		catPath := filepath.Join(cfg.DPortsPath, catName)
		portDirs, err := os.ReadDir(catPath)
		if err != nil {
			continue
		}

		for _, portDir := range portDirs {
			if !portDir.IsDir() {
				continue
			}

			portName := portDir.Name()
			if strings.HasPrefix(portName, ".") {
				continue
			}

			// Check if Makefile exists
			makefilePath := filepath.Join(catPath, portName, "Makefile")
			if _, err := os.Stat(makefilePath); err == nil {
				ports = append(ports, catName+"/"+portName)
			}
		}
	}

	return ports, nil
}

// Parse is a thin alias for ParsePortList for Phase 1 API compatibility
func Parse(portSpecs []string, cfg *config.Config, registry *BuildStateRegistry, pkgRegistry *PackageRegistry) ([]*Package, error) {
	return ParsePortList(portSpecs, cfg, registry, pkgRegistry)
}

// Resolve wraps ResolveDependencies for Phase 1 API compatibility
func Resolve(packages []*Package, cfg *config.Config, registry *BuildStateRegistry, pkgRegistry *PackageRegistry) error {
	return ResolveDependencies(packages, cfg, registry, pkgRegistry)
}

// TopoOrder wraps GetBuildOrder for Phase 1 API compatibility
func TopoOrder(packages []*Package) []*Package {
	return GetBuildOrder(packages)
}
