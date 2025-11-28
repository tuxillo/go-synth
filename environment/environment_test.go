package environment

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestNew_ValidBackend(t *testing.T) {
	// Test mock backend (bsd backend tested in environment/bsd package)
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

func TestNew_InvalidBackend(t *testing.T) {
	env, err := New("nonexistent")
	if env != nil {
		t.Error("New(\"nonexistent\") should return nil environment")
	}

	if err == nil {
		t.Fatal("New(\"nonexistent\") should return error")
	}

	var unknownErr *ErrUnknownBackend
	if !errors.As(err, &unknownErr) {
		t.Errorf("error type = %T, want *ErrUnknownBackend", err)
	}

	if unknownErr.Backend != "nonexistent" {
		t.Errorf("ErrUnknownBackend.Backend = %q, want %q", unknownErr.Backend, "nonexistent")
	}
}

func TestRegister_Duplicate(t *testing.T) {
	// Trying to register "mock" again should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Register() with duplicate name should panic")
		}
	}()

	Register("mock", NewMockEnvironment)
}

func TestExecCommand_Fields(t *testing.T) {
	cmd := &ExecCommand{
		Command: "/usr/bin/make",
		Args:    []string{"install", "clean"},
		WorkDir: "/xports/editors/vim",
		Env:     map[string]string{"BATCH": "yes", "MAKEFLAGS": "-j8"},
		Stdout:  io.Discard,
		Stderr:  io.Discard,
		Timeout: 30 * time.Minute,
	}

	if cmd.Command != "/usr/bin/make" {
		t.Errorf("Command = %q, want /usr/bin/make", cmd.Command)
	}

	if len(cmd.Args) != 2 {
		t.Errorf("len(Args) = %d, want 2", len(cmd.Args))
	}

	if cmd.Args[0] != "install" {
		t.Errorf("Args[0] = %q, want install", cmd.Args[0])
	}

	if cmd.WorkDir != "/xports/editors/vim" {
		t.Errorf("WorkDir = %q, want /xports/editors/vim", cmd.WorkDir)
	}

	if cmd.Env["BATCH"] != "yes" {
		t.Errorf("Env[BATCH] = %q, want yes", cmd.Env["BATCH"])
	}

	if cmd.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", cmd.Timeout)
	}
}

func TestExecResult_Fields(t *testing.T) {
	err := errors.New("command failed")
	result := &ExecResult{
		ExitCode: 1,
		Duration: 5 * time.Second,
		Error:    err,
	}

	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}

	if result.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", result.Duration)
	}

	if result.Error != err {
		t.Errorf("Error = %v, want %v", result.Error, err)
	}
}

func TestErrUnknownBackend_Error(t *testing.T) {
	err := &ErrUnknownBackend{Backend: "test-backend"}

	msg := err.Error()
	if !strings.Contains(msg, "test-backend") {
		t.Errorf("Error() = %q, should contain backend name", msg)
	}

	expected := "unknown environment backend: test-backend"
	if msg != expected {
		t.Errorf("Error() = %q, want %q", msg, expected)
	}
}

func TestErrSetupFailed_Error(t *testing.T) {
	tests := []struct {
		name       string
		op         string
		err        error
		wantSubstr string
	}{
		{
			name:       "with operation",
			op:         "mkdir",
			err:        errors.New("permission denied"),
			wantSubstr: "setup failed (mkdir)",
		},
		{
			name:       "without operation",
			op:         "",
			err:        errors.New("generic error"),
			wantSubstr: "environment setup failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ErrSetupFailed{
				Op:  tt.op,
				Err: tt.err,
			}

			msg := err.Error()
			if !strings.Contains(msg, tt.wantSubstr) {
				t.Errorf("Error() = %q, should contain %q", msg, tt.wantSubstr)
			}
		})
	}
}

func TestErrSetupFailed_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &ErrSetupFailed{
		Op:  "test",
		Err: innerErr,
	}

	if unwrapped := err.Unwrap(); unwrapped != innerErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, innerErr)
	}

	// Test with errors.Is
	if !errors.Is(err, innerErr) {
		t.Error("errors.Is() should find inner error")
	}
}

func TestErrExecutionFailed_Error(t *testing.T) {
	tests := []struct {
		name       string
		op         string
		command    string
		exitCode   int
		err        error
		wantSubstr string
	}{
		{
			name:       "with exit code",
			op:         "chroot",
			command:    "/usr/bin/make",
			exitCode:   127,
			err:        errors.New("command not found"),
			wantSubstr: "exited with code 127",
		},
		{
			name:       "with operation",
			op:         "timeout",
			command:    "/bin/sleep",
			exitCode:   0,
			err:        errors.New("deadline exceeded"),
			wantSubstr: "timeout failed",
		},
		{
			name:       "without operation",
			op:         "",
			command:    "/usr/bin/test",
			exitCode:   0,
			err:        errors.New("generic error"),
			wantSubstr: "failed to execute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ErrExecutionFailed{
				Op:       tt.op,
				Command:  tt.command,
				ExitCode: tt.exitCode,
				Err:      tt.err,
			}

			msg := err.Error()
			if !strings.Contains(msg, tt.wantSubstr) {
				t.Errorf("Error() = %q, should contain %q", msg, tt.wantSubstr)
			}

			if !strings.Contains(msg, tt.command) {
				t.Errorf("Error() = %q, should contain command %q", msg, tt.command)
			}
		})
	}
}

func TestErrExecutionFailed_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &ErrExecutionFailed{
		Op:      "test",
		Command: "/bin/test",
		Err:     innerErr,
	}

	if unwrapped := err.Unwrap(); unwrapped != innerErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, innerErr)
	}

	// Test with errors.Is
	if !errors.Is(err, innerErr) {
		t.Error("errors.Is() should find inner error")
	}
}

func TestErrCleanupFailed_Error(t *testing.T) {
	tests := []struct {
		name       string
		op         string
		err        error
		mounts     []string
		wantSubstr string
	}{
		{
			name:       "with mounts",
			op:         "unmount",
			err:        errors.New("device busy"),
			mounts:     []string{"/build/SL01/dev", "/build/SL01/proc"},
			wantSubstr: "remaining mounts",
		},
		{
			name:       "with operation",
			op:         "rmdir",
			err:        errors.New("directory not empty"),
			mounts:     nil,
			wantSubstr: "cleanup failed (rmdir)",
		},
		{
			name:       "without operation",
			op:         "",
			err:        errors.New("generic error"),
			mounts:     nil,
			wantSubstr: "environment cleanup failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ErrCleanupFailed{
				Op:     tt.op,
				Err:    tt.err,
				Mounts: tt.mounts,
			}

			msg := err.Error()
			if !strings.Contains(msg, tt.wantSubstr) {
				t.Errorf("Error() = %q, should contain %q", msg, tt.wantSubstr)
			}

			if len(tt.mounts) > 0 {
				for _, mount := range tt.mounts {
					if !strings.Contains(msg, mount) {
						t.Errorf("Error() = %q, should contain mount %q", msg, mount)
					}
				}
			}
		})
	}
}

func TestErrCleanupFailed_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &ErrCleanupFailed{
		Op:  "test",
		Err: innerErr,
	}

	if unwrapped := err.Unwrap(); unwrapped != innerErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, innerErr)
	}

	// Test with errors.Is
	if !errors.Is(err, innerErr) {
		t.Error("errors.Is() should find inner error")
	}
}
