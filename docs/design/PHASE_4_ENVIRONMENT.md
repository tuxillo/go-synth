# Phase 4: Environment Abstraction

**Status**: 🚧 In Progress (8/10 tasks - 80%)  
**Last Updated**: 2025-11-28  
**Completion Date**: TBD  
**Dependencies**: Phase 3 complete ✅

**Progress Summary**:
- ✅ VM Testing Infrastructure (Task 0)
- ✅ Environment Interface (Task 1)
- ✅ BSD Implementation: Mount Logic, Setup, Execute, Cleanup (Tasks 2-5)
- ✅ Build Package Integration: phases.go, Worker Lifecycle (Tasks 6-7)
- ❌ Context & Error Handling (Task 8)
- ❌ Unit Tests (Task 9)
- ❌ Integration Tests & Documentation (Task 10)

**See [PHASE_4_TODO.md](PHASE_4_TODO.md) for detailed implementation tasks.**

## Goals

Phase 4 extracts build isolation (mount + chroot operations) from the build package into a clean, testable abstraction. The existing codebase has mount operations (294 lines in mount/mount.go) and direct chroot calls (5 locations in build/phases.go) tightly coupled with business logic. This phase creates proper separation of concerns.

### Primary Objectives

1. **Define Clean Interface**: Create minimal Environment interface for build isolation
2. **Implement BSD Backend**: Extract mount/chroot logic into FreeBSD/DragonFly implementation
3. **Decouple Build Package**: Remove all platform-specific code from build orchestration
4. **Enable Future Backends**: Design interface to support jails, containers, etc.
5. **Improve Testability**: Enable testing without root privileges via mocks

### Scope

**In Scope:**
- Environment interface (Setup, Execute, Cleanup)
- BSD implementation using nullfs/tmpfs + chroot
- Extract all mount logic from mount package
- Update build package to use Environment
- Context support for cancellation/timeout
- Structured error types
- Comprehensive testing (unit + integration)

**Out of Scope (Future Phases):**
- FreeBSD jails backend
- Linux containers (Docker, Podman)
- Remote worker execution
- Nested environments
- Resource limits (ulimit, cgroups)

### Non-Goals

- Change build orchestration logic (Phase 3)
- Change package resolution (Phase 1)
- Change database schema (Phase 2)
- Support non-BSD platforms (deferred)

---

## Architecture Overview

### Implementation State (70% Complete - Tasks 0-7 Done)

**What's Been Completed**:
- ✅ Environment interface defined (Task 1)
- ✅ BSD implementation complete: Setup, Execute, Cleanup (Tasks 2-5)
- ✅ Build package fully migrated to use Environment (Tasks 6-7)
- ✅ mount package usage removed from build package
- ✅ All chroot calls go through Environment.Execute()

**What Remains**:
- ❌ Context and error handling improvements (Task 8)
- ❌ Unit tests (Task 9)
- ❌ Integration tests and final documentation (Task 10)

### Original State (Before Phase 4)

```
┌─────────────────────────────────────────────┐
│ build/build.go (BuildContext, Workers)      │
│                                             │
│  - Calls mount.DoWorkerMounts()            │
│  - Calls mount.DoWorkerUnmounts()          │
│  - Direct coupling to mount package        │
└──────────────┬──────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────┐
│ build/phases.go (Phase Execution)           │
│                                             │
│  - exec.Command("chroot", ...) (5 places)  │
│  - Hard-coded chroot calls                 │
│  - No abstraction                          │
└──────────────┬──────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────┐
│ mount/mount.go (Mount Operations)           │
│                                             │
│  - DoWorkerMounts() - 27 mount points      │
│  - DoWorkerUnmounts() - retry logic        │
│  - BSD-specific implementation             │
└─────────────────────────────────────────────┘
```

**Problems:**
- Build logic tightly coupled to BSD mount operations
- Hard to test without root privileges
- Hard to port to other platforms
- No isolation layer
- Direct chroot execution scattered

### Current State (After Tasks 0-7 - 70% Complete) ✅

```
┌─────────────────────────────────────────────┐
│ build/build.go (BuildContext, Workers)      │
│                                             │
│  - Creates Environment via factory         │
│  - Calls env.Setup()                       │
│  - Manages environment lifecycle           │
└──────────────┬──────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────┐
│ build/phases.go (Phase Execution)           │
│                                             │
│  - Calls env.Execute(ctx, cmd)             │
│  - No direct chroot calls                  │
│  - Platform-agnostic                       │
└──────────────┬──────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────┐
│ environment/environment.go (Interface)       │
│                                             │
│  - Environment interface                   │
│  - ExecCommand, ExecResult types           │
│  - Backend registry                        │
└──────────────┬──────────────────────────────┘
               │
               ├─────────────────┬─────────────┐
               ▼                 ▼             ▼
         ┌─────────┐      ┌─────────┐   ┌─────────┐
         │   BSD   │      │  Mock   │   │ Future  │
         │ Backend │      │ Backend │   │Backends │
         └─────────┘      └─────────┘   └─────────┘
               │
               ▼
┌─────────────────────────────────────────────┐
│ environment/bsd/ (BSD Implementation)        │
│                                             │
│  - bsd.go: BSDEnvironment struct           │
│  - mounts.go: Mount operations             │
│  - All BSD-specific code                   │
└─────────────────────────────────────────────┘
```

**Benefits:**
- Build package platform-agnostic
- Easy to test with mock environment
- Easy to add new backends (jails, containers)
- Clear separation of concerns
- Structured error handling

---

## Interface Design

### Core Interface

```go
package environment

import (
    "context"
    "io"
    "time"
)

// Environment provides isolated execution for build phases
type Environment interface {
    // Setup prepares the build environment
    // - Creates directories
    // - Sets up mounts (if applicable)
    // - Copies template files
    // Returns error if setup fails
    Setup(workerID int, cfg *config.Config) error
    
    // Execute runs a command in the isolated environment
    // - Respects context cancellation
    // - Captures stdout/stderr
    // - Returns exit code and error
    Execute(ctx context.Context, cmd *ExecCommand) (*ExecResult, error)
    
    // Cleanup tears down the environment
    // - Unmounts filesystems (if applicable)
    // - Removes temporary directories
    // - Retries on failure
    // Must succeed even if Setup() failed
    Cleanup() error
    
    // GetBasePath returns the root path of the environment
    // Used for compatibility and debugging
    GetBasePath() string
}
```

### Supporting Types

```go
// ExecCommand describes a command to execute
type ExecCommand struct {
    WorkDir string            // Working directory inside environment
    Command string            // Command to execute (absolute path)
    Args    []string          // Command arguments
    Env     map[string]string // Environment variables
    Stdout  io.Writer         // Standard output writer
    Stderr  io.Writer         // Standard error writer
    Timeout time.Duration     // Execution timeout (0 = no timeout)
}

// ExecResult contains command execution results
type ExecResult struct {
    ExitCode int           // Command exit code
    Duration time.Duration // Execution duration
    Error    error         // Execution error (if any)
}

// Backend registry
type NewEnvironmentFunc func() Environment

func Register(name string, fn NewEnvironmentFunc)
func New(backend string) (Environment, error)
```

### Design Rationale

**Why Execute() takes ExecCommand?**
- Single argument vs many parameters
- Easy to extend without breaking API
- Clear ownership of I/O writers

**Why context.Context?**
- Support cancellation (Ctrl+C, timeout)
- Standard Go pattern
- Works with exec.CommandContext

**Why GetBasePath()?**
- Needed for Worker compatibility
- Useful for debugging
- Allows checking work directories

---

## BSD Implementation Details

### Mount Topology

The BSD environment creates 27 mount points per worker:

```
{BuildBase}/SL{N}/          (tmpfs, root)
├── bin/                    (nullfs ro, system)
├── sbin/                   (nullfs ro, system)
├── lib/                    (nullfs ro, system)
├── libexec/                (nullfs ro, system)
├── boot/                   (tmpfs rw)
│   └── modules.local/
├── usr/
│   ├── bin/                (nullfs ro, system)
│   ├── sbin/               (nullfs ro, system)
│   ├── lib/                (nullfs ro, system)
│   ├── libdata/            (nullfs ro, system)
│   ├── libexec/            (nullfs ro, system)
│   ├── include/            (nullfs ro, system)
│   ├── share/              (nullfs ro, system)
│   ├── games/              (nullfs ro, system)
│   ├── src/                (nullfs ro, system, optional)
│   ├── local/              (tmpfs rw, 16g)
│   └── packages/           (mkdir)
├── xports/                 (nullfs ro, ports tree)
├── options/                (nullfs rw, options)
├── packages/               (nullfs rw, packages dir)
├── distfiles/              (nullfs rw, distfiles)
├── construction/           (tmpfs rw, 64g)
├── ccache/                 (nullfs rw, optional)
├── tmp/                    (tmpfs rw)
├── dev/                    (devfs rw)
└── proc/                   (procfs ro)
```

**Mount Types:**
- **tmpfs**: Fast temporary filesystem (RAM-backed)
- **nullfs**: Loopback mount (like bind mount on Linux)
- **devfs**: Device filesystem
- **procfs**: Process filesystem

**Size Allocations:**
- `/construction`: 64g (build working directory)
- `/usr/local`: 16g (installed packages)
- Other tmpfs: 16g default

### Execution Flow

```go
// In build/build.go (workerRoutine):
env, _ := environment.New("bsd")
env.Setup(workerID, cfg)
defer env.Cleanup()

// In build/phases.go (executePhase):
cmd := &environment.ExecCommand{
    Command: "/usr/bin/make",
    Args:    []string{"-C", "/xports/editors/vim", "build"},
    Env:     map[string]string{
        "PORTSDIR": "/xports",
        "WRKDIRPREFIX": "/construction",
    },
    Stdout:  logger,
    Stderr:  logger,
}
result, _ := worker.Env.Execute(ctx, cmd)
```

### BSD-Specific Considerations

**Root Privileges:**
- Required for mount operations
- Required for chroot execution
- Validate early: `if os.Getuid() != 0 { return ErrRequiresRoot }`

**Path Resolution:**
- `"dummy"` → `"tmpfs"` (for tmpfs mounts)
- `"$/bin"` → `"{SystemPath}/bin"` (system paths)
- Absolute paths used as-is

**Template Copying:**
- Template directory: `{BuildBase}/Template/`
- Contains resolv.conf, make.conf, pkg repos, etc.
- Copied with `cp -Rp` to preserve permissions

**Cleanup Challenges:**
- Filesystems may be busy (open files)
- Requires retry logic (10 attempts, 5s interval)
- Must unmount in reverse order

---

## Integration Points

### Worker Lifecycle

```go
type Worker struct {
    ID        int
    Env       environment.Environment  // New field
    Mount     *mount.Worker            // Deprecated (Phase 4)
    Current   *pkg.Package
    Status    string
    StartTime time.Time
}

func workerRoutine(ctx *BuildContext, worker *Worker) {
    defer ctx.wg.Done()
    
    // Create environment
    env, err := environment.New("bsd")
    if err != nil {
        ctx.logger.Error("Failed to create environment: %v", err)
        return
    }
    
    // Setup (replaces mount.DoWorkerMounts)
    if err := env.Setup(worker.ID, ctx.cfg); err != nil {
        ctx.logger.Error("Environment setup failed: %v", err)
        return
    }
    worker.Env = env
    
    // Cleanup on exit (replaces mount.DoWorkerUnmounts)
    defer func() {
        if err := env.Cleanup(); err != nil {
            ctx.logger.Error("Cleanup failed: %v", err)
        }
    }()
    
    // Process packages...
}
```

### Phase Execution

```go
// Before (build/phases.go):
cmd := exec.Command("chroot", worker.Mount.BaseDir, "/usr/bin/make", args...)

// After:
execCmd := &environment.ExecCommand{
    Command: "/usr/bin/make",
    Args:    args,
    Stdout:  logger,
    Stderr:  logger,
}
result, err := worker.Env.Execute(ctx, execCmd)
```

### Signal Handling

```go
// In DoBuild:
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

go func() {
    <-sigChan
    ctx.logger.Info("Interrupt received, canceling builds...")
    cancel() // Propagates to all Execute() calls
}()
```

---

## Migration Strategy

### Backward Compatibility

**Phase 4 Goals:**
- Introduce Environment abstraction
- Keep mount package functional (deprecated)
- Update Worker to support both paths

**Compatibility Approach:**

```go
type Worker struct {
    Env   environment.Environment  // New (Phase 4)
    Mount *mount.Worker            // Old (deprecated in Phase 4)
}

// Helper for transition period
func (w *Worker) getBasePath() string {
    if w.Env != nil {
        return w.Env.GetBasePath()
    }
    if w.Mount != nil {
        return w.Mount.BaseDir
    }
    return ""
}
```

### Migration Path

**Phase 4 (This Phase):**
1. Create environment package
2. Implement BSD backend
3. Update build package to use Environment
4. Mark mount package as deprecated
5. Both paths work (Env preferred)

**Phase 7 (Integration):**
1. Remove mount package entirely
2. Remove Worker.Mount field
3. Remove compatibility shims

### Deprecation Plan

```go
// mount/mount.go
// Deprecated: Use environment package instead.
// This package will be removed in Phase 7.
package mount
```

**Timeline:**
- Phase 4: Deprecate mount package
- Phase 5-6: Grace period
- Phase 7: Remove mount package

---

## Security & Privileges

### Root Requirement

**Why Root?**
- mount() system call requires CAP_SYS_ADMIN
- chroot() requires CAP_SYS_CHROOT
- FreeBSD/DragonFly: root only

**Validation:**

```go
func (e *BSDEnvironment) Setup(workerID int, cfg *config.Config) error {
    if os.Getuid() != 0 {
        return &SetupError{
            Op:  "validate",
            Err: ErrRequiresRoot,
        }
    }
    // ...
}
```

### Path Validation

**Prevent Path Traversal:**

```go
func validateMountPath(path string) error {
    if strings.Contains(path, "..") {
        return fmt.Errorf("path contains .. : %s", path)
    }
    if !filepath.IsAbs(path) {
        return fmt.Errorf("path must be absolute: %s", path)
    }
    return nil
}
```

**Verify Mount Sources Exist:**

```go
if source != "tmpfs" {
    if _, err := os.Stat(source); err != nil {
        return &MountError{
            Op:   "validate",
            Path: source,
            Err:  fmt.Errorf("source does not exist: %w", err),
        }
    }
}
```

### Device Restrictions

**devfs Limitations:**
- Mounted with default ruleset
- No raw disk devices exposed
- Network devices available (needed for pkg)
- Consider adding ruleset restrictions in future

### Signal Safety

**Cleanup on Interrupt:**

```go
// Install signal handler before Setup()
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

go func() {
    <-sigChan
    env.Cleanup() // Safe to call multiple times
    os.Exit(130)  // 128 + SIGINT
}()
```

---

## Error Handling

### Structured Error Types

```go
// Sentinel errors
var (
    ErrMountFailed   = errors.New("mount operation failed")
    ErrRequiresRoot  = errors.New("operation requires root privileges")
)

// Structured errors
type MountError struct {
    Op     string // "mount", "unmount"
    Path   string
    FSType string
    Source string
    Err    error
}

type SetupError struct {
    Op  string // "mkdir", "mount", "copy-template"
    Err error
}

type ExecutionError struct {
    Op       string // "chroot", "execute"
    Command  string
    ExitCode int
    Err      error
}

type CleanupError struct {
    Op     string   // "unmount", "remove"
    Err    error
    Mounts []string // Remaining mounts
}
```

### Error Inspection

```go
// Check error type
if environment.IsMountError(err) {
    // Handle mount failure
}

// Extract details
var mountErr *environment.MountError
if errors.As(err, &mountErr) {
    log.Printf("Mount failed: path=%s fstype=%s", 
        mountErr.Path, mountErr.FSType)
}
```

### Fail-Safe Behavior

**Setup Failures:**
- Track mount errors but continue
- Return aggregate error at end
- Cleanup attempts unmount on partial setup

**Cleanup Failures:**
- Retry 10 times with 5s interval
- Return error with remaining mounts
- Log warnings but don't panic

---

## Testing Strategy

### Unit Tests (No Root Required)

**Packages to Test:**
- `environment/` - Interface, registry, types
- `environment/mock.go` - Mock implementation
- `environment/bsd/` - Path resolution, logic (no actual mounts)

**Example:**

```go
func TestMountPathResolution(t *testing.T) {
    env := &BSDEnvironment{
        cfg: &config.Config{SystemPath: "/custom"},
    }
    
    tests := []struct{
        input string
        want  string
    }{
        {"dummy", "tmpfs"},
        {"$/bin", "/custom/bin"},
        {"/usr/ports", "/usr/ports"},
    }
    
    for _, tt := range tests {
        got := env.resolveMountSource(tt.input)
        if got != tt.want {
            t.Errorf("resolveMountSource(%q) = %q, want %q",
                tt.input, got, tt.want)
        }
    }
}
```

**Coverage Target:** >80%

### Integration Tests (Root Required)

**Packages to Test:**
- `environment/bsd/integration_test.go`

**Test Scenarios:**
1. Full lifecycle (Setup → Execute → Cleanup)
2. Multiple commands in same environment
3. Concurrent environments
4. Cleanup on partial setup
5. Signal handling
6. Mount retry logic

**Example:**

```go
//go:build integration
// +build integration

func TestBSD_FullLifecycle(t *testing.T) {
    if os.Getuid() != 0 {
        t.Skip("requires root")
    }
    
    env := environment.New("bsd")
    defer env.Cleanup()
    
    if err := env.Setup(99, cfg); err != nil {
        t.Fatal(err)
    }
    
    // Verify mounts present
    // Execute commands
    // Cleanup
    // Verify mounts gone
}
```

**Run with:** `sudo go test -tags=integration ./environment/bsd/`

---

## Performance Considerations

### Mount Setup Time

**Target:** <2 seconds per worker

**Breakdown:**
- Directory creation: ~100ms (27 directories)
- Mount operations: ~1.5s (27 mounts)
- Template copy: ~200ms (depends on template size)
- Total: ~1.8s typical

**Optimization:**
- Pre-create directories in parallel
- Use tmpfs for speed
- Minimize template size

### Cleanup Time

**Target:** <1 second typical, <60s worst-case

**Breakdown:**
- Unmount operations: ~500ms (if not busy)
- Retry on busy: up to 50s (10 retries × 5s)
- Directory removal: ~100ms

**Known Issues:**
- Busy filesystems require retries
- /construction may have open build logs
- /dev may have process references

### Memory Usage

**tmpfs Sizes:**
- `/construction`: 64g (build workspace)
- `/usr/local`: 16g (installed packages)
- Others: 16g default
- **Total per worker**: ~96g tmpfs allocated

**Actual Usage:**
- Depends on package being built
- tmpfs only uses actual memory needed
- Over-commit safe on modern systems

---

## Exit Criteria

Phase 4 is complete when:

### Functionality
- [x] Environment interface defined and documented (Task 1) ✅
- [x] BSD implementation complete (Setup, Execute, Cleanup) (Tasks 2-5) ✅
- [x] All mount logic moved to environment package (Task 2) ✅
- [x] All chroot execution goes through Environment.Execute() (Task 6) ✅
- [x] Workers use Environment for build isolation (Task 7) ✅
- [x] Context support for cancellation/timeout (Task 6) ✅

### Code Quality
- [x] No `exec.Command("chroot")` calls outside environment package (Task 6) ✅
- [x] No mount operations outside environment package (Task 7) ✅
- [ ] Structured error types for all failure modes (Task 8 - pending)
- [ ] >80% test coverage for environment package (Task 9 - pending)

### Testing
- [ ] Unit tests pass without root (Task 9 - pending)
- [ ] Integration tests pass with root (skip if not root) (Task 10 - pending)
- [ ] All tests pass with `-race` flag (Task 10 - pending)
- [x] Builds succeed on FreeBSD/DragonFly (Verified - compiles cleanly) ✅

### Backward Compatibility
- [x] Existing builds still work (Code compiles, logic preserved) ✅
- [x] mount package usage removed from build package (Task 7) ✅
- [ ] Migration path documented (Task 10 - pending)

### Documentation
- [x] Godoc for all exported types (Tasks 1-5) ✅
- [ ] PHASE_4_TODO.md completed (7/10 tasks done - 70%)
- [ ] environment/README.md created (Task 10 - pending)
- [x] DEVELOPMENT.md updated (Updated to 70% complete) ✅
- [ ] Phase 4 marked complete (3 tasks remaining)

---

## Dependencies

### Hard Dependencies
- **Phase 3 complete** ✅ - Builder orchestration with builddb integration

### Soft Dependencies
- **Phase 1 complete** ✅ - Package metadata and dependency resolution
- **Phase 2 complete** ✅ - BuildDB for CRC tracking

### System Dependencies
- FreeBSD or DragonFly BSD operating system
- Root privileges for mount/chroot operations
- Kernel support for: nullfs, tmpfs, devfs, procfs

---

## Future Enhancements (Post-Phase 4)

### Additional Backends

**FreeBSD Jails** (Phase 5+):
```go
type JailEnvironment struct {
    jailName string
    jid      int
}

func (e *JailEnvironment) Setup(...) error {
    // jail -c name=... path=... command=/bin/sh
}
```

**Linux Containers** (Future):
```go
type ContainerEnvironment struct {
    containerID string
}

func (e *ContainerEnvironment) Setup(...) error {
    // docker run / podman run
}
```

### Resource Limits

**ulimit Support:**
```go
type ExecCommand struct {
    // ...existing fields...
    Limits ResourceLimits
}

type ResourceLimits struct {
    MaxMemory     uint64 // bytes
    MaxOpenFiles  int
    MaxProcesses  int
    CPUTime       time.Duration
}
```

**cgroups (Linux):**
- Memory limits
- CPU quotas
- I/O limits

### Advanced Features

- Nested environments (chroot in jail)
- Shared mount optimization (dedupe read-only mounts)
- Environment pooling (reuse instead of teardown/setup)
- Snapshot/restore support

---

**Last Updated**: 2025-11-27  
**Phase Status**: 🔵 Ready to Start  
**Next Review**: After Phase 4 kickoff
