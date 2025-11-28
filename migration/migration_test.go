package migration_test

import (
	"os"
	"path/filepath"
	"testing"

	"dsynth/builddb"
	"dsynth/config"
	"dsynth/migration"
)

// TestMigrateLegacyCRC tests the main migration workflow with 3 CRC records.
func TestMigrateLegacyCRC(t *testing.T) {
	tmpDir := t.TempDir()

	// Create legacy CRC file
	legacyFile := filepath.Join(tmpDir, "crc_index")
	legacyData := `# Legacy CRC index
editors/vim:deadbeef
devel/git:cafebabe
www/nginx:12345678
`
	if err := os.WriteFile(legacyFile, []byte(legacyData), 0644); err != nil {
		t.Fatalf("Failed to create legacy file: %v", err)
	}

	// Setup config
	cfg := &config.Config{BuildBase: tmpDir}

	// Open BuildDB
	db, err := builddb.OpenDB(filepath.Join(tmpDir, "builds.db"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run migration
	err = migration.MigrateLegacyCRC(cfg, db)
	if err != nil {
		t.Fatalf("MigrateLegacyCRC() failed: %v", err)
	}

	// Verify CRCs imported
	testCases := []struct {
		port string
		want uint32
	}{
		{"editors/vim", 0xdeadbeef},
		{"devel/git", 0xcafebabe},
		{"www/nginx", 0x12345678},
	}

	for _, tc := range testCases {
		got, found, err := db.GetCRC(tc.port)
		if err != nil {
			t.Errorf("GetCRC(%s) error: %v", tc.port, err)
		}
		if !found {
			t.Errorf("CRC not found for %s", tc.port)
		}
		if got != tc.want {
			t.Errorf("CRC mismatch for %s: got %x, want %x", tc.port, got, tc.want)
		}
	}

	// Verify backup created
	backupFile := legacyFile + ".bak"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Error("Expected backup file to exist")
	}

	// Verify original removed
	if _, err := os.Stat(legacyFile); !os.IsNotExist(err) {
		t.Error("Expected original file to be renamed")
	}
}

// TestMigrateLegacyCRC_NoLegacyFile tests that missing file returns nil.
func TestMigrateLegacyCRC_NoLegacyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup config (no legacy file created)
	cfg := &config.Config{BuildBase: tmpDir}

	// Open BuildDB
	db, err := builddb.OpenDB(filepath.Join(tmpDir, "builds.db"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run migration
	err = migration.MigrateLegacyCRC(cfg, db)
	if err != nil {
		t.Errorf("Expected no error for missing file, got: %v", err)
	}
}

// TestMigrateLegacyCRC_InvalidLines tests that malformed lines are skipped.
func TestMigrateLegacyCRC_InvalidLines(t *testing.T) {
	tmpDir := t.TempDir()

	// Create legacy CRC file with invalid lines
	legacyFile := filepath.Join(tmpDir, "crc_index")
	legacyData := `# Comments should be skipped
editors/vim:deadbeef
invalid-line-no-colon
devel/git:INVALID_HEX
www/nginx:12345678

extra:fields:here:ignored
`
	if err := os.WriteFile(legacyFile, []byte(legacyData), 0644); err != nil {
		t.Fatalf("Failed to create legacy file: %v", err)
	}

	// Setup config
	cfg := &config.Config{BuildBase: tmpDir}

	// Open BuildDB
	db, err := builddb.OpenDB(filepath.Join(tmpDir, "builds.db"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run migration (should not fail)
	err = migration.MigrateLegacyCRC(cfg, db)
	if err != nil {
		t.Fatalf("MigrateLegacyCRC() failed: %v", err)
	}

	// Verify only valid records imported (vim and nginx)
	validPorts := []struct {
		port string
		want uint32
	}{
		{"editors/vim", 0xdeadbeef},
		{"www/nginx", 0x12345678},
	}

	for _, tc := range validPorts {
		_, found, _ := db.GetCRC(tc.port)
		if !found {
			t.Errorf("Expected valid CRC for %s to be imported", tc.port)
		}
	}

	// Verify invalid record not imported
	_, found, _ := db.GetCRC("devel/git")
	if found {
		t.Error("Expected invalid CRC for devel/git to be skipped")
	}
}

// TestMigrateLegacyCRC_EmptyFile tests empty file handling.
func TestMigrateLegacyCRC_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty legacy CRC file
	legacyFile := filepath.Join(tmpDir, "crc_index")
	if err := os.WriteFile(legacyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create legacy file: %v", err)
	}

	// Setup config
	cfg := &config.Config{BuildBase: tmpDir}

	// Open BuildDB
	db, err := builddb.OpenDB(filepath.Join(tmpDir, "builds.db"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run migration (should not fail)
	err = migration.MigrateLegacyCRC(cfg, db)
	if err != nil {
		t.Errorf("Expected no error for empty file, got: %v", err)
	}

	// Verify backup created
	backupFile := legacyFile + ".bak"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Error("Expected backup file to exist")
	}
}

// TestReadLegacyCRCFile tests file parsing logic.
func TestReadLegacyCRCFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test_crc")
	testData := `# Test CRC file
# Another comment
editors/vim:deadbeef

devel/git:cafebabe
www/nginx:12345678
`
	if err := os.WriteFile(testFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Parse file (we're testing the internal function via public API)
	// Since readLegacyCRCFile is not exported, we test via MigrateLegacyCRC
	cfg := &config.Config{BuildBase: tmpDir}

	// Rename to crc_index so MigrateLegacyCRC can find it
	legacyFile := filepath.Join(tmpDir, "crc_index")
	if err := os.Rename(testFile, legacyFile); err != nil {
		t.Fatalf("Failed to rename file: %v", err)
	}

	db, err := builddb.OpenDB(filepath.Join(tmpDir, "builds.db"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	err = migration.MigrateLegacyCRC(cfg, db)
	if err != nil {
		t.Fatalf("MigrateLegacyCRC() failed: %v", err)
	}

	// Verify 3 records imported (comments and blank lines ignored)
	testCases := []string{"editors/vim", "devel/git", "www/nginx"}
	for _, port := range testCases {
		_, found, _ := db.GetCRC(port)
		if !found {
			t.Errorf("Expected CRC for %s to be imported", port)
		}
	}
}

// TestDetectMigrationNeeded tests detection logic.
func TestDetectMigrationNeeded(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{BuildBase: tmpDir}

	// Test: no legacy file
	if migration.DetectMigrationNeeded(cfg) {
		t.Error("Expected DetectMigrationNeeded to return false when no file exists")
	}

	// Create legacy file
	legacyFile := filepath.Join(tmpDir, "crc_index")
	if err := os.WriteFile(legacyFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create legacy file: %v", err)
	}

	// Test: legacy file exists
	if !migration.DetectMigrationNeeded(cfg) {
		t.Error("Expected DetectMigrationNeeded to return true when file exists")
	}
}

// TestMigrateLegacyCRC_Idempotent tests that migration is safe to run twice.
func TestMigrateLegacyCRC_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create legacy CRC file
	legacyFile := filepath.Join(tmpDir, "crc_index")
	legacyData := `editors/vim:deadbeef
devel/git:cafebabe
`
	if err := os.WriteFile(legacyFile, []byte(legacyData), 0644); err != nil {
		t.Fatalf("Failed to create legacy file: %v", err)
	}

	// Setup config
	cfg := &config.Config{BuildBase: tmpDir}

	// Open BuildDB
	db, err := builddb.OpenDB(filepath.Join(tmpDir, "builds.db"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run migration first time
	err = migration.MigrateLegacyCRC(cfg, db)
	if err != nil {
		t.Fatalf("MigrateLegacyCRC() first run failed: %v", err)
	}

	// Verify backup exists
	backupFile := legacyFile + ".bak"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Fatal("Expected backup file to exist after first run")
	}

	// Run migration second time (should be no-op, no legacy file exists)
	err = migration.MigrateLegacyCRC(cfg, db)
	if err != nil {
		t.Errorf("MigrateLegacyCRC() second run failed: %v", err)
	}

	// Verify CRCs still exist
	_, found, _ := db.GetCRC("editors/vim")
	if !found {
		t.Error("Expected CRC to still exist after second migration")
	}
}
