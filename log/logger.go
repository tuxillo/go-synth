package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dsynth/config"
)

// Logger manages multiple log files for dsynth
type Logger struct {
	cfg          *config.Config
	resultsFile  *os.File
	successFile  *os.File
	failureFile  *os.File
	ignoredFile  *os.File
	skippedFile  *os.File
	abnormalFile *os.File
	obsoleteFile *os.File
	debugFile    *os.File
	mu           sync.Mutex
}

// NewLogger creates a new logger
func NewLogger(cfg *config.Config) (*Logger, error) {
	// Ensure logs directory exists
	if err := os.MkdirAll(cfg.LogsPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	l := &Logger{cfg: cfg}

	// Open all log files
	var err error

	l.resultsFile, err = os.Create(filepath.Join(cfg.LogsPath, "00_last_results.log"))
	if err != nil {
		return nil, err
	}

	l.successFile, err = os.Create(filepath.Join(cfg.LogsPath, "01_success_list.log"))
	if err != nil {
		return nil, err
	}

	l.failureFile, err = os.Create(filepath.Join(cfg.LogsPath, "02_failure_list.log"))
	if err != nil {
		return nil, err
	}

	l.ignoredFile, err = os.Create(filepath.Join(cfg.LogsPath, "03_ignored_list.log"))
	if err != nil {
		return nil, err
	}

	l.skippedFile, err = os.Create(filepath.Join(cfg.LogsPath, "04_skipped_list.log"))
	if err != nil {
		return nil, err
	}

	l.abnormalFile, err = os.Create(filepath.Join(cfg.LogsPath, "05_abnormal_command_output.log"))
	if err != nil {
		return nil, err
	}

	l.obsoleteFile, err = os.Create(filepath.Join(cfg.LogsPath, "06_obsolete_packages.log"))
	if err != nil {
		return nil, err
	}

	l.debugFile, err = os.Create(filepath.Join(cfg.LogsPath, "07_debug.log"))
	if err != nil {
		return nil, err
	}

	// Write headers
	l.writeHeaders()

	return l, nil
}

// Close closes all log files
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.resultsFile != nil {
		l.resultsFile.Close()
	}
	if l.successFile != nil {
		l.successFile.Close()
	}
	if l.failureFile != nil {
		l.failureFile.Close()
	}
	if l.ignoredFile != nil {
		l.ignoredFile.Close()
	}
	if l.skippedFile != nil {
		l.skippedFile.Close()
	}
	if l.abnormalFile != nil {
		l.abnormalFile.Close()
	}
	if l.obsoleteFile != nil {
		l.obsoleteFile.Close()
	}
	if l.debugFile != nil {
		l.debugFile.Close()
	}
}

// writeHeaders writes initial headers to log files
func (l *Logger) writeHeaders() {
	timestamp := time.Now().Format(time.RFC3339)

	fmt.Fprintf(l.resultsFile, "dsynth build log - %s\n", timestamp)
	fmt.Fprintf(l.resultsFile, "%s\n\n", strings.Repeat("=", 70))

	fmt.Fprintf(l.successFile, "Successful builds - %s\n\n", timestamp)
	fmt.Fprintf(l.failureFile, "Failed builds - %s\n\n", timestamp)
	fmt.Fprintf(l.ignoredFile, "Ignored packages - %s\n\n", timestamp)
	fmt.Fprintf(l.skippedFile, "Skipped packages - %s\n\n", timestamp)
	fmt.Fprintf(l.abnormalFile, "Abnormal output - %s\n\n", timestamp)
	fmt.Fprintf(l.obsoleteFile, "Obsolete packages - %s\n\n", timestamp)
	fmt.Fprintf(l.debugFile, "Debug log - %s\n\n", timestamp)
}

// Success logs a successful build
func (l *Logger) Success(portDir string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf("[%s] SUCCESS: %s\n", timestamp, portDir)

	l.resultsFile.WriteString(msg)
	l.successFile.WriteString(portDir + "\n")

	l.resultsFile.Sync()
	l.successFile.Sync()
}

// Failed logs a failed build
func (l *Logger) Failed(portDir, phase string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf("[%s] FAILED: %s (phase: %s)\n", timestamp, portDir, phase)

	l.resultsFile.WriteString(msg)
	l.failureFile.WriteString(fmt.Sprintf("%s (phase: %s)\n", portDir, phase))

	l.resultsFile.Sync()
	l.failureFile.Sync()
}

// Skipped logs a skipped package
func (l *Logger) Skipped(portDir string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf("[%s] SKIPPED: %s\n", timestamp, portDir)

	l.resultsFile.WriteString(msg)
	l.skippedFile.WriteString(portDir + "\n")

	l.resultsFile.Sync()
	l.skippedFile.Sync()
}

// Ignored logs an ignored package
func (l *Logger) Ignored(portDir, reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf("[%s] IGNORED: %s (%s)\n", timestamp, portDir, reason)

	l.resultsFile.WriteString(msg)
	l.ignoredFile.WriteString(fmt.Sprintf("%s: %s\n", portDir, reason))

	l.resultsFile.Sync()
	l.ignoredFile.Sync()
}

// Abnormal logs abnormal command output
func (l *Logger) Abnormal(portDir, output string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf("[%s] ABNORMAL: %s\n%s\n\n", timestamp, portDir, output)

	l.abnormalFile.WriteString(msg)
	l.abnormalFile.Sync()
}

// Obsolete logs an obsolete package
func (l *Logger) Obsolete(pkgFile string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.obsoleteFile.WriteString(pkgFile + "\n")
	l.obsoleteFile.Sync()
}

// Debug logs debug information
func (l *Logger) Debug(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	l.debugFile.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
	l.debugFile.Sync()
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	errMsg := fmt.Sprintf("[%s] ERROR: %s\n", timestamp, msg)

	l.resultsFile.WriteString(errMsg)
	l.debugFile.WriteString(errMsg)

	l.resultsFile.Sync()
	l.debugFile.Sync()
}

// Info logs an informational message
func (l *Logger) Info(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	l.resultsFile.WriteString(fmt.Sprintf("[%s] INFO: %s\n", timestamp, msg))
	l.resultsFile.Sync()
}

// WriteSummary writes a summary to the results log
func (l *Logger) WriteSummary(total, success, failed, skipped, ignored int, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprintf(l.resultsFile, "\n%s\n", strings.Repeat("=", 70))
	fmt.Fprintf(l.resultsFile, "BUILD SUMMARY\n")
	fmt.Fprintf(l.resultsFile, "%s\n", strings.Repeat("=", 70))
	fmt.Fprintf(l.resultsFile, "Total packages:    %d\n", total)
	fmt.Fprintf(l.resultsFile, "Success:           %d\n", success)
	fmt.Fprintf(l.resultsFile, "Failed:            %d\n", failed)
	fmt.Fprintf(l.resultsFile, "Skipped:           %d\n", skipped)
	fmt.Fprintf(l.resultsFile, "Ignored:           %d\n", ignored)
	fmt.Fprintf(l.resultsFile, "Duration:          %s\n", duration)
	fmt.Fprintf(l.resultsFile, "%s\n", strings.Repeat("=", 70))

	l.resultsFile.Sync()
}
