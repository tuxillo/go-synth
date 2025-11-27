// Package builddb provides build database functionality using bbolt
// for persistent tracking of build attempts and CRC-based change detection.
package builddb

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket names for bbolt database
const (
	BucketBuilds   = "builds"
	BucketPackages = "packages"
	BucketCRCIndex = "crc_index"
)

// DB wraps a bbolt database for build tracking and CRC indexing
type DB struct {
	db   *bolt.DB
	path string
}

// BuildRecord represents a single build attempt with status and timestamps
type BuildRecord struct {
	UUID      string    `json:"uuid"`
	PortDir   string    `json:"portdir"`
	Version   string    `json:"version"`
	Status    string    `json:"status"` // "running" | "success" | "failed"
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// OpenDB opens or creates a bbolt database at the given path.
// It automatically initializes the required buckets (builds, packages, crc_index)
// if they don't exist. The database is opened with 0600 permissions.
//
// Parameters:
//   - path: Filesystem path to the database file
//
// Returns:
//   - *DB: Database handle if successful
//   - error: Any error encountered during open or initialization
//
// Example:
//
//	db, err := OpenDB("/var/db/go-synth/builds.db")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
func OpenDB(path string) (*DB, error) {
	// Open database with user read/write permissions only (0600)
	bdb, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize required buckets in a single write transaction
	err = bdb.Update(func(tx *bolt.Tx) error {
		// Create builds bucket for storing BuildRecord JSON
		if _, err := tx.CreateBucketIfNotExists([]byte(BucketBuilds)); err != nil {
			return fmt.Errorf("create builds bucket: %w", err)
		}

		// Create packages bucket for tracking latest successful builds
		// Key format: "portdir@version" -> UUID
		if _, err := tx.CreateBucketIfNotExists([]byte(BucketPackages)); err != nil {
			return fmt.Errorf("create packages bucket: %w", err)
		}

		// Create crc_index bucket for fast CRC lookups
		// Key: portdir -> binary uint32 CRC value
		if _, err := tx.CreateBucketIfNotExists([]byte(BucketCRCIndex)); err != nil {
			return fmt.Errorf("create crc_index bucket: %w", err)
		}

		return nil
	})

	if err != nil {
		// Close database if bucket initialization fails
		bdb.Close()
		return nil, err
	}

	return &DB{
		db:   bdb,
		path: path,
	}, nil
}

// Close closes the database connection and flushes any pending writes to disk.
// It is safe to call Close multiple times. After Close is called, the DB
// should not be used.
//
// Returns:
//   - error: Any error encountered during close operation
//
// Example:
//
//	db, err := OpenDB("/tmp/builds.db")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
func (db *DB) Close() error {
	if db.db == nil {
		return nil
	}
	return db.db.Close()
}

// SaveRecord stores a BuildRecord in the database. The record is serialized
// to JSON and stored in the builds bucket with the UUID as the key.
//
// Parameters:
//   - rec: Pointer to BuildRecord to save
//
// Returns:
//   - error: Any error encountered during save operation
//
// Example:
//
//	rec := &BuildRecord{
//	    UUID:      "abc-123",
//	    PortDir:   "editors/vim",
//	    Version:   "9.0.1",
//	    Status:    "running",
//	    StartTime: time.Now(),
//	}
//	if err := db.SaveRecord(rec); err != nil {
//	    log.Fatal(err)
//	}
func (db *DB) SaveRecord(rec *BuildRecord) error {
	if rec.UUID == "" {
		return fmt.Errorf("build record UUID cannot be empty")
	}

	// Marshal BuildRecord to JSON
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to marshal build record: %w", err)
	}

	// Store in builds bucket
	err = db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBuilds))
		if bucket == nil {
			return fmt.Errorf("builds bucket not found")
		}
		return bucket.Put([]byte(rec.UUID), data)
	})

	if err != nil {
		return fmt.Errorf("failed to save build record: %w", err)
	}

	return nil
}

// GetRecord retrieves a BuildRecord from the database by its UUID.
//
// Parameters:
//   - uuid: The unique identifier of the build record
//
// Returns:
//   - *BuildRecord: The retrieved record, or nil if not found
//   - error: Any error encountered, including not found errors
//
// Example:
//
//	rec, err := db.GetRecord("abc-123")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Build status: %s\n", rec.Status)
func (db *DB) GetRecord(uuid string) (*BuildRecord, error) {
	if uuid == "" {
		return nil, fmt.Errorf("UUID cannot be empty")
	}

	var rec BuildRecord

	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBuilds))
		if bucket == nil {
			return fmt.Errorf("builds bucket not found")
		}

		data := bucket.Get([]byte(uuid))
		if data == nil {
			return fmt.Errorf("build record not found: %s", uuid)
		}

		return json.Unmarshal(data, &rec)
	})

	if err != nil {
		return nil, err
	}

	return &rec, nil
}

// UpdateRecordStatus updates the status and end time of an existing BuildRecord.
// This is more efficient than retrieving the full record, modifying it, and
// saving it back, as it does the read-modify-write in a single transaction.
//
// Parameters:
//   - uuid: The unique identifier of the build record to update
//   - status: New status value (e.g., "success", "failed")
//   - endTime: The completion timestamp
//
// Returns:
//   - error: Any error encountered during update operation
//
// Example:
//
//	err := db.UpdateRecordStatus("abc-123", "success", time.Now())
//	if err != nil {
//	    log.Fatal(err)
//	}
func (db *DB) UpdateRecordStatus(uuid, status string, endTime time.Time) error {
	if uuid == "" {
		return fmt.Errorf("UUID cannot be empty")
	}

	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBuilds))
		if bucket == nil {
			return fmt.Errorf("builds bucket not found")
		}

		// Read existing record
		data := bucket.Get([]byte(uuid))
		if data == nil {
			return fmt.Errorf("build record not found: %s", uuid)
		}

		// Unmarshal, update, marshal
		var rec BuildRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			return fmt.Errorf("failed to unmarshal build record: %w", err)
		}

		rec.Status = status
		rec.EndTime = endTime

		updatedData, err := json.Marshal(&rec)
		if err != nil {
			return fmt.Errorf("failed to marshal updated record: %w", err)
		}

		// Save back
		return bucket.Put([]byte(uuid), updatedData)
	})

	if err != nil {
		return fmt.Errorf("failed to update build record status: %w", err)
	}

	return nil
}

// LatestFor retrieves the most recent successful build record for a given port
// directory and version combination.
//
// The function looks up the package index using the key format "portdir@version"
// (e.g., "editors/vim@9.0.1") and returns the full BuildRecord for the associated
// UUID. Returns nil with no error if no record exists for this port/version.
//
// Parameters:
//   - portDir: The port directory path (e.g., "editors/vim")
//   - version: The version string (e.g., "9.0.1")
//
// Returns:
//   - *BuildRecord: The latest successful build record, or nil if not found
//   - error: Any database or unmarshaling errors
func (db *DB) LatestFor(portDir, version string) (*BuildRecord, error) {
	key := []byte(portDir + "@" + version)
	var rec *BuildRecord

	err := db.db.View(func(tx *bolt.Tx) error {
		packages := tx.Bucket([]byte("packages"))
		if packages == nil {
			return fmt.Errorf("packages bucket not found")
		}

		// Look up UUID in packages bucket
		uuidBytes := packages.Get(key)
		if uuidBytes == nil {
			// No record found - not an error, just means no builds yet
			return nil
		}

		// Retrieve the full record from builds bucket
		builds := tx.Bucket([]byte("builds"))
		if builds == nil {
			return fmt.Errorf("builds bucket not found")
		}

		recordBytes := builds.Get(uuidBytes)
		if recordBytes == nil {
			// UUID points to non-existent record - data inconsistency
			return fmt.Errorf("package index references non-existent build UUID: %s", string(uuidBytes))
		}

		// Unmarshal the build record
		rec = &BuildRecord{}
		if err := json.Unmarshal(recordBytes, rec); err != nil {
			return fmt.Errorf("failed to unmarshal build record: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve latest build for %s@%s: %w", portDir, version, err)
	}

	return rec, nil
}

// UpdatePackageIndex updates the package index to point to the latest successful
// build for a given port directory and version combination.
//
// This function should be called when a build completes successfully to ensure
// the package index tracks the most recent successful build. The key format is
// "portdir@version" (matching the flavor syntax used throughout go-synth).
//
// Parameters:
//   - portDir: The port directory path (e.g., "editors/vim")
//   - version: The version string (e.g., "9.0.1")
//   - uuid: The UUID of the successful build to track
//
// Returns:
//   - error: Any database errors that occur during the update
func (db *DB) UpdatePackageIndex(portDir, version, uuid string) error {
	key := []byte(portDir + "@" + version)
	value := []byte(uuid)

	err := db.db.Update(func(tx *bolt.Tx) error {
		packages := tx.Bucket([]byte("packages"))
		if packages == nil {
			return fmt.Errorf("packages bucket not found")
		}

		return packages.Put(key, value)
	})

	if err != nil {
		return fmt.Errorf("failed to update package index for %s@%s: %w", portDir, version, err)
	}

	return nil
}
