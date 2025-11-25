package pkg

import (
	"sync"
	"testing"
)

func TestBuildStateRegistry_Basic(t *testing.T) {
	registry := NewBuildStateRegistry()

	pkg := &Package{
		PortDir: "editors/vim",
		Name:    "vim",
	}

	// Test Get creates new state
	state := registry.Get(pkg)
	if state == nil {
		t.Fatal("Get should return non-nil state")
	}
	if state.Pkg != pkg {
		t.Error("BuildState.Pkg should reference the package")
	}
	if state.Flags != 0 {
		t.Error("New BuildState should have zero flags")
	}

	// Test Get returns same state
	state2 := registry.Get(pkg)
	if state != state2 {
		t.Error("Get should return same BuildState instance")
	}

	// Test Has
	if !registry.Has(pkg) {
		t.Error("Has should return true for tracked package")
	}

	otherPkg := &Package{PortDir: "editors/emacs"}
	if registry.Has(otherPkg) {
		t.Error("Has should return false for untracked package")
	}
}

func TestBuildStateRegistry_Flags(t *testing.T) {
	registry := NewBuildStateRegistry()
	pkg := &Package{PortDir: "editors/vim"}

	// Test SetFlags
	registry.SetFlags(pkg, PkgFManualSel)
	if registry.GetFlags(pkg) != PkgFManualSel {
		t.Error("SetFlags/GetFlags mismatch")
	}

	// Test AddFlags
	registry.AddFlags(pkg, PkgFSuccess)
	expected := PkgFManualSel | PkgFSuccess
	if registry.GetFlags(pkg) != expected {
		t.Errorf("AddFlags failed: got %x, want %x", registry.GetFlags(pkg), expected)
	}

	// Test HasFlags
	if !registry.HasFlags(pkg, PkgFManualSel) {
		t.Error("HasFlags should return true for set flag")
	}
	if !registry.HasFlags(pkg, PkgFSuccess) {
		t.Error("HasFlags should return true for added flag")
	}
	if registry.HasFlags(pkg, PkgFFailed) {
		t.Error("HasFlags should return false for unset flag")
	}

	// Test HasAnyFlags
	if !registry.HasAnyFlags(pkg, PkgFManualSel|PkgFFailed) {
		t.Error("HasAnyFlags should return true when at least one flag is set")
	}
	if registry.HasAnyFlags(pkg, PkgFFailed|PkgFSkipped) {
		t.Error("HasAnyFlags should return false when no flags are set")
	}

	// Test ClearFlags
	registry.ClearFlags(pkg, PkgFSuccess)
	if registry.HasFlags(pkg, PkgFSuccess) {
		t.Error("ClearFlags should clear the specified flag")
	}
	if !registry.HasFlags(pkg, PkgFManualSel) {
		t.Error("ClearFlags should not affect other flags")
	}
}

func TestBuildStateRegistry_IgnoreReason(t *testing.T) {
	registry := NewBuildStateRegistry()
	pkg := &Package{PortDir: "editors/vim"}

	// Test initial empty
	if registry.GetIgnoreReason(pkg) != "" {
		t.Error("Initial ignore reason should be empty")
	}

	// Test SetIgnoreReason
	reason := "requires X11"
	registry.SetIgnoreReason(pkg, reason)
	if registry.GetIgnoreReason(pkg) != reason {
		t.Error("SetIgnoreReason/GetIgnoreReason mismatch")
	}
}

func TestBuildStateRegistry_LastPhase(t *testing.T) {
	registry := NewBuildStateRegistry()
	pkg := &Package{PortDir: "editors/vim"}

	// Test initial empty
	if registry.GetLastPhase(pkg) != "" {
		t.Error("Initial last phase should be empty")
	}

	// Test SetLastPhase
	phase := "build"
	registry.SetLastPhase(pkg, phase)
	if registry.GetLastPhase(pkg) != phase {
		t.Error("SetLastPhase/GetLastPhase mismatch")
	}
}

func TestBuildStateRegistry_Set(t *testing.T) {
	registry := NewBuildStateRegistry()
	pkg := &Package{PortDir: "editors/vim"}

	state := &BuildState{
		Pkg:          pkg,
		Flags:        PkgFSuccess,
		IgnoreReason: "test",
		LastPhase:    "package",
	}

	registry.Set(pkg, state)

	retrieved := registry.Get(pkg)
	if retrieved != state {
		t.Error("Set should store the exact BuildState instance")
	}
	if retrieved.Flags != PkgFSuccess {
		t.Error("Set state not preserved")
	}
}

func TestBuildStateRegistry_Count(t *testing.T) {
	registry := NewBuildStateRegistry()

	if registry.Count() != 0 {
		t.Error("New registry should have count 0")
	}

	pkg1 := &Package{PortDir: "editors/vim"}
	pkg2 := &Package{PortDir: "editors/emacs"}

	registry.Get(pkg1)
	if registry.Count() != 1 {
		t.Error("Count should be 1 after first Get")
	}

	registry.Get(pkg2)
	if registry.Count() != 2 {
		t.Error("Count should be 2 after second Get")
	}

	// Getting same package shouldn't increase count
	registry.Get(pkg1)
	if registry.Count() != 2 {
		t.Error("Count should still be 2 after getting existing package")
	}
}

func TestBuildStateRegistry_Clear(t *testing.T) {
	registry := NewBuildStateRegistry()

	pkg := &Package{PortDir: "editors/vim"}
	registry.AddFlags(pkg, PkgFSuccess)

	if registry.Count() != 1 {
		t.Error("Should have 1 package before clear")
	}

	registry.Clear()

	if registry.Count() != 0 {
		t.Error("Should have 0 packages after clear")
	}

	// Getting after clear should create new state
	state := registry.Get(pkg)
	if state.Flags != 0 {
		t.Error("State after clear should be fresh with no flags")
	}
}

func TestBuildStateRegistry_Concurrent(t *testing.T) {
	registry := NewBuildStateRegistry()
	pkg := &Package{PortDir: "editors/vim"}

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent Get operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state := registry.Get(pkg)
			if state == nil {
				t.Error("Get returned nil")
			}
		}()
	}

	wg.Wait()

	// Should only have created one state
	if registry.Count() != 1 {
		t.Errorf("Concurrent Get should create exactly 1 state, got %d", registry.Count())
	}

	// Concurrent flag operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(flag int) {
			defer wg.Done()
			registry.AddFlags(pkg, 1<<flag)
		}(i % 10) // Use 10 different flags
	}

	wg.Wait()

	// Should have some flags set (exact value depends on timing, but not zero)
	if registry.GetFlags(pkg) == 0 {
		t.Error("Concurrent AddFlags should have set some flags")
	}
}
