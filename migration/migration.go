// Package migration provides utilities for migrating from legacy file-based
// CRC storage to the new BuildDB format.
//
// The legacy format is a plain text file located at ${BuildBase}/crc_index
// with lines in the format: portdir:crc32_hex
//
// Example usage:
//
//	if migration.DetectMigrationNeeded(cfg) {
//	    if err := migration.MigrateLegacyCRC(cfg, db); err != nil {
//	        log.Fatalf("Migration failed: %v", err)
//	    }
//	}
package migration

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dsynth/builddb"
	"dsynth/config"
)

// CRCRecord represents a legacy CRC entry from file.
type CRCRecord struct {
	PortDir string
	CRC     uint32
}

// MigrateLegacyCRC imports CRC data from legacy file format into BuildDB.
//
// The legacy file is expected at ${BuildBase}/crc_index with the format:
//
//	portdir:crc32_hex
//
// Lines starting with '#' are treated as comments and skipped.
// Empty lines are ignored.
// Invalid lines are logged as warnings but do not cause the migration to fail.
//
// After successful migration, the legacy file is backed up to crc_index.bak.
//
// Returns nil if no legacy file exists (no migration needed).
// Returns error only for critical failures (file read errors, etc).
func MigrateLegacyCRC(cfg *config.Config, db *builddb.DB) error {
	legacyFile := filepath.Join(cfg.BuildBase, "crc_index")

	// Check if legacy file exists
	if _, err := os.Stat(legacyFile); os.IsNotExist(err) {
		// No legacy data, nothing to migrate
		return nil
	}

	fmt.Printf("Found legacy CRC file: %s\n", legacyFile)

	// Read legacy file
	records, err := readLegacyCRCFile(legacyFile)
	if err != nil {
		return fmt.Errorf("failed to read legacy CRC file: %w", err)
	}

	fmt.Printf("Migrating %d CRC records...\n", len(records))

	// Import into BuildDB
	migrated := 0
	for _, rec := range records {
		if err := db.UpdateCRC(rec.PortDir, rec.CRC); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to migrate %s: %v\n",
				rec.PortDir, err)
			continue
		}
		migrated++
	}

	fmt.Printf("Successfully migrated %d/%d records\n", migrated, len(records))

	// Backup legacy file
	backupFile := legacyFile + ".bak"
	if err := os.Rename(legacyFile, backupFile); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to backup legacy file: %v\n", err)
	} else {
		fmt.Printf("Legacy file backed up to: %s\n", backupFile)
	}

	return nil
}

// readLegacyCRCFile parses the legacy CRC file format.
//
// Expected format:
//
//	# Comment lines start with hash
//	portdir:crc32_hex
//
// Example:
//
//	# Legacy CRC index
//	editors/vim:deadbeef
//	devel/git:cafebabe
//
// Returns a slice of CRCRecord entries.
// Invalid lines are logged as warnings but do not cause an error.
func readLegacyCRCFile(path string) ([]CRCRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []CRCRecord
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Expected format: "portdir:crc32_hex"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid line (no colon): %s\n", line)
			continue
		}

		portDir := parts[0]
		var crc uint32
		if _, err := fmt.Sscanf(parts[1], "%x", &crc); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid CRC for %s: %v\n",
				portDir, err)
			continue
		}

		records = append(records, CRCRecord{
			PortDir: portDir,
			CRC:     crc,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

// DetectMigrationNeeded checks if migration is required.
//
// Returns true if a legacy CRC file exists at ${BuildBase}/crc_index,
// false otherwise.
//
// This function can be used to prompt the user before running migration.
func DetectMigrationNeeded(cfg *config.Config) bool {
	legacyFile := filepath.Join(cfg.BuildBase, "crc_index")
	_, err := os.Stat(legacyFile)
	return err == nil
}
