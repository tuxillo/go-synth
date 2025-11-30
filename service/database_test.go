package service

import (
	"os"
	"path/filepath"
	"testing"

	"dsynth/config"
)

// TestDatabaseExists tests database file existence check
func TestDatabaseExists(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Database should exist after NewService (it creates it)
	if !svc.DatabaseExists() {
		t.Error("DatabaseExists() returned false, expected true")
	}
}

// TestDatabaseExists_NoDB tests when database doesn't exist
func TestDatabaseExists_NoDB(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "nonexistent.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Remove the database after service creation
	if err := os.Remove(cfg.Database.Path); err != nil {
		t.Fatalf("Failed to remove database: %v", err)
	}

	// Now it shouldn't exist
	if svc.DatabaseExists() {
		t.Error("DatabaseExists() returned true, expected false")
	}
}

// TestGetDatabasePath tests database path accessor
func TestGetDatabasePath(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "build.db")

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = dbPath

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	if svc.GetDatabasePath() != dbPath {
		t.Errorf("GetDatabasePath() = %q, want %q", svc.GetDatabasePath(), dbPath)
	}
}

// TestBackupDatabase tests database backup creation
func TestBackupDatabase(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Backup the database
	backupPath, err := svc.BackupDatabase()
	if err != nil {
		t.Fatalf("BackupDatabase() failed: %v", err)
	}

	// Check that backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup file not created: %s", backupPath)
	}

	// Check that backup has expected name format
	expectedBackup := cfg.Database.Path + ".backup"
	if backupPath != expectedBackup {
		t.Errorf("Backup path = %q, want %q", backupPath, expectedBackup)
	}
}

// TestBackupDatabase_NoDatabase tests backup when database doesn't exist
func TestBackupDatabase_NoDatabase(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Remove database
	if err := os.Remove(cfg.Database.Path); err != nil {
		t.Fatalf("Failed to remove database: %v", err)
	}

	// Backup should fail
	_, err = svc.BackupDatabase()
	if err == nil {
		t.Error("BackupDatabase() succeeded, expected error")
	}
}

// TestResetDatabase tests database reset
func TestResetDatabase(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Database should exist
	if !svc.DatabaseExists() {
		t.Fatal("Database doesn't exist before reset")
	}

	// Reset the database
	result, err := svc.ResetDatabase()
	if err != nil {
		t.Fatalf("ResetDatabase() failed: %v", err)
	}

	if !result.DatabaseRemoved {
		t.Error("DatabaseRemoved is false, expected true")
	}

	if len(result.FilesRemoved) == 0 {
		t.Error("No files removed")
	}

	// Database should not exist after reset
	if _, err := os.Stat(cfg.Database.Path); err == nil {
		t.Error("Database still exists after reset")
	}
}

// TestResetDatabase_NoDB tests reset when database doesn't exist
func TestResetDatabase_NoDB(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "nonexistent.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}

	// Remove database immediately after creation
	if err := os.Remove(cfg.Database.Path); err != nil {
		t.Fatalf("Failed to remove database: %v", err)
	}
	svc.Close()

	// Create a new service without database
	cfg.Database.Path = filepath.Join(tmpDir, "never-existed.db")
	svc, err = NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Remove it so it doesn't exist
	os.Remove(cfg.Database.Path)

	// Reset should succeed but not remove anything
	result, err := svc.ResetDatabase()
	if err != nil {
		t.Fatalf("ResetDatabase() failed: %v", err)
	}

	if result.DatabaseRemoved {
		t.Error("DatabaseRemoved is true, expected false")
	}
}

// TestResetDatabase_WithLegacyFiles tests reset removes legacy CRC files
func TestResetDatabase_WithLegacyFiles(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create legacy CRC files
	legacyFile := filepath.Join(tmpDir, "crc_index")
	backupFile := legacyFile + ".bak"

	if err := os.WriteFile(legacyFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create legacy file: %v", err)
	}
	if err := os.WriteFile(backupFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create backup file: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Reset should remove database and legacy files
	result, err := svc.ResetDatabase()
	if err != nil {
		t.Fatalf("ResetDatabase() failed: %v", err)
	}

	// Should have removed 3 files: database, legacy, backup
	if len(result.FilesRemoved) < 1 {
		t.Errorf("Expected at least 1 file removed, got %d", len(result.FilesRemoved))
	}

	// Check legacy files are removed
	if _, err := os.Stat(legacyFile); err == nil {
		t.Error("Legacy CRC file still exists")
	}
	if _, err := os.Stat(backupFile); err == nil {
		t.Error("Legacy CRC backup still exists")
	}
}
