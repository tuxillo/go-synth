//go:build freebsd || dragonfly
// +build freebsd dragonfly

package pkg

import (
	"os"
	"path/filepath"
	"testing"

	"dsynth/config"
	"dsynth/log"
)

// TestIntegrationBSD_RealPort tests against a real port in the ports tree
// This test only runs on FreeBSD/DragonFly where ports are available
func TestIntegrationBSD_RealPort(t *testing.T) {
	// Check if ports tree exists
	portsPath := "/usr/ports"
	if _, err := os.Stat(portsPath); os.IsNotExist(err) {
		t.Skip("Ports tree not found at /usr/ports")
	}

	// Test with a simple, stable port that's unlikely to change
	testPort := "devel/gmake"
	portPath := filepath.Join(portsPath, testPort)
	if _, err := os.Stat(portPath); os.IsNotExist(err) {
		t.Skipf("Test port %s not found", testPort)
	}

	// Create config
	cfg := &config.Config{
		DPortsPath: portsPath,
		MaxWorkers: 4,
	}

	// Create registries
	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Parse the port
	packages, err := ParsePortList([]string{testPort}, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	if len(packages) == 0 {
		t.Fatal("Expected at least 1 package")
	}

	gmake := packages[0]
	if gmake.PortDir != testPort {
		t.Errorf("Expected PortDir '%s', got '%s'", testPort, gmake.PortDir)
	}

	if gmake.Version == "" {
		t.Error("Expected non-empty version")
	}

	if gmake.PkgFile == "" {
		t.Error("Expected non-empty package file")
	}

	t.Logf("Successfully parsed %s: version=%s pkgfile=%s",
		gmake.PortDir, gmake.Version, gmake.PkgFile)

	// Resolve dependencies
	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	allPackages := pkgRegistry.AllPackages()
	t.Logf("Resolved %d total packages", len(allPackages))

	// Get build order
	buildOrder := GetBuildOrder(allPackages, log.NoOpLogger{})
	if len(buildOrder) < 1 {
		t.Error("Expected at least 1 package in build order")
	}

	// Verify gmake is in the build order
	found := false
	for _, pkg := range buildOrder {
		if pkg.PortDir == testPort {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("%s not found in build order", testPort)
	}

	// Verify build order is valid (dependencies come before dependents)
	pkgPositions := make(map[string]int)
	for i, pkg := range buildOrder {
		pkgPositions[pkg.PortDir] = i
	}

	for i, pkg := range buildOrder {
		for _, dep := range pkg.IDependOn {
			depPos, ok := pkgPositions[dep.Pkg.PortDir]
			if !ok {
				t.Errorf("Dependency %s not found in build order", dep.Pkg.PortDir)
				continue
			}
			if depPos >= i {
				t.Errorf("Dependency %s (pos %d) comes after %s (pos %d)",
					dep.Pkg.PortDir, depPos, pkg.PortDir, i)
			}
		}
	}

	t.Logf("Build order validated: %d packages in correct dependency order", len(buildOrder))
}

// TestIntegrationBSD_ComplexPort tests against a port with many dependencies
func TestIntegrationBSD_ComplexPort(t *testing.T) {
	// Check if ports tree exists
	portsPath := "/usr/ports"
	if _, err := os.Stat(portsPath); os.IsNotExist(err) {
		t.Skip("Ports tree not found at /usr/ports")
	}

	// Test with git which has multiple dependencies
	testPort := "devel/git"
	portPath := filepath.Join(portsPath, testPort)
	if _, err := os.Stat(portPath); os.IsNotExist(err) {
		t.Skipf("Test port %s not found", testPort)
	}

	// Create config
	cfg := &config.Config{
		DPortsPath: portsPath,
		MaxWorkers: 4,
	}

	// Create registries
	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Parse and resolve
	packages, err := ParsePortList([]string{testPort}, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	allPackages := pkgRegistry.AllPackages()
	t.Logf("Resolved %d total packages for %s", len(allPackages), testPort)

	if len(allPackages) < 5 {
		t.Errorf("Expected at least 5 packages for git (including deps), got %d", len(allPackages))
	}

	// Get build order
	buildOrder := GetBuildOrder(allPackages, log.NoOpLogger{})

	// Verify topological order
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
		t.Logf("Complex port validated: %d packages in correct dependency order", len(buildOrder))
	} else {
		t.Errorf("Found %d topological order violations", violations)
	}
}

// TestIntegrationBSD_FlavoredPort tests against a real flavored port
func TestIntegrationBSD_FlavoredPort(t *testing.T) {
	// Check if ports tree exists
	portsPath := "/usr/ports"
	if _, err := os.Stat(portsPath); os.IsNotExist(err) {
		t.Skip("Ports tree not found at /usr/ports")
	}

	// Test with python (has flavors)
	testPort := "lang/python@3.9"
	category := "lang"
	name := "python"
	flavor := "3.9"

	portPath := filepath.Join(portsPath, category, name)
	if _, err := os.Stat(portPath); os.IsNotExist(err) {
		t.Skipf("Test port %s not found", portPath)
	}

	// Create config
	cfg := &config.Config{
		DPortsPath: portsPath,
		MaxWorkers: 4,
	}

	// Create registries
	pkgRegistry := NewPackageRegistry()
	bsRegistry := NewBuildStateRegistry()

	// Parse the flavored port
	packages, err := ParsePortList([]string{testPort}, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ParsePortList failed: %v", err)
	}

	if len(packages) == 0 {
		t.Fatal("Expected at least 1 package")
	}

	python := packages[0]
	if python.Flavor != flavor {
		t.Errorf("Expected flavor '%s', got '%s'", flavor, python.Flavor)
	}

	// The PortDir might include the flavor or not, depending on implementation
	t.Logf("Successfully parsed flavored port: %s (flavor=%s)", python.PortDir, python.Flavor)

	// Resolve dependencies
	err = ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	allPackages := pkgRegistry.AllPackages()
	t.Logf("Resolved %d total packages for flavored port", len(allPackages))

	// Get build order
	buildOrder := GetBuildOrder(allPackages, log.NoOpLogger{})
	if len(buildOrder) < 1 {
		t.Error("Expected at least 1 package in build order")
	}

	t.Logf("Flavored port test complete: %d packages", len(buildOrder))
}
