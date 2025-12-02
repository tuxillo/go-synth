package stats

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// MockBuildDB implements BuildDB interface for testing
type MockBuildDB struct {
	snapshots map[string]string // runID -> snapshot JSON
	updateErr error             // Simulate update errors
}

func NewMockBuildDB() *MockBuildDB {
	return &MockBuildDB{
		snapshots: make(map[string]string),
	}
}

func (m *MockBuildDB) UpdateRunSnapshot(runID string, snapshot string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.snapshots[runID] = snapshot
	return nil
}

func (m *MockBuildDB) GetSnapshot(runID string) string {
	return m.snapshots[runID]
}

// TestBuildDBWriter_OnStatsUpdate tests basic stats persistence
func TestBuildDBWriter_OnStatsUpdate(t *testing.T) {
	mockDB := NewMockBuildDB()
	runID := "test-run-1"
	writer := NewBuildDBWriter(mockDB, runID)

	// Create test TopInfo
	info := TopInfo{
		ActiveWorkers: 4,
		MaxWorkers:    8,
		DynMaxWorkers: 8,
		Load:          3.24,
		SwapPct:       2,
		Rate:          24.3,
		Impulse:       3.0,
		Elapsed:       15 * time.Minute,
		Queued:        142,
		Built:         38,
		Failed:        2,
		Ignored:       0,
		Skipped:       5,
	}

	// Update stats
	writer.OnStatsUpdate(info)

	// Verify snapshot was stored
	snapshotJSON := mockDB.GetSnapshot(runID)
	if snapshotJSON == "" {
		t.Fatal("BuildDBWriter did not store snapshot")
	}

	// Verify JSON can be unmarshaled back to TopInfo
	var storedInfo TopInfo
	if err := json.Unmarshal([]byte(snapshotJSON), &storedInfo); err != nil {
		t.Fatalf("Stored snapshot is not valid JSON: %v", err)
	}

	// Verify key fields
	if storedInfo.ActiveWorkers != info.ActiveWorkers {
		t.Errorf("ActiveWorkers = %d, want %d", storedInfo.ActiveWorkers, info.ActiveWorkers)
	}
	if storedInfo.Built != info.Built {
		t.Errorf("Built = %d, want %d", storedInfo.Built, info.Built)
	}
	if storedInfo.Load != info.Load {
		t.Errorf("Load = %f, want %f", storedInfo.Load, info.Load)
	}
}

// TestBuildDBWriter_MultipleUpdates tests overwriting snapshots
func TestBuildDBWriter_MultipleUpdates(t *testing.T) {
	mockDB := NewMockBuildDB()
	runID := "test-run-2"
	writer := NewBuildDBWriter(mockDB, runID)

	// Simulate multiple 1 Hz updates
	updates := []TopInfo{
		{ActiveWorkers: 0, Built: 0, Failed: 0},
		{ActiveWorkers: 2, Built: 5, Failed: 0},
		{ActiveWorkers: 4, Built: 12, Failed: 1},
		{ActiveWorkers: 3, Built: 20, Failed: 1},
	}

	for _, info := range updates {
		writer.OnStatsUpdate(info)
	}

	// Should have latest snapshot only (in-place update)
	snapshotJSON := mockDB.GetSnapshot(runID)
	var storedInfo TopInfo
	if err := json.Unmarshal([]byte(snapshotJSON), &storedInfo); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	// Verify we have the LAST update
	lastUpdate := updates[len(updates)-1]
	if storedInfo.ActiveWorkers != lastUpdate.ActiveWorkers {
		t.Errorf("ActiveWorkers = %d, want %d (last update)", storedInfo.ActiveWorkers, lastUpdate.ActiveWorkers)
	}
	if storedInfo.Built != lastUpdate.Built {
		t.Errorf("Built = %d, want %d (last update)", storedInfo.Built, lastUpdate.Built)
	}
}

// TestBuildDBWriter_ErrorHandling tests graceful degradation on DB errors
func TestBuildDBWriter_ErrorHandling(t *testing.T) {
	mockDB := NewMockBuildDB()
	runID := "test-run-3"

	// Simulate database error
	mockDB.updateErr = fmt.Errorf("database connection lost")

	writer := NewBuildDBWriter(mockDB, runID)

	info := TopInfo{
		Built: 10,
	}

	// Should not panic despite DB error
	// (Error is logged but doesn't fail build)
	writer.OnStatsUpdate(info)

	// Verify no snapshot was stored due to error
	if mockDB.GetSnapshot(runID) != "" {
		t.Error("Snapshot was stored despite database error")
	}
}

// TestBuildDBWriter_EmptyTopInfo tests handling of zero-value TopInfo
func TestBuildDBWriter_EmptyTopInfo(t *testing.T) {
	mockDB := NewMockBuildDB()
	runID := "test-run-4"
	writer := NewBuildDBWriter(mockDB, runID)

	// Empty TopInfo (all zero values)
	var info TopInfo

	writer.OnStatsUpdate(info)

	// Should still store valid JSON (even if all zeros)
	snapshotJSON := mockDB.GetSnapshot(runID)
	if snapshotJSON == "" {
		t.Fatal("Empty TopInfo was not stored")
	}

	var storedInfo TopInfo
	if err := json.Unmarshal([]byte(snapshotJSON), &storedInfo); err != nil {
		t.Fatalf("Failed to unmarshal empty TopInfo: %v", err)
	}
}

// TestBuildDBWriter_ConcurrentUpdates tests thread safety
func TestBuildDBWriter_ConcurrentUpdates(t *testing.T) {
	mockDB := NewMockBuildDB()
	runID := "test-run-5"
	writer := NewBuildDBWriter(mockDB, runID)

	// Simulate concurrent updates from multiple goroutines
	// (BuildDBWriter itself doesn't need locks - BuildDB handles concurrency)
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(n int) {
			info := TopInfo{
				Built: n,
			}
			writer.OnStatsUpdate(info)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have *some* valid snapshot (race condition on which update wins)
	snapshotJSON := mockDB.GetSnapshot(runID)
	if snapshotJSON == "" {
		t.Fatal("No snapshot stored after concurrent updates")
	}

	var storedInfo TopInfo
	if err := json.Unmarshal([]byte(snapshotJSON), &storedInfo); err != nil {
		t.Fatalf("Failed to unmarshal concurrent snapshot: %v", err)
	}

	// Built should be 0-9 (one of the concurrent updates)
	if storedInfo.Built < 0 || storedInfo.Built > 9 {
		t.Errorf("Built = %d, expected 0-9 (one of the updates)", storedInfo.Built)
	}
}
