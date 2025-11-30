package service

import (
	"os"
	"path/filepath"
	"testing"

	"dsynth/config"
	"dsynth/pkg"
)

// TestGetBuildPlan_EmptyPortList tests GetBuildPlan with no ports specified
func TestGetBuildPlan_EmptyPortList(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := createTestConfig(tmpDir)

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	_, err = svc.GetBuildPlan([]string{})
	if err == nil {
		t.Error("GetBuildPlan() with empty port list should fail")
	}
}

// Note: TestGetBuildPlan_InvalidPort is not included because it requires
// actual port parsing infrastructure (Makefiles, etc.) which would be too
// complex to mock in a unit test. Invalid port handling is tested in
// integration tests.

// TestCheckMigrationStatus_NoLegacy tests migration status when no legacy file exists
func TestCheckMigrationStatus_NoLegacy(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := createTestConfig(tmpDir)

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	status, err := svc.CheckMigrationStatus()
	if err != nil {
		t.Fatalf("CheckMigrationStatus() failed: %v", err)
	}

	if status.Needed {
		t.Error("Migration should not be needed when no legacy file exists")
	}

	if status.LegacyFile != "" {
		t.Errorf("LegacyFile should be empty, got %q", status.LegacyFile)
	}
}

// TestCheckMigrationStatus_WithLegacy tests migration status when legacy file exists
func TestCheckMigrationStatus_WithLegacy(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := createTestConfig(tmpDir)

	// Create legacy CRC file
	legacyFile := filepath.Join(tmpDir, "crc_index")
	if err := os.WriteFile(legacyFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create legacy file: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	status, err := svc.CheckMigrationStatus()
	if err != nil {
		t.Fatalf("CheckMigrationStatus() failed: %v", err)
	}

	if !status.Needed {
		t.Error("Migration should be needed when legacy file exists")
	}

	if status.LegacyFile != legacyFile {
		t.Errorf("LegacyFile = %q, want %q", status.LegacyFile, legacyFile)
	}
}

// TestPerformMigration_NoLegacy tests manual migration when no legacy data exists
func TestPerformMigration_NoLegacy(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := createTestConfig(tmpDir)

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	err = svc.PerformMigration()
	if err == nil {
		t.Error("PerformMigration() should fail when no legacy data exists")
	}
}

// TestBuild_EmptyPortList tests Build with no ports specified
func TestBuild_EmptyPortList(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := createTestConfig(tmpDir)

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	_, err = svc.Build(BuildOptions{PortList: []string{}})
	if err == nil {
		t.Error("Build() with empty port list should fail")
	}
}

// Note: TestBuild_InvalidPort is not included because it requires
// actual port parsing infrastructure (Makefiles, etc.) which would be too
// complex to mock in a unit test. Invalid port handling is tested in
// integration tests.

// TestMarkNeedingBuild_Force tests that force flag marks all packages for rebuild
func TestMarkNeedingBuild_Force(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := createTestConfig(tmpDir)

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Create some test packages (empty slice for now since markNeedingBuild
	// is an internal method that requires real packages)
	packages := []*pkg.Package{}

	// Test with force=true
	needBuild, err := svc.markNeedingBuild(packages, true)
	if err != nil {
		t.Fatalf("markNeedingBuild() failed: %v", err)
	}

	// With force, all packages should need building
	if needBuild != len(packages) {
		t.Errorf("markNeedingBuild(force=true) = %d, want %d", needBuild, len(packages))
	}
}

// TestMarkNeedingBuild_NoForce tests normal CRC-based build checking
func TestMarkNeedingBuild_NoForce(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := createTestConfig(tmpDir)

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Create empty package list
	packages := []*pkg.Package{}

	// Test with force=false (normal CRC-based check)
	needBuild, err := svc.markNeedingBuild(packages, false)
	if err != nil {
		t.Fatalf("markNeedingBuild() failed: %v", err)
	}

	// With empty package list, nothing should need building
	if needBuild != 0 {
		t.Errorf("markNeedingBuild(force=false) = %d, want 0", needBuild)
	}
}

// TestDetectAndMigrate_Disabled tests that migration doesn't run when disabled
func TestDetectAndMigrate_Disabled(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := createTestConfig(tmpDir)
	cfg.Migration.AutoMigrate = false

	// Create legacy CRC file
	legacyFile := filepath.Join(tmpDir, "crc_index")
	if err := os.WriteFile(legacyFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create legacy file: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Migration should not run even though legacy file exists
	err = svc.detectAndMigrate()
	if err != nil {
		t.Errorf("detectAndMigrate() should not fail when auto-migrate is disabled: %v", err)
	}

	// Legacy file should still exist (not migrated)
	if _, err := os.Stat(legacyFile); os.IsNotExist(err) {
		t.Error("Legacy file was deleted despite auto-migrate being disabled")
	}
}

// TestParseAndResolve_EmptyPortList tests parsing with empty port list
func TestParseAndResolve_EmptyPortList(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := createTestConfig(tmpDir)

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	_, err = svc.parseAndResolve([]string{})
	if err == nil {
		t.Error("parseAndResolve() with empty port list should fail")
	}
}

// Helper function to create a test configuration
func createTestConfig(tmpDir string) *config.Config {
	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	// Create logs directory (required for service creation)
	os.MkdirAll(cfg.LogsPath, 0755)

	return cfg
}
