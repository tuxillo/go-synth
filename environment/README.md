# Environment Package

The `environment` package provides abstractions for isolated build execution in go-synth. It enables build phases to run in isolated environments without knowing implementation details, allowing for testing via mock implementations and support for multiple platform-specific isolation mechanisms.

## Overview

The core design separates **what** needs to happen (the `Environment` interface) from **how** it happens (platform-specific backends). This allows:

- **Testing without root**: Mock backend for unit tests
- **Multiple platforms**: BSD chroot, future jail support
- **Clean separation**: Build logic independent of isolation mechanism
- **Extensibility**: Easy to add new isolation backends

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Build System                           │
│                 (pkg/builder, build/)                       │
└────────────┬────────────────────────────────────────────────┘
             │
             │ Uses Environment interface
             │
┌────────────▼────────────────────────────────────────────────┐
│              Environment Interface                          │
│         Setup() / Execute() / Cleanup()                     │
└────────────┬─────────────────────────────┬──────────────────┘
             │                             │
    ┌────────▼────────┐          ┌────────▼────────┐
    │  BSD Backend    │          │  Mock Backend   │
    │  (chroot)       │          │  (testing)      │
    └─────────────────┘          └─────────────────┘
```

## Supported Backends

### BSD Backend (`bsd`)

**Platform**: DragonFlyBSD, FreeBSD  
**Isolation**: chroot with nullfs/tmpfs mounts  
**Requirements**: Root privileges  

The BSD backend implements the proven mount strategy from the original C dsynth:

- **27 mount points** total for complete isolation
- **Root tmpfs overlay** for ephemeral build state
- **Read-only nullfs** mounts for system directories (`/bin`, `/usr/lib`, etc.)
- **Read-write mounts** for build work areas (`/construction`, `/packages`, `/distfiles`)
- **Special filesystems**: `devfs` for devices, `procfs` for process info

See `bsd/bsd.go` for the complete mount layout documentation.

### Mock Backend (`mock`)

**Platform**: Any  
**Isolation**: None (records calls)  
**Requirements**: None (no root required)  

The mock backend is designed for testing and development:

- **Thread-safe recording** of all method calls
- **Configurable return values** and errors
- **No actual isolation** (safe for unit tests)
- **Auto-registered** as `"mock"` backend

See `mock.go` and `mock_test.go` for usage examples.

## Usage

### Basic Workflow

```go
import (
    "context"
    "go-synth/environment"
    "go-synth/config"
)

// Step 1: Create environment instance
env, err := environment.New("bsd")
if err != nil {
    return fmt.Errorf("failed to create environment: %w", err)
}

// Step 2: Setup the environment
ctx := context.Background()
baseDir, err := env.Setup(ctx, cfg)
if err != nil {
    env.Cleanup(ctx, cfg)  // Always cleanup on error
    return fmt.Errorf("setup failed: %w", err)
}
defer env.Cleanup(ctx, cfg)  // Ensure cleanup runs

// Step 3: Execute commands in the environment
result, err := env.Execute(ctx, cfg, environment.ExecCommand{
    Program: "/usr/bin/make",
    Args:    []string{"install", "clean"},
    WorkDir: "/xports/editors/vim",
    Env: map[string]string{
        "MAKE_JOBS_NUMBER": "4",
    },
})
if err != nil {
    return fmt.Errorf("execution failed: %w", err)
}

// Step 4: Check result
if result.ExitCode != 0 {
    log.Printf("Command failed with exit code %d", result.ExitCode)
    log.Printf("Stderr: %s", result.Stderr)
    return fmt.Errorf("build failed")
}

log.Printf("Build succeeded (took %v)", result.Duration)
```

### Multiple Commands

You can execute multiple commands in the same environment:

```go
env, _ := environment.New("bsd")
baseDir, _ := env.Setup(ctx, cfg)
defer env.Cleanup(ctx, cfg)

// Command 1: Configure
result, _ := env.Execute(ctx, cfg, environment.ExecCommand{
    Program: "/usr/bin/make",
    Args:    []string{"configure"},
    WorkDir: "/xports/editors/vim",
})

// Command 2: Build
result, _ = env.Execute(ctx, cfg, environment.ExecCommand{
    Program: "/usr/bin/make",
    Args:    []string{"build"},
    WorkDir: "/xports/editors/vim",
})

// Command 3: Install
result, _ = env.Execute(ctx, cfg, environment.ExecCommand{
    Program: "/usr/bin/make",
    Args:    []string{"install"},
    WorkDir: "/xports/editors/vim",
})
```

### Testing with Mock Backend

```go
// Use mock backend for testing
env, _ := environment.New("mock")
mockEnv := env.(*environment.MockEnvironment)

// Configure mock behavior
mockEnv.SetSetupReturn("/tmp/mock-base", nil)
mockEnv.SetExecuteReturn(&environment.ExecResult{
    ExitCode: 0,
    Stdout:   "Build successful\n",
}, nil)

// Run your code under test
builder := NewBuilder(env)
err := builder.BuildPackage(ctx, cfg, "editors/vim")

// Verify interactions
calls := mockEnv.GetSetupCalls()
if len(calls) != 1 {
    t.Errorf("Expected 1 Setup call, got %d", len(calls))
}
```

### Context Cancellation

All operations respect context cancellation:

```go
// Create context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

// If build takes longer than 30 minutes, it will be cancelled
result, err := env.Execute(ctx, cfg, environment.ExecCommand{
    Program: "/usr/bin/make",
    Args:    []string{"build"},
})
if err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        log.Println("Build timed out after 30 minutes")
    }
    return err
}
```

## Error Handling

The package provides structured error types for better diagnostics:

```go
// SetupError - Environment setup failed
err := env.Setup(ctx, cfg)
if err != nil {
    var setupErr *environment.SetupError
    if errors.As(err, &setupErr) {
        log.Printf("Setup failed: %s", setupErr.Reason)
        log.Printf("BaseDir: %s", setupErr.BaseDir)
    }
}

// ExecuteError - Command execution failed
result, err := env.Execute(ctx, cfg, cmd)
if err != nil {
    var execErr *environment.ExecuteError
    if errors.As(err, &execErr) {
        log.Printf("Execution failed: %s", execErr.Reason)
        log.Printf("Command: %s", execErr.Command)
        log.Printf("WorkDir: %s", execErr.WorkDir)
    }
}

// CleanupError - Cleanup failed (usually non-fatal)
err = env.Cleanup(ctx, cfg)
if err != nil {
    var cleanupErr *environment.CleanupError
    if errors.As(err, &cleanupErr) {
        log.Printf("Cleanup warning: %s", cleanupErr.Reason)
        // Continue - cleanup errors are usually non-fatal
    }
}

// MountError - BSD-specific mount operation failed
var mountErr *environment.MountError
if errors.As(err, &mountErr) {
    log.Printf("Mount failed: %s -> %s (%s)",
        mountErr.Source, mountErr.Target, mountErr.Type)
}
```

## Testing

### Unit Tests

Unit tests use the mock backend and require **no root privileges**:

```bash
# Run all unit tests
go test ./environment/...

# Run with race detector
go test -race ./environment/...

# Run with coverage
go test -cover ./environment/...
```

Expected coverage: **>80%** for all packages

### Integration Tests

Integration tests use the BSD backend and require **root privileges** and a **DragonFlyBSD VM**:

```bash
# IMPORTANT: Run integration tests in a VM, not on the host system!
# Integration tests create chroot environments and mount filesystems.

# SSH into DragonFlyBSD VM
ssh root@dragonfly-vm

# Run integration tests (requires root)
cd /path/to/go-synth
go test -tags=integration ./environment/bsd/

# Or use the VM workflow:
make vm-quick        # Build + deploy + test in VM
```

**VM Workflow** (recommended):

```bash
# On host machine:
make vm-build        # Build binary for DragonFlyBSD
make vm-deploy       # Copy to VM
make vm-test         # Run integration tests in VM
make vm-quick        # All three steps (fast iteration)
```

Integration tests verify:

1. **Full lifecycle**: Setup → Execute → Cleanup
2. **Multiple commands**: Sequential execution in same environment
3. **Concurrency**: Multiple environments running simultaneously
4. **Partial setup cleanup**: Cleanup works even if Setup fails
5. **Mount verification**: All 27 mounts created and removed correctly
6. **Context cancellation**: Operations respect timeouts
7. **Command timeouts**: Long-running commands can be interrupted

**NEVER run integration tests on the host system** - they manipulate system mounts and chroot environments.

## Performance Considerations

### BSD Backend

- **Setup time**: ~100-200ms (27 mounts + directory creation)
- **Execute overhead**: ~10-50ms (chroot + exec)
- **Cleanup time**: ~100-200ms (unmount all + directory removal)

Mount operations are the primary performance bottleneck. The implementation:

- Pre-allocates mount tracking slices (capacity 30)
- Uses batch unmount during cleanup
- Continues on mount errors (fail-safe mode)

### Mock Backend

- **Setup time**: <1ms (just records the call)
- **Execute overhead**: <1ms (returns configured result)
- **Cleanup time**: <1ms (no actual cleanup needed)

The mock backend is designed for high-performance unit testing with no I/O overhead.

## Adding New Backends

To add a new backend (e.g., jails, containers):

### 1. Create Backend Package

```go
// environment/jail/jail.go
package jail

import "go-synth/environment"

type JailEnvironment struct {
    jid  int
    name string
}

func NewJailEnvironment() environment.Environment {
    return &JailEnvironment{}
}

func init() {
    environment.Register("jail", NewJailEnvironment)
}
```

### 2. Implement Environment Interface

```go
func (j *JailEnvironment) Setup(ctx context.Context, cfg *config.Configuration) (string, error) {
    // Create jail with jail_set()
    // Return base directory
}

func (j *JailEnvironment) Execute(ctx context.Context, cfg *config.Configuration, cmd environment.ExecCommand) (*environment.ExecResult, error) {
    // Execute command in jail using jexec
    // Return result
}

func (j *JailEnvironment) Cleanup(ctx context.Context, cfg *config.Configuration) error {
    // Remove jail with jail_remove()
    // Cleanup directories
}
```

### 3. Add Tests

```go
// environment/jail/jail_test.go
func TestJailEnvironment_Interface(t *testing.T) {
    var _ environment.Environment = (*JailEnvironment)(nil)
}

func TestJailEnvironment_Setup(t *testing.T) {
    // Unit tests (no root required)
}

// environment/jail/integration_test.go
//go:build integration

func TestIntegration_JailFullLifecycle(t *testing.T) {
    // Integration tests (requires root + jail support)
}
```

### 4. Document Backend

Add documentation to:
- `environment/jail/README.md` - Backend-specific docs
- `environment/README.md` - Update supported backends list
- `docs/design/PHASE_4_ENVIRONMENT.md` - Architecture notes

### 5. Update Build System

Update `build/phases.go` to support the new backend:

```go
// Select backend based on configuration or platform
backendType := cfg.Environment.Backend
if backendType == "" {
    backendType = detectDefaultBackend() // "bsd", "jail", etc.
}

env, err := environment.New(backendType)
```

## Design Rationale

### Why Interface-Based Design?

1. **Testability**: Mock backend allows testing without root privileges
2. **Flexibility**: Easy to add new backends (jails, containers)
3. **Separation**: Build logic independent of isolation mechanism
4. **Safety**: Compile-time verification of implementation completeness

### Why Not Abstract Away Platform Differences?

Some backends have unique features (jail parameters, container networking). The interface provides the **minimum common contract**, while backends can expose additional functionality through type assertions:

```go
if bsdEnv, ok := env.(*bsd.BSDEnvironment); ok {
    // Access BSD-specific features
    mountErrors := bsdEnv.GetMountErrors()
}
```

### Why 27 Mounts?

The BSD backend follows the mount strategy from the original C dsynth, which has been proven in production for years. Each mount serves a specific purpose:

- **System binaries/libraries**: Read-only isolation
- **Build workspace**: Read-write tmpfs for speed
- **Shared directories**: Access to distfiles, packages, ports tree
- **Special filesystems**: devfs for devices, procfs for process info

See `bsd/bsd.go` for the complete mount layout with detailed comments.

## References

- **Interface definition**: `environment/environment.go`
- **BSD backend**: `environment/bsd/bsd.go`
- **Mock backend**: `environment/mock.go`
- **Error types**: `environment/errors.go`
- **Integration tests**: `environment/bsd/integration_test.go`
- **Design document**: `docs/design/PHASE_4_ENVIRONMENT.md`

## License

This package is part of go-synth (DragonFly BSD Ports Builder) and follows the project's license.
