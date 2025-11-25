# Phase 1 TODO List - Complete Library Separation

**Created**: 2025-11-25  
**Status**: 🟡 In Progress  
**Goal**: Complete Phase 1 by creating a truly pure `pkg` library with no build concerns

## Overview

This document tracks the remaining work to properly complete Phase 1. While the core functionality is working (Parse, Resolve, TopoOrder), the library still has mixed concerns that violate the "pure library" goal.

---

## 🔴 CRITICAL - Blocking Phase 1 Exit

### Task 1: Separate Build State from Package Struct ✅ COMPLETE

**Priority**: CRITICAL  
**Estimated Effort**: 4-6 hours → **Actual: ~2 hours**  
**Status**: ✅ **COMPLETE** (2025-11-25)

**Problem**: 
- `Package` struct contains build-time flags and state
- Makes the library not reusable for pure metadata operations
- Violates single responsibility principle

**Solution**:
Create separate `BuildState` struct to track build-specific information.

**Completed Steps**:
- [x] 1.1. Create new file `pkg/buildstate.go` (143 lines)
- [x] 1.2. Define `BuildState` struct with Pkg, Flags, IgnoreReason, LastPhase
- [x] 1.3. Create `BuildStateRegistry` with thread-safe map and mutex
- [x] 1.4. Remove these fields from `Package` struct:
  - `Flags` (all PkgF* flags) ✅
  - `IgnoreReason` ✅
  - `LastPhase` ✅
  - ~~`LastStatus`~~ (kept - not build state)
- [x] 1.5. Update `bulk.go` to return flags separately
- [x] 1.6. Update `main.go` to create and use registry
- [x] 1.7. Update `cmd/build.go` to create and use registry
- [x] 1.8. Update `build/build.go`, `build/phases.go`, `build/fetch.go` to use registry
- [x] 1.9. Update `getPackageInfo()` to return flags separately
- [x] 1.10. Update `ParsePortList()` and `ResolveDependencies()` to accept registry
- [x] 1.11. Run tests - all passing ✅
- [x] 1.12. Update documentation comments

**Files Created**:
- `pkg/buildstate.go` - BuildState infrastructure (NEW)
- `pkg/buildstate_test.go` - 8 comprehensive tests (NEW)

**Files Modified**:
- `pkg/pkg.go` - Package struct, getPackageInfo, ParsePortList, ResolveDependencies
- `pkg/bulk.go` - BulkQueue to pass flags through bulkResult
- `pkg/deps.go` - resolveDependencies signature and registry usage
- `pkg/topo_test.go` - Test updates for registry parameter
- `main.go` - Registry creation and usage
- `cmd/build.go` - Registry creation and usage
- `build/build.go` - Registry usage (15 locations)
- `build/phases.go` - Registry usage (4 locations)
- `build/fetch.go` - Registry usage (2 locations)

**Commits**:
- e261af8 - Task 1.1: Create BuildState infrastructure
- 6514473 - Task 1.2: Migrate build package to BuildStateRegistry
- 28be09c - Task 1.5: Update pkg parsing to work with BuildStateRegistry
- 0f04fc9 - Task 1.3: Remove build state fields from Package struct

**Acceptance Criteria**: ✅ ALL MET
- ✅ Package struct has zero build-state fields
- ✅ All existing functionality still works
- ✅ All tests pass (15 tests including 8 new BuildState tests)

---

### Task 2: Extract CRC Database to Separate Package ✅ COMPLETE

**Priority**: CRITICAL  
**Estimated Effort**: 3-4 hours → **Actual: Already complete**  
**Status**: ✅ **COMPLETE** (Pre-existing)

**Problem**:
- CRC database code lived in `pkg/` package
- CRC tracking is build-time concern, not metadata
- Prevented pkg from being a pure metadata library

**Solution**:
Move CRC database to its own package (prepare for Phase 2 BuildDB).

**Status**: This task was already completed in an earlier refactoring. The CRC database has been separated into the `builddb/` package.

**Completed Steps**:
- [x] 2.1. Create new directory `builddb/`
- [x] 2.2. Create `builddb/crc.go` and move CRC database code
- [x] 2.3. Create `builddb/helpers.go` and move helper functions
- [x] 2.4. Update package declaration to `package builddb`
- [x] 2.5. Update imports in `pkg/pkg.go`
- [x] 2.6. Update all callers
- [x] 2.7. Run tests - all passing

**Acceptance Criteria**: ✅ ALL MET
- ✅ No CRC code remains in pkg/ directory
- ✅ CRC functionality still works
- ✅ Tests pass
- ✅ `pkg` package has no knowledge of CRC tracking

---

### Task 3: Add Structured Error Types ✅ COMPLETE

**Priority**: HIGH  
**Estimated Effort**: 1-2 hours → **Actual: ~1.5 hours**  
**Status**: ✅ **COMPLETE** (2025-11-25)

**Problem**:
- All errors use `fmt.Errorf()` string formatting
- Cannot distinguish error types programmatically
- Harder to test specific error conditions

**Solution**:
Define custom error types for common failure modes.

**Completed Steps**:
- [x] 3.1. Create new file `pkg/errors.go`
- [x] 3.2. Define 5 sentinel errors and 2 structured error types
- [x] 3.3. Update `TopoOrderStrict()` to return `*CycleError` (deps.go:388)
- [x] 3.4. Update `ParsePortList()` to return `ErrNoValidPorts` (pkg.go:188)
- [x] 3.5. Update `getPackageInfo()` to return `*PortNotFoundError` (pkg.go:233)
- [x] 3.6. Update error handling in cycle_test.go and topo_test.go
- [x] 3.7. Add comprehensive godoc comments for all error types
- [x] 3.8. Create errors_test.go with 4 test functions
- [x] 3.9. Run tests - all 23 tests passing ✅

**Files Created**:
- `pkg/errors.go` - 80 lines with 5 sentinel errors + 2 structured types (NEW)
- `pkg/errors_test.go` - 115 lines with 4 comprehensive test functions (NEW)

**Files Modified**:
- `pkg/pkg.go` - Updated 2 error returns (lines 188, 233)
- `pkg/deps.go` - Updated 1 error return (line 388)
- `pkg/topo_test.go` - Enhanced error checking with errors.Is()
- `pkg/cycle_test.go` - Enhanced error checking with errors.As()

**Error Types Defined**:
- Sentinel errors: `ErrCycleDetected`, `ErrInvalidSpec`, `ErrPortNotFound`, `ErrNoValidPorts`, `ErrEmptySpec`
- Structured errors: `*PortNotFoundError` (with PortSpec, Path), `*CycleError` (with counts, optional packages)

**Acceptance Criteria**: ✅ ALL MET
- ✅ All critical functions return typed errors
- ✅ Tests use `errors.Is()` and `errors.As()` successfully
- ✅ Error messages remain informative and helpful
- ✅ All 23 tests passing (including 4 new error tests)

---

### Task 4: Remove Global State from pkg Package

**Priority**: HIGH  
**Estimated Effort**: 2-3 hours  

**Problem**:
- `globalRegistry` is package-level global
- `globalCRCDB` is package-level global (will be moved in Task 2)
- Makes concurrent use impossible
- Makes testing harder (shared state)

**Solution**:
Make registry instance-based, pass as parameter or context.

**Steps**:
- [ ] 4.1. Add `Registry` field to Config or create Context struct:
  ```go
  type Context struct {
      Config   *config.Config
      Registry *PackageRegistry
  }
  ```
- [ ] 4.2. Update `Parse()` signature:
  ```go
  func Parse(portSpecs []string, ctx *Context) (*Package, error)
  ```
- [ ] 4.3. Update `Resolve()` signature:
  ```go
  func Resolve(head *Package, ctx *Context) error
  ```
- [ ] 4.4. Pass registry through call chain:
  - `ParsePortList()` takes registry parameter
  - `resolveDependencies()` takes registry parameter
  - `BulkQueue` takes registry parameter
- [ ] 4.5. Update all callers in main.go and cmd/build.go
- [ ] 4.6. Remove `globalRegistry` variable
- [ ] 4.7. Update tests to create isolated registries
- [ ] 4.8. Add test for concurrent operations

**Alternative approach** (simpler, less breaking):
- [ ] 4.1. Keep global registry but add `NewRegistry()` function
- [ ] 4.2. Add `SetRegistry()` for dependency injection
- [ ] 4.3. Add `ResetRegistry()` for tests
- [ ] 4.4. Document that global registry is for convenience only

**Files to modify**:
- `pkg/pkg.go` (add Context or modify signatures)
- `pkg/deps.go` (pass registry through)
- `pkg/bulk.go` (pass registry through)
- `main.go` (create/pass context)
- `cmd/build.go` (create/pass context)
- All test files (create isolated registries)

**Acceptance Criteria**:
- No package-level global registry (or properly managed)
- Tests can run in parallel without interference
- Concurrent uses don't conflict

---

## 🟡 MEDIUM PRIORITY - Quality & Usability

### Task 5: Add Comprehensive Godoc Comments

**Priority**: MEDIUM  
**Estimated Effort**: 2-3 hours  

**Problem**:
- Many exported functions lack documentation
- Package-level comment missing
- Users don't know how to use the API

**Solution**:
Add proper godoc comments to all exported symbols.

**Steps**:
- [ ] 5.1. Add package-level comment to `pkg/pkg.go`:
  ```go
  // Package pkg provides package metadata parsing and dependency resolution
  // for BSD ports. It supports parsing port specifications, resolving
  // dependency graphs, and computing topological build order.
  //
  // Basic usage:
  //   head, err := pkg.Parse([]string{"editors/vim"}, cfg)
  //   if err != nil { ... }
  //   
  //   err = pkg.Resolve(head, cfg)
  //   if err != nil { ... }
  //   
  //   order, err := pkg.TopoOrder(head)
  //   if err != nil { ... }
  package pkg
  ```
- [ ] 5.2. Add godoc to `Package` struct explaining each field
- [ ] 5.3. Add godoc to `Parse()` function
- [ ] 5.4. Add godoc to `Resolve()` function
- [ ] 5.5. Add godoc to `TopoOrder()` function
- [ ] 5.6. Add godoc to `TopoOrderStrict()` function
- [ ] 5.7. Add godoc to `PkgLink` struct
- [ ] 5.8. Add godoc to `PackageRegistry` type and methods
- [ ] 5.9. Run `go doc` to verify formatting
- [ ] 5.10. Generate HTML docs with `godoc -http=:6060` and review

**Files to modify**:
- `pkg/pkg.go`
- `pkg/deps.go`
- `pkg/bulk.go`

**Acceptance Criteria**:
- All exported symbols have godoc comments
- Package overview is clear
- Examples are included where helpful

---

### Task 6: Create Developer Guide

**Priority**: MEDIUM  
**Estimated Effort**: 3-4 hours  

**Problem**:
- No documentation on how to use pkg library
- Developers have to read source code
- Phase 1 deliverable missing

**Solution**:
Create `PHASE_1_DEVELOPER_GUIDE.md` with examples.

**Steps**:
- [ ] 6.1. Create file `PHASE_1_DEVELOPER_GUIDE.md`
- [ ] 6.2. Write overview section explaining pkg library purpose
- [ ] 6.3. Write "Quick Start" section with basic example
- [ ] 6.4. Document Package struct and its fields
- [ ] 6.5. Document Parse() function with examples
- [ ] 6.6. Document Resolve() function with examples
- [ ] 6.7. Document TopoOrder() function with examples
- [ ] 6.8. Add error handling section
- [ ] 6.9. Add advanced usage section (flavors, large graphs)
- [ ] 6.10. Add "Common Patterns" section
- [ ] 6.11. Add troubleshooting section
- [ ] 6.12. Link from main README.md

**Content outline**:
```markdown
# Phase 1 Developer Guide - Using the pkg Library

## Overview
## Installation
## Quick Start
## Core Concepts
  - Package Struct
  - Dependency Graph
  - Topological Ordering
## API Reference
  - Parse()
  - Resolve()
  - TopoOrder()
  - TopoOrderStrict()
## Error Handling
## Advanced Usage
  - Flavors
  - Custom Registries
  - Large Dependency Graphs
## Common Patterns
## Troubleshooting
## Examples
```

**Files to create**:
- `PHASE_1_DEVELOPER_GUIDE.md`

**Files to modify**:
- `README.md` (add link to developer guide)

**Acceptance Criteria**:
- Guide is comprehensive and clear
- Examples are tested and working
- Covers common use cases

---

### Task 7: Add Integration Tests

**Priority**: MEDIUM  
**Estimated Effort**: 2-3 hours  

**Problem**:
- Only unit tests exist
- No end-to-end workflow tests
- Cannot verify Parse→Resolve→TopoOrder pipeline

**Solution**:
Add integration tests for complete workflows.

**Steps**:
- [ ] 7.1. Create file `pkg/integration_test.go`
- [ ] 7.2. Add test for Parse→Resolve→TopoOrder workflow:
  ```go
  func TestFullWorkflow(t *testing.T) {
      cfg := &config.Config{DPortsPath: "/usr/ports"}
      
      // Parse
      head, err := Parse([]string{"editors/vim"}, cfg)
      if err != nil { t.Fatal(err) }
      
      // Resolve
      err = Resolve(head, cfg)
      if err != nil { t.Fatal(err) }
      
      // Order
      order, err := TopoOrder(head)
      if err != nil { t.Fatal(err) }
      
      // Verify order is valid
      if len(order) == 0 {
          t.Fatal("expected non-empty order")
      }
  }
  ```
- [ ] 7.3. Add test for multiple packages with shared dependencies
- [ ] 7.4. Add test for packages with flavors
- [ ] 7.5. Add test for error propagation through pipeline
- [ ] 7.6. Add test with mock port directory (no filesystem dependency)
- [ ] 7.7. Document how to run integration tests

**Files to create**:
- `pkg/integration_test.go`

**Acceptance Criteria**:
- Tests cover full Parse→Resolve→TopoOrder workflow
- Tests can run without real ports tree (using mocks)
- Tests verify correctness of build order

---

### Task 8: Improve Error Path Test Coverage

**Priority**: MEDIUM  
**Estimated Effort**: 2-3 hours  

**Problem**:
- Tests only cover happy paths
- Error handling not tested
- Edge cases not covered

**Solution**:
Add tests for error conditions and edge cases.

**Steps**:
- [ ] 8.1. Add test for invalid port specs:
  ```go
  func TestParseInvalidSpec(t *testing.T) {
      cfg := &config.Config{DPortsPath: "/usr/ports"}
      _, err := Parse([]string{"invalid"}, cfg)
      if !errors.Is(err, ErrInvalidSpec) {
          t.Errorf("expected ErrInvalidSpec, got %v", err)
      }
  }
  ```
- [ ] 8.2. Add test for non-existent port
- [ ] 8.3. Add test for empty port spec list
- [ ] 8.4. Add test for port with missing Makefile
- [ ] 8.5. Add test for port with invalid dependency syntax
- [ ] 8.6. Add test for very large dependency graph (performance)
- [ ] 8.7. Add test for self-dependency (should fail)
- [ ] 8.8. Add test for multiple dependency cycles
- [ ] 8.9. Run test coverage analysis: `go test -cover ./pkg`
- [ ] 8.10. Aim for >80% coverage on pkg package

**Files to modify**:
- `pkg/topo_test.go` (add error cases)
- `pkg/dep_parse_test.go` (add error cases)
- `pkg/cycle_test.go` (add more cycle scenarios)

**Files to create**:
- `pkg/errors_test.go` (test error types)

**Acceptance Criteria**:
- Error paths are tested
- Edge cases are covered
- Test coverage >80% in pkg package

---

### Task 9: Update README with API Examples

**Priority**: MEDIUM  
**Estimated Effort**: 1-2 hours  

**Problem**:
- README doesn't document Phase 1 API
- Users don't know the library exists
- No examples of programmatic usage

**Solution**:
Add "Library Usage" section to README.

**Steps**:
- [ ] 9.1. Add new section to README.md after "Quick Start"
- [ ] 9.2. Section: "## Library Usage (Phase 1 API)"
- [ ] 9.3. Add example of using pkg library programmatically:
  ```go
  package main
  
  import (
      "fmt"
      "dsynth/config"
      "dsynth/pkg"
  )
  
  func main() {
      cfg, _ := config.LoadConfig("", "default")
      
      // Parse port specs
      head, err := pkg.Parse([]string{"editors/vim"}, cfg)
      if err != nil {
          panic(err)
      }
      
      // Resolve dependencies
      err = pkg.Resolve(head, cfg)
      if err != nil {
          panic(err)
      }
      
      // Get build order
      order, err := pkg.TopoOrder(head)
      if err != nil {
          panic(err)
      }
      
      // Print build order
      for _, p := range order {
          fmt.Printf("Build: %s\n", p.PortDir)
      }
  }
  ```
- [ ] 9.4. Link to developer guide
- [ ] 9.5. Add note about Phase 1 status
- [ ] 9.6. Update architecture diagram to show pkg library

**Files to modify**:
- `README.md`

**Acceptance Criteria**:
- README clearly explains library can be used programmatically
- Example code is tested and works
- Links to developer guide

---

## 🟢 LOW PRIORITY - Nice to Have

### Task 10: Add context.Context Support

**Priority**: LOW  
**Estimated Effort**: 2-3 hours  

**Problem**:
- Operations cannot be cancelled
- No timeout support
- Not idiomatic Go for long operations

**Solution**:
Add context.Context parameter to main functions.

**Steps**:
- [ ] 10.1. Update function signatures:
  ```go
  func Parse(ctx context.Context, portSpecs []string, cfg *config.Config) (*Package, error)
  func Resolve(ctx context.Context, head *Package, cfg *config.Config) error
  ```
- [ ] 10.2. Check context cancellation in loops:
  ```go
  select {
  case <-ctx.Done():
      return ctx.Err()
  default:
      // continue work
  }
  ```
- [ ] 10.3. Pass context through to goroutines in BulkQueue
- [ ] 10.4. Update all callers to pass context
- [ ] 10.5. Add test for cancellation
- [ ] 10.6. Add test for timeout

**Files to modify**:
- `pkg/pkg.go`
- `pkg/deps.go`
- `pkg/bulk.go`
- `main.go`
- `cmd/build.go`

**Acceptance Criteria**:
- Operations can be cancelled via context
- Cancellation is quick (not waiting for full operation)

---

### Task 11: Make BulkQueue Internal

**Priority**: LOW  
**Estimated Effort**: 1 hour  

**Problem**:
- `BulkQueue` is an implementation detail
- Shouldn't be part of public API
- Currently uppercase (exported)

**Solution**:
Make BulkQueue internal/private.

**Steps**:
- [ ] 11.1. Rename `BulkQueue` to `bulkQueue` (lowercase)
- [ ] 11.2. Rename all methods to lowercase
- [ ] 11.3. Move to separate internal file or keep in bulk.go
- [ ] 11.4. Verify no external packages use it
- [ ] 11.5. Update comments to reflect internal status

**Files to modify**:
- `pkg/bulk.go`

**Acceptance Criteria**:
- BulkQueue is not exported
- No external packages can access it

---

### Task 12: Add Benchmark Tests

**Priority**: LOW  
**Estimated Effort**: 2 hours  

**Problem**:
- Unknown performance characteristics
- Cannot detect performance regressions
- No data for optimization decisions

**Solution**:
Add benchmark tests for key operations.

**Steps**:
- [ ] 12.1. Create file `pkg/benchmark_test.go`
- [ ] 12.2. Add benchmark for Parse with 10 packages
- [ ] 12.3. Add benchmark for Parse with 100 packages
- [ ] 12.4. Add benchmark for Resolve with 10 packages
- [ ] 12.5. Add benchmark for Resolve with 100 packages
- [ ] 12.6. Add benchmark for TopoOrder with various graph sizes
- [ ] 12.7. Add benchmark for cycle detection
- [ ] 12.8. Run benchmarks: `go test -bench=. ./pkg`
- [ ] 12.9. Document baseline performance in PHASE_1_LIBRARY.md
- [ ] 12.10. Set up benchmark tracking for future changes

**Files to create**:
- `pkg/benchmark_test.go`

**Acceptance Criteria**:
- Benchmarks run successfully
- Baseline performance documented
- Can detect regressions with `benchcmp`

---

## Summary Statistics

**Total Tasks**: 12  
**Completed**: 3 tasks (Task 1 ✅, Task 2 ✅, Task 3 ✅)  
**Critical (Blocking)**: 1 task remaining (Task 4)  
**High Priority**: 0 tasks  
**Medium Priority**: 5 tasks  
**Low Priority**: 3 tasks  

**Estimated Total Effort**: 25-35 hours  
**Completed Effort**: ~9-12 hours  
**Remaining Effort**: ~16-23 hours

**Completion Status**:
- ✅ Completed: 5 items (Task 1 ✅, Task 2 ✅, Task 3 ✅, core functions, cycle detection, basic tests)
- ❌ Remaining: 10 items
- 📊 Progress: ~35% complete by effort

---

## Suggested Order of Execution

For efficient completion, tackle tasks in this order:

1. **Week 1 - Critical Architecture** (~~12-16 hours~~ **2-3 hours remaining**)
   - ~~Task 2: Extract CRC Database (3-4h)~~ ✅ COMPLETE
   - ~~Task 1: Separate Build State (4-6h)~~ ✅ COMPLETE
   - ~~Task 3: Add Structured Errors (1-2h)~~ ✅ COMPLETE
   - Task 4: Remove Global State (2-3h) ⬅️ **NEXT RECOMMENDED**

2. **Week 2 - Documentation & Quality** (8-12 hours)
   - Task 5: Add Godoc Comments (2-3h)
   - Task 6: Create Developer Guide (3-4h)
   - Task 9: Update README (1-2h)
   - Task 7: Add Integration Tests (2-3h)

3. **Week 3 - Polish** (5-7 hours)
   - Task 8: Improve Error Test Coverage (2-3h)
   - Task 11: Make BulkQueue Internal (1h)
   - Task 12: Add Benchmark Tests (2h)
   - Task 10: Add context.Context (optional) (2-3h)

---

## Definition of Done for Phase 1

Phase 1 can be considered **complete** when:

### Functional Requirements
- ✅ Parse() correctly parses port specs including flavors
- ✅ Resolve() builds complete dependency graph
- ✅ TopoOrder() returns valid build order with cycle detection
- ✅ All existing commands work with the API

### Architectural Requirements
- ✅ Package struct contains ONLY metadata (no build state) - **Task 1 COMPLETE**
- ✅ CRC/build concerns in separate package - **Task 2 COMPLETE**
- ✅ Clean API with typed errors - **Task 3 COMPLETE**
- ❌ No global state in pkg package - **Task 4 remaining**

### Quality Requirements
- ❌ Comprehensive godoc comments
- ❌ Developer guide exists
- ❌ >80% test coverage
- ❌ Integration tests pass
- ❌ Error paths tested

### Documentation Requirements
- ❌ README documents library usage
- ❌ Developer guide with examples
- ❌ PHASE_1_LIBRARY.md reflects reality
- ❌ API examples are tested

---

## Notes

- Keep backward compatibility where possible
- Deprecate old functions rather than removing them immediately
- Run tests after each task
- Commit work incrementally (each completed task = 1 commit)
- Update PHASE_1_LIBRARY.md status as tasks complete
- Document any deviations from this plan

---

**Last Updated**: 2025-11-25  
**Next Review**: After completing Week 1 tasks
