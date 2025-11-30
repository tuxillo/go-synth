package service

import (
	"os"
	"path/filepath"
	"testing"

	"dsynth/config"
)

// TestInitialize_CreatesDirectories tests that Initialize creates all required directories
func TestInitialize_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	// Create logs directory (required for service creation)
	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Initialize should create all directories
	result, err := svc.Initialize(InitOptions{SkipSystemFiles: true})
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Check that directories were created
	expectedDirs := []string{
		cfg.BuildBase,
		cfg.LogsPath,
		cfg.DPortsPath,
		cfg.RepositoryPath,
		cfg.PackagesPath,
		cfg.DistFilesPath,
		cfg.OptionsPath,
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory not created: %s", dir)
		}
	}

	// Check result contains created directories
	if len(result.DirsCreated) == 0 {
		t.Error("No directories reported as created")
	}
}

// TestInitialize_CreatesTemplate tests that Initialize creates the template directory
func TestInitialize_CreatesTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
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

	result, err := svc.Initialize(InitOptions{SkipSystemFiles: true})
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	if !result.TemplateCreated {
		t.Error("Template not reported as created")
	}

	// Check template directory exists
	templateDir := filepath.Join(cfg.BuildBase, "Template")
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Error("Template directory not created")
	}

	// Check template subdirectories exist
	templateSubdirs := []string{"etc", "var/run", "var/db", "tmp"}
	for _, subdir := range templateSubdirs {
		fullPath := filepath.Join(templateDir, subdir)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Template subdirectory not created: %s", subdir)
		}
	}
}

// TestInitialize_DatabaseInitialized tests that database is initialized
func TestInitialize_DatabaseInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
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

	result, err := svc.Initialize(InitOptions{SkipSystemFiles: true})
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	if !result.DatabaseInitalized {
		t.Error("Database not reported as initialized")
	}

	// Check database file exists
	if _, err := os.Stat(cfg.Database.Path); os.IsNotExist(err) {
		t.Error("Database file not created")
	}
}

// TestInitialize_VerifiesPortsDirectory tests ports directory verification
func TestInitialize_VerifiesPortsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create ports directory with some content
	if err := os.MkdirAll(cfg.DPortsPath, 0755); err != nil {
		t.Fatalf("Failed to create dports dir: %v", err)
	}

	// Add some fake ports
	testPorts := []string{"editors", "shells", "devel"}
	for _, port := range testPorts {
		portDir := filepath.Join(cfg.DPortsPath, port)
		if err := os.MkdirAll(portDir, 0755); err != nil {
			t.Fatalf("Failed to create port dir: %v", err)
		}
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	result, err := svc.Initialize(InitOptions{SkipSystemFiles: true})
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	if result.PortsFound != 3 {
		t.Errorf("PortsFound = %d, want 3", result.PortsFound)
	}

	if len(result.Warnings) != 0 {
		t.Errorf("Expected no warnings, got %d", len(result.Warnings))
	}
}

// TestInitialize_EmptyPortsDirectory tests warning for empty ports directory
func TestInitialize_EmptyPortsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
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

	result, err := svc.Initialize(InitOptions{SkipSystemFiles: true})
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	if result.PortsFound != 0 {
		t.Errorf("PortsFound = %d, want 0", result.PortsFound)
	}

	// Should have warning about empty ports directory
	if len(result.Warnings) == 0 {
		t.Error("Expected warning about empty ports directory")
	}
}

// TestNeedsMigration tests migration detection
func TestNeedsMigration(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create legacy CRC file
	legacyFile := filepath.Join(tmpDir, "crc_index")
	if err := os.WriteFile(legacyFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create legacy file: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	if !svc.NeedsMigration() {
		t.Error("NeedsMigration() returned false, expected true")
	}
}

// TestNeedsMigration_NoLegacyFile tests when no migration is needed
func TestNeedsMigration_NoLegacyFile(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
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

	if svc.NeedsMigration() {
		t.Error("NeedsMigration() returned true, expected false")
	}
}

// TestGetLegacyCRCFile tests getting legacy CRC file path
func TestGetLegacyCRCFile(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create legacy CRC file
	legacyFile := filepath.Join(tmpDir, "crc_index")
	if err := os.WriteFile(legacyFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create legacy file: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	file, err := svc.GetLegacyCRCFile()
	if err != nil {
		t.Fatalf("GetLegacyCRCFile() failed: %v", err)
	}

	if file != legacyFile {
		t.Errorf("GetLegacyCRCFile() = %q, want %q", file, legacyFile)
	}
}

// TestGetLegacyCRCFile_NotExists tests when legacy file doesn't exist
func TestGetLegacyCRCFile_NotExists(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
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

	file, err := svc.GetLegacyCRCFile()
	if err != nil {
		t.Fatalf("GetLegacyCRCFile() failed: %v", err)
	}

	if file != "" {
		t.Errorf("GetLegacyCRCFile() = %q, want empty string", file)
	}
}

// TestInitialize_Idempotent tests that Initialize can be called multiple times
func TestInitialize_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase:      tmpDir,
		LogsPath:       filepath.Join(tmpDir, "logs"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repository"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "distfiles"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
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

	// First initialization
	result1, err := svc.Initialize(InitOptions{SkipSystemFiles: true})
	if err != nil {
		t.Fatalf("First Initialize() failed: %v", err)
	}

	// Second initialization should succeed (idempotent)
	result2, err := svc.Initialize(InitOptions{SkipSystemFiles: true})
	if err != nil {
		t.Fatalf("Second Initialize() failed: %v", err)
	}

	// Both should report success
	if !result1.DatabaseInitalized {
		t.Error("First init: Database not initialized")
	}
	if !result2.DatabaseInitalized {
		t.Error("Second init: Database not initialized")
	}
}
