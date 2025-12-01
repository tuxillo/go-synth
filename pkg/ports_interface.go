package pkg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go-synth/config"
)

// portsQuerier is the package-level variable used to query port metadata.
// It can be replaced in tests to use fixtures instead of real make commands.
var portsQuerier PortsQuerier = &realPortsQuerier{}

// skipPortDirCheck controls whether getPackageInfo skips the filesystem check.
// This is set to true in tests that use fixtures.
var skipPortDirCheck = false

// PortsQuerier defines the interface for querying port metadata from the ports tree.
// This abstraction allows tests to use fixtures instead of requiring a real ports tree.
type PortsQuerier interface {
	// QueryMakefile extracts metadata from a port's Makefile by querying make variables.
	// It returns the package flags and ignore reason (if any).
	QueryMakefile(pkg *Package, portPath string, cfg *config.Config) (PackageFlags, string, error)
}

// realPortsQuerier implements PortsQuerier by executing actual make commands.
// This is the production implementation used on BSD systems with a ports tree.
type realPortsQuerier struct{}

// QueryMakefile implements PortsQuerier for real ports tree queries using make.
func (r *realPortsQuerier) QueryMakefile(pkg *Package, portPath string, cfg *config.Config) (PackageFlags, string, error) {
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
		return 0, "", fmt.Errorf("make query failed: %w", err)
	}

	return parseQueryOutput(pkg, out.String())
}

// testFixtureQuerier implements PortsQuerier by loading data from test fixtures.
// This allows tests to run without a real ports tree by using pre-captured make output.
type testFixtureQuerier struct {
	fixtures map[string]string // portDir -> fixture file path
}

// newTestFixtureQuerier creates a querier that loads fixture data from files.
// The fixtures map should contain portDir (e.g., "editors/vim") to fixture file paths.
func newTestFixtureQuerier(fixtures map[string]string) *testFixtureQuerier {
	return &testFixtureQuerier{
		fixtures: fixtures,
	}
}

// QueryMakefile implements PortsQuerier for test fixtures.
func (t *testFixtureQuerier) QueryMakefile(pkg *Package, portPath string, cfg *config.Config) (PackageFlags, string, error) {
	// Get fixture path for this port
	fixturePath, ok := t.fixtures[pkg.PortDir]
	if !ok {
		// Port not found in fixtures - simulate port not found error
		return PkgFNotFound, "", &PortNotFoundError{
			PortSpec: pkg.PortDir,
			Path:     portPath,
		}
	}

	// Load fixture file
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		return PkgFCorrupt, "", fmt.Errorf("failed to load fixture %s: %w", fixturePath, err)
	}

	return parseQueryOutput(pkg, string(data))
}

// parseQueryOutput parses the output from make -V queries and populates the Package struct.
// It expects 10 lines of output corresponding to the variables queried in order.
// Returns the package flags and ignore reason (if any).
func parseQueryOutput(pkg *Package, output string) (PackageFlags, string, error) {
	lines := strings.Split(output, "\n")
	if len(lines) < 10 {
		return 0, "", fmt.Errorf("insufficient output from make (got %d lines, expected 10)", len(lines))
	}

	// Parse output
	pkg.Version = strings.TrimSpace(lines[1])
	if pkg.Version == "" {
		pkg.Version = "unknown"
	}

	// CRITICAL: Extract just the basename from PKGFILE
	// The Makefile might return a full path, but we only want the filename
	pkgFileRaw := strings.TrimSpace(lines[2])
	if pkgFileRaw != "" {
		pkg.PkgFile = filepath.Base(pkgFileRaw)
	}

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

	// Compute flags based on metadata
	var flags PackageFlags
	ignoreReason := strings.TrimSpace(lines[9])
	if ignoreReason != "" {
		flags |= PkgFIgnored | PkgFNoBuildIgnore
	}

	// Mark meta ports
	if isMeta {
		flags |= PkgFMeta
	}

	return flags, ignoreReason, nil
}

// setTestQuerier replaces the global querier with a test implementation.
// This is a test helper function that should only be used in tests.
// Returns a function that restores the original querier.
func setTestQuerier(querier PortsQuerier) func() {
	oldQuerier := portsQuerier
	oldSkip := skipPortDirCheck
	portsQuerier = querier
	skipPortDirCheck = true // Skip filesystem checks when using test querier
	return func() {
		portsQuerier = oldQuerier
		skipPortDirCheck = oldSkip
	}
}

// loadFixturesFromDir loads all fixtures from a directory as a map.
// Fixture filenames should use the pattern: category__port.txt
// Returns a map of portDir -> absolute fixture path.
func loadFixturesFromDir(dir string) (map[string]string, error) {
	fixtures := make(map[string]string)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read fixture directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		// Parse filename: category__port.txt or category__port@flavor.txt
		name := strings.TrimSuffix(entry.Name(), ".txt")
		parts := strings.SplitN(name, "__", 2)
		if len(parts) != 2 {
			continue // Skip invalid filenames
		}

		category := parts[0]
		portWithFlavor := parts[1]

		// Construct portDir (e.g., "editors/vim" or "editors/vim@python39")
		portDir := category + "/" + portWithFlavor

		// Store absolute path
		absPath := filepath.Join(dir, entry.Name())
		fixtures[portDir] = absPath
	}

	return fixtures, nil
}

// autoLoadTestFixtures is a helper that loads all fixtures from testdata/fixtures/.
// This is useful for tests that want to use all available fixtures.
func autoLoadTestFixtures() (map[string]string, error) {
	// Determine path to testdata/fixtures relative to pkg package
	fixtureDir := filepath.Join("testdata", "fixtures")
	return loadFixturesFromDir(fixtureDir)
}
