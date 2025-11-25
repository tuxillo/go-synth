package pkg

import (
	"errors"
	"testing"

	"dsynth/config"
)

// createSimpleChain builds A->B->C where A depends on B and C
func createSimpleChain() *Package {
	c := &Package{PortDir: "cat/C", Category: "cat", Name: "C"}
	b := &Package{PortDir: "cat/B", Category: "cat", Name: "B"}
	a := &Package{PortDir: "cat/A", Category: "cat", Name: "A"}

	// Link A depends on B and C, B depends on C
	a.IDependOn = []*PkgLink{{Pkg: b, DepType: DepTypeBuild}, {Pkg: c, DepType: DepTypeBuild}}
	b.IDependOn = []*PkgLink{{Pkg: c, DepType: DepTypeBuild}}

	// Reverse links
	b.DependsOnMe = []*PkgLink{{Pkg: a, DepType: DepTypeBuild}}
	c.DependsOnMe = []*PkgLink{{Pkg: a, DepType: DepTypeBuild}, {Pkg: b, DepType: DepTypeBuild}}

	// Build linked list a->b->c
	a.Next = b
	b.Prev = a
	b.Next = c
	c.Prev = b

	return a
}

func TestTopoOrderSimple(t *testing.T) {
	head := createSimpleChain()
	order := TopoOrder(head)
	if len(order) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(order))
	}

	// First must be C (no deps)
	if order[0].Name != "C" {
		t.Fatalf("expected first to be C, got %s", order[0].Name)
	}
	// Second should be B
	if order[1].Name != "B" {
		t.Fatalf("expected second to be B, got %s", order[1].Name)
	}
	// Last should be A
	if order[2].Name != "A" {
		t.Fatalf("expected third to be A, got %s", order[2].Name)
	}
}

func TestParseAliasNoPorts(t *testing.T) {
	cfg := &config.Config{DPortsPath: "/nonexistent"}
	registry := NewBuildStateRegistry()
	pkgRegistry := NewPackageRegistry()
	_, err := Parse([]string{}, cfg, registry, pkgRegistry)

	if err == nil {
		// ParsePortList returns error on no valid ports; ensure we handle it
		t.Fatalf("expected error for empty spec list")
	}

	// Verify it's the right error type
	if !errors.Is(err, ErrNoValidPorts) {
		t.Errorf("expected ErrNoValidPorts, got: %v", err)
	}
}
