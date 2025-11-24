package pkg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"dsynth/config"
)

// Package flags
const (
	PkgFManualSel     = 0x00000001 // Manually selected
	PkgFMeta          = 0x00000002 // Meta port (no build)
	PkgFDummy         = 0x00000004 // Dummy package
	PkgFSuccess       = 0x00000008 // Build succeeded
	PkgFFailed        = 0x00000010 // Build failed
	PkgFSkipped       = 0x00000020 // Skipped
	PkgFIgnored       = 0x00000040 // Ignored
	PkgFNoBuildIgnore = 0x00000080 // Don't build (ignored)
	PkgFNotFound      = 0x00000100 // Port not found
	PkgFCorrupt       = 0x00000200 // Port corrupted
	PkgFPackaged      = 0x00000400 // Package exists
	PkgFRunning       = 0x00000800 // Currently building
)

// Dependency types
const (
	DepTypeFetch   = 1
	DepTypeExtract = 2
	DepTypePatch   = 3
	DepTypeBuild   = 4
	DepTypeLib     = 5
	DepTypeRun     = 6
)

// Package represents a port/package
type Package struct {
	PortDir  string // e.g., "editors/vim"
	Category string
	Name     string
	Flavor   string
	Version  string
	PkgFile  string // Package filename
	Flags    int

	// Dependencies
	FetchDeps   string
	ExtractDeps string
	PatchDeps   string
	BuildDeps   string
	LibDeps     string
	RunDeps     string

	// Dependency graph
	IDependOn   []*PkgLink // Packages I depend on
	DependsOnMe []*PkgLink // Packages that depend on me
	DepiCount   int        // Number of packages that depend on me
	DepiDepth   int        // Maximum dependency depth

	// Build info
	IgnoreReason string
	LastPhase    string
	LastStatus   string

	// Linked list
	Next *Package
	Prev *Package

	// Synchronization
	mu sync.Mutex
}

// PkgLink represents a dependency link
type PkgLink struct {
	Pkg     *Package
	DepType int
}

// Global package registry
var globalRegistry = &PackageRegistry{
	packages: make(map[string]*Package),
}

// PackageRegistry maintains all packages
type PackageRegistry struct {
	mu       sync.RWMutex
	packages map[string]*Package
}

// Enter adds a package to the registry
func (r *PackageRegistry) Enter(pkg *Package) *Package {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.packages[pkg.PortDir]; ok {
		return existing
	}

	r.packages[pkg.PortDir] = pkg
	return pkg
}

// Find looks up a package by PortDir
func (r *PackageRegistry) Find(portDir string) *Package {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.packages[portDir]
}

// ParsePortList parses a list of port specifications
func ParsePortList(portList []string, cfg *config.Config) (*Package, error) {
	var head, tail *Package

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
		pkg, err := bq.GetResult()
		if err != nil {
			fmt.Printf("Warning: failed to get package info: %v\n", err)
			continue
		}

		// Mark as manually selected
		pkg.Flags |= PkgFManualSel

		// Add to linked list
		if head == nil {
			head = pkg
			tail = pkg
		} else {
			tail.Next = pkg
			pkg.Prev = tail
			tail = pkg
		}

		// Register package
		globalRegistry.Enter(pkg)
	}

	if head == nil {
		return nil, fmt.Errorf("no valid ports found")
	}

	return head, nil
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
func getPackageInfo(category, name, flavor string, cfg *config.Config) (*Package, error) {
	portDir := category + "/" + name
	if flavor != "" {
		portDir += "@" + flavor
	}

	portPath := filepath.Join(cfg.DPortsPath, category, name)

	// Check if port exists
	if _, err := os.Stat(portPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("port not found: %s", portDir)
	}

	pkg := &Package{
		PortDir:  portDir,
		Category: category,
		Name:     name,
		Flavor:   flavor,
	}

	// Query port Makefile for metadata
	if err := queryMakefile(pkg, portPath, cfg); err != nil {
		pkg.Flags |= PkgFCorrupt
		return pkg, err
	}

	return pkg, nil
}

// queryMakefile extracts information from port Makefile
func queryMakefile(pkg *Package, portPath string, cfg *config.Config) error {
	// Build make command to extract variables
	vars := []string{
		"PKGNAME",
		"PKGVERSION",
		"PKGFILE",
		"FETCH_DEPENDS",
		"EXTRACT_DEPENDS",
		"PATCH_DEPENDS",
		"BUILD_DEPENDS",
		"LIB_DEPENDS",
		"RUN_DEPENDS",
		"IGNORE",
	}

	// Use make -V to query variables
	args := []string{
		"-C", portPath,
	}

	// Add flavor if specified
	if pkg.Flavor != "" {
		args = append(args, "FLAVOR="+pkg.Flavor)
	}

	// Query all variables in one go
	for _, v := range vars {
		args = append(args, "-V", v)
	}

	cmd := exec.Command("make", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("make query failed: %w", err)
	}

	lines := strings.Split(out.String(), "\n")
	if len(lines) < len(vars) {
		return fmt.Errorf("insufficient output from make")
	}

	// Parse output
	pkg.Version = strings.TrimSpace(lines[1])
	if pkg.Version == "" {
		pkg.Version = "unknown"
	}

	pkg.PkgFile = strings.TrimSpace(lines[2])

	// Check if it's a meta port BEFORE setting default
	// Meta ports don't produce a package file
	isMeta := pkg.PkgFile == ""

	if pkg.PkgFile == "" {
		pkgname := strings.TrimSpace(lines[0])
		if pkgname == "" {
			pkgname = pkg.Name + "-" + pkg.Version
		}
		pkg.PkgFile = pkgname + ".pkg"
	}

	pkg.FetchDeps = strings.TrimSpace(lines[3])
	pkg.ExtractDeps = strings.TrimSpace(lines[4])
	pkg.PatchDeps = strings.TrimSpace(lines[5])
	pkg.BuildDeps = strings.TrimSpace(lines[6])
	pkg.LibDeps = strings.TrimSpace(lines[7])
	pkg.RunDeps = strings.TrimSpace(lines[8])

	ignoreReason := strings.TrimSpace(lines[9])
	if ignoreReason != "" {
		pkg.Flags |= PkgFIgnored | PkgFNoBuildIgnore
		pkg.IgnoreReason = ignoreReason
	}

	// Mark meta ports
	if isMeta {
		pkg.Flags |= PkgFMeta
	}

	return nil
}

// ResolveDependencies resolves all dependencies
func ResolveDependencies(head *Package, cfg *config.Config) error {
	return resolveDependencies(head, cfg)
}

// MarkPackagesNeedingBuild analyzes which packages need rebuilding
func MarkPackagesNeedingBuild(head *Package, cfg *config.Config) (int, error) {
	// Initialize CRC database
	crcDB, err := InitCRCDatabase(cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to initialize CRC database: %w", err)
	}

	fmt.Println("\nChecking which packages need rebuilding...")

	needBuild := 0
	total := 0

	for pkg := head; pkg != nil; pkg = pkg.Next {
		total++

		// Skip packages marked with errors
		if pkg.Flags&(PkgFNotFound|PkgFCorrupt) != 0 {
			pkg.Flags |= PkgFNoBuildIgnore
			continue
		}

		// Skip meta packages
		if pkg.Flags&PkgFMeta != 0 {
			pkg.Flags |= PkgFSuccess // Don't build metaports
			continue
		}

		// Skip ignored packages
		if pkg.Flags&PkgFIgnored != 0 {
			pkg.Flags |= PkgFNoBuildIgnore
			continue
		}

		// Check if build is needed
		if crcDB.CheckNeedsBuild(pkg, cfg) {
			needBuild++
		} else {
			// Mark as already successful (no build needed)
			pkg.Flags |= PkgFSuccess | PkgFPackaged
			fmt.Printf("  %s: up-to-date\n", pkg.PortDir)
		}

		if total%100 == 0 {
			fmt.Printf("  Checked %d packages...\r", total)
		}
	}

	fmt.Printf("  Checked %d packages\n", total)
	fmt.Printf("  %d packages need building\n", needBuild)
	fmt.Printf("  %d packages are up-to-date\n", total-needBuild)

	return needBuild, nil
}

// SaveCRCDatabase saves the CRC database after builds
func SaveCRCDatabase() error {
	if globalCRCDB == nil {
		return nil
	}
	return globalCRCDB.Save()
}

// UpdateCRCAfterBuild updates CRC database for a successfully built package
func UpdateCRCAfterBuild(pkg *Package, cfg *config.Config) {
	if globalCRCDB != nil {
		globalCRCDB.UpdateAfterBuild(pkg, cfg)
	}
}

// GetInstalledPackages returns a list of installed packages
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

// GetAllPorts returns all ports in the ports tree
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
