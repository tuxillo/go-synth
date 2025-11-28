//go:build integration
// +build integration

// Package main_test contains end-to-end integration tests for the dsynth CLI.
//
// These tests verify the complete Phase 7 integration:
// - Init command creates directories and BuildDB
// - Legacy CRC migration works correctly
// - Status command displays database information
// - Commands work together as a cohesive system
//
// Limitations:
// - Does not test actual port builds (requires root + ports tree)
// - Focuses on CLI, database, and migration integration
// - Full build testing is done in builddb/integration_test.go
//
// Run with: go test -tags=integration -v .
package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dsynth/builddb"
	"github.com/google/uuid"
)

// ==================== Test Helper Functions ====================

// execDsynth executes the dsynth CLI with given arguments
func execDsynth(t *testing.T, args []string, configDir string) (stdout string, err error) {
	t.Helper()

	// Prepend -C flag if config directory specified
	if configDir != "" {
		args = append([]string{"-C", configDir}, args...)
	}

	cmd := exec.Command("./dsynth", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// setupTestEnvironment creates a temporary directory with minimal INI config
func setupTestEnvironment(t *testing.T) (tmpDir, configDir string) {
	t.Helper()

	tmpDir = t.TempDir()
	configDir = tmpDir
	buildBase := filepath.Join(tmpDir, "build")
	portsDir := filepath.Join(tmpDir, "dports")

	// Create minimal INI config
	configPath := filepath.Join(configDir, "dsynth.ini")
	configContent := fmt.Sprintf(`[Global Configuration]
Directory_buildbase=%s
Directory_portsdir=%s
Directory_repository=%s/packages
Directory_logs=%s/logs
Directory_distfiles=%s/distfiles
Directory_packages=%s/packages
Directory_options=%s/options
Number_of_builders=1
`, buildBase, portsDir, buildBase, buildBase, buildBase, buildBase, buildBase)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	return tmpDir, configDir
}

// assertDatabaseExists verifies database file exists and can be opened
func assertDatabaseExists(t *testing.T, dbPath string) *builddb.DB {
	t.Helper()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("Database does not exist: %s", dbPath)
	}

	db, err := builddb.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database %s: %v", dbPath, err)
	}

	return db
}

// assertDirectoryExists verifies directory was created
func assertDirectoryExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Fatalf("Directory does not exist: %s", path)
	}
	if err != nil {
		t.Fatalf("Failed to stat directory %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("Path exists but is not a directory: %s", path)
	}
}

// createLegacyCRCFile creates a fake legacy CRC file for migration testing
func createLegacyCRCFile(t *testing.T, buildBase string, entries map[string]uint32) {
	t.Helper()

	// Ensure build base exists
	if err := os.MkdirAll(buildBase, 0755); err != nil {
		t.Fatalf("Failed to create build base: %v", err)
	}

	crcFile := filepath.Join(buildBase, "crc_index")
	f, err := os.Create(crcFile)
	if err != nil {
		t.Fatalf("Failed to create legacy CRC file: %v", err)
	}
	defer f.Close()

	for portDir, crc := range entries {
		fmt.Fprintf(f, "%s:%08x\n", portDir, crc)
	}
}

// ==================== E2E Test Cases ====================

func TestE2E_InitCommand(t *testing.T) {
	tmpDir, configDir := setupTestEnvironment(t)
	buildBase := filepath.Join(tmpDir, "build")

	// Execute: dsynth init
	stdout, err := execDsynth(t, []string{"-y", "init"}, configDir)
	if err != nil {
		t.Fatalf("dsynth init failed: %v\nOutput: %s", err, stdout)
	}

	// Verify: Output contains success indicators
	if !strings.Contains(stdout, "✓") {
		t.Error("Expected success checkmarks in output")
	}
	if !strings.Contains(stdout, "Initialization complete") {
		t.Error("Expected completion message in output")
	}

	// Verify: Directories created
	assertDirectoryExists(t, buildBase)
	assertDirectoryExists(t, filepath.Join(buildBase, "logs"))
	assertDirectoryExists(t, filepath.Join(buildBase, "Template"))

	// Verify: Database created
	dbPath := filepath.Join(buildBase, "builds.db")
	db := assertDatabaseExists(t, dbPath)
	defer db.Close()

	// Verify: Database is functional
	stats, err := db.Stats()
	if err != nil {
		t.Fatalf("Failed to get database stats: %v", err)
	}

	if stats.TotalBuilds != 0 {
		t.Errorf("Expected 0 builds in fresh database, got %d", stats.TotalBuilds)
	}

	t.Log("✓ Init command successfully creates environment")
}

func TestE2E_InitIdempotent(t *testing.T) {
	tmpDir, configDir := setupTestEnvironment(t)
	buildBase := filepath.Join(tmpDir, "build")

	// Execute: dsynth init (first time)
	_, err := execDsynth(t, []string{"-y", "init"}, configDir)
	if err != nil {
		t.Fatalf("First init failed: %v", err)
	}

	// Execute: dsynth init (second time)
	stdout, err := execDsynth(t, []string{"-y", "init"}, configDir)
	if err != nil {
		t.Fatalf("Second init failed: %v\nOutput: %s", err, stdout)
	}

	// Verify: Still successful
	if !strings.Contains(stdout, "Initialization complete") {
		t.Error("Expected completion message on second init")
	}

	// Verify: Database still functional
	dbPath := filepath.Join(buildBase, "builds.db")
	db := assertDatabaseExists(t, dbPath)
	defer db.Close()

	t.Log("✓ Init command is idempotent")
}

func TestE2E_LegacyMigration(t *testing.T) {
	tmpDir, configDir := setupTestEnvironment(t)
	buildBase := filepath.Join(tmpDir, "build")

	// Setup: Create legacy CRC file
	legacyEntries := map[string]uint32{
		"editors/vim": 0x12345678,
		"shells/bash": 0xabcdef00,
		"devel/git":   0xdeadbeef,
	}
	createLegacyCRCFile(t, buildBase, legacyEntries)

	// Execute: dsynth init (should detect and migrate)
	stdout, err := execDsynth(t, []string{"-y", "init"}, configDir)
	if err != nil {
		t.Fatalf("Init with migration failed: %v\nOutput: %s", err, stdout)
	}

	// Verify: Output mentions migration
	if !strings.Contains(stdout, "Legacy CRC data detected") {
		t.Error("Expected migration detection message")
	}
	if !strings.Contains(stdout, "migrated successfully") {
		t.Error("Expected migration success message")
	}

	// Verify: Legacy file backed up
	backupFile := filepath.Join(buildBase, "crc_index.bak")
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Error("Expected legacy file to be backed up")
	}

	// Verify: CRC data in database
	dbPath := filepath.Join(buildBase, "builds.db")
	db := assertDatabaseExists(t, dbPath)
	defer db.Close()

	for portDir, expectedCRC := range legacyEntries {
		crc, exists, err := db.GetCRC(portDir)
		if err != nil {
			t.Errorf("Failed to get CRC for %s: %v", portDir, err)
			continue
		}
		if !exists {
			t.Errorf("CRC not found for %s after migration", portDir)
			continue
		}

		if crc != expectedCRC {
			t.Errorf("CRC mismatch for %s: got 0x%08x, want 0x%08x", portDir, crc, expectedCRC)
		}
	}

	t.Log("✓ Legacy migration works correctly")
}

func TestE2E_StatusCommand(t *testing.T) {
	tmpDir, configDir := setupTestEnvironment(t)
	buildBase := filepath.Join(tmpDir, "build")

	// Setup: Init environment
	_, err := execDsynth(t, []string{"-y", "init"}, configDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Setup: Add some fake build records
	dbPath := filepath.Join(buildBase, "builds.db")
	db, err := builddb.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Add fake records
	uuid1 := uuid.New().String()
	rec1 := &builddb.BuildRecord{
		UUID:      uuid1,
		PortDir:   "editors/vim",
		Version:   "1.2.3",
		Status:    "success",
		StartTime: time.Now().Add(-10 * time.Minute),
		EndTime:   time.Now(),
	}
	if err := db.SaveRecord(rec1); err != nil {
		db.Close()
		t.Fatalf("Failed to save record 1: %v", err)
	}
	if err := db.UpdatePackageIndex("editors/vim", "1.2.3", uuid1); err != nil {
		db.Close()
		t.Fatalf("Failed to update package index 1: %v", err)
	}

	uuid2 := uuid.New().String()
	rec2 := &builddb.BuildRecord{
		UUID:      uuid2,
		PortDir:   "shells/bash",
		Version:   "5.1.0",
		Status:    "failed",
		StartTime: time.Now().Add(-5 * time.Minute),
		EndTime:   time.Now(),
	}
	if err := db.SaveRecord(rec2); err != nil {
		db.Close()
		t.Fatalf("Failed to save record 2: %v", err)
	}
	if err := db.UpdatePackageIndex("shells/bash", "5.1.0", uuid2); err != nil {
		db.Close()
		t.Fatalf("Failed to update package index 2: %v", err)
	}

	db.Close()

	// Execute: dsynth status
	stdout, err := execDsynth(t, []string{"status"}, configDir)
	if err != nil {
		t.Fatalf("Status command failed: %v\nOutput: %s", err, stdout)
	}

	// Verify: Output contains database stats
	if !strings.Contains(stdout, "Build Database Status") {
		t.Error("Expected database statistics header")
	}
	if !strings.Contains(stdout, "Total builds:  2") {
		t.Error("Expected total builds count")
	}

	// Execute: dsynth status <port>
	stdout, err = execDsynth(t, []string{"status", "editors/vim"}, configDir)
	if err != nil {
		t.Fatalf("Port status failed: %v\nOutput: %s", err, stdout)
	}

	// Verify: Output contains port-specific info
	if !strings.Contains(stdout, "editors/vim") {
		t.Error("Expected port directory in output")
	}
	if !strings.Contains(stdout, uuid1[:8]) {
		t.Error("Expected UUID in output")
	}

	t.Log("✓ Status command displays correct information")
}

func TestE2E_ResetDBCommand(t *testing.T) {
	tmpDir, configDir := setupTestEnvironment(t)
	buildBase := filepath.Join(tmpDir, "build")

	// Setup: Init environment
	_, err := execDsynth(t, []string{"-y", "init"}, configDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	dbPath := filepath.Join(buildBase, "builds.db")

	// Verify: Database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("Database should exist before reset")
	}

	// Execute: dsynth reset-db
	stdout, err := execDsynth(t, []string{"-y", "reset-db"}, configDir)
	if err != nil {
		t.Fatalf("Reset-db failed: %v\nOutput: %s", err, stdout)
	}

	// Verify: Database removed
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Error("Database should be removed after reset")
	}

	// Verify: Can init again after reset
	_, err = execDsynth(t, []string{"-y", "init"}, configDir)
	if err != nil {
		t.Fatalf("Init after reset failed: %v", err)
	}

	t.Log("✓ Reset-db command works correctly")
}
