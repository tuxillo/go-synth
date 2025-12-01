package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-synth/config"
)

func TestGetLogSummary(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	// Create logs directory
	os.MkdirAll(cfg.LogsPath, 0755)

	// Create success log with 3 entries (use # for header to be skipped)
	successPath := filepath.Join(cfg.LogsPath, "01_success_list.log")
	successContent := "# Header line\n\ndevel/git\nwww/nginx\nlang/python\n"
	os.WriteFile(successPath, []byte(successContent), 0644)

	// Create failure log with 2 entries
	failurePath := filepath.Join(cfg.LogsPath, "02_failure_list.log")
	failureContent := "# Header line\n\nshells/bash (phase: configure)\neditors/vim (phase: build)\n"
	os.WriteFile(failurePath, []byte(failureContent), 0644)

	// Create ignored log with 1 entry
	ignoredPath := filepath.Join(cfg.LogsPath, "03_ignored_list.log")
	ignoredContent := "# Header\n\ngraphics/blender: BROKEN\n"
	os.WriteFile(ignoredPath, []byte(ignoredContent), 0644)

	// Create skipped log with 1 entry
	skippedPath := filepath.Join(cfg.LogsPath, "04_skipped_list.log")
	skippedContent := "# Header\n\nnet/samba\n"
	os.WriteFile(skippedPath, []byte(skippedContent), 0644)

	// Get summary
	summary := GetLogSummary(cfg)

	// Verify counts (should skip empty lines and headers starting with #)
	if summary["success"] != 3 {
		t.Errorf("success count = %d, want 3", summary["success"])
	}
	if summary["failed"] != 2 {
		t.Errorf("failed count = %d, want 2", summary["failed"])
	}
	if summary["ignored"] != 1 {
		t.Errorf("ignored count = %d, want 1", summary["ignored"])
	}
	if summary["skipped"] != 1 {
		t.Errorf("skipped count = %d, want 1", summary["skipped"])
	}
}

func TestGetLogSummary_MissingFiles(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	// Don't create any log files
	os.MkdirAll(cfg.LogsPath, 0755)

	// Should return empty map (or zero values)
	summary := GetLogSummary(cfg)

	// Missing files should result in 0 counts
	if summary["success"] != 0 {
		t.Errorf("success count = %d, want 0 for missing file", summary["success"])
	}
}

func TestCountLines(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.log")

	tests := []struct {
		name        string
		content     string
		expectCount int
	}{
		{
			name:        "empty file",
			content:     "",
			expectCount: 0,
		},
		{
			name:        "single line",
			content:     "line1\n",
			expectCount: 1,
		},
		{
			name:        "multiple lines",
			content:     "line1\nline2\nline3\n",
			expectCount: 3,
		},
		{
			name:        "with empty lines",
			content:     "line1\n\nline2\n\nline3\n",
			expectCount: 3,
		},
		{
			name:        "with comment lines",
			content:     "line1\n# comment\nline2\n",
			expectCount: 2,
		},
		{
			name:        "whitespace only lines",
			content:     "line1\n   \nline2\n\t\n",
			expectCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			count, err := countLines(testFile)
			if err != nil {
				t.Fatalf("countLines failed: %v", err)
			}

			if count != tt.expectCount {
				t.Errorf("countLines() = %d, want %d", count, tt.expectCount)
			}
		})
	}
}

func TestCountLines_NonExistentFile(t *testing.T) {
	_, err := countLines("/nonexistent/file.log")
	if err == nil {
		t.Error("countLines should return error for non-existent file")
	}
}

func TestUsePager(t *testing.T) {
	// Save original PAGER
	originalPager := os.Getenv("PAGER")
	defer os.Setenv("PAGER", originalPager)

	tests := []struct {
		name         string
		pagerEnv     string
		shouldCreate bool
		expectPager  bool
	}{
		{
			name:         "default less",
			pagerEnv:     "",
			shouldCreate: false,
			expectPager:  false, // less might not exist in test environment
		},
		{
			name:         "nonexistent pager",
			pagerEnv:     "nonexistentpager",
			shouldCreate: false,
			expectPager:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pagerEnv != "" {
				os.Setenv("PAGER", tt.pagerEnv)
			} else {
				os.Unsetenv("PAGER")
			}

			result := usePager()
			// Just verify it doesn't panic
			_ = result
		})
	}
}

func TestListLogs(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	// Create logs directory
	os.MkdirAll(cfg.LogsPath, 0755)

	// Create some package logs
	logsDir := filepath.Join(cfg.LogsPath, "logs")
	os.MkdirAll(logsDir, 0755)
	os.WriteFile(filepath.Join(logsDir, "devel___git.log"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(logsDir, "www___nginx.log"), []byte("test"), 0644)

	// ListLogs prints to stdout - we just verify it doesn't panic
	// In a real test we'd capture stdout, but that's complex
	ListLogs(cfg)
}

func TestViewLog_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	os.MkdirAll(cfg.LogsPath, 0755)

	// Should not panic, should write error to stderr
	ViewLog(cfg, "nonexistent.log")
}

func TestViewPackageLog_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	os.MkdirAll(cfg.LogsPath, 0755)

	// Should not panic, should write error to stderr
	ViewPackageLog(cfg, "nonexistent/port")
}

func TestTailLog(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	os.MkdirAll(cfg.LogsPath, 0755)

	// Create test log file
	logPath := filepath.Join(cfg.LogsPath, "test.log")
	content := strings.Join([]string{
		"line1",
		"line2",
		"line3",
		"line4",
		"line5",
	}, "\n")
	os.WriteFile(logPath, []byte(content), 0644)

	// TailLog prints to stdout - we just verify it doesn't panic
	TailLog(cfg, "test.log", 3)
}

func TestTailLog_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	os.MkdirAll(cfg.LogsPath, 0755)

	// Should not panic, should write error to stderr
	TailLog(cfg, "nonexistent.log", 10)
}

func TestGrepLog(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	os.MkdirAll(cfg.LogsPath, 0755)

	// Create test log file
	logPath := filepath.Join(cfg.LogsPath, "test.log")
	content := strings.Join([]string{
		"normal line",
		"ERROR: something went wrong",
		"another normal line",
		"ERROR: another error",
	}, "\n")
	os.WriteFile(logPath, []byte(content), 0644)

	// GrepLog prints to stdout - we just verify it doesn't panic
	GrepLog(cfg, "test.log", "ERROR")
}

func TestGrepLog_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LogsPath: filepath.Join(tempDir, "logs"),
	}

	os.MkdirAll(cfg.LogsPath, 0755)

	// Should not panic, should write error to stderr
	GrepLog(cfg, "nonexistent.log", "pattern")
}
