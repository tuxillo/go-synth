# Post-MVP Build Architecture Analysis

**Date**: 2025-12-05  
**Status**: Analysis / Proposed Refactoring  
**Context**: Phase 7 complete, working build system deployed

## Executive Summary

The current `build` package successfully orchestrates parallel package builds
with CRC-based incremental logic, environment isolation, and comprehensive
monitoring. However, the architecture mixes concerns (orchestration, UI, stats,
persistence) in ways that limit reusability and testability.

This document analyzes the current design, identifies pain points, and proposes
a layered architecture with narrow interfaces that separates the core build
engine from runtime concerns.

## Current Architecture Assessment

### Core Components

**BuildContext** (`build/build.go:107-127`)
- Holds: config, logger, buildDB, worker pool, queue, stats, statsCollector,
  throttler, UI, context/cancel
- Responsibilities: 
  - Worker lifecycle management
  - Dependency scheduling
  - Stats aggregation
  - UI updates
  - Build record persistence
  - Signal handling via cleanup function
  - Environment creation

**Worker** (`build/build.go:92-99`)
- Fields: ID, `environment.Environment`, current package, status, timing
- Executes: phase commands via `executePhase()`

**Phase Execution** (`build/phases.go:27-129`)
- Builds `make` commands with dsynth-compatible overrides
- Installs dependency packages
- Tightly coupled to dsynth paths (`/xports`, `/construction`, etc.)

**Environment Abstraction** (`environment/`)
- Clean interface: `Setup`, `Execute`, `Cleanup`, `GetBasePath`
- BSD backend with 27-mount chroot isolation
- Mock backend for testing

### Pain Points

#### 1. Mixed Responsibilities in BuildContext

**Problem**: BuildContext conflates orchestration with presentation and
persistence.

```go
type BuildContext struct {
    // Core orchestration
    cfg       *config.Config
    registry  *pkg.BuildStateRegistry
    workers   []*Worker
    queue     chan *pkg.Package
    
    // Persistence
    buildDB   *builddb.DB
    runID     string
    
    // Presentation
    ui              BuildUI
    statsCollector  *stats.StatsCollector
    
    // Runtime concerns
    ctx     context.Context
    cancel  context.CancelFunc
}
```

**Impact**:
- Cannot test orchestration without mocking UI, DB, and stats
- Cannot reuse scheduling logic outside the worker pool context
- Difficult to swap UI or persistence implementations

#### 2. Tight Coupling in Phase Execution

**Problem**: `executePhase()` hardcodes dsynth conventions and `make`-specific
logic.

```go
// From build/phases.go:30-48
portPath := filepath.Join("/xports", p.Category, p.Name)
args := []string{"-C", portPath}
args = append(args,
    "PORTSDIR=/xports",
    "WRKDIRPREFIX=/construction",
    "DISTDIR=/distfiles",
    // ...
)
```

**Impact**:
- Cannot support alternative build systems (e.g., custom scripts, containers)
- Path conventions leaked into business logic
- Difficult to test phase execution without full environment

#### 3. Environment Backend Fixed at Worker Creation

**Problem**: Backend selection hardcoded in `DoBuild()`:

```go
// From build/build.go:428
env, err := environment.New("bsd")
```

**Impact**:
- Cannot inject mock environments for testing
- Cannot dynamically select backend per worker
- Tight coupling between build orchestration and environment implementation

#### 4. CRC Logic Intertwined with Scheduling

**Problem**: Incremental build decisions scattered across multiple locations:
- Pre-queue CRC check in main.go before `DoBuild`
- Success handling updates CRC in `buildPackage()`
- Bootstrap has separate CRC handling

**Impact**:
- Cannot test incremental logic independently
- Difficult to change CRC strategy (e.g., hash algorithm, cache policy)
- Logic duplicated for bootstrap and regular builds

#### 5. UI and Signal Handling in Core Loop

**Problem**: UI selection and cleanup signal wiring embedded in `DoBuild()`:

```go
// From build/build.go:205-222
useNcurses := !cfg.DisableUI && term.IsTerminal(int(os.Stdout.Fd()))
if useNcurses {
    ctx.ui = NewNcursesUI()
} else {
    ctx.ui = NewStdoutUI()
}

// Lines 237-266: Interrupt handler setup
if ncursesUI, ok := ctx.ui.(*NcursesUI); ok {
    setupInterruptHandler = func(cleanup func()) {
        ncursesUI.SetInterruptHandler(func() {
            // ...cleanup and exit...
        })
    }
}
```

**Impact**:
- Build engine cannot run without UI decision
- Signal handling mixed with business logic
- Difficult to reuse `DoBuild` in different contexts (CLI, API, tests)

#### 6. No Clear Build Job Abstraction

**Problem**: Packages are scheduled directly without a "build job" concept.

**Impact**:
- Cannot represent non-package builds (fetch-only, verify, custom tasks)
- Difficult to extend with pre/post hooks
- Job metadata scattered across Package, BuildStateRegistry, and BuildContext

## Target Architecture

### Design Principles

1. **Separation of Concerns**: Core build engine independent of UI, persistence,
   and runtime.
2. **Dependency Injection**: Consumers provide backends, policies, and
   recorders.
3. **Interface-Based Design**: Small, focused interfaces over large
   implementations.
4. **Testability**: Each layer mockable and testable in isolation.
5. **Extensibility**: Support future builders, environments, and use cases
   (API, fetch-only, rebuild).

### Layered Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      CLI / API                          │
│  (runtime: environment selection, UI, signal handling)  │
└─────────────────────┬───────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────┐
│                  Build Orchestrator                     │
│  (worker pool, dependency scheduling, event dispatch)   │
└─────────┬────────────┬────────────┬─────────────────────┘
          │            │            │
  ┌───────▼────┐  ┌────▼─────┐  ┌──▼───────────┐
  │  Scheduler │  │ Executor │  │   Policy     │
  │            │  │          │  │              │
  │ - Plan     │  │ - Execute│  │ - NeedsBuild │
  │ - Next     │  │ - phases │  │ - OnSuccess  │
  │ - Complete │  │          │  │ - OnSkip     │
  └────────────┘  └─────┬────┘  └──────────────┘
                        │
              ┌─────────▼──────────┐
              │  Environment       │
              │  (bsd, mock, ...)  │
              └────────────────────┘
                        │
     ┌──────────────────┼──────────────────┐
     │                  │                  │
┌────▼─────┐  ┌─────────▼────────┐  ┌─────▼──────┐
│    UI    │  │   RunRecorder    │  │  Metrics   │
│ Consumer │  │   (persistence)  │  │  Consumer  │
└──────────┘  └──────────────────┘  └────────────┘
```

### Responsibility Breakdown

| Layer              | Responsibility                           | Knows About       |
|--------------------|------------------------------------------|-------------------|
| CLI/Runtime        | Environment selection, UI wiring, signals| All layers        |
| Build Orchestrator | Worker pool, dependency ordering, events | Interfaces only   |
| Scheduler          | Topological sort, ready queue            | Package graph     |
| Executor           | Phase execution, result handling         | Environment iface |
| Policy             | Incremental decisions, bootstrap         | CRC, config       |
| Environment        | Isolation, command execution             | OS primitives     |
| Consumers          | UI updates, persistence, metrics         | Events only       |

## Proposed Interfaces

### 1. Scheduler Interface

Manages dependency-aware task scheduling.

```go
// Scheduler orders build jobs respecting dependencies.
//
// Responsibilities:
//   - Topological ordering of dependency graph
//   - Track completion status and propagate failures
//   - Provide ready-to-build queue
//
// The scheduler is stateful and NOT thread-safe. The orchestrator must
// serialize access.
type Scheduler interface {
    // Plan computes the build order from a package graph.
    // Returns error if graph has cycles or invalid dependencies.
    Plan(packages []*pkg.Package) error
    
    // Next returns the next package ready to build (all deps satisfied).
    // Returns nil when no packages are ready or all complete.
    // Blocks if dependencies are pending but not yet complete.
    Next() *pkg.Package
    
    // Complete marks a package as finished with the given status.
    // Status propagates to dependents (failed → dependents skipped).
    Complete(p *pkg.Package, status BuildStatus)
    
    // Stats returns current scheduling statistics.
    Stats() SchedulerStats
}

type SchedulerStats struct {
    Total      int // Total packages in plan
    Pending    int // Not yet started
    Running    int // Currently building
    Complete   int // Finished (success + failed + skipped)
    Success    int // Successfully built
    Failed     int // Build failed
    Skipped    int // Skipped due to dependency failure
}

type BuildStatus int

const (
    BuildPending BuildStatus = iota
    BuildRunning
    BuildSuccess
    BuildFailed
    BuildSkipped
    BuildIgnored
)
```

### 2. BuildExecutor Interface

Executes a build job against an environment.

```go
// BuildExecutor runs build phases for a package.
//
// Responsibilities:
//   - Execute all phases in sequence
//   - Handle phase failures and logging
//   - Return structured result
//
// Implementations may use PhaseRunner for individual phases.
type BuildExecutor interface {
    // Execute runs all build phases for the given package.
    // Context cancellation stops execution between phases.
    Execute(ctx context.Context, job *BuildJob) (*BuildResult, error)
}

// BuildJob describes what to build and how.
type BuildJob struct {
    Package  *pkg.Package
    Env      environment.Environment
    Logger   *log.PackageLogger
    Phases   []string // Ordered list of phases to execute
    Config   *config.Config
    Registry *pkg.BuildStateRegistry
}

// BuildResult captures the outcome of a build execution.
type BuildResult struct {
    Status    BuildStatus
    Duration  time.Duration
    LastPhase string
    ExitCode  int
    Error     error
}
```

### 3. PhaseRunner Interface

Executes a single build phase.

```go
// PhaseRunner executes individual build phases.
//
// Responsibilities:
//   - Compose environment commands for a phase
//   - Handle phase-specific logic (install-pkgs, fetch, build, etc.)
//   - Return exit code and logs
//
// This interface allows swapping build backends (make, custom scripts, etc.).
type PhaseRunner interface {
    // Run executes a single phase in the given environment.
    // Returns exit code (0 = success) and error (execution failure).
    Run(ctx context.Context, env environment.Environment, pkg *pkg.Package,
        phase string, opts *PhaseOptions) (exitCode int, err error)
}

// PhaseOptions provides configuration for phase execution.
type PhaseOptions struct {
    Config   *config.Config
    Logger   *log.PackageLogger
    Registry *pkg.BuildStateRegistry
    Env      map[string]string // Extra environment variables
}
```

### 4. IncrementalPolicy Interface

Decides which packages need building.

```go
// IncrementalPolicy determines if a package needs rebuilding.
//
// Responsibilities:
//   - CRC-based change detection
//   - Force-rebuild logic
//   - Post-build CRC updates
//
// Implementations may cache state across calls for performance.
type IncrementalPolicy interface {
    // NeedsBuild returns true if the package should be built.
    // Factors: CRC mismatch, force flag, previous failures.
    NeedsBuild(p *pkg.Package) (bool, error)
    
    // MarkSuccess updates CRC and metadata after successful build.
    MarkSuccess(p *pkg.Package) error
    
    // MarkSkipped records that package was skipped (no CRC update).
    MarkSkipped(p *pkg.Package) error
}
```

### 5. EnvironmentProvider Interface

Creates and manages environment backends.

```go
// EnvironmentProvider creates isolated build environments.
//
// Responsibilities:
//   - Backend selection (bsd, mock, jail, container)
//   - Pooling and reuse (optional)
//   - Lifecycle management
//
// Providers may create environments lazily or maintain a pool.
type EnvironmentProvider interface {
    // Create returns a new environment for the given worker.
    // The environment must be cleaned up by the caller.
    Create(workerID int, cfg *config.Config, logger log.LibraryLogger) (environment.Environment, error)
    
    // Close releases any provider-level resources (pools, caches).
    Close() error
}
```

### 6. RunRecorder Interface

Persists build records and run history.

```go
// RunRecorder tracks build execution for historical analysis.
//
// Responsibilities:
//   - Create build records with UUIDs
//   - Update status as builds progress
//   - Link builds to runs for bulk operations
//
// Implementations may buffer writes or write immediately.
type RunRecorder interface {
    // StartBuild creates a new build record and returns its UUID.
    StartBuild(p *pkg.Package, workerID int) (uuid string, err error)
    
    // FinishBuild updates the build record with final status.
    FinishBuild(uuid string, result *BuildResult) error
    
    // RecordRunPackage associates a package with the current run.
    // Used for tracking bulk build sessions.
    RecordRunPackage(runID string, p *pkg.Package, status BuildStatus,
        workerID int, start, end time.Time, lastPhase string) error
    
    // Close flushes any buffered writes.
    Close() error
}
```

### 7. BuildEvents Interface

Dispatches build lifecycle events to consumers.

```go
// BuildEvents notifies consumers of build lifecycle events.
//
// Responsibilities:
//   - UI updates (progress, logs)
//   - Metrics collection (rates, durations)
//   - Custom integrations (webhooks, notifications)
//
// All methods are fire-and-forget; errors are logged but not propagated.
type BuildEvents interface {
    // OnBuildStart is called when a package starts building.
    OnBuildStart(p *pkg.Package, workerID int)
    
    // OnPhaseStart is called when a build phase begins.
    OnPhaseStart(p *pkg.Package, phase string, workerID int)
    
    // OnPhaseEnd is called when a build phase completes.
    OnPhaseEnd(p *pkg.Package, phase string, exitCode int, duration time.Duration)
    
    // OnBuildComplete is called when a package build finishes.
    OnBuildComplete(p *pkg.Package, result *BuildResult, workerID int)
    
    // OnWorkerIdle is called when a worker becomes available.
    OnWorkerIdle(workerID int)
}

// MulticastEvents dispatches events to multiple consumers.
type MulticastEvents struct {
    consumers []BuildEvents
}

func (m *MulticastEvents) Add(consumer BuildEvents) {
    m.consumers = append(m.consumers, consumer)
}

func (m *MulticastEvents) OnBuildStart(p *pkg.Package, workerID int) {
    for _, c := range m.consumers {
        c.OnBuildStart(p, workerID)
    }
}

// ... other methods dispatch to all consumers ...
```

## Refactored Architecture Sketch

### DoBuildLite: Core Orchestration

```go
// DoBuildLite orchestrates parallel builds with dependency injection.
//
// This function focuses solely on orchestration:
//   - Create worker pool
//   - Schedule packages from the queue
//   - Dispatch to executor
//   - Fire events for consumers
//
// All policies, persistence, and presentation are injected.
func DoBuildLite(
    ctx context.Context,
    packages []*pkg.Package,
    cfg *config.Config,
    deps Dependencies,
) (*BuildStats, error) {
    // Dependencies bundles all injected components
    scheduler := deps.Scheduler
    executor := deps.Executor
    envProvider := deps.EnvProvider
    policy := deps.IncrementalPolicy
    recorder := deps.RunRecorder
    events := deps.Events
    
    // Plan the build (topological sort)
    if err := scheduler.Plan(packages); err != nil {
        return nil, fmt.Errorf("scheduling failed: %w", err)
    }
    
    // Apply incremental policy (CRC checks)
    for _, p := range packages {
        needsBuild, err := policy.NeedsBuild(p)
        if err != nil {
            return nil, fmt.Errorf("incremental check failed: %w", err)
        }
        if !needsBuild {
            scheduler.Complete(p, BuildSkipped)
            policy.MarkSkipped(p)
        }
    }
    
    // Create worker pool
    workers := make([]*worker, cfg.MaxWorkers)
    for i := 0; i < cfg.MaxWorkers; i++ {
        env, err := envProvider.Create(i, cfg, deps.Logger)
        if err != nil {
            return nil, fmt.Errorf("worker %d env creation failed: %w", i, err)
        }
        workers[i] = &worker{
            id:  i,
            env: env,
        }
    }
    
    // Cleanup function
    defer func() {
        for _, w := range workers {
            if w.env != nil {
                w.env.Cleanup()
            }
        }
        envProvider.Close()
        recorder.Close()
    }()
    
    // Worker loop (simplified)
    var wg sync.WaitGroup
    for _, w := range workers {
        wg.Add(1)
        go func(worker *worker) {
            defer wg.Done()
            for {
                // Check context cancellation
                select {
                case <-ctx.Done():
                    return
                default:
                }
                
                // Get next package to build
                p := scheduler.Next()
                if p == nil {
                    // No more work
                    return
                }
                
                // Mark as running
                scheduler.Complete(p, BuildRunning)
                events.OnBuildStart(p, worker.id)
                
                // Start build record
                uuid, _ := recorder.StartBuild(p, worker.id)
                p.BuildUUID = uuid
                
                // Create build job
                job := &BuildJob{
                    Package: p,
                    Env:     worker.env,
                    Logger:  log.NewPackageLogger(cfg, p.PortDir),
                    Phases:  DefaultPhases,
                    Config:  cfg,
                }
                
                // Execute build
                result, err := executor.Execute(ctx, job)
                if err != nil {
                    result = &BuildResult{
                        Status: BuildFailed,
                        Error:  err,
                    }
                }
                
                // Update records
                recorder.FinishBuild(uuid, result)
                scheduler.Complete(p, result.Status)
                
                // Update incremental policy
                if result.Status == BuildSuccess {
                    policy.MarkSuccess(p)
                }
                
                // Fire completion event
                events.OnBuildComplete(p, result, worker.id)
                events.OnWorkerIdle(worker.id)
            }
        }(w)
    }
    
    // Wait for all workers
    wg.Wait()
    
    // Return final stats
    return &BuildStats{
        Total:   scheduler.Stats().Total,
        Success: scheduler.Stats().Success,
        Failed:  scheduler.Stats().Failed,
        Skipped: scheduler.Stats().Skipped,
    }, nil
}

// Dependencies bundles injected components for DoBuildLite.
type Dependencies struct {
    Scheduler         Scheduler
    Executor          BuildExecutor
    EnvProvider       EnvironmentProvider
    IncrementalPolicy IncrementalPolicy
    RunRecorder       RunRecorder
    Events            BuildEvents
    Logger            log.LibraryLogger
}
```

### Default Implementations (Adapters)

```go
// DefaultDependencies creates production dependencies from current code.
func DefaultDependencies(cfg *config.Config, db *builddb.DB, runID string) *Dependencies {
    return &Dependencies{
        Scheduler:         NewTopoScheduler(),
        Executor:          NewMakeExecutor(),
        EnvProvider:       NewBSDEnvProvider(),
        IncrementalPolicy: NewCRCPolicy(cfg, db),
        RunRecorder:       NewBuildDBRecorder(db, runID),
        Events:            NewMulticastEvents(),
        Logger:            log.NewStdLogger(),
    }
}

// TopoScheduler wraps pkg.GetBuildOrder and dependency tracking.
type TopoScheduler struct {
    packages []*pkg.Package
    registry *pkg.BuildStateRegistry
    // ... state tracking ...
}

func (s *TopoScheduler) Plan(packages []*pkg.Package) error {
    s.packages = pkg.GetBuildOrder(packages, logger)
    return nil
}

func (s *TopoScheduler) Next() *pkg.Package {
    // Return next package with all dependencies satisfied
    // ... current waitForDependencies logic ...
}

func (s *TopoScheduler) Complete(p *pkg.Package, status BuildStatus) {
    // Update registry flags
    s.registry.AddFlags(p, statusToFlag(status))
}

// MakeExecutor wraps current executePhase logic.
type MakeExecutor struct{}

func (e *MakeExecutor) Execute(ctx context.Context, job *BuildJob) (*BuildResult, error) {
    startTime := time.Now()
    for _, phase := range job.Phases {
        exitCode, err := executePhase(ctx, job.Env, job.Package, phase, job.Config, job.Registry, job.Logger)
        if err != nil || exitCode != 0 {
            return &BuildResult{
                Status:    BuildFailed,
                Duration:  time.Since(startTime),
                LastPhase: phase,
                ExitCode:  exitCode,
                Error:     err,
            }, nil
        }
    }
    return &BuildResult{
        Status:   BuildSuccess,
        Duration: time.Since(startTime),
    }, nil
}

// BSDEnvProvider wraps environment.New("bsd").
type BSDEnvProvider struct{}

func (p *BSDEnvProvider) Create(workerID int, cfg *config.Config, logger log.LibraryLogger) (environment.Environment, error) {
    env, err := environment.New("bsd")
    if err != nil {
        return nil, err
    }
    if err := env.Setup(workerID, cfg, logger); err != nil {
        env.Cleanup() // Cleanup partial state
        return nil, err
    }
    return env, nil
}

func (p *BSDEnvProvider) Close() error {
    return nil // No pooling yet
}

// CRCPolicy wraps builddb CRC operations.
type CRCPolicy struct {
    cfg *config.Config
    db  *builddb.DB
}

func (p *CRCPolicy) NeedsBuild(pkg *pkg.Package) (bool, error) {
    // Current MarkPackagesNeedingBuild logic for single package
    portPath := filepath.Join(p.cfg.DPortsPath, pkg.Category, pkg.Name)
    currentCRC, err := builddb.ComputePortCRC(portPath)
    if err != nil {
        return true, nil // Build if CRC fails
    }
    
    storedCRC, err := p.db.GetCRC(pkg.PortDir)
    if err != nil || storedCRC != currentCRC {
        return true, nil // CRC mismatch or no record
    }
    
    return false, nil // CRC match, skip
}

func (p *CRCPolicy) MarkSuccess(pkg *pkg.Package) error {
    portPath := filepath.Join(p.cfg.DPortsPath, pkg.Category, pkg.Name)
    crc, err := builddb.ComputePortCRC(portPath)
    if err != nil {
        return err
    }
    return p.db.UpdateCRC(pkg.PortDir, crc)
}

func (p *CRCPolicy) MarkSkipped(pkg *pkg.Package) error {
    return nil // No-op for skipped packages
}

// BuildDBRecorder wraps builddb persistence.
type BuildDBRecorder struct {
    db    *builddb.DB
    runID string
}

func (r *BuildDBRecorder) StartBuild(p *pkg.Package, workerID int) (string, error) {
    uuid := uuid.New().String()
    record := &builddb.BuildRecord{
        UUID:      uuid,
        PortDir:   p.PortDir,
        Version:   p.Version,
        Status:    "running",
        StartTime: time.Now(),
    }
    return uuid, r.db.SaveRecord(record)
}

func (r *BuildDBRecorder) FinishBuild(uuid string, result *BuildResult) error {
    status := "failed"
    if result.Status == BuildSuccess {
        status = "success"
    }
    return r.db.UpdateRecordStatus(uuid, status, time.Now())
}

func (r *BuildDBRecorder) RecordRunPackage(runID string, p *pkg.Package, status BuildStatus, workerID int, start, end time.Time, lastPhase string) error {
    rec := &builddb.RunPackageRecord{
        PortDir:   p.PortDir,
        Version:   p.Version,
        Status:    buildStatusToString(status),
        StartTime: start,
        EndTime:   end,
        WorkerID:  workerID,
        LastPhase: lastPhase,
    }
    return r.db.PutRunPackage(runID, rec)
}

func (r *BuildDBRecorder) Close() error {
    return nil // DB lifecycle managed externally
}
```

### CLI Integration (main.go)

```go
// In main.go doBuild command
func doBuild(portList []string, cfg *config.Config, logger *log.Logger) error {
    // Parse packages and resolve dependencies (unchanged)
    packages, err := pkg.ParsePortList(portList, cfg, stateRegistry, pkgRegistry)
    // ... dependency resolution ...
    
    // Open buildDB (unchanged)
    db, err := builddb.OpenDB(dbPath)
    defer db.Close()
    
    // Create dependencies with defaults
    deps := DefaultDependencies(cfg, db, runID)
    
    // Wire up UI as event consumer
    ui := selectUI(cfg) // ncurses or stdout based on TTY
    if err := ui.Start(); err != nil {
        logger.Warn("Failed to start UI: %v", err)
    }
    defer ui.Stop()
    
    // Register consumers
    events := NewMulticastEvents()
    events.Add(NewUIEventAdapter(ui))
    events.Add(NewStatsEventAdapter(statsCollector))
    deps.Events = events
    
    // Setup signal handling
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-sigChan
        logger.Info("Interrupt received, stopping build...")
        cancel()
    }()
    
    // Run build with injected dependencies
    stats, err := DoBuildLite(ctx, packages, cfg, deps)
    if err != nil {
        return err
    }
    
    // Print summary
    fmt.Printf("Build complete: %d success, %d failed, %d skipped\n",
        stats.Success, stats.Failed, stats.Skipped)
    
    return nil
}
```

## Migration Strategy

### Phase 1: Extract Interfaces (Low Risk)

1. **Define interfaces** in new file `build/interfaces.go` (no behavior change)
2. **Create adapter types** wrapping existing code:
   - `TopoScheduler` → calls `pkg.GetBuildOrder` + `waitForDependencies`
   - `MakeExecutor` → calls `executePhase` loop
   - `BSDEnvProvider` → calls `environment.New("bsd")`
   - `CRCPolicy` → calls `builddb.ComputePortCRC` + `db.UpdateCRC`
   - `BuildDBRecorder` → wraps `db.SaveRecord` + `db.UpdateRecordStatus`

**Verification**: All adapters compile, pass existing integration tests.

### Phase 2: Inject Dependencies (Medium Risk)

1. **Add optional parameters to `DoBuild`**:
   ```go
   func DoBuild(packages []*pkg.Package, cfg *config.Config, logger *log.Logger,
                db *builddb.DB, registry *pkg.BuildStateRegistry,
                onCleanupReady func(func()), runID string,
                deps *Dependencies) (*BuildStats, func(), error)
   ```
2. **Default to current behavior** if `deps == nil`:
   ```go
   if deps == nil {
       deps = DefaultDependencies(cfg, db, runID)
   }
   ```
3. **Replace direct calls** with interface methods:
   - `environment.New("bsd")` → `deps.EnvProvider.Create()`
   - Direct `buildDB` calls → `deps.RunRecorder.StartBuild()` / `FinishBuild()`
   - CRC logic → `deps.IncrementalPolicy.NeedsBuild()`

**Verification**: Existing tests pass, CLI behavior unchanged.

### Phase 3: Create DoBuildLite (Medium Risk)

1. **Implement clean `DoBuildLite` function** without UI/cleanup in core loop
2. **Keep `DoBuild` as compatibility wrapper** calling `DoBuildLite`
3. **Move UI selection to CLI** (main.go)
4. **Move signal handling to CLI** (main.go)

**Verification**: Run integration tests against both `DoBuild` and `DoBuildLite`.

### Phase 4: Refactor Scheduler (High Value)

1. **Implement standalone `TopoScheduler`** with state machine
2. **Add unit tests** for scheduling logic (dependency propagation, cycles)
3. **Replace inline scheduling** in `DoBuildLite` worker loop

**Verification**: Unit tests cover edge cases (cycles, skipped deps, partial
failures).

### Phase 5: Extract PhaseRunner (High Value)

1. **Define `PhaseRunner` interface** for pluggable phase execution
2. **Implement `MakePhaseRunner`** wrapping `executePhase`
3. **Inject into `MakeExecutor`** via constructor
4. **Add unit tests** for phase composition logic

**Verification**: Phase execution testable without full environment.

### Phase 6: Event-Driven UI (Future)

1. **Replace direct UI calls** with `BuildEvents.OnBuildStart()` etc.
2. **Implement `UIEventAdapter`** bridging events to existing UI interface
3. **Add webhook consumer** demonstrating extensibility

**Verification**: UI updates identical, new consumers work in parallel.

## Benefits of Refactoring

### Testability

**Before**:
```go
// Cannot test without full BuildContext, workers, UI, DB
func TestBuildPackage(t *testing.T) {
    // Need: config, logger, buildDB, workers, mounts, ...
}
```

**After**:
```go
// Test executor in isolation with mock environment
func TestMakeExecutor(t *testing.T) {
    mockEnv := &environment.MockEnvironment{}
    executor := &MakeExecutor{}
    
    job := &BuildJob{
        Package: testPackage,
        Env:     mockEnv,
        Phases:  []string{"configure", "build"},
    }
    
    result, err := executor.Execute(context.Background(), job)
    assert.NoError(t, err)
    assert.Equal(t, BuildSuccess, result.Status)
}
```

### Reusability

**Before**: Cannot reuse build logic outside `DoBuild` function.

**After**: Mix and match components:
```go
// Fetch-only mode
deps := DefaultDependencies(cfg, db, runID)
deps.Executor = &FetchOnlyExecutor{} // Only fetch phase
deps.IncrementalPolicy = &AlwaysBuildPolicy{} // Ignore CRC
stats, _ := DoBuildLite(ctx, packages, cfg, deps)

// API mode (no UI)
deps := DefaultDependencies(cfg, db, runID)
deps.Events = &WebhookEvents{URL: "http://api/builds"}
stats, _ := DoBuildLite(ctx, packages, cfg, deps)

// Test mode
deps := &Dependencies{
    Scheduler:   NewTopoScheduler(),
    Executor:    &MockExecutor{},
    EnvProvider: &MockEnvProvider{},
    // ... all mocks ...
}
stats, _ := DoBuildLite(ctx, packages, cfg, deps)
```

### Extensibility

Add new capabilities without modifying core:

**Custom Build System**:
```go
type PodmanExecutor struct {
    image string
}

func (e *PodmanExecutor) Execute(ctx context.Context, job *BuildJob) (*BuildResult, error) {
    // Run build in container instead of chroot
    cmd := exec.Command("podman", "run", e.image, "make", "install", ...)
    // ...
}

deps.Executor = &PodmanExecutor{image: "freebsd:14.0"}
```

**Alternative Incremental Policy**:
```go
type HashPolicy struct {
    algorithm string // SHA256, BLAKE3, etc.
}

func (p *HashPolicy) NeedsBuild(pkg *pkg.Package) (bool, error) {
    // Use content hash instead of CRC32
}

deps.IncrementalPolicy = &HashPolicy{algorithm: "SHA256"}
```

**Multi-Backend Environments**:
```go
type MultiBackendProvider struct {
    backends map[string]EnvironmentProvider
}

func (p *MultiBackendProvider) Create(workerID int, cfg *config.Config, logger log.LibraryLogger) (environment.Environment, error) {
    // Assign workers to different backends (jails, containers, VMs)
    backend := p.backends[p.selectBackend(workerID)]
    return backend.Create(workerID, cfg, logger)
}
```

### Maintainability

Clear boundaries make code easier to understand and modify:

| Current                      | Refactored                          |
|------------------------------|-------------------------------------|
| 832-line BuildContext        | 200-line orchestrator + interfaces  |
| Mixed scheduling + execution | Separate Scheduler + Executor       |
| Embedded UI logic            | Event-driven consumers              |
| Hardcoded policies           | Injected policy objects             |

## Quick Wins (No Breaking Changes)

### 1. Extract RunRecorder (2-4 hours)

Create `build/recorder.go`:
```go
type buildDBRecorder struct {
    db    *builddb.DB
    runID string
}

func (r *buildDBRecorder) StartBuild(...) (string, error) { /* ... */ }
func (r *buildDBRecorder) FinishBuild(...) error { /* ... */ }
```

Replace direct `buildDB` calls in `BuildContext.buildPackage()` with
`recorder.StartBuild()`.

**Benefit**: Persistence testable independently.

### 2. Add EnvironmentProvider Constructor (1-2 hours)

Add optional parameter to `DoBuild`:
```go
func DoBuild(..., envProvider EnvironmentProvider) (*BuildStats, func(), error) {
    if envProvider == nil {
        envProvider = &defaultBSDProvider{}
    }
    // Use envProvider.Create() instead of environment.New("bsd")
}
```

**Benefit**: Enable mock environments in tests.

### 3. Extract Phase List to Config (1 hour)

Move hardcoded phase list to constant:
```go
var DefaultPhases = []string{
    "install-pkgs", "check-sanity", "fetch-depends", "fetch", "checksum",
    "extract-depends", "extract", "patch-depends", "patch", "build-depends",
    "lib-depends", "configure", "build", "run-depends", "stage",
    "check-plist", "package",
}
```

Pass as part of `BuildJob` or config.

**Benefit**: Support custom phase lists (e.g., fetch-only = `["fetch"]`).

### 4. Move UI Selection to CLI (2-3 hours)

Remove UI creation from `DoBuild`, accept `ui BuildUI` parameter:
```go
func DoBuild(..., ui BuildUI) (*BuildStats, func(), error) {
    ctx.ui = ui
    // ...
}
```

Move selection logic to `main.go`:
```go
ui := selectUI(cfg) // ncurses or stdout
stats, cleanup, err := DoBuild(..., ui)
```

**Benefit**: DoBuild no longer depends on terminal detection.

## Open Questions

1. **Backward Compatibility**: Should we keep `DoBuild` as-is and create
   `DoBuildV2`, or refactor incrementally with optional parameters?

2. **Testing Strategy**: Unit tests first, or integration tests verifying
   behavior preservation?

3. **Performance**: Does interface indirection add measurable overhead? (Likely
   negligible compared to build times.)

4. **Scheduler Blocking**: Should `Scheduler.Next()` block waiting for
   dependencies, or return `nil` and require caller to poll?

5. **Event Ordering**: Do consumers need guaranteed ordering of events, or is
   best-effort sufficient?

## Conclusion

The current build system works well but mixes concerns in ways that limit
reusability and testability. The proposed refactoring:

- **Separates** orchestration, execution, policy, and presentation
- **Introduces** narrow interfaces enabling dependency injection
- **Preserves** existing behavior via adapter pattern
- **Enables** future extensions (API, alternative builders, distributed builds)
- **Improves** testability via mockable interfaces

The migration can proceed incrementally with low risk, starting with quick wins
(recorder extraction, environment provider) before tackling larger refactors
(scheduler, event system).

**Recommendation**: Start with Phase 1 (interface definitions) and Phase 2
(dependency injection with defaults) to prove the pattern without breaking
changes. Evaluate benefits before committing to full refactor.

---

**Next Steps**:
1. Review proposed interfaces with team
2. Prototype `RunRecorder` extraction to validate approach
3. Create feature branch for incremental refactoring
4. Write integration tests capturing current behavior
5. Implement Phase 1 interface definitions
