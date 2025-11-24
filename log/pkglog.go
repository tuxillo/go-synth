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

// PackageLogger logs build output for a specific package
type PackageLogger struct {
	cfg     *config.Config
	portDir string
	file    *os.File
	mu      sync.Mutex
}

// NewPackageLogger creates a new package logger
func NewPackageLogger(cfg *config.Config, portDir string) *PackageLogger {
	// Convert category/name to category___name format
	logFileName := strings.ReplaceAll(portDir, "/", "___") + ".log"
	logFile := filepath.Join(cfg.LogsPath, logFileName)
	
	file, err := os.Create(logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create package log: %v\n", err)
		return &PackageLogger{
			cfg:     cfg,
			portDir: portDir,
			file:    nil,
		}
	}

	return &PackageLogger{
		cfg:     cfg,
		portDir: portDir,
		file:    file,
	}
}

// Close closes the package logger
func (pl *PackageLogger) Close() {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.file != nil {
		pl.file.Close()
	}
}

// WriteHeader writes the log header
func (pl *PackageLogger) WriteHeader() {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.file == nil {
		return
	}

	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	fmt.Fprintf(pl.file, "Build Log: %s\n", pl.portDir)
	fmt.Fprintf(pl.file, "Started: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(pl.file, "%s\n\n", strings.Repeat("=", 70))
	pl.file.Sync()
}

// WritePhase writes a phase header
func (pl *PackageLogger) WritePhase(phase string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.file == nil {
		return
	}

	fmt.Fprintf(pl.file, "\n")
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("-", 70))
	fmt.Fprintf(pl.file, "Phase: %s\n", phase)
	fmt.Fprintf(pl.file, "Time: %s\n", time.Now().Format("15:04:05"))
	fmt.Fprintf(pl.file, "%s\n\n", strings.Repeat("-", 70))
	pl.file.Sync()
}

// Write writes output to the log
func (pl *PackageLogger) Write(output []byte) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.file == nil {
		return
	}

	pl.file.Write(output)
	pl.file.Sync()
}

// WriteString writes a string to the log
func (pl *PackageLogger) WriteString(s string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.file == nil {
		return
	}

	pl.file.WriteString(s)
	pl.file.Sync()
}

// WriteCommand writes a command being executed
func (pl *PackageLogger) WriteCommand(cmd string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.file == nil {
		return
	}

	fmt.Fprintf(pl.file, ">>> %s\n", cmd)
	pl.file.Sync()
}

// WriteWarning writes a warning message
func (pl *PackageLogger) WriteWarning(msg string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.file == nil {
		return
	}

	fmt.Fprintf(pl.file, "WARNING: %s\n", msg)
	pl.file.Sync()
}

// WriteError writes an error message
func (pl *PackageLogger) WriteError(msg string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.file == nil {
		return
	}

	fmt.Fprintf(pl.file, "ERROR: %s\n", msg)
	pl.file.Sync()
}

// WriteSuccess writes a success message
func (pl *PackageLogger) WriteSuccess(duration time.Duration) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.file == nil {
		return
	}

	fmt.Fprintf(pl.file, "\n")
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	fmt.Fprintf(pl.file, "BUILD SUCCESS\n")
	fmt.Fprintf(pl.file, "Completed: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(pl.file, "Duration: %s\n", duration)
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	pl.file.Sync()
}

// WriteFailure writes a failure message
func (pl *PackageLogger) WriteFailure(duration time.Duration, reason string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.file == nil {
		return
	}

	fmt.Fprintf(pl.file, "\n")
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	fmt.Fprintf(pl.file, "BUILD FAILED\n")
	fmt.Fprintf(pl.file, "Reason: %s\n", reason)
	fmt.Fprintf(pl.file, "Completed: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(pl.file, "Duration: %s\n", duration)
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	pl.file.Sync()
}