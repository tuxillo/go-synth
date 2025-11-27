# go-dsynth Architecture – MVP Scope

This document defines the Minimum Viable Product (MVP) architecture extracted from the full `IDEAS.md`. It focuses on delivering a clean, testable, backward‑compatible core without advanced/distributed features.

## 1. Executive Summary (MVP)

Goals:
- Extract reusable core libraries (`pkg`, `builder`, `builddb`).
- Track builds with minimal persistent state (success/fail + timestamps + CRC).
- Provide environment abstraction (DragonFly/FreeBSD only in MVP).
- Retain existing CLI, thinly wrapping new library components.
- Optional minimal REST API (3 endpoints) for automation.

Non-Goals (Deferred):
- Distributed builds, web UI, dashboards, visualization.
- Advanced metrics, Prometheus/Grafana, alerting, health checks.
- Multiple auth methods (JWT, OAuth2, mTLS) – MVP uses API key only if API enabled.
- SSE/WebSocket log streaming (poll only or CLI logs).
- Advanced scheduling, load balancing, multi-host/cloud support.
- Chaos testing, large-scale performance benchmarking, cost modeling.
- Package signing/repository management automation (manual or future add-on).

## 2. Current Pain Points (Condensed)
- Mixed concerns in `pkg` (metadata, CRC, status flags, dependency resolution).
- Tight coupling between `build` logic and mount/platform specifics.
- CRC database lacks history; cannot audit build outcomes.

## 3. Target MVP Structure
```
dsynth/
├── pkg/          # Pure: metadata + dependency resolution
├── builddb/      # Minimal build tracking (bbolt)
├── builder/      # Orchestration + worker loop + phases
├── environment/  # DragonFly/FreeBSD impl only (interface)
├── cmd/dsynth/   # Thin CLI wrapper
└── api/ (optional) # Minimal REST (if enabled)
```

## 4. Core Packages & Interfaces (Essential Snippets)

### pkg
Responsible for:
- Parsing port specifications.
- Building dependency graph.
- Providing ordered build list (topological sort).

```go
type Package struct {
    PortDir  string
    Name     string
    Version  string
    Flavor   string
    PkgFile  string
    Deps     []*Package // resolved dependencies
}

func Parse(portSpecs []string, cfg *Config) ([]*Package, error)
func Resolve(pkgs []*Package, cfg *Config) error
func TopoOrder(pkgs []*Package) []*Package
```

### builddb (Minimal)
Tracks only what is needed to skip unchanged builds and display basic history.

Schema (bbolt buckets):
- `builds`: `uuid -> {portdir, version, status, start_time, end_time}`
- `packages`: `portdir@version -> uuid` (latest successful)
- `crc_index`: `portdir -> crc32`

```go
type BuildRecord struct {
    UUID      string
    PortDir   string
    Version   string
    Status    string // success|failed|running
    StartTime time.Time
    EndTime   time.Time
}

func SaveRecord(rec *BuildRecord) error
func GetRecord(uuid string) (*BuildRecord, error)
func LatestFor(portDir, version string) (*BuildRecord, error)
func NeedsBuild(portDir string, crc uint32) bool
func UpdateCRC(portDir string, crc uint32) error
```

### environment
Single implementation (DragonFly/FreeBSD). Interface kept small.
```go
type Environment interface {
    Setup(workerID int, cfg *Config) error
    Execute(port *pkg.Package, phase string) error
    Cleanup() error
}
```

### builder
Manages worker goroutines and phase execution.
```go
type BuildStats struct { Total, Success, Failed, Skipped int; Duration time.Duration }

type Builder struct {
    Env      Environment
    DB       *builddb.DB
    Workers  int
}

func (b *Builder) Run(pkgs []*pkg.Package) (*BuildStats, error)
```

## 5. Build Phases (MVP Subset)
Implement only essential phases to produce a package:
1. fetch (distfiles)
2. checksum
3. extract
4. patch
5. build (compile)
6. stage
7. package

Deferred phases (run‑depends, lib‑depends refinement, check-plist, verify, purge-distfiles) are future add-ons.

## 6. Minimal API (Optional)
If enabled via build tag or config flag:
- `POST /api/v1/builds` → start build: body `{"packages":["editors/vim"]}` returns `{build_id}`.
- `GET /api/v1/builds/:id` → status `{status, success, failed, elapsed}`.
- `GET /api/v1/builds` → list recent builds (pagination).
Auth: Static `X-API-Key` header compared against config value.
No queue, worker, log streaming, metrics, or stats endpoints.

## 7. Testing Plan (MVP)
- Unit: pkg (dependency resolution), builddb (CRUD + NeedsBuild), builder (single worker happy path).
- Integration: Start build for 1–3 packages → verify statuses persisted.
- Skip chaos/performance benchmarks; manually measure build time for sanity.

## 8. Migration Strategy (Minimal)
1. Introduce bbolt (`go.etcd.io/bbolt`) alongside existing custom CRC file.
2. Populate `crc_index` on first run by scanning ports selected for build.
3. Migrate latest successful builds opportunistically when packages finish.
4. Allow fallback to custom CRC file if DB unavailable (temporary shim).

## 9. Future Work (Deferred Backlog)
- Distributed builds & multi-host controller.
- Web UI, WebSocket/SSE streaming, log viewer.
- Prometheus metrics, alerting, health checks.
- Advanced scheduling (resource-aware, priority queue).
- Extended build history (full dependency versions, environment fingerprint).
- Multi-auth (JWT/mTLS/OAuth2), rate limiting.
- Linux container & cloud orchestration backends.
- Package signing & repository automation.
- Chaos/performance benchmarking suite.

## 10. Accepted Decisions (MVP)
- **Database Backend**: bbolt (`go.etcd.io/bbolt`) – maintained BoltDB fork (ADR-001).
- **Environment Abstraction**: Interface with single initial implementation (ADR-003 simplified).
- **Authentication**: Single API key (only if API enabled) – ADR-002 deferred.

## 11. Non-Goals Summary
Explicitly NOT delivering in MVP: distribution, metrics, UI, streaming, multi-auth, advanced phases, cloud, cost modeling, chaos tests.

## 12. Success Criteria (MVP)
- Build N specified ports with dependencies without manual intervention.
- Skip unchanged ports using CRC diff logic.
- Persist basic build outcome records.
- Core library usable via CLI and optionally API.
- Unit + basic integration tests pass in CI.

---
**Status**: MVP Definition Draft  
**Generated**: 2025-11-25  
**Source**: Reduced from full IDEAS.md
