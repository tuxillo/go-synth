// Package builddb provides build database functionality using bbolt
// for persistent tracking of build attempts and CRC-based change detection.
package builddb

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
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
		return nil, &DatabaseError{Op: "open", Err: err}
	}

	// Initialize required buckets in a single write transaction
	err = bdb.Update(func(tx *bolt.Tx) error {
		// Create builds bucket for storing BuildRecord JSON
		if _, err := tx.CreateBucketIfNotExists([]byte(BucketBuilds)); err != nil {
			return &DatabaseError{Op: "create bucket", Bucket: BucketBuilds, Err: err}
		}

		// Create packages bucket for tracking latest successful builds
		// Key format: "portdir@version" -> UUID
		if _, err := tx.CreateBucketIfNotExists([]byte(BucketPackages)); err != nil {
			return &DatabaseError{Op: "create bucket", Bucket: BucketPackages, Err: err}
		}

		// Create crc_index bucket for fast CRC lookups
		// Key: portdir -> binary uint32 CRC value
		if _, err := tx.CreateBucketIfNotExists([]byte(BucketCRCIndex)); err != nil {
			return &DatabaseError{Op: "create bucket", Bucket: BucketCRCIndex, Err: err}
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
		return &ValidationError{Field: "record.UUID", Err: ErrEmptyUUID}
	}

	// Marshal BuildRecord to JSON
	data, err := json.Marshal(rec)
	if err != nil {
		return &RecordError{Op: "marshal", UUID: rec.UUID, Err: err}
	}

	// Store in builds bucket
	err = db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBuilds))
		if bucket == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketBuilds, Err: ErrBucketNotFound}
		}
		return bucket.Put([]byte(rec.UUID), data)
	})

	if err != nil {
		return &RecordError{Op: "save", UUID: rec.UUID, Err: err}
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
		return nil, &ValidationError{Field: "uuid", Err: ErrEmptyUUID}
	}

	var rec BuildRecord

	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBuilds))
		if bucket == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketBuilds, Err: ErrBucketNotFound}
		}

		data := bucket.Get([]byte(uuid))
		if data == nil {
			return &RecordError{Op: "get", UUID: uuid, Err: ErrRecordNotFound}
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
		return &ValidationError{Field: "uuid", Err: ErrEmptyUUID}
	}

	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBuilds))
		if bucket == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketBuilds, Err: ErrBucketNotFound}
		}

		// Read existing record
		data := bucket.Get([]byte(uuid))
		if data == nil {
			return &RecordError{Op: "update status", UUID: uuid, Err: ErrRecordNotFound}
		}

		// Unmarshal, update, marshal
		var rec BuildRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			return &RecordError{Op: "unmarshal", UUID: uuid, Err: err}
		}

		rec.Status = status
		rec.EndTime = endTime

		updatedData, err := json.Marshal(&rec)
		if err != nil {
			return &RecordError{Op: "marshal", UUID: uuid, Err: err}
		}

		// Save back
		return bucket.Put([]byte(uuid), updatedData)
	})

	if err != nil {
		return &RecordError{Op: "update status", UUID: uuid, Err: err}
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
			return &DatabaseError{Op: "get bucket", Bucket: BucketPackages, Err: ErrBucketNotFound}
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
			return &DatabaseError{Op: "get bucket", Bucket: BucketBuilds, Err: ErrBucketNotFound}
		}

		recordBytes := builds.Get(uuidBytes)
		if recordBytes == nil {
			// UUID points to non-existent record - data inconsistency
			return &PackageIndexError{
				Op:      "validate",
				PortDir: portDir,
				Version: version,
				Err:     ErrOrphanedRecord,
			}
		}

		// Unmarshal the build record
		rec = &BuildRecord{}
		if err := json.Unmarshal(recordBytes, rec); err != nil {
			return &RecordError{Op: "unmarshal", UUID: string(uuidBytes), Err: err}
		}

		return nil
	})

	if err != nil {
		return nil, &PackageIndexError{Op: "lookup", PortDir: portDir, Version: version, Err: err}
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
			return &DatabaseError{Op: "get bucket", Bucket: BucketPackages, Err: ErrBucketNotFound}
		}

		return packages.Put(key, value)
	})

	if err != nil {
		return &PackageIndexError{Op: "update", PortDir: portDir, Version: version, Err: err}
	}

	return nil
}

// NeedsBuild determines whether a port needs to be rebuilt based on CRC comparison.
//
// The function compares the provided currentCRC against the stored CRC in the crc_index
// bucket. Returns true if the port needs rebuilding (CRC changed or no stored CRC exists),
// and false if the CRC matches (port unchanged).
//
// This is the primary function for incremental build detection - call it before starting
// a build to determine if the port's source files have changed.
//
// Parameters:
//   - portDir: The port directory path (e.g., "editors/vim")
//   - currentCRC: The CRC32 checksum of the port's current state
//
// Returns:
//   - bool: true if build is needed (CRC changed or missing), false if unchanged
//   - error: Any database access errors
func (db *DB) NeedsBuild(portDir string, currentCRC uint32) (bool, error) {
	storedCRC, exists, err := db.GetCRC(portDir)
	if err != nil {
		return false, &CRCError{Op: "check needs build", PortDir: portDir, Err: err}
	}

	// No stored CRC means this port has never been built
	if !exists {
		return true, nil
	}

	// CRC mismatch means port has changed
	return storedCRC != currentCRC, nil
}

// UpdateCRC updates the stored CRC checksum for a given port directory.
//
// This function should be called after a successful build to record the port's
// current state. The CRC is stored as a 4-byte binary value (little-endian uint32)
// in the crc_index bucket.
//
// Parameters:
//   - portDir: The port directory path (e.g., "editors/vim")
//   - crc: The CRC32 checksum to store
//
// Returns:
//   - error: Any database errors that occur during the update
func (db *DB) UpdateCRC(portDir string, crc uint32) error {
	key := []byte(portDir)
	value := make([]byte, 4)

	// Store CRC as little-endian binary (4 bytes)
	value[0] = byte(crc)
	value[1] = byte(crc >> 8)
	value[2] = byte(crc >> 16)
	value[3] = byte(crc >> 24)

	err := db.db.Update(func(tx *bolt.Tx) error {
		crcIndex := tx.Bucket([]byte("crc_index"))
		if crcIndex == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketCRCIndex, Err: ErrBucketNotFound}
		}

		return crcIndex.Put(key, value)
	})

	if err != nil {
		return &CRCError{Op: "update", PortDir: portDir, Err: err}
	}

	return nil
}

// GetCRC retrieves the stored CRC checksum for a given port directory.
//
// The function reads the 4-byte binary CRC value from the crc_index bucket
// and returns it as a uint32. The second return value indicates whether a
// CRC exists for this port (false means the port has never been built).
//
// Parameters:
//   - portDir: The port directory path (e.g., "editors/vim")
//
// Returns:
//   - uint32: The stored CRC32 checksum (0 if not found)
//   - bool: true if a CRC was found, false if no entry exists
//   - error: Any database access errors
func (db *DB) GetCRC(portDir string) (uint32, bool, error) {
	key := []byte(portDir)
	var crc uint32
	var found bool

	err := db.db.View(func(tx *bolt.Tx) error {
		crcIndex := tx.Bucket([]byte("crc_index"))
		if crcIndex == nil {
			return &DatabaseError{Op: "get bucket", Bucket: BucketCRCIndex, Err: ErrBucketNotFound}
		}

		value := crcIndex.Get(key)
		if value == nil {
			// No CRC stored for this port
			found = false
			return nil
		}

		// Validate value length
		if len(value) != 4 {
			return &ValidationError{
				Field: "crc",
				Value: fmt.Sprintf("%d bytes", len(value)),
				Err:   ErrCorruptedData,
			}
		}

		// Read little-endian binary CRC (4 bytes)
		crc = uint32(value[0]) | uint32(value[1])<<8 | uint32(value[2])<<16 | uint32(value[3])<<24
		found = true
		return nil
	})

	if err != nil {
		return 0, false, &CRCError{Op: "get", PortDir: portDir, Err: err}
	}

	return crc, found, nil
}

// ComputePortCRC calculates a CRC32 checksum of all files in a port directory.
//
// Unlike metadata-based approaches (which hash file size + mtime), this function
// hashes actual file contents to reliably detect changes regardless of modification
// times. This eliminates false positives from operations like git clone, rsync, or
// tar extraction that reset file timestamps.
//
// The function walks the port directory and:
//   - Hashes each file's relative path (to detect structure changes like renamed files)
//   - Hashes each file's actual content (to detect content changes)
//   - Skips work directories and version control systems (.git, .svn, CVS)
//   - Uses CRC32-IEEE polynomial for speed and collision resistance
//
// Performance: Typical ports contain 4-9 small files (Makefiles, patches, distinfo)
// totaling a few KB. Reading and hashing these files takes ~10-50 microseconds per
// port on modern hardware, making this approach practical for thousands of ports.
//
// Use this function before calling NeedsBuild() to determine if a port's source
// files have changed since the last successful build.
//
// Parameters:
//   - portPath: Absolute path to port directory (e.g., "/dports/editors/vim")
//
// Returns:
//   - uint32: CRC32 checksum of all port files' paths and contents
//   - error: Filesystem errors, I/O errors, or path errors
//
// Example:
//
//	crc, err := builddb.ComputePortCRC("/usr/dports/editors/vim")
//	if err != nil {
//	    return fmt.Errorf("failed to compute CRC: %w", err)
//	}
//	needsBuild, err := db.NeedsBuild("editors/vim", crc)
func ComputePortCRC(portPath string) (uint32, error) {
	hash := crc32.NewIEEE()

	// Walk the port directory tree
	err := filepath.Walk(portPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip work directories and version control systems
		base := filepath.Base(path)
		if base == ".git" || base == "work" || base == ".svn" || base == "CVS" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process regular files
		if !info.Mode().IsRegular() {
			return nil
		}

		// Hash relative file path (detects renamed/moved files)
		relPath, err := filepath.Rel(portPath, path)
		if err != nil {
			return &CRCError{Op: "compute", PortDir: portPath, Err: err}
		}
		hash.Write([]byte(relPath))
		hash.Write([]byte{0}) // Null separator

		// Hash actual file contents (detects content changes)
		data, err := os.ReadFile(path)
		if err != nil {
			return &CRCError{Op: "compute", PortDir: portPath, Err: err}
		}
		hash.Write(data)

		return nil
	})

	if err != nil {
		return 0, &CRCError{Op: "compute", PortDir: portPath, Err: err}
	}

	return hash.Sum32(), nil
}
