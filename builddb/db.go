// Package builddb provides build database functionality using bbolt
// for persistent tracking of build attempts and CRC-based change detection.
package builddb

import (
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
