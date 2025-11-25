package pkg

import (
	"fmt"
	"sync"
	"testing"
)

// TestPackageRegistry_Concurrent verifies that PackageRegistry is thread-safe
func TestPackageRegistry_Concurrent(t *testing.T) {
	registry := NewPackageRegistry()

	const numGoroutines = 100
	const packagesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Spawn multiple goroutines that simultaneously add and query packages
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Each goroutine creates and enters its own packages
			for j := 0; j < packagesPerGoroutine; j++ {
				portDir := fmt.Sprintf("cat%d/pkg%d", id, j)
				pkg := &Package{
					PortDir:  portDir,
					Category: fmt.Sprintf("cat%d", id),
					Name:     fmt.Sprintf("pkg%d", j),
				}

				// Enter the package
				result := registry.Enter(pkg)

				// Verify the returned package is valid
				if result == nil {
					t.Errorf("Enter returned nil for %s", portDir)
					return
				}

				// If this is a duplicate, result should be the existing package
				// Otherwise it should be our new package
				if result.PortDir != portDir {
					t.Errorf("Enter returned wrong package: expected %s, got %s", portDir, result.PortDir)
					return
				}

				// Try to find the package
				found := registry.Find(portDir)
				if found == nil {
					t.Errorf("Find returned nil for %s", portDir)
					return
				}
				if found.PortDir != portDir {
					t.Errorf("Find returned wrong package: expected %s, got %s", portDir, found.PortDir)
					return
				}
			}

			// Query packages created by other goroutines
			for k := 0; k < numGoroutines; k++ {
				for j := 0; j < packagesPerGoroutine; j++ {
					portDir := fmt.Sprintf("cat%d/pkg%d", k, j)
					_ = registry.Find(portDir) // May or may not find it yet, that's ok
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all packages were registered
	expectedCount := numGoroutines * packagesPerGoroutine
	actualCount := 0
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < packagesPerGoroutine; j++ {
			portDir := fmt.Sprintf("cat%d/pkg%d", i, j)
			if registry.Find(portDir) != nil {
				actualCount++
			}
		}
	}

	if actualCount != expectedCount {
		t.Errorf("Expected %d packages, found %d", expectedCount, actualCount)
	}
}

// TestPackageRegistry_EnterDuplicate verifies that Enter returns existing packages
func TestPackageRegistry_EnterDuplicate(t *testing.T) {
	registry := NewPackageRegistry()

	// Create and enter first package
	pkg1 := &Package{
		PortDir:  "editors/vim",
		Category: "editors",
		Name:     "vim",
		Version:  "9.0",
	}

	result1 := registry.Enter(pkg1)
	if result1 != pkg1 {
		t.Errorf("First Enter should return the same package")
	}

	// Try to enter a duplicate
	pkg2 := &Package{
		PortDir:  "editors/vim",
		Category: "editors",
		Name:     "vim",
		Version:  "9.1", // Different version
	}

	result2 := registry.Enter(pkg2)

	// Should return the first package, not the second
	if result2 != pkg1 {
		t.Errorf("Enter should return existing package on duplicate")
	}

	// Verify the version wasn't changed
	if result2.Version != "9.0" {
		t.Errorf("Expected version 9.0, got %s", result2.Version)
	}
}

// TestPackageRegistry_FindNonexistent verifies that Find returns nil for missing packages
func TestPackageRegistry_FindNonexistent(t *testing.T) {
	registry := NewPackageRegistry()

	result := registry.Find("editors/nonexistent")
	if result != nil {
		t.Errorf("Expected nil for nonexistent package, got %v", result)
	}
}
