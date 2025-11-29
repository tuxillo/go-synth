package environment

import (
	"context"
	"dsynth/config"
	"dsynth/log"
	"sync"
	"time"
)

// MockEnvironment is a test implementation of Environment.
//
// MockEnvironment records all method calls and can be configured to return
// specific results or errors. It's thread-safe and suitable for testing
// concurrent code.
//
// Usage example:
//
//	mock := NewMockEnvironment()
//	mock.ExecuteResult = &ExecResult{ExitCode: 1}
//	mock.ExecuteError = errors.New("command failed")
//
//	result, err := mock.Execute(ctx, cmd)
//	// result.ExitCode == 1, err == "command failed"
//
//	if mock.GetExecuteCallCount() != 1 {
//	    t.Error("Execute not called")
//	}
type MockEnvironment struct {
	mu sync.Mutex

	// Setup tracking
	SetupCalled   bool
	SetupWorkerID int
	SetupConfig   *config.Config
	SetupError    error

	// Execute tracking
	ExecuteCalls  []*ExecCommand
	ExecuteResult *ExecResult
	ExecuteError  error

	// Cleanup tracking
	CleanupCalled bool
	CleanupError  error

	// GetBasePath return value
	BasePath string
}

// NewMockEnvironment creates a new mock environment with default values.
//
// Default values:
//   - BasePath: "/mock/base"
//   - ExecuteResult: &ExecResult{ExitCode: 0} (success)
//   - All errors: nil
func NewMockEnvironment() Environment {
	return &MockEnvironment{
		BasePath:      "/mock/base",
		ExecuteResult: &ExecResult{ExitCode: 0, Duration: 0},
	}
}

func init() {
	// Register mock backend for testing
	Register("mock", NewMockEnvironment)
}

// Setup records the Setup call and returns the configured error.
func (m *MockEnvironment) Setup(workerID int, cfg *config.Config, logger log.LibraryLogger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetupCalled = true
	m.SetupWorkerID = workerID
	m.SetupConfig = cfg

	return m.SetupError
}

// Execute records the Execute call and returns the configured result/error.
//
// The command is appended to ExecuteCalls for inspection.
// Returns a copy of ExecuteResult and ExecuteError.
func (m *MockEnvironment) Execute(ctx context.Context, cmd *ExecCommand) (*ExecResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.ExecuteCalls = append(m.ExecuteCalls, cmd)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return &ExecResult{
			ExitCode: -1,
			Duration: 0,
		}, ctx.Err()
	default:
	}

	// Return configured result
	if m.ExecuteResult != nil {
		// Return a copy to avoid sharing state
		result := &ExecResult{
			ExitCode: m.ExecuteResult.ExitCode,
			Duration: m.ExecuteResult.Duration,
			Error:    m.ExecuteResult.Error,
		}
		return result, m.ExecuteError
	}

	// Default: success
	return &ExecResult{ExitCode: 0, Duration: 0}, m.ExecuteError
}

// Cleanup records the Cleanup call and returns the configured error.
func (m *MockEnvironment) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CleanupCalled = true
	return m.CleanupError
}

// GetBasePath returns the configured base path.
func (m *MockEnvironment) GetBasePath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.BasePath
}

// GetExecuteCallCount returns the number of times Execute was called.
func (m *MockEnvironment) GetExecuteCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.ExecuteCalls)
}

// GetLastExecuteCall returns the most recent Execute call, or nil if none.
func (m *MockEnvironment) GetLastExecuteCall() *ExecCommand {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.ExecuteCalls) == 0 {
		return nil
	}
	return m.ExecuteCalls[len(m.ExecuteCalls)-1]
}

// GetExecuteCall returns the Execute call at the given index, or nil.
func (m *MockEnvironment) GetExecuteCall(index int) *ExecCommand {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.ExecuteCalls) {
		return nil
	}
	return m.ExecuteCalls[index]
}

// Reset clears all recorded calls and resets to default state.
//
// Useful for reusing the same mock across multiple test cases.
func (m *MockEnvironment) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetupCalled = false
	m.SetupWorkerID = 0
	m.SetupConfig = nil
	m.SetupError = nil

	m.ExecuteCalls = nil
	m.ExecuteResult = &ExecResult{ExitCode: 0, Duration: 0}
	m.ExecuteError = nil

	m.CleanupCalled = false
	m.CleanupError = nil
}

// WasSetupCalled returns true if Setup was called.
func (m *MockEnvironment) WasSetupCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.SetupCalled
}

// WasCleanupCalled returns true if Cleanup was called.
func (m *MockEnvironment) WasCleanupCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.CleanupCalled
}

// SimulateExecutionTime simulates command execution by sleeping.
//
// This is useful for testing timeout and cancellation behavior.
// The duration is added to ExecuteResult.Duration.
func (m *MockEnvironment) SimulateExecutionTime(d time.Duration) {
	time.Sleep(d)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ExecuteResult != nil {
		m.ExecuteResult.Duration += d
	}
}
