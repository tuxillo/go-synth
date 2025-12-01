# Phase 4: Environment Abstraction - Task Breakdown

**Status**: 🟢 Complete  
**Last Updated**: 2025-11-28  
**Completion Date**: 2025-11-28  
**Dependencies**: Phase 3 complete ✅  
**Total Time**: 30 hours (27h implementation + 3h VM setup)

## ⚠️ Prerequisites: VM Testing Infrastructure

**Phase 4 requires a DragonFlyBSD VM for testing mount operations.**

Before starting Phase 4 implementation (Tasks 1-10), **Task 0 must be completed**:

### Task 0: Local DragonFlyBSD VM Testing Infrastructure ✅ COMPLETE

**Why We Need This**:
- Phase 4 implements 27 mount points (nullfs, tmpfs, devfs, procfs)
- Tests require root privileges + BSD-specific system calls
- Existing E2E tests skip 5 critical integration tests (require root + BSD)
- Cannot verify Phase 4 functionality without real BSD environment

**What Was Built**:
- QEMU/KVM-based DragonFlyBSD 6.4.2 VM (configurable version)
- Programmatic VM lifecycle management (create, start, stop, destroy, snapshot)
- SSH-based file sync and command execution
- Makefile integration (`make vm-*` targets)
- Comprehensive documentation (`docs/testing/VM_TESTING.md`)

**First-Time Setup (15 minutes, fully automated)**:
```bash
make vm-setup         # Download ISO, create disk
make vm-auto-install  # Fully automated 3-phase installation (no interaction)
# VM is ready! Clean snapshot created automatically
```

**Alternative (Manual)**:
```bash
make vm-setup      # Download ISO, create disk
make vm-install    # Manual OS installation
# SSH in, run provision script
make vm-snapshot   # Save clean state
```

**Daily Development Workflow**:
```bash
make vm-start      # Boot VM (30s)
# Edit code locally
make vm-quick      # Sync + test Phase 4
make vm-stop       # Shut down
```

**Files Created**:
- `scripts/vm/` - 9 VM management scripts (~500 lines)
- `docs/testing/VM_TESTING.md` - Complete documentation (~600 lines)
- `Makefile` - VM management targets (~150 lines)

**See**: `docs/testing/VM_TESTING.md` for complete setup and usage instructions.

---

## Overview

Phase 4 extracts mount and chroot operations from the build package into a clean Environment abstraction. The existing builder (~1,253 lines across build/*.go) directly calls `exec.Command("chroot", ...)` and uses mount/mount.go (294 lines) for filesystem setup. Phase 4 creates a proper abstraction layer that will enable future platform support (jails, containers) while keeping the builder platform-agnostic.

**Current Coupling Issues:**
- `build/phases.go` directly executes `chroot` commands (5 locations)
- Mount operations scattered between mount package and build logic
- No abstraction - BSD-specific code mixed with business logic
- Hard to test without root privileges
- Hard to port to other isolation mechanisms

**Phase 4 Goals:**
- Define minimal Environment interface for build isolation
- Implement BSD backend (FreeBSD/DragonFly with nullfs/tmpfs + chroot)
- Move all mount logic from mount package to environment package
- Update build package to use Environment interface
- Add context support for cancellation/timeout
- Comprehensive testing (unit + integration)

## Task Progress: 10/10 Complete (100%) ✅ PHASE 4 COMPLETE

### ✅ Completed: 10 tasks
- Task 0: VM Testing Infrastructure ✅
- Task 1: Define Environment Interface ✅
- Task 2: Implement BSD Environment - Mount Logic ✅
- Task 3: Implement BSD Environment - Setup() ✅
- Task 4: Implement BSD Environment - Execute() ✅
- Task 5: Implement BSD Environment - Cleanup() ✅
- Task 6: Update build/phases.go ✅
- Task 7: Update Worker Lifecycle ✅
- Task 8: Add Context and Error Handling ✅
- Task 9: Unit Tests ✅
- Task 10: Integration Tests and Documentation ✅

### 🚧 In Progress: 0 tasks

### ❌ Remaining: 0 tasks

---

## Task 1: Define Environment Interface

**Priority**: 🔴 High  
**Effort**: 2 hours  
**Status**: ✅ Complete

### Objective
Create clean abstraction for build isolation that works with any backend (BSD chroot, FreeBSD jails, DragonFly jails).

**Note**: This interface is designed for BSD isolation mechanisms:
- **chroot**: Current implementation (nullfs/tmpfs + chroot)
- **FreeBSD jails**: Future backend (FreeBSD-specific features)
- **DragonFly jails**: Future backend (DragonFly-specific features)
- **mock**: Testing backend (no actual isolation)

### Implementation Steps

1. **Create environment package** (environment/environment.go)
   ```go
   package environment
   
   import (
       "context"
       "io"
       "time"
   )
   
   // Environment provides isolated execution for build phases
   type Environment interface {
       // Setup prepares the build environment (mounts, directories, etc.)
       Setup(workerID int, cfg *config.Config) error
       
       // Execute runs a command in the isolated environment
       Execute(ctx context.Context, cmd *ExecCommand) (*ExecResult, error)
       
       // Cleanup tears down the environment (unmounts, removes temp dirs)
       Cleanup() error
       
       // GetBasePath returns the root path of the environment
       GetBasePath() string
   }
   
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
   ```

2. **Add constructor type**
   ```go
   // NewEnvironmentFunc creates a new Environment instance
   type NewEnvironmentFunc func() Environment
   
   // Registry for environment backends
   var backends = make(map[string]NewEnvironmentFunc)
   
   func Register(name string, fn NewEnvironmentFunc) {
       backends[name] = fn
   }
   
   func New(backend string) (Environment, error) {
       fn, ok := backends[backend]
       if !ok {
           return nil, fmt.Errorf("unknown backend: %s", backend)
       }
       return fn(), nil
   }
   ```

3. **Add package documentation**
   - Document Environment interface usage
   - Document when to use each method
   - Document error handling expectations
   - Document cleanup guarantees

### Files Created
- `environment/environment.go` (286 lines) ✅

### Testing Checklist
- [x] Package compiles
- [x] Interface methods documented
- [x] ExecCommand has all needed fields
- [x] Registry pattern works

### Success Criteria
- ✅ Clean interface with clear responsibilities
- ✅ ExecCommand supports all build phase needs
- ✅ Documentation explains usage patterns
- ✅ Error types defined (4 types)
- ✅ BSD-specific documentation
- ✅ Future jail support noted
- Ready for BSD implementation

### Dependencies
- Phase 3 complete ✅
- No other dependencies

---

## Task 2: Implement BSD Environment - Mount Logic

**Priority**: 🔴 High  
**Effort**: 2 hours  
**Status**: ✅ Complete

### Objective
Extract mount operations from mount/mount.go into environment/bsd/ package.

### Implementation Steps

1. **Create environment/bsd/mounts.go** (extract from mount/mount.go)
   ```go
   package bsd
   
   import (
       "go-synth/config"
       "fmt"
       "os"
       "os/exec"
       "path/filepath"
       "strings"
       "time"
       
       "golang.org/x/sys/unix"
   )
   
   // Mount types (from mount/mount.go)
   const (
       MountTypeMask   = 0x000F
       MountTypeTmpfs  = 0x0001
       MountTypeNullfs = 0x0002
       MountTypeDevfs  = 0x0003
       MountTypeProcfs = 0x0004
       MountTypeRW     = 0x0010
       MountTypeBig    = 0x0020
       MountTypeMed    = 0x0080
   )
   
   const (
       TmpfsRW    = MountTypeTmpfs | MountTypeRW
       TmpfsRWBig = MountTypeTmpfs | MountTypeRW | MountTypeBig
       TmpfsRWMed = MountTypeTmpfs | MountTypeRW | MountTypeMed
       NullfsRO   = MountTypeNullfs
       NullfsRW   = MountTypeNullfs | MountTypeRW
       DevfsRW    = MountTypeDevfs | MountTypeRW
       ProcfsRO   = MountTypeProcfs
   )
   
   // mountState tracks mounted filesystems
   type mountState struct {
       target string
       fstype string
       source string
   }
   
   // doMount performs a single mount operation
   func (e *BSDEnvironment) doMount(mountType int, spath, dpath string) error {
       // Resolve source path
       var source string
       if spath == "dummy" {
           source = "tmpfs"
       } else if strings.HasPrefix(spath, "$") {
           // System path
           sysPath := e.cfg.SystemPath
           if sysPath == "/" {
               source = spath[1:]
           } else {
               source = filepath.Join(sysPath, spath[1:])
           }
       } else {
           source = spath
       }
       
       // Resolve target path
       target := filepath.Join(e.baseDir, dpath)
       
       // Create target directory
       if err := os.MkdirAll(target, 0755); err != nil {
           return &MountError{Op: "mkdir", Path: target, Err: err}
       }
       
       // Determine mount options
       rwOpt := "ro"
       if mountType&MountTypeRW != 0 {
           rwOpt = "rw"
       }
       
       var fstype string
       var opts []string
       
       switch mountType & MountTypeMask {
       case MountTypeTmpfs:
           fstype = "tmpfs"
           opts = []string{rwOpt}
           if mountType&MountTypeBig != 0 {
               opts = append(opts, "size=64g")
           } else if mountType&MountTypeMed != 0 {
               opts = append(opts, "size=16g")
           } else {
               opts = append(opts, "size=16g")
           }
           
       case MountTypeNullfs:
           fstype = "null"
           opts = []string{rwOpt}
           
       case MountTypeDevfs:
           fstype = "devfs"
           opts = []string{rwOpt}
           
       case MountTypeProcfs:
           fstype = "procfs"
           opts = []string{rwOpt}
           
       default:
           return &MountError{
               Op:   "mount",
               Path: target,
               Err:  fmt.Errorf("unknown mount type: %x", mountType),
           }
       }
       
       // Execute mount command
       optStr := strings.Join(opts, ",")
       cmd := exec.Command("mount", "-t", fstype, "-o", optStr, source, target)
       output, err := cmd.CombinedOutput()
       if err != nil {
           return &MountError{
               Op:     "mount",
               Path:   target,
               Err:    fmt.Errorf("%w: %s", err, string(output)),
               FSType: fstype,
               Source: source,
           }
       }
       
       // Track mounted filesystem
       e.mounts = append(e.mounts, mountState{
           target: target,
           fstype: fstype,
           source: source,
       })
       
       return nil
   }
   
   // doUnmount unmounts a single filesystem
   func (e *BSDEnvironment) doUnmount(rpath string) error {
       target := filepath.Join(e.baseDir, rpath)
       
       if err := unix.Unmount(target, 0); err != nil {
           switch err {
           case unix.EPERM, unix.ENOENT, unix.EINVAL:
               // Expected errors, ignore
               return nil
           default:
               return &MountError{Op: "unmount", Path: target, Err: err}
           }
       }
       
       return nil
   }
   ```

2. **Track mount state**
   - Keep list of mounted filesystems
   - Use for cleanup verification
   - Help with debugging mount issues

### Files to Create
- `environment/bsd/mounts.go` (~200 lines)

### Testing Checklist
- [x] Mount type constants correct
- [x] Path resolution works
- [x] Mount options correct
- [x] Error handling proper
- [x] Mount state tracked

### Success Criteria
- ✅ All mount logic extracted from mount/mount.go
- ✅ Mount operations return structured errors
- ✅ Mount state tracked for cleanup
- ✅ Code compiles and follows Go conventions
- ✅ go fmt and go vet pass with no warnings

### Dependencies
- Task 1 (Environment interface)

---

## Task 3: Implement BSD Environment - Setup()

**Priority**: 🔴 High  
**Effort**: 2 hours  
**Status**: ✅ Complete

### Objective
Implement Setup() method that creates and mounts the build environment.

### Implementation Steps

1. **Create environment/bsd/bsd.go**
   ```go
   package bsd
   
   import (
       "context"
       "go-synth/config"
       "go-synth/environment"
       "fmt"
       "os"
       "os/exec"
       "path/filepath"
   )
   
   // BSDEnvironment implements Environment for FreeBSD/DragonFly BSD
   type BSDEnvironment struct {
       baseDir     string
       cfg         *config.Config
       mounts      []mountState
       mountErrors int
   }
   
   // NewBSDEnvironment creates a new BSD environment
   func NewBSDEnvironment() environment.Environment {
       return &BSDEnvironment{
           mounts: make([]mountState, 0, 30), // Pre-allocate for ~27 mounts
       }
   }
   
   func init() {
       environment.Register("bsd", NewBSDEnvironment)
   }
   
   // Setup prepares the build environment
   func (e *BSDEnvironment) Setup(workerID int, cfg *config.Config) error {
       e.cfg = cfg
       e.baseDir = filepath.Join(cfg.BuildBase, fmt.Sprintf("SL%02d", workerID))
       e.mountErrors = 0
       
       // Create base directory
       if err := os.MkdirAll(e.baseDir, 0755); err != nil {
           return &SetupError{
               Op:  "mkdir",
               Err: fmt.Errorf("cannot create basedir: %w", err),
           }
       }
       
       // Mount root tmpfs
       if err := e.doMount(TmpfsRW, "dummy", ""); err != nil {
           e.mountErrors++
           return &SetupError{Op: "mount-root", Err: err}
       }
       
       // Create all mount point directories
       mountPoints := []string{
           "usr", "usr/packages", "boot", "boot/modules.local",
           "bin", "sbin", "lib", "libexec",
           "usr/bin", "usr/include", "usr/lib", "usr/libdata",
           "usr/libexec", "usr/sbin", "usr/share", "usr/games",
           "usr/src", "xports", "options", "packages", "distfiles",
           "construction", "usr/local", "ccache", "tmp", "dev", "proc",
       }
       
       for _, mp := range mountPoints {
           dir := filepath.Join(e.baseDir, mp)
           if err := os.MkdirAll(dir, 0755); err != nil {
               return &SetupError{
                   Op:  "mkdir",
                   Err: fmt.Errorf("mkdir %s failed: %w", dir, err),
               }
           }
       }
       
       // System mounts
       if err := e.doMount(TmpfsRW, "dummy", "/boot"); err != nil {
           e.mountErrors++
       }
       if err := e.doMount(DevfsRW, "dummy", "/dev"); err != nil {
           e.mountErrors++
       }
       if err := e.doMount(ProcfsRO, "dummy", "/proc"); err != nil {
           e.mountErrors++
       }
       
       // Nullfs mounts from system
       systemMounts := []struct {
           src  string
           dst  string
           mode int
       }{
           {"$/bin", "/bin", NullfsRO},
           {"$/sbin", "/sbin", NullfsRO},
           {"$/lib", "/lib", NullfsRO},
           {"$/libexec", "/libexec", NullfsRO},
           {"$/usr/bin", "/usr/bin", NullfsRO},
           {"$/usr/include", "/usr/include", NullfsRO},
           {"$/usr/lib", "/usr/lib", NullfsRO},
           {"$/usr/libdata", "/usr/libdata", NullfsRO},
           {"$/usr/libexec", "/usr/libexec", NullfsRO},
           {"$/usr/sbin", "/usr/sbin", NullfsRO},
           {"$/usr/share", "/usr/share", NullfsRO},
           {"$/usr/games", "/usr/games", NullfsRO},
       }
       
       for _, m := range systemMounts {
           if err := e.doMount(m.mode, m.src, m.dst); err != nil {
               e.mountErrors++
           }
       }
       
       // Optional /usr/src
       if cfg.UseUsrSrc {
           if err := e.doMount(NullfsRO, "$/usr/src", "/usr/src"); err != nil {
               e.mountErrors++
           }
       }
       
       // Ports and build directories
       if err := e.doMount(NullfsRO, cfg.DPortsPath, "/xports"); err != nil {
           e.mountErrors++
       }
       if err := e.doMount(NullfsRW, cfg.OptionsPath, "/options"); err != nil {
           e.mountErrors++
       }
       if err := e.doMount(NullfsRW, cfg.PackagesPath, "/packages"); err != nil {
           e.mountErrors++
       }
       if err := e.doMount(NullfsRW, cfg.DistFilesPath, "/distfiles"); err != nil {
           e.mountErrors++
       }
       if err := e.doMount(TmpfsRWBig, "dummy", "/construction"); err != nil {
           e.mountErrors++
       }
       if err := e.doMount(TmpfsRWMed, "dummy", "/usr/local"); err != nil {
           e.mountErrors++
       }
       
       // Optional ccache
       if cfg.UseCCache {
           if err := e.doMount(NullfsRW, cfg.CCachePath, "/ccache"); err != nil {
               e.mountErrors++
           }
       }
       
       // Copy template
       templatePath := filepath.Join(cfg.BuildBase, "Template")
       cmd := exec.Command("cp", "-Rp", templatePath+"/.", e.baseDir)
       if err := cmd.Run(); err != nil {
           return &SetupError{
               Op:  "copy-template",
               Err: fmt.Errorf("template copy failed: %w", err),
           }
       }
       
       if e.mountErrors > 0 {
           return &SetupError{
               Op:  "mount",
               Err: fmt.Errorf("mount errors occurred: %d", e.mountErrors),
           }
       }
       
       return nil
   }
   
   // GetBasePath returns the root path of the environment
   func (e *BSDEnvironment) GetBasePath() string {
       return e.baseDir
   }
   ```

2. **Add mount verification**
   - Check each mount succeeded
   - Track errors but continue (fail-safe)
   - Return aggregate error at end

### Files to Modify
- `environment/bsd/bsd.go` (new file, ~250 lines total with Setup)

### Testing Checklist
- [x] Base directory created
- [x] All mount points created
- [x] All mounts execute
- [x] Template copied
- [x] Errors tracked properly
- [x] Mount state populated

### Success Criteria
- ✅ Setup() creates fully functional environment
- ✅ All 27 mount points mounted
- ✅ Template files copied
- ✅ Error handling preserves context
- ✅ Mount state tracked for cleanup
- ✅ go fmt and go vet pass with no warnings

### Dependencies
- Task 1 (Environment interface)
- Task 2 (Mount logic)

---

## Task 4: Implement BSD Environment - Execute()

**Priority**: 🔴 High  
**Effort**: 2 hours  
**Status**: ✅ Complete

### Objective
Implement Execute() method that runs commands in chroot environment.

### Implementation Steps

1. **Add Execute() to bsd.go**
   ```go
   // Execute runs a command in the chroot environment
   func (e *BSDEnvironment) Execute(ctx context.Context, cmd *environment.ExecCommand) (*environment.ExecResult, error) {
       if e.baseDir == "" {
           return nil, &ExecutionError{
               Op:  "execute",
               Err: fmt.Errorf("environment not set up"),
           }
       }
       
       startTime := time.Now()
       
       // Build chroot command
       args := []string{e.baseDir, cmd.Command}
       args = append(args, cmd.Args...)
       
       execCmd := exec.CommandContext(ctx, "chroot", args...)
       execCmd.Dir = "/"
       
       // Set environment variables
       if len(cmd.Env) > 0 {
           env := make([]string, 0, len(cmd.Env))
           for k, v := range cmd.Env {
               env = append(env, fmt.Sprintf("%s=%s", k, v))
           }
           execCmd.Env = env
       }
       
       // Set output writers
       if cmd.Stdout != nil {
           execCmd.Stdout = cmd.Stdout
       }
       if cmd.Stderr != nil {
           execCmd.Stderr = cmd.Stderr
       }
       
       // Execute with timeout support
       err := execCmd.Run()
       duration := time.Since(startTime)
       
       result := &environment.ExecResult{
           Duration: duration,
       }
       
       // Get exit code
       if err != nil {
           if exitErr, ok := err.(*exec.ExitError); ok {
               result.ExitCode = exitErr.ExitCode()
               result.Error = &ExecutionError{
                   Op:       "chroot",
                   Command:  cmd.Command,
                   ExitCode: result.ExitCode,
                   Err:      err,
               }
           } else {
               result.ExitCode = -1
               result.Error = &ExecutionError{
                   Op:      "chroot",
                   Command: cmd.Command,
                   Err:     err,
               }
           }
       } else {
           result.ExitCode = 0
       }
       
       return result, result.Error
   }
   ```

2. **Add timeout handling**
   - Use CommandContext for cancellation
   - Respect cmd.Timeout if set
   - Return appropriate error on timeout

3. **Add environment variable support**
   - Convert map to []string
   - Pass to exec.Cmd.Env

### Files to Modify
- `environment/bsd/bsd.go` (add ~80 lines)

### Testing Checklist
- [x] Chroot command constructed correctly
- [x] Context cancellation works
- [x] Timeout handling works
- [x] Environment variables passed
- [x] Output capture works
- [x] Exit codes returned correctly
- [x] Errors are structured

### Success Criteria
- ✅ Execute() runs commands in chroot
- ✅ Context support for cancellation
- ✅ Timeout support works
- ✅ Exit codes captured correctly
- ✅ Structured errors with context
- ✅ go fmt and go vet pass with no warnings
- ✅ Comprehensive documentation (70 lines godoc)

### Dependencies
- Task 1 (Environment interface)
- Task 3 (Setup implementation)

---

## Task 5: Implement BSD Environment - Cleanup()

**Priority**: 🔴 High  
**Effort**: 1 hour  
**Status**: ✅ Complete

### Objective
Implement Cleanup() method that unmounts and removes the environment.

### Implementation Steps

1. **Add Cleanup() to bsd.go**
   ```go
   // Cleanup tears down the environment
   func (e *BSDEnvironment) Cleanup() error {
       if e.baseDir == "" {
           return nil // Nothing to clean up
       }
       
       e.mountErrors = 0
       
       // Unmount in reverse order (10 retries with 5s sleep)
       unmountOrder := []string{
           "/proc", "/dev", "/usr/src", "/usr/games", "/boot",
           "/usr/local", "/construction", "/ccache", "/distfiles",
           "/packages", "/options", "/xports",
           "/usr/share", "/usr/sbin", "/usr/libexec", "/usr/libdata",
           "/usr/lib", "/usr/include", "/usr/bin",
           "/libexec", "/lib", "/sbin", "/bin",
           "", // root tmpfs
       }
       
       for retry := 0; retry < 10; retry++ {
           for _, path := range unmountOrder {
               if err := e.doUnmount(path); err != nil {
                   e.mountErrors++
               }
           }
           
           if e.mountErrors == 0 {
               break
           }
           
           if retry < 9 {
               time.Sleep(5 * time.Second)
               e.mountErrors = 0
           }
       }
       
       if e.mountErrors > 0 {
           return &CleanupError{
               Op:     "unmount",
               Err:    fmt.Errorf("unable to unmount all filesystems after 10 retries"),
               Mounts: e.listRemainingMounts(),
           }
       }
       
       // Remove base directory
       if err := os.RemoveAll(e.baseDir); err != nil {
           return &CleanupError{
               Op:  "remove",
               Err: fmt.Errorf("failed to remove basedir: %w", err),
           }
       }
       
       return nil
   }
   
   // listRemainingMounts returns list of mounts that couldn't be unmounted
   func (e *BSDEnvironment) listRemainingMounts() []string {
       remaining := make([]string, 0)
       for _, m := range e.mounts {
           // Check if still mounted
           cmd := exec.Command("mount")
           output, err := cmd.Output()
           if err == nil && strings.Contains(string(output), m.target) {
               remaining = append(remaining, m.target)
           }
       }
       return remaining
   }
   ```

2. **Add retry logic**
   - Current mount/mount.go has 10 retries with 5s sleep
   - Preserve this behavior (handles busy filesystems)
   - Track which mounts failed

3. **Add cleanup verification**
   - List remaining mounts on failure
   - Help debugging stuck mounts

### Files to Modify
- `environment/bsd/bsd.go` (add ~60 lines)

### Testing Checklist
- [x] All mounts unmounted
- [x] Retry logic works
- [x] Base directory removed
- [x] Errors tracked
- [x] Remaining mounts listed on failure

### Success Criteria
- Cleanup() successfully unmounts all filesystems
- Retry logic handles busy filesystems
- Base directory removed
- Clear error messages on failure

### Dependencies
- Task 2 (Mount logic)
- Task 3 (Setup implementation)

---

## Task 6: Update build/phases.go

**Priority**: 🔴 High  
**Effort**: 3 hours  
**Status**: ✅ Complete (2025-11-28)

### Objective
Remove all direct chroot calls and use Environment.Execute() instead.

### Implementation Steps

1. **Update executePhase() function** (build/phases.go:16)
   ```go
   // Before (lines 88-89):
   cmd := exec.Command("chroot", worker.Mount.BaseDir, "/usr/bin/make")
   cmd.Args = append([]string{"chroot", worker.Mount.BaseDir, "/usr/bin/make"}, args...)
   
   // After:
   execCmd := &environment.ExecCommand{
       WorkDir: "/",
       Command: "/usr/bin/make",
       Args:    args,
       Env: map[string]string{
           "PATH": "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin",
       },
       Stdout: logger,
       Stderr: logger,
   }
   
   result, err := worker.Env.Execute(ctx, execCmd)
   if err != nil {
       return fmt.Errorf("phase failed: %w", err)
   }
   
   if result.ExitCode != 0 {
       return fmt.Errorf("phase failed with exit code %d", result.ExitCode)
   }
   ```

2. **Update installDependencyPackages()** (build/phases.go:105)
   ```go
   // Before (line 137):
   cmd := exec.Command("chroot", worker.Mount.BaseDir, "pkg", "add", pkgPath)
   
   // After:
   execCmd := &environment.ExecCommand{
       WorkDir: "/",
        Command: "/usr/local/sbin/pkg",

       Args:    []string{"add", pkgPath},
       Stdout:  logger,
       Stderr:  logger,
   }
   
   result, err := worker.Env.Execute(ctx, execCmd)
   ```

3. **Update installMissingPackages()** (build/phases.go:184)
   - Similar conversion for pkg info and pkg add
   - Use Environment.Execute() for both operations

4. **Add context parameter**
   - Add `context.Context` to executePhase signature
   - Pass through to Execute() calls
   - Support cancellation

5. **Remove all exec.Command("chroot") calls**
   - 5 total locations to update
   - All should use Environment.Execute()

### Files to Modify
- `build/phases.go` (~30 lines changed across 5 functions)

### Testing Checklist
- [ ] No exec.Command("chroot") calls remain
- [ ] All phases use Environment.Execute()
- [ ] Context passed through
- [ ] Output still captured to logger
- [ ] Exit codes checked properly
- [ ] Error messages clear

### Success Criteria
- No direct chroot execution in build package
- All isolation goes through Environment interface
- Context support for cancellation
- Existing tests still pass

### Dependencies
- Task 1 (Environment interface)
- Task 4 (Execute implementation)

---

## Task 7: Update Worker Lifecycle

**Priority**: 🔴 High  
**Effort**: 2 hours  
**Status**: ✅ Complete (2025-11-28)

### Objective
Update Worker struct to own Environment and manage its lifecycle.

### Implementation Steps

1. **Update Worker struct** (build/build.go)
   ```go
   // Before:
   type Worker struct {
       ID        int
       Mount     *mount.Worker
       Current   *pkg.Package
       Status    string
       StartTime time.Time
   }
   
   // After:
   type Worker struct {
       ID        int
       Env       environment.Environment // New
       Mount     *mount.Worker           // Deprecated, keep for compatibility
       Current   *pkg.Package
       Status    string
       StartTime time.Time
   }
   ```

2. **Update worker goroutine** (build/build.go, workerRoutine)
   ```go
   func workerRoutine(ctx *BuildContext, worker *Worker) {
       defer ctx.wg.Done()
       
       // Create and setup environment
       env, err := environment.New("bsd")
       if err != nil {
           ctx.logger.Error(fmt.Sprintf("Worker %d: failed to create environment: %v", 
               worker.ID, err))
           return
       }
       
       if err := env.Setup(worker.ID, ctx.cfg); err != nil {
           ctx.logger.Error(fmt.Sprintf("Worker %d: environment setup failed: %v", 
               worker.ID, err))
           return
       }
       worker.Env = env
       
       // Ensure cleanup happens
       defer func() {
           if err := env.Cleanup(); err != nil {
               ctx.logger.Error(fmt.Sprintf("Worker %d: cleanup failed: %v", 
                   worker.ID, err))
           }
       }()
       
       // Process packages from queue
       for p := range ctx.queue {
           worker.Current = p
           worker.Status = "building"
           worker.StartTime = time.Now()
           
           success := ctx.buildPackage(worker, p)
           
           // Update statistics
           ctx.statsMu.Lock()
           if success {
               ctx.stats.Success++
           } else {
               ctx.stats.Failed++
           }
           ctx.statsMu.Unlock()
           
           worker.Current = nil
           worker.Status = "idle"
       }
   }
   ```

3. **Update DoBuild()** to remove mount/unmount calls
   - Environment now handles mount lifecycle
   - Remove DoWorkerMounts/DoWorkerUnmounts calls
   - Keep cleanup function but make it call Env.Cleanup()

4. **Add compatibility shim** (optional, for migration)
   ```go
   // getBasePath returns base directory for compatibility
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

### Files to Modify
- `build/build.go` (~100 lines modified)

### Testing Checklist
- [ ] Worker.Env field initialized
- [ ] Environment setup in worker goroutine
- [ ] Cleanup deferred properly
- [ ] No DoWorkerMounts calls
- [ ] No DoWorkerUnmounts calls
- [ ] Errors logged appropriately

### Success Criteria
- Workers own their Environment
- Setup/cleanup managed in worker lifecycle
- No mount package calls in build.go
- Proper error handling for setup failures
- Defer ensures cleanup always runs

### Dependencies
- Task 3 (Setup implementation)
- Task 5 (Cleanup implementation)

---

## Task 8: Add Context and Error Handling

**Priority**: 🟡 Medium  
**Effort**: 3 hours  
**Status**: ✅ Complete

### Objective
Add context support for cancellation and structured error types.

### Implementation Steps

1. **Create environment/errors.go**
   ```go
   package environment
   
   import (
       "errors"
       "fmt"
   )
   
   var (
       ErrMountFailed     = errors.New("mount operation failed")
       ErrUnmountFailed   = errors.New("unmount operation failed")
       ErrChrootFailed    = errors.New("chroot execution failed")
       ErrSetupFailed     = errors.New("environment setup failed")
       ErrCleanupFailed   = errors.New("environment cleanup failed")
       ErrNotSetup        = errors.New("environment not set up")
   )
   
   // MountError represents a mount operation error
   type MountError struct {
       Op     string // "mount" or "unmount"
       Path   string
       FSType string
       Source string
       Err    error
   }
   
   func (e *MountError) Error() string {
       if e.FSType != "" {
           return fmt.Sprintf("%s %s: mount %s (type=%s, source=%s): %v",
               e.Op, e.Path, e.Path, e.FSType, e.Source, e.Err)
       }
       return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
   }
   
   func (e *MountError) Unwrap() error { return e.Err }
   
   // SetupError represents an environment setup error
   type SetupError struct {
       Op  string
       Err error
   }
   
   func (e *SetupError) Error() string {
       return fmt.Sprintf("setup failed (%s): %v", e.Op, e.Err)
   }
   
   func (e *SetupError) Unwrap() error { return e.Err }
   
   // ExecutionError represents a command execution error
   type ExecutionError struct {
       Op       string
       Command  string
       ExitCode int
       Err      error
   }
   
   func (e *ExecutionError) Error() string {
       if e.ExitCode > 0 {
           return fmt.Sprintf("%s failed: command %s exited with code %d: %v",
               e.Op, e.Command, e.ExitCode, e.Err)
       }
       return fmt.Sprintf("%s failed: command %s: %v", e.Op, e.Command, e.Err)
   }
   
   func (e *ExecutionError) Unwrap() error { return e.Err }
   
   // CleanupError represents an environment cleanup error
   type CleanupError struct {
       Op     string
       Err    error
       Mounts []string // Remaining mounts that couldn't be unmounted
   }
   
   func (e *CleanupError) Error() string {
       if len(e.Mounts) > 0 {
           return fmt.Sprintf("cleanup failed (%s): %v (remaining mounts: %v)",
               e.Op, e.Err, e.Mounts)
       }
       return fmt.Sprintf("cleanup failed (%s): %v", e.Op, e.Err)
   }
   
   func (e *CleanupError) Unwrap() error { return e.Err }
   
   // Helper functions for error inspection
   func IsMountError(err error) bool {
       var e *MountError
       return errors.As(err, &e)
   }
   
   func IsSetupError(err error) bool {
       var e *SetupError
       return errors.As(err, &e)
   }
   
   func IsExecutionError(err error) bool {
       var e *ExecutionError
       return errors.As(err, &e)
   }
   
   func IsCleanupError(err error) bool {
       var e *CleanupError
       return errors.As(err, &e)
   }
   ```

2. **Add context to build orchestration** (build/build.go)
   ```go
   func DoBuild(...) (*BuildStats, func(), error) {
       // Create context with timeout
       ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
       defer cancel()
       
       // Pass context through to workers
       buildCtx := &BuildContext{
           ctx:    ctx,
           cancel: cancel,
           // ... other fields
       }
       
       // ...
   }
   ```

3. **Add signal handling for cancellation**
   ```go
   // In DoBuild
   sigChan := make(chan os.Signal, 1)
   signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
   
   go func() {
       <-sigChan
       ctx.logger.Info("Received interrupt signal, canceling builds...")
       cancel()
   }()
   ```

### Files to Create
- `environment/errors.go` (~120 lines)

### Files to Modify
- `build/build.go` (add context support, ~30 lines)
- `build/phases.go` (accept context parameter, ~10 lines)

### Testing Checklist
- [ ] All error types implement error interface
- [ ] Errors are unwrappable
- [ ] Helper functions work
- [ ] Context propagates through layers
- [ ] Cancellation works
- [ ] Signal handling works

### Success Criteria
- Structured error types for all operations
- Errors provide useful context
- Context cancellation supported
- Signal handling graceful

### Dependencies
- Task 1 (Environment interface)
- Task 4 (Execute implementation)

---

## Task 9: Unit Tests

**Priority**: 🔴 High  
**Effort**: 4 hours  
**Status**: ✅ Complete (2025-11-28)

### Objective
Test environment logic without requiring root or real mounts.

### Implementation Steps

1. **Create mock environment** (environment/mock.go)
   ```go
   package environment
   
   import (
       "context"
       "go-synth/config"
       "fmt"
       "sync"
   )
   
   // MockEnvironment is a test implementation
   type MockEnvironment struct {
       mu sync.Mutex
       
       SetupCalled   bool
       SetupError    error
       CleanupCalled bool
       CleanupError  error
       
       ExecuteCalls  []*ExecCommand
       ExecuteResult *ExecResult
       ExecuteError  error
       
       BasePath string
   }
   
   func NewMockEnvironment() Environment {
       return &MockEnvironment{
           BasePath:      "/mock/base",
           ExecuteResult: &ExecResult{ExitCode: 0},
       }
   }
   
   func (m *MockEnvironment) Setup(workerID int, cfg *config.Config) error {
       m.mu.Lock()
       defer m.mu.Unlock()
       m.SetupCalled = true
       return m.SetupError
   }
   
   func (m *MockEnvironment) Execute(ctx context.Context, cmd *ExecCommand) (*ExecResult, error) {
       m.mu.Lock()
       defer m.mu.Unlock()
       m.ExecuteCalls = append(m.ExecuteCalls, cmd)
       return m.ExecuteResult, m.ExecuteError
   }
   
   func (m *MockEnvironment) Cleanup() error {
       m.mu.Lock()
       defer m.mu.Unlock()
       m.CleanupCalled = true
       return m.CleanupError
   }
   
   func (m *MockEnvironment) GetBasePath() string {
       return m.BasePath
   }
   
   // Helper methods for tests
   func (m *MockEnvironment) GetExecuteCallCount() int {
       m.mu.Lock()
       defer m.mu.Unlock()
       return len(m.ExecuteCalls)
   }
   
   func (m *MockEnvironment) GetLastExecuteCall() *ExecCommand {
       m.mu.Lock()
       defer m.mu.Unlock()
       if len(m.ExecuteCalls) == 0 {
           return nil
       }
       return m.ExecuteCalls[len(m.ExecuteCalls)-1]
   }
   ```

2. **Create environment tests** (environment/environment_test.go)
   - Test interface implementation
   - Test registry pattern
   - Test ExecCommand struct
   - Test ExecResult struct

3. **Create BSD tests** (environment/bsd/bsd_test.go)
   ```go
   package bsd
   
   import (
       "context"
       "go-synth/config"
       "go-synth/environment"
       "testing"
       "time"
   )
   
   func TestBSDEnvironment_Interface(t *testing.T) {
       var _ environment.Environment = (*BSDEnvironment)(nil)
   }
   
   func TestBSDEnvironment_GetBasePath(t *testing.T) {
       env := &BSDEnvironment{baseDir: "/test/path"}
       if got := env.GetBasePath(); got != "/test/path" {
           t.Errorf("GetBasePath() = %q, want %q", got, "/test/path")
       }
   }
   
   func TestMountPathResolution(t *testing.T) {
       tests := []struct {
           name   string
           spath  string
           sysPath string
           want   string
       }{
           {"dummy", "dummy", "/", "tmpfs"},
           {"system-root", "$/bin", "/", "/bin"},
           {"system-custom", "$/bin", "/custom", "/custom/bin"},
           {"absolute", "/usr/ports", "/", "/usr/ports"},
       }
       
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               env := &BSDEnvironment{
                   cfg: &config.Config{SystemPath: tt.sysPath},
               }
               // Test path resolution logic (extracted to helper function)
               got := env.resolveMountSource(tt.spath)
               if got != tt.want {
                   t.Errorf("resolveMountSource(%q) = %q, want %q", 
                       tt.spath, got, tt.want)
               }
           })
       }
   }
   
   func TestExecCommand_Timeout(t *testing.T) {
       ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
       defer cancel()
       
       env := NewMockEnvironment()
       env.(*MockEnvironment).ExecuteError = context.DeadlineExceeded
       
       cmd := &environment.ExecCommand{
           Command: "/usr/bin/sleep",
           Args:    []string{"10"},
       }
       
       _, err := env.Execute(ctx, cmd)
       if err != context.DeadlineExceeded {
           t.Errorf("Execute() error = %v, want %v", err, context.DeadlineExceeded)
       }
   }
   ```

4. **Create error tests** (environment/errors_test.go)
   - Test error type implementations
   - Test error unwrapping
   - Test helper functions
   - Test error messages

### Files Created ✅
- `environment/mock.go` (195 lines) ✅
- `environment/mock_test.go` (295 lines) ✅
- `environment/environment_test.go` (321 lines) ✅
- `environment/bsd/bsd_test.go` (479 lines) ✅
- Error tests integrated into `environment_test.go` (no separate errors_test.go) ✅

**Total**: 1,290 lines of test code, 38 passing tests

### Testing Checklist
- [x] Mock environment works
- [x] All error types tested
- [x] Path resolution tested
- [x] Context cancellation tested
- [x] No root required for unit tests
- [x] All tests pass
- [x] Run with -race flag
- [x] Coverage >80%

### Success Criteria
- ✅ >80% test coverage for environment package (91.6%)
- ✅ All tests pass without root (38 tests passing)
- ✅ Mock can simulate all scenarios
- ✅ Tests run with -race detector (pass)

### Dependencies
- Tasks 1-5 (Implementation complete)

---

## Task 10: Integration Tests and Documentation

**Priority**: 🟡 Medium  
**Effort**: 4 hours  
**Status**: ✅ Complete (2025-11-28)  
**Completion**: 8 integration tests passing in VM, context timeout bug fixed

### Objective
Validate with real mounts and document everything.

### Implementation Steps

1. **Create integration tests** (environment/bsd/integration_test.go)
    
    **⚠️ Integration tests MUST be run in the DragonFlyBSD VM (Task 0).**
    
    The VM testing infrastructure provides:
    - Clean BSD environment with root access
    - Snapshot-based restoration for repeatable tests
    - No need for `sudo` on host system
    - Fast iteration with automated sync
    
    **VM Workflow for Integration Testing:**
    ```bash
    # First time setup (15 minutes, fully automated)
    make vm-setup         # Download ISO, create disk
    make vm-auto-install  # Automated OS installation
    
    # Daily workflow
    make vm-start         # Boot VM (30s)
    make vm-sync          # Sync code to VM
    make vm-ssh           # SSH into VM
    
    # Inside VM, run integration tests:
    cd /root/go-synth
    doas go test -v -tags=integration ./environment/bsd/
    
    # Or quick workflow (sync + test):
    make vm-quick         # Syncs and runs Phase 4 tests
    
    # When done:
    make vm-stop          # Shut down VM
    ```
    
    **See:** `docs/testing/VM_TESTING.md` for complete documentation.
    
    ```go
    //go:build integration
    // +build integration
    
    package bsd
    
    import (
        "context"
        "go-synth/config"
        "os"
        "os/exec"
        "strings"
        "testing"
    )
    
    func TestBSD_FullLifecycle(t *testing.T) {
        if os.Getuid() != 0 {
            t.Skip("requires root")
        }
        
        cfg := &config.Config{
            BuildBase:     "/tmp/test-build",
            DPortsPath:    "/usr/ports",
            PackagesPath:  "/tmp/test-packages",
            DistFilesPath: "/tmp/test-distfiles",
            OptionsPath:   "/tmp/test-options",
            SystemPath:    "/",
        }
        
        env := NewBSDEnvironment()
        
        // Setup
        if err := env.Setup(99, cfg); err != nil {
            t.Fatalf("Setup() failed: %v", err)
        }
        defer env.Cleanup()
        
        // Verify mounts
        output, err := exec.Command("mount").Output()
        if err != nil {
            t.Fatalf("mount command failed: %v", err)
        }
        
        mountOutput := string(output)
        expectedMounts := []string{
            "/tmp/test-build/SL99 ",
            "/tmp/test-build/SL99/xports",
            "/tmp/test-build/SL99/dev",
        }
        
        for _, mount := range expectedMounts {
            if !strings.Contains(mountOutput, mount) {
                t.Errorf("mount %q not found in output", mount)
            }
        }
        
        // Execute simple command
        cmd := &ExecCommand{
            Command: "/bin/echo",
            Args:    []string{"hello"},
        }
        
        result, err := env.Execute(context.Background(), cmd)
        if err != nil {
            t.Fatalf("Execute() failed: %v", err)
        }
        
        if result.ExitCode != 0 {
            t.Errorf("Execute() exit code = %d, want 0", result.ExitCode)
        }
        
        // Cleanup
        if err := env.Cleanup(); err != nil {
            t.Fatalf("Cleanup() failed: %v", err)
        }
        
        // Verify no mounts remain
        output, _ = exec.Command("mount").Output()
        if strings.Contains(string(output), "/tmp/test-build/SL99") {
            t.Error("mounts still present after cleanup")
        }
    }
    ```

2. **Create environment README** (environment/README.md)
   ```markdown
   # Environment Package
   
   The environment package provides build isolation for go-synth.
   
   ## Overview
   
   The Environment interface abstracts platform-specific isolation mechanisms
   (chroot, jails, containers) from the build orchestration logic.
   
   ## Usage
   
   ### Creating an Environment
   
   ```go
   env, err := environment.New("bsd")
   if err != nil {
       return err
   }
   ```
   
   ### Lifecycle
   
   1. Setup - prepares isolation (mounts, directories)
   2. Execute - runs commands in isolation
   3. Cleanup - tears down isolation
   
   ## Backends
   
   ### BSD (FreeBSD/DragonFly)
   
   Uses nullfs/tmpfs mounts + chroot for isolation.
   
   - Requires root privileges
   - 27 mount points per worker
   - Retry logic for busy filesystems
   
   ## Testing
   
   ### Unit Tests
   
   ```bash
   go test ./environment/...
   ```
   
   ### Integration Tests (requires root)
   
   ```bash
   sudo go test -tags=integration ./environment/bsd/
   ```
   
   ## Adding New Backends
   
   1. Implement Environment interface
   2. Register with environment.Register()
   3. Add tests
   ```

3. **Update PHASE_4_ENVIRONMENT.md**
   - Expand from 35 lines to ~450 lines
   - Add all sections from design analysis
   - Document architecture decisions
   - Document mount topology

4. **Update DEVELOPMENT.md**
   - Mark Phase 4 with detailed task breakdown
   - Update Phase 4 status indicators
   - Update estimated timeline

5. **Add godoc to all exported types**
   - Document Environment interface
   - Document all error types
   - Document ExecCommand/ExecResult
   - Package-level documentation

### Files to Create
- `environment/bsd/integration_test.go` (~200 lines)
- `environment/README.md` (~200 lines)

### Files to Modify
- `docs/design/PHASE_4_ENVIRONMENT.md` (expand to ~450 lines)
- `docs/design/PHASE_4_TODO.md` (this file - mark tasks complete)
- `DEVELOPMENT.md` (Phase 4 status updates)

### Testing Checklist
- [ ] VM infrastructure ready (Task 0) ✅
- [ ] Integration tests pass in VM (with doas)
- [ ] All mounts cleaned up after tests
- [ ] No leftover directories
- [ ] Tests skip gracefully without root (host system)

### Success Criteria
- Integration tests validate full lifecycle
- Documentation comprehensive
- Phase 4 marked complete in DEVELOPMENT.md
- All exported types documented

### Dependencies
- Tasks 1-8 complete

---

## Summary

### Total Effort: 27 hours
- Task 1: Define Interface (2h)
- Task 2: BSD Mount Logic (2h)
- Task 3: BSD Setup (2h)
- Task 4: BSD Execute (2h)
- Task 5: BSD Cleanup (1h)
- Task 6: Update phases.go (3h)
- Task 7: Update Worker (2h)
- Task 8: Context + Errors (3h)
- Task 9: Unit Tests (4h)
- Task 10: Integration + Docs (4h)
- Buffer: 2h (11% contingency)

### Critical Path
Tasks 1 → 2 → 3 → 4 → 5 → 6 → 7 → 9 → 10 (22 hours)

Task 8 can overlap with Tasks 6-7

### Recommended Order

**Week 1 (16 hours):**
1. Day 1-2: Tasks 1-3 (Interface + BSD Setup) - 6h
2. Day 3: Tasks 4-5 (Execute + Cleanup) - 3h
3. Day 4-5: Tasks 6-7 (Update build package) - 5h
4. Day 5: Task 8 (Context + Errors) - 2h

**Week 2 (11 hours):**
1. Day 6-7: Task 9 (Unit Tests) - 4h
2. Day 8-9: Task 10 (Integration + Docs) - 4h
3. Day 9-10: Buffer + final polish - 3h

### Success Metrics

**Performance:**
- Mount setup: <2 seconds per worker
- Cleanup success rate: 100%
- No leftover mounts

**Code Quality:**
- Test coverage: >80%
- No exec.Command("chroot") outside environment package
- No mount operations outside environment package
- All tests pass with -race flag

**Architecture:**
- Clean Environment interface
- BSD implementation fully extracted
- build package platform-agnostic
- Ready for future backends (jails, containers)

### Exit Criteria

- [ ] Environment interface defined and documented
- [ ] BSD implementation complete (setup/execute/cleanup)
- [ ] All mount logic moved to environment package
- [ ] All chroot calls go through Environment.Execute()
- [ ] Workers use Environment for isolation
- [ ] Context support for cancellation/timeout
- [ ] Structured error types
- [ ] >80% test coverage
- [ ] Unit tests pass without root
- [ ] Integration tests pass with root
- [ ] mount package marked deprecated
- [ ] Phase 4 marked complete in DEVELOPMENT.md

---

## Critical Bug Fixed During Task 10

**Bug**: Context timeout handling in Execute() not working properly  
**Discovered**: 2025-11-28, during integration test TestIntegration_ExecuteTimeout  
**Fixed**: 2025-11-28  
**Location**: environment/bsd/bsd.go:421-448

### Problem
When a context times out, `exec.CommandContext` kills the process with a signal (SIGKILL). This results in an `*exec.ExitError`, NOT `context.DeadlineExceeded`. The original error handling code checked for ExitError first and returned success (nil error), then checked for context.DeadlineExceeded which was never reached.

### Root Cause
```go
// WRONG (original order):
if exitErr, ok := err.(*exec.ExitError); ok {
    return result, nil  // Returns success even though context timed out!
}
if errors.Is(err, context.DeadlineExceeded) {
    return error  // Never reached
}
```

### Solution
Check the context state BEFORE checking for ExitError:

```go
// CORRECT (fixed order):
if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
    return error  // Check context FIRST
}
if errors.Is(execCtx.Err(), context.Canceled) {
    return error
}
if exitErr, ok := err.(*exec.ExitError); ok {
    return result, nil  // Only after checking context
}
```

### Impact
- Now properly handles Ctrl+C interrupts
- Command timeouts work correctly
- Integration test TestIntegration_ExecuteTimeout passes
- No more silent failures when context is cancelled

---

**Last Updated**: 2025-11-28  
**Phase Status**: 🟢 Complete
