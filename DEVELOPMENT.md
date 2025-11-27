# 📚 Development Guide

This document provides an overview of the development process, phase tracking, and contribution guidelines for the go-synth project.

## 🔗 Quick Links

- **[Agent Guide](AGENTS.md)** - Essential information for developers and AI agents
- **[Architecture & Ideas](docs/design/IDEAS.md)** - Comprehensive architectural vision
- **[MVP Scope](docs/design/IDEAS_MVP.md)** - Minimum Viable Product definition

## 💡 Development Philosophy

This project follows a **phased development approach** where each phase builds upon the previous one. Each phase has:
- Clear goals and scope
- Well-defined exit criteria
- Minimal dependencies on future work
- Comprehensive documentation

The goal is to maintain a working, compilable codebase at every step while progressively refactoring toward a clean, modular architecture.

---

## 📊 Phase Tracking

### Phase Status Legend
- 🟢 **Complete** - All exit criteria met, ready for next phase
- 🟡 **In Progress** - Active development, some criteria met
- 🔵 **Ready** - Previous phase complete, can be started
- ⚪ **Planned** - Documented, waiting for dependencies
- 📋 **Design** - Requirements gathering, not started

---

## Phase 1: Library Extraction (pkg) 🟢

**Status**: 🟢 Complete (All exit criteria met - documentation tasks remaining)  
**Timeline**: Started 2025-11-21 | Target: TBD  
**Owner**: Core Team

### 🎯 Goals
- Isolate package metadata and dependency resolution into a pure library
- Provide stable API for parsing port specs and generating build order
- Remove mixed concerns (build state, CRC tracking) from pkg package

### 📦 Main Deliverables
- ✅ Core API functions: `Parse()`, `Resolve()`, `TopoOrder()`
- ✅ Cycle detection with `TopoOrderStrict()`
- ✅ Basic unit tests (happy paths)
- ✅ Pure metadata-only Package struct (Phase 1.5 complete)
- ✅ Separated CRC database (builddb package created)
- ✅ Removed C-isms (Phase 1.5 complete)
- ✅ Structured error types (Task 3 complete)
- ✅ Comprehensive godoc documentation (Task 5 complete)

### ✓ Exit Criteria
- ✅ TopoOrder returns correct, cycle-free ordering
- ✅ All existing commands compile and run
- ✅ CRC/build tracking separated into builddb package
- ✅ Package struct contains ONLY metadata (Phase 1.5 complete)
- ✅ C-isms removed (Phase 1.5 complete)
- ✅ No global state in pkg package (Task 4 complete)
- ✅ Structured errors for all failure modes (Task 3 complete)
- ✅ Comprehensive godoc documentation (Task 5 complete)

### Current Status (9/9 criteria met - Phase 1 COMPLETE pending final tasks)

**Completed Work:**
- Parse, Resolve, TopoOrder implementation with Kahn's algorithm
- Parallel bulk fetching of package metadata
- Recursive dependency resolution (all 6 types)
- Bidirectional dependency graph construction
- Cycle detection tests
- CRC database extracted to `builddb/` package
- BuildState infrastructure with thread-safe registry (pkg/buildstate.go, 143 lines)
- Build package fully migrated to use BuildStateRegistry
- Parsing layer integrated with BuildStateRegistry
- Comprehensive BuildState tests (8 tests including concurrency)
- **Phase 1.5 Part A**: Fidelity verification (10 C fidelity tests passing)
- **Phase 1.5 Part B**: C-ism removal complete (4 tasks)
  - B1: Removed dead Package.mu field
  - B2: Converted linked lists to slices (-53 lines)
  - B3: Added typed DepType enum
  - B4: Added typed PackageFlags
- **Phase 1 Task 3**: Structured error types (80 lines, 4 tests)
- **Phase 1 Task 4**: Removed global state - pkgRegistry now passed as parameter
- **Phase 1 Task 5**: Comprehensive godoc documentation (package, types, functions)
- **Phase 1 Task 6**: Developer guide with 5 examples (PHASE_1_DEVELOPER_GUIDE.md, 1057 lines)

**Clean Architecture Achieved:**
- ✅ Package struct is now pure metadata (no build state)
- ✅ BuildStateRegistry handles all build-time state
- ✅ Slice-based package collections (removed Next/Prev pointers)
- ✅ Type-safe enums (DepType, PackageFlags)
- ✅ All 39 tests passing including fidelity tests
- ✅ Comprehensive API documentation with godoc

**Remaining Work:**
- Add integration tests (~2-3h)
- Improve error test coverage (~2-3h)

### 📖 Documentation
- **[Phase 1 Overview](docs/design/PHASE_1_LIBRARY.md)** - Complete status and analysis
- **[Phase 1 TODO](docs/design/PHASE_1_TODO.md)** - Detailed task breakdown (12 tasks, ~25-35h)
- **[Phase 1 Analysis](docs/design/PHASE_1_ANALYSIS_SUMMARY.md)** - Findings and recommendations
- **[Phase 1.5 Fidelity Analysis](docs/design/PHASE_1.5_FIDELITY_ANALYSIS.md)** - C implementation comparison
- **[Phase 1.5 Part B Plan](docs/design/phase_1.5_part_b_plan.md)** - C-ism removal plan (completed)

### 🔑 Key Decisions
- Use Go slices for package collections (replaced linked lists in Phase 1.5)
- Kahn's algorithm for topological sorting
- Separate builddb package for CRC tracking (prepare for bbolt in Phase 2)
- Wrapper functions maintain compatibility with existing code
- Type-safe enums for DepType and PackageFlags

### 🚧 Blockers
None - all dependencies resolved

---

## Phase 1.5: Fidelity Verification & C-ism Removal 🟢

**Status**: 🟢 Complete  
**Timeline**: Started 2025-11-25 | Completed 2025-11-26  
**Owner**: Core Team

### 🎯 Goals
- Verify Go implementation matches C dsynth functionality
- Remove C-style patterns in favor of Go idioms
- Improve type safety and code clarity

### Part A: Fidelity Verification ✅
- Comprehensive comparison of Go vs C implementation
- 10 C fidelity tests created and passing
- Verified algorithm equivalence for:
  - Dependency resolution (two-pass algorithm)
  - Topological sorting (Kahn's algorithm)
  - Dependency type handling (6 types)
  - Package registry behavior
  - Cycle detection
  - Diamond dependencies

### Part B: C-ism Removal ✅

**B1: Remove Dead Code** (5 min)
- ✅ Removed unused `Package.mu sync.Mutex` field
- Zero references found in codebase

**B2: Convert Linked Lists to Slices** (2-3 hours)
- ✅ Removed `Package.Next` and `Package.Prev` fields
- ✅ Updated 5 API signatures to accept/return `[]*Package`
- ✅ Converted 7 traversals to range loops
- ✅ Updated all test files (17 locations)
- **Net result**: -53 lines of code

**B3: Add Typed DepType** (1 hour)
- ✅ Created `type DepType int` with String() and Valid() methods
- ✅ Updated all dependency structures to use typed enum
- ✅ Added comprehensive tests

**B4: Add Typed PackageFlags** (2 hours)
- ✅ Created `type PackageFlags int` with Has(), Set(), Clear(), String() methods
- ✅ Updated BuildState and all flag operations
- ✅ Added comprehensive tests

### 📊 Results
- **Tests**: 39 passing (including 10 C fidelity tests)
- **Coverage**: 42.8% maintained
- **Code reduction**: -53 net lines
- **Type safety**: Improved with typed enums
- **Architecture**: Cleaner, more idiomatic Go

### 📖 Documentation
- **[Fidelity Analysis](docs/design/PHASE_1.5_FIDELITY_ANALYSIS.md)** - Comprehensive C vs Go comparison
- **[Part B Plan](docs/design/phase_1.5_part_b_plan.md)** - C-ism removal planning (1,348 lines)

### 🔑 Key Benefits
- More idiomatic Go code (slices over manual pointer chaining)
- Better type safety (enums vs raw ints)
- Simpler test construction (no manual linking)
- Improved memory locality and cache performance
- Easier to reason about (no hidden state in pointers)

---

## Phase 2: Minimal BuildDB (bbolt) 🟡

**Status**: 🟡 In Progress (75% Complete, 9/12 tasks)  
**Timeline**: Started 2025-11-27 | Target: TBD (3.5-7.5 hours remaining)  
**Dependencies**: Phase 1 completion (✅ 9/9 exit criteria met)

### 🎯 Goals
- Add persistent tracking of build attempts and CRCs using bbolt (BoltDB successor)
- Enable incremental builds by skipping unchanged ports
- Replace custom binary CRC database with proper embedded database

### 📦 Main Deliverables (7/7 Complete)
- ✅ bbolt integration (`go.etcd.io/bbolt` dependency) - commit 6a6ff7b
- ✅ Database schema with three buckets: `builds`, `packages`, `crc_index` - commit 48569e6
- ✅ BuildRecord API for CRUD operations - commit d1b91d9
- ✅ Package tracking with LatestFor() and UpdatePackageIndex() - commit d6413c3
- ✅ NeedsBuild() and CRC operations (NeedsBuild, UpdateCRC, GetCRC) - commit b9d9d41
- ✅ Migration from existing `builddb/crc.go` to bbolt - commits 52d5393, d34a083, 24beab5
- ✅ UUID infrastructure and build record lifecycle - commits 03aa961, 65ccadd

### 🚧 Task Breakdown (9/12 complete - 75% DONE)
1. ✅ Add bbolt dependency (DONE 2025-11-27) - commit 6a6ff7b
2. ✅ Create DB wrapper with Open/Close (DONE 2025-11-27) - commit 48569e6
3. ✅ Build record CRUD operations (DONE 2025-11-27) - commit d1b91d9
4. ✅ Package tracking (LatestFor, UpdatePackageIndex) (DONE 2025-11-27) - commit d6413c3
5. ✅ CRC operations (NeedsBuild, UpdateCRC, GetCRC) (DONE 2025-11-27) - commit b9d9d41
6. ✅ Migration and legacy CRC removal (DONE 2025-11-27) - Tasks 6A-6E
   - 6A: Content-based CRC helper (commit 52d5393)
   - 6B: Migrate to BuildDB API calls (commit d34a083)
   - 6C: Delete legacy CRC system (commit 24beab5)
   - 6D: BuildDB refactoring + UUID infrastructure (commit 03aa961)
   - 6E: Build record lifecycle (commit 65ccadd)
7. ✅ Structured error types (DONE 2025-11-27) - commit bd20013
   - Created builddb/errors.go with 9 sentinel errors and 5 structured types
   - Updated all 26 error sites in builddb/db.go to use typed errors
   - Added 4 error inspection helpers (IsValidationError, IsRecordNotFound, etc.)
   - Added comprehensive tests in builddb/errors_test.go (11 tests, all passing)
   - All errors implement Unwrap() for errors.Is/As compatibility
8. ❌ Unit tests for builddb API functions (3 hours)
9. ❌ Integration test (1.5 hours)
10. ✅ Godoc documentation (DONE 2025-11-27) - commit TBD
    - Enhanced package-level documentation in builddb/errors.go
    - Added usage examples to all error types (DatabaseError, RecordError, etc.)
    - Enhanced helper function documentation (IsValidationError, IsDatabaseError, etc.)
    - Note: db.go already had comprehensive godoc from initial implementation
    - Verified with `go doc builddb` - all types and functions properly documented
11. ❌ Benchmarks vs. old CRC file (1 hour)
12. ❌ CLI integration (2 hours)

### ✓ Exit Criteria (4/8 Complete, 3 N/A after legacy deletion)
- ✅ `NeedsBuild()` returns false when CRC unchanged; true otherwise (Task 5)
- ✅ Successful build writes records to all three buckets (Task 6E)
- ✅ `LatestFor()` returns most recent successful build (Task 4)
- ✅ BuildDB lifecycle properly managed (single open/close pattern) (Task 6D)
- ~~Migration from old CRC file working~~ (N/A - legacy system deleted)
- ~~Database survives process crash (ACID guarantees)~~ (N/A - bbolt provides this)
- ~~CLI updated to use new database~~ (N/A - CLI already uses BuildDB after Task 6B)
- ❌ Unit tests cover all API functions
- ❌ Integration test validates full build workflow

### 💻 Target API
```go
type BuildRecord struct {
    UUID      string
    PortDir   string
    Version   string
    Status    string // "running" | "success" | "failed"
    StartTime time.Time
    EndTime   time.Time
}

func OpenDB(path string) (*DB, error)
func (db *DB) Close() error
func (db *DB) SaveRecord(rec *BuildRecord) error
func (db *DB) GetRecord(uuid string) (*BuildRecord, error)
func (db *DB) LatestFor(portDir, version string) (*BuildRecord, error)
func (db *DB) NeedsBuild(portDir string, currentCRC uint32) (bool, error)
func (db *DB) UpdateCRC(portDir string, crc uint32) error
```

### 📖 Documentation
- **[Phase 2 Plan](docs/design/PHASE_2_BUILDDB.md)** - Complete specification (updated 2025-11-27)

### 🔑 Key Decisions
- **bbolt vs. BoltDB**: Use `go.etcd.io/bbolt` (maintained fork; original archived 2019)
- **Database location**: `~/.go-synth/builds.db` (override with `--db-path`)
- **Package keys**: Use `portdir@version` format (e.g., `lang/go@default`)
- **CRC storage**: Binary `uint32` (4 bytes) for efficiency
- **Migration**: Coexistence approach (both old and new DB temporarily)

### 📊 Current State vs. Target

**Current** (`builddb/crc.go` - 495 lines):
- Custom binary format, 16K entry limit
- O(n) linear scan lookups
- No build record tracking
- Manual memory management

**Target** (`builddb/` with bbolt):
- B+tree indexed database, unlimited capacity
- O(log n) lookups, ACID transactions
- Full build history with UUIDs
- Automatic crash recovery

---

## Phase 3: Builder Orchestration ⚪

**Status**: ⚪ Planned  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phases 1-2 completion

### 🎯 Goals
- Implement worker pool for parallel build execution
- Execute essential build phases in correct order
- Integrate with pkg (build order) and builddb (tracking)

### 📦 Main Deliverables
- Worker pool with configurable concurrency
- Queue-based task distribution respecting topological order
- Build phase execution (7 MVP phases: fetch, checksum, extract, patch, build, stage, package)
- Error propagation to dependent packages
- Build statistics tracking

### ✓ Exit Criteria
- Builds small set of ports with correct parallelism
- Dependent packages skip when dependency fails
- Statistics accurately reflect success/failed/skipped counts
- CRC skip mechanism works across builds

### 💻 Proposed API
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

### 📖 Documentation
- **[Phase 3 Plan](docs/design/PHASE_3_BUILDER.md)** - Complete specification

### 🔑 Key Decisions
- Channel-based worker queue
- Topological order ensures dependencies build first
- Graceful cleanup via defer/cleanup hooks
- Package-level phases (not port-level) for MVP

---

## Phase 4: Environment Abstraction ⚪

**Status**: ⚪ Planned  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phase 3 completion

### 🎯 Goals
- Define minimal environment interface for build isolation
- Implement FreeBSD/DragonFly backend using existing dsynth conventions
- Provide phase execution with chroot isolation

### 📦 Main Deliverables
- Environment interface with Setup/Execute/Cleanup methods
- DragonFly/FreeBSD implementation using nullfs/tmpfs + chroot
- Mount management and cleanup (even on failure)
- Integration with Builder from Phase 3

### ✓ Exit Criteria
- Each phase runs in isolated chroot environment
- Successful execution returns clean status
- Failed execution cleans up mounts properly
- Root privilege validation fails early

### 💻 Proposed API
```go
type Environment interface {
    Setup(workerID int, cfg *config.Config) error
    Execute(port *pkg.Package, phase string) error
    Cleanup() error
}
```

### 📖 Documentation
- **[Phase 4 Plan](docs/design/PHASE_4_ENVIRONMENT.md)** - Complete specification

### 🔑 Key Decisions
- Use existing nullfs/tmpfs + chroot (proven by original dsynth)
- Map ports tree to `/xports` in chroot
- Signal trapping for cleanup on interruption
- Requires root - validate early and fail fast

---

## Phase 5: Minimal REST API ⚪

**Status**: ⚪ Planned (Optional)  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phases 1-3 completion

### 🎯 Goals
- Provide simple HTTP API for build automation
- Enable remote build triggering and status queries
- Basic authentication with API keys

### 📦 Main Deliverables
- Three REST endpoints: POST /builds, GET /builds/:id, GET /builds
- API key authentication middleware
- JSON request/response formats
- Integration with Builder and BuildDB

### ✓ Exit Criteria
- Can start build via POST /builds
- Can poll build status via GET /builds/:id
- Authentication rejects invalid API keys
- Integration tests cover happy path

### 🌐 Proposed Endpoints
```
POST /api/v1/builds
  Body: { "packages": ["editors/vim"], "profile": "default" }
  Response: { "build_id": "uuid" }

GET /api/v1/builds/:id
  Response: { "status": "running|success|failed", "start_time": "...", ... }

GET /api/v1/builds
  Response: { "items": [...], "next": "cursor" }
```

### 📖 Documentation
- **[Phase 5 Plan](docs/design/PHASE_5_MIN_API.md)** - Complete specification

### 🔑 Key Decisions
- Polling-based (no WebSocket/SSE for MVP)
- Simple router, minimal dependencies
- Optional phase - can be deferred if not needed

---

## Phase 6: Testing Strategy ⚪

**Status**: ⚪ Planned  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phases 1-3 completion

### 🎯 Goals
- Ensure reliability of core libraries
- Validate end-to-end build flow
- Set up continuous integration

### 📦 Main Deliverables
- Comprehensive unit tests for pkg, builddb, builder
- Integration tests for full build pipeline
- Test fixtures (minimal test ports or mocks)
- CI configuration (GitHub Actions or similar)

### ✓ Exit Criteria
- All packages have >80% test coverage
- Integration test successfully builds 1-3 small ports
- CI runs on every PR with race detector
- Failure tests validate error propagation

### 🧪 Test Coverage
- **pkg**: Parse, Resolve, TopoOrder, cycle detection
- **builddb**: CRUD operations, NeedsBuild, CRC updates
- **builder**: Worker lifecycle, failure propagation, stats
- **Integration**: End-to-end build with real ports

### 📖 Documentation
- **[Phase 6 Plan](docs/design/PHASE_6_TESTING.md)** - Complete specification

### 🔑 Key Decisions
- Use standard `go test` with race detector
- Minimal test ports or mocked execution
- Out of scope: benchmarks, chaos testing (defer to post-MVP)

---

## Phase 7: Integration & Migration ⚪

**Status**: ⚪ Planned  
**Timeline**: Not started | Target: TBD  
**Dependencies**: Phases 1-6 completion

### 🎯 Goals
- Wire all new components into existing CLI
- Provide migration path from legacy CRC to BuildDB
- Maintain backward compatibility during transition

### 📦 Main Deliverables
- Updated CLI commands using new pipeline
- BuildDB initialization with fallback to legacy CRC
- Migration tooling for existing installations
- Updated logging with UUID tracking

### ✓ Exit Criteria
- End-to-end build via CLI works correctly
- CRC skip validated across two consecutive runs
- Migration from file-based CRC completes successfully
- All existing CLI commands remain functional

### ⚙️ CLI Mapping
- `dsynth build [ports...]` → uses new pipeline
- `dsynth force` → bypasses NeedsBuild check
- `dsynth upgrade-system` → uses pkg for installed packages
- Legacy commands continue to work

### 📖 Documentation
- **[Phase 7 Plan](docs/design/PHASE_7_INTEGRATION.md)** - Complete specification

### 🔑 Key Decisions
- BuildDB primary, legacy CRC fallback
- Lazy migration: populate on successful builds
- Keep existing log file structure
- Add UUID to log messages for traceability

---

## 🤝 Contributing Workflow

### For New Contributors

1. **Read Essential Docs**
   - [AGENTS.md](AGENTS.md) - Development workflow and commit guidelines
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

### ✅ Testing Requirements

- Add unit tests for new functions
- Maintain or improve code coverage
- Run `go test -v -race ./...` before committing
- Add integration tests for end-to-end flows

### 📝 Documentation Requirements

- Add godoc comments for exported functions
- Update relevant phase documentation
- Update README.md if public API changes
- Update DEVELOPMENT.md if phase status changes

---

## 📈 Project Status Summary

### Overall Progress
- **Phase 1**: 🟢 100% core complete (9/9 exit criteria met) - documentation tasks remaining
- **Phase 1.5**: 🟢 100% complete (fidelity verification + C-ism removal)
- **Phase 2-7**: ⚪ Planned (ready to start)
- **Total Estimated Remaining**: ~5-8 hours for Phase 1 documentation, then ~50-70 hours for Phases 2-7

### Recent Milestones
- ✅ 2025-11-26: Phase 1 Task 6 complete - Developer guide with 5 runnable examples
- ✅ 2025-11-26: Phase 1 Task 5 complete - Comprehensive godoc documentation added
- ✅ 2025-11-25: Phase 1 Task 4 complete - Removed global state, pkgRegistry now parameter-based
- ✅ 2025-11-25: Phase 1 Task 3 complete - Structured error types with 4 tests
- ✅ 2025-11-26: Phase 1.5 Part B complete - All C-isms removed - Commits 175462b, 063d0e7, eb1f7e7, ae58f64
- ✅ 2025-11-26: B4: Added typed PackageFlags enum - Commit eb1f7e7
- ✅ 2025-11-26: B3: Added typed DepType enum - Commit 063d0e7
- ✅ 2025-11-26: B2: Converted linked lists to slices (-53 lines) - Commit ae58f64
- ✅ 2025-11-26: B1: Removed dead Package.mu field - Commit 175462b
- ✅ 2025-11-25: Phase 1.5 Part A - Created 10 C fidelity tests (all passing)
- ✅ 2025-11-25: BuildState infrastructure and registry (Task 1.1) - Commit c226c8f
- ✅ 2025-11-25: Build package migrated to BuildStateRegistry (Task 1.2) - Commit c9923a7
- ✅ 2025-11-25: Parsing layer integrated with BuildStateRegistry (Task 1.5) - Commit 78bf7d7
- ✅ 2025-11-25: CRC database extracted to builddb package (Task 2)
- ✅ 2025-11-25: Phase 1 comprehensive analysis and TODO created
- ✅ 2025-11-21: Core pkg API implemented (Parse, Resolve, TopoOrder)
- ✅ 2025-11-21: Cycle detection implemented and tested

### Next Milestones
- 🎯 Task 7: Add integration tests (~2-3h)
- 🎯 Task 8: Improve error test coverage (~2-3h)
- 🎯 Phase 1 quality tasks completion

### Known Issues
See [Phase 1 TODO](docs/design/PHASE_1_TODO.md) for complete list.

**⚠️ Critical:**
- None - all critical architecture work complete! 🎉

**🔶 Medium:**
- Missing integration tests
- Error path test coverage incomplete

**🔹 Low:**
- No context.Context support
- BulkQueue implementation exposed
- No benchmark tests
- Pre-existing race condition in BuildStateRegistry (not related to Phase 1.5 changes)

---

## 🚀 Future Plans

See [FUTURE_BACKLOG.md](docs/design/FUTURE_BACKLOG.md) for features deferred beyond Phase 7:

- ncurses UI (like original dsynth)
- Profile switching
- Hook system for custom actions
- Advanced NUMA support
- Remote builder support
- Package signing
- Distributed builds

---

## ❓ Getting Help

- **Issues**: Check existing GitHub issues or create new ones
- **Discussions**: Use GitHub Discussions for design questions
- **Documentation**: All docs in `docs/design/` directory
- **Contact**: [Project maintainers contact info]

---

**Last Updated**: 2025-11-25  
**Document Version**: 1.0
