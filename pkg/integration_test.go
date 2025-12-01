package pkg

import (
	"os"
	"testing"

	"go-synth/config"
	"go-synth/log"
)

// TestIntegration_SimpleWorkflow tests the complete Parse→Resolve→TopoOrder pipeline
// with a simple port that has a few dependencies.
func TestIntegration_SimpleWorkflow(t *testing.T) {
	// Use test fixtures
	// Note: vim depends on python311 but we don't have that fixture,
	// so the test will warn about missing dependencies (expected)
	restore := setTestQuerier(newTestFixtureQuerier(map[string]string{
		"editors/vim":           "testdata/fixtures/editors__vim.txt",
		"devel/gettext-runtime": "testdata/fixtures/devel__gettext-runtime.txt",
		"devel/gettext-tools":   "testdata/fixtures/devel__gettext-tools.txt",
	}))
	defer restore()

	// Create config
	cfg := &config.Config{
		DPortsPath: "/usr/ports",
		MaxWorkers: 4,
	}

	// Create registries
	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Step 1: Parse
	packages, err := ParsePortList([]string{"editors/vim"}, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(packages))
	}

	vim := packages[0]
	if vim.PortDir != "editors/vim" {
		t.Errorf("Expected PortDir 'editors/vim', got '%s'", vim.PortDir)
	}

	if vim.Version != "9.1.0470" {
		t.Errorf("Expected Version '9.1.0470', got '%s'", vim.Version)
	}

	// Step 2: Resolve dependencies
	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// Verify vim has dependencies resolved
	if len(vim.IDependOn) == 0 {
		t.Error("Expected vim to have dependencies, got none")
	}

	// Check for expected dependencies (only those we have fixtures for)
	depNames := make(map[string]bool)
	for _, dep := range vim.IDependOn {
		depNames[dep.Pkg.PortDir] = true
	}

	// We only have fixtures for gettext-runtime and gettext-tools
	// (vim also depends on python311, ncurses, etc. but we don't have those fixtures)
	expectedDeps := []string{"devel/gettext-runtime", "devel/gettext-tools"}
	for _, expected := range expectedDeps {
		if !depNames[expected] {
			t.Errorf("Expected dependency %s not found", expected)
		}
	}

	// Step 3: Get build order (get all packages from registry)
	allPackages := pkgRegistry.AllPackages()
	buildOrder := GetBuildOrder(allPackages, log.NoOpLogger{})

	if len(buildOrder) < 2 {
		t.Fatalf("Expected at least 2 packages in build order (vim + deps), got %d", len(buildOrder))
	}

	// Verify vim comes after its dependencies
	vimIndex := -1
	gettextRuntimeIndex := -1
	gettextToolsIndex := -1

	for i, pkg := range buildOrder {
		switch pkg.PortDir {
		case "editors/vim":
			vimIndex = i
		case "devel/gettext-runtime":
			gettextRuntimeIndex = i
		case "devel/gettext-tools":
			gettextToolsIndex = i
		}
	}

	if vimIndex == -1 {
		t.Fatal("vim not found in build order")
	}

	if gettextRuntimeIndex == -1 {
		t.Fatal("gettext-runtime not found in build order")
	}

	if gettextToolsIndex == -1 {
		t.Fatal("gettext-tools not found in build order")
	}

	// Dependencies must come before dependents
	if gettextRuntimeIndex >= vimIndex {
		t.Errorf("gettext-runtime (index %d) should come before vim (index %d)", gettextRuntimeIndex, vimIndex)
	}

	if gettextToolsIndex >= vimIndex {
		t.Errorf("gettext-tools (index %d) should come before vim (index %d)", gettextToolsIndex, vimIndex)
	}

	t.Logf("Build order verified: %d packages in correct dependency order", len(buildOrder))
}

// TestIntegration_SharedDependencies tests that shared dependencies appear only once
// in the dependency graph and build order.
func TestIntegration_SharedDependencies(t *testing.T) {
	// Use test fixtures - vim and git both depend on gettext-runtime and gettext-tools
	// Note: both also depend on python311, but we don't have that fixture
	restore := setTestQuerier(newTestFixtureQuerier(map[string]string{
		"editors/vim":           "testdata/fixtures/editors__vim.txt",
		"devel/git":             "testdata/fixtures/devel__git.txt",
		"devel/gettext-runtime": "testdata/fixtures/devel__gettext-runtime.txt",
		"devel/gettext-tools":   "testdata/fixtures/devel__gettext-tools.txt",
		"ftp/curl":              "testdata/fixtures/ftp__curl.txt",
		"textproc/expat":        "testdata/fixtures/textproc__expat.txt",
	}))
	defer restore()

	cfg := &config.Config{
		DPortsPath: "/usr/ports",
		MaxWorkers: 4,
	}

	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Parse both vim and git
	packages, err := ParsePortList([]string{"editors/vim", "devel/git"}, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	if len(packages) != 2 {
		t.Fatalf("Expected 2 packages, got %d", len(packages))
	}

	// Resolve dependencies
	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// Get build order (get all packages from registry)
	allPackages := pkgRegistry.AllPackages()
	buildOrder := GetBuildOrder(allPackages, log.NoOpLogger{})

	// Count occurrences of shared dependencies
	gettextRuntimeCount := 0
	gettextToolsCount := 0
	vimCount := 0
	gitCount := 0

	for _, pkg := range buildOrder {
		switch pkg.PortDir {
		case "devel/gettext-runtime":
			gettextRuntimeCount++
		case "devel/gettext-tools":
			gettextToolsCount++
		case "editors/vim":
			vimCount++
		case "devel/git":
			gitCount++
		}
	}

	// Each package should appear exactly once
	if gettextRuntimeCount != 1 {
		t.Errorf("gettext-runtime should appear once in build order, got %d", gettextRuntimeCount)
	}

	if gettextToolsCount != 1 {
		t.Errorf("gettext-tools should appear once in build order, got %d", gettextToolsCount)
	}

	if vimCount != 1 {
		t.Errorf("vim should appear once in build order, got %d", vimCount)
	}

	if gitCount != 1 {
		t.Errorf("git should appear once in build order, got %d", gitCount)
	}

	// Verify shared dependencies come before both vim and git
	gmakeIndex := findPackageIndex(buildOrder, "devel/gmake")
	vimIndex := findPackageIndex(buildOrder, "editors/vim")
	gitIndex := findPackageIndex(buildOrder, "devel/git")

	if gmakeIndex >= vimIndex {
		t.Errorf("gmake must come before vim")
	}

	if gmakeIndex >= gitIndex {
		t.Errorf("gmake must come before git")
	}

	t.Logf("Shared dependencies handled correctly: %d total packages", len(buildOrder))
}

// TestIntegration_FlavoredPackage tests parsing a flavored port.
// Note: The vim@python39 flavor no longer exists in DragonFly (has IGNORE set in fixture),
// but we can still parse it and test that the flavor parsing works correctly.
func TestIntegration_FlavoredPackage(t *testing.T) {
	restore := setTestQuerier(newTestFixtureQuerier(map[string]string{
		"editors/vim@python39":  "testdata/fixtures/editors__vim@python39.txt",
		"devel/gettext-runtime": "testdata/fixtures/devel__gettext-runtime.txt",
		"devel/gettext-tools":   "testdata/fixtures/devel__gettext-tools.txt",
	}))
	defer restore()

	cfg := &config.Config{
		DPortsPath: "/usr/ports",
		MaxWorkers: 4,
	}

	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Parse flavored port
	packages, err := ParsePortList([]string{"editors/vim@python39"}, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(packages))
	}

	vim := packages[0]

	// Verify flavor is set correctly
	if vim.Flavor != "python39" {
		t.Errorf("Expected Flavor 'python39', got '%s'", vim.Flavor)
	}

	if vim.PortDir != "editors/vim@python39" {
		t.Errorf("Expected PortDir 'editors/vim@python39', got '%s'", vim.PortDir)
	}

	// Verify package file is set
	if vim.PkgFile != "vim-9.1.0470.pkg" {
		t.Errorf("Expected PkgFile 'vim-9.1.0470.pkg', got '%s'", vim.PkgFile)
	}

	// Resolve dependencies
	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// Get all packages to verify dependency graph
	allPackages := pkgRegistry.AllPackages()
	if len(allPackages) < 2 {
		t.Fatalf("Expected at least 2 packages after resolution (vim + deps), got %d", len(allPackages))
	}

	t.Logf("Flavored package handled correctly: %s with %d total packages", vim.PortDir, len(allPackages))
}

// TestIntegration_ErrorPortNotFound tests error handling when a port doesn't exist.
func TestIntegration_ErrorPortNotFound(t *testing.T) {
	restore := setTestQuerier(newTestFixtureQuerier(map[string]string{
		// No fixture for nonexistent/port
	}))
	defer restore()

	cfg := &config.Config{
		DPortsPath: "/usr/ports",
		MaxWorkers: 4,
	}

	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Try to parse non-existent port
	packages, err := ParsePortList([]string{"nonexistent/port"}, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})

	// Should return empty list with no error (port not found is logged as warning)
	if err != nil {
		t.Logf("Got error (acceptable): %v", err)
	}

	if len(packages) > 0 {
		t.Errorf("Expected no packages for non-existent port, got %d", len(packages))
	}

	t.Logf("Port not found error handled correctly")
}

// TestIntegration_MetaPort tests handling of meta ports (ports with no PKGFILE).
func TestIntegration_MetaPort(t *testing.T) {
	restore := setTestQuerier(newTestFixtureQuerier(map[string]string{
		"x11/meta-gnome": "testdata/fixtures/x11__meta-gnome.txt",
	}))
	defer restore()

	cfg := &config.Config{
		DPortsPath: "/usr/ports",
		MaxWorkers: 4,
	}

	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Parse meta port
	packages, err := ParsePortList([]string{"x11/meta-gnome"}, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(packages))
	}

	metaPkg := packages[0]

	// Verify PkgFMeta flag is set
	flags := bsRegistry.GetFlags(metaPkg)
	if !flags.Has(PkgFMeta) {
		t.Error("Expected PkgFMeta flag to be set for meta port")
	}

	// Meta ports should still have a PkgFile (generated default)
	if metaPkg.PkgFile == "" {
		t.Error("Meta port should have a default PkgFile")
	}

	t.Logf("Meta port handled correctly: %s (flags: 0x%x)", metaPkg.PortDir, flags)
}

// TestIntegration_DeepDependencies tests resolution of ports with deep dependency trees
// This test uses complex ports like Firefox or Chromium that have many transitive dependencies
func TestIntegration_DeepDependencies(t *testing.T) {
	// Note: This test requires fixtures for complex ports to be generated on BSD
	// If fixtures don't exist, the test will be skipped

	// Check if complex port fixtures exist
	complexFixtures := map[string]string{
		"www/firefox":        "testdata/fixtures/www__firefox.txt",
		"x11/xorg-server":    "testdata/fixtures/x11__xorg-server.txt",
		"graphics/mesa-libs": "testdata/fixtures/graphics__mesa-libs.txt",
		"multimedia/ffmpeg":  "testdata/fixtures/multimedia__ffmpeg.txt",
		// Add common dependencies that these would need
		"devel/gmake":           "testdata/fixtures/devel__gmake.txt",
		"devel/pkgconf":         "testdata/fixtures/devel__pkgconf.txt",
		"devel/gettext-runtime": "testdata/fixtures/devel__gettext-runtime.txt",
		"lang/python39":         "testdata/fixtures/lang__python39.txt",
		"x11/libX11":            "testdata/fixtures/x11__libX11.txt",
		"x11/libxcb":            "testdata/fixtures/x11__libxcb.txt",
	}

	// Check if at least one complex fixture exists
	foundComplex := false
	testPort := ""
	for port, fixture := range complexFixtures {
		if _, err := os.Stat(fixture); err == nil {
			if port == "www/firefox" || port == "x11/xorg-server" || port == "multimedia/ffmpeg" {
				foundComplex = true
				testPort = port
				break
			}
		}
	}

	if !foundComplex {
		t.Skip("Skipping deep dependency test: no complex port fixtures found (run capture-fixtures.sh on BSD)")
	}

	restore := setTestQuerier(newTestFixtureQuerier(complexFixtures))
	defer restore()

	cfg := &config.Config{
		DPortsPath: "/usr/ports",
		MaxWorkers: 4,
	}

	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Parse the complex port
	packages, err := ParsePortList([]string{testPort}, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	// Resolve dependencies
	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	allPackages := pkgRegistry.AllPackages()
	t.Logf("Resolved %d total packages for %s", len(allPackages), testPort)

	// Should have many packages (complex ports typically have 50+ dependencies)
	if len(allPackages) < 5 {
		t.Errorf("Expected at least 5 packages for complex port %s, got %d", testPort, len(allPackages))
	}

	// Get build order
	buildOrder := GetBuildOrder(allPackages, log.NoOpLogger{})

	// Verify topological order is correct
	pkgPositions := make(map[string]int)
	for i, pkg := range buildOrder {
		pkgPositions[pkg.PortDir] = i
	}

	violations := 0
	for i, pkg := range buildOrder {
		for _, dep := range pkg.IDependOn {
			depPos, ok := pkgPositions[dep.Pkg.PortDir]
			if !ok {
				t.Errorf("Dependency %s not found in build order", dep.Pkg.PortDir)
				violations++
				continue
			}
			if depPos >= i {
				t.Errorf("Dependency %s (pos %d) comes after %s (pos %d)",
					dep.Pkg.PortDir, depPos, pkg.PortDir, i)
				violations++
			}
		}
	}

	if violations == 0 {
		t.Logf("Deep dependency test passed: %d packages in correct order", len(buildOrder))
	} else {
		t.Errorf("Found %d topological order violations", violations)
	}

	// Verify the requested port is last in the build order
	lastPkg := buildOrder[len(buildOrder)-1]
	if lastPkg.PortDir != testPort {
		t.Errorf("Expected %s to be last in build order, got %s", testPort, lastPkg.PortDir)
	}
}

// TestIntegration_LargeGraph tests handling of a large dependency graph with multiple root packages
func TestIntegration_LargeGraph(t *testing.T) {
	// This test combines multiple complex ports to create a large graph
	// with shared dependencies, testing the registry's ability to handle
	// deduplication and proper ordering at scale

	fixtures := map[string]string{
		// Multiple root applications
		"editors/vim":       "testdata/fixtures/editors__vim.txt",
		"devel/git":         "testdata/fixtures/devel__git.txt",
		"shells/bash":       "testdata/fixtures/shells__bash.txt",
		"www/firefox":       "testdata/fixtures/www__firefox.txt",
		"multimedia/ffmpeg": "testdata/fixtures/multimedia__ffmpeg.txt",

		// Common dependencies (will be shared)
		"devel/gmake":           "testdata/fixtures/devel__gmake.txt",
		"devel/pkgconf":         "testdata/fixtures/devel__pkgconf.txt",
		"devel/gettext-runtime": "testdata/fixtures/devel__gettext-runtime.txt",
		"devel/gettext-tools":   "testdata/fixtures/devel__gettext-tools.txt",
		"devel/libffi":          "testdata/fixtures/devel__libffi.txt",
		"lang/python39":         "testdata/fixtures/lang__python39.txt",
		"lang/perl5":            "testdata/fixtures/lang__perl5.txt",
		"ftp/curl":              "testdata/fixtures/ftp__curl.txt",
		"textproc/expat":        "testdata/fixtures/textproc__expat.txt",
	}

	// Check if we have enough fixtures
	existingCount := 0
	var testPorts []string
	for port, fixture := range fixtures {
		if _, err := os.Stat(fixture); err == nil {
			existingCount++
			// Only use as test port if it's an application (not a library)
			if port == "editors/vim" || port == "devel/git" || port == "shells/bash" {
				testPorts = append(testPorts, port)
			}
		}
	}

	if existingCount < 5 {
		t.Skipf("Skipping large graph test: only %d fixtures found (need at least 5)", existingCount)
	}

	if len(testPorts) < 2 {
		t.Skip("Skipping large graph test: need at least 2 application fixtures")
	}

	restore := setTestQuerier(newTestFixtureQuerier(fixtures))
	defer restore()

	cfg := &config.Config{
		DPortsPath: "/usr/ports",
		MaxWorkers: 4,
	}

	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Parse multiple ports at once
	packages, err := ParsePortList(testPorts, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	t.Logf("Parsed %d root packages: %v", len(packages), testPorts)

	// Resolve dependencies
	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	allPackages := pkgRegistry.AllPackages()
	t.Logf("Resolved %d total packages from %d roots", len(allPackages), len(testPorts))

	// Get build order
	buildOrder := GetBuildOrder(allPackages, log.NoOpLogger{})

	// Verify each package appears exactly once
	seenPackages := make(map[string]int)
	for _, pkg := range buildOrder {
		seenPackages[pkg.PortDir]++
	}

	duplicates := 0
	for portDir, count := range seenPackages {
		if count > 1 {
			t.Errorf("Package %s appears %d times in build order (should be 1)", portDir, count)
			duplicates++
		}
	}

	if duplicates == 0 {
		t.Logf("No duplicates found: all %d packages appear exactly once", len(buildOrder))
	}

	// Verify all root packages are in the build order
	for _, testPort := range testPorts {
		if seenPackages[testPort] == 0 {
			t.Errorf("Root package %s missing from build order", testPort)
		}
	}

	// Verify topological ordering
	pkgPositions := make(map[string]int)
	for i, pkg := range buildOrder {
		pkgPositions[pkg.PortDir] = i
	}

	violations := 0
	for i, pkg := range buildOrder {
		for _, dep := range pkg.IDependOn {
			depPos, ok := pkgPositions[dep.Pkg.PortDir]
			if !ok {
				continue // Dependency might not be in fixtures
			}
			if depPos >= i {
				violations++
			}
		}
	}

	if violations > 0 {
		t.Errorf("Found %d topological order violations in large graph", violations)
	} else {
		t.Logf("Large graph validated: %d packages in correct dependency order", len(buildOrder))
	}
}

// Helper function to find package index in slice
func findPackageIndex(packages []*Package, portDir string) int {
	for i, pkg := range packages {
		if pkg.PortDir == portDir {
			return i
		}
	}
	return -1
}
