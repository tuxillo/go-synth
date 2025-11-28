package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dsynth/config"
)

func TestNewPackageLogger(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	// Create logs directory
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "devel/git"
	pl := NewPackageLogger(cfg, portDir)
	defer pl.Close()

	// Verify log file was created
	expectedPath := filepath.Join(cfg.LogsPath, "devel___git.log")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Package log file was not created at %s", expectedPath)
	}
}

func TestPackageLogger_WriteHeader(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "www/nginx"
	pl := NewPackageLogger(cfg, portDir)
	defer pl.Close()

	pl.WriteHeader()

	// Read log file
	logPath := filepath.Join(cfg.LogsPath, "www___nginx.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Build Log") {
		t.Error("Header does not contain 'Build Log'")
	}
	if !strings.Contains(contentStr, portDir) {
		t.Errorf("Header does not contain port directory %s", portDir)
	}
	if !strings.Contains(contentStr, "Started:") {
		t.Error("Header does not contain 'Started:'")
	}
}

func TestPackageLogger_WritePhase(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "editors/vim"
	pl := NewPackageLogger(cfg, portDir)
	defer pl.Close()

	phase := "configure"
	pl.WritePhase(phase)

	// Read log file
	logPath := filepath.Join(cfg.LogsPath, "editors___vim.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Phase:") {
		t.Error("Log does not contain 'Phase:'")
	}
	if !strings.Contains(contentStr, phase) {
		t.Errorf("Log does not contain phase %s", phase)
	}
}

func TestPackageLogger_Write(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "lang/python"
	pl := NewPackageLogger(cfg, portDir)
	defer pl.Close()

	output := []byte("Build output line 1\nBuild output line 2\n")
	pl.Write(output)

	// Read log file
	logPath := filepath.Join(cfg.LogsPath, "lang___python.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if string(content) != string(output) {
		t.Errorf("Log content = %q, want %q", string(content), string(output))
	}
}

func TestPackageLogger_WriteString(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "databases/postgresql"
	pl := NewPackageLogger(cfg, portDir)
	defer pl.Close()

	msg := "Configuration complete\n"
	pl.WriteString(msg)

	// Read log file
	logPath := filepath.Join(cfg.LogsPath, "databases___postgresql.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if string(content) != msg {
		t.Errorf("Log content = %q, want %q", string(content), msg)
	}
}

func TestPackageLogger_WriteCommand(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "net/curl"
	pl := NewPackageLogger(cfg, portDir)
	defer pl.Close()

	cmd := "./configure --prefix=/usr/local"
	pl.WriteCommand(cmd)

	// Read log file
	logPath := filepath.Join(cfg.LogsPath, "net___curl.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, ">>>") {
		t.Error("Command log does not contain '>>>' prefix")
	}
	if !strings.Contains(contentStr, cmd) {
		t.Errorf("Command log does not contain command %s", cmd)
	}
}

func TestPackageLogger_WriteWarning(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "security/openssl"
	pl := NewPackageLogger(cfg, portDir)
	defer pl.Close()

	warning := "Deprecated function used"
	pl.WriteWarning(warning)

	// Read log file
	logPath := filepath.Join(cfg.LogsPath, "security___openssl.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "WARNING:") {
		t.Error("Log does not contain 'WARNING:' prefix")
	}
	if !strings.Contains(contentStr, warning) {
		t.Errorf("Log does not contain warning %s", warning)
	}
}

func TestPackageLogger_WriteError(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "multimedia/ffmpeg"
	pl := NewPackageLogger(cfg, portDir)
	defer pl.Close()

	errMsg := "Compilation failed"
	pl.WriteError(errMsg)

	// Read log file
	logPath := filepath.Join(cfg.LogsPath, "multimedia___ffmpeg.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "ERROR:") {
		t.Error("Log does not contain 'ERROR:' prefix")
	}
	if !strings.Contains(contentStr, errMsg) {
		t.Errorf("Log does not contain error %s", errMsg)
	}
}

func TestPackageLogger_WriteSuccess(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "shells/bash"
	pl := NewPackageLogger(cfg, portDir)
	defer pl.Close()

	duration := 2 * time.Minute
	pl.WriteSuccess(duration)

	// Read log file
	logPath := filepath.Join(cfg.LogsPath, "shells___bash.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "BUILD SUCCESS") {
		t.Error("Log does not contain 'BUILD SUCCESS'")
	}
	if !strings.Contains(contentStr, "Completed:") {
		t.Error("Log does not contain 'Completed:'")
	}
	if !strings.Contains(contentStr, "Duration:") {
		t.Error("Log does not contain 'Duration:'")
	}
}

func TestPackageLogger_WriteFailure(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "x11/xorg"
	pl := NewPackageLogger(cfg, portDir)
	defer pl.Close()

	duration := 5 * time.Minute
	reason := "Missing dependency"
	pl.WriteFailure(duration, reason)

	// Read log file
	logPath := filepath.Join(cfg.LogsPath, "x11___xorg.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "BUILD FAILED") {
		t.Error("Log does not contain 'BUILD FAILED'")
	}
	if !strings.Contains(contentStr, "Reason:") {
		t.Error("Log does not contain 'Reason:'")
	}
	if !strings.Contains(contentStr, reason) {
		t.Errorf("Log does not contain reason %s", reason)
	}
	if !strings.Contains(contentStr, "Duration:") {
		t.Error("Log does not contain 'Duration:'")
	}
}

func TestPackageLogger_Close(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	portDir := "devel/make"
	pl := NewPackageLogger(cfg, portDir)

	// Close should not panic
	pl.Close()

	// Close again should not panic
	pl.Close()
}

func TestPackageLogger_NilFile(t *testing.T) {
	// Test that operations don't panic when file is nil
	pl := &PackageLogger{
		cfg:     &config.Config{LogsPath: "/tmp"},
		portDir: "test/port",
		file:    nil,
	}

	// None of these should panic
	pl.WriteHeader()
	pl.WritePhase("test")
	pl.Write([]byte("test"))
	pl.WriteString("test")
	pl.WriteCommand("test")
	pl.WriteWarning("test")
	pl.WriteError("test")
	pl.WriteSuccess(time.Second)
	pl.WriteFailure(time.Second, "test")
	pl.Close()
}

func TestPackageLogger_FileNameConversion(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}
	os.MkdirAll(cfg.LogsPath, 0755)

	tests := []struct {
		portDir      string
		expectedFile string
	}{
		{"devel/git", "devel___git.log"},
		{"www/nginx", "www___nginx.log"},
		{"x11/gnome-desktop", "x11___gnome-desktop.log"},
	}

	for _, tt := range tests {
		t.Run(tt.portDir, func(t *testing.T) {
			pl := NewPackageLogger(cfg, tt.portDir)
			defer pl.Close()

			pl.WriteString("test\n")

			expectedPath := filepath.Join(cfg.LogsPath, tt.expectedFile)
			if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
				t.Errorf("Expected log file %s does not exist", expectedPath)
			}
		})
	}
}
