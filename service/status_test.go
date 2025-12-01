package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-synth/builddb"
	"go-synth/config"
)

// TestGetStatus_EmptyDatabase tests status query on empty database
func TestGetStatus_EmptyDatabase(t *testing.T) {
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

	// Get overall stats (empty port list)
	result, err := svc.GetStatus(StatusOptions{})
	if err != nil {
		t.Fatalf("GetStatus() failed: %v", err)
	}

	if result.Stats == nil {
		t.Fatal("Stats is nil")
	}

	if result.Stats.TotalBuilds != 0 {
		t.Errorf("TotalBuilds = %d, want 0", result.Stats.TotalBuilds)
	}

	if result.Stats.TotalPorts != 0 {
		t.Errorf("TotalPorts = %d, want 0", result.Stats.TotalPorts)
	}
}

// TestGetStatus_OverallStats tests getting overall database statistics
func TestGetStatus_OverallStats(t *testing.T) {
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

	// Add some test data
	db := svc.Database()
	record := &builddb.BuildRecord{
		UUID:      "test-uuid-123",
		PortDir:   "editors/vim",
		Version:   "9.0",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(time.Minute),
		Status:    "success",
	}
	if err := db.SaveRecord(record); err != nil {
		t.Fatalf("Failed to save test record: %v", err)
	}
	if err := db.UpdatePackageIndex(record.PortDir, record.Version, record.UUID); err != nil {
		t.Fatalf("Failed to update package index: %v", err)
	}

	// Get overall stats
	result, err := svc.GetStatus(StatusOptions{})
	if err != nil {
		t.Fatalf("GetStatus() failed: %v", err)
	}

	if result.Stats == nil {
		t.Fatal("Stats is nil")
	}

	if result.Stats.TotalBuilds == 0 {
		t.Error("TotalBuilds is 0, expected > 0")
	}

	if result.Stats.TotalPorts == 0 {
		t.Error("TotalPorts is 0, expected > 0")
	}
}

// TestGetStatus_SpecificPort tests getting status for a specific port
func TestGetStatus_SpecificPort(t *testing.T) {
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

	// Add test data for a port
	db := svc.Database()
	portDir := "editors/vim"
	record := &builddb.BuildRecord{
		UUID:      "test-uuid-456",
		PortDir:   portDir,
		Version:   "9.0.1000",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(2 * time.Minute),
		Status:    "success",
	}
	if err := db.SaveRecord(record); err != nil {
		t.Fatalf("Failed to save test record: %v", err)
	}
	if err := db.UpdatePackageIndex(record.PortDir, record.Version, record.UUID); err != nil {
		t.Fatalf("Failed to update package index: %v", err)
	}

	// Update CRC for the port
	if err := db.UpdateCRC(portDir, 0x12345678); err != nil {
		t.Fatalf("Failed to update CRC: %v", err)
	}

	// Get status for specific port
	result, err := svc.GetStatus(StatusOptions{
		PortList: []string{portDir},
	})
	if err != nil {
		t.Fatalf("GetStatus() failed: %v", err)
	}

	if len(result.Ports) != 1 {
		t.Fatalf("Expected 1 port, got %d", len(result.Ports))
	}

	portStatus := result.Ports[0]
	if portStatus.PortDir != portDir {
		t.Errorf("PortDir = %q, want %q", portStatus.PortDir, portDir)
	}

	if portStatus.Version != "9.0.1000" {
		t.Errorf("Version = %q, want %q", portStatus.Version, "9.0.1000")
	}

	if portStatus.LastBuild == nil {
		t.Fatal("LastBuild is nil")
	}

	if portStatus.LastBuild.Status != "success" {
		t.Errorf("Status = %q, want %q", portStatus.LastBuild.Status, "success")
	}

	if portStatus.CRC != 0x12345678 {
		t.Errorf("CRC = %08x, want %08x", portStatus.CRC, 0x12345678)
	}
}

// TestGetStatus_NeverBuilt tests status for a port that was never built
func TestGetStatus_NeverBuilt(t *testing.T) {
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

	// Query a port that doesn't exist
	result, err := svc.GetStatus(StatusOptions{
		PortList: []string{"editors/nonexistent"},
	})
	if err != nil {
		t.Fatalf("GetStatus() failed: %v", err)
	}

	if len(result.Ports) != 1 {
		t.Fatalf("Expected 1 port, got %d", len(result.Ports))
	}

	portStatus := result.Ports[0]
	if portStatus.LastBuild != nil {
		t.Error("LastBuild should be nil for never-built port")
	}

	if portStatus.Version != "" {
		t.Error("Version should be empty for never-built port")
	}

	if portStatus.CRC != 0 {
		t.Error("CRC should be 0 for never-built port")
	}
}

// TestGetStatus_MultiplePorts tests status for multiple ports
func TestGetStatus_MultiplePorts(t *testing.T) {
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

	// Add test data for multiple ports
	db := svc.Database()
	ports := []string{"editors/vim", "shells/bash", "devel/git"}
	for i, portDir := range ports {
		record := &builddb.BuildRecord{
			UUID:      "test-uuid-" + portDir,
			PortDir:   portDir,
			Version:   "1.0",
			StartTime: time.Now(),
			EndTime:   time.Now().Add(time.Minute),
			Status:    "success",
		}
		if err := db.SaveRecord(record); err != nil {
			t.Fatalf("Failed to save record %d: %v", i, err)
		}
		if err := db.UpdatePackageIndex(record.PortDir, record.Version, record.UUID); err != nil {
			t.Fatalf("Failed to update package index %d: %v", i, err)
		}
	}

	// Get status for all ports
	result, err := svc.GetStatus(StatusOptions{
		PortList: ports,
	})
	if err != nil {
		t.Fatalf("GetStatus() failed: %v", err)
	}

	if len(result.Ports) != 3 {
		t.Fatalf("Expected 3 ports, got %d", len(result.Ports))
	}

	// Check all ports are present
	for i, expected := range ports {
		if result.Ports[i].PortDir != expected {
			t.Errorf("Port %d: got %q, want %q", i, result.Ports[i].PortDir, expected)
		}
		if result.Ports[i].LastBuild == nil {
			t.Errorf("Port %d: LastBuild is nil", i)
		}
	}
}

// TestGetDatabaseStats tests getting database statistics
func TestGetDatabaseStats(t *testing.T) {
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

	stats, err := svc.GetDatabaseStats()
	if err != nil {
		t.Fatalf("GetDatabaseStats() failed: %v", err)
	}

	if stats == nil {
		t.Fatal("Stats is nil")
	}

	if stats.DatabasePath != cfg.Database.Path {
		t.Errorf("DatabasePath = %q, want %q", stats.DatabasePath, cfg.Database.Path)
	}
}

// TestGetPortStatus tests getting status for a single port
func TestGetPortStatus(t *testing.T) {
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

	// Add test data
	db := svc.Database()
	portDir := "editors/emacs"
	record := &builddb.BuildRecord{
		UUID:      "test-uuid-emacs",
		PortDir:   portDir,
		Version:   "29.1",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(5 * time.Minute),
		Status:    "success",
	}
	if err := db.SaveRecord(record); err != nil {
		t.Fatalf("Failed to save test record: %v", err)
	}
	if err := db.UpdatePackageIndex(record.PortDir, record.Version, record.UUID); err != nil {
		t.Fatalf("Failed to update package index: %v", err)
	}

	// Get port status
	status, err := svc.GetPortStatus(portDir)
	if err != nil {
		t.Fatalf("GetPortStatus() failed: %v", err)
	}

	if status.PortDir != portDir {
		t.Errorf("PortDir = %q, want %q", status.PortDir, portDir)
	}

	if status.LastBuild == nil {
		t.Fatal("LastBuild is nil")
	}

	if status.LastBuild.UUID != "test-uuid-emacs" {
		t.Errorf("UUID = %q, want %q", status.LastBuild.UUID, "test-uuid-emacs")
	}
}
