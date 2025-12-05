# Post-MVP Build Architecture Analysis

**Date**: 2025-12-05 (Updated: 2025-12-05)  
**Status**: Analysis / Proposed Refactoring  
**Context**: Phase 7 complete, working build system deployed  
**Goal**: API-first architecture for REST/WebSocket support

## Executive Summary

The current `build` package successfully orchestrates parallel package builds
with CRC-based incremental logic, environment isolation, and comprehensive
monitoring. However, the architecture mixes concerns (orchestration, UI, stats,
persistence) in ways that limit reusability and testability.

**Critical Finding**: The proposed refactoring addresses CLI reusability but is
**incomplete for API-first architecture**. This document analyzes the current
design, identifies **7 critical gaps** for API readiness, and proposes an
enhanced layered architecture that supports:

- **Asynchronous build execution** (start → return immediately)
- **Multi-build concurrency** (queue, prioritize, isolate)
- **Real-time event streaming** (WebSocket/SSE compatible)
- **Worker pool reuse** (amortize expensive 27-mount setup)
- **Resource isolation** (separate mount points per build)

The enhanced design separates the core build engine from runtime concerns while
adding a **Build Management Layer** for lifecycle, queuing, and real-time
updates.

## Critical Gaps for API-First Architecture

The original refactoring proposal (sections below) addresses CLI-level concerns
like separation of UI, persistence, and orchestration. However, **API-first
development requires fundamentally different capabilities** that are missing
from both the current implementation and the proposed refactoring:

### Gap #1: No Build Lifecycle Management

**Problem**: Current `DoBuild()` is synchronous and blocking. It returns only
when the entire build completes.

```go
// Current API (synchronous)
func DoBuild(cfg *config.Config, portList []string) error {
    // ... setup ...
    // ... execute all phases ...
    // Returns after completion (minutes/hours later)
}
```

**What REST/WebSocket APIs Need**:
```go
// Async API required
buildID := BuildManager.Start(portList, options)  // Returns immediately
status := BuildManager.GetStatus(buildID)         // Query progress
stream := BuildManager.StreamEvents(buildID)      // Real-time updates
BuildManager.Cancel(buildID)                      // Early termination
```

**Impact**: 
- Cannot implement `POST /builds` endpoint that returns immediately
- Cannot support multiple concurrent API clients monitoring different builds
- Cannot implement `DELETE /builds/{id}` (cancel operation)
- Cannot implement WebSocket streaming without blocking the API server thread

**Missing Components**:
- Build lifecycle state machine (queued → running → completed/failed/cancelled)
- Background execution with goroutine management
- Build ID generation and tracking
- Graceful cancellation propagation to workers

### Gap #2: No Build Queue/Scheduler

**Problem**: Current design assumes one build at a time. Multiple `DoBuild()`
calls would:
- Collide on mount points (`/build/SL0/`, `/build/SL1/`, ...)
- Collide on database writes (single bbolt file, single writer)
- Collide on log directories (`/build/logs/`)
- Exhaust system resources (each build spawns N workers × 27 mounts)

**What REST/WebSocket APIs Need**:
```go
// Queue management required
queue.Enqueue(BuildRequest{Ports: [...], Priority: High})
queue.SetMaxConcurrent(2)  // Limit parallel builds
queue.SetPriority(buildID, Critical)
queue.Cancel(buildID)
```

**Impact**:
- Cannot handle concurrent API requests (`POST /builds` while another build
  runs)
- Cannot implement priority queuing (rebuild security fix while routine build
  queued)
- Cannot implement resource limits (max 2 builds, each with 4 workers = 8 total
  workers)
- Cannot implement fair scheduling (user A vs user B in multi-tenant scenario)

**Missing Components**:
- Build request queue (FIFO, priority-based, or fair-share)
- Concurrency limiter (max N builds system-wide)
- Resource allocation (partition workers across builds)
- Build isolation (separate directories per build ID)

### Gap #3: No Structured Event Stream

**Problem**: Current UI callbacks are fire-and-forget. No event history, no
replay, no buffering.

```go
// Current callback pattern
type BuildUI interface {
    OnStart()
    OnComplete(success, failed, ignored, skipped int)
    OnPackageQueued(pkg *pkg.Package)
    OnPackageStarted(pkg *pkg.Package, workerID int)
    // ... etc ...
}
```

**What REST/WebSocket APIs Need**:
```go
// Event stream required
type Event struct {
    BuildID   string
    Timestamp time.Time
    Type      string  // "package_started", "phase_completed", etc.
    Data      json.RawMessage
}

stream := EventStream.Subscribe(buildID, sinceTimestamp)
for event := range stream {
    websocket.Send(event)  // Stream to client
}
```

**Impact**:
- Cannot implement `GET /builds/{id}/events?since=T` (event history API)
- Cannot implement WebSocket reconnect with replay (client disconnected, needs
  catchup)
- Cannot implement `GET /builds/{id}/logs/tail` (streaming logs)
- Cannot implement audit logging (who started what, when)

**Missing Components**:
- Event journal (in-memory ring buffer or persistent log)
- Event subscription API with filtering
- Event schema versioning
- Backpressure handling (slow WebSocket consumer)

### Gap #4: No Worker Pool Management

**Problem**: Workers created per-build and destroyed after. Each worker
creation:
- Performs 27 `mount` syscalls (~5-10 seconds on NFS)
- Allocates tmpfs (gigabytes of RAM)
- Creates directory structures

**What REST/WebSocket APIs Need**:
```go
// Persistent worker pool required
pool := WorkerPool.Start(maxWorkers=8)
worker := pool.Acquire(buildID)  // Reuse existing worker
worker.Execute(package)
pool.Release(worker)  // Return to pool for next build
```

**Impact**:
- Every API-triggered build pays 5-10s setup cost × N workers
- Cannot implement fast turnaround for small builds (setup > build time)
- Resource thrashing (repeated mount/umount)
- Cannot implement pre-warmed workers for low-latency APIs

**Missing Components**:
- Worker lifecycle management (start, stop, health checks)
- Worker assignment to builds (isolation, cleanup between builds)
- Worker recycling (reset state without destroying environment)
- Worker scaling (grow/shrink pool based on load)

### Gap #5: No Authentication/Authorization

**Problem**: Current code has no concept of users, permissions, or audit
trails.

**What REST/WebSocket APIs Need**:
```go
// Auth/authz required
user := Auth.Authenticate(token)
if !Authz.CanStartBuild(user, portList) {
    return ErrForbidden
}
AuditLog.Record("build_started", user, buildID)
```

**Impact**:
- Cannot implement `Authorization: Bearer <token>` header validation
- Cannot implement RBAC (user roles: admin, builder, viewer)
- Cannot implement multi-tenancy (user A cannot see user B's builds)
- Cannot implement audit logging (compliance, security)

**Missing Components**:
- Authentication middleware (JWT, API keys, OAuth)
- Authorization policy engine (RBAC, ABAC)
- Audit logging (structured logs, retention)
- Multi-tenancy support (user/org isolation)

### Gap #6: No API Versioning Strategy

**Problem**: No plan for evolving APIs without breaking clients.

**What REST/WebSocket APIs Need**:
```go
// Versioning required
// Accept: application/vnd.gosynth.v1+json
router.HandleFunc("/v1/builds", v1.CreateBuild)
router.HandleFunc("/v2/builds", v2.CreateBuild)  // Future: batching support
```

**Impact**:
- Cannot evolve APIs (e.g., add batching, change response schema)
- Cannot deprecate features gracefully
- Cannot support old/new clients simultaneously

**Missing Components**:
- API version negotiation (URL, header, query param)
- Deprecation policy (sunset dates, warnings)
- Version compatibility matrix

### Gap #7: No Distributed/Scale-Out Design

**Problem**: Current design assumes single-node execution. All workers, mounts,
and state on one machine.

**What REST/WebSocket APIs Need (Future)**:
```go
// Distributed design required (future)
type WorkerPool interface {
    Acquire(buildID string) (Worker, error)  // May return remote worker
}

type EventStream interface {
    Publish(event Event)  // Broadcast to all API servers
    Subscribe(buildID) <-chan Event
}
```

**Impact**:
- Cannot scale beyond single-node capacity (16-32 workers max)
- Cannot implement distributed builds (50+ workers across 3 nodes)
- Cannot implement HA (if node dies, builds lost)

**Missing Components** (future):
- Network-transparent worker management (gRPC, RabbitMQ)
- Distributed event bus (Redis Pub/Sub, NATS, Kafka)
- Distributed state (etcd, Consul)
- Load balancing (assign builds to least-loaded node)

**Note**: While full distributed support is post-MVP, designing interfaces now
that *could* support distribution avoids painful rewrites later.

---

### Summary: What's Missing vs. What's Proposed

| Capability | Current | Original Proposal | Required for API |
|------------|---------|-------------------|------------------|
| Sync build execution | ✅ | ✅ | ✅ |
| Async build execution | ❌ | ❌ | ✅ Required |
| Multi-build concurrency | ❌ | ❌ | ✅ Required |
| UI/orchestration separation | ❌ | ✅ | ✅ |
| Event streaming | Partial (callbacks) | ✅ (via BuildEvent) | ✅ (needs journal) |
| Worker pool reuse | ❌ | ❌ | ✅ Required |
| Build queue/scheduler | ❌ | ❌ | ✅ Required |
| Auth/authz | ❌ | ❌ | ✅ Required |
| API versioning | ❌ | ❌ | ✅ Required |
| Distributed design | ❌ | ❌ | ⚠️ Desirable |

**Conclusion**: The original refactoring proposal is a necessary first step
(separation of concerns), but is **insufficient for API development**. We need
a **Build Management Layer** that sits above the orchestration layer and
provides:

1. **BuildManager**: Async lifecycle (Start, Status, Cancel, Wait, List)
2. **EventStream**: Structured event journal with replay
3. **WorkerPool**: Persistent worker reuse with per-build isolation
4. **BuildQueue**: Multi-build scheduling with prioritization

These additions are **required** for Phase 5 (REST API), not optional
enhancements.

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

### Build Management Layer (New for API Support)

The following interfaces form a **Build Management Layer** that sits above the
orchestration layer. They address the 7 critical gaps identified for API-first
architecture.

#### 1. BuildManager Interface

Manages asynchronous build lifecycle (Gap #1).

```go
// BuildManager orchestrates asynchronous build execution with lifecycle
// management, status tracking, and graceful cancellation.
//
// Responsibilities:
//   - Start builds in background goroutines
//   - Track build state (queued → running → completed/failed/cancelled)
//   - Provide status queries and event streaming
//   - Support graceful cancellation with cleanup
//
// Thread-safe: All methods safe for concurrent access.
type BuildManager interface {
    // Start begins a new build asynchronously.
    // Returns buildID immediately (non-blocking).
    // Actual build runs in background with progress reported via events.
    Start(ctx context.Context, req BuildRequest) (buildID string, err error)
    
    // GetStatus returns current build status snapshot.
    // Returns ErrBuildNotFound if buildID invalid.
    GetStatus(buildID string) (*BuildStatus, error)
    
    // Cancel requests graceful build cancellation.
    // Waits for workers to finish current packages, then stops.
    // Returns ErrBuildNotFound if buildID invalid.
    // Returns ErrBuildAlreadyComplete if build finished.
    Cancel(ctx context.Context, buildID string) error
    
    // Wait blocks until build completes or context cancelled.
    // Returns final status or error.
    Wait(ctx context.Context, buildID string) (*BuildStatus, error)
    
    // List returns all builds matching filter criteria.
    // Filter by status, user, time range, etc.
    List(filter BuildFilter) ([]*BuildStatus, error)
    
    // Delete removes build metadata and logs.
    // Returns ErrBuildRunning if build still active.
    Delete(buildID string) error
}

// BuildRequest specifies what to build.
type BuildRequest struct {
    Ports     []string          // Port origins (e.g., "editors/vim")
    Options   BuildOptions      // Build configuration
    UserID    string            // User requesting build (for auth/audit)
    Priority  BuildPriority     // Queue priority
}

// BuildOptions configures build behavior.
type BuildOptions struct {
    Force       bool              // Ignore CRC, rebuild all
    FetchOnly   bool              // Download distfiles only
    MaxWorkers  int               // Override default worker count (0=default)
    Timeout     time.Duration     // Max build duration (0=unlimited)
    Tags        map[string]string // User-defined metadata
}

// BuildPriority affects queue ordering.
type BuildPriority int

const (
    PriorityLow BuildPriority = iota
    PriorityNormal
    PriorityHigh
    PriorityCritical  // Security fixes, etc.
)

// BuildStatus represents current build state.
type BuildStatus struct {
    BuildID    string
    State      BuildState
    Request    BuildRequest
    
    // Timing
    QueuedAt   time.Time
    StartedAt  *time.Time  // nil if not started
    CompletedAt *time.Time  // nil if running
    Duration   time.Duration
    
    // Progress
    Packages   PackageStats
    
    // Results
    Error      string  // Non-empty if build failed
    LogPath    string  // Path to build logs
}

// BuildState is the high-level build lifecycle state.
type BuildState string

const (
    StateQueued    BuildState = "queued"     // Waiting for resources
    StateRunning   BuildState = "running"    // Actively building
    StateCompleted BuildState = "completed"  // Finished successfully
    StateFailed    BuildState = "failed"     // Build failed
    StateCancelled BuildState = "cancelled"  // User cancelled
)

// PackageStats tracks per-package progress.
type PackageStats struct {
    Total    int  // Total packages to build
    Pending  int  // Not yet started
    Building int  // Currently building
    Success  int  // Successfully built
    Failed   int  // Build failed
    Skipped  int  // Skipped (dependencies failed)
}

// BuildFilter specifies list query criteria.
type BuildFilter struct {
    States   []BuildState  // Filter by state
    UserID   string        // Filter by user
    Since    *time.Time    // Filter by queued time
    Limit    int           // Max results (0=all)
}
```

**Implementation Notes**:
- Initial implementation: in-memory state, single-node only
- Future: persistent state (bbolt/SQLite), distributed coordination (etcd)
- Build ID generation: UUID v4 or `time.Now().UnixNano()` for sortability

#### 2. EventStream Interface

Provides structured event journal with replay (Gap #3).

```go
// EventStream publishes and subscribes to build events with history replay.
//
// Responsibilities:
//   - Publish structured events from orchestrator
//   - Subscribe to event streams (all builds or specific buildID)
//   - Replay historical events (for reconnect, audit)
//   - Buffer events for slow consumers (backpressure)
//
// Thread-safe: All methods safe for concurrent access.
type EventStream interface {
    // Publish sends event to all subscribers.
    // Non-blocking: event buffered if subscribers slow.
    Publish(event Event)
    
    // Subscribe returns channel of events for buildID.
    // If buildID empty, subscribes to all builds.
    // If since non-zero, replays historical events first.
    // Channel closed when unsubscribed or buildID completed.
    Subscribe(ctx context.Context, opts SubscribeOptions) (<-chan Event, error)
    
    // Unsubscribe closes subscription channel.
    Unsubscribe(subscriptionID string)
    
    // History returns past events for buildID.
    // If buildID empty, returns events for all builds.
    History(buildID string, since time.Time, limit int) ([]Event, error)
}

// SubscribeOptions configures event subscription.
type SubscribeOptions struct {
    BuildID    string        // Empty = all builds
    EventTypes []EventType   // Filter by type (empty = all)
    Since      time.Time     // Replay from timestamp
    BufferSize int           // Channel buffer (default 100)
}

// Event represents a structured build event.
type Event struct {
    ID        string          // Unique event ID
    BuildID   string          // Build that generated event
    Timestamp time.Time       // Event time (UTC)
    Type      EventType       // Event category
    Data      interface{}     // Type-specific payload
}

// EventType categorizes events.
type EventType string

const (
    // Build lifecycle
    EventBuildQueued    EventType = "build.queued"
    EventBuildStarted   EventType = "build.started"
    EventBuildCompleted EventType = "build.completed"
    EventBuildFailed    EventType = "build.failed"
    EventBuildCancelled EventType = "build.cancelled"
    
    // Package lifecycle
    EventPackageQueued  EventType = "package.queued"
    EventPackageStarted EventType = "package.started"
    EventPackageSuccess EventType = "package.success"
    EventPackageFailed  EventType = "package.failed"
    EventPackageSkipped EventType = "package.skipped"
    
    // Worker events
    EventWorkerAcquired EventType = "worker.acquired"
    EventWorkerReleased EventType = "worker.released"
    EventWorkerFailed   EventType = "worker.failed"
    
    // Phase events
    EventPhaseStarted   EventType = "phase.started"
    EventPhaseCompleted EventType = "phase.completed"
    EventPhaseFailed    EventType = "phase.failed"
    
    // System events
    EventLogLine        EventType = "log.line"
    EventStatsUpdate    EventType = "stats.update"
)

// Example event data structures:

type BuildStartedData struct {
    BuildID   string
    Ports     []string
    Workers   int
}

type PackageStartedData struct {
    Package   string  // e.g., "editors/vim-9.0.1"
    WorkerID  int
}

type PhaseStartedData struct {
    Package   string
    Phase     string  // "fetch", "extract", "build", etc.
}

type LogLineData struct {
    Package   string
    Phase     string
    Line      string
}
```

**Implementation Notes**:
- Initial: in-memory ring buffer (1000 events per build, FIFO eviction)
- Future: persistent journal (append-only log, SQLite, or event store)
- WebSocket integration: subscribe → range over channel → JSON encode events
- Backpressure: slow subscribers get dropped events (log warning)

#### 3. WorkerPool Interface

Manages persistent worker lifecycle and reuse (Gap #4).

```go
// WorkerPool manages a pool of pre-initialized workers with lifecycle
// management, resource allocation, and build isolation.
//
// Responsibilities:
//   - Start/stop persistent workers (amortize mount setup cost)
//   - Assign workers to builds (isolation via per-build directories)
//   - Health monitoring and worker recycling
//   - Dynamic pool resizing based on load
//
// Thread-safe: All methods safe for concurrent access.
type WorkerPool interface {
    // Start initializes the worker pool with given capacity.
    // Workers created lazily on first Acquire() call.
    // Returns error if pool already started.
    Start(ctx context.Context, size int) error
    
    // Acquire returns a worker for buildID.
    // Blocks until worker available or context cancelled.
    // Worker isolated to buildID (separate mount points).
    // Caller must call Release() when done.
    Acquire(ctx context.Context, buildID string) (Worker, error)
    
    // Release returns worker to pool for reuse.
    // Worker cleaned up (remove build artifacts) before reuse.
    // Returns error if worker in bad state (worker destroyed).
    Release(worker Worker) error
    
    // Resize changes pool capacity (grows or shrinks).
    // Growing: creates new workers immediately.
    // Shrinking: waits for workers to become idle, then destroys.
    Resize(ctx context.Context, newSize int) error
    
    // Stop gracefully shuts down pool.
    // Waits for all workers to finish current tasks.
    // Context timeout controls max wait time.
    Stop(ctx context.Context) error
    
    // Stats returns current pool statistics.
    Stats() WorkerPoolStats
}

// Worker represents a build executor with environment isolation.
type Worker interface {
    // ID returns unique worker identifier.
    ID() int
    
    // Execute runs command in worker's isolated environment.
    // BuildID used to determine mount point isolation.
    Execute(ctx context.Context, cmd ExecCommand) (*ExecResult, error)
    
    // Cleanup removes build artifacts for buildID.
    // Called automatically by Release(), but exposed for explicit use.
    Cleanup(buildID string) error
    
    // Health checks worker state.
    // Returns error if worker environment corrupted.
    Health(ctx context.Context) error
}

// WorkerPoolStats tracks pool health and utilization.
type WorkerPoolStats struct {
    Capacity  int  // Max workers
    Active    int  // Currently executing builds
    Idle      int  // Available for assignment
    Unhealthy int  // Failed health checks
}

// ExecCommand specifies command execution parameters.
type ExecCommand struct {
    Program string
    Args    []string
    Env     []string
    WorkDir string
    Timeout time.Duration
}

// ExecResult contains command execution result.
type ExecResult struct {
    ExitCode int
    Stdout   string
    Stderr   string
    Duration time.Duration
}
```

**Implementation Notes**:
- Worker creation: call `environment.New("bsd")` once, reuse for multiple builds
- Build isolation: mount points use `/build/{buildID}/SL{N}/` pattern (see
  Mount Point Isolation section)
- Cleanup between builds: `rm -rf /build/{oldBuildID}/`, not full teardown
- Health checks: verify mounts present, chroot accessible, ~5s interval
- Initial size: `config.MaxWorkers` (default 8)

#### 4. BuildQueue Interface

Implements multi-build scheduling with prioritization (Gap #2).

```go
// BuildQueue manages multiple concurrent builds with priority scheduling,
// resource limits, and fairness policies.
//
// Responsibilities:
//   - Queue build requests when resources exhausted
//   - Prioritize builds (critical > high > normal > low)
//   - Enforce concurrency limits (max N simultaneous builds)
//   - Fair scheduling (round-robin per user in multi-tenant)
//
// Thread-safe: All methods safe for concurrent access.
type BuildQueue interface {
    // Enqueue adds build request to queue.
    // Returns buildID immediately (build may not start yet).
    // Build transitions: queued → running when resources available.
    Enqueue(req BuildRequest) (buildID string, err error)
    
    // Dequeue returns next build ready to execute.
    // Blocks until build available or context cancelled.
    // Considers priority, fairness, and resource limits.
    // Returns nil if queue empty and no builds waiting.
    Dequeue(ctx context.Context) (*QueuedBuild, error)
    
    // SetPriority changes priority of queued build.
    // Returns ErrBuildNotQueued if build already running.
    SetPriority(buildID string, priority BuildPriority) error
    
    // Remove cancels queued build before it starts.
    // Returns ErrBuildNotQueued if already running.
    Remove(buildID string) error
    
    // Stats returns queue statistics.
    Stats() QueueStats
    
    // SetMaxConcurrent changes concurrent build limit.
    // If reducing limit below current running count, waits for builds to
    // complete (does not cancel).
    SetMaxConcurrent(max int) error
}

// QueuedBuild represents a build ready for execution.
type QueuedBuild struct {
    BuildID  string
    Request  BuildRequest
    QueuedAt time.Time
}

// QueueStats tracks queue health.
type QueueStats struct {
    Queued     int  // Waiting for resources
    Running    int  // Currently executing
    MaxConcurrent int  // Concurrency limit
}
```

**Implementation Notes**:
- Initial: simple FIFO + priority queue (heap-based)
- Future: fair-share scheduling (weighted round-robin per user)
- Concurrency limit: default 1 (sequential builds), configurable via config or
  API
- Resource allocation: each build gets `MaxWorkers / MaxConcurrent` workers
  (e.g., 8 workers, 2 builds = 4 workers each)

---

### Orchestration Layer Interfaces (Original Proposal)

The following interfaces refactor the existing orchestration layer for better
separation of concerns. They work with the Build Management Layer above.

#### 5. Scheduler Interface

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

### 6. BuildExecutor Interface

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

### 7. PhaseRunner Interface

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

### 8. IncrementalPolicy Interface

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

### 9. EnvironmentProvider Interface

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

### 10. RunRecorder Interface

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

### 11. BuildEvents Interface (Original Proposal - Superseded)

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

## Mount Point Isolation Strategy

### Problem Statement

Current design uses fixed mount points per worker:
```
/build/SL0/   → Worker 0
/build/SL1/   → Worker 1
...
/build/SL7/   → Worker 7
```

This works for single builds but **collides when multiple builds run
concurrently**:
- Build A starts with 8 workers (SL0-SL7)
- Build B starts while A running → collides on same mount points
- Workers from different builds interfere with each other's artifacts

**User Requirement**: "For multi-builds we will need separate mount points for
each build run, otherwise they'll just collide."

### Proposed Solution: Build-Scoped Directories

Use a two-level hierarchy: `{BuildBase}/{buildID}/SL{N}/`

```
/build/
├── build-123e4567-e89b-12d3-a456-426614174000/
│   ├── SL0/  → Worker 0 for build 123e...
│   ├── SL1/  → Worker 1 for build 123e...
│   └── SL7/
├── build-abcd1234-e89b-12d3-a456-426614174001/
│   ├── SL0/  → Worker 0 for build abcd...
│   └── SL3/
└── logs/
    ├── build-123e.../  → Logs for build 123e...
    └── build-abcd.../  → Logs for build abcd...
```

**Benefits**:
- **Isolation**: Each build has dedicated directories (no collision)
- **Concurrency**: Builds run simultaneously without interference
- **Cleanup**: Delete entire `build-{ID}/` directory when done
- **Debugging**: Easy to identify which build owns which mounts
- **Compatibility**: Worker IDs remain 0-N (no changes to worker logic)

### Implementation Changes

#### 1. Worker Environment Creation

**Current** (`build/build.go:428`):
```go
env, err := environment.New("bsd")
if err != nil {
    return nil, err
}
```

**Proposed** (with build-scoped base path):
```go
// WorkerPool.Acquire() sets per-build base path
env, err := environment.NewWithBasePath("bsd", worker.ID, buildID, cfg)
if err != nil {
    return nil, err
}

// Inside environment/bsd/bsd.go
func NewWithBasePath(workerID int, buildID string, cfg *config.Config) (*BSDEnvironment, error) {
    // Build-scoped base path
    basePath := filepath.Join(cfg.BuildBase, fmt.Sprintf("build-%s", buildID), fmt.Sprintf("SL%d", workerID))
    
    // Create directory structure
    if err := os.MkdirAll(basePath, 0755); err != nil {
        return nil, err
    }
    
    // Rest of setup (27 mounts) unchanged
    // ...
}
```

#### 2. WorkerPool Lifecycle

**Worker Acquisition** (assigns worker to buildID):
```go
func (p *WorkerPool) Acquire(ctx context.Context, buildID string) (Worker, error) {
    // Wait for idle worker
    worker := <-p.idleWorkers
    
    // Configure worker for this build
    worker.buildID = buildID
    worker.basePath = filepath.Join(p.cfg.BuildBase, fmt.Sprintf("build-%s", buildID))
    
    // Create build-specific directories
    if err := os.MkdirAll(worker.basePath, 0755); err != nil {
        p.idleWorkers <- worker  // Return to pool
        return nil, err
    }
    
    // Reconfigure mounts to point to new base path
    if err := worker.env.SetBasePath(worker.basePath); err != nil {
        p.idleWorkers <- worker
        return nil, err
    }
    
    return worker, nil
}
```

**Worker Release** (cleanup between builds):
```go
func (p *WorkerPool) Release(worker Worker) error {
    // Remove build-specific artifacts
    buildPath := filepath.Join(p.cfg.BuildBase, fmt.Sprintf("build-%s", worker.buildID))
    
    // Unmount all 27 mount points (but keep environment alive)
    if err := worker.env.UnmountAll(); err != nil {
        // Worker corrupted, destroy it
        worker.env.Cleanup()
        return err
    }
    
    // Delete build directory
    if err := os.RemoveAll(buildPath); err != nil {
        log.Warn("Failed to cleanup build directory: %v", err)
    }
    
    // Return worker to pool (ready for next build)
    worker.buildID = ""
    p.idleWorkers <- worker
    
    return nil
}
```

#### 3. Environment Interface Extension

Add `SetBasePath()` method for dynamic reconfiguration:

```go
// In environment/environment.go
type Environment interface {
    Setup(ctx context.Context, cfg *EnvironmentConfig) error
    Execute(ctx context.Context, cmd ExecCommand) (*ExecResult, error)
    Cleanup() error
    GetBasePath() string
    
    // NEW: Reconfigure base path for multi-build support
    SetBasePath(basePath string) error
    
    // NEW: Unmount all mounts but keep environment alive
    UnmountAll() error
}
```

**BSD Implementation** (`environment/bsd/bsd.go`):
```go
func (e *BSDEnvironment) SetBasePath(basePath string) error {
    // Update base path
    e.basePath = basePath
    
    // Remount all 27 mount points to new base path
    for _, mnt := range e.mounts {
        newTarget := filepath.Join(basePath, mnt.relPath)
        
        // Create mount point
        if err := os.MkdirAll(newTarget, 0755); err != nil {
            return err
        }
        
        // Mount (nullfs, tmpfs, devfs, procfs)
        if err := mount(mnt.source, newTarget, mnt.fstype, mnt.flags); err != nil {
            return err
        }
    }
    
    return nil
}

func (e *BSDEnvironment) UnmountAll() error {
    // Unmount in reverse order (innermost first)
    for i := len(e.mounts) - 1; i >= 0; i-- {
        target := filepath.Join(e.basePath, e.mounts[i].relPath)
        if err := unmount(target); err != nil {
            return fmt.Errorf("unmount %s: %w", target, err)
        }
    }
    return nil
}
```

### Migration Path

**Phase 0a: Add buildID parameter** (backward compatible)
1. Add `buildID` parameter to `environment.New()` (default to empty string)
2. If `buildID == ""`, use legacy path `/build/SL{N}/` (current behavior)
3. If `buildID != ""`, use new path `/build/build-{ID}/SL{N}/`
4. Update tests to pass explicit buildID

**Phase 0b: Implement WorkerPool**
1. Create `WorkerPool` with `Acquire()`/`Release()` methods
2. Pool initializes N workers at startup (one-time mount setup)
3. `Acquire()` assigns worker to buildID, reconfigures base path
4. `Release()` unmounts, deletes build directory, returns to pool

**Phase 0c: Update BuildManager**
1. `BuildManager.Start()` generates unique buildID (UUID)
2. Acquires workers from pool: `workers := pool.AcquireN(buildID, maxWorkers)`
3. Passes buildID to orchestrator for logging/metrics
4. On completion: `pool.ReleaseAll(workers)`

**Verification**:
- Single build: works with `/build/build-{ID}/SL{N}/` (no `/build/SL{N}/`)
- Concurrent builds: `/build/build-{ID1}/` and `/build/build-{ID2}/` coexist
- Worker reuse: single worker handles build-{ID1}, then build-{ID2} (different
  paths)
- Cleanup: `rm -rf /build/build-{ID}/` removes all artifacts

### Performance Considerations

**Mount/Unmount Overhead**:
- **Current**: 27 mounts per worker × N workers = 27N mounts per build
- **Proposed**: 27 mounts per worker at pool startup (one-time), then 27
  unmount/remount per `Acquire()` (~2-3s on NFS)
- **Optimization**: Keep mounts alive across builds (only change symlinks or
  chroot path), reducing to ~0.1s per `Acquire()`

**Disk Space**:
- Each build needs ~5-10GB temporary space (work directories, distfiles)
- With 2 concurrent builds: 10-20GB peak usage
- Cleanup after each build keeps usage bounded

**Future Optimization** (post-MVP):
- Use bind mounts or overlayfs to share read-only layers (`/xports`,
  `/packages`)
- Only isolate writable layers (`/construction`, `/distfiles`)
- Reduces per-build overhead to ~500MB

### Summary

**What Changes**:
- Mount points: `/build/SL{N}/` → `/build/build-{ID}/SL{N}/`
- Worker lifecycle: create once → acquire/release many times
- Environment interface: add `SetBasePath()`, `UnmountAll()`

**What Stays the Same**:
- Worker IDs remain 0-N (no changes to phase execution logic)
- 27-mount chroot structure unchanged (same nullfs/tmpfs/devfs layout)
- Package building logic unchanged (`executePhase()`, dependencies, CRC)

**Risk**: Medium (involves mount operations, but BSD backend has 38 unit tests +
8 integration tests covering mount edge cases)

## Migration Strategy

### Overview: Revised Phased Approach

The original migration strategy focused on refactoring the orchestration layer
(separation of concerns). However, **API-first architecture requires** the Build
Management Layer **first**, before orchestration refactoring provides value.

**Revised Phase Order**:
1. **Phase 0**: Build Management Layer (async, queuing, events, worker pool) →
   **Enables API development**
2. **Phase 1-6**: Orchestration refactoring (scheduler, executor, policies) →
   Improves testability, reusability
3. **Phase 7**: REST API Layer (HTTP endpoints, WebSocket, auth) → Consumes
   Build Management Layer

**Rationale**: Without Phase 0, the REST API cannot be built (no async
execution, no multi-build support, no event streaming). The orchestration
refactoring (original phases 1-6) improves code quality but doesn't unblock API
work.

---

### Phase 0: Build Management Layer (NEW - Required for API)

**Goal**: Implement 4 core interfaces that enable API-first architecture.

**Deliverables**:
1. `BuildManager` interface + in-memory implementation
2. `EventStream` interface + ring buffer implementation
3. `WorkerPool` interface + persistent worker pool
4. `BuildQueue` interface + priority queue implementation
5. Mount point isolation (per-build directories)
6. Integration with existing `DoBuild()` (backward compatible wrapper)

**Effort**: 30-40 hours (4-5 days)

#### Phase 0.1: BuildManager + EventStream (10-12h)

**Tasks**:
1. Create `build/manager.go` with `BuildManager` interface
2. Implement `DefaultBuildManager`:
   - `Start()`: generate UUID, launch goroutine calling `DoBuild()`, publish
     `EventBuildStarted`
   - `GetStatus()`: query in-memory map[buildID]*BuildStatus
   - `Cancel()`: call context cancel function
   - `Wait()`: block on completion channel
   - `List()`: filter in-memory builds by state/user/time
3. Create `build/events.go` with `EventStream` interface
4. Implement `RingBufferEventStream`:
   - Ring buffer: 1000 events per build (FIFO eviction)
   - `Publish()`: append to ring buffer, notify subscribers
   - `Subscribe()`: create buffered channel, replay history, forward new events
   - `History()`: query ring buffer by buildID/time range
5. Unit tests: 20 tests covering lifecycle, cancellation, event replay

**Verification**: Build can be started async, status queried, events subscribed.

#### Phase 0.2: WorkerPool + Mount Isolation (12-15h)

**Tasks**:
1. Create `build/workerpool.go` with `WorkerPool` interface
2. Implement `DefaultWorkerPool`:
   - `Start()`: create N workers, call `environment.New()` once per worker
   - `Acquire()`: block until worker available, assign buildID, reconfigure
     mounts
   - `Release()`: unmount, cleanup build directory, return to pool
   - `Health()`: verify mounts present, chroot accessible
3. Extend `environment.Environment` interface:
   - Add `SetBasePath(basePath string) error`
   - Add `UnmountAll() error`
4. Implement `SetBasePath()` in `environment/bsd/bsd.go`:
   - Unmount all 27 mounts from old base path
   - Remount all 27 mounts to new base path (`/build/build-{ID}/SL{N}/`)
5. Update `DoBuild()` to accept `buildID` parameter (default to `""` for legacy
   behavior)
6. Unit tests: 15 tests covering acquisition, release, mount reconfiguration,
   cleanup

**Verification**: Worker can be acquired by build-{ID1}, released, then acquired
by build-{ID2} (different mount points).

#### Phase 0.3: BuildQueue + Multi-Build Support (8-10h)

**Tasks**:
1. Create `build/queue.go` with `BuildQueue` interface
2. Implement `PriorityBuildQueue`:
   - `Enqueue()`: add to heap sorted by priority + FIFO within priority
   - `Dequeue()`: pop from heap, block if concurrent limit reached
   - `SetPriority()`: remove + re-add with new priority
   - `SetMaxConcurrent()`: enforce limit (default 1)
3. Integrate with `BuildManager`:
   - `Start()` → `Enqueue()` → dequeue when resources available
   - Track running builds count
   - Wait for slots before calling `DoBuild()`
4. Unit tests: 12 tests covering priority ordering, concurrency limits, fairness

**Verification**: 2 builds enqueued, first starts immediately, second waits
until first completes (with MaxConcurrent=1).

#### Phase 0.4: Integration + Backward Compatibility (5-8h)

**Tasks**:
1. Update `main.go` to use `BuildManager`:
   ```go
   buildMgr := build.NewDefaultBuildManager(cfg, workerPool, eventStream, queue)
   buildID, err := buildMgr.Start(ctx, build.BuildRequest{Ports: portList})
   if err != nil {
       return err
   }
   
   // Wait for completion (CLI is synchronous)
   status, err := buildMgr.Wait(ctx, buildID)
   ```
2. Keep legacy `DoBuild()` as wrapper for CLI compatibility:
   ```go
   func DoBuild(cfg *config.Config, portList []string) error {
       buildMgr := getGlobalBuildManager()  // Singleton
       buildID, err := buildMgr.Start(ctx, BuildRequest{Ports: portList})
       if err != nil {
           return err
       }
       _, err = buildMgr.Wait(ctx, buildID)
       return err
   }
   ```
3. Update integration tests to test both `BuildManager.Start()` and legacy
   `DoBuild()`
4. Documentation: Update DEVELOPMENT.md with new architecture diagram

**Verification**: Existing CLI behavior unchanged, new `BuildManager` API tested
via integration tests.

**Exit Criteria**:
- [ ] All 4 interfaces implemented with in-memory backends
- [ ] 47+ unit tests passing (BuildManager, EventStream, WorkerPool, BuildQueue)
- [ ] Integration tests pass (single build, concurrent builds, cancellation)
- [ ] Mount point isolation verified: `/build/build-{ID1}/SL0/` vs
  `/build/build-{ID2}/SL0/` coexist
- [ ] Legacy `DoBuild()` wrapper functional (CLI backward compatible)
- [ ] Documentation updated (architecture diagram, API examples)

---

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

## Effort Estimates (Revised for API-First Development)

### Phase 0: Build Management Layer (NEW - API Required)
**Effort**: 30-40 hours (4-5 days)

| Sub-Phase | Description | Time |
|-----------|-------------|------|
| 0.1 | BuildManager + EventStream interfaces + in-memory impl | 10-12h |
| 0.2 | WorkerPool + mount point isolation | 12-15h |
| 0.3 | BuildQueue + multi-build support | 8-10h |
| 0.4 | Integration + backward compatibility | 5-8h |
| **Total** | **Phase 0** | **30-40h** |

**Exit Criteria**: Async builds, event streaming, worker reuse, concurrent build
isolation verified via tests.

### Original Refactoring Phases (1-6)
**Effort**: 51-70 hours (7-9 days) - unchanged from original proposal

| Phase | Description | Time |
|-------|-------------|------|
| 1 | Extract interfaces + adapters | 6-8h |
| 2 | Inject dependencies with defaults | 8-10h |
| 3 | Create DoBuildLite | 10-12h |
| 4 | Refactor Scheduler | 12-15h |
| 5 | Extract PhaseRunner | 8-10h |
| 6 | Extract Events + UI | 10-12h |
| **Total** | **Phases 1-6** | **51-70h** |

### Phase 7: REST API Layer (NEW)
**Effort**: 20-30 hours (3-4 days)

| Task | Description | Time |
|------|-------------|------|
| 7.1 | HTTP server + routing (chi/gorilla) | 4-5h |
| 7.2 | REST endpoints (POST /builds, GET /builds/{id}, etc.) | 6-8h |
| 7.3 | WebSocket streaming (/builds/{id}/events) | 5-6h |
| 7.4 | Authentication middleware (JWT/API keys) | 4-5h |
| 7.5 | OpenAPI spec + documentation | 3-4h |
| 7.6 | Integration tests (API endpoints) | 5-7h |
| **Total** | **Phase 7** | **20-30h** |

### Testing & Documentation
**Effort**: 10-15 hours (1-2 days)

| Task | Time |
|------|------|
| Integration tests (multi-build scenarios) | 5-6h |
| Performance testing (worker pool, event streaming) | 3-4h |
| Documentation updates (DEVELOPMENT.md, API guide) | 3-4h |
| **Total** | **10-15h** |

### Grand Total
**Total Effort**: **110-140 hours (14-18 days)**

**Breakdown**:
- Phase 0 (Build Management Layer): 30-40h ← **Required for API**
- Phases 1-6 (Orchestration Refactoring): 51-70h ← Improves quality
- Phase 7 (REST API): 20-30h ← **API development**
- Testing/docs: 10-15h

**Critical Path for API Development**:
1. Phase 0 (Build Management) → 30-40h
2. Phase 7 (REST API) → 20-30h
3. **Total for minimal API**: 50-70h (7-9 days)

Phases 1-6 (orchestration refactoring) can be done in parallel or deferred
post-API launch if needed.

## Conclusion

The current build system works well but is **not API-ready**. This analysis
identifies **7 critical gaps** that block REST/WebSocket API development:

1. No async build execution (synchronous `DoBuild()` blocks)
2. No multi-build concurrency (mount/DB collisions)
3. No structured event streaming (fire-and-forget callbacks)
4. No worker pool reuse (expensive per-build setup)
5. No authentication/authorization (security)
6. No API versioning strategy (evolution)
7. No distributed design (future scalability)

The original refactoring proposal (separation of concerns) is **necessary but
insufficient**. We must add a **Build Management Layer** (Phase 0) that
provides:

- **BuildManager**: Async lifecycle (Start, Status, Cancel, Wait)
- **EventStream**: Event journal with replay (WebSocket streaming)
- **WorkerPool**: Persistent worker reuse (amortize mount setup cost)
- **BuildQueue**: Multi-build scheduling (concurrency limits, priorities)
- **Mount Isolation**: Per-build directories (`/build/{buildID}/SL{N}/`)

**Revised Architecture**:
```
CLI/API → BuildManager → BuildQueue → WorkerPool → Orchestrator → Environment
                ↓
           EventStream (WebSocket, logging, audit)
```

**Benefits**:
- **API-first**: Async builds, real-time streaming, multi-build support
- **Testability**: All layers mockable (BuildManager, EventStream, WorkerPool)
- **Scalability**: Worker pool reuse, mount point isolation
- **Extensibility**: Pluggable backends (in-memory → Redis → distributed)

**Recommendation**:
1. **Implement Phase 0 (Build Management Layer) FIRST** - required for API work
   (30-40h)
2. **Implement Phase 7 (REST API)** - enables API development (20-30h)
3. **Defer Phases 1-6 (Orchestration Refactoring)** - improves quality but not
   blocking (51-70h)

**Why Phase 0 First?**:
- Unblocks REST API development immediately
- Provides async execution, event streaming, worker reuse
- Backward compatible (legacy `DoBuild()` wrapper for CLI)
- Can be tested independently (47+ unit tests)

**Why Defer Phases 1-6?**:
- Improves code quality (testability, separation) but doesn't add features
- API can be built on current orchestration layer (Phase 0 provides async
  wrapper)
- Can refactor incrementally after API launch

**Total Effort for API-Ready System**: 50-70 hours (Phase 0 + Phase 7)  
**Total Effort for Full Refactoring**: 110-140 hours (Phase 0-7 + testing)

---

**Next Steps**:
1. Review Phase 0 interfaces (BuildManager, EventStream, WorkerPool, BuildQueue)
2. Validate mount point isolation strategy (build-scoped directories)
3. Create feature branch: `feature/phase-0-build-management`
4. Implement Phase 0.1 (BuildManager + EventStream)
5. Write 47+ unit tests (lifecycle, events, worker pool, queue)
6. Update DEVELOPMENT.md with Phase 0 tasks and exit criteria
