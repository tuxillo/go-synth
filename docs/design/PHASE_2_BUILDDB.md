# Phase 2: Minimal BuildDB (bbolt)

**Status**: 🟡 25% Complete (3/12 tasks)  
**Last Updated**: 2025-11-27

## Goals
- Add minimal persistent tracking of build attempts and CRCs using bbolt (BoltDB successor).
- Enable incremental builds by skipping unchanged ports.
- Replace custom binary CRC database with proper embedded database.

## Scope (MVP)
- Store build records with UUID, status, timestamps.
- Track latest successful build per port/version combination.
- Maintain CRC index for incremental build detection.
- Provide clean API for build tracking and CRC queries.

## Non-Goals (Deferred)
- Advanced build analytics or historical reporting.
- Multi-database synchronization or distributed builds.
- Full build log storage (logs remain file-based).
- Migration of historical build data (fresh start acceptable).

## Target Public API

```go
// Core structures
type BuildRecord struct {
    UUID      string
    PortDir   string
    Version   string
    Status    string // "running" | "success" | "failed"
    StartTime time.Time
    EndTime   time.Time
}

// Primary API functions
func OpenDB(path string) (*DB, error)
func (db *DB) Close() error

// Build record operations
func (db *DB) SaveRecord(rec *BuildRecord) error
func (db *DB) GetRecord(uuid string) (*BuildRecord, error)
func (db *DB) LatestFor(portDir, version string) (*BuildRecord, error)
func (db *DB) UpdateRecordStatus(uuid, status string, endTime time.Time) error

// CRC operations for incremental builds
func (db *DB) NeedsBuild(portDir string, currentCRC uint32) (bool, error)
func (db *DB) UpdateCRC(portDir string, crc uint32) error
func (db *DB) GetCRC(portDir string) (uint32, bool, error)
```

## Current Implementation Status

### Current State: Custom Binary CRC Database

**Existing Implementation** (`builddb/crc.go` - 495 lines):
- Custom binary format with 32-byte header
- Fixed 1MB record array (max 16,384 entries)
- Manual CRC32 checksum validation
- File-based locking for concurrent access
- Simple linear scan for lookups
- No build record tracking

**Limitations:**
- No structured build records
- Fixed capacity (16K ports maximum)
- Linear search performance O(n)
- No indexing or query capabilities
- Manual memory management
- No ACID guarantees

### Target State: bbolt Embedded Database

**Technology Choice:**
- **Package**: `go.etcd.io/bbolt` (maintained fork of BoltDB)
- **Why bbolt**: BoltDB archived in 2019; bbolt is actively maintained by etcd.io
- **License**: MIT (compatible)
- **Features**: ACID transactions, B+tree indexing, zero-config embedded DB

**Database Schema** (3 buckets):
```
builds/
  └─ [uuid] → {portdir, version, status, start_time, end_time} (JSON)

packages/
  └─ [portdir@version] → uuid (ASCII, points to latest successful build)

crc_index/
  └─ [portdir] → crc32 (4 bytes, binary uint32)
```

**Performance Benefits:**
- O(log n) lookups vs O(n) linear scan
- ACID transactions (atomic status updates)
- Dynamic growth (no 16K limit)
- Concurrent read access
- Automatic crash recovery

## Remaining Work for Phase 2 Completion

### High Priority (Core Implementation)
1. ✅ **Add bbolt dependency** - Update `go.mod` with `go.etcd.io/bbolt` (DONE: commit 6a6ff7b)
2. ✅ **Create DB wrapper** - Implement `DB` struct with Open/Close (DONE: commit 48569e6)
3. ✅ **Implement bucket creation** - Initialize 3 buckets on first open (DONE: commit 48569e6)
4. ✅ **Build record CRUD** - SaveRecord, GetRecord, UpdateRecordStatus (DONE: commit TBD)
5. ❌ **Package tracking** - LatestFor, update packages bucket on success
6. ❌ **CRC operations** - NeedsBuild, UpdateCRC, GetCRC

### Medium Priority (Integration & Testing)
7. ❌ **Migration path** - Coexistence strategy with existing CRC file
8. ❌ **Unit tests** - Test each API function (CRUD, CRC logic)
9. ❌ **Integration test** - Full build workflow with DB updates
10. ❌ **Error handling** - Structured errors for not found, corruption, etc.

### Low Priority (Quality & Polish)
11. ❌ **Godoc comments** - Document all exported types and functions
12. ❌ **Benchmarks** - Compare performance vs. old CRC file

## Current Task Breakdown

### Task 1: Add bbolt Dependency ✅ COMPLETE
- ✅ Run `go get go.etcd.io/bbolt@latest`
- ✅ Update `go.mod` and `go.sum`
- ✅ Verify compilation
- **Completed**: 2025-11-27 (commit 6a6ff7b)
- **Result**: Added go.etcd.io/bbolt v1.4.3, upgraded Go 1.21→1.23, golang.org/x/sys v0.15.0→v0.29.0

### Task 2: Create Database Wrapper ✅ COMPLETE
- ✅ Create `builddb/db.go` with `DB` struct
- ✅ Implement `OpenDB(path string) (*DB, error)`
- ✅ Implement `Close() error`
- ✅ Initialize 3 buckets: `builds`, `packages`, `crc_index`
- **Completed**: 2025-11-27 (commit TBD)
- **Result**: Created db.go (113 lines), bbolt now direct dependency, verified with test

### Task 3: Build Record CRUD ✅ COMPLETE
- ✅ Implement `SaveRecord(rec *BuildRecord) error`
- ✅ Implement `GetRecord(uuid string) (*BuildRecord, error)`
- ✅ Implement `UpdateRecordStatus(uuid, status, endTime)` 
- ✅ Use JSON encoding for build records
- **Completed**: 2025-11-27 (commit TBD)
- **Result**: Added 3 CRUD methods (152 lines), full save/retrieve/update cycle working

### Task 4: Package Tracking (1 hour)
- Implement `LatestFor(portDir, version string) (*BuildRecord, error)`
- Update `packages` bucket on successful builds
- Use `portdir@version` as key format

### Task 5: CRC Operations (1.5 hours)
- Implement `NeedsBuild(portDir, currentCRC) (bool, error)`
- Implement `UpdateCRC(portDir, crc) error`
- Implement `GetCRC(portDir) (uint32, bool, error)`
- Use binary encoding for CRC values

### Task 6: Migration Strategy (1 hour)
- Document coexistence approach (both old and new DB temporarily)
- Implement `MigrateFromCRCFile(oldPath string) error` (optional)
- Populate `crc_index` from existing `builddb/crc.db` on first run

### Task 7: Error Types (1 hour)
- Add `ErrNotFound`, `ErrCorrupted`, `ErrInvalidUUID`, etc.
- Use sentinel errors for `errors.Is()` checks
- Add structured error types for detailed context

### Task 8: Unit Tests (3 hours)
- Test `SaveRecord` → `GetRecord` roundtrip
- Test `UpdateRecordStatus` transitions
- Test `LatestFor` returns most recent successful build
- Test `NeedsBuild` logic (match, changed, missing)
- Test concurrent access (read/write transactions)

### Task 9: Integration Test (1.5 hours)
- Simulate full build workflow:
  1. Check `NeedsBuild` (returns true)
  2. `SaveRecord` with status="running"
  3. `UpdateRecordStatus` to "success"
  4. Update `packages` bucket
  5. `UpdateCRC` with new value
  6. Check `NeedsBuild` again (returns false)

### Task 10: Godoc & Documentation (1 hour)
- Add godoc comments to all exported functions
- Document bucket schemas and key formats
- Add code examples to README

### Task 11: Benchmarks (1 hour)
- Benchmark CRC lookups (old vs. new)
- Benchmark build record queries
- Compare memory usage

### Task 12: Integration with CLI (2 hours)
- Update `go-synth build` command to use new DB
- Add `--db-path` flag (default: `~/.go-synth/builds.db`)
- Maintain backward compatibility with old CRC file

**Total Estimated Effort**: 12-16 hours

## Deliverables

### Completed (3/6)
- ✅ bbolt dependency added (go.etcd.io/bbolt v1.4.3)
- ✅ bbolt integration (`builddb/db.go` with OpenDB/Close)
- ✅ Build record CRUD operations (SaveRecord, GetRecord, UpdateRecordStatus)

### Incomplete (3/6)
- ❌ CRC indexing with NeedsBuild logic
- ❌ Unit and integration tests
- ❌ Migration from old CRC file
- ❌ CLI integration

## Exit Criteria

### Core Functionality
- ✅ `NeedsBuild` returns false when CRC unchanged; true when changed or missing
- ✅ Successful build writes to `builds`, updates `packages` and `crc_index`
- ✅ `LatestFor` returns most recent successful build for port/version
- ✅ Database survives process crash (ACID guarantees)

### Quality Standards
- ✅ Unit tests cover all API functions
- ✅ Integration test validates full build workflow
- ✅ Godoc comments on all exported types and functions
- ✅ Performance benchmarks vs. old CRC file

### Migration
- ✅ Can coexist with old CRC file during transition
- ✅ Optional migration utility to import old CRC data
- ✅ CLI updated to use new database

**Phase 2 Status**: In progress (3/12 tasks, 25% complete). Phase 1 complete (9/9 exit criteria met), providing stable `pkg` API for port metadata. Tasks 1-3 completed 2025-11-27 (dependency + DB wrapper + CRUD). No blockers.

## Dependencies
- Phase 1 (`pkg` provides stable `PortDir`, `Version`, and `Package` API)
- `go.etcd.io/bbolt` package (to be added)

## Key Decisions

### Technology: bbolt vs. BoltDB
- **Decision**: Use `go.etcd.io/bbolt` instead of `github.com/boltdb/bolt`
- **Rationale**: Original BoltDB archived in 2019; bbolt is maintained fork by etcd.io
- **API Compatibility**: bbolt maintains 100% compatibility with original Bolt API
- **Import Path**: `import bolt "go.etcd.io/bbolt"`

### Database Location
- **Decision**: `~/.go-synth/builds.db` (default)
- **Override**: `--db-path` CLI flag
- **Rationale**: User-specific, not tied to ports tree location

### Key Format: packages Bucket
- **Decision**: Use `portdir@version` as key (e.g., `lang/go@default`)
- **Alternative Considered**: Composite key with separator `|`
- **Rationale**: Matches flavor syntax used throughout codebase

### CRC Storage: Binary vs. ASCII
- **Decision**: Store CRC as binary `uint32` (4 bytes)
- **Alternative Considered**: Store as hex string (8 bytes)
- **Rationale**: Smaller, faster comparisons, consistent with old format

### Migration Strategy
- **Decision**: Coexistence approach (both old and new DB temporarily)
- **Alternative Considered**: Hard cutover with required migration
- **Rationale**: Gradual rollout, easier testing, less risk

## Risks & Mitigations

### Risk: Database Corruption
- **Mitigation**: bbolt provides ACID guarantees and automatic recovery
- **Mitigation**: Regular backups (user responsibility)

### Risk: Performance Regression
- **Likelihood**: Low (B+tree indexing faster than linear scan)
- **Mitigation**: Add benchmarks to validate improvement

### Risk: Concurrent Access Issues
- **Mitigation**: bbolt handles concurrent reads; writes serialized via transactions
- **Mitigation**: Test concurrent access patterns in integration tests

### Risk: Migration Data Loss
- **Mitigation**: Keep old CRC file during transition
- **Mitigation**: Make migration optional, log errors

## Comparison: Old vs. New

| Feature | Current (crc.go) | Target (bbolt) |
|---------|------------------|----------------|
| **Format** | Custom binary | bbolt (B+tree) |
| **Capacity** | 16,384 entries | Unlimited |
| **Lookup** | O(n) linear scan | O(log n) indexed |
| **Build Records** | No | Yes (UUID, status, timestamps) |
| **Indexing** | None | packages, crc_index buckets |
| **ACID** | No | Yes |
| **Crash Recovery** | Manual | Automatic |
| **Concurrent Access** | File locks | Transaction-based |
| **Query Capabilities** | CRC only | Build history, latest success |

## Notes for Phase 3

When starting Phase 3 (CLI Enhancements), consider:
- Use Phase 2 `builddb` API for `--skip-built` flag implementation
- Add `go-synth status` command to query build records
- Consider `go-synth clean-db` command to prune old records
- Build record UUIDs can link to build logs in filesystem
