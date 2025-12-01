package service

import (
	"os"
	"path/filepath"
	"testing"

	"go-synth/config"
)

// TestNewService tests successful service initialization
func TestNewService(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	// Create logs directory (logger expects it to exist)
	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	if svc.cfg != cfg {
		t.Error("Service config not set correctly")
	}

	if svc.logger == nil {
		t.Error("Service logger is nil")
	}

	if svc.db == nil {
		t.Error("Service database is nil")
	}
}

// TestNewService_InvalidLogPath tests service initialization with invalid log path
func TestNewService_InvalidLogPath(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  "/invalid/nonexistent/path/logs",
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	svc, err := NewService(cfg)
	if err == nil {
		svc.Close()
		t.Fatal("Expected error for invalid log path, got nil")
	}
}

// TestNewService_InvalidDatabasePath tests service initialization with invalid DB path
func TestNewService_InvalidDatabasePath(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = "/invalid/nonexistent/path/build.db"

	// Create logs directory
	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err == nil {
		svc.Close()
		t.Fatal("Expected error for invalid database path, got nil")
	}
}

// TestService_Close tests proper resource cleanup
func TestService_Close(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	// Create logs directory
	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}

	// Close should not return error
	if err := svc.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Close should be idempotent (calling twice shouldn't panic)
	if err := svc.Close(); err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}
}

// TestService_Config tests Config() accessor
func TestService_Config(t *testing.T) {
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

	if svc.Config() != cfg {
		t.Error("Config() returned wrong config")
	}
}

// TestService_Logger tests Logger() accessor
func TestService_Logger(t *testing.T) {
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

	if svc.Logger() == nil {
		t.Error("Logger() returned nil")
	}
}

// TestService_Database tests Database() accessor
func TestService_Database(t *testing.T) {
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

	if svc.Database() == nil {
		t.Error("Database() returned nil")
	}
}
