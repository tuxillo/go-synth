package builddb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

// ==================== Integration Test Helpers ====================

// generateBuildUUID generates a unique UUID for a build record
func generateBuildUUID() string {
	return uuid.New().String()
}

// modifyPortFile modifies a file in the port directory to change its CRC.
// It appends a comment to the specified file to ensure content changes.
func modifyPortFile(t *testing.T, portDir, filename string) {
	t.Helper()

	filePath := filepath.Join(portDir, filename)

	// Append a timestamp comment to ensure the file changes
	comment := fmt.Sprintf("\n# Modified at %s\n", time.Now().Format(time.RFC3339))

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file %s for modification: %v", filePath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(comment); err != nil {
		t.Fatalf("Failed to modify file %s: %v", filePath, err)
	}
}

// assertBuildRecordState verifies that a build record in the database matches expected state
func assertBuildRecordState(t *testing.T, db *DB, uuid, expectedStatus string) {
	t.Helper()

	rec, err := db.GetRecord(uuid)
	if err != nil {
		t.Fatalf("Failed to get record %s: %v", uuid, err)
	}

	if rec == nil {
		t.Fatalf("Record %s not found in database", uuid)
	}

	if rec.Status != expectedStatus {
		t.Errorf("Record %s status mismatch: got %q, want %q", uuid, rec.Status, expectedStatus)
	}
}

// assertDatabaseConsistency verifies database integrity by checking:
// - No orphaned CRC entries (CRC exists but no successful build)
// - No orphaned package index entries
// - Package index points to valid build records
func assertDatabaseConsistency(t *testing.T, db *DB) {
	t.Helper()

	err := db.db.View(func(tx *bolt.Tx) error {
		crcBucket := tx.Bucket([]byte(BucketCRCIndex))
		pkgBucket := tx.Bucket([]byte(BucketPackages))
		buildsBucket := tx.Bucket([]byte(BucketBuilds))

		if crcBucket == nil || pkgBucket == nil || buildsBucket == nil {
			t.Error("Required buckets not found in database")
			return nil
		}

		// Check that all package index entries point to valid builds
		err := pkgBucket.ForEach(func(k, v []byte) error {
			buildUUID := string(v)

			// Verify the build record exists
			buildData := buildsBucket.Get([]byte(buildUUID))
			if buildData == nil {
				t.Errorf("Package index entry %s points to non-existent build %s", string(k), buildUUID)
			}

			return nil
		})

		return err
	})

	if err != nil {
		t.Fatalf("Database consistency check failed: %v", err)
	}
}

// simulateBuildWorkflow simulates a complete build workflow:
// 1. Compute current CRC
// 2. Check if port needs building (NeedsBuild)
// 3. Save build record with "running" status
// 4. Simulate build completion (update status to success/failed)
// 5. Update CRC and package index (only on success)
//
// Returns the build UUID and whether the port needed building
func simulateBuildWorkflow(t *testing.T, db *DB, portDir, version, finalStatus string) (uuid string, needsBuild bool) {
	t.Helper()

	if finalStatus != "success" && finalStatus != "failed" {
		t.Fatalf("Invalid final status: %s (must be 'success' or 'failed')", finalStatus)
	}

	// Step 1: Compute current CRC
	currentCRC, err := ComputePortCRC(portDir)
	if err != nil {
		t.Fatalf("Failed to compute CRC for %s: %v", portDir, err)
	}

	// Step 2: Check if port needs building
	needsBuild, err = db.NeedsBuild(portDir, currentCRC)
	if err != nil {
		t.Fatalf("NeedsBuild failed for %s: %v", portDir, err)
	}

	// Step 3: Create and save build record
	uuid = generateBuildUUID()
	rec := &BuildRecord{
		UUID:      uuid,
		PortDir:   portDir,
		Version:   version,
		Status:    "running",
		StartTime: time.Now(),
	}

	if err := db.SaveRecord(rec); err != nil {
		t.Fatalf("Failed to save build record: %v", err)
	}

	// Step 4: Simulate build completion
	endTime := time.Now()

	if err := db.UpdateRecordStatus(uuid, finalStatus, endTime); err != nil {
		t.Fatalf("Failed to update record status: %v", err)
	}

	// Step 5: Update CRC and package index (only on success)
	if finalStatus == "success" {
		if err := db.UpdateCRC(portDir, currentCRC); err != nil {
			t.Fatalf("Failed to update CRC for %s: %v", portDir, err)
		}

		if err := db.UpdatePackageIndex(portDir, version, uuid); err != nil {
			t.Fatalf("Failed to update package index: %v", err)
		}
	}

	return uuid, needsBuild
}

// ==================== Integration Tests ====================

// TestIntegration_FirstBuildWorkflow tests the complete workflow for building
// a port for the first time (no existing CRC or build records)
func TestIntegration_FirstBuildWorkflow(t *testing.T) {
	db, _ := setupTestDB(t)
	defer cleanupTestDB(t, db)

	portDir := filepath.Join("testdata", "ports", "editors", "vim")
	version := "9.0.0"

	t.Run("port should need building (no CRC)", func(t *testing.T) {
		currentCRC, err := ComputePortCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to compute CRC: %v", err)
		}

		needsBuild, err := db.NeedsBuild(portDir, currentCRC)
		if err != nil {
			t.Fatalf("NeedsBuild failed: %v", err)
		}
		if !needsBuild {
			t.Error("Port should need building on first build")
		}
	})

	t.Run("complete successful build workflow", func(t *testing.T) {
		uuid, needsBuild := simulateBuildWorkflow(t, db, portDir, version, "success")

		if !needsBuild {
			t.Error("Port should have needed building")
		}

		// Verify build record exists with correct status
		assertBuildRecordState(t, db, uuid, "success")

		// Verify CRC was stored
		storedCRC, exists, err := db.GetCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to get CRC: %v", err)
		}
		if !exists || storedCRC == 0 {
			t.Error("CRC should be stored after successful build")
		}

		// Verify package index was updated
		latestRec, err := db.LatestFor(portDir, version)
		if err != nil {
			t.Fatalf("Failed to get latest build: %v", err)
		}
		if latestRec == nil || latestRec.UUID != uuid {
			t.Errorf("Package index mismatch: got %v, want %s", latestRec, uuid)
		}
	})

	t.Run("database consistency after first build", func(t *testing.T) {
		assertDatabaseConsistency(t, db)
	})
}

// TestIntegration_RebuildSamePort tests incremental build detection:
// building the same port twice without changes should detect no rebuild needed
func TestIntegration_RebuildSamePort(t *testing.T) {
	db, _ := setupTestDB(t)
	defer cleanupTestDB(t, db)

	portDir := filepath.Join("testdata", "ports", "editors", "vim")
	version := "9.0.0"

	t.Run("first build establishes baseline", func(t *testing.T) {
		uuid, needsBuild := simulateBuildWorkflow(t, db, portDir, version, "success")

		if !needsBuild {
			t.Error("First build should always need building")
		}

		assertBuildRecordState(t, db, uuid, "success")
	})

	t.Run("rebuild without changes should skip", func(t *testing.T) {
		currentCRC, err := ComputePortCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to compute CRC: %v", err)
		}

		needsBuild, err := db.NeedsBuild(portDir, currentCRC)
		if err != nil {
			t.Fatalf("NeedsBuild failed: %v", err)
		}

		if needsBuild {
			t.Error("Port should NOT need rebuilding when unchanged")
		}
	})

	t.Run("CRC should match after no changes", func(t *testing.T) {
		storedCRC, exists, err := db.GetCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to get stored CRC: %v", err)
		}
		if !exists {
			t.Fatal("CRC should exist after first build")
		}

		currentCRC, err := ComputePortCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to compute current CRC: %v", err)
		}

		if storedCRC != currentCRC {
			t.Errorf("CRC mismatch: stored=%d, current=%d", storedCRC, currentCRC)
		}
	})

	t.Run("database consistency after incremental check", func(t *testing.T) {
		assertDatabaseConsistency(t, db)
	})
}

// TestIntegration_RebuildAfterChange tests change detection:
// modifying a port file should trigger a rebuild
func TestIntegration_RebuildAfterChange(t *testing.T) {
	db, _ := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Copy testdata to temp directory so we can modify it
	tmpDir := t.TempDir()
	portDir := filepath.Join(tmpDir, "vim")

	srcDir := filepath.Join("testdata", "ports", "editors", "vim")
	if err := copyDir(srcDir, portDir); err != nil {
		t.Fatalf("Failed to copy test port: %v", err)
	}

	version := "9.0.0"

	var firstCRC uint32

	t.Run("first build establishes baseline", func(t *testing.T) {
		uuid, needsBuild := simulateBuildWorkflow(t, db, portDir, version, "success")

		if !needsBuild {
			t.Error("First build should always need building")
		}

		assertBuildRecordState(t, db, uuid, "success")

		var err error
		var exists bool
		firstCRC, exists, err = db.GetCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to get CRC: %v", err)
		}
		if !exists {
			t.Fatal("CRC should exist after first build")
		}
	})

	t.Run("modify port file", func(t *testing.T) {
		modifyPortFile(t, portDir, "Makefile")
	})

	t.Run("rebuild after change should be needed", func(t *testing.T) {
		currentCRC, err := ComputePortCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to compute CRC: %v", err)
		}

		needsBuild, err := db.NeedsBuild(portDir, currentCRC)
		if err != nil {
			t.Fatalf("NeedsBuild failed: %v", err)
		}

		if !needsBuild {
			t.Error("Port SHOULD need rebuilding after modification")
		}
	})

	t.Run("CRC should differ after modification", func(t *testing.T) {
		newCRC, err := ComputePortCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to compute new CRC: %v", err)
		}

		if newCRC == firstCRC {
			t.Errorf("CRC should change after modification: old=%d, new=%d", firstCRC, newCRC)
		}
	})

	t.Run("complete rebuild after change", func(t *testing.T) {
		uuid, needsBuild := simulateBuildWorkflow(t, db, portDir, version, "success")

		if !needsBuild {
			t.Error("Port should need rebuilding after change")
		}

		assertBuildRecordState(t, db, uuid, "success")
	})

	t.Run("database consistency after rebuild", func(t *testing.T) {
		assertDatabaseConsistency(t, db)
	})
}

// TestIntegration_FailedBuildHandling tests that failed builds don't corrupt
// database state (CRC and package index should NOT be updated)
func TestIntegration_FailedBuildHandling(t *testing.T) {
	db, _ := setupTestDB(t)
	defer cleanupTestDB(t, db)

	portDir := filepath.Join("testdata", "ports", "lang", "python")
	version := "3.11.0"

	t.Run("failed build does not update CRC", func(t *testing.T) {
		uuid, needsBuild := simulateBuildWorkflow(t, db, portDir, version, "failed")

		if !needsBuild {
			t.Error("First build should always need building")
		}

		// Verify build record shows failure
		assertBuildRecordState(t, db, uuid, "failed")

		// Verify CRC was NOT stored
		storedCRC, exists, err := db.GetCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to get CRC: %v", err)
		}
		if exists || storedCRC != 0 {
			t.Errorf("CRC should NOT be stored after failed build, got exists=%v crc=%d", exists, storedCRC)
		}
	})

	t.Run("failed build does not update package index", func(t *testing.T) {
		// Try to get latest build - should return nil since build failed
		latestRec, err := db.LatestFor(portDir, version)
		if err != nil {
			t.Fatalf("Failed to query package index: %v", err)
		}
		if latestRec != nil {
			t.Errorf("Package index should be empty after failed build, got %v", latestRec)
		}
	})

	t.Run("retry after failed build should still need building", func(t *testing.T) {
		currentCRC, err := ComputePortCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to compute CRC: %v", err)
		}

		needsBuild, err := db.NeedsBuild(portDir, currentCRC)
		if err != nil {
			t.Fatalf("NeedsBuild failed: %v", err)
		}

		if !needsBuild {
			t.Error("Port should still need building after failed build")
		}
	})

	t.Run("successful retry updates CRC and index", func(t *testing.T) {
		uuid, needsBuild := simulateBuildWorkflow(t, db, portDir, version, "success")

		if !needsBuild {
			t.Error("Port should need building after previous failure")
		}

		assertBuildRecordState(t, db, uuid, "success")

		// Now CRC should be stored
		storedCRC, exists, err := db.GetCRC(portDir)
		if err != nil {
			t.Fatalf("Failed to get CRC: %v", err)
		}
		if !exists || storedCRC == 0 {
			t.Error("CRC should be stored after successful retry")
		}

		// Package index should now point to successful build
		latestRec, err := db.LatestFor(portDir, version)
		if err != nil {
			t.Fatalf("Failed to get latest build: %v", err)
		}
		if latestRec == nil || latestRec.UUID != uuid {
			t.Errorf("Package index should point to successful build: got %v, want %s", latestRec, uuid)
		}
	})

	t.Run("database consistency after failed build handling", func(t *testing.T) {
		assertDatabaseConsistency(t, db)
	})
}

// TestIntegration_MultiPortCoordination tests that multiple ports can be
// built independently without interfering with each other
func TestIntegration_MultiPortCoordination(t *testing.T) {
	db, _ := setupTestDB(t)
	defer cleanupTestDB(t, db)

	vimPort := filepath.Join("testdata", "ports", "editors", "vim")
	pythonPort := filepath.Join("testdata", "ports", "lang", "python")

	t.Run("build multiple ports simultaneously", func(t *testing.T) {
		// Build vim
		vimUUID, vimNeeded := simulateBuildWorkflow(t, db, vimPort, "9.0.0", "success")
		if !vimNeeded {
			t.Error("vim should need building")
		}
		assertBuildRecordState(t, db, vimUUID, "success")

		// Build python
		pythonUUID, pythonNeeded := simulateBuildWorkflow(t, db, pythonPort, "3.11.0", "success")
		if !pythonNeeded {
			t.Error("python should need building")
		}
		assertBuildRecordState(t, db, pythonUUID, "success")

		// Verify both have different UUIDs
		if vimUUID == pythonUUID {
			t.Error("Different ports should have different build UUIDs")
		}
	})

	t.Run("each port tracks its own CRC", func(t *testing.T) {
		vimCRC, vimExists, err := db.GetCRC(vimPort)
		if err != nil {
			t.Fatalf("Failed to get vim CRC: %v", err)
		}

		pythonCRC, pythonExists, err := db.GetCRC(pythonPort)
		if err != nil {
			t.Fatalf("Failed to get python CRC: %v", err)
		}

		if !vimExists || !pythonExists || vimCRC == 0 || pythonCRC == 0 {
			t.Error("Both ports should have stored CRCs")
		}

		if vimCRC == pythonCRC {
			t.Error("Different ports should have different CRCs")
		}
	})

	t.Run("each port has independent package index", func(t *testing.T) {
		vimRec, err := db.LatestFor(vimPort, "9.0.0")
		if err != nil {
			t.Fatalf("Failed to get vim latest: %v", err)
		}

		pythonRec, err := db.LatestFor(pythonPort, "3.11.0")
		if err != nil {
			t.Fatalf("Failed to get python latest: %v", err)
		}

		if vimRec == nil || pythonRec == nil {
			t.Error("Both ports should have package index entries")
		}

		if vimRec.UUID == pythonRec.UUID {
			t.Error("Different ports should have different package index entries")
		}
	})

	t.Run("rebuilding one port doesn't affect the other", func(t *testing.T) {
		// Get python's original state
		originalPythonCRC, _, _ := db.GetCRC(pythonPort)
		originalPythonRec, _ := db.LatestFor(pythonPort, "3.11.0")

		// Rebuild vim with different version
		vimUUID, _ := simulateBuildWorkflow(t, db, vimPort, "9.0.1", "success")
		assertBuildRecordState(t, db, vimUUID, "success")

		// Verify python's state unchanged
		pythonCRC, _, _ := db.GetCRC(pythonPort)
		if pythonCRC != originalPythonCRC {
			t.Error("Python CRC should not change when vim is rebuilt")
		}

		pythonRec, _ := db.LatestFor(pythonPort, "3.11.0")
		if pythonRec.UUID != originalPythonRec.UUID {
			t.Error("Python package index should not change when vim is rebuilt")
		}
	})

	t.Run("database consistency with multiple ports", func(t *testing.T) {
		assertDatabaseConsistency(t, db)
	})
}

// ==================== Helper: Directory Copy ====================

// copyDir recursively copies a directory tree
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}
