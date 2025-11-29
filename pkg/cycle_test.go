package pkg

import (
	"errors"
	"testing"
	"dsynth/log"
)

// createCycle builds A->B->C and introduces C depends on A forming a cycle
func createCycle() []*Package {
	c := &Package{PortDir: "cat/C", Category: "cat", Name: "C"}
	b := &Package{PortDir: "cat/B", Category: "cat", Name: "B"}
	a := &Package{PortDir: "cat/A", Category: "cat", Name: "A"}

	a.IDependOn = []*PkgLink{{Pkg: b, DepType: DepTypeBuild}}
	b.IDependOn = []*PkgLink{{Pkg: c, DepType: DepTypeBuild}}
	c.IDependOn = []*PkgLink{{Pkg: a, DepType: DepTypeBuild}} // cycle back to A

	b.DependsOnMe = []*PkgLink{{Pkg: a, DepType: DepTypeBuild}}
	c.DependsOnMe = []*PkgLink{{Pkg: b, DepType: DepTypeBuild}}
	a.DependsOnMe = []*PkgLink{{Pkg: c, DepType: DepTypeBuild}}

	return []*Package{a, b, c}
}

func TestTopoOrderStrictCycle(t *testing.T) {
	packages := createCycle()
	order, err := TopoOrderStrict(packages, log.NoOpLogger{})
	if err == nil {
		t.Fatalf("expected cycle detection error, got none (order len=%d)", len(order))
	}

	// Check that it's a CycleError with correct type
	var cycleErr *CycleError
	if !errors.As(err, &cycleErr) {
		t.Fatalf("expected *CycleError, got %T: %v", err, err)
	}

	// Verify cycle error has correct info
	if cycleErr.TotalPackages != 3 {
		t.Errorf("expected TotalPackages=3, got %d", cycleErr.TotalPackages)
	}
	if cycleErr.OrderedPackages >= 3 {
		t.Errorf("expected OrderedPackages < 3 (due to cycle), got %d", cycleErr.OrderedPackages)
	}

	// Verify we can also check with errors.Is()
	if !errors.Is(err, ErrCycleDetected) {
		t.Error("CycleError should be detected with errors.Is(err, ErrCycleDetected)")
	}

	if len(order) == 3 {
		t.Fatalf("expected partial order, got full length 3")
	}
}
