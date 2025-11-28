package environment

import (
	"context"
	"dsynth/config"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMockEnvironment_Interface(t *testing.T) {
	// Compile-time check that MockEnvironment implements Environment
	var _ Environment = (*MockEnvironment)(nil)
}

func TestNewMockEnvironment(t *testing.T) {
	mock := NewMockEnvironment()

	if mock == nil {
		t.Fatal("NewMockEnvironment() returned nil")
	}

	m := mock.(*MockEnvironment)

	if m.BasePath != "/mock/base" {
		t.Errorf("BasePath = %q, want %q", m.BasePath, "/mock/base")
	}

	if m.ExecuteResult == nil {
		t.Fatal("ExecuteResult is nil")
	}

	if m.ExecuteResult.ExitCode != 0 {
		t.Errorf("ExecuteResult.ExitCode = %d, want 0", m.ExecuteResult.ExitCode)
	}
}

func TestMockEnvironment_Setup(t *testing.T) {
	mock := NewMockEnvironment().(*MockEnvironment)
	cfg := &config.Config{BuildBase: "/test"}

	// Test successful setup
	err := mock.Setup(42, cfg)
	if err != nil {
		t.Errorf("Setup() error = %v, want nil", err)
	}

	if !mock.WasSetupCalled() {
		t.Error("Setup() not called")
	}

	if mock.SetupWorkerID != 42 {
		t.Errorf("SetupWorkerID = %d, want 42", mock.SetupWorkerID)
	}

	if mock.SetupConfig != cfg {
		t.Error("SetupConfig not recorded")
	}

	// Test setup with error
	mock.Reset()
	expectedErr := errors.New("setup failed")
	mock.SetupError = expectedErr

	err = mock.Setup(1, cfg)
	if err != expectedErr {
		t.Errorf("Setup() error = %v, want %v", err, expectedErr)
	}
}

func TestMockEnvironment_Execute(t *testing.T) {
	mock := NewMockEnvironment().(*MockEnvironment)
	ctx := context.Background()

	cmd := &ExecCommand{
		Command: "/usr/bin/make",
		Args:    []string{"install"},
	}

	// Test successful execution
	result, err := mock.Execute(ctx, cmd)
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0", result.ExitCode)
	}

	if mock.GetExecuteCallCount() != 1 {
		t.Errorf("GetExecuteCallCount() = %d, want 1", mock.GetExecuteCallCount())
	}

	lastCall := mock.GetLastExecuteCall()
	if lastCall == nil {
		t.Fatal("GetLastExecuteCall() returned nil")
	}

	if lastCall.Command != cmd.Command {
		t.Errorf("LastCall.Command = %q, want %q", lastCall.Command, cmd.Command)
	}

	// Test execution with error
	mock.Reset()
	expectedErr := errors.New("exec failed")
	mock.ExecuteError = expectedErr
	mock.ExecuteResult = &ExecResult{ExitCode: 1}

	result, err = mock.Execute(ctx, cmd)
	if err != expectedErr {
		t.Errorf("Execute() error = %v, want %v", err, expectedErr)
	}

	if result.ExitCode != 1 {
		t.Errorf("Execute() exit code = %d, want 1", result.ExitCode)
	}
}

func TestMockEnvironment_Execute_ContextCancellation(t *testing.T) {
	mock := NewMockEnvironment().(*MockEnvironment)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cmd := &ExecCommand{Command: "/bin/sleep", Args: []string{"10"}}

	result, err := mock.Execute(ctx, cmd)
	if err != context.Canceled {
		t.Errorf("Execute() error = %v, want %v", err, context.Canceled)
	}

	if result.ExitCode != -1 {
		t.Errorf("Execute() exit code = %d, want -1", result.ExitCode)
	}
}

func TestMockEnvironment_Execute_ContextTimeout(t *testing.T) {
	mock := NewMockEnvironment().(*MockEnvironment)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure timeout

	cmd := &ExecCommand{Command: "/bin/sleep", Args: []string{"10"}}

	result, err := mock.Execute(ctx, cmd)
	if err != context.DeadlineExceeded {
		t.Errorf("Execute() error = %v, want %v", err, context.DeadlineExceeded)
	}

	if result.ExitCode != -1 {
		t.Errorf("Execute() exit code = %d, want -1", result.ExitCode)
	}
}

func TestMockEnvironment_Cleanup(t *testing.T) {
	mock := NewMockEnvironment().(*MockEnvironment)

	// Test successful cleanup
	err := mock.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error = %v, want nil", err)
	}

	if !mock.WasCleanupCalled() {
		t.Error("Cleanup() not called")
	}

	// Test cleanup with error
	mock.Reset()
	expectedErr := errors.New("cleanup failed")
	mock.CleanupError = expectedErr

	err = mock.Cleanup()
	if err != expectedErr {
		t.Errorf("Cleanup() error = %v, want %v", err, expectedErr)
	}
}

func TestMockEnvironment_GetBasePath(t *testing.T) {
	mock := NewMockEnvironment().(*MockEnvironment)

	if got := mock.GetBasePath(); got != "/mock/base" {
		t.Errorf("GetBasePath() = %q, want %q", got, "/mock/base")
	}

	// Test custom base path
	mock.BasePath = "/custom/path"
	if got := mock.GetBasePath(); got != "/custom/path" {
		t.Errorf("GetBasePath() = %q, want %q", got, "/custom/path")
	}
}

func TestMockEnvironment_GetExecuteCall(t *testing.T) {
	mock := NewMockEnvironment().(*MockEnvironment)
	ctx := context.Background()

	// No calls yet
	if call := mock.GetExecuteCall(0); call != nil {
		t.Error("GetExecuteCall(0) should return nil when no calls")
	}

	// Add some calls
	cmd1 := &ExecCommand{Command: "/bin/echo", Args: []string{"hello"}}
	cmd2 := &ExecCommand{Command: "/bin/ls", Args: []string{"-la"}}

	mock.Execute(ctx, cmd1)
	mock.Execute(ctx, cmd2)

	// Test valid indices
	if call := mock.GetExecuteCall(0); call == nil || call.Command != "/bin/echo" {
		t.Error("GetExecuteCall(0) incorrect")
	}

	if call := mock.GetExecuteCall(1); call == nil || call.Command != "/bin/ls" {
		t.Error("GetExecuteCall(1) incorrect")
	}

	// Test invalid indices
	if call := mock.GetExecuteCall(-1); call != nil {
		t.Error("GetExecuteCall(-1) should return nil")
	}

	if call := mock.GetExecuteCall(10); call != nil {
		t.Error("GetExecuteCall(10) should return nil")
	}
}

func TestMockEnvironment_Reset(t *testing.T) {
	mock := NewMockEnvironment().(*MockEnvironment)
	ctx := context.Background()
	cfg := &config.Config{}

	// Make some calls
	mock.Setup(1, cfg)
	mock.Execute(ctx, &ExecCommand{Command: "/bin/test"})
	mock.Cleanup()

	// Verify state before reset
	if !mock.WasSetupCalled() {
		t.Error("Setup should be called before reset")
	}
	if mock.GetExecuteCallCount() != 1 {
		t.Error("Execute should be called before reset")
	}
	if !mock.WasCleanupCalled() {
		t.Error("Cleanup should be called before reset")
	}

	// Reset
	mock.Reset()

	// Verify state after reset
	if mock.WasSetupCalled() {
		t.Error("Setup should not be called after reset")
	}
	if mock.GetExecuteCallCount() != 0 {
		t.Errorf("GetExecuteCallCount() = %d after reset, want 0", mock.GetExecuteCallCount())
	}
	if mock.WasCleanupCalled() {
		t.Error("Cleanup should not be called after reset")
	}
	if mock.SetupWorkerID != 0 {
		t.Error("SetupWorkerID not reset")
	}
}

func TestMockEnvironment_Concurrent(t *testing.T) {
	mock := NewMockEnvironment().(*MockEnvironment)
	ctx := context.Background()

	// Test concurrent Execute calls
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			cmd := &ExecCommand{
				Command: "/bin/echo",
				Args:    []string{"hello"},
			}
			mock.Execute(ctx, cmd)
		}(i)
	}

	wg.Wait()

	if count := mock.GetExecuteCallCount(); count != numGoroutines {
		t.Errorf("GetExecuteCallCount() = %d, want %d", count, numGoroutines)
	}
}

func TestMockEnvironment_Registration(t *testing.T) {
	// Verify mock backend is registered
	env, err := New("mock")
	if err != nil {
		t.Fatalf("New(\"mock\") error = %v, want nil", err)
	}

	if env == nil {
		t.Fatal("New(\"mock\") returned nil")
	}

	// Verify it's a MockEnvironment
	if _, ok := env.(*MockEnvironment); !ok {
		t.Errorf("New(\"mock\") returned %T, want *MockEnvironment", env)
	}
}
