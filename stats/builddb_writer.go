package stats

import (
	"encoding/json"
	"log"
)

// BuildDBWriter implements StatsConsumer to persist live stats to BuildDB.
// It is the primary stats consumer - BuildDB is the canonical storage backend
// for live build statistics (not monitor.dat files).
//
// The writer updates the RunRecord.LiveSnapshot field with JSON-encoded TopInfo
// every time OnStatsUpdate() is called (typically 1 Hz during builds).
//
// Database write failures are logged but do not interrupt builds - stats
// updates are best-effort to avoid blocking build workers.
type BuildDBWriter struct {
	db    BuildDB
	runID string
}

// BuildDB interface defines the minimal BuildDB operations needed by BuildDBWriter.
// This allows mocking in tests without importing the full builddb package.
type BuildDB interface {
	UpdateRunSnapshot(runID string, snapshot string) error
}

// NewBuildDBWriter creates a BuildDB stats consumer for the given run.
// The runID must match an active build run in the database.
func NewBuildDBWriter(db BuildDB, runID string) *BuildDBWriter {
	return &BuildDBWriter{
		db:    db,
		runID: runID,
	}
}

// OnStatsUpdate persists the current stats snapshot to BuildDB.
// Called by StatsCollector at 1 Hz during builds.
//
// This is a best-effort operation - database errors are logged as warnings
// but do not fail the build or block the stats collector.
func (w *BuildDBWriter) OnStatsUpdate(info TopInfo) {
	// Marshal TopInfo to JSON
	data, err := json.Marshal(info)
	if err != nil {
		// Should never happen with a valid TopInfo struct
		log.Printf("Warning: Failed to marshal stats snapshot: %v", err)
		return
	}

	// Update BuildDB (best-effort)
	if err := w.db.UpdateRunSnapshot(w.runID, string(data)); err != nil {
		// Log warning but don't fail the build
		// Stats update is non-critical compared to actual package builds
		log.Printf("Warning: Failed to update BuildDB snapshot for run %s: %v", w.runID, err)
	}
}
