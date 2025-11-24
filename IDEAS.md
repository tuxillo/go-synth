# Go-Dsynth Architecture Ideas & Future Plans

This document outlines potential architectural improvements and features for go-dsynth.

## Table of Contents
- [Library/Package Architecture](#librarypackage-architecture)
- [Build Identification & Database](#build-identification--database)
- [API Design](#api-design)
- [Benefits & Use Cases](#benefits--use-cases)

---

## Library/Package Architecture

### Current State Analysis

The current go-dsynth has several packages that mix concerns:
- `pkg` - Package metadata, dependency resolution, CRC database, build status tracking (mixed concerns)
- `build` - Build execution, tightly coupled to mount/worker
- `mount` - DragonFly/FreeBSD specific mount operations
- `config` - Configuration management (already independent ✓)
- `log` - Build logging (already independent ✓)

### Proposed Refactored Structure

```
dsynth/
├── pkg/              # Package metadata & deps (pure library)
│   ├── package.go    # Package struct and parsing
│   ├── deps.go       # Dependency resolution
│   └── registry.go   # Package registry
│
├── builddb/          # Build database (pure library)
│   ├── crc.go        # CRC calculation
│   ├── db.go         # Database interface
│   ├── boltdb.go     # BoltDB implementation
│   └── types.go      # Build record types
│
├── builder/          # Build execution (library)
│   ├── worker.go     # Worker pool
│   ├── executor.go   # Build phase execution
│   ├── scheduler.go  # Dependency-aware scheduling
│   └── environment.go # Build environment interface
│
├── environment/      # Platform-specific (implementations)
│   ├── dragonfly.go  # DragonFly-specific mounts
│   ├── freebsd.go    # FreeBSD jails
│   └── linux.go      # Linux containers?
│
├── api/              # API server (new)
│   ├── server.go
│   ├── handlers.go
│   └── websocket.go
│
└── cmd/
    └── dsynth/       # CLI tool (thin wrapper)
        └── main.go
```

### Core Reusable Packages

#### 1. `pkg` package (Package Metadata & Dependencies)
- **Purpose**: Pure library for understanding ports and dependencies
- **Responsibilities**:
  - Port specification parsing
  - Package metadata extraction
  - Dependency graph construction
  - Package registry management
- **Reusable for**: Any tool that needs to understand ports/dependencies
- **Dependencies**: Minimal (only config)

#### 2. `builddb` package (Build Database)
- **Purpose**: Track build history and status
- **Responsibilities**:
  - Build record storage/retrieval
  - CRC calculation and comparison
  - Database interface abstraction
  - Multiple backend support (custom binary, BoltDB, SQLite)
- **Reusable for**: Build tracking, reproducible builds, auditing
- **Dependencies**: None (pure library)

#### 3. `builder` package (Build Execution Engine)
- **Purpose**: Core build orchestration logic
- **Responsibilities**:
  - Worker pool management
  - Build phase execution
  - Dependency-aware scheduling
  - Build environment abstraction (via interface)
- **Reusable for**: Any distributed build system
- **Dependencies**: `pkg`, `builddb`, `environment` (interface only)

#### 4. `environment` package (Platform-Specific)
- **Purpose**: Abstract build environment setup
- **Responsibilities**:
  - Mount operations (nullfs, tmpfs, devfs)
  - Chroot/jail/container management
  - Platform-specific isolation
- **Interface-based**: Easy to add new platforms
- **Implementations**:
  - DragonFly: nullfs mounts + chroot
  - FreeBSD: jails
  - Linux: containers/namespaces?

#### 5. `api` package (API Server)
- **Purpose**: REST + WebSocket API for control and monitoring
- **Responsibilities**:
  - HTTP handlers
  - WebSocket event streaming
  - Authentication/authorization
  - Rate limiting
- **Reusable for**: Web UI, CLI, CI/CD integration
- **Dependencies**: `builder`, `builddb`, `pkg`

---

## Build Identification & Database

### Problem
Current CRC database only tracks "latest" state. No history, no audit trail, no way to correlate build failures with environment changes.

### Solution: Comprehensive Build Tracking

#### Build Identity Structure

```go
type BuildID struct {
    // Unique identifier
    UUID        string    // Random UUID for this specific build attempt
    
    // Package identity
    PortDir     string    // "editors/vim"
    Version     string    // "9.1.1199"
    Flavor      string    // Optional flavor
    
    // Build environment
    PortsCRC    uint32    // CRC of port directory
    PortsCommit string    // Git commit of ports tree
    SystemVer   string    // "DragonFly 6.4"
    
    // Build context
    BuildTime   time.Time // When build started
    WorkerID    int       // Which worker built it
    Options     string    // Build options hash
    
    // Composite key for lookups
    Key         string    // "editors/vim@9.1.1199"
}

type BuildRecord struct {
    ID          BuildID
    
    // Build status
    Status      string    // "success", "failed", "running"
    StartTime   time.Time
    EndTime     time.Time
    Duration    time.Duration
    
    // Package info
    PkgFile     string    // "vim-9.1.1199.pkg"
    PkgSize     int64
    PkgChecksum string    // SHA256 of package
    
    // Dependencies (what was actually used)
    DepVersions map[string]string  // "devel/pkgconf" -> "2.3.0"
    
    // Build artifacts
    LogFile     string    // Path to build log
    Phase       string    // Last completed phase
    ErrorMsg    string    // If failed
    
    // Reproducibility
    EnvVars     map[string]string
    MakeArgs    []string
}
```

#### Database Schema

**Buckets/Tables:**

1. **`builds`** - All build attempts
   - Key: `UUID`
   - Value: `BuildRecord`
   - Purpose: Complete history of every build attempt

2. **`packages`** - Latest successful build per package
   - Key: `portdir@version`
   - Value: `UUID` (reference to builds bucket)
   - Purpose: Fast lookup of current package state

3. **`crc_index`** - Fast CRC lookup
   - Key: `portdir`
   - Value: `{crc, uuid}`
   - Purpose: Detect port directory changes

4. **`history`** - Build history timeline
   - Key: `timestamp + uuid`
   - Value: `BuildRecord`
   - Purpose: Time-based queries, statistics

5. **`workers`** - Worker activity log
   - Key: `worker_id + timestamp`
   - Value: `{build_uuid, status}`
   - Purpose: Worker performance tracking

#### Database Backend Options

**Current**: Custom binary format
- ✅ Simple, no dependencies
- ❌ No transactional safety
- ❌ Corruption risk on interruption

**Option 1: BoltDB** (Recommended)
- ✅ Pure Go, no CGO
- ✅ ACID transactions
- ✅ Embedded (no separate server)
- ✅ Widely used (etcd, Consul)
- ✅ Similar to Berkeley DB
- Package: `go.etcd.io/bbolt`

**Option 2: BadgerDB**
- ✅ Very fast
- ✅ LSM-tree based
- ✅ Good for write-heavy workloads
- ❌ More complex
- Package: `github.com/dgraph-io/badger`

**Option 3: SQLite**
- ✅ SQL interface
- ✅ Well-tested
- ✅ Good tooling
- ❌ CGO dependency (or pure Go version slower)
- Package: `modernc.org/sqlite` or `github.com/mattn/go-sqlite3`

### Benefits of Build Tracking

- ✅ Track every build attempt (not just latest)
- ✅ Correlate failures with environment changes
- ✅ Reproducible builds (know exact deps used)
- ✅ Audit trail (who built what when)
- ✅ Statistics (build times, success rates, trends)
- ✅ Rollback (find last known good build)
- ✅ Debug failures (compare with previous successful builds)
- ✅ Performance analysis (which packages are slow?)

---

## API Design

### Architecture Overview

```
REST API + WebSocket for real-time updates
```

### API Structure

```go
type API struct {
    Builder  *builder.Builder
    Database *builddb.Database
    WS       *websocket.Hub  // For real-time updates
}
```

### REST Endpoints

#### 1. Build Control

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/builds` | Start new build |
| `GET` | `/api/v1/builds` | List builds (with filters) |
| `GET` | `/api/v1/builds/:id` | Get build details |
| `DELETE` | `/api/v1/builds/:id` | Cancel running build |
| `POST` | `/api/v1/builds/:id/retry` | Retry failed build |
| `POST` | `/api/v1/queue` | Add to build queue |
| `GET` | `/api/v1/queue` | Show build queue |
| `DELETE` | `/api/v1/queue/:id` | Remove from queue |

**Example Request:**
```bash
curl -X POST http://localhost:8080/api/v1/builds \
  -H "Content-Type: application/json" \
  -d '{
    "packages": ["editors/vim", "devel/git"],
    "profile": "LiveSystem",
    "force": false
  }'
```

**Example Response:**
```json
{
  "status": "success",
  "data": {
    "build_id": "550e8400-e29b-41d4-a716-446655440000",
    "queued_packages": 16,
    "estimated_duration": "15m30s"
  }
}
```

#### 2. Package Information

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/packages` | List all packages |
| `GET` | `/api/v1/packages/:portdir` | Get package info |
| `GET` | `/api/v1/packages/:portdir/deps` | Get dependencies |
| `GET` | `/api/v1/packages/:portdir/history` | Build history |
| `GET` | `/api/v1/packages/:portdir/status` | Current build status |

**Example:**
```bash
curl http://localhost:8080/api/v1/packages/editors/vim
```

```json
{
  "status": "success",
  "data": {
    "portdir": "editors/vim",
    "version": "9.1.1199",
    "pkgfile": "vim-9.1.1199.pkg",
    "last_build": {
      "status": "success",
      "timestamp": "2025-01-15T10:30:00Z",
      "duration": "5m23s"
    },
    "dependencies": {
      "build": ["devel/pkgconf", "devel/gettext-tools"],
      "run": ["devel/ncurses"]
    }
  }
}
```

#### 3. Workers

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/workers` | List workers |
| `GET` | `/api/v1/workers/:id` | Worker details |
| `POST` | `/api/v1/workers/:id/pause` | Pause worker |
| `POST` | `/api/v1/workers/:id/resume` | Resume worker |
| `GET` | `/api/v1/workers/:id/log` | Worker log stream |

**Example:**
```bash
curl http://localhost:8080/api/v1/workers
```

```json
{
  "status": "success",
  "data": {
    "workers": [
      {
        "id": 0,
        "status": "busy",
        "current_package": "editors/vim",
        "phase": "build",
        "started_at": "2025-01-15T10:25:00Z"
      },
      {
        "id": 1,
        "status": "idle"
      },
      {
        "id": 2,
        "status": "busy",
        "current_package": "devel/git",
        "phase": "configure",
        "started_at": "2025-01-15T10:28:00Z"
      }
    ],
    "total": 4,
    "busy": 2,
    "idle": 2
  }
}
```

#### 4. Status & Statistics

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/status` | Overall system status |
| `GET` | `/api/v1/stats` | Build statistics |
| `GET` | `/api/v1/stats/today` | Today's stats |
| `GET` | `/api/v1/stats/package/:portdir` | Per-package stats |

**Example:**
```bash
curl http://localhost:8080/api/v1/stats
```

```json
{
  "status": "success",
  "data": {
    "total_builds": 1523,
    "successful": 1487,
    "failed": 36,
    "success_rate": 97.6,
    "avg_build_time": "3m45s",
    "total_build_time": "95h12m",
    "packages_per_hour": 16.2,
    "busiest_worker": 2,
    "slowest_package": {
      "portdir": "lang/rust",
      "avg_time": "45m23s"
    }
  }
}
```

#### 5. Logs & Artifacts

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/logs/:build_id` | Get build log |
| `GET` | `/api/v1/logs/:build_id/stream` | Stream log (SSE) |
| `GET` | `/api/v1/artifacts/:build_id` | Download package |

**Example (Server-Sent Events):**
```bash
curl -N http://localhost:8080/api/v1/logs/abc-123/stream
```

```
data: {"line": "===>  Configuring for vim-9.1.1199", "timestamp": "2025-01-15T10:30:01Z"}

data: {"line": "checking for gcc... cc", "timestamp": "2025-01-15T10:30:02Z"}

data: {"line": "checking whether cc accepts -g... yes", "timestamp": "2025-01-15T10:30:03Z"}
```

#### 6. Configuration

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/config` | Get config |
| `PUT` | `/api/v1/config` | Update config |
| `GET` | `/api/v1/config/profiles` | List profiles |
| `POST` | `/api/v1/config/profiles` | Create profile |

#### 7. Database Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/database/stats` | Database stats |
| `POST` | `/api/v1/database/rebuild` | Rebuild CRC database |
| `POST` | `/api/v1/database/clean` | Clean stale entries |
| `GET` | `/api/v1/database/export` | Export database |

### WebSocket Events (Real-time)

**Connection:**
```
ws://localhost:8080/api/v1/events
```

**Event Types:**

#### Build Events
```javascript
// Build started
{
  "type": "build.started",
  "build_id": "550e8400-e29b-41d4-a716-446655440000",
  "package": "editors/vim",
  "worker": 2,
  "timestamp": "2025-01-15T10:30:00Z"
}

// Build progress
{
  "type": "build.progress",
  "build_id": "550e8400-e29b-41d4-a716-446655440000",
  "phase": "configure",
  "progress": 45,
  "timestamp": "2025-01-15T10:32:00Z"
}

// Build completed
{
  "type": "build.completed",
  "build_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "success",
  "duration": "5m23s",
  "timestamp": "2025-01-15T10:35:23Z"
}

// Build failed
{
  "type": "build.failed",
  "build_id": "550e8400-e29b-41d4-a716-446655440000",
  "phase": "build",
  "error": "compilation error in foo.c",
  "timestamp": "2025-01-15T10:33:45Z"
}
```

#### Worker Events
```javascript
{
  "type": "worker.status",
  "worker": 2,
  "status": "busy",
  "package": "editors/vim",
  "timestamp": "2025-01-15T10:30:00Z"
}

{
  "type": "worker.idle",
  "worker": 2,
  "timestamp": "2025-01-15T10:35:23Z"
}
```

#### Log Events
```javascript
{
  "type": "log.line",
  "build_id": "550e8400-e29b-41d4-a716-446655440000",
  "line": "checking for compiler...",
  "timestamp": "2025-01-15T10:30:05Z"
}
```

#### System Events
```javascript
{
  "type": "system.load",
  "load_avg": [2.5, 2.3, 2.1],
  "workers_busy": 3,
  "workers_total": 4,
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### API Response Format

**Standard Response:**
```json
{
  "status": "success",
  "data": {
    // Response data here
  },
  "meta": {
    "timestamp": "2025-01-15T10:32:45Z",
    "version": "1.0.0",
    "request_id": "req-abc-123"
  }
}
```

**Error Response:**
```json
{
  "status": "error",
  "error": {
    "code": "BUILD_NOT_FOUND",
    "message": "Build with ID abc-123 not found",
    "details": {}
  },
  "meta": {
    "timestamp": "2025-01-15T10:32:45Z",
    "version": "1.0.0",
    "request_id": "req-abc-123"
  }
}
```

### Authentication & Security

**Options:**
1. **API Keys** - Simple, good for CI/CD
2. **JWT Tokens** - Stateless, good for web UI
3. **mTLS** - Certificate-based, very secure
4. **OAuth2** - For third-party integrations

**Example with API Key:**
```bash
curl -H "X-API-Key: your-api-key-here" \
  http://localhost:8080/api/v1/builds
```

---

## Benefits & Use Cases

### Benefits of Library-First Design

1. **Reusability** - Core functionality can be embedded in other tools
2. **Testing** - Easy to mock and test components independently
3. **Multiple Interfaces** - CLI, API, Web UI all use same core
4. **Extensibility** - Easy to add new build environments or platforms
5. **Maintenance** - Clear separation of concerns
6. **Distribution** - Can be used as a Go module by other projects

### Use Cases with API

#### 1. Web UI Dashboard
- Real-time build monitoring
- Interactive build queue management
- Visual dependency graphs
- Historical statistics and charts
- Worker utilization graphs

#### 2. CI/CD Integration
```yaml
# GitHub Actions example
- name: Build packages
  run: |
    curl -X POST http://build-server:8080/api/v1/builds \
      -H "X-API-Key: ${{ secrets.DSYNTH_API_KEY }}" \
      -d '{"packages": ["editors/vim"], "profile": "Release"}'
```

#### 3. Distributed Build Farm
- Multiple machines coordinated via API
- Central controller distributing work
- Workers on different hosts
- Load balancing across machines

#### 4. Mobile App
- Monitor builds on phone
- Get push notifications for build failures
- Start/cancel builds remotely
- View logs on the go

#### 5. Chat Bot (Slack/Discord)
```
/build editors/vim
/status editors/vim
/logs abc-123
/workers
```

#### 6. Metrics & Monitoring
- Export to Prometheus
- Visualize in Grafana
- Track build times over time
- Alert on failed builds
- Capacity planning

#### 7. Build Reproducibility
- Record exact environment for each build
- Replay builds with same conditions
- Compare builds to find what changed
- Verify binary reproducibility

#### 8. Package Repository Management
- Automatic repository updates
- Package signing integration
- Mirror synchronization
- Cleanup old packages

### Development Workflow

**Phase 1: Library Extraction**
1. Extract `pkg` into pure library
2. Create `builddb` package
3. Abstract `builder` package
4. Define `environment` interface

**Phase 2: API Development**
1. Implement REST API
2. Add WebSocket support
3. Add authentication
4. Documentation (OpenAPI/Swagger)

**Phase 3: Advanced Features**
1. Distributed builds
2. Web UI
3. Metrics/monitoring
4. Advanced scheduling

**Phase 4: Platform Support**
1. FreeBSD jails
2. Linux containers
3. Cloud integration (AWS, GCP)

### Migration Path

**Backwards Compatibility:**
- Keep existing CLI interface working
- API is additive (doesn't break existing usage)
- Database migration tools for new format
- Gradual refactoring, no "big bang" rewrite

**Testing Strategy:**
- Unit tests for each package
- Integration tests for API
- End-to-end tests with actual builds
- Performance benchmarks
- Chaos testing (random failures, interruptions)

---

## Open Questions

1. **Database Backend**: Stay with custom binary or switch to BoltDB/SQLite?
2. **API Authentication**: Which method? API keys, JWT, or both?
3. **Platform Priority**: Focus on DragonFly/FreeBSD or add Linux support early?
4. **API Versioning**: How to handle breaking changes?
5. **WebSocket vs SSE**: Both or pick one for real-time updates?
6. **Distributed Builds**: Architecture for worker coordination?
7. **Package Repository**: Integrate with pkg repo tools or stay independent?

## Next Steps

- [ ] Review and refine architecture
- [ ] Prototype `builddb` package with BoltDB
- [ ] Design environment abstraction interface
- [ ] Create API specification (OpenAPI)
- [ ] Build proof-of-concept web UI
- [ ] Performance testing of database backends
- [ ] Security review of API design

---

**Document Status**: Draft  
**Last Updated**: 2025-01-15  
**Author**: Architecture Planning Session