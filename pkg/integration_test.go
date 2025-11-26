package pkg

import (
	"testing"

	"dsynth/config"
)

// TestIntegration_SimpleWorkflow tests the complete Parse→Resolve→TopoOrder pipeline
// with a simple port that has a few dependencies.
func TestIntegration_SimpleWorkflow(t *testing.T) {
	// Use test fixtures
	restore := setTestQuerier(newTestFixtureQuerier(map[string]string{
		"editors/vim":           "testdata/fixtures/editors__vim.txt",
		"devel/gmake":           "testdata/fixtures/devel__gmake.txt",
		"lang/python39":         "testdata/fixtures/lang__python39.txt",
		"devel/gettext-runtime": "testdata/fixtures/devel__gettext-runtime.txt",
		"devel/gettext-tools":   "testdata/fixtures/devel__gettext-tools.txt",
		"devel/libffi":          "testdata/fixtures/devel__libffi.txt",
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
	packages, err := ParsePortList([]string{"editors/vim"}, cfg, bsRegistry, pkgRegistry)
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

	if vim.Version != "9.0.1234" {
		t.Errorf("Expected Version '9.0.1234', got '%s'", vim.Version)
	}

	// Step 2: Resolve dependencies
	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry)
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// Verify vim has dependencies resolved
	if len(vim.IDependOn) == 0 {
		t.Error("Expected vim to have dependencies, got none")
	}

	// Check for expected dependencies
	depNames := make(map[string]bool)
	for _, dep := range vim.IDependOn {
		depNames[dep.Pkg.PortDir] = true
	}

	expectedDeps := []string{"devel/gmake", "lang/python39", "devel/gettext-runtime", "devel/gettext-tools"}
	for _, expected := range expectedDeps {
		if !depNames[expected] {
			t.Errorf("Expected dependency %s not found", expected)
		}
	}

	// Step 3: Get build order (get all packages from registry)
	allPackages := pkgRegistry.AllPackages()
	buildOrder := GetBuildOrder(allPackages)

	if len(buildOrder) < 2 {
		t.Fatalf("Expected at least 2 packages in build order (vim + deps), got %d", len(buildOrder))
	}

	// Verify vim comes after its dependencies
	vimIndex := -1
	gmakeIndex := -1
	pythonIndex := -1

	for i, pkg := range buildOrder {
		switch pkg.PortDir {
		case "editors/vim":
			vimIndex = i
		case "devel/gmake":
			gmakeIndex = i
		case "lang/python39":
			pythonIndex = i
		}
	}

	if vimIndex == -1 {
		t.Fatal("vim not found in build order")
	}

	if gmakeIndex == -1 {
		t.Fatal("gmake not found in build order")
	}

	if pythonIndex == -1 {
		t.Fatal("python39 not found in build order")
	}

	// Dependencies must come before dependents
	if gmakeIndex >= vimIndex {
		t.Errorf("gmake (index %d) should come before vim (index %d)", gmakeIndex, vimIndex)
	}

	if pythonIndex >= vimIndex {
		t.Errorf("python39 (index %d) should come before vim (index %d)", pythonIndex, vimIndex)
	}

	t.Logf("Build order verified: %d packages in correct dependency order", len(buildOrder))
}

// TestIntegration_SharedDependencies tests that shared dependencies appear only once
// in the dependency graph and build order.
func TestIntegration_SharedDependencies(t *testing.T) {
	// Use test fixtures - vim and git both depend on gmake and python39
	restore := setTestQuerier(newTestFixtureQuerier(map[string]string{
		"editors/vim":           "testdata/fixtures/editors__vim.txt",
		"devel/git":             "testdata/fixtures/devel__git.txt",
		"devel/gmake":           "testdata/fixtures/devel__gmake.txt",
		"lang/python39":         "testdata/fixtures/lang__python39.txt",
		"devel/gettext-runtime": "testdata/fixtures/devel__gettext-runtime.txt",
		"devel/gettext-tools":   "testdata/fixtures/devel__gettext-tools.txt",
		"devel/libffi":          "testdata/fixtures/devel__libffi.txt",
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
	packages, err := ParsePortList([]string{"editors/vim", "devel/git"}, cfg, bsRegistry, pkgRegistry)
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	if len(packages) != 2 {
		t.Fatalf("Expected 2 packages, got %d", len(packages))
	}

	// Resolve dependencies
	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry)
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// Get build order (get all packages from registry)
	allPackages := pkgRegistry.AllPackages()
	buildOrder := GetBuildOrder(allPackages)

	// Count occurrences of shared dependencies
	gmakeCount := 0
	pythonCount := 0
	vimCount := 0
	gitCount := 0

	for _, pkg := range buildOrder {
		switch pkg.PortDir {
		case "devel/gmake":
			gmakeCount++
		case "lang/python39":
			pythonCount++
		case "editors/vim":
			vimCount++
		case "devel/git":
			gitCount++
		}
	}

	// Each package should appear exactly once
	if gmakeCount != 1 {
		t.Errorf("gmake should appear once in build order, got %d", gmakeCount)
	}

	if pythonCount != 1 {
		t.Errorf("python39 should appear once in build order, got %d", pythonCount)
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

// TestIntegration_FlavoredPackage tests parsing and resolving a flavored port.
func TestIntegration_FlavoredPackage(t *testing.T) {
	restore := setTestQuerier(newTestFixtureQuerier(map[string]string{
		"editors/vim@python39":  "testdata/fixtures/editors__vim@python39.txt",
		"devel/gmake":           "testdata/fixtures/devel__gmake.txt",
		"lang/python39":         "testdata/fixtures/lang__python39.txt",
		"devel/gettext-runtime": "testdata/fixtures/devel__gettext-runtime.txt",
		"devel/gettext-tools":   "testdata/fixtures/devel__gettext-tools.txt",
		"devel/libffi":          "testdata/fixtures/devel__libffi.txt",
	}))
	defer restore()

	cfg := &config.Config{
		DPortsPath: "/usr/ports",
		MaxWorkers: 4,
	}

	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Parse flavored port
	packages, err := ParsePortList([]string{"editors/vim@python39"}, cfg, bsRegistry, pkgRegistry)
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(packages))
	}

	vim := packages[0]

	// Verify flavor is set
	if vim.Flavor != "python39" {
		t.Errorf("Expected Flavor 'python39', got '%s'", vim.Flavor)
	}

	if vim.PortDir != "editors/vim@python39" {
		t.Errorf("Expected PortDir 'editors/vim@python39', got '%s'", vim.PortDir)
	}

	// Verify package file reflects flavor
	if vim.PkgFile != "vim-python39-9.0.1234.pkg" {
		t.Errorf("Expected PkgFile 'vim-python39-9.0.1234.pkg', got '%s'", vim.PkgFile)
	}

	// Resolve dependencies
	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry)
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// Get all packages to verify dependency graph
	allPackages := pkgRegistry.AllPackages()
	if len(allPackages) < 2 {
		t.Fatalf("Expected at least 2 packages after resolution (vim + deps), got %d", len(allPackages))
	}

	// Verify python39 is a dependency (both build and run)
	hasPythonDep := false
	for _, dep := range vim.IDependOn {
		if dep.Pkg.PortDir == "lang/python39" {
			hasPythonDep = true
			break
		}
	}

	if !hasPythonDep {
		t.Error("Flavored vim should depend on python39")
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
	packages, err := ParsePortList([]string{"nonexistent/port"}, cfg, bsRegistry, pkgRegistry)

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
	packages, err := ParsePortList([]string{"x11/meta-gnome"}, cfg, bsRegistry, pkgRegistry)
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

// Helper function to find package index in slice
func findPackageIndex(packages []*Package, portDir string) int {
	for i, pkg := range packages {
		if pkg.PortDir == portDir {
			return i
		}
	}
	return -1
}
