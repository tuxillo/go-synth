package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dsynth/config"
)

func TestNewLogger(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	// Verify log directory was created
	if _, err := os.Stat(cfg.LogsPath); os.IsNotExist(err) {
		t.Error("Logs directory was not created")
	}

	// Verify all log files exist
	expectedFiles := []string{
		"00_last_results.log",
		"01_success_list.log",
		"02_failure_list.log",
		"03_ignored_list.log",
		"04_skipped_list.log",
		"05_abnormal_command_output.log",
		"06_obsolete_packages.log",
		"07_debug.log",
	}

	for _, filename := range expectedFiles {
		filePath := filepath.Join(cfg.LogsPath, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Log file %s was not created", filename)
		}
	}
}

func TestLogger_Success(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	// Log a success
	portDir := "devel/git"
	logger.Success(portDir)

	// Read success list log
	successPath := filepath.Join(cfg.LogsPath, "01_success_list.log")
	content, err := os.ReadFile(successPath)
	if err != nil {
		t.Fatalf("Failed to read success log: %v", err)
	}

	if !strings.Contains(string(content), portDir) {
		t.Errorf("Success log does not contain %s", portDir)
	}

	// Read results log
	resultsPath := filepath.Join(cfg.LogsPath, "00_last_results.log")
	content, err = os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("Failed to read results log: %v", err)
	}

	if !strings.Contains(string(content), "SUCCESS") {
		t.Error("Results log does not contain SUCCESS")
	}
	if !strings.Contains(string(content), portDir) {
		t.Errorf("Results log does not contain %s", portDir)
	}
}

func TestLogger_Failed(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	// Log a failure
	portDir := "www/nginx"
	phase := "configure"
	logger.Failed(portDir, phase)

	// Read failure list log
	failPath := filepath.Join(cfg.LogsPath, "02_failure_list.log")
	content, err := os.ReadFile(failPath)
	if err != nil {
		t.Fatalf("Failed to read failure log: %v", err)
	}

	if !strings.Contains(string(content), portDir) {
		t.Errorf("Failure log does not contain %s", portDir)
	}
	if !strings.Contains(string(content), phase) {
		t.Errorf("Failure log does not contain phase %s", phase)
	}

	// Read results log
	resultsPath := filepath.Join(cfg.LogsPath, "00_last_results.log")
	content, err = os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("Failed to read results log: %v", err)
	}

	if !strings.Contains(string(content), "FAILED") {
		t.Error("Results log does not contain FAILED")
	}
}

func TestLogger_Skipped(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	portDir := "editors/vim"
	logger.Skipped(portDir)

	// Read skipped list log
	skipPath := filepath.Join(cfg.LogsPath, "04_skipped_list.log")
	content, err := os.ReadFile(skipPath)
	if err != nil {
		t.Fatalf("Failed to read skipped log: %v", err)
	}

	if !strings.Contains(string(content), portDir) {
		t.Errorf("Skipped log does not contain %s", portDir)
	}
}

func TestLogger_Ignored(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	portDir := "graphics/blender"
	reason := "BROKEN: does not compile"
	logger.Ignored(portDir, reason)

	// Read ignored list log
	ignPath := filepath.Join(cfg.LogsPath, "03_ignored_list.log")
	content, err := os.ReadFile(ignPath)
	if err != nil {
		t.Fatalf("Failed to read ignored log: %v", err)
	}

	if !strings.Contains(string(content), portDir) {
		t.Errorf("Ignored log does not contain %s", portDir)
	}
	if !strings.Contains(string(content), reason) {
		t.Errorf("Ignored log does not contain reason %s", reason)
	}
}

func TestLogger_Abnormal(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	portDir := "lang/python"
	output := "Segmentation fault (core dumped)"
	logger.Abnormal(portDir, output)

	// Read abnormal log
	abnPath := filepath.Join(cfg.LogsPath, "05_abnormal_command_output.log")
	content, err := os.ReadFile(abnPath)
	if err != nil {
		t.Fatalf("Failed to read abnormal log: %v", err)
	}

	if !strings.Contains(string(content), portDir) {
		t.Errorf("Abnormal log does not contain %s", portDir)
	}
	if !strings.Contains(string(content), output) {
		t.Errorf("Abnormal log does not contain output %s", output)
	}
}

func TestLogger_Obsolete(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	pkgFile := "ruby-2.6.0.txz"
	logger.Obsolete(pkgFile)

	// Read obsolete log
	obsPath := filepath.Join(cfg.LogsPath, "06_obsolete_packages.log")
	content, err := os.ReadFile(obsPath)
	if err != nil {
		t.Fatalf("Failed to read obsolete log: %v", err)
	}

	if !strings.Contains(string(content), pkgFile) {
		t.Errorf("Obsolete log does not contain %s", pkgFile)
	}
}

func TestLogger_Debug(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	msg := "Debug message: checking dependency tree"
	logger.Debug(msg)

	// Read debug log
	debugPath := filepath.Join(cfg.LogsPath, "07_debug.log")
	content, err := os.ReadFile(debugPath)
	if err != nil {
		t.Fatalf("Failed to read debug log: %v", err)
	}

	if !strings.Contains(string(content), msg) {
		t.Errorf("Debug log does not contain message: %s", msg)
	}
}

func TestLogger_Error(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	msg := "Error: out of disk space"
	logger.Error(msg)

	// Should appear in both results and debug logs
	resultsPath := filepath.Join(cfg.LogsPath, "00_last_results.log")
	content, err := os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("Failed to read results log: %v", err)
	}

	if !strings.Contains(string(content), "ERROR") {
		t.Error("Results log does not contain ERROR")
	}
	if !strings.Contains(string(content), msg) {
		t.Errorf("Results log does not contain message: %s", msg)
	}

	debugPath := filepath.Join(cfg.LogsPath, "07_debug.log")
	content, err = os.ReadFile(debugPath)
	if err != nil {
		t.Fatalf("Failed to read debug log: %v", err)
	}

	if !strings.Contains(string(content), msg) {
		t.Errorf("Debug log does not contain message: %s", msg)
	}
}

func TestLogger_Info(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	msg := "Starting build process"
	logger.Info(msg)

	resultsPath := filepath.Join(cfg.LogsPath, "00_last_results.log")
	content, err := os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("Failed to read results log: %v", err)
	}

	if !strings.Contains(string(content), "INFO") {
		t.Error("Results log does not contain INFO")
	}
	if !strings.Contains(string(content), msg) {
		t.Errorf("Results log does not contain message: %s", msg)
	}
}

func TestLogger_WriteSummary(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	total := 100
	success := 85
	failed := 10
	skipped := 3
	ignored := 2
	duration := 45 * time.Minute

	logger.WriteSummary(total, success, failed, skipped, ignored, duration)

	resultsPath := filepath.Join(cfg.LogsPath, "00_last_results.log")
	content, err := os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("Failed to read results log: %v", err)
	}

	contentStr := string(content)

	// Check for summary section
	if !strings.Contains(contentStr, "BUILD SUMMARY") {
		t.Error("Summary does not contain BUILD SUMMARY header")
	}

	// Check for all counts
	expectedStrings := []string{
		"Total packages:",
		"Success:",
		"Failed:",
		"Skipped:",
		"Ignored:",
		"Duration:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("Summary does not contain %q", expected)
		}
	}
}

func TestLogger_Close(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// Close should not panic
	logger.Close()

	// Close again should not panic
	logger.Close()
}

func TestNewLogger_CreateDirError(t *testing.T) {
	// Try to create logger with invalid path (read-only parent)
	if os.Getuid() == 0 {
		t.Skip("Cannot test directory creation errors as root")
	}

	cfg := &config.Config{
		LogsPath: "/proc/invalid/logs", // /proc is read-only
	}

	_, err := NewLogger(cfg)
	if err == nil {
		t.Error("Expected error when creating logger in invalid directory")
	}
}

func TestLogger_ImplementsLibraryLogger(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	// Compile-time check: verify Logger implements LibraryLogger
	var _ LibraryLogger = logger

	// Test Info with formatting
	logger.Info("Build %s started for worker %d", "test-build", 5)
	resultsPath := filepath.Join(cfg.LogsPath, "00_last_results.log")
	content, err := os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("Failed to read results log: %v", err)
	}
	if !strings.Contains(string(content), "Build test-build started for worker 5") {
		t.Error("Info with formatting did not work correctly")
	}

	// Test Debug with formatting
	logger.Debug("Processing port %d of %d", 10, 100)
	debugPath := filepath.Join(cfg.LogsPath, "07_debug.log")
	content, err = os.ReadFile(debugPath)
	if err != nil {
		t.Fatalf("Failed to read debug log: %v", err)
	}
	if !strings.Contains(string(content), "Processing port 10 of 100") {
		t.Error("Debug with formatting did not work correctly")
	}

	// Test Error with formatting
	logger.Error("Failed to process %s: %s", "editors/vim", "timeout")
	content, err = os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("Failed to read results log: %v", err)
	}
	if !strings.Contains(string(content), "Failed to process editors/vim: timeout") {
		t.Error("Error with formatting did not work correctly")
	}
}

func TestLogger_Warn(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	// Test basic warning
	msg := "Package dependency not found"
	logger.Warn(msg)

	// Should appear in both results and debug logs
	resultsPath := filepath.Join(cfg.LogsPath, "00_last_results.log")
	content, err := os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("Failed to read results log: %v", err)
	}

	if !strings.Contains(string(content), "WARN") {
		t.Error("Results log does not contain WARN")
	}
	if !strings.Contains(string(content), msg) {
		t.Errorf("Results log does not contain message: %s", msg)
	}

	debugPath := filepath.Join(cfg.LogsPath, "07_debug.log")
	content, err = os.ReadFile(debugPath)
	if err != nil {
		t.Fatalf("Failed to read debug log: %v", err)
	}

	if !strings.Contains(string(content), msg) {
		t.Errorf("Debug log does not contain message: %s", msg)
	}

	// Test warning with formatting
	logger.Warn("Port %s has %d missing dependencies", "devel/git", 3)
	content, err = os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("Failed to read results log: %v", err)
	}
	if !strings.Contains(string(content), "Port devel/git has 3 missing dependencies") {
		t.Error("Warn with formatting did not work correctly")
	}
}
