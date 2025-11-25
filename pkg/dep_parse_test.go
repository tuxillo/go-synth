package pkg

import (
	"dsynth/config"
	"testing"
)

func TestParseDependencyStringBasic(t *testing.T) {
	cfg := &config.Config{DPortsPath: "/usr/ports"}
	deps := parseDependencyString("tool:/usr/ports/editors/vim:editors/vim lib.so:devel/git", cfg)
	if len(deps) != 2 {
		// Should capture two origins
		t.Fatalf("expected 2 origins, got %d", len(deps))
	}
	if deps[0].portDir != "editors/vim" || deps[1].portDir != "devel/git" {
		t.Fatalf("unexpected origins: %+v", deps)
	}
}

func TestParseDependencyStringFlavor(t *testing.T) {
	cfg := &config.Config{DPortsPath: "/usr/ports"}
	deps := parseDependencyString("bin:/usr/ports/lang/python@py39:lang/python@py39", cfg)
	if len(deps) != 1 || deps[0].portDir != "lang/python@py39" {
		t.Fatalf("flavor parsing failed: %+v", deps)
	}
}

func TestParseDependencyStringSkipsNonexistent(t *testing.T) {
	cfg := &config.Config{DPortsPath: "/usr/ports"}
	deps := parseDependencyString("/nonexistent:ignored editors/vim:editors/vim", cfg)
	if len(deps) != 1 || deps[0].portDir != "editors/vim" {
		t.Fatalf("expected only editors/vim, got %+v", deps)
	}
}
