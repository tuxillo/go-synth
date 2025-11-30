# Issue: Child Processes Not Killed During Signal-Triggered Cleanup

**Status**: 🔴 Open  
**Priority**: High  
**Discovered**: 2025-11-30  
**Component**: `build/`, `environment/bsd`

---

## Problem Statement

When a build is interrupted with SIGINT/SIGTERM, the cleanup function correctly attempts to unmount worker environments, but child processes (e.g., `make`, `pkg`) remain running and prevent unmounting with "device busy" errors.

## Observed Behavior

### VM Test Results (2025-11-30)

```
Received signal terminated, cleaning up...
Cleaning up active build workers...
2025/11/30 18:52:14 [Cleanup] Starting cleanup for environment: /build/SL00
2025/11/30 18:52:14 [Cleanup] Unmounting /build/SL00/usr/local (attempt 1/10)
2025/11/30 18:52:14 [Cleanup] Successfully unmounted /build/SL00/usr/local
2025/11/30 18:52:14 [Cleanup] Unmounting /build/SL00/construction (attempt 1/10)
2025/11/30 18:52:27 [Cleanup] Unmount failed (attempt 1/10): unmount failed for /build/SL00/construction: device busy, retrying in 5s...
```

**Processes still running after SIGINT:**
```bash
root   34659  0.0  0.0   9364   2480 ??  I2      6:51PM   0:00.01 /usr/bin/make
root   34687  0.0  0.0   9020   2528 ??  I2      6:51PM   0:00.02 /usr/bin/make
root   35374  0.0  0.0   9020   2516 ??  I2      6:51PM   0:00.02 /usr/bin/make
root   35378  0.0  0.0   6224   1360 ??  S0      6:51PM   0:00.00 /usr/bin/make
root   35381  0.0  0.0   6224   1372 ??  S0      6:51PM   0:00.00 /usr/bin/make
root   35472  0.0  0.0   6480   1424 ??  S3      6:52PM   0:00.03 /usr/bin/make
```

## Root Cause

The signal handler calls `cleanup()` which invokes `env.Cleanup()` to unmount filesystems, but the worker goroutines and their child processes (spawned via `env.Execute()`) are not terminated first.

### Current Signal Handler Flow

```go
// main.go:588-604
go func() {
    sig := <-sigChan
    fmt.Fprintf(os.Stderr, "\nReceived signal %v, cleaning up...\n", sig)
    
    cleanup := svc.GetActiveCleanup()  // Gets the cleanup function
    if cleanup != nil {
        cleanup()  // Unmounts immediately, but children still running!
    }
    
    _ = svc.Close()
    os.Exit(1)
}()
```

### Current Cleanup Function

```go
// build/build.go:152-165
cleanup := func() {
    logger.Info("Cleaning up worker environments (total workers: %d)", len(ctx.workers))
    for i, worker := range ctx.workers {
        if worker != nil && worker.Env != nil {
            logger.Info("Cleaning up worker %d", i)
            if err := worker.Env.Cleanup(); err != nil {  // Unmount attempt
                logger.Warn("Failed to cleanup worker %d: %v", i, err)
            }
        }
    }
}
```

**Missing**: No mechanism to kill worker goroutines or their child processes before unmounting.

## Expected Behavior

1. Signal received (SIGINT/SIGTERM)
2. **Kill all worker processes and their children** (NEW)
3. **Wait for processes to exit** (NEW)
4. Unmount filesystems (existing)
5. Remove directories (existing)
6. Exit cleanly

## Proposed Solutions

### Option 1: Context Cancellation (Recommended)

Use `context.Context` cancellation to signal workers to stop:

**Pros:**
- Clean, idiomatic Go pattern
- Allows graceful worker shutdown
- Workers can cleanup their own processes

**Cons:**
- Requires adding context support to build loop
- Workers need to check context and terminate children

**Implementation:**
```go
// build/build.go
ctx := &BuildContext{
    ctx:       context.WithCancel(context.Background()),  // Cancellable context
    // ...
}

// Store cancel function for signal handler
cancelBuild := ctx.ctx.Cancel

cleanup := func() {
    logger.Info("Stopping workers...")
    cancelBuild()  // Signal all workers to stop
    
    logger.Info("Waiting for workers to terminate...")
    ctx.wg.Wait()  // Wait for worker goroutines to exit
    
    logger.Info("Cleaning up environments...")
    for i, worker := range ctx.workers {
        if worker != nil && worker.Env != nil {
            if err := worker.Env.Cleanup(); err != nil {
                logger.Warn("Failed to cleanup worker %d: %v", i, err)
            }
        }
    }
}
```

### Option 2: Process Group Termination

Track process PIDs and send SIGTERM to process groups:

**Pros:**
- Direct approach, kills all children immediately
- No need to modify worker loop

**Cons:**
- Requires tracking PIDs from `env.Execute()`
- Platform-specific (PGID handling)
- Less graceful than context cancellation

**Implementation:**
```go
// environment/bsd/bsd.go - Track PIDs
func (e *BSDEnvironment) Execute(cmd ExecCommand) (*ExecResult, error) {
    execCmd := exec.CommandContext(cmd.Ctx, "chroot", args...)
    execCmd.Start()
    
    e.mu.Lock()
    e.activePIDs = append(e.activePIDs, execCmd.Process.Pid)
    e.mu.Unlock()
    
    // ... rest of execution
}

// Add to Cleanup()
func (e *BSDEnvironment) Cleanup() error {
    e.mu.Lock()
    pids := e.activePIDs
    e.mu.Unlock()
    
    // Kill process groups
    for _, pid := range pids {
        syscall.Kill(-pid, syscall.SIGTERM)  // Negative PID kills group
    }
    
    time.Sleep(1 * time.Second)  // Allow graceful shutdown
    
    // Force kill if still running
    for _, pid := range pids {
        syscall.Kill(-pid, syscall.SIGKILL)
    }
    
    // ... then unmount
}
```

### Option 3: Hybrid Approach (Best)

Combine both:
1. Context cancellation for graceful worker shutdown
2. Process tracking as fallback for stuck processes

## Impact Assessment

**Severity**: High - Prevents clean shutdown, leaves stale mounts

**Affected Operations:**
- Ctrl+C during build
- SIGTERM from system
- SIGHUP on terminal disconnect

**Workarounds:**
- Manually kill processes: `pkill -9 make`
- Manually unmount: `umount -f /build/SL*`
- Restore VM from snapshot

## Testing Plan

1. **Unit Test**: Verify context cancellation propagates to workers
2. **Integration Test**: Build with intentional SIGINT, verify clean unmount
3. **VM Test**: Full end-to-end test on DragonFlyBSD
   ```bash
   make vm-build
   (echo "y" | timeout 25 ./dsynth build -f devel/gmake) &
   sleep 10
   kill -INT $!
   # Verify: mount | grep /build/SL should be 0
   # Verify: ps aux | grep make should be 0
   ```

## Related Issues

- **Signal Handler Cleanup Race** (RESOLVED 2025-11-30) - This issue emerged during that fix
- **Worker Slot Assignment Bug** - See `WORKER_SLOT_ASSIGNMENT.md` (all workers using SL00)

## References

- Commit that introduced callback pattern: (pending)
- VM test logs: `/build/logs/` on DragonFlyBSD VM
- Environment abstraction: `environment/bsd/bsd.go`
- Build orchestration: `build/build.go`

---

## Implementation Plan

**Decision**: Proceeding with **Option 3 (Hybrid Approach)** - Context cancellation + Process tracking

**Total Estimated Time**: 9.5 hours  
**Status**: 🟡 In Progress  
**Time Spent**: 8.5 hours (Tasks 1-5 completed)  
**Remaining**: 2 hours (Task 6: VM Testing)

### Task Breakdown

#### Task 1: Add Cancellable Context to BuildContext (2 hours)
**Status**: ✅ Completed  
**Files**: `build/build.go`  
**Description**: Replace `context.Background()` with cancellable context, update cleanup closure to cancel context before unmounting.

**Changes**:
1. ✅ Add `cancel context.CancelFunc` field to `BuildContext` struct
2. ✅ Create cancellable context in `DoBuild()`: `buildCtx, cancel := context.WithCancel(context.Background())`
3. ✅ Update cleanup closure to call `cancel()` before `ctx.wg.Wait()`
4. ✅ Ensure cancel is called on error paths during setup

**Rationale**: Provides graceful shutdown mechanism that workers can detect

---

#### Task 2: Check Context Cancellation in Worker Loop (1.5 hours)
**Status**: ✅ Completed  
**Depends on**: Task 1  
**Files**: `build/build.go`  
**Description**: Update `workerLoop()` to check for context cancellation using `select` statement.

**Changes**:
1. ✅ Replace `for p := range ctx.queue` with `select` checking `ctx.ctx.Done()` and channel
2. ✅ Exit loop gracefully when context is cancelled
3. ✅ Log worker shutdown for debugging

**Rationale**: Allows workers to exit gracefully when context is cancelled

---

#### Task 3: Pass Context to Execute Commands (2 hours)
**Status**: ✅ Completed  
**Depends on**: Task 1  
**Files**: `build/phases.go`, `build/build.go`  
**Description**: Ensure BuildContext's cancellable context propagates to `env.Execute()` calls.

**Changes**:
1. ✅ Verify `executePhase()` receives correct context parameter
2. ✅ Ensure all `env.Execute()` calls use the passed context
3. ✅ Context will propagate to `exec.CommandContext()` in `BSDEnvironment.Execute()`

**Rationale**: Ensures running commands are interrupted when context is cancelled

---

#### Task 4: Add Process Tracking to BSDEnvironment (2.5 hours)
**Status**: ✅ Completed  
**Files**: `environment/bsd/bsd.go`, `environment/bsd/mounts.go`  
**Description**: Track spawned processes and kill them in `Cleanup()` before unmounting.

**Changes**:
1. ✅ Add `activePIDs []int` and `pidMu sync.Mutex` fields to `BSDEnvironment`
2. ✅ Track process PID in `Execute()` after `execCmd.Start()`
3. ✅ Remove PID from tracking after `execCmd.Wait()`
4. ✅ Add `killActiveProcesses()` helper method in `mounts.go:338-397`
5. ✅ Call `killActiveProcesses()` at start of `Cleanup()` before unmounting (`bsd.go:549`)
6. ✅ Process group termination: SIGTERM (graceful, 2s wait) → SIGKILL (forceful)

**Rationale**: Provides forceful termination fallback for processes that don't respond to context cancellation

**Implementation Details**:
- `killActiveProcesses()` method added to `environment/bsd/mounts.go:338-397`
- Uses process group signaling with negative PIDs (`syscall.Kill(-pid, signal)`)
- Two-phase approach: SIGTERM → 2s wait → SIGKILL
- Thread-safe PID list access with mutex
- Integrated into `Cleanup()` flow before unmounting

---

#### Task 5: Verify Signal Handler Integration (0.5 hours)
**Status**: ✅ Completed  
**Depends on**: Tasks 1-4  
**Files**: `main.go`, `service/build.go`, `service/service.go`  
**Description**: Verify signal handler flow with new cleanup behavior.

**Changes**:
1. ✅ Review signal handler code (no changes needed)
2. ✅ Verify cleanup closure captures BuildContext correctly
3. ✅ Ensure flow is: signal → get cleanup → call cleanup (cancel → wait → kill → unmount) → close → exit

**Verification Results**:
- Signal handler correctly calls `svc.GetActiveCleanup()` (`main.go:594`)
- Cleanup function registered immediately when workers created (`service/build.go:69`)
- Cleanup flow verified: cancel context → wait for workers → kill processes → unmount → remove dirs
- Context cancellation propagates through: BuildContext → workerLoop → buildPackage → executePhase → env.Execute()
- All integration points correct, no code changes needed

**Rationale**: Ensure all pieces work together correctly

---

#### Task 6: VM Testing & Validation (2 hours)
**Status**: ⚪ Pending  
**Depends on**: Tasks 1-5  
**Files**: VM environment  
**Description**: Test on DragonFlyBSD VM with real builds and SIGINT.

**Test Procedure**:
1. Build dsynth: `make vm-build`
2. Start long build: `./dsynth build devel/gmake`
3. Wait 10 seconds for build to start
4. Send SIGINT: `kill -INT <pid>` or Ctrl+C
5. Verify:
   - "Stopping build workers..." message
   - "Waiting for workers to finish..." message
   - Workers exit within 5 seconds
   - No "device busy" errors
   - `mount | grep /build/SL` returns empty
   - `ps aux | grep make` returns empty

**Success Criteria**:
- ✅ Workers exit gracefully within 5 seconds
- ✅ All child processes terminated before unmount
- ✅ All mounts successfully unmounted
- ✅ No "device busy" errors
- ✅ Base directories removed cleanly

**Rationale**: Only way to validate the fix works in real environment

---

### Deferred Tasks (Optional)

- **Unit Tests** (2h): Test context cancellation and cleanup logic
- **Integration Tests** (2h): Automated signal handling tests (covered by VM testing)
- **Documentation Updates** (1h): Update DEVELOPMENT.md, environment/README.md after validation

---

## Implementation Notes

### Why Hybrid Approach?

1. **Context Cancellation** (graceful):
   - Idiomatic Go pattern
   - Respects ongoing work (finish current phase)
   - Clean shutdown for cooperative processes

2. **Process Tracking** (forceful):
   - Fallback for stuck/unresponsive processes
   - Guarantees cleanup even with misbehaving builds
   - Platform-specific but necessary for BSD mounts

3. **Together**:
   - Best of both worlds: try graceful, fallback to forceful
   - Robust against all failure modes
   - Users see fast response to Ctrl+C

### Key Decisions

- **Context cancellation first**: Give workers 5 seconds to finish current operation
- **Process group killing**: SIGTERM (-PID) kills process tree, not just parent
- **Timing**: 2s for SIGTERM, 1s for SIGKILL (total 3s max delay)
- **Fail-safe**: Even if process kill fails, continue with unmount retries

---

**Next Steps:**
1. ✅ Document implementation plan
2. ✅ Implement Task 1 (cancellable context)
3. ✅ Implement Task 2 (worker loop)
4. ✅ Implement Task 3 (pass context - verified, already correct)
5. ✅ Implement Task 4 (process tracking)
6. ✅ Verify Task 5 (signal handler integration)
7. ⚪ **NEXT**: Validate Task 6 (VM testing) - **REQUIRES VM ENVIRONMENT**

**Current Status**: Implementation complete, ready for VM testing
