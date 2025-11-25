# Development Guide

This document provides an overview of the development process, phase tracking, and contribution guidelines for the go-synth project.

## Quick Links

- **[Agent Guide](docs/design/AGENTS.md)** - Essential information for developers and AI agents
- **[Architecture & Ideas](docs/design/IDEAS.md)** - Comprehensive architectural vision
- **[MVP Scope](docs/design/IDEAS_MVP.md)** - Minimum Viable Product definition

## Development Philosophy

This project follows a **phased development approach** where each phase builds upon the previous one. Each phase has:
- Clear goals and scope
- Well-defined exit criteria
- Minimal dependencies on future work
- Comprehensive documentation

The goal is to maintain a working, compilable codebase at every step while progressively refactoring toward a clean, modular architecture.

---

## Phase Tracking

### Phase Status Legend
- 🟢 **Complete** - All exit criteria met, ready for next phase
- 🟡 **In Progress** - Active development, some criteria met
- 🔵 **Ready** - Previous phase complete, can be started
- ⚪ **Planned** - Documented, waiting for dependencies
- 📋 **Design** - Requirements gathering, not started

---

## Phase 1: Library Extraction (pkg) 🟡

**Status**: 🟡 In Progress (43% complete)  
**Timeline**: Started 2025-11-21 | Target: TBD  
**Owner**: Core Team

### Goals
- Isolate package metadata and dependency resolution into a pure library
- Provide stable API for parsing port specs and generating build order
- Remove mixed concerns (build state, CRC tracking) from pkg package

### Main Deliverables
- ✅ Core API functions: `Parse()`, `Resolve()`, `TopoOrder()`
- ✅ Cycle detection with `TopoOrderStrict()`
- ✅ Basic unit tests (happy paths)
- 🔄 Pure metadata-only Package struct (in progress)
- 🔄 Separated CRC database (builddb package created)
- ❌ Structured error types
- ❌ Comprehensive documentation

### Exit Criteria
- ✅ TopoOrder returns correct, cycle-free ordering
- ✅ All existing commands compile and run
- ✅ CRC/build tracking separated into builddb package
- ❌ Package struct contains ONLY metadata (no build state)
- ❌ No global state in pkg package
- ❌ Structured errors for all failure modes
- ❌ Comprehensive godoc documentation

### Current Status (3/7 criteria met)

**Completed Work:**
- Parse, Resolve, TopoOrder implementation with Kahn's algorithm
- Parallel bulk fetching of package metadata
- Recursive dependency resolution (all 6 types)
- Bidirectional dependency graph construction
- Cycle detection tests
- CRC database extracted to `builddb/` package

**In Progress:**
- Separating build state from Package struct
- Adding structured error types
- Removing global state

**Remaining Work:**
- Complete build state separation (~4-6h)
- Add structured error types (~1-2h)
- Remove global registry (~2-3h)
- Add comprehensive godoc (~3-4h)
- Write developer guide (~2-3h)
- Add integration tests (~3-4h)

### Documentation
- **[Phase 1 Overview](docs/design/PHASE_1_LIBRARY.md)** - Complete status and analysis
- **[Phase 1 TODO](docs/design/PHASE_1_TODO.md)** - Detailed task breakdown (12 tasks, ~25-35h)
- **[Phase 1 Analysis](docs/design/PHASE_1_ANALYSIS_SUMMARY.md)** - Findings and recommendations

### Key Decisions
- Use linked list for package traversal (preserve original dsynth design)
- Kahn's algorithm for topological sorting
- Separate builddb package for CRC tracking (prepare for BoltDB in Phase 2)
- Wrapper functions maintain compatibility with existing code

### Blockers
None - all dependencies resolved

---

## Phase 2: Minimal BuildDB (BoltDB) ⚪

**Status**: ⚪ Planned  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phase 1 completion

### Goals
- Add persistent tracking of build attempts and CRCs using BoltDB
- Enable incremental builds by skipping unchanged ports
- Replace file-based CRC database with structured database

### Main Deliverables
- BoltDB schema with three buckets: `builds`, `packages`, `crc_index`
- BuildRecord API for CRUD operations
- NeedsBuild() function using CRC comparison
- Migration from file-based CRC to BoltDB

### Exit Criteria
- `NeedsBuild()` returns false when CRC unchanged
- Successful build writes records to all three buckets
- Migration handles existing CRC file data
- Unit tests for all CRUD operations

### Proposed API
```go
type BuildRecord struct {
    UUID      string
    PortDir   string
    Version   string
    Status    string // running|success|failed
    StartTime time.Time
    EndTime   time.Time
}

func SaveRecord(rec *BuildRecord) error
func GetRecord(uuid string) (*BuildRecord, error)
func LatestFor(portDir, version string) (*BuildRecord, error)
func NeedsBuild(portDir string, crc uint32) bool
func UpdateCRC(portDir string, crc uint32) error
```

### Documentation
- **[Phase 2 Plan](docs/design/PHASE_2_BUILDDB.md)** - Complete specification

### Key Decisions
- BoltDB chosen for embedded, ACID-compliant storage
- Package keys use `portdir@version` format
- Lazy migration: populate database on first successful build
- Keep file-based CRC as fallback during transition

---

## Phase 3: Builder Orchestration ⚪

**Status**: ⚪ Planned  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phases 1-2 completion

### Goals
- Implement worker pool for parallel build execution
- Execute essential build phases in correct order
- Integrate with pkg (build order) and builddb (tracking)

### Main Deliverables
- Worker pool with configurable concurrency
- Queue-based task distribution respecting topological order
- Build phase execution (7 MVP phases: fetch, checksum, extract, patch, build, stage, package)
- Error propagation to dependent packages
- Build statistics tracking

### Exit Criteria
- Builds small set of ports with correct parallelism
- Dependent packages skip when dependency fails
- Statistics accurately reflect success/failed/skipped counts
- CRC skip mechanism works across builds

### Proposed API
```go
type BuildStats struct { 
    Total, Success, Failed, Skipped int
    Duration time.Duration 
}

type Builder struct {
    Env      environment.Environment
    DB       *builddb.DB
    Workers  int
}

func (b *Builder) Run(pkgs []*pkg.Package) (*BuildStats, error)
```

### Documentation
- **[Phase 3 Plan](docs/design/PHASE_3_BUILDER.md)** - Complete specification

### Key Decisions
- Channel-based worker queue
- Topological order ensures dependencies build first
- Graceful cleanup via defer/cleanup hooks
- Package-level phases (not port-level) for MVP

---

## Phase 4: Environment Abstraction ⚪

**Status**: ⚪ Planned  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phase 3 completion

### Goals
- Define minimal environment interface for build isolation
- Implement FreeBSD/DragonFly backend using existing dsynth conventions
- Provide phase execution with chroot isolation

### Main Deliverables
- Environment interface with Setup/Execute/Cleanup methods
- DragonFly/FreeBSD implementation using nullfs/tmpfs + chroot
- Mount management and cleanup (even on failure)
- Integration with Builder from Phase 3

### Exit Criteria
- Each phase runs in isolated chroot environment
- Successful execution returns clean status
- Failed execution cleans up mounts properly
- Root privilege validation fails early

### Proposed API
```go
type Environment interface {
    Setup(workerID int, cfg *config.Config) error
    Execute(port *pkg.Package, phase string) error
    Cleanup() error
}
```

### Documentation
- **[Phase 4 Plan](docs/design/PHASE_4_ENVIRONMENT.md)** - Complete specification

### Key Decisions
- Use existing nullfs/tmpfs + chroot (proven by original dsynth)
- Map ports tree to `/xports` in chroot
- Signal trapping for cleanup on interruption
- Requires root - validate early and fail fast

---

## Phase 5: Minimal REST API ⚪

**Status**: ⚪ Planned (Optional)  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phases 1-3 completion

### Goals
- Provide simple HTTP API for build automation
- Enable remote build triggering and status queries
- Basic authentication with API keys

### Main Deliverables
- Three REST endpoints: POST /builds, GET /builds/:id, GET /builds
- API key authentication middleware
- JSON request/response formats
- Integration with Builder and BuildDB

### Exit Criteria
- Can start build via POST /builds
- Can poll build status via GET /builds/:id
- Authentication rejects invalid API keys
- Integration tests cover happy path

### Proposed Endpoints
```
POST /api/v1/builds
  Body: { "packages": ["editors/vim"], "profile": "default" }
  Response: { "build_id": "uuid" }

GET /api/v1/builds/:id
  Response: { "status": "running|success|failed", "start_time": "...", ... }

GET /api/v1/builds
  Response: { "items": [...], "next": "cursor" }
```

### Documentation
- **[Phase 5 Plan](docs/design/PHASE_5_MIN_API.md)** - Complete specification

### Key Decisions
- Polling-based (no WebSocket/SSE for MVP)
- Simple router, minimal dependencies
- Optional phase - can be deferred if not needed

---

## Phase 6: Testing Strategy ⚪

**Status**: ⚪ Planned  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phases 1-3 completion

### Goals
- Ensure reliability of core libraries
- Validate end-to-end build flow
- Set up continuous integration

### Main Deliverables
- Comprehensive unit tests for pkg, builddb, builder
- Integration tests for full build pipeline
- Test fixtures (minimal test ports or mocks)
- CI configuration (GitHub Actions or similar)

### Exit Criteria
- All packages have >80% test coverage
- Integration test successfully builds 1-3 small ports
- CI runs on every PR with race detector
- Failure tests validate error propagation

### Test Coverage
- **pkg**: Parse, Resolve, TopoOrder, cycle detection
- **builddb**: CRUD operations, NeedsBuild, CRC updates
- **builder**: Worker lifecycle, failure propagation, stats
- **Integration**: End-to-end build with real ports

### Documentation
- **[Phase 6 Plan](docs/design/PHASE_6_TESTING.md)** - Complete specification

### Key Decisions
- Use standard `go test` with race detector
- Minimal test ports or mocked execution
- Out of scope: benchmarks, chaos testing (defer to post-MVP)

---

## Phase 7: Integration & Migration ⚪

**Status**: ⚪ Planned  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phases 1-6 completion

### Goals
- Wire all new components into existing CLI
- Provide migration path from legacy CRC to BuildDB
- Maintain backward compatibility during transition

### Main Deliverables
- Updated CLI commands using new pipeline
- BuildDB initialization with fallback to legacy CRC
- Migration tooling for existing installations
- Updated logging with UUID tracking

### Exit Criteria
- End-to-end build via CLI works correctly
- CRC skip validated across two consecutive runs
- Migration from file-based CRC completes successfully
- All existing CLI commands remain functional

### CLI Mapping
- `dsynth build [ports...]` → uses new pipeline
- `dsynth force` → bypasses NeedsBuild check
- `dsynth upgrade-system` → uses pkg for installed packages
- Legacy commands continue to work

### Documentation
- **[Phase 7 Plan](docs/design/PHASE_7_INTEGRATION.md)** - Complete specification

### Key Decisions
- BuildDB primary, legacy CRC fallback
- Lazy migration: populate on successful builds
- Keep existing log file structure
- Add UUID to log messages for traceability

---

## Contributing Workflow

### For New Contributors

1. **Read Essential Docs**
   - [AGENTS.md](docs/design/AGENTS.md) - Development workflow and commit guidelines
   - [Phase 1 TODO](docs/design/PHASE_1_TODO.md) - Current task list

2. **Pick a Task**
   - Check Phase 1 TODO for available tasks
   - Start with tasks marked "High Priority" or "Easy"
   - Comment on GitHub issue or create one if none exists

3. **Development Cycle**
   - Create feature branch: `git checkout -b feature/task-name`
   - Make changes with tests
   - Commit locally following guidelines in AGENTS.md
   - DO NOT push to remote until feature is complete
   - Run tests: `go test -v -race ./...`

4. **Submit Changes**
   - Push feature branch when complete
   - Create pull request with clear description
   - Reference related issues/tasks
   - Wait for review

### Commit Guidelines

Follow the commit style from AGENTS.md:

```
Component: brief summary (50 chars or less)

- Detailed explanation of what changed
- Why the change was necessary
- Any important side effects or considerations

Rationale: Explain the "why" behind the change
```

Examples:
```
Phase 1 Task 3: Add structured error types

- Create pkg/errors.go with ErrCycleDetected, ErrInvalidSpec
- Update TopoOrderStrict to return ErrCycleDetected
- Update Parse to return ErrInvalidSpec for malformed specs

Rationale: Structured errors enable better error handling and testing

---

pkg: separate build state from Package struct

- Create pkg/buildstate.go with BuildState struct
- Remove Flags, IgnoreReason, LastPhase from Package
- Update all callers to use BuildState registry

Rationale: Package should contain only metadata, not build-time state
```

### Testing Requirements

- Add unit tests for new functions
- Maintain or improve code coverage
- Run `go test -v -race ./...` before committing
- Add integration tests for end-to-end flows

### Documentation Requirements

- Add godoc comments for exported functions
- Update relevant phase documentation
- Update README.md if public API changes
- Update DEVELOPMENT.md if phase status changes

---

## Project Status Summary

### Overall Progress
- **Phase 1**: 🟡 43% complete (3/7 exit criteria met)
- **Phase 2-7**: ⚪ Planned (waiting for Phase 1)
- **Total Estimated Remaining**: ~50-70 hours across all phases

### Recent Milestones
- ✅ 2025-11-25: CRC database extracted to builddb package (Task 2)
- ✅ 2025-11-25: Phase 1 comprehensive analysis and TODO created
- ✅ 2025-11-21: Core pkg API implemented (Parse, Resolve, TopoOrder)
- ✅ 2025-11-21: Cycle detection implemented and tested

### Next Milestones
- 🎯 Task 3: Add structured error types (~1-2h)
- 🎯 Task 1: Separate build state from Package (~4-6h)
- 🎯 Task 4: Remove global state (~2-3h)
- 🎯 Phase 1 completion (7/7 exit criteria met)

### Known Issues
See [Phase 1 TODO](docs/design/PHASE_1_TODO.md) for complete list.

**Critical:**
- Package struct still contains build state (Flags, IgnoreReason, LastPhase)
- Global state (globalRegistry) not yet removed
- No structured error types

**Medium:**
- Incomplete godoc documentation
- Missing integration tests
- No developer guide yet

**Low:**
- No context.Context support
- BulkQueue implementation exposed
- No benchmark tests

---

## Future Plans

See [FUTURE_BACKLOG.md](docs/design/FUTURE_BACKLOG.md) for features deferred beyond Phase 7:

- ncurses UI (like original dsynth)
- Profile switching
- Hook system for custom actions
- Advanced NUMA support
- Remote builder support
- Package signing
- Distributed builds

---

## Getting Help

- **Issues**: Check existing GitHub issues or create new ones
- **Discussions**: Use GitHub Discussions for design questions
- **Documentation**: All docs in `docs/design/` directory
- **Contact**: [Project maintainers contact info]

---

**Last Updated**: 2025-11-25  
**Document Version**: 1.0
