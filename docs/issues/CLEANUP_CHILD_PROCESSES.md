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

**Next Steps:**
1. Decide on solution approach (Option 3 recommended)
2. Implement context cancellation in BuildContext
3. Add process tracking to BSDEnvironment
4. Update cleanup function to cancel context → wait → unmount
5. Add tests for graceful shutdown
6. Test on VM with SIGINT during active build
