// Package environment provides abstractions for isolated build execution.
//
// The Environment interface enables build phases to execute in isolated
// environments without knowing implementation details. This allows:
//   - Testing via mock implementations (no root required)
//   - Multiple platform support (chroot, jails)
//   - Clean separation of concerns (build logic vs isolation)
//
// Supported backends:
//   - "bsd": chroot with nullfs/tmpfs (DragonFlyBSD/FreeBSD)
//   - "mock": testing backend (no actual isolation)
//
// Future backends (platform-specific):
//   - "jail": FreeBSD jails (FreeBSD-specific features)
//   - "jail": DragonFly jails (DragonFly-specific features)
//
// Note: FreeBSD and DragonFly jails have different implementations and
// feature sets. When jail support is added, platform-specific implementations
// will be required.
//
// Usage example:
//
//	env, err := environment.New("bsd")
//	if err != nil {
//	    return err
//	}
//	defer env.Cleanup()
//
//	if err := env.Setup(workerID, cfg); err != nil {
//	    return err
//	}
//
//	cmd := &environment.ExecCommand{
//	    Command: "/usr/bin/make",
//	    Args:    []string{"install"},
//	    WorkDir: "/xports/editors/vim",
//	    Stdout:  os.Stdout,
//	    Stderr:  os.Stderr,
//	}
//
//	result, err := env.Execute(ctx, cmd)
//	if err != nil {
//	    return err
//	}
package environment

import (
	"context"
	"dsynth/config"
	"dsynth/log"
	"fmt"
	"io"
	"time"
)

// Environment provides isolated execution for build phases.
//
// Implementations must handle:
//   - Directory creation and cleanup
//   - Filesystem isolation (mounts for BSD, other mechanisms for other platforms)
//   - Process isolation (chroot, jails, etc.)
//   - Resource cleanup even if Setup() fails
//
// Lifecycle:
//  1. Create via New()
//  2. Setup() - prepare environment
//  3. Execute() - run commands (multiple times)
//  4. Cleanup() - tear down environment
//
// All implementations must be safe for concurrent use by multiple goroutines
// after Setup() completes successfully.
type Environment interface {
	// Setup prepares the build environment for the given worker.
	//
	// For BSD chroot backend:
	//   - Creates base directory at cfg.BuildBase/Workers/{workerID}
	//   - Creates all mount point directories
	//   - Sets up 27 filesystem mounts (nullfs, tmpfs, devfs, procfs)
	//   - Creates necessary subdirectories
	//
	// For mock backend:
	//   - Creates temporary directory structure
	//   - No actual isolation
	//
	// The logger is used to report warnings about non-fatal setup issues
	// (e.g., mount failures that don't prevent the build from proceeding).
	//
	// Returns error if setup fails. If Setup() returns error, caller
	// must still call Cleanup() to release any partial resources.
	Setup(workerID int, cfg *config.Config, logger log.LibraryLogger) error

	// Execute runs a command in the isolated environment.
	//
	// The command is executed inside the environment (e.g., via chroot for BSD).
	// Respects context cancellation and timeout.
	//
	// Command execution:
	//   - WorkDir is relative to environment root
	//   - Command must be absolute path inside environment
	//   - Stdout/Stderr capture output (if provided)
	//   - Timeout applies to command execution (0 = no timeout)
	//
	// Example (BSD chroot):
	//   Execute runs: chroot <basepath> <command> <args...>
	//   With WorkDir, sets working directory inside chroot
	//
	// Returns:
	//   - ExecResult with exit code and duration
	//   - error if execution fails (not if command exits non-zero)
	//
	// The distinction: command returning exit code 1 is success (ExecResult.ExitCode=1, err=nil).
	// Failure to execute command (e.g., chroot failed) returns err != nil.
	Execute(ctx context.Context, cmd *ExecCommand) (*ExecResult, error)

	// Cleanup tears down the environment.
	//
	// For BSD chroot backend:
	//   - Unmounts all filesystems (with retry logic)
	//   - Removes temporary directories
	//   - Logs but does not fail on unmount errors after retries
	//
	// For mock backend:
	//   - Removes temporary directory structure
	//
	// Cleanup must be idempotent (safe to call multiple times).
	// Cleanup must succeed even if Setup() failed or was never called.
	// Cleanup must not return error for transient failures (e.g., busy mounts)
	// after exhausting retries - log warnings instead.
	//
	// Returns error only for catastrophic failures that prevent cleanup.
	Cleanup() error

	// GetBasePath returns the root path of the environment.
	//
	// For BSD chroot: returns the chroot base directory
	// For mock: returns the temporary directory
	//
	// Used for:
	//   - Logging and debugging
	//   - Compatibility with existing code during migration
	//   - Copying files into environment before execution
	GetBasePath() string
}

// ExecCommand describes a command to execute in an isolated environment.
//
// All paths are relative to the environment root (e.g., inside chroot).
type ExecCommand struct {
	// Command is the absolute path to the executable inside the environment.
	// Example: "/usr/bin/make", "/usr/local/bin/pkg"
	Command string

	// Args are the command arguments (excluding Command itself).
	// Example: []string{"install", "BATCH=yes"}
	Args []string

	// WorkDir is the working directory inside the environment.
	// Must be absolute path inside environment.
	// Example: "/xports/editors/vim"
	// If empty, uses environment's default (typically root).
	WorkDir string

	// Env contains environment variables to set.
	// Example: map[string]string{"BATCH": "yes", "MAKEFLAGS": "-j8"}
	// If nil or empty, inherits environment variables from parent.
	Env map[string]string

	// Stdout receives standard output from the command.
	// If nil, output is discarded.
	Stdout io.Writer

	// Stderr receives standard error from the command.
	// If nil, output is discarded.
	Stderr io.Writer

	// Timeout is the maximum execution duration.
	// Zero means no timeout.
	// Context cancellation takes precedence.
	Timeout time.Duration
}

// ExecResult contains the result of command execution.
type ExecResult struct {
	// ExitCode is the command's exit code.
	// 0 indicates success.
	// Non-zero indicates command-specific error.
	ExitCode int

	// Duration is how long the command took to execute.
	Duration time.Duration

	// Error is set if command execution failed.
	// This is different from non-zero exit code:
	//   - err != nil: failed to execute command (e.g., chroot failed)
	//   - err == nil, ExitCode != 0: command ran but returned error
	Error error
}

// NewEnvironmentFunc is a constructor function for Environment implementations.
type NewEnvironmentFunc func() Environment

// Backend registry for environment implementations.
var backends = make(map[string]NewEnvironmentFunc)

// Register registers an environment backend.
//
// Typically called from init() functions in backend packages:
//
//	func init() {
//	    environment.Register("bsd", func() environment.Environment {
//	        return &BSDEnvironment{}
//	    })
//	}
//
// Panics if name is already registered (programming error).
func Register(name string, fn NewEnvironmentFunc) {
	if _, exists := backends[name]; exists {
		panic(fmt.Sprintf("environment backend already registered: %s", name))
	}
	backends[name] = fn
}

// New creates a new Environment instance for the specified backend.
//
// Returns error if backend is not registered.
//
// Example:
//
//	env, err := environment.New("bsd")
//	if err != nil {
//	    return fmt.Errorf("failed to create environment: %w", err)
//	}
func New(backend string) (Environment, error) {
	fn, ok := backends[backend]
	if !ok {
		return nil, &ErrUnknownBackend{Backend: backend}
	}
	return fn(), nil
}

// ErrUnknownBackend is returned when requesting an unregistered backend.
type ErrUnknownBackend struct {
	Backend string
}

func (e *ErrUnknownBackend) Error() string {
	return fmt.Sprintf("unknown environment backend: %s", e.Backend)
}

// ErrSetupFailed indicates environment setup failed.
//
// This error type provides context for setup failures, including the
// specific operation that failed (e.g., "mkdir", "mount-root", "template-copy").
type ErrSetupFailed struct {
	Op  string // Operation that failed: "mkdir", "mount-root", "template-copy", etc.
	Err error  // Underlying error
}

func (e *ErrSetupFailed) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("setup failed (%s): %v", e.Op, e.Err)
	}
	return fmt.Sprintf("environment setup failed: %v", e.Err)
}

func (e *ErrSetupFailed) Unwrap() error {
	return e.Err
}

// ErrExecutionFailed indicates command execution failed.
//
// This is different from command returning non-zero exit code.
// This error type distinguishes between:
//   - Execution failures (chroot not found, permission denied, timeout)
//   - Command failures (command ran but returned non-zero exit code)
type ErrExecutionFailed struct {
	Op       string // Operation: "chroot", "timeout", "cancel"
	Command  string // Command path
	ExitCode int    // Exit code (0 if execution failed, >0 if command failed)
	Err      error  // Underlying error
}

func (e *ErrExecutionFailed) Error() string {
	if e.ExitCode > 0 {
		return fmt.Sprintf("%s failed: command %s exited with code %d: %v",
			e.Op, e.Command, e.ExitCode, e.Err)
	}
	if e.Op != "" {
		return fmt.Sprintf("%s failed: command %s: %v", e.Op, e.Command, e.Err)
	}
	return fmt.Sprintf("failed to execute %s: %v", e.Command, e.Err)
}

func (e *ErrExecutionFailed) Unwrap() error {
	return e.Err
}

// ErrCleanupFailed indicates catastrophic cleanup failure.
//
// Transient failures (e.g., busy mounts) should be logged but not returned.
// This error type provides context for cleanup failures, including:
//   - The specific operation that failed
//   - List of remaining mounts that couldn't be unmounted
type ErrCleanupFailed struct {
	Op     string   // Operation that failed: "unmount", "rmdir", etc.
	Err    error    // Underlying error
	Mounts []string // Remaining mounts that couldn't be unmounted (empty if not applicable)
}

func (e *ErrCleanupFailed) Error() string {
	if len(e.Mounts) > 0 {
		return fmt.Sprintf("cleanup failed (%s): %v (remaining mounts: %v)",
			e.Op, e.Err, e.Mounts)
	}
	if e.Op != "" {
		return fmt.Sprintf("cleanup failed (%s): %v", e.Op, e.Err)
	}
	return fmt.Sprintf("environment cleanup failed: %v", e.Err)
}

func (e *ErrCleanupFailed) Unwrap() error {
	return e.Err
}
