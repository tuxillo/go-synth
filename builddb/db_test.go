package builddb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

// ==================== Test Helpers ====================

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) (*DB, string) {
	t.Helper()

	// Create temp directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	return db, dbPath
}

// cleanupTestDB closes the database and removes files
func cleanupTestDB(t *testing.T, db *DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
}

// createTestRecord creates a BuildRecord with test data
func createTestRecord(uuid, portDir, version, status string) *BuildRecord {
	now := time.Now()
	rec := &BuildRecord{
		UUID:      uuid,
		PortDir:   portDir,
		Version:   version,
		Status:    status,
		StartTime: now,
	}
	if status == "success" || status == "failed" {
		rec.EndTime = now.Add(5 * time.Minute)
	}
	return rec
}

// assertRecordEqual compares two BuildRecords for equality
func assertRecordEqual(t *testing.T, expected, actual *BuildRecord) {
	t.Helper()
	if actual.UUID != expected.UUID {
		t.Errorf("UUID mismatch: got %q, want %q", actual.UUID, expected.UUID)
	}
	if actual.PortDir != expected.PortDir {
		t.Errorf("PortDir mismatch: got %q, want %q", actual.PortDir, expected.PortDir)
	}
	if actual.Version != expected.Version {
		t.Errorf("Version mismatch: got %q, want %q", actual.Version, expected.Version)
	}
	if actual.Status != expected.Status {
		t.Errorf("Status mismatch: got %q, want %q", actual.Status, expected.Status)
	}
	// Compare timestamps within 1 second tolerance (JSON serialization may lose precision)
	if !actual.StartTime.Round(time.Second).Equal(expected.StartTime.Round(time.Second)) {
		t.Errorf("StartTime mismatch: got %v, want %v", actual.StartTime, expected.StartTime)
	}
	if !actual.EndTime.IsZero() && !expected.EndTime.IsZero() {
		if !actual.EndTime.Round(time.Second).Equal(expected.EndTime.Round(time.Second)) {
			t.Errorf("EndTime mismatch: got %v, want %v", actual.EndTime, expected.EndTime)
		}
	}
}

// createTestPortDir creates a temporary port directory with files
func createTestPortDir(t *testing.T, files map[string]string) string {
	t.Helper()

	portDir := filepath.Join(t.TempDir(), "testport")
	if err := os.MkdirAll(portDir, 0755); err != nil {
		t.Fatalf("Failed to create test port directory: %v", err)
	}

	for relPath, content := range files {
		fullPath := filepath.Join(portDir, relPath)
		dir := filepath.Dir(fullPath)

		// Create parent directories if needed
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	return portDir
}

// verifyBucketsExist checks that all required buckets exist in the database
func verifyBucketsExist(t *testing.T, db *DB) {
	t.Helper()

	err := db.db.View(func(tx *bolt.Tx) error {
		buckets := []string{BucketBuilds, BucketPackages, BucketCRCIndex}
		for _, name := range buckets {
			if tx.Bucket([]byte(name)) == nil {
				t.Errorf("Bucket %q does not exist", name)
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to verify buckets: %v", err)
	}
}

// ==================== Group 1: Database Lifecycle Tests ====================

func TestOpenDB(t *testing.T) {
	t.Run("create new database", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "new.db")

		db, err := OpenDB(dbPath)
		if err != nil {
			t.Fatalf("OpenDB() failed: %v", err)
		}
		defer cleanupTestDB(t, db)

		// Verify database exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Error("Database file was not created")
		}

		// Verify buckets exist
		verifyBucketsExist(t, db)
	})

	t.Run("open existing database", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "existing.db")

		// Create database
		db1, err := OpenDB(dbPath)
		if err != nil {
			t.Fatalf("OpenDB() failed on create: %v", err)
		}
		db1.Close()

		// Reopen database
		db2, err := OpenDB(dbPath)
		if err != nil {
			t.Fatalf("OpenDB() failed on reopen: %v", err)
		}
		defer cleanupTestDB(t, db2)

		// Verify buckets still exist
		verifyBucketsExist(t, db2)
	})

	t.Run("invalid path", func(t *testing.T) {
		_, err := OpenDB("/nonexistent/directory/test.db")
		if err == nil {
			t.Error("OpenDB() should fail with invalid path")
		}
		if !IsDatabaseError(err) {
			t.Errorf("Expected DatabaseError, got %T", err)
		}
	})
}

func TestClose(t *testing.T) {
	t.Run("close open database", func(t *testing.T) {
		db, _ := setupTestDB(t)

		err := db.Close()
		if err != nil {
			t.Errorf("Close() failed: %v", err)
		}
	})

	t.Run("multiple close calls", func(t *testing.T) {
		db, _ := setupTestDB(t)

		// First close
		if err := db.Close(); err != nil {
			t.Errorf("First Close() failed: %v", err)
		}

		// Second close should not error (idempotent)
		if err := db.Close(); err != nil {
			t.Errorf("Second Close() failed: %v", err)
		}
	})
}

// ==================== Group 2: Build Record CRUD Tests ====================

func TestSaveRecord(t *testing.T) {
	t.Run("save valid record", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		rec := createTestRecord("test-uuid-1", "editors/vim", "9.0.1", "running")

		err := db.SaveRecord(rec)
		if err != nil {
			t.Fatalf("SaveRecord() failed: %v", err)
		}

		// Verify record was saved
		retrieved, err := db.GetRecord("test-uuid-1")
		if err != nil {
			t.Fatalf("GetRecord() failed: %v", err)
		}
		assertRecordEqual(t, rec, retrieved)
	})

	t.Run("overwrite existing record", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Save first version
		rec1 := createTestRecord("test-uuid-1", "editors/vim", "9.0.1", "running")
		db.SaveRecord(rec1)

		// Overwrite with second version
		rec2 := createTestRecord("test-uuid-1", "editors/vim", "9.0.2", "success")
		db.SaveRecord(rec2)

		// Verify second version is stored
		retrieved, _ := db.GetRecord("test-uuid-1")
		if retrieved.Version != "9.0.2" {
			t.Errorf("Expected version 9.0.2, got %s", retrieved.Version)
		}
	})

	t.Run("save multiple records", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		records := []*BuildRecord{
			createTestRecord("uuid-1", "editors/vim", "9.0.1", "success"),
			createTestRecord("uuid-2", "lang/python", "3.11.0", "running"),
			createTestRecord("uuid-3", "www/nginx", "1.24.0", "failed"),
		}

		// Save all records
		for _, rec := range records {
			if err := db.SaveRecord(rec); err != nil {
				t.Fatalf("SaveRecord() failed for %s: %v", rec.UUID, err)
			}
		}

		// Verify all records exist
		for _, expected := range records {
			retrieved, err := db.GetRecord(expected.UUID)
			if err != nil {
				t.Errorf("GetRecord(%s) failed: %v", expected.UUID, err)
			}
			assertRecordEqual(t, expected, retrieved)
		}
	})

	t.Run("empty UUID validation", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		rec := createTestRecord("", "editors/vim", "9.0.1", "running")

		err := db.SaveRecord(rec)
		if err == nil {
			t.Error("SaveRecord() should fail with empty UUID")
		}
		if !IsValidationError(err) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})
}

func TestGetRecord(t *testing.T) {
	t.Run("retrieve existing record", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		expected := createTestRecord("test-uuid-1", "editors/vim", "9.0.1", "success")
		db.SaveRecord(expected)

		retrieved, err := db.GetRecord("test-uuid-1")
		if err != nil {
			t.Fatalf("GetRecord() failed: %v", err)
		}

		assertRecordEqual(t, expected, retrieved)
	})

	t.Run("UUID not found", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		_, err := db.GetRecord("nonexistent-uuid")
		if err == nil {
			t.Error("GetRecord() should fail for nonexistent UUID")
		}
		if !IsRecordNotFound(err) {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("empty UUID validation", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		_, err := db.GetRecord("")
		if err == nil {
			t.Error("GetRecord() should fail with empty UUID")
		}
		if !IsValidationError(err) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})
}

func TestUpdateRecordStatus(t *testing.T) {
	t.Run("update running to success", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Save running record
		rec := createTestRecord("test-uuid-1", "editors/vim", "9.0.1", "running")
		rec.EndTime = time.Time{} // Clear end time
		db.SaveRecord(rec)

		// Update to success
		endTime := time.Now()
		err := db.UpdateRecordStatus("test-uuid-1", "success", endTime)
		if err != nil {
			t.Fatalf("UpdateRecordStatus() failed: %v", err)
		}

		// Verify update
		updated, _ := db.GetRecord("test-uuid-1")
		if updated.Status != "success" {
			t.Errorf("Status not updated: got %q, want %q", updated.Status, "success")
		}
		if updated.EndTime.Round(time.Second) != endTime.Round(time.Second) {
			t.Errorf("EndTime not updated: got %v, want %v", updated.EndTime, endTime)
		}
	})

	t.Run("update running to failed", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		rec := createTestRecord("test-uuid-1", "editors/vim", "9.0.1", "running")
		db.SaveRecord(rec)

		endTime := time.Now()
		err := db.UpdateRecordStatus("test-uuid-1", "failed", endTime)
		if err != nil {
			t.Fatalf("UpdateRecordStatus() failed: %v", err)
		}

		updated, _ := db.GetRecord("test-uuid-1")
		if updated.Status != "failed" {
			t.Errorf("Status not updated: got %q, want %q", updated.Status, "failed")
		}
	})

	t.Run("other fields unchanged", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		rec := createTestRecord("test-uuid-1", "editors/vim", "9.0.1", "running")
		db.SaveRecord(rec)

		endTime := time.Now()
		db.UpdateRecordStatus("test-uuid-1", "success", endTime)

		updated, _ := db.GetRecord("test-uuid-1")
		// Verify other fields didn't change
		if updated.UUID != rec.UUID {
			t.Error("UUID should not change")
		}
		if updated.PortDir != rec.PortDir {
			t.Error("PortDir should not change")
		}
		if updated.Version != rec.Version {
			t.Error("Version should not change")
		}
	})

	t.Run("nonexistent UUID", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		err := db.UpdateRecordStatus("nonexistent-uuid", "success", time.Now())
		if err == nil {
			t.Error("UpdateRecordStatus() should fail for nonexistent UUID")
		}
		if !IsRecordNotFound(err) {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

// ==================== Group 3: Package Index Tests ====================

func TestUpdatePackageIndex(t *testing.T) {
	t.Run("create new index entry", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		err := db.UpdatePackageIndex("editors/vim", "9.0.1", "test-uuid-1")
		if err != nil {
			t.Fatalf("UpdatePackageIndex() failed: %v", err)
		}

		// Note: We don't verify with LatestFor here because the build record
		// doesn't exist yet (that would be an orphaned record error).
		// Full workflow testing is done in TestPackageIndexWorkflow.
	})

	t.Run("update existing entry", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Create first index entry
		db.UpdatePackageIndex("editors/vim", "9.0.1", "uuid-1")

		// Update to newer UUID
		err := db.UpdatePackageIndex("editors/vim", "9.0.1", "uuid-2")
		if err != nil {
			t.Fatalf("UpdatePackageIndex() failed on update: %v", err)
		}
	})

	t.Run("multiple packages and versions", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		entries := map[string]map[string]string{
			"editors/vim": {"9.0.1": "uuid-vim-1", "9.0.2": "uuid-vim-2"},
			"lang/python": {"3.11.0": "uuid-py-1", "3.12.0": "uuid-py-2"},
			"www/nginx":   {"1.24.0": "uuid-nginx-1"},
		}

		for portDir, versions := range entries {
			for version, uuid := range versions {
				err := db.UpdatePackageIndex(portDir, version, uuid)
				if err != nil {
					t.Fatalf("UpdatePackageIndex(%s, %s) failed: %v", portDir, version, err)
				}
			}
		}
	})
}

func TestLatestFor(t *testing.T) {
	t.Run("retrieve latest build", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Save build record
		rec := createTestRecord("test-uuid-1", "editors/vim", "9.0.1", "success")
		db.SaveRecord(rec)

		// Update package index
		db.UpdatePackageIndex("editors/vim", "9.0.1", "test-uuid-1")

		// Retrieve latest
		latest, err := db.LatestFor("editors/vim", "9.0.1")
		if err != nil {
			t.Fatalf("LatestFor() failed: %v", err)
		}
		if latest == nil {
			t.Fatal("LatestFor() returned nil")
		}

		assertRecordEqual(t, rec, latest)
	})

	t.Run("no record exists", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		latest, err := db.LatestFor("editors/vim", "9.0.1")
		if err != nil {
			t.Fatalf("LatestFor() should not error when record not found: %v", err)
		}
		if latest != nil {
			t.Error("LatestFor() should return nil when no record exists")
		}
	})

	t.Run("orphaned record detection", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Create index entry pointing to non-existent build
		db.UpdatePackageIndex("editors/vim", "9.0.1", "nonexistent-uuid")

		_, err := db.LatestFor("editors/vim", "9.0.1")
		if err == nil {
			t.Error("LatestFor() should error on orphaned record")
		}
		// Just log the error type for debugging
		t.Logf("Orphaned record error type: %T, error: %v", err, err)
	})

	t.Run("multiple versions independent", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Save two versions of vim
		rec1 := createTestRecord("uuid-1", "editors/vim", "9.0.1", "success")
		rec2 := createTestRecord("uuid-2", "editors/vim", "9.0.2", "success")
		db.SaveRecord(rec1)
		db.SaveRecord(rec2)

		db.UpdatePackageIndex("editors/vim", "9.0.1", "uuid-1")
		db.UpdatePackageIndex("editors/vim", "9.0.2", "uuid-2")

		// Retrieve each version
		latest1, _ := db.LatestFor("editors/vim", "9.0.1")
		latest2, _ := db.LatestFor("editors/vim", "9.0.2")

		if latest1.Version != "9.0.1" {
			t.Errorf("Version 9.0.1 not correctly stored")
		}
		if latest2.Version != "9.0.2" {
			t.Errorf("Version 9.0.2 not correctly stored")
		}
	})
}

func TestPackageIndexWorkflow(t *testing.T) {
	t.Run("full workflow", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Step 1: Save build record
		rec1 := createTestRecord("uuid-1", "editors/vim", "9.0.1", "success")
		db.SaveRecord(rec1)
		db.UpdatePackageIndex("editors/vim", "9.0.1", "uuid-1")

		// Step 2: Save newer build
		rec2 := createTestRecord("uuid-2", "editors/vim", "9.0.1", "success")
		db.SaveRecord(rec2)
		db.UpdatePackageIndex("editors/vim", "9.0.1", "uuid-2")

		// Step 3: Verify latest returns newest
		latest, err := db.LatestFor("editors/vim", "9.0.1")
		if err != nil {
			t.Fatalf("LatestFor() failed: %v", err)
		}
		if latest.UUID != "uuid-2" {
			t.Errorf("LatestFor() returned wrong UUID: got %s, want uuid-2", latest.UUID)
		}
	})
}

// ==================== Group 4: CRC Operations Tests ====================

func TestUpdateCRC(t *testing.T) {
	t.Run("store CRC for new port", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		err := db.UpdateCRC("editors/vim", 0x12345678)
		if err != nil {
			t.Fatalf("UpdateCRC() failed: %v", err)
		}

		// Verify CRC was stored
		crc, found, err := db.GetCRC("editors/vim")
		if err != nil {
			t.Fatalf("GetCRC() failed: %v", err)
		}
		if !found {
			t.Error("CRC not found after update")
		}
		if crc != 0x12345678 {
			t.Errorf("CRC mismatch: got 0x%08x, want 0x12345678", crc)
		}
	})

	t.Run("update existing CRC", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Store initial CRC
		db.UpdateCRC("editors/vim", 0x11111111)

		// Update to new value
		err := db.UpdateCRC("editors/vim", 0x22222222)
		if err != nil {
			t.Fatalf("UpdateCRC() failed on update: %v", err)
		}

		// Verify new value
		crc, _, _ := db.GetCRC("editors/vim")
		if crc != 0x22222222 {
			t.Errorf("CRC not updated: got 0x%08x, want 0x22222222", crc)
		}
	})

	t.Run("multiple ports with different CRCs", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		ports := map[string]uint32{
			"editors/vim": 0xAAAAAAAA,
			"lang/python": 0xBBBBBBBB,
			"www/nginx":   0xCCCCCCCC,
		}

		// Store all CRCs
		for portDir, crc := range ports {
			if err := db.UpdateCRC(portDir, crc); err != nil {
				t.Fatalf("UpdateCRC(%s) failed: %v", portDir, err)
			}
		}

		// Verify all CRCs
		for portDir, expectedCRC := range ports {
			actualCRC, found, err := db.GetCRC(portDir)
			if err != nil {
				t.Fatalf("GetCRC(%s) failed: %v", portDir, err)
			}
			if !found {
				t.Errorf("CRC for %s not found", portDir)
			}
			if actualCRC != expectedCRC {
				t.Errorf("CRC for %s: got 0x%08x, want 0x%08x", portDir, actualCRC, expectedCRC)
			}
		}
	})
}

func TestGetCRC(t *testing.T) {
	t.Run("retrieve existing CRC", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		expectedCRC := uint32(0xDEADBEEF)
		db.UpdateCRC("editors/vim", expectedCRC)

		crc, found, err := db.GetCRC("editors/vim")
		if err != nil {
			t.Fatalf("GetCRC() failed: %v", err)
		}
		if !found {
			t.Error("CRC should be found")
		}
		if crc != expectedCRC {
			t.Errorf("CRC mismatch: got 0x%08x, want 0x%08x", crc, expectedCRC)
		}
	})

	t.Run("nonexistent port", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		crc, found, err := db.GetCRC("nonexistent/port")
		if err != nil {
			t.Fatalf("GetCRC() should not error for missing port: %v", err)
		}
		if found {
			t.Error("found should be false for nonexistent port")
		}
		if crc != 0 {
			t.Errorf("CRC should be 0 for nonexistent port, got 0x%08x", crc)
		}
	})

	t.Run("edge case CRC zero", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		db.UpdateCRC("editors/vim", 0)

		crc, found, err := db.GetCRC("editors/vim")
		if err != nil {
			t.Fatalf("GetCRC() failed: %v", err)
		}
		if !found {
			t.Error("CRC=0 should still be found")
		}
		if crc != 0 {
			t.Errorf("Expected CRC=0, got 0x%08x", crc)
		}
	})

	t.Run("edge case CRC max uint32", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		maxCRC := uint32(0xFFFFFFFF)
		db.UpdateCRC("editors/vim", maxCRC)

		crc, found, err := db.GetCRC("editors/vim")
		if err != nil {
			t.Fatalf("GetCRC() failed: %v", err)
		}
		if !found {
			t.Error("Max CRC should be found")
		}
		if crc != maxCRC {
			t.Errorf("Expected CRC=0xFFFFFFFF, got 0x%08x", crc)
		}
	})
}

func TestNeedsBuild(t *testing.T) {
	t.Run("no stored CRC needs build", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		needsBuild, err := db.NeedsBuild("editors/vim", 0x12345678)
		if err != nil {
			t.Fatalf("NeedsBuild() failed: %v", err)
		}
		if !needsBuild {
			t.Error("Port with no stored CRC should need build")
		}
	})

	t.Run("CRC match no build needed", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		crc := uint32(0xABCDEF12)
		db.UpdateCRC("editors/vim", crc)

		needsBuild, err := db.NeedsBuild("editors/vim", crc)
		if err != nil {
			t.Fatalf("NeedsBuild() failed: %v", err)
		}
		if needsBuild {
			t.Error("Port with matching CRC should not need build")
		}
	})

	t.Run("CRC changed needs build", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		db.UpdateCRC("editors/vim", 0x11111111)

		needsBuild, err := db.NeedsBuild("editors/vim", 0x22222222)
		if err != nil {
			t.Fatalf("NeedsBuild() failed: %v", err)
		}
		if !needsBuild {
			t.Error("Port with changed CRC should need build")
		}
	})

	t.Run("zero to non-zero needs build", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		db.UpdateCRC("editors/vim", 0)

		needsBuild, err := db.NeedsBuild("editors/vim", 0x12345678)
		if err != nil {
			t.Fatalf("NeedsBuild() failed: %v", err)
		}
		if !needsBuild {
			t.Error("CRC change from 0 to non-zero should need build")
		}
	})

	t.Run("non-zero to zero needs build", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		db.UpdateCRC("editors/vim", 0x12345678)

		needsBuild, err := db.NeedsBuild("editors/vim", 0)
		if err != nil {
			t.Fatalf("NeedsBuild() failed: %v", err)
		}
		if !needsBuild {
			t.Error("CRC change from non-zero to 0 should need build")
		}
	})
}

func TestCRCWorkflow(t *testing.T) {
	t.Run("incremental build workflow", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		portDir := "editors/vim"
		crc1 := uint32(0x11111111)

		// First build: no CRC stored, should need build
		needsBuild, err := db.NeedsBuild(portDir, crc1)
		if err != nil {
			t.Fatalf("NeedsBuild() failed: %v", err)
		}
		if !needsBuild {
			t.Error("First build should need build (no stored CRC)")
		}

		// After successful build: update CRC
		if err := db.UpdateCRC(portDir, crc1); err != nil {
			t.Fatalf("UpdateCRC() failed: %v", err)
		}

		// Second build with same CRC: should not need build
		needsBuild, err = db.NeedsBuild(portDir, crc1)
		if err != nil {
			t.Fatalf("NeedsBuild() failed: %v", err)
		}
		if needsBuild {
			t.Error("Second build with matching CRC should not need build")
		}

		// Port changes: new CRC, should need build
		crc2 := uint32(0x22222222)
		needsBuild, err = db.NeedsBuild(portDir, crc2)
		if err != nil {
			t.Fatalf("NeedsBuild() failed: %v", err)
		}
		if !needsBuild {
			t.Error("Build with changed CRC should need build")
		}

		// After rebuild: update to new CRC
		db.UpdateCRC(portDir, crc2)

		// Verify new CRC stored
		needsBuild, _ = db.NeedsBuild(portDir, crc2)
		if needsBuild {
			t.Error("After update, CRC should match")
		}
	})
}

func TestComputePortCRC(t *testing.T) {
	t.Run("compute CRC for real port", func(t *testing.T) {
		// Use testdata/ports/editors/vim
		portPath := filepath.Join("testdata", "ports", "editors", "vim")

		crc, err := ComputePortCRC(portPath)
		if err != nil {
			t.Fatalf("ComputePortCRC() failed: %v", err)
		}
		if crc == 0 {
			t.Error("CRC should not be zero for port with files")
		}
	})

	t.Run("idempotent same directory", func(t *testing.T) {
		portPath := filepath.Join("testdata", "ports", "editors", "vim")

		crc1, err := ComputePortCRC(portPath)
		if err != nil {
			t.Fatalf("First ComputePortCRC() failed: %v", err)
		}

		crc2, err := ComputePortCRC(portPath)
		if err != nil {
			t.Fatalf("Second ComputePortCRC() failed: %v", err)
		}

		if crc1 != crc2 {
			t.Errorf("CRC should be deterministic: got 0x%08x and 0x%08x", crc1, crc2)
		}
	})

	t.Run("file content change detected", func(t *testing.T) {
		// Create test port with initial content
		portDir := createTestPortDir(t, map[string]string{
			"Makefile": "PORTNAME=test\nVERSION=1.0\n",
			"distinfo": "SHA256=abc123\n",
		})

		crc1, _ := ComputePortCRC(portDir)

		// Modify file content
		makefilePath := filepath.Join(portDir, "Makefile")
		os.WriteFile(makefilePath, []byte("PORTNAME=test\nVERSION=2.0\n"), 0644)

		crc2, _ := ComputePortCRC(portDir)

		if crc1 == crc2 {
			t.Error("CRC should change when file content changes")
		}
	})

	t.Run("file rename detected", func(t *testing.T) {
		// Create test port
		portDir := createTestPortDir(t, map[string]string{
			"Makefile":  "content",
			"file1.txt": "data",
		})

		crc1, _ := ComputePortCRC(portDir)

		// Rename file (changes path, same content)
		oldPath := filepath.Join(portDir, "file1.txt")
		newPath := filepath.Join(portDir, "file2.txt")
		os.Rename(oldPath, newPath)

		crc2, _ := ComputePortCRC(portDir)

		if crc1 == crc2 {
			t.Error("CRC should change when file is renamed (path changes)")
		}
	})

	t.Run("skips work directory", func(t *testing.T) {
		portDir := createTestPortDir(t, map[string]string{
			"Makefile":        "PORTNAME=test\n",
			"work/temp.txt":   "should be ignored",
			"work/obj/file.o": "should be ignored",
		})

		crc1, _ := ComputePortCRC(portDir)

		// Modify work directory content
		workFile := filepath.Join(portDir, "work", "temp.txt")
		os.WriteFile(workFile, []byte("modified content"), 0644)

		crc2, _ := ComputePortCRC(portDir)

		if crc1 != crc2 {
			t.Error("CRC should not change when only work/ directory changes")
		}
	})

	t.Run("skips .git directory", func(t *testing.T) {
		portDir := createTestPortDir(t, map[string]string{
			"Makefile":    "PORTNAME=test\n",
			".git/config": "should be ignored",
			".git/HEAD":   "should be ignored",
		})

		crc1, _ := ComputePortCRC(portDir)

		// Modify .git content
		gitFile := filepath.Join(portDir, ".git", "config")
		os.WriteFile(gitFile, []byte("modified git config"), 0644)

		crc2, _ := ComputePortCRC(portDir)

		if crc1 != crc2 {
			t.Error("CRC should not change when only .git/ directory changes")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := ComputePortCRC("/nonexistent/port/directory")
		if err == nil {
			t.Error("ComputePortCRC() should fail for nonexistent directory")
		}
	})

	t.Run("different ports have different CRCs", func(t *testing.T) {
		vimPath := filepath.Join("testdata", "ports", "editors", "vim")
		pythonPath := filepath.Join("testdata", "ports", "lang", "python")

		vimCRC, _ := ComputePortCRC(vimPath)
		pythonCRC, _ := ComputePortCRC(pythonPath)

		if vimCRC == pythonCRC {
			t.Error("Different ports should have different CRCs")
		}
	})
}

// ==================== Group 5: Concurrent Access Tests ====================

func TestConcurrentAccess(t *testing.T) {
	t.Run("concurrent reads", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Save some test records
		for i := 0; i < 10; i++ {
			rec := createTestRecord(
				fmt.Sprintf("uuid-%d", i),
				"editors/vim",
				"9.0.1",
				"success",
			)
			db.SaveRecord(rec)
		}

		// Concurrent reads
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(id int) {
				uuid := fmt.Sprintf("uuid-%d", id)
				rec, err := db.GetRecord(uuid)
				if err != nil {
					t.Errorf("Concurrent GetRecord(%s) failed: %v", uuid, err)
				}
				if rec == nil {
					t.Errorf("Concurrent GetRecord(%s) returned nil", uuid)
				}
				done <- true
			}(i)
		}

		// Wait for all reads to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("concurrent writes different keys", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Concurrent writes to different ports (no contention)
		done := make(chan bool)
		for i := 0; i < 5; i++ {
			go func(id int) {
				portDir := fmt.Sprintf("category/port-%d", id)
				crc := uint32(0x10000000 + id)

				err := db.UpdateCRC(portDir, crc)
				if err != nil {
					t.Errorf("Concurrent UpdateCRC(%s) failed: %v", portDir, err)
				}
				done <- true
			}(i)
		}

		// Wait for all writes
		for i := 0; i < 5; i++ {
			<-done
		}

		// Verify all writes succeeded
		for i := 0; i < 5; i++ {
			portDir := fmt.Sprintf("category/port-%d", i)
			expectedCRC := uint32(0x10000000 + i)
			crc, found, _ := db.GetCRC(portDir)
			if !found {
				t.Errorf("CRC for %s not found after concurrent write", portDir)
			}
			if crc != expectedCRC {
				t.Errorf("CRC for %s: got 0x%08x, want 0x%08x", portDir, crc, expectedCRC)
			}
		}
	})

	t.Run("mixed read write workload", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer cleanupTestDB(t, db)

		// Pre-populate some data
		for i := 0; i < 5; i++ {
			rec := createTestRecord(
				fmt.Sprintf("uuid-%d", i),
				fmt.Sprintf("port-%d", i),
				"1.0",
				"success",
			)
			db.SaveRecord(rec)
		}

		done := make(chan bool)

		// Readers
		for i := 0; i < 5; i++ {
			go func(id int) {
				uuid := fmt.Sprintf("uuid-%d", id)
				for j := 0; j < 10; j++ {
					db.GetRecord(uuid)
				}
				done <- true
			}(i)
		}

		// Writers (new records)
		for i := 5; i < 10; i++ {
			go func(id int) {
				rec := createTestRecord(
					fmt.Sprintf("uuid-%d", id),
					fmt.Sprintf("port-%d", id),
					"1.0",
					"success",
				)
				db.SaveRecord(rec)
				done <- true
			}(i)
		}

		// Wait for all operations
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify data integrity - all 10 records should exist
		for i := 0; i < 10; i++ {
			uuid := fmt.Sprintf("uuid-%d", i)
			rec, err := db.GetRecord(uuid)
			if err != nil {
				t.Errorf("Record %s not found after concurrent operations", uuid)
			}
			if rec == nil {
				t.Errorf("Record %s is nil after concurrent operations", uuid)
			}
		}
	})
}

// TestUpdateRunSnapshot tests updating live snapshot data
func TestUpdateRunSnapshot(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	t.Run("update_snapshot_success", func(t *testing.T) {
		// Create a run first
		runID := "run-snapshot-1"
		if err := db.StartRun(runID, time.Now()); err != nil {
			t.Fatalf("StartRun failed: %v", err)
		}

		// Update snapshot
		snapshotJSON := `{"load":3.24,"swap_pct":2,"active":4}`
		if err := db.UpdateRunSnapshot(runID, snapshotJSON); err != nil {
			t.Fatalf("UpdateRunSnapshot failed: %v", err)
		}

		// Verify snapshot was stored
		snapshot, err := db.GetRunSnapshot(runID)
		if err != nil {
			t.Fatalf("GetRunSnapshot failed: %v", err)
		}
		if snapshot != snapshotJSON {
			t.Errorf("GetRunSnapshot() = %q, want %q", snapshot, snapshotJSON)
		}
	})

	t.Run("update_snapshot_multiple_times", func(t *testing.T) {
		runID := "run-snapshot-2"
		if err := db.StartRun(runID, time.Now()); err != nil {
			t.Fatalf("StartRun failed: %v", err)
		}

		// Update snapshot multiple times (simulating 1 Hz updates)
		snapshots := []string{
			`{"active":0,"built":0}`,
			`{"active":2,"built":5}`,
			`{"active":4,"built":12}`,
		}

		for i, snap := range snapshots {
			if err := db.UpdateRunSnapshot(runID, snap); err != nil {
				t.Fatalf("UpdateRunSnapshot iteration %d failed: %v", i, err)
			}
		}

		// Should have latest snapshot only
		snapshot, err := db.GetRunSnapshot(runID)
		if err != nil {
			t.Fatalf("GetRunSnapshot failed: %v", err)
		}
		if snapshot != snapshots[len(snapshots)-1] {
			t.Errorf("GetRunSnapshot() = %q, want %q", snapshot, snapshots[len(snapshots)-1])
		}
	})

	t.Run("update_snapshot_empty_runid", func(t *testing.T) {
		err := db.UpdateRunSnapshot("", "snapshot")
		if err == nil {
			t.Error("UpdateRunSnapshot with empty runID should fail")
		}

		// Check error type
		if _, ok := err.(*ValidationError); !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})

	t.Run("update_snapshot_nonexistent_run", func(t *testing.T) {
		err := db.UpdateRunSnapshot("nonexistent-run", "snapshot")
		if err == nil {
			t.Error("UpdateRunSnapshot with nonexistent run should fail")
		}

		// Check error type
		if _, ok := err.(*RecordError); !ok {
			t.Errorf("Expected RecordError, got %T", err)
		}
	})
}

// TestGetRunSnapshot tests retrieving snapshot data
func TestGetRunSnapshot(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	t.Run("get_snapshot_exists", func(t *testing.T) {
		runID := "run-get-1"
		if err := db.StartRun(runID, time.Now()); err != nil {
			t.Fatalf("StartRun failed: %v", err)
		}

		snapshotJSON := `{"load":1.5}`
		if err := db.UpdateRunSnapshot(runID, snapshotJSON); err != nil {
			t.Fatalf("UpdateRunSnapshot failed: %v", err)
		}

		snapshot, err := db.GetRunSnapshot(runID)
		if err != nil {
			t.Fatalf("GetRunSnapshot failed: %v", err)
		}
		if snapshot != snapshotJSON {
			t.Errorf("GetRunSnapshot() = %q, want %q", snapshot, snapshotJSON)
		}
	})

	t.Run("get_snapshot_no_snapshot_yet", func(t *testing.T) {
		runID := "run-get-2"
		if err := db.StartRun(runID, time.Now()); err != nil {
			t.Fatalf("StartRun failed: %v", err)
		}

		// No snapshot updated yet
		snapshot, err := db.GetRunSnapshot(runID)
		if err != nil {
			t.Fatalf("GetRunSnapshot failed: %v", err)
		}
		if snapshot != "" {
			t.Errorf("GetRunSnapshot() = %q, want empty string", snapshot)
		}
	})

	t.Run("get_snapshot_empty_runid", func(t *testing.T) {
		_, err := db.GetRunSnapshot("")
		if err == nil {
			t.Error("GetRunSnapshot with empty runID should fail")
		}
	})

	t.Run("get_snapshot_nonexistent_run", func(t *testing.T) {
		_, err := db.GetRunSnapshot("nonexistent-run")
		if err == nil {
			t.Error("GetRunSnapshot with nonexistent run should fail")
		}
	})
}

// TestActiveRunSnapshot tests retrieving active run snapshot
func TestActiveRunSnapshot(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	t.Run("active_run_with_snapshot", func(t *testing.T) {
		runID := "active-run-1"
		if err := db.StartRun(runID, time.Now()); err != nil {
			t.Fatalf("StartRun failed: %v", err)
		}

		snapshotJSON := `{"active":2,"built":10}`
		if err := db.UpdateRunSnapshot(runID, snapshotJSON); err != nil {
			t.Fatalf("UpdateRunSnapshot failed: %v", err)
		}

		gotRunID, snapshot, err := db.ActiveRunSnapshot()
		if err != nil {
			t.Fatalf("ActiveRunSnapshot failed: %v", err)
		}
		if gotRunID != runID {
			t.Errorf("ActiveRunSnapshot runID = %q, want %q", gotRunID, runID)
		}
		if snapshot != snapshotJSON {
			t.Errorf("ActiveRunSnapshot snapshot = %q, want %q", snapshot, snapshotJSON)
		}
	})

	t.Run("active_run_no_snapshot_yet", func(t *testing.T) {
		// Clean previous run
		db.Close()
		db, _ = setupTestDB(t)
		defer db.Close()

		runID := "active-run-2"
		if err := db.StartRun(runID, time.Now()); err != nil {
			t.Fatalf("StartRun failed: %v", err)
		}

		gotRunID, snapshot, err := db.ActiveRunSnapshot()
		if err != nil {
			t.Fatalf("ActiveRunSnapshot failed: %v", err)
		}
		if gotRunID != runID {
			t.Errorf("ActiveRunSnapshot runID = %q, want %q", gotRunID, runID)
		}
		if snapshot != "" {
			t.Errorf("ActiveRunSnapshot snapshot = %q, want empty", snapshot)
		}
	})

	t.Run("no_active_run", func(t *testing.T) {
		// Clean previous runs
		db.Close()
		db, _ = setupTestDB(t)
		defer db.Close()

		// Finish all runs
		runID := "finished-run"
		if err := db.StartRun(runID, time.Now()); err != nil {
			t.Fatalf("StartRun failed: %v", err)
		}
		if err := db.FinishRun(runID, RunStats{}, time.Now(), false); err != nil {
			t.Fatalf("FinishRun failed: %v", err)
		}

		gotRunID, snapshot, err := db.ActiveRunSnapshot()
		if err != nil {
			t.Fatalf("ActiveRunSnapshot failed: %v", err)
		}
		if gotRunID != "" {
			t.Errorf("ActiveRunSnapshot with no active run returned runID = %q, want empty", gotRunID)
		}
		if snapshot != "" {
			t.Errorf("ActiveRunSnapshot with no active run returned snapshot = %q, want empty", snapshot)
		}
	})
}
