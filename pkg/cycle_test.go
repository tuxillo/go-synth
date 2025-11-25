package pkg

import "testing"

// createCycle builds A->B->C and introduces C depends on A forming a cycle
func createCycle() *Package {
	c := &Package{PortDir: "cat/C", Category: "cat", Name: "C"}
	b := &Package{PortDir: "cat/B", Category: "cat", Name: "B"}
	a := &Package{PortDir: "cat/A", Category: "cat", Name: "A"}

	a.IDependOn = []*PkgLink{{Pkg: b, DepType: DepTypeBuild}}
	b.IDependOn = []*PkgLink{{Pkg: c, DepType: DepTypeBuild}}
	c.IDependOn = []*PkgLink{{Pkg: a, DepType: DepTypeBuild}} // cycle back to A

	b.DependsOnMe = []*PkgLink{{Pkg: a, DepType: DepTypeBuild}}
	c.DependsOnMe = []*PkgLink{{Pkg: b, DepType: DepTypeBuild}}
	a.DependsOnMe = []*PkgLink{{Pkg: c, DepType: DepTypeBuild}}

	a.Next = b
	b.Prev = a
	b.Next = c
	c.Prev = b
	return a
}

func TestTopoOrderStrictCycle(t *testing.T) {
	head := createCycle()
	order, err := TopoOrderStrict(head)
	if err == nil {
		t.Fatalf("expected cycle detection error, got none (order len=%d)", len(order))
	}
	if len(order) == 3 {
		t.Fatalf("expected partial order, got full length 3")
	}
}
