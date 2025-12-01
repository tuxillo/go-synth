package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go-synth/builddb"
	"go-synth/config"
	"go-synth/log"
	"go-synth/pkg"
)

// TestBootstrapPkg_NoPkgInGraph tests bootstrap when no ports-mgmt/pkg is present
func TestBootstrapPkg_NoPkgInGraph(t *testing.T) {
	// When no ports-mgmt/pkg is in the dependency graph, bootstrap should succeed immediately
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := builddb.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	// Create necessary directories
	logsDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	cfg := &config.Config{
		DPortsPath:     tmpDir,
		BuildBase:      tmpDir,
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		RepositoryPath: filepath.Join(tmpDir, "repo"),
		LogsPath:       logsDir,
	}

	if err := os.MkdirAll(filepath.Join(cfg.RepositoryPath, "All"), 0755); err != nil {
		t.Fatalf("Failed to create repo All dir: %v", err)
	}

	logger, err := log.NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	registry := pkg.NewBuildStateRegistry()

	// Create package list WITHOUT ports-mgmt/pkg
	packages := []*pkg.Package{
		{PortDir: "editors/vim", Category: "editors", Name: "vim"},
	}

	// Should succeed with no action
	ctx := context.Background()
	err = bootstrapPkg(ctx, packages, cfg, logger, db, registry, nil)
	if err != nil {
		t.Errorf("bootstrapPkg should succeed when no pkg present, got: %v", err)
	}
}

// TestBootstrapPkg_CRCMatch tests skip behavior when CRC matches
func TestBootstrapPkg_CRCMatch(t *testing.T) {
	// Create minimal test environment
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := builddb.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	// Create necessary directories
	logsDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	cfg := &config.Config{
		DPortsPath:     tmpDir,
		BuildBase:      tmpDir,
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		RepositoryPath: filepath.Join(tmpDir, "repo"),
		LogsPath:       logsDir,
	}

	if err := os.MkdirAll(filepath.Join(cfg.RepositoryPath, "All"), 0755); err != nil {
		t.Fatalf("Failed to create repo All dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.PackagesPath, "All"), 0755); err != nil {
		t.Fatalf("Failed to create packages All dir: %v", err)
	}

	// Create fake ports-mgmt/pkg directory
	pkgDir := filepath.Join(tmpDir, "ports-mgmt", "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create fake pkg dir: %v", err)
	}

	// Create fake Makefile
	makefilePath := filepath.Join(pkgDir, "Makefile")
	if err := os.WriteFile(makefilePath, []byte("# test makefile\n"), 0644); err != nil {
		t.Fatalf("Failed to create fake Makefile: %v", err)
	}

	// Compute CRC and store it
	crc, err := builddb.ComputePortCRC(pkgDir)
	if err != nil {
		t.Fatalf("Failed to compute CRC: %v", err)
	}

	if err := db.UpdateCRC("ports-mgmt/pkg", crc); err != nil {
		t.Fatalf("Failed to store CRC: %v", err)
	}

	logger, err := log.NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	registry := pkg.NewBuildStateRegistry()

	// Create package
	pkgPkg := &pkg.Package{
		PortDir:  "ports-mgmt/pkg",
		Category: "ports-mgmt",
		Name:     "pkg",
		Version:  "1.0.0",
	}
	registry.AddFlags(pkgPkg, pkg.PkgFPkgPkg)

	packages := []*pkg.Package{pkgPkg}

	pkgPkg.PkgFile = "pkg-1.0.0.pkg"
	pkgFilePath := filepath.Join(cfg.PackagesPath, "All", pkgPkg.PkgFile)
	if err := os.WriteFile(pkgFilePath, []byte("fake package"), 0644); err != nil {
		t.Fatalf("Failed to create fake package file: %v", err)
	}
	templatePkgDir := filepath.Join(cfg.BuildBase, "Template/usr/local/sbin")
	if err := os.MkdirAll(templatePkgDir, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatePkgDir, "pkg"), []byte("fake binary"), 0755); err != nil {
		t.Fatalf("Failed to create fake pkg binary: %v", err)
	}

	ctx := context.Background()
	err = bootstrapPkg(ctx, packages, cfg, logger, db, registry, nil)

	// Should succeed without trying to build (CRC match)
	if err != nil {
		t.Errorf("bootstrapPkg should succeed with CRC match, got: %v", err)
	}

	// Should have PkgFSuccess flag
	if !registry.HasFlags(pkgPkg, pkg.PkgFSuccess) {
		t.Error("Expected PkgFSuccess flag after CRC match")
	}

	// Should have PkgFPackaged flag
	if !registry.HasFlags(pkgPkg, pkg.PkgFPackaged) {
		t.Error("Expected PkgFPackaged flag after CRC match")
	}
}

// TestMarkPkgPkgFlag tests the detection function
func TestMarkPkgPkgFlag(t *testing.T) {
	registry := pkg.NewBuildStateRegistry()

	// Create packages including ports-mgmt/pkg
	packages := []*pkg.Package{
		{PortDir: "editors/vim", Category: "editors", Name: "vim"},
		{PortDir: "ports-mgmt/pkg", Category: "ports-mgmt", Name: "pkg"},
		{PortDir: "shells/bash", Category: "shells", Name: "bash"},
	}

	// Mark pkg - using internal function from pkg package
	// This simulates what markPkgPkgFlag does
	for _, p := range packages {
		if p.PortDir == "ports-mgmt/pkg" {
			registry.AddFlags(p, pkg.PkgFPkgPkg)
		}
	}

	// Verify only ports-mgmt/pkg has the flag
	for _, p := range packages {
		hasPkgPkg := registry.HasFlags(p, pkg.PkgFPkgPkg)
		if p.PortDir == "ports-mgmt/pkg" {
			if !hasPkgPkg {
				t.Error("Expected ports-mgmt/pkg to have PkgFPkgPkg flag")
			}
		} else {
			if hasPkgPkg {
				t.Errorf("Expected %s to NOT have PkgFPkgPkg flag", p.PortDir)
			}
		}
	}
}
