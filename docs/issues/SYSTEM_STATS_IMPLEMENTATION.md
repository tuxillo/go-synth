# System Stats Implementation

**Status**: 🔵 OPEN  
**Priority**: High  
**Created**: 2025-12-02  
**Component**: `build/`, `log/`, new `stats/` package  
**Depends On**: Phase 3 (Builder Integration), Phase 4 (Environment Abstraction)

---

## Problem Statement

go-synth currently lacks the real-time system statistics monitoring that the original dsynth provides. Users cannot see:

- Active worker count and dynamic worker throttling status
- Package build rate (packages/hour) and impulse (instant completions)
- System load average (adjusted for page-fault waits)
- Swap usage percentage
- Elapsed build time
- Build totals (queued, built, failed, ignored, skipped)

Without these metrics, users have no visibility into build progress, system health, or performance bottlenecks. The original dsynth provides this via:
1. Ncurses UI (live updating top panel)
2. Stdout text output (periodic status lines)
3. `monitor.dat` file (external monitoring/web UI consumption)

## Expected Behavior

### Original dsynth Stats Display

**Ncurses UI (top panel)**:
```
Workers:  4 / 4    Load: 3.24  Swap:  2%    [DynMax: 4]
Elapsed: 00:15:43  Rate: 24.3 pkg/hr  Impulse: 3
Queued: 142  Built: 38  Failed: 2  Ignored: 0  Skipped: 5
```

**Stdout Text Mode**:
```
[00:15:43] Load 3.24/4.00 Swap 2% Rate 24.3/hr Built 38 Failed 2
```

**Monitor File (`monitor.dat`)**:
```
Load=3.24
Swap=2
Workers=4/4
DynMax=4
Rate=24.3
Impulse=3
Elapsed=943
Queued=142
Built=38
Failed=2
Ignored=0
Skipped=5
```

### Key Metrics Defined

| Metric | Description | Source | Update Freq |
|--------|-------------|--------|-------------|
| **Workers** | Active/Total worker count | Build orchestrator | 1 Hz |
| **DynMax** | Dynamic max workers (throttled) | Load/swap-based throttling | 1 Hz |
| **Load** | Adjusted 1-min load avg | `getloadavg()` + `vm.vmtotal.t_pw` | 1 Hz |
| **Swap** | Swap usage percentage | `vm.swap_info` sysctl or `kvm_getswapinfo` | 1 Hz |
| **Rate** | Packages/hour (60s avg) | Completion timestamp sliding window | 1 Hz |
| **Impulse** | Instant completions/sec | Current 1s bucket | 1 Hz |
| **Elapsed** | Build duration | Start timestamp → now | 1 Hz |
| **Totals** | Queued/Built/Failed/etc | Build state tracking | On state change |

## Investigation Summary

### Step 1: Original dsynth Source Analysis

**Files Analyzed**:
- `.original-c-source/dsynth.h:469-500` - `topinfo_t` struct, `runstats_t` interface
- `.original-c-source/build.c` - `waitbuild()` sampling loop, `adjloadavg()`, dynamic throttling

**Key Findings**:

#### `topinfo_t` Structure (dsynth.h:469-485)
```c
typedef struct topinfo {
    int h;              // ncurses LINES
    int w;              // ncurses COLS
    int pkgmax;         // max origin length (display width)
    int active;         // active workers
    double rate;        // packages/hour
    double impulse;     // instant completions
    double dload[3];    // adjusted load avg (1/5/15 min)
    int dswap;          // swap percentage
    time_t start_time;  // build start
    time_t elapsed;     // elapsed seconds
    // Build totals
    int total;          // queued count
    int successful;     // built
    int failed;         // failed
    int ignored;        // ignored
    int remaining;      // queued - (successful + failed + ignored)
    int skipped;        // skipped
    int dynmax;         // dynamic max workers
} topinfo_t;
```

#### `runstats_t` Interface (dsynth.h:491-500)
```c
typedef struct runstats {
    void (*init)(void);
    void (*done)(void);
    void (*reset)(void);
    void (*update)(worker_t *work, const char *portdir);
    void (*updateTop)(topinfo_t *info);
    void (*updateLogs)(void);
    void (*sync)(void);
} runstats_t;
```

**Call Sites**:
- `runstats.init()` - Initialize stats system (ncurses setup or monitor file)
- `runstats.updateTop(&topinfo)` - Update top panel/status line (1 Hz)
- `runstats.update(worker, portdir)` - Worker status change (WMSG events)
- `runstats.sync()` - Flush monitor file atomically
- `runstats.done()` - Cleanup (ncurses teardown)

#### Dynamic Worker Throttling (build.c)

**Sampling Loop** (`waitbuild()` - runs every 1 second):
1. Query adjusted load via `adjloadavg(&topinfo.dload[0])`
2. Query swap percentage via `getswappct(&topinfo.dswap)`
3. Calculate dynamic max workers:
   - **High load threshold**: `load > 2.0 * ncpus` → reduce to 75% of max workers
   - **High swap threshold**: `swap > 10%` → reduce to 50% of max workers
   - Otherwise: use configured max workers
4. If current active > dynmax: delay new job assignment
5. Update `topinfo` and call `runstats.updateTop(&topinfo)`

**Adjusted Load Average** (`adjloadavg()`):
```c
void adjloadavg(double *dload) {
    getloadavg(dload, 3);  // Standard 1/5/15 min load
    
    // Add page-fault waiting processes to 1-min load
    struct vmtotal vmt;
    size_t len = sizeof(vmt);
    sysctlbyname("vm.vmtotal", &vmt, &len, NULL, 0);
    dload[0] += (double)vmt.t_pw;  // Processes waiting on page faults
}
```

**Rationale**: Standard load average doesn't account for disk I/O waits. Adding `vm.vmtotal.t_pw` gives more accurate picture of system stress during I/O-heavy builds.

**Swap Percentage** (`getswappct()`):
```c
int getswappct(int *swap_pct) {
    struct kvm_swap swapinfo;
    kvm_t *kd = kvm_open(NULL, NULL, NULL, O_RDONLY, "kvm_open");
    kvm_getswapinfo(kd, &swapinfo, 1, 0);
    kvm_close(kd);
    
    if (swapinfo.ksw_total > 0) {
        *swap_pct = (swapinfo.ksw_used * 100) / swapinfo.ksw_total;
    }
    return 0;
}
```

**Alternative (sysctl-based)**:
```c
// vm.swap_info sysctl returns array of swap devices
// Sum ksw_used / ksw_total across all devices
```

### Step 2: Missing Upstream Files

**Files NOT in `.original-c-source/` that need analysis**:
- `monitor.c` - Monitor file writer implementation (format, locking, atomic writes)
- `curses.c` / `ncurses.c` - Full ncurses UI implementation (layout, refresh logic)

**Action Required**: Fetch latest dsynth sources from upstream:
```bash
git clone https://github.com/DragonFlyBSD/DragonFlyBSD.git
# Extract: usr.bin/dsynth/monitor.c, curses.c
```

### Step 3: Rate & Impulse Calculation

**Original Implementation** (inferred from behavior):
- **Data Structure**: 60-element ring buffer (1-second buckets)
- **Sampling**: 1 Hz ticker advances current bucket index
- **Recording**: On package completion, increment `buckets[currentBucket]`
- **Rate Calculation**: `sum(buckets[0..59]) * 3600 / 60` (completions/hour, 60s window)
- **Impulse Calculation**: `buckets[currentBucket]` (instant completions in last 1s)
- **Wraparound**: `currentBucket = (currentBucket + 1) % 60`

**Example**:
```
Time:    0s   1s   2s   3s  ...  59s  60s (wrap to 0s)
Bucket:  [0] [1] [2] [3] ... [59] [0]
Counts:   3   0   1   2  ...   0   4

Rate = (3+0+1+2+...+0) * 3600 / 60 = total_in_window * 60 pkg/hr
Impulse = buckets[currentBucket] = 4 (in last second)
```

## Proposed Solution: 7-Phase Implementation Plan

### Phase 1: Locate and Document Original dsynth Stats Plumbing

**Estimated Time**: 3 hours

#### 1a. Directory Inventory (30 min)
- ✅ **Completed**: Analyzed `.original-c-source/` directory
- **Files Present**: `dsynth.h`, `build.c`
- **Files Missing**: `monitor.c`, `curses.c`

#### 1b. Header Deep-Dive (45 min)
- ✅ **Completed**: Extracted `topinfo_t` (15 fields) and `runstats_t` (7 callbacks)
- **Documented**: All metric fields, data types, and semantic meanings

#### 1c. Source Correlation (45 min)
- ✅ **Completed**: Mapped `runstats` call sites in `build.c`
- **Call Sites**:
  - `waitbuild()` main loop: `runstats.updateTop()` @ 1 Hz
  - Worker state changes: `runstats.update(worker, portdir)` on WMSG
  - Periodic flush: `runstats.sync()` every 2 seconds
  - Lifecycle: `runstats.init()` before build, `runstats.done()` after

#### 1d. External Reference Gathering (30 min)
- **Action Required**: Clone upstream DragonFlyBSD repository
- **Target Files**: `usr.bin/dsynth/monitor.c`, `usr.bin/dsynth/curses.c`
- **Purpose**: Understand monitor file format (text vs binary) and ncurses layout

#### 1e. Documentation Artifacts (30 min)
- **Deliverable**: Metric mapping table (see "Key Metrics Defined" above)
- **Additions**: Document adjloadavg/getswappct algorithms, throttling thresholds

---

### Phase 2: Review C Implementation - Metrics & Data Flow

**Estimated Time**: 4 hours  
**Depends On**: Phase 1d (upstream source fetch)

#### 2a. Track RunStats Hooks (1 hour)
- **Task**: Document all `runstats.*()` call contexts
- **Deliverable**: Call graph showing init → update loop → sync → done lifecycle
- **Focus Areas**:
  - When is `topinfo` populated vs passed to callbacks?
  - Threading model: is `topinfo` shared across threads?
  - Lock requirements for stat updates

#### 2b. Decode topinfo_t Population (1 hour)
- **Task**: Analyze `waitbuild()` throttling logic in detail
- **Questions**:
  - Exact formula for `dynmax` calculation (75% vs 50% reduction logic)
  - Hysteresis behavior (does it ramp up gradually or jump back to max?)
  - Edge cases (what if load drops while swap is high?)

#### 2c. Instrumentation Functions (1 hour)
- **Task**: Study `adjloadavg()`, `getswappct()`, watchdog scaling
- **adjloadavg()**: Verify `vm.vmtotal.t_pw` extraction
- **getswappct()**: Document kvm vs sysctl approach (Go should use sysctl)
- **Watchdog**: Check if dsynth has timeout/stall detection using stats

#### 2d. Worker Status Flow (30 min)
- **Task**: Follow WMSG status updates from worker → build.c → runstats
- **WMSG Types**: RUNNING, SUCCESS, FAILURE, IGNORED, SKIPPED
- **Question**: When does `topinfo.active` increment/decrement?

#### 2e. Missing Source Recon (30 min)
- **Task**: List all missing implementations from upstream
- **Files Needed**:
  - `monitor.c` - File format (line-based text? JSON?)
  - `curses.c` - Panel layout (how many lines/columns for stats?)
  - Any helper utilities for rate calculation

#### 2f. Summarize Metrics & Update Frequency (1 hour)
- **Deliverable**: Comprehensive table with:
  - Metric name, type, source function, update frequency, threading notes
- **Example**:
  | Metric | Type | Source | Frequency | Notes |
  |--------|------|--------|-----------|-------|
  | Load | float64 | adjloadavg() | 1 Hz | Add vm.vmtotal.t_pw |
  | Swap | int | getswappct() | 1 Hz | Use sysctl in Go |
  | Rate | float64 | completion events | 1 Hz | 60s sliding window |

---

### Phase 3: Go Port Strategy for System Stats

**Estimated Time**: 6 hours

#### 3a. Collector Architecture (1.5 hours)
- **Package**: `stats/` (new)
- **Core Type**: `StatsCollector` struct
  ```go
  type StatsCollector struct {
      mu            sync.RWMutex
      topInfo       TopInfo           // Current snapshot
      rateBuckets   [60]int           // 1-second buckets for rate calculation
      currentBucket int               // Ring buffer index
      startTime     time.Time         // Build start timestamp
      ticker        *time.Ticker      // 1 Hz sampling
      consumers     []StatsConsumer   // UI, monitor writer, etc.
      ctx           context.Context   // Cancellation
      cancel        context.CancelFunc
  }
  ```

- **Public API**:
  ```go
  // Initialize and start 1 Hz sampling loop
  func NewStatsCollector(ctx context.Context) *StatsCollector

  // Record package completion (updates rate/impulse)
  func (sc *StatsCollector) RecordCompletion(pkg *pkg.Package, status BuildStatus)

  // Update worker status (active count, current package)
  func (sc *StatsCollector) RecordWorkerUpdate(workerID int, status WorkerStatus, pkgName string)

  // Get current snapshot (thread-safe read)
  func (sc *StatsCollector) GetSnapshot() TopInfo

  // Register consumer (UI, monitor writer)
  func (sc *StatsCollector) AddConsumer(consumer StatsConsumer)

  // Shutdown
  func (sc *StatsCollector) Close() error
  ```

- **Internal Methods**:
  ```go
  // 1 Hz ticker loop (goroutine)
  func (sc *StatsCollector) run()

  // Sample system metrics (load, swap) - called by run()
  func (sc *StatsCollector) sampleSystemMetrics()

  // Calculate rate from ring buffer
  func (sc *StatsCollector) calculateRate() float64

  // Notify all consumers with updated TopInfo
  func (sc *StatsCollector) notifyConsumers()
  ```

#### 3b. Metric Acquisition (2 hours)

**3b.1: Adjusted Load Average (45 min)**
```go
// stats/metrics_bsd.go
func getAdjustedLoad() (float64, error) {
    // Standard load average
    var loadavg [3]float64
    if err := unix.Getloadavg(loadavg[:]); err != nil {
        return 0, err
    }
    
    // Get vm.vmtotal for page-fault waiting processes
    var vmtotal unix.Vmtotal
    mib := []int32{unix.CTL_VM, unix.VM_TOTAL}
    if err := unix.Sysctl(mib, &vmtotal); err != nil {
        return 0, err
    }
    
    // Adjusted load = 1-min load + page-fault waits
    adjusted := loadavg[0] + float64(vmtotal.T_pw)
    return adjusted, nil
}
```

**3b.2: Swap Percentage (45 min)**
```go
// stats/metrics_bsd.go
func getSwapUsage() (int, error) {
    // Query vm.swap_info sysctl
    // Returns array of swap devices with ksw_used/ksw_total
    
    // Pseudo-code (actual sysctl call needs syscall.Sysctl)
    mib := []int32{unix.CTL_VM, unix.VM_SWAPINFO}
    // ... extract swap device array
    
    var totalUsed, totalSize uint64
    for _, swapDev := range swapDevices {
        totalUsed += swapDev.Used
        totalSize += swapDev.Total
    }
    
    if totalSize == 0 {
        return 0, nil  // No swap configured
    }
    
    pct := int((totalUsed * 100) / totalSize)
    return pct, nil
}
```

**Note**: If `vm.swap_info` sysctl is complex, fallback to parsing `swapinfo` command output.

**3b.3: Memory Tracking (15 min)**
```go
// Already tracked in BuildContext
// Just expose via StatsCollector API:
func (sc *StatsCollector) UpdateMemoryUsage(runningDepSize, totalDepSize int64)
```

**3b.4: Rate/Impulse (15 min)**
```go
func (sc *StatsCollector) calculateRate() float64 {
    sc.mu.RLock()
    defer sc.mu.RUnlock()
    
    sum := 0
    for _, count := range sc.rateBuckets {
        sum += count
    }
    
    // Packages per hour: (completions in 60s) * 60 min/hr
    rate := float64(sum * 60)
    return rate
}

func (sc *StatsCollector) getImpulse() int {
    sc.mu.RLock()
    defer sc.mu.RUnlock()
    return sc.rateBuckets[sc.currentBucket]
}
```

#### 3c. Data Model (30 min)
```go
// stats/types.go

// TopInfo mirrors dsynth's topinfo_t
type TopInfo struct {
    ActiveWorkers   int       // Currently building
    MaxWorkers      int       // Configured max
    DynMaxWorkers   int       // Dynamic max (throttled)
    
    Load            float64   // Adjusted 1-min load
    SwapPct         int       // Swap usage percentage
    
    Rate            float64   // Packages/hour (60s window)
    Impulse         int       // Instant completions/sec
    
    Elapsed         time.Duration  // Time since build start
    StartTime       time.Time
    
    // Build totals
    Queued          int
    Built           int
    Failed          int
    Ignored         int
    Skipped         int
    Remaining       int  // Queued - (Built + Failed + Ignored)
}

// WorkerStatus for per-worker tracking
type WorkerStatus int
const (
    WorkerIdle WorkerStatus = iota
    WorkerRunning
    WorkerWaiting
)

// StatsConsumer interface for UI/monitor writer
type StatsConsumer interface {
    OnStatsUpdate(info TopInfo)
}
```

#### 3d. Hook Integration (1 hour)
- **Task**: Modify `build/build.go` to create and use `StatsCollector`
- **Changes**:
  ```go
  // build/build.go
  type BuildContext struct {
      // ... existing fields
      stats *stats.StatsCollector  // NEW
  }
  
  func DoBuild(...) error {
      statsCollector := stats.NewStatsCollector(buildCtx)
      defer statsCollector.Close()
      
      ctx := &BuildContext{
          // ...
          stats: statsCollector,
      }
      
      // Register UI as consumer
      ctx.stats.AddConsumer(ctx.ui)
      
      // Register monitor writer (if enabled)
      if cfg.MonitorFile != "" {
          monWriter := stats.NewMonitorWriter(cfg.MonitorFile)
          ctx.stats.AddConsumer(monWriter)
          defer monWriter.Close()
      }
      
      // ... rest of build
  }
  
  // In buildPackage():
  func (ctx *BuildContext) buildPackage(p *pkg.Package) {
      ctx.stats.RecordWorkerUpdate(workerID, stats.WorkerRunning, p.PortDir)
      
      // ... execute build phases
      
      if success {
          ctx.stats.RecordCompletion(p, stats.BuildSuccess)
      } else {
          ctx.stats.RecordCompletion(p, stats.BuildFailed)
      }
      
      ctx.stats.RecordWorkerUpdate(workerID, stats.WorkerIdle, "")
  }
  ```

#### 3e. Monitor Writer & UI Consumers (1 hour)
```go
// stats/monitor.go
type MonitorWriter struct {
    path     string
    lockPath string  // monitor.lk
    mu       sync.Mutex
}

func (mw *MonitorWriter) OnStatsUpdate(info TopInfo) {
    mw.mu.Lock()
    defer mw.mu.Unlock()
    
    // Acquire flock on monitor.lk
    lockFd := syscall.Open(mw.lockPath, os.O_CREATE|os.O_RDWR, 0644)
    syscall.Flock(lockFd, syscall.LOCK_EX)
    defer syscall.Flock(lockFd, syscall.LOCK_UN)
    defer syscall.Close(lockFd)
    
    // Write to temp file
    tmpPath := mw.path + ".tmp"
    content := fmt.Sprintf(`Load=%.2f
Swap=%d
Workers=%d/%d
DynMax=%d
Rate=%.1f
Impulse=%d
Elapsed=%d
Queued=%d
Built=%d
Failed=%d
Ignored=%d
Skipped=%d
`, info.Load, info.SwapPct, info.ActiveWorkers, info.MaxWorkers,
   info.DynMaxWorkers, info.Rate, info.Impulse, int(info.Elapsed.Seconds()),
   info.Queued, info.Built, info.Failed, info.Ignored, info.Skipped)
    
    ioutil.WriteFile(tmpPath, []byte(content), 0644)
    
    // Atomic rename
    os.Rename(tmpPath, mw.path)
}
```

**UI Consumer**: Already implemented via `BuildUI` interface - just call `ui.UpdateStats(info)`.

#### 3f. Configuration & Testing (1 hour)
- **Config Options**:
  ```ini
  [Global Configuration]
  Enable_monitor=yes           # Write monitor.dat
  Monitor_file=/build/monitor.dat
  Stats_update_freq=1          # Hz (fixed at 1, future: configurable)
  ```

- **Unit Tests**:
  - `stats/collector_test.go`: Test rate calculation with mock completions
  - `stats/metrics_test.go`: Mock sysctl calls, test adjloadavg/swap logic
  - `stats/monitor_test.go`: Verify file format matches original dsynth

- **Integration Tests**:
  - Build small port, verify stats update every second
  - Simulate high load/swap, verify dynmax throttling

---

### Phase 4: Integrate Stats into UI/CLI Outputs

**Estimated Time**: 4 hours

#### 4.1 Define Unified TopInfo Payload (30 min)
- ✅ **Completed in Phase 3c**: `TopInfo` struct defined
- **Task**: Ensure all UI consumers use same struct (no duplication)

#### 4.2 Ncurses UI Enhancements (1.5 hours)
```go
// build/ui_ncurses.go

func (ui *NcursesUI) OnStatsUpdate(info stats.TopInfo) {
    ui.app.QueueUpdateDraw(func() {
        // Update top panel (header box)
        header := fmt.Sprintf(
            "Workers: %2d/%2d  Load: %4.2f  Swap: %2d%%  [DynMax: %d]\n" +
            "Elapsed: %s  Rate: %.1f pkg/hr  Impulse: %d\n" +
            "Queued: %d  Built: %d  Failed: %d  Ignored: %d  Skipped: %d",
            info.ActiveWorkers, info.MaxWorkers, info.Load, info.SwapPct,
            info.DynMaxWorkers, formatDuration(info.Elapsed), info.Rate,
            info.Impulse, info.Queued, info.Built, info.Failed, info.Ignored,
            info.Skipped)
        
        ui.headerBox.SetText(header)
        
        // Highlight dynmax if throttling
        if info.DynMaxWorkers < info.MaxWorkers {
            ui.headerBox.SetBorderColor(tcell.ColorYellow)
        } else {
            ui.headerBox.SetBorderColor(tcell.ColorWhite)
        }
    })
}
```

#### 4.3 Stdout Text UI (1 hour)
```go
// build/ui_stdout.go

func (ui *StdoutUI) OnStatsUpdate(info stats.TopInfo) {
    // Print condensed status line every 5 seconds (reduce spam)
    if time.Since(ui.lastPrint) < 5*time.Second {
        return
    }
    ui.lastPrint = time.Now()
    
    fmt.Printf("[%s] Load %.2f Swap %d%% Rate %.1f/hr Built %d Failed %d\n",
        formatDuration(info.Elapsed), info.Load, info.SwapPct,
        info.Rate, info.Built, info.Failed)
}
```

#### 4.4 Monitor/CLI Consumers (30 min)
- **Monitor Writer**: Already implemented in Phase 3e
- **CLI Consumer**: Add `go-synth status --watch` command
  ```go
  // cmd/status.go
  func watchMonitor(monitorPath string) {
      for {
          data, _ := ioutil.ReadFile(monitorPath)
          // Parse monitor.dat and display
          fmt.Print(string(data))
          time.Sleep(1 * time.Second)
      }
  }
  ```

#### 4.5 Dynamic Worker Feedback (30 min)
- **Throttle Notification**: When `dynmax < max`, show warning
  ```go
  if info.DynMaxWorkers < info.MaxWorkers {
      reason := ""
      if info.Load > float64(runtime.NumCPU())*2.0 {
          reason = "high load"
      } else if info.SwapPct > 10 {
          reason = "high swap"
      }
      ui.ShowWarning(fmt.Sprintf("Workers throttled to %d/%d (%s)",
          info.DynMaxWorkers, info.MaxWorkers, reason))
  }
  ```

#### 4.6 Testing & Validation (1 hour)
- **Test Cases**:
  1. Normal build: stats update smoothly, rate increases over time
  2. High load simulation: `stress-ng --cpu 16` → verify throttling
  3. High swap simulation: Fill memory → verify throttling
  4. Burst completions: Fast-building ports → verify impulse spikes
  5. Long build: Verify elapsed time formatting (HH:MM:SS)

---

### Phase 5: Package Rate & Impulse (Expanded)

**Estimated Time**: 3 hours

#### 5a. Define Metrics (15 min)
- **Impulse**: Instant completions in current 1-second bucket
  - Range: 0 to N (where N = number of fast ports completing simultaneously)
  - Useful for: Detecting burst activity, showing real-time progress
- **Rate**: Smoothed average over 60-second sliding window
  - Units: Packages per hour (pkg/hr)
  - Calculation: `sum(buckets[0..59]) * 60`
  - Useful for: Estimating completion time, comparing build performance

#### 5b. Data Sources (15 min)
- **Capture Point**: `StatsCollector.RecordCompletion(pkg, status)`
- **Event Types**:
  - Success → increment bucket
  - Failure → increment bucket (counts as completed work)
  - Skipped → DO NOT increment (not built)
  - Ignored → increment bucket (decision made, counts as completion)

#### 5c. Collector Design (45 min)
```go
// stats/collector.go

type StatsCollector struct {
    // ... other fields
    rateBuckets   [60]int  // Ring buffer: rateBuckets[i] = completions in second i
    currentBucket int      // Index 0..59, increments every second
    bucketTime    time.Time // Time when currentBucket started
}

func (sc *StatsCollector) RecordCompletion(pkg *pkg.Package, status BuildStatus) {
    sc.mu.Lock()
    defer sc.mu.Unlock()
    
    // Check if status should count toward rate
    if status == BuildSkipped {
        return  // Skipped ports don't count
    }
    
    // Ensure we're in correct bucket (handle clock skew)
    sc.advanceBucketIfNeeded()
    
    // Increment current bucket
    sc.rateBuckets[sc.currentBucket]++
}
```

#### 5d. Sampling Loop (1 hour)
```go
func (sc *StatsCollector) run() {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            sc.tick()
        case <-sc.ctx.Done():
            return
        }
    }
}

func (sc *StatsCollector) tick() {
    sc.mu.Lock()
    
    // Advance to next bucket
    sc.currentBucket = (sc.currentBucket + 1) % 60
    sc.rateBuckets[sc.currentBucket] = 0  // Clear new bucket
    sc.bucketTime = time.Now()
    
    // Sample system metrics (load, swap)
    load, _ := getAdjustedLoad()
    swap, _ := getSwapUsage()
    
    // Update TopInfo
    sc.topInfo.Load = load
    sc.topInfo.SwapPct = swap
    sc.topInfo.Elapsed = time.Since(sc.startTime)
    sc.topInfo.Rate = sc.calculateRateLocked()
    sc.topInfo.Impulse = sc.rateBuckets[(sc.currentBucket+59)%60]  // Previous bucket
    
    // Check dynamic throttling
    sc.updateDynMaxLocked()
    
    snapshot := sc.topInfo
    sc.mu.Unlock()
    
    // Notify consumers (outside lock)
    sc.notifyConsumers(snapshot)
}

func (sc *StatsCollector) calculateRateLocked() float64 {
    sum := 0
    for _, count := range sc.rateBuckets {
        sum += count
    }
    return float64(sum * 60)  // Packages per hour
}

func (sc *StatsCollector) updateDynMaxLocked() {
    maxWorkers := sc.topInfo.MaxWorkers
    ncpus := runtime.NumCPU()
    
    // High load threshold: reduce to 75%
    if sc.topInfo.Load > float64(ncpus)*2.0 {
        sc.topInfo.DynMaxWorkers = maxWorkers * 3 / 4
        return
    }
    
    // High swap threshold: reduce to 50%
    if sc.topInfo.SwapPct > 10 {
        sc.topInfo.DynMaxWorkers = maxWorkers / 2
        return
    }
    
    // Normal: use full capacity
    sc.topInfo.DynMaxWorkers = maxWorkers
}
```

#### 5e. Integration with topinfo (15 min)
- ✅ Already covered in `tick()` method above
- **Fields Updated**:
  - `topInfo.Rate` ← `calculateRateLocked()`
  - `topInfo.Impulse` ← `rateBuckets[prevBucket]`

#### 5f. Persistence (15 min)
- **Monitor File**: Already includes Rate/Impulse in Phase 3e
- **BuildDB**: Optionally store historical rates for analysis
  ```go
  type BuildRunStats struct {
      RunID      string
      Timestamp  time.Time
      PeakRate   float64
      AvgRate    float64
      PeakLoad   float64
  }
  ```

#### 5g. Testing (30 min)
```go
// stats/rate_test.go

func TestRateCalculation(t *testing.T) {
    sc := &StatsCollector{rateBuckets: [60]int{}}
    
    // Simulate burst: 10 completions in bucket 0
    sc.rateBuckets[0] = 10
    rate := sc.calculateRateLocked()
    assert.Equal(t, 600.0, rate)  // 10 * 60 pkg/hr
    
    // Simulate sustained: 1 completion per second for 60s
    for i := 0; i < 60; i++ {
        sc.rateBuckets[i] = 1
    }
    rate = sc.calculateRateLocked()
    assert.Equal(t, 3600.0, rate)  // 60 * 60 pkg/hr
}

func TestBucketRollover(t *testing.T) {
    sc := NewStatsCollector(context.Background())
    sc.currentBucket = 59
    
    sc.tick()
    assert.Equal(t, 0, sc.currentBucket)  // Wrapped to 0
    assert.Equal(t, 0, sc.rateBuckets[0]) // Bucket cleared
}

func TestImpulseTracking(t *testing.T) {
    sc := NewStatsCollector(context.Background())
    
    // Record 5 completions in current second
    for i := 0; i < 5; i++ {
        sc.RecordCompletion(mockPkg, BuildSuccess)
    }
    
    // Tick to next bucket
    sc.tick()
    
    // Impulse should reflect previous bucket
    assert.Equal(t, 5, sc.topInfo.Impulse)
}
```

---

### Phase 6: Monitor File & Lock (Expanded)

**Estimated Time**: 3 hours

#### 6a. Understand Original Semantics (30 min)
- **Action**: Fetch and analyze `usr.bin/dsynth/monitor.c` from upstream
- **Questions to Answer**:
  1. File format: Plain text (key=value) or binary?
  2. Update frequency: Every tick (1 Hz) or batched?
  3. Lock mechanism: flock on separate `.lk` file or inline?
  4. Atomicity: Temp file + rename or direct write?
  5. Consumers: What tools read monitor.dat? (web UI, CLI, scripts)

**Expected Format** (based on observation):
```
Load=3.24
Swap=2
Workers=4/4
DynMax=4
Rate=24.3
Impulse=3
Elapsed=943
Queued=142
Built=38
Failed=2
Ignored=0
Skipped=5
```

#### 6b. Recreate in Go (1 hour)
```go
// stats/monitor.go

type MonitorWriter struct {
    path        string
    lockPath    string  // e.g., /build/monitor.lk
    lastSync    time.Time
    syncInterval time.Duration  // e.g., 2s (don't write every tick)
}

func NewMonitorWriter(path string) *MonitorWriter {
    return &MonitorWriter{
        path:         path,
        lockPath:     path + ".lk",
        syncInterval: 2 * time.Second,
    }
}

func (mw *MonitorWriter) OnStatsUpdate(info stats.TopInfo) {
    // Throttle writes (don't spam disk every 1s)
    if time.Since(mw.lastSync) < mw.syncInterval {
        return
    }
    mw.lastSync = time.Now()
    
    if err := mw.writeMonitorFile(info); err != nil {
        log.Printf("Failed to write monitor file: %v", err)
    }
}

func (mw *MonitorWriter) writeMonitorFile(info stats.TopInfo) error {
    // Acquire exclusive lock
    lockFile, err := os.OpenFile(mw.lockPath, os.O_CREATE|os.O_RDWR, 0644)
    if err != nil {
        return fmt.Errorf("open lock file: %w", err)
    }
    defer lockFile.Close()
    
    if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
        return fmt.Errorf("flock: %w", err)
    }
    defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
    
    // Write to temp file
    tmpPath := mw.path + ".tmp"
    content := mw.formatMonitorData(info)
    if err := ioutil.WriteFile(tmpPath, []byte(content), 0644); err != nil {
        return fmt.Errorf("write temp file: %w", err)
    }
    
    // Atomic rename
    if err := os.Rename(tmpPath, mw.path); err != nil {
        return fmt.Errorf("atomic rename: %w", err)
    }
    
    return nil
}

func (mw *MonitorWriter) formatMonitorData(info stats.TopInfo) string {
    // Match original dsynth format exactly
    return fmt.Sprintf(`Load=%.2f
Swap=%d
Workers=%d/%d
DynMax=%d
Rate=%.1f
Impulse=%d
Elapsed=%d
Queued=%d
Built=%d
Failed=%d
Ignored=%d
Skipped=%d
`,
        info.Load, info.SwapPct,
        info.ActiveWorkers, info.MaxWorkers,
        info.DynMaxWorkers,
        info.Rate, info.Impulse,
        int(info.Elapsed.Seconds()),
        info.Queued, info.Built, info.Failed,
        info.Ignored, info.Skipped)
}
```

#### 6c. Hooks for Consumers (30 min)
- **Integration**: Already handled in Phase 3d
- **External Tools**:
  - Web UI: Poll monitor.dat every 1-2 seconds
  - CLI: `go-synth status --watch` reads and displays
  - Scripts: `tail -f monitor.dat`, `watch cat monitor.dat`

#### 6d. Update Lifecycle (15 min)
- **Frequency**: Write every 2 seconds (reduce disk I/O)
- **Trigger**: `OnStatsUpdate()` called by `StatsCollector.tick()`
- **Cleanup**: Remove monitor.dat and monitor.lk on build completion

#### 6e. Testing Strategy (30 min)
```go
// stats/monitor_test.go

func TestMonitorFileFormat(t *testing.T) {
    tmpDir := t.TempDir()
    mw := NewMonitorWriter(filepath.Join(tmpDir, "monitor.dat"))
    
    info := stats.TopInfo{
        Load: 3.24, SwapPct: 2,
        ActiveWorkers: 4, MaxWorkers: 4, DynMaxWorkers: 4,
        Rate: 24.3, Impulse: 3, Elapsed: 943 * time.Second,
        Queued: 142, Built: 38, Failed: 2, Ignored: 0, Skipped: 5,
    }
    
    mw.OnStatsUpdate(info)
    
    // Read file and verify format
    data, err := ioutil.ReadFile(mw.path)
    require.NoError(t, err)
    
    lines := strings.Split(string(data), "\n")
    assert.Contains(t, lines[0], "Load=3.24")
    assert.Contains(t, lines[1], "Swap=2")
    assert.Contains(t, lines[4], "Rate=24.3")
}

func TestMonitorAtomicity(t *testing.T) {
    // Test that concurrent reads never see partial writes
    // Simulate: writer updates every 100ms, reader polls every 50ms
    // Verify: reader always sees complete, valid data
}
```

#### 6f. Future Enhancements (deferred)
- **JSON Mode**: Add `Monitor_format=json` config option
  ```json
  {
    "load": 3.24,
    "swap_pct": 2,
    "workers": {"active": 4, "max": 4, "dynmax": 4},
    "rate": 24.3,
    "impulse": 3,
    "elapsed": 943,
    "totals": {"queued": 142, "built": 38, "failed": 2}
  }
  ```
- **WebSocket Push**: Instead of polling file, push updates via WebSocket
- **Historical Metrics**: Store time-series data in builddb for graphing

---

### Phase 7: Commit & Documentation Strategy

**Estimated Time**: 2 hours

#### 7a. Staging Scope (15 min)
**Commit Breakdown** (6 commits total):

1. **stats: Add StatsCollector with rate/impulse tracking** (Phase 3a-3c, 5)
   - New `stats/` package
   - `collector.go`, `types.go`, `rate.go`
   - Ring buffer implementation, TopInfo struct

2. **stats: Add BSD system metrics (load, swap)** (Phase 3b)
   - `metrics_bsd.go` with `getAdjustedLoad()`, `getSwapUsage()`
   - Tests: `metrics_test.go`

3. **stats: Add monitor file writer** (Phase 6)
   - `monitor.go` with atomic writes and flock
   - Tests: `monitor_test.go`

4. **build: Integrate StatsCollector with BuildContext** (Phase 3d)
   - Modify `build/build.go` to create collector
   - Hook `RecordCompletion()` and `RecordWorkerUpdate()` calls
   - Register UI as consumer

5. **ui: Add stats display to ncurses and stdout UIs** (Phase 4)
   - Update `build/ui_ncurses.go` header panel
   - Update `build/ui_stdout.go` periodic status line
   - Add throttling warnings

6. **docs: Document system stats implementation** (Phase 7c)
   - Update `DEVELOPMENT.md` (Phase 3 complete)
   - Update `README.md` (add stats features)
   - Update `AGENTS.md` (new stats package)

#### 7b. Commit Messaging (15 min)
**Template**:
```
<scope>: <imperative verb> <what>

<why - 1-2 sentences explaining rationale>

<details - optional bullet points for complex changes>

Co-authored-by: Claude 3.7 Sonnet <claude-3.7-sonnet@anthropic.com>
```

**Example**:
```
stats: Add StatsCollector with rate/impulse tracking

Implements real-time package build statistics matching original dsynth
behavior. Uses 60-second sliding window for rate calculation and tracks
instant completions (impulse) for burst detection.

- Ring buffer with 1-second buckets for rate calculation
- TopInfo struct mirrors dsynth's topinfo_t
- StatsConsumer interface for UI/monitor writer integration
- 1 Hz sampling ticker for metric updates

Co-authored-by: Claude 3.7 Sonnet <claude-3.7-sonnet@anthropic.com>
```

#### 7c. Documentation Updates (45 min)
**Files to Update**:

1. **`DEVELOPMENT.md`**:
   - Mark Phase 3 Task "System Stats" as completed
   - Add commits to Recent Milestones
   - Update Current Status to reflect stats implementation

2. **`README.md`**:
   - Add "Real-Time Statistics" to Features section:
     ```markdown
     - **Real-Time Statistics**: Monitor build progress with live metrics
       - System load (adjusted for I/O waits) and swap usage
       - Package build rate (pkg/hr) and instant completions
       - Dynamic worker throttling based on system health
       - Monitor file (`monitor.dat`) for external tools/web UI
     ```
   - Add example `monitor.dat` output
   - Document `go-synth status --watch` command

3. **`AGENTS.md`**:
   - Add `stats/` package to "Core Components" table:
     ```markdown
     | `stats/` | Real-time build statistics collection and reporting | `stats/*.go` |
     ```
   - Add to "Key Data Structures":
     ```markdown
     - **`stats.TopInfo`** - Real-time build statistics snapshot
     - **`stats.StatsCollector`** - 1 Hz metrics sampler with ring buffer
     - **`stats.StatsConsumer`** - Interface for UI/monitor writer
     ```

4. **`docs/design/PHASE_3_BUILDER.md`**:
   - Add "System Stats Integration" section
   - Document stats lifecycle: init → sample → notify → cleanup

#### 7d. Testing Checklist (30 min)
- [ ] Unit tests pass: `go test ./stats/...`
- [ ] Integration tests pass: `go test ./...`
- [ ] Manual VM test: Build devel/gmake, verify stats update every 1s
- [ ] Monitor file test: Verify `monitor.dat` format matches original
- [ ] Throttling test: Simulate high load/swap, verify dynmax reduction
- [ ] UI test: Ncurses shows stats, stdout prints periodic lines
- [ ] Lock test: No corruption with concurrent reads of monitor.dat

#### 7e. Review & Validation (15 min)
```bash
# Format code
gofmt -w stats/

# Vet for issues
go vet ./stats/...

# Check for unused dependencies
go mod tidy

# Build and run
make build
sudo ./go-synth build devel/gmake

# Watch monitor file
watch -n 1 cat /build/monitor.dat
```

#### 7f. Commit Sequencing (done incrementally)
1. Commit after each phase completes
2. Test before committing (unit + integration)
3. Update docs in same commit as code changes
4. Push after all 6 commits complete

#### 7g. Future Follow-ups (deferred to FUTURE_BACKLOG.md)
- [ ] JSON monitor format
- [ ] WebSocket push for web UI
- [ ] Historical metrics storage in builddb
- [ ] Grafana/Prometheus metrics export
- [ ] Per-worker stats tracking (current package, phase, duration)
- [ ] Stall detection (no progress for N minutes)

---

## Success Criteria

### Functional Requirements
- ✅ StatsCollector samples system metrics at 1 Hz
- ✅ Rate calculated from 60-second sliding window
- ✅ Impulse tracks instant completions
- ✅ Load adjusted with `vm.vmtotal.t_pw` (page-fault waits)
- ✅ Swap percentage queried via sysctl
- ✅ Dynamic worker throttling matches original dsynth behavior
- ✅ Monitor file written atomically with flock
- ✅ Ncurses UI shows stats in header panel
- ✅ Stdout UI prints periodic status lines
- ✅ External tools can read `monitor.dat` without corruption

### Performance Requirements
- Stats collection adds <1% CPU overhead
- Monitor file writes don't block build workers
- Lock contention on monitor.lk is negligible
- UI updates are smooth (no flicker/lag)

### Compatibility Requirements
- `monitor.dat` format matches original dsynth (external tools work)
- Ncurses layout similar to original (familiar to users)
- Stdout output concise and readable

### Testing Requirements
- Unit test coverage >80% for stats package
- Integration tests validate end-to-end stats flow
- VM tests confirm BSD-specific syscalls work correctly
- Manual testing verifies UI display and monitor file

---

## Implementation Task Breakdown

| Phase | Task | Estimated | Files Changed |
|-------|------|-----------|---------------|
| 1 | Source analysis | 3h | (docs only) |
| 2 | C implementation review | 4h | (docs only) |
| 3a | Collector architecture | 1.5h | `stats/collector.go` |
| 3b | Metric acquisition | 2h | `stats/metrics_bsd.go` |
| 3c | Data model | 0.5h | `stats/types.go` |
| 3d | Hook integration | 1h | `build/build.go` |
| 3e | Monitor writer | 1h | `stats/monitor.go` |
| 3f | Config & tests | 1h | `stats/*_test.go` |
| 4.2 | Ncurses UI | 1.5h | `build/ui_ncurses.go` |
| 4.3 | Stdout UI | 1h | `build/ui_stdout.go` |
| 4.4 | CLI consumer | 0.5h | `cmd/status.go` |
| 4.5 | Throttle feedback | 0.5h | `build/ui*.go` |
| 4.6 | UI testing | 1h | (manual) |
| 5 | Rate/impulse (detailed) | 3h | (already in 3a) |
| 6 | Monitor file (detailed) | 3h | (already in 3e) |
| 7 | Docs & commits | 2h | `DEVELOPMENT.md`, etc. |
| **Total** | | **27h** | ~10 files |

**Note**: Phases 5 & 6 overlap with Phase 3, so total is not 34h.

---

## Dependencies & Prerequisites

### External Dependencies
- Original dsynth source: `usr.bin/dsynth/` from DragonFlyBSD repo
  - **Action**: `git clone https://github.com/DragonFlyBSD/DragonFlyBSD.git`
  - **Files Needed**: `monitor.c`, `curses.c`

### Go Packages
- `golang.org/x/sys/unix` - BSD syscalls (already imported)
- No new external dependencies

### System Requirements
- BSD system with `vm.vmtotal` and `vm.swap_info` sysctls
- Alternatively: Mock implementations for Linux (future work)

### Code Dependencies
- Phase 3 (Builder Integration) must be complete
- Phase 4 (Environment Abstraction) for proper context propagation
- `build.BuildContext` needs stats hooks
- `build.BuildUI` interface for consumer pattern

---

## Related Issues

- **Phase 3: Builder Integration** - System stats is part of Phase 3 completion
- **Phase 4: Environment Abstraction** - Context propagation enables stats
- **CLEANUP_CHILD_PROCESSES.md** - Signal handling affects stats shutdown
- **Ncurses UI Implementation** - UI consumer for stats display

---

## References

### Original dsynth Source Files
- `.original-c-source/dsynth.h:469-500` - `topinfo_t` struct, `runstats_t` interface
- `.original-c-source/build.c` - `waitbuild()`, `adjloadavg()`, throttling logic
- Upstream (to fetch): `usr.bin/dsynth/monitor.c`, `usr.bin/dsynth/curses.c`

### Go Implementation Files
- `stats/collector.go` - StatsCollector implementation (NEW)
- `stats/types.go` - TopInfo, StatsConsumer (NEW)
- `stats/metrics_bsd.go` - System metric acquisition (NEW)
- `stats/monitor.go` - Monitor file writer (NEW)
- `build/build.go` - Integration hooks
- `build/ui_ncurses.go`, `build/ui_stdout.go` - Stat display

### Documentation
- `DEVELOPMENT.md` - Phase tracking
- `README.md` - User-facing features
- `docs/design/PHASE_3_BUILDER.md` - Builder phase design

---

**Document Version**: 1.0  
**Last Updated**: 2025-12-02  
**Next Review**: After Phase 1 & 2 completion (source analysis)
