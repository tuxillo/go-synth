package log

import "fmt"

// LibraryLogger is a minimal interface for library packages that need to
// output progress/diagnostics without depending on specific log file formats
// or terminal output.
//
// This interface allows libraries to be reusable in different contexts:
// - CLI tools (stdout/file logging)
// - REST APIs (structured logging)
// - Tests (memory/silent logging)
// - GUIs (event-based logging)
type LibraryLogger interface {
	// Info logs informational messages (e.g., "Resolving dependencies...")
	Info(format string, args ...any)

	// Debug logs debug/diagnostic messages (may be no-op in production)
	Debug(format string, args ...any)

	// Warn logs warning messages (non-fatal issues)
	Warn(format string, args ...any)

	// Error logs error messages (failures, but execution continues)
	Error(format string, args ...any)
}

// NoOpLogger discards all log messages.
// Useful for tests, silent mode, or when logging is not needed.
type NoOpLogger struct{}

func (NoOpLogger) Info(format string, args ...any)  {}
func (NoOpLogger) Debug(format string, args ...any) {}
func (NoOpLogger) Warn(format string, args ...any)  {}
func (NoOpLogger) Error(format string, args ...any) {}

// StdoutLogger prints all messages to stdout with severity prefix.
// Useful for CLI debugging and development.
type StdoutLogger struct{}

func (StdoutLogger) Info(format string, args ...any) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (StdoutLogger) Debug(format string, args ...any) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}

func (StdoutLogger) Warn(format string, args ...any) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}

func (StdoutLogger) Error(format string, args ...any) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}
