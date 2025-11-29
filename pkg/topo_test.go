package pkg

import (
	"dsynth/log"
	"errors"
	"fmt"
	"testing"

	"dsynth/config"
)

// createSimpleChain builds A->B->C where A depends on B and C
func createSimpleChain() []*Package {
	c := &Package{PortDir: "cat/C", Category: "cat", Name: "C"}
	b := &Package{PortDir: "cat/B", Category: "cat", Name: "B"}
	a := &Package{PortDir: "cat/A", Category: "cat", Name: "A"}

	// Link A depends on B and C, B depends on C
	a.IDependOn = []*PkgLink{{Pkg: b, DepType: DepTypeBuild}, {Pkg: c, DepType: DepTypeBuild}}
	b.IDependOn = []*PkgLink{{Pkg: c, DepType: DepTypeBuild}}

	// Reverse links
	b.DependsOnMe = []*PkgLink{{Pkg: a, DepType: DepTypeBuild}}
	c.DependsOnMe = []*PkgLink{{Pkg: a, DepType: DepTypeBuild}, {Pkg: b, DepType: DepTypeBuild}}

	return []*Package{a, b, c}
}

func TestTopoOrderSimple(t *testing.T) {
	packages := createSimpleChain()
	order := TopoOrder(packages, log.NoOpLogger{})
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

// createPriorityTestGraph builds a graph to test priority ordering:
//
// Zero-dependency packages (all at same level):
//   - pkgconf: 0 deps, 3 dependents (high fanout)
//   - perl: 0 deps, 2 dependents (medium fanout)
//   - expat: 0 deps, 1 dependent (low fanout)
//
// First-level dependents:
//   - lib1, lib2, lib3 depend on pkgconf
//   - tool1, tool2 depend on perl
//   - parser depends on expat
//
// Second-level:
//   - app depends on lib1, tool1, parser
//
// Expected ordering with priority: pkgconf, perl, expat (by dependent count)
func createPriorityTestGraph() []*Package {
	// Zero-dependency packages
	pkgconf := &Package{PortDir: "devel/pkgconf", Category: "devel", Name: "pkgconf"}
	perl := &Package{PortDir: "lang/perl", Category: "lang", Name: "perl"}
	expat := &Package{PortDir: "textproc/expat", Category: "textproc", Name: "expat"}

	// First-level dependents
	lib1 := &Package{PortDir: "devel/lib1", Category: "devel", Name: "lib1"}
	lib2 := &Package{PortDir: "devel/lib2", Category: "devel", Name: "lib2"}
	lib3 := &Package{PortDir: "devel/lib3", Category: "devel", Name: "lib3"}
	tool1 := &Package{PortDir: "devel/tool1", Category: "devel", Name: "tool1"}
	tool2 := &Package{PortDir: "devel/tool2", Category: "devel", Name: "tool2"}
	parser := &Package{PortDir: "textproc/parser", Category: "textproc", Name: "parser"}

	// Second-level
	app := &Package{PortDir: "devel/app", Category: "devel", Name: "app"}

	// Setup dependencies
	lib1.IDependOn = []*PkgLink{{Pkg: pkgconf, DepType: DepTypeBuild}}
	lib2.IDependOn = []*PkgLink{{Pkg: pkgconf, DepType: DepTypeBuild}}
	lib3.IDependOn = []*PkgLink{{Pkg: pkgconf, DepType: DepTypeBuild}}
	tool1.IDependOn = []*PkgLink{{Pkg: perl, DepType: DepTypeBuild}}
	tool2.IDependOn = []*PkgLink{{Pkg: perl, DepType: DepTypeBuild}}
	parser.IDependOn = []*PkgLink{{Pkg: expat, DepType: DepTypeBuild}}
	app.IDependOn = []*PkgLink{
		{Pkg: lib1, DepType: DepTypeBuild},
		{Pkg: tool1, DepType: DepTypeBuild},
		{Pkg: parser, DepType: DepTypeBuild},
	}

	// Setup reverse dependencies (DependsOnMe)
	pkgconf.DependsOnMe = []*PkgLink{
		{Pkg: lib1, DepType: DepTypeBuild},
		{Pkg: lib2, DepType: DepTypeBuild},
		{Pkg: lib3, DepType: DepTypeBuild},
	}
	perl.DependsOnMe = []*PkgLink{
		{Pkg: tool1, DepType: DepTypeBuild},
		{Pkg: tool2, DepType: DepTypeBuild},
	}
	expat.DependsOnMe = []*PkgLink{
		{Pkg: parser, DepType: DepTypeBuild},
	}
	lib1.DependsOnMe = []*PkgLink{{Pkg: app, DepType: DepTypeBuild}}
	tool1.DependsOnMe = []*PkgLink{{Pkg: app, DepType: DepTypeBuild}}
	parser.DependsOnMe = []*PkgLink{{Pkg: app, DepType: DepTypeBuild}}

	// Calculate depths
	calculateDepthRecursive(pkgconf)
	calculateDepthRecursive(perl)
	calculateDepthRecursive(expat)
	calculateDepthRecursive(lib1)
	calculateDepthRecursive(lib2)
	calculateDepthRecursive(lib3)
	calculateDepthRecursive(tool1)
	calculateDepthRecursive(tool2)
	calculateDepthRecursive(parser)
	calculateDepthRecursive(app)

	return []*Package{pkgconf, perl, expat, lib1, lib2, lib3, tool1, tool2, parser, app}
}

func TestGetBuildOrderPriority(t *testing.T) {
	packages := createPriorityTestGraph()
	order := GetBuildOrder(packages, log.NoOpLogger{})

	if len(order) != 10 {
		t.Fatalf("expected 10 packages, got %d", len(order))
	}

	// First three should be the zero-dependency packages in priority order
	// pkgconf (3 dependents) > perl (2 dependents) > expat (1 dependent)
	if order[0].Name != "pkgconf" {
		t.Errorf("expected first package to be pkgconf (3 dependents), got %s", order[0].Name)
	}
	if order[1].Name != "perl" {
		t.Errorf("expected second package to be perl (2 dependents), got %s", order[1].Name)
	}
	if order[2].Name != "expat" {
		t.Errorf("expected third package to be expat (1 dependent), got %s", order[2].Name)
	}

	// Last package should be app (depends on everything)
	if order[9].Name != "app" {
		t.Errorf("expected last package to be app, got %s", order[9].Name)
	}

	// Verify topological correctness: all dependencies come before dependents
	pos := make(map[string]int)
	for i, pkg := range order {
		pos[pkg.PortDir] = i
	}

	for _, pkg := range order {
		for _, depLink := range pkg.IDependOn {
			depPos := pos[depLink.Pkg.PortDir]
			pkgPos := pos[pkg.PortDir]
			if depPos >= pkgPos {
				t.Errorf("package %s (pos %d) depends on %s (pos %d) - dependency violation",
					pkg.PortDir, pkgPos, depLink.Pkg.PortDir, depPos)
			}
		}
	}
}

// createDepiDepthVsFanoutGraph tests that fanout takes priority over DepiDepth:
//
// Zero-dependency packages (will be ready at same time):
//   - base1: 0 deps, 1 dependent (low fanout, but will have high DepiDepth)
//   - base2: 0 deps, 100 dependents (high fanout, but will have low DepiDepth)
//
// Chain from base1 (creates high DepiDepth):
//   - chain1 -> chain2 -> chain3 -> chain4 -> chain5
//
// Wide tree from base2 (creates low DepiDepth but high fanout):
//   - wide1, wide2, wide3, ... wide100
//
// Expected: base2 should come BEFORE base1 despite base1 having higher DepiDepth
func createDepiDepthVsFanoutGraph() []*Package {
	base1 := &Package{PortDir: "devel/base1", Category: "devel", Name: "base1"}
	base2 := &Package{PortDir: "devel/base2", Category: "devel", Name: "base2"}

	// Create deep chain from base1
	chain1 := &Package{PortDir: "devel/chain1", Category: "devel", Name: "chain1"}
	chain2 := &Package{PortDir: "devel/chain2", Category: "devel", Name: "chain2"}
	chain3 := &Package{PortDir: "devel/chain3", Category: "devel", Name: "chain3"}
	chain4 := &Package{PortDir: "devel/chain4", Category: "devel", Name: "chain4"}
	chain5 := &Package{PortDir: "devel/chain5", Category: "devel", Name: "chain5"}

	chain1.IDependOn = []*PkgLink{{Pkg: base1, DepType: DepTypeBuild}}
	chain2.IDependOn = []*PkgLink{{Pkg: chain1, DepType: DepTypeBuild}}
	chain3.IDependOn = []*PkgLink{{Pkg: chain2, DepType: DepTypeBuild}}
	chain4.IDependOn = []*PkgLink{{Pkg: chain3, DepType: DepTypeBuild}}
	chain5.IDependOn = []*PkgLink{{Pkg: chain4, DepType: DepTypeBuild}}

	base1.DependsOnMe = []*PkgLink{{Pkg: chain1, DepType: DepTypeBuild}}
	chain1.DependsOnMe = []*PkgLink{{Pkg: chain2, DepType: DepTypeBuild}}
	chain2.DependsOnMe = []*PkgLink{{Pkg: chain3, DepType: DepTypeBuild}}
	chain3.DependsOnMe = []*PkgLink{{Pkg: chain4, DepType: DepTypeBuild}}
	chain4.DependsOnMe = []*PkgLink{{Pkg: chain5, DepType: DepTypeBuild}}

	// Create wide fanout from base2 (just 5 to keep test simple)
	widePackages := make([]*Package, 5)
	base2.DependsOnMe = make([]*PkgLink, 5)
	for i := 0; i < 5; i++ {
		widePackages[i] = &Package{
			PortDir:  fmt.Sprintf("devel/wide%d", i),
			Category: "devel",
			Name:     fmt.Sprintf("wide%d", i),
		}
		widePackages[i].IDependOn = []*PkgLink{{Pkg: base2, DepType: DepTypeBuild}}
		base2.DependsOnMe[i] = &PkgLink{Pkg: widePackages[i], DepType: DepTypeBuild}
	}

	// Calculate depths
	calculateDepthRecursive(base1)
	calculateDepthRecursive(base2)
	calculateDepthRecursive(chain1)
	calculateDepthRecursive(chain2)
	calculateDepthRecursive(chain3)
	calculateDepthRecursive(chain4)
	calculateDepthRecursive(chain5)
	for _, p := range widePackages {
		calculateDepthRecursive(p)
	}

	allPackages := []*Package{base1, base2, chain1, chain2, chain3, chain4, chain5}
	allPackages = append(allPackages, widePackages...)
	return allPackages
}

func TestPriorityFanoutOverDepth(t *testing.T) {
	packages := createDepiDepthVsFanoutGraph()
	order := GetBuildOrder(packages, log.NoOpLogger{})

	// Find positions of base1 and base2
	base1Pos := -1
	base2Pos := -1
	for i, pkg := range order {
		if pkg.Name == "base1" {
			base1Pos = i
		}
		if pkg.Name == "base2" {
			base2Pos = i
		}
	}

	if base1Pos == -1 || base2Pos == -1 {
		t.Fatalf("couldn't find base1 or base2 in build order")
	}

	// base2 (5 dependents) should come BEFORE base1 (1 dependent)
	// even though base1 has higher DepiDepth
	if base2Pos > base1Pos {
		t.Errorf("base2 (5 dependents, pos %d) should come before base1 (1 dependent, pos %d) - fanout should take priority over DepiDepth",
			base2Pos, base1Pos)
	}
}

func TestParseAliasNoPorts(t *testing.T) {
	cfg := &config.Config{DPortsPath: "/nonexistent"}
	registry := NewBuildStateRegistry()
	pkgRegistry := NewPackageRegistry()
	_, err := Parse([]string{}, cfg, registry, pkgRegistry, log.NoOpLogger{})

	if err == nil {
		// ParsePortList returns error on no valid ports; ensure we handle it
		t.Fatalf("expected error for empty spec list")
	}

	// Verify it's the right error type
	if !errors.Is(err, ErrNoValidPorts) {
		t.Errorf("expected ErrNoValidPorts, got: %v", err)
	}
}
