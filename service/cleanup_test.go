package service

import (
	"os"
	"path/filepath"
	"testing"

	"go-synth/config"
)

// TestCleanup_NoWorkers tests cleanup when no worker directories exist
func TestCleanup_NoWorkers(t *testing.T) {
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

	// Cleanup with no workers
	result, err := svc.CleanupStaleWorkers(CleanupOptions{})
	if err != nil {
		t.Fatalf("CleanupStaleWorkers() failed: %v", err)
	}

	if result.WorkersCleaned != 0 {
		t.Errorf("WorkersCleaned = %d, want 0", result.WorkersCleaned)
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got %d", len(result.Errors))
	}
}

// TestCleanup_SingleWorker tests cleanup of a single worker directory
func TestCleanup_SingleWorker(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create a worker directory (format: SL00, SL01, etc.)
	workerDir := filepath.Join(tmpDir, "SL00")
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		t.Fatalf("Failed to create worker dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Cleanup should remove the worker
	result, err := svc.CleanupStaleWorkers(CleanupOptions{})
	if err != nil {
		t.Fatalf("CleanupStaleWorkers() failed: %v", err)
	}

	if result.WorkersCleaned != 1 {
		t.Errorf("WorkersCleaned = %d, want 1", result.WorkersCleaned)
	}

	// Worker directory should be gone
	if _, err := os.Stat(workerDir); !os.IsNotExist(err) {
		t.Error("Worker directory still exists after cleanup")
	}
}

// TestCleanup_MultipleWorkers tests cleanup of multiple worker directories
func TestCleanup_MultipleWorkers(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create multiple worker directories (format: SL00, SL01, etc.)
	workers := []string{"SL00", "SL01", "SL02"}
	for _, worker := range workers {
		workerDir := filepath.Join(tmpDir, worker)
		if err := os.MkdirAll(workerDir, 0755); err != nil {
			t.Fatalf("Failed to create worker dir %s: %v", worker, err)
		}
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Cleanup should remove all workers
	result, err := svc.CleanupStaleWorkers(CleanupOptions{})
	if err != nil {
		t.Fatalf("CleanupStaleWorkers() failed: %v", err)
	}

	if result.WorkersCleaned != 3 {
		t.Errorf("WorkersCleaned = %d, want 3", result.WorkersCleaned)
	}

	// All worker directories should be gone
	for _, worker := range workers {
		workerDir := filepath.Join(tmpDir, worker)
		if _, err := os.Stat(workerDir); !os.IsNotExist(err) {
			t.Errorf("Worker directory %s still exists after cleanup", worker)
		}
	}
}

// TestCleanup_IgnoresNonWorkerDirs tests that cleanup ignores non-worker directories
func TestCleanup_IgnoresNonWorkerDirs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create non-worker directories
	nonWorkers := []string{"Template", "packages", "distfiles", "SomeOtherDir"}
	for _, dir := range nonWorkers {
		fullPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create one worker directory (format: SL00, SL01, etc.)
	workerDir := filepath.Join(tmpDir, "SL00")
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		t.Fatalf("Failed to create worker dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Cleanup should only remove the worker
	result, err := svc.CleanupStaleWorkers(CleanupOptions{})
	if err != nil {
		t.Fatalf("CleanupStaleWorkers() failed: %v", err)
	}

	if result.WorkersCleaned != 1 {
		t.Errorf("WorkersCleaned = %d, want 1", result.WorkersCleaned)
	}

	// Non-worker directories should still exist
	for _, dir := range nonWorkers {
		fullPath := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Non-worker directory %s was removed", dir)
		}
	}

	// Worker directory should be gone
	if _, err := os.Stat(workerDir); !os.IsNotExist(err) {
		t.Error("Worker directory still exists after cleanup")
	}
}

// TestGetWorkerDirectories tests listing worker directories
func TestGetWorkerDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		BuildBase: tmpDir,
		LogsPath:  filepath.Join(tmpDir, "logs"),
	}
	cfg.Database.Path = filepath.Join(tmpDir, "build.db")

	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create worker directories (format: SL00, SL01, etc.)
	expectedWorkers := []string{"SL00", "SL01"}
	for _, worker := range expectedWorkers {
		workerDir := filepath.Join(tmpDir, worker)
		if err := os.MkdirAll(workerDir, 0755); err != nil {
			t.Fatalf("Failed to create worker dir %s: %v", worker, err)
		}
	}

	// Create non-worker directory
	nonWorkerDir := filepath.Join(tmpDir, "Template")
	if err := os.MkdirAll(nonWorkerDir, 0755); err != nil {
		t.Fatalf("Failed to create non-worker dir: %v", err)
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}
	defer svc.Close()

	// Get worker directories
	workers, err := svc.GetWorkerDirectories()
	if err != nil {
		t.Fatalf("GetWorkerDirectories() failed: %v", err)
	}

	if len(workers) != 2 {
		t.Fatalf("Expected 2 workers, got %d", len(workers))
	}

	// Check that worker paths are correct
	for i, expected := range expectedWorkers {
		expectedPath := filepath.Join(tmpDir, expected)
		if workers[i] != expectedPath {
			t.Errorf("Worker %d: got %q, want %q", i, workers[i], expectedPath)
		}
	}
}

// TestGetWorkerDirectories_Empty tests listing when no workers exist
func TestGetWorkerDirectories_Empty(t *testing.T) {
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

	// Get worker directories (should be empty)
	workers, err := svc.GetWorkerDirectories()
	if err != nil {
		t.Fatalf("GetWorkerDirectories() failed: %v", err)
	}

	if len(workers) != 0 {
		t.Errorf("Expected 0 workers, got %d", len(workers))
	}
}
