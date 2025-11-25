# Phase 1: Library Extraction (pkg)

**Status**: 🟡 Functionally Complete, Architecturally Incomplete  
**Last Updated**: 2025-11-25

## Goals
- Isolate package metadata and dependency resolution into a pure library (`pkg`).
- Provide a small, stable API for parsing port specs and generating build order.
- Remove mixed concerns (CRC/status/build flags) from `pkg` where possible.

## Scope (MVP)
- Parse port specs (supports flavor syntax `origin@flavor`).
- Build dependency graph and compute topological order.
- Expose minimal `Package` struct and functions.

## Non-Goals (Deferred)
- Persistent package registry, advanced metadata caching.
- Deep validation of port Makefiles beyond what MVP needs.

## Target Public API
```go
// Pure metadata structure (no build state)
type Package struct {
    PortDir  string
    Category string
    Name     string
    Version  string
    Flavor   string
    PkgFile  string
    
    // Dependency raw strings (from Makefile)
    FetchDeps   string
    ExtractDeps string
    PatchDeps   string
    BuildDeps   string
    LibDeps     string
    RunDeps     string
    
    // Resolved dependency graph
    IDependOn   []*PkgLink
    DependsOnMe []*PkgLink
    DepiCount   int
    DepiDepth   int
    
    // Linked list traversal
    Next *Package
    Prev *Package
}

// Core API functions
func Parse(portSpecs []string, cfg *config.Config) (*Package, error)
func Resolve(head *Package, cfg *config.Config) error
func TopoOrder(head *Package) ([]*Package, error)
```

## Current Implementation Status

### ✅ Completed Features

**Core Functions:**
- ✅ `Parse()` - Wrapper for `ParsePortList()` - parses port specs into linked list
- ✅ `Resolve()` - Wrapper for `ResolveDependencies()` - builds dependency graph
- ✅ `TopoOrder()` - Wrapper for `GetBuildOrder()` - Kahn's algorithm topological sort
- ✅ `TopoOrderStrict()` - Cycle detection variant with error return

**Parsing & Resolution:**
- ✅ Port spec parsing with flavor support (`origin@flavor`)
- ✅ Parallel bulk fetching of package metadata via `BulkQueue`
- ✅ Recursive dependency resolution (all 6 dependency types)
- ✅ Bidirectional dependency graph construction
- ✅ Global package registry to avoid duplicates

**Testing:**
- ✅ `topo_test.go` - Topological ordering happy path
- ✅ `cycle_test.go` - Cycle detection
- ✅ `dep_parse_test.go` - Dependency string parsing (basic, flavor, nonexistent)

### ⚠️ Issues Identified

**1. Mixed Concerns (CRITICAL - Violates Phase 1 Goal)**
- ❌ `Package` struct contains build state flags (`Flags`, `PkgFManualSel`, `PkgFSuccess`, etc.)
- ❌ `Package` struct contains build tracking fields (`IgnoreReason`, `LastPhase`, `LastStatus`)
- ❌ CRC database code lives in `pkg/` package (`crcdb.go`, `crcdb_helpers.go`)
- ❌ Build-time functions mixed with metadata (`MarkPackagesNeedingBuild()`, `UpdateCRCAfterBuild()`)
- **Impact**: Library is not reusable, tightly coupled to build system

**2. Global State Issues**
- ❌ `globalRegistry` - Package-level global, not thread-safe for independent operations
- ❌ `globalCRCDB` - Package-level global CRC database instance
- **Impact**: Makes testing harder, prevents concurrent independent uses

**3. Missing Error Types**
- ❌ No structured error types (uses `fmt.Errorf()` strings only)
- ❌ Should have: `ErrCycleDetected`, `ErrInvalidSpec`, `ErrPortNotFound`
- **Impact**: Error handling is less precise, harder to test

**4. Incomplete Documentation**
- ❌ No godoc comments on exported functions
- ❌ No developer guide for using the pkg library
- ❌ README doesn't document Phase 1 API
- **Impact**: Library is not developer-friendly

**5. Test Coverage Gaps**
- ❌ No integration test for full Parse→Resolve→TopoOrder workflow
- ❌ No error path tests (invalid inputs, missing ports)
- ❌ No tests for global registry behavior
- ❌ No benchmark tests for large graphs
- **Impact**: Unknown edge case behavior, performance characteristics

**6. API Design Issues**
- ❌ `BulkQueue` implementation detail exposed in `pkg/` package
- ❌ Some internal functions should be private (lowercase names)
- ❌ No `context.Context` support for cancellation
- **Impact**: API surface too large, not cancellable

## Remaining Work for Phase 1 Completion

See `PHASE_1_TODO.md` for detailed task breakdown.

### High Priority (Blocking Phase 1 Exit)
1. ✅ ~~Implement core Parse/Resolve/TopoOrder functions~~ - DONE
2. ✅ ~~Add cycle detection~~ - DONE
3. ✅ ~~Basic unit tests~~ - DONE
4. ❌ **Separate build state from Package struct** - CRITICAL
5. ❌ **Move CRC database to separate package** - CRITICAL
6. ❌ **Add structured error types** - HIGH
7. ❌ **Remove global state** - HIGH

### Medium Priority (Quality & Usability)
8. ❌ Add comprehensive godoc comments
9. ❌ Create developer guide
10. ❌ Add integration tests
11. ❌ Improve error path test coverage
12. ❌ Update README with API examples

### Low Priority (Nice to Have)
13. ❌ Add context.Context support
14. ❌ Make BulkQueue internal/private
15. ❌ Add benchmark tests

## Deliverables

### Completed
- ✅ Compilable `pkg` library
- ✅ Basic unit tests (happy paths)
- ✅ Core API functions (Parse, Resolve, TopoOrder)

### Incomplete
- ❌ Pure metadata-only Package struct (still has build state)
- ❌ Separated CRC/build tracking (still in pkg/)
- ❌ Comprehensive godoc comments
- ❌ Minimal developer guide
- ❌ Structured error types
- ❌ Full test coverage (edge cases, errors, integration)

## Exit Criteria

### Original Criteria
- ✅ Given a set of ports, `TopoOrder` returns a correct, cycle-free order - **ACHIEVED**
- ✅ All existing commands compile and run with new API - **ACHIEVED**

### Additional Criteria for True "Pure Library" Goal
- ❌ Package struct contains ONLY metadata (no build state/flags) - **NOT ACHIEVED**
- ❌ CRC/build tracking separated into different package - **NOT ACHIEVED**
- ❌ No global state in pkg package - **NOT ACHIEVED**
- ❌ Structured errors for all failure modes - **NOT ACHIEVED**
- ❌ Comprehensive documentation (godoc + guide) - **NOT ACHIEVED**

**Phase 1 Status**: Functionally complete but architecturally incomplete. The library works but doesn't meet the "pure library" separation goal stated in the phase objectives.

## Dependencies
- None (foundation for later phases).

## Risks & Mitigations
- ✅ Risk: Cycles in ports graph → **Mitigated**: Cycle detection implemented with TopoOrderStrict
- ✅ Risk: Flavors parsing ambiguity → **Mitigated**: Explicit parser with tests
- ⚠️ Risk: Breaking changes during refactor → **Mitigation**: Keep wrapper functions, deprecate gradually
- ⚠️ Risk: Performance regression from separation → **Mitigation**: Add benchmark tests before/after

## Notes for Phase 2

When starting Phase 2 (BuildDB), consider:
- CRC database will be migrated from `pkg/crcdb.go` to new `builddb/` package
- Build state flags should move to `build/` package or new `buildstate/` package
- Phase 2 should use the cleaned Phase 1 API for package metadata
- Current wrapper functions (ParsePortList, ResolveDependencies, GetBuildOrder) can be deprecated