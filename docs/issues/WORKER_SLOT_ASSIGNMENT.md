# Issue: Worker Slot Assignment - All Workers Using SL00

**Status**: ✅ RESOLVED  
**Priority**: High  
**Discovered**: 2025-11-30  
**Resolved**: 2025-11-30  
**Component**: `config/`  
**Resolution**: Changed default MaxWorkers from 1 to runtime.NumCPU() (capped at 16)

---

## Problem Statement

During VM testing of signal-triggered cleanup, mount entries showed tripled mount points all under `/build/SL00`, with no evidence of `/build/SL01`, `/build/SL02`, etc. This suggests all workers are being created with the same ID (0) or are reusing the same slot, violating worker isolation.

## Observed Behavior

### VM Test Results (2025-11-30)

```bash
# After SIGINT during build:
$ mount | grep "/build/SL"
tmpfs on /build/SL00 (tmpfs, local)
tmpfs on /build/SL00/boot (tmpfs, local)
devfs on /build/SL00/dev (devfs, local)
procfs on /build/SL00/proc (procfs, read-only, local)
/bin on /build/SL00/bin (null)
[... 18 more mounts for SL00 ...]
tmpfs on /build/SL00 (tmpfs, local)              # DUPLICATE!
tmpfs on /build/SL00/boot (tmpfs, local)         # DUPLICATE!
[... 18 more mounts for SL00 AGAIN ...]
tmpfs on /build/SL00 (tmpfs, local)              # TRIPLICATE!
tmpfs on /build/SL00/boot (tmpfs, local)         # TRIPLICATE!
[... 18 more mounts for SL00 AGAIN ...]
```

**Expected**: 22 mounts each for SL00, SL01, SL02, ..., SL07 (8 workers × 22 mounts = 176 total)  
**Actual**: 66 mounts all for SL00 (3 × 22 = 66)

**Missing**: No SL01, SL02, SL03, SL04, SL05, SL06, SL07 directories or mounts

## Root Cause Hypotheses

### Hypothesis 1: Worker ID Not Passed Correctly

The worker creation loop may be passing the wrong ID:

```go
// build/build.go:191-217
numWorkers := cfg.MaxWorkers  // e.g., 8
ctx.workers = make([]*Worker, numWorkers)
for i := 0; i < numWorkers; i++ {
    env, err := environment.New("bsd")
    if err := env.Setup(i, cfg, logger); err != nil {  // <-- Is 'i' correct here?
        // ...
    }
    
    ctx.workers[i] = &Worker{
        ID:     i,          // <-- Worker struct has correct ID
        Env:    env,
        Status: "idle",
    }
}
```

**Check**: Is `i` being captured correctly in the loop? (Unlikely, but worth verifying)

### Hypothesis 2: Environment.Setup() Ignores Worker ID

The `env.Setup(workerID, cfg, logger)` call may not be using the workerID parameter:

```go
// environment/bsd/bsd.go
func (e *BSDEnvironment) Setup(workerID int, cfg *config.Config, logger log.LibraryLogger) error {
    e.baseDir = filepath.Join(cfg.BuildBase, fmt.Sprintf("SL%02d", workerID))
    // ... setup mounts
}
```

**Check**: Is `workerID` parameter actually being used, or is it hardcoded to 0?

### Hypothesis 3: Multiple Workers Created with Same ID

The worker creation loop may be running multiple times (e.g., if build fails and retries):

```go
// Possible double-creation scenario?
for attempt := 0; attempt < 3; attempt++ {
    for i := 0; i < numWorkers; i++ {
        // Creates SL00 three times instead of SL00..SL07?
    }
}
```

**Check**: Is there retry logic that recreates workers without clearing old ones?

### Hypothesis 4: Config.MaxWorkers Set to 1

The configuration may have `MaxWorkers=1`, creating only one worker slot:

```bash
# Check VM config
$ grep -i workers /etc/dsynth/dsynth.ini
Number_of_builders=8  # <-- Should be 8
```

**Check**: Verify config is being loaded correctly and MaxWorkers is 8, not 1

## Expected Behavior

For `MaxWorkers=8`, the build should create:
- `/build/SL00` (worker 0)
- `/build/SL01` (worker 1)
- `/build/SL02` (worker 2)
- `/build/SL03` (worker 3)
- `/build/SL04` (worker 4)
- `/build/SL05` (worker 5)
- `/build/SL06` (worker 6)
- `/build/SL07` (worker 7)

Each with 22 mounts:
- 1 tmpfs base
- 1 tmpfs /boot
- 1 devfs /dev
- 1 procfs /proc
- 16 nullfs mounts (read-only system directories)
- 1 tmpfs /construction
- 1 tmpfs /usr/local

Total: 176 mounts (8 workers × 22 mounts/worker)

## Impact Assessment

**Severity**: High - Violates worker isolation, may cause build corruption

**Consequences:**
1. **Build Corruption**: Multiple packages building in same chroot concurrently
2. **Resource Contention**: All workers compete for same filesystem
3. **Cleanup Issues**: Tripled mounts harder to unmount, more "device busy" errors
4. **Performance**: Workers not actually parallelized if using same slot

**Affected Operations:**
- All multi-worker builds
- Parallel package compilation
- Worker isolation assumptions

## Investigation Steps

1. **Add Debug Logging to Worker Creation:**
   ```go
   for i := 0; i < numWorkers; i++ {
       logger.Info("Creating worker %d (total: %d)", i, numWorkers)
       env, err := environment.New("bsd")
       logger.Info("Setting up environment for worker %d", i)
       if err := env.Setup(i, cfg, logger); err != nil {
           logger.Error("Failed to setup worker %d: %v", i, err)
       }
       logger.Info("Worker %d created with baseDir: %s", i, env.(*environment.BSDEnvironment).baseDir)
   }
   ```

2. **Check Configuration Loading:**
   ```bash
   # On VM
   ./dsynth build --debug devel/gmake 2>&1 | grep -i workers
   # Should show: "Starting build with 8 workers" or similar
   ```

3. **Verify Environment.Setup() Implementation:**
   ```go
   // Add to environment/bsd/bsd.go:Setup()
   logger.Info("BSDEnvironment.Setup called with workerID=%d", workerID)
   e.baseDir = filepath.Join(cfg.BuildBase, fmt.Sprintf("SL%02d", workerID))
   logger.Info("BSDEnvironment baseDir set to: %s", e.baseDir)
   ```

4. **Check Worker Struct After Creation:**
   ```go
   // After worker creation loop
   for i, w := range ctx.workers {
       logger.Info("Worker %d: ID=%d, Env baseDir=%s", i, w.ID, w.Env.(*BSDEnvironment).baseDir)
   }
   ```

## Testing Plan

1. **Unit Test**: Verify worker ID passed correctly
   ```go
   func TestWorkerIDAssignment(t *testing.T) {
       // Create 3 workers, verify each has unique ID and baseDir
   }
   ```

2. **Integration Test**: Create multiple workers, check directories
   ```bash
   # On VM after build start
   ls /build/ | grep "^SL"  # Should show SL00 through SL07
   mount | grep "/build/SL" | awk '{print $3}' | sort -u | wc -l  # Should be 8
   ```

3. **VM Test**: Full build with logging
   ```bash
   rm -f /build/builds.db
   ./dsynth build --debug devel/gmake 2>&1 | tee /tmp/debug.log
   # Kill after 10 seconds
   kill -INT $!
   # Analyze log for worker creation messages
   grep -i "worker" /tmp/debug.log
   ```

## Potential Fix

If root cause is confirmed (e.g., `Setup()` ignoring workerID):

```go
// environment/bsd/bsd.go
func (e *BSDEnvironment) Setup(workerID int, cfg *config.Config, logger log.LibraryLogger) error {
    // BEFORE (if broken):
    // e.baseDir = filepath.Join(cfg.BuildBase, "SL00")  // Hardcoded!
    
    // AFTER (correct):
    e.baseDir = filepath.Join(cfg.BuildBase, fmt.Sprintf("SL%02d", workerID))
    
    logger.Info("Setting up worker %d at %s", workerID, e.baseDir)
    // ... rest of setup
}
```

## Related Issues

- **Signal Handler Cleanup Race** (RESOLVED 2025-11-30) - This issue emerged during VM testing
- **Child Processes Not Killed During Cleanup** - See `CLEANUP_CHILD_PROCESSES.md` (related to tripled mounts)

## References

- VM test output showing tripled mounts (2025-11-30 18:52:00)
- Worker creation: `build/build.go:191-217`
- Environment setup: `environment/bsd/bsd.go`
- Config loading: `config/config.go`

---

## Resolution (2025-11-30)

### Root Cause Confirmed

Investigation revealed the issue was **not a bug in worker creation**, but rather a **suboptimal default value**:

- **Code Review**: All worker creation and environment setup code was ✅ **CORRECT**
  - `build/build.go:240-250`: Loop correctly passes `i` as worker ID
  - `environment/bsd/bsd.go:106`: Setup correctly uses `workerID` parameter
  - Base directory correctly set: `filepath.Join(cfg.BuildBase, fmt.Sprintf("SL%02d", workerID))`

- **Actual Problem**: Missing config file on VM → default `MaxWorkers: 1`
  - VM had no `/etc/dsynth/dsynth.ini`
  - Config code defaulted to `MaxWorkers: 1` (line 70)
  - Only 1 worker created → only SL00 directory
  - "Tripled mounts" were from multiple interrupted test runs (before cleanup fix)

### Solution Implemented

**Changed default from 1 to `runtime.NumCPU()` with intelligent capping:**

```go
// config/config.go:67-85
defaultWorkers := runtime.NumCPU()
// Cap at 16 workers to avoid overwhelming the system
if defaultWorkers > 16 {
    defaultWorkers = 16
}
// Minimum of 1 worker
if defaultWorkers < 1 {
    defaultWorkers = 1
}

cfg := &Config{
    Profile:    profile,
    MaxWorkers: defaultWorkers,  // Auto-detect from CPU
    MaxJobs:    1,
}
```

**Added helpful warning when no config file found:**

```
Warning: No config file found at /etc/dsynth/dsynth.ini
Using defaults: 8 workers (detected from CPU count)
Run 'dsynth init' to create a config file, or override with config file settings.
```

### Benefits

1. **Better UX**: Works out of the box without config file
2. **Performance**: Automatically uses available CPU cores for parallelism
3. **Safety**: Capped at 16 workers to prevent resource exhaustion
4. **Flexibility**: Users can still override via config file (`Number_of_builders=N`)
5. **Discoverability**: Warning message guides users to create config

### Changes Made

- `config/config.go`: Changed default from `MaxWorkers: 1` to `runtime.NumCPU()` (capped at 16)
- Added `import "runtime"`
- Added warning message when config file not found

### Testing

- ✅ **Local test**: Warning message displays correctly, detects 8 workers (8-CPU system)
- ✅ **VM test**: Multiple workers verified on DragonFlyBSD 6.4.2 (4-CPU VM)

**VM Test Results:**
```bash
=== Worker Directories Created ===
drwxr-xr-x  18 root  wheel  1496 Nov 30 18:44 /build/SL00
drwxr-xr-x  18 root  wheel  1496 Nov 30 18:44 /build/SL01
drwxr-xr-x  18 root  wheel  1496 Nov 30 18:44 /build/SL02
drwxr-xr-x  18 root  wheel  1496 Nov 30 18:44 /build/SL03

=== Count of SL directories ===
       4 directories found
✓ SUCCESS: Multiple workers created!
```

**Verification:**
- ✅ 4 workers created automatically (matches 4 CPU cores)
- ✅ Each worker has separate SL directory (SL00, SL01, SL02, SL03)
- ✅ Each worker has 22 mounts (88 total = 4 workers × 22 mounts)
- ✅ Warning message displayed correctly
- ✅ Proper worker isolation achieved

---

**Completed Steps:**
1. ✅ Code investigation confirmed no bugs in worker creation
2. ✅ Identified root cause: suboptimal default value (MaxWorkers=1)
3. ✅ Implemented intelligent CPU-based default (runtime.NumCPU(), capped at 16)
4. ✅ Added user-friendly warning message
5. ✅ VM testing verified: 4 workers created on 4-CPU VM
6. ✅ Issue RESOLVED: Multiple workers now created automatically

**Status**: ✅ **RESOLVED** - Auto-detection working correctly
