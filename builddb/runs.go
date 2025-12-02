package builddb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	RunStatusRunning = "running"
	RunStatusSuccess = "success"
	RunStatusFailed  = "failed"
	RunStatusSkipped = "skipped"
	RunStatusIgnored = "ignored"
)

// RunStats aggregates per-run port outcomes.
type RunStats struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Ignored int `json:"ignored"`
}

// RunRecord captures metadata for a go-synth build invocation.
type RunRecord struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Aborted   bool      `json:"aborted"`
	Stats     RunStats  `json:"stats"`
}

// RunPackageRecord represents a port build that ran within a build run.
type RunPackageRecord struct {
	PortDir   string    `json:"portdir"`
	Version   string    `json:"version"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	WorkerID  int       `json:"worker_id"`
	LastPhase string    `json:"last_phase"`
}

// StartRun writes a new run entry with the provided run ID and start time.
func (db *DB) StartRun(runID string, startTime time.Time) error {
	if runID == "" {
		return &ValidationError{Field: "runID", Err: ErrEmptyUUID}
	}

	rec := RunRecord{StartTime: startTime, Stats: RunStats{}}
	return db.saveRunRecord(runID, &rec)
}

// FinishRun updates an existing run with stats, end time, and abortion flag.
func (db *DB) FinishRun(runID string, stats RunStats, endTime time.Time, aborted bool) error {
	if runID == "" {
		return &ValidationError{Field: "runID", Err: ErrEmptyUUID}
	}

	return db.updateRunRecord(runID, func(rec *RunRecord) {
		rec.EndTime = endTime
		rec.Aborted = aborted
		rec.Stats = stats
	})
}

// GetRun fetches a run record by its ID.
func (db *DB) GetRun(runID string) (*RunRecord, error) {
	if runID == "" {
		return nil, &ValidationError{Field: "runID", Err: ErrEmptyUUID}
	}

	var rec RunRecord
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBuildRuns))
		if bucket == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketBuildRuns, Err: ErrBucketNotFound}
		}

		data := bucket.Get([]byte(runID))
		if data == nil {
			return &RecordError{Op: "get run", UUID: runID, Err: ErrRecordNotFound}
		}

		return json.Unmarshal(data, &rec)
	})
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// ActiveRun returns the first run that has no end time (if any).
func (db *DB) ActiveRun() (string, *RunRecord, error) {
	var runID string
	var rec *RunRecord

	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBuildRuns))
		if bucket == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketBuildRuns, Err: ErrBucketNotFound}
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var r RunRecord
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			if r.EndTime.IsZero() {
				runID = string(k)
				rec = &r
				break
			}
		}
		return nil
	})

	if err != nil {
		return "", nil, err
	}
	if rec == nil {
		return "", nil, nil
	}
	return runID, rec, nil
}

// PutRunPackage writes or updates a package record for the given run.
func (db *DB) PutRunPackage(runID string, pkg *RunPackageRecord) error {
	if runID == "" {
		return &ValidationError{Field: "runID", Err: ErrEmptyUUID}
	}
	if pkg == nil {
		return fmt.Errorf("package record is nil")
	}

	key := runPackageKey(runID, pkg.PortDir, pkg.Version)
	data, err := json.Marshal(pkg)
	if err != nil {
		return &RecordError{Op: "marshal run package", UUID: runID, Err: err}
	}

	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketRunPackages))
		if bucket == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketRunPackages, Err: ErrBucketNotFound}
		}
		return bucket.Put(key, data)
	})
}

// ListRunPackages returns all package records for the given run.
func (db *DB) ListRunPackages(runID string) ([]RunPackageRecord, error) {
	if runID == "" {
		return nil, &ValidationError{Field: "runID", Err: ErrEmptyUUID}
	}

	prefix := runPackagePrefix(runID)
	var records []RunPackageRecord

	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketRunPackages))
		if bucket == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketRunPackages, Err: ErrBucketNotFound}
		}

		c := bucket.Cursor()
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var rec RunPackageRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return err
			}
			records = append(records, rec)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return records, nil
}

func runPackageKey(runID, portDir, version string) []byte {
	key := fmt.Sprintf("%s@%s", portDir, version)
	return append(runPackagePrefix(runID), []byte(key)...)
}

func runPackagePrefix(runID string) []byte {
	return []byte(runID + "\x00")
}

func (db *DB) saveRunRecord(runID string, rec *RunRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return &RecordError{Op: "marshal run", UUID: runID, Err: err}
	}

	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBuildRuns))
		if bucket == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketBuildRuns, Err: ErrBucketNotFound}
		}
		return bucket.Put([]byte(runID), data)
	})
}

func (db *DB) updateRunRecord(runID string, mutate func(*RunRecord)) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBuildRuns))
		if bucket == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketBuildRuns, Err: ErrBucketNotFound}
		}

		data := bucket.Get([]byte(runID))
		if data == nil {
			return &RecordError{Op: "update run", UUID: runID, Err: ErrRecordNotFound}
		}

		var rec RunRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			return &RecordError{Op: "unmarshal run", UUID: runID, Err: err}
		}

		mutate(&rec)

		updated, err := json.Marshal(&rec)
		if err != nil {
			return &RecordError{Op: "marshal run", UUID: runID, Err: err}
		}

		return bucket.Put([]byte(runID), updated)
	})
}
