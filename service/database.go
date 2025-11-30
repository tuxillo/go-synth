package service

import (
	"fmt"
	"os"
	"path/filepath"
)

// DatabaseResult contains the results of a database operation.
type DatabaseResult struct {
	DatabaseRemoved bool     // Whether the database was removed
	FilesRemoved    []string // List of files that were removed
}

// ResetDatabase removes the build database and optionally legacy CRC files.
//
// This is a destructive operation that deletes all build history.
//
// This method handles all the business logic but does not interact with the user.
// The caller is responsible for:
//   - Confirming the operation with the user
//   - Displaying what will be deleted
//
// Returns DatabaseResult containing information about what was removed.
func (s *Service) ResetDatabase() (*DatabaseResult, error) {
	result := &DatabaseResult{
		FilesRemoved: make([]string, 0),
	}

	dbPath := s.cfg.Database.Path

	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return result, nil // No database to remove
	}

	// Close the database connection before removing
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return nil, fmt.Errorf("failed to close database before reset: %w", err)
		}
		s.db = nil // Mark as closed
	}

	// Remove database file
	if err := os.Remove(dbPath); err != nil {
		return nil, fmt.Errorf("failed to remove database: %w", err)
	}

	result.DatabaseRemoved = true
	result.FilesRemoved = append(result.FilesRemoved, dbPath)
	s.logger.Info("Build database removed: %s", dbPath)

	// Also remove legacy CRC file if present (optional cleanup)
	legacyFile := filepath.Join(s.cfg.BuildBase, "crc_index")
	if _, err := os.Stat(legacyFile); err == nil {
		if err := os.Remove(legacyFile); err == nil {
			result.FilesRemoved = append(result.FilesRemoved, legacyFile)
			s.logger.Info("Legacy CRC file removed: %s", legacyFile)
		}
	}

	// Also remove backup if present
	backupFile := legacyFile + ".bak"
	if _, err := os.Stat(backupFile); err == nil {
		if err := os.Remove(backupFile); err == nil {
			result.FilesRemoved = append(result.FilesRemoved, backupFile)
			s.logger.Info("Legacy CRC backup removed: %s", backupFile)
		}
	}

	return result, nil
}

// DatabaseExists checks if the build database file exists.
func (s *Service) DatabaseExists() bool {
	_, err := os.Stat(s.cfg.Database.Path)
	return err == nil
}

// GetDatabasePath returns the path to the build database.
func (s *Service) GetDatabasePath() string {
	return s.cfg.Database.Path
}

// BackupDatabase creates a backup of the build database.
func (s *Service) BackupDatabase() (string, error) {
	dbPath := s.cfg.Database.Path

	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return "", fmt.Errorf("database does not exist: %s", dbPath)
	}

	// Create backup file with timestamp
	backupPath := fmt.Sprintf("%s.backup", dbPath)

	// Copy database file to backup location
	input, err := os.ReadFile(dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to read database: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	s.logger.Info("Database backed up to: %s", backupPath)
	return backupPath, nil
}
