package build

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-synth/config"
	"go-synth/environment"
	"go-synth/log"
)

// mockEnv is a fake environment.Environment for testing
type mockEnv struct {
	setupCalled   bool
	cleanupCalled bool
	executeCalls  []mockExecCall
	executeDelay  time.Duration                                                                            // Delay to simulate work
	executeErr    error                                                                                    // Error to return from Execute
	executeFunc   func(ctx context.Context, cmd *environment.ExecCommand) (*environment.ExecResult, error) // Custom execute function
	mu            sync.Mutex
}

type mockExecCall struct {
	cmd     environment.ExecCommand
	called  time.Time
	success bool
}

func newMockEnv() *mockEnv {
	return &mockEnv{
		executeCalls: make([]mockExecCall, 0),
	}
}

func (m *mockEnv) Setup(workerID int, cfg *config.Config, logger log.LibraryLogger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setupCalled = true
	return nil
}

func (m *mockEnv) Execute(ctx context.Context, cmd *environment.ExecCommand) (*environment.ExecResult, error) {
	// If custom execute function is provided, use it
	if m.executeFunc != nil {
		return m.executeFunc(ctx, cmd)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate work and check for cancellation
	if m.executeDelay > 0 {
		select {
		case <-ctx.Done():
			// Context cancelled during delay
			m.executeCalls = append(m.executeCalls, mockExecCall{
				cmd:     *cmd,
				called:  time.Now(),
				success: false,
			})
			return nil, ctx.Err()
		case <-time.After(m.executeDelay):
			// Delay completed
		}
	}

	success := m.executeErr == nil
	m.executeCalls = append(m.executeCalls, mockExecCall{
		cmd:     *cmd,
		called:  time.Now(),
		success: success,
	})

	if m.executeErr != nil {
		return &environment.ExecResult{
			ExitCode: 1,
			Duration: m.executeDelay,
		}, m.executeErr
	}

	return &environment.ExecResult{
		ExitCode: 0,
		Duration: m.executeDelay,
	}, nil
}

func (m *mockEnv) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupCalled = true
	return nil
}

func (m *mockEnv) GetExecuteCalls() []mockExecCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mockExecCall(nil), m.executeCalls...)
}

func (m *mockEnv) WasSetupCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setupCalled
}

func (m *mockEnv) WasCleanupCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cleanupCalled
}

func (m *mockEnv) GetBasePath() string {
	return "/tmp/mock-worker"
}

// mockLogger is a fake log.Logger for testing
type mockLogger struct {
	entries []mockLogEntry
	mu      sync.Mutex
}

type mockLogEntry struct {
	level   string
	message string
	time    time.Time
}

func newMockLogger() *mockLogger {
	return &mockLogger{
		entries: make([]mockLogEntry, 0),
	}
}

func (m *mockLogger) Debug(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, mockLogEntry{
		level:   "DEBUG",
		message: fmt.Sprintf(format, args...),
		time:    time.Now(),
	})
}

func (m *mockLogger) Info(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, mockLogEntry{
		level:   "INFO",
		message: fmt.Sprintf(format, args...),
		time:    time.Now(),
	})
}

func (m *mockLogger) Warn(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, mockLogEntry{
		level:   "WARN",
		message: fmt.Sprintf(format, args...),
		time:    time.Now(),
	})
}

func (m *mockLogger) Error(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, mockLogEntry{
		level:   "ERROR",
		message: fmt.Sprintf(format, args...),
		time:    time.Now(),
	})
}

func (m *mockLogger) Success(portDir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, mockLogEntry{
		level:   "SUCCESS",
		message: portDir,
		time:    time.Now(),
	})
}

func (m *mockLogger) Failed(portDir, phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, mockLogEntry{
		level:   "FAILED",
		message: fmt.Sprintf("%s (phase: %s)", portDir, phase),
		time:    time.Now(),
	})
}

func (m *mockLogger) Skipped(portDir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, mockLogEntry{
		level:   "SKIPPED",
		message: portDir,
		time:    time.Now(),
	})
}

func (m *mockLogger) WithContext(ctx log.LogContext) *mockContextLogger {
	// Return a simple wrapper that forwards to this logger
	return &mockContextLogger{parent: m, ctx: ctx}
}

func (m *mockLogger) Close() error {
	return nil
}

func (m *mockLogger) GetEntries() []mockLogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mockLogEntry(nil), m.entries...)
}

// mockContextLogger wraps mockLogger to provide log.ContextLogger interface
type mockContextLogger struct {
	parent *mockLogger
	ctx    log.LogContext
}

func (m *mockContextLogger) Debug(format string, args ...interface{}) {
	m.parent.Debug(format, args...)
}

func (m *mockContextLogger) Info(format string, args ...interface{}) {
	m.parent.Info(format, args...)
}

func (m *mockContextLogger) Warn(format string, args ...interface{}) {
	m.parent.Warn(format, args...)
}

func (m *mockContextLogger) Error(format string, args ...interface{}) {
	m.parent.Error(format, args...)
}

func (m *mockContextLogger) Success(message string) {
	m.parent.Info("SUCCESS: %s", message)
}

func (m *mockContextLogger) Failed(phase, message string) {
	m.parent.Error("FAILED phase=%s: %s", phase, message)
}
