# Refactoring: Issue Fixes Implementation

This document tracks detailed implementation steps for fixing issues identified in [INCONSISTENCIES.md](../../INCONSISTENCIES.md).

**Last Updated**: 2025-11-29

---

## Active Refactoring: Remove stdout/stderr from Library Packages

**Issue Reference**: INCONSISTENCIES.md Pattern 1  
**Tracking**: DEVELOPMENT.md - Known Issues (Architectural/Design Critical)  
**Status**: 🔄 In Progress  
**Priority**: HIGH - Blocks Phase 5 REST API  
**Effort**: ~8 hours  
**Started**: 2025-11-29

### Problem Statement

Library packages (pkg, build, environment, migration, mount, util) print directly to stdout/stderr using `fmt.Print*`, making them unusable in non-CLI contexts (REST API, GUI, services).

**Impact**: 71 print statements across 6 packages block Phase 5 REST API implementation.

### Solution Approach

Define minimal logger interface → Add optional logger parameters → Update all library functions → Progressive migration.

---

## Implementation Plan

### Stage 1: Foundation (1.5 hours)

#### Task 1.1: Define Logger Interface ✅ COMPLETE
**File**: `log/interface.go`  
**Time**: 30 minutes  
**Status**: ✅ Complete

Create minimal `LibraryLogger` interface with implementations:
- `LibraryLogger` interface (Info, Debug, Warn, Error)
- `NoOpLogger` (silent mode)
- `StdoutLogger` (CLI debugging)

**Validation**:
- [x] Interface compiles
- [x] NoOpLogger implements interface
- [x] StdoutLogger implements interface

---

#### Task 1.2: Make log.Logger Implement Interface ✅ COMPLETE
**File**: `log/logger.go`  
**Time**: 30 minutes  
**Status**: ✅ Complete

Add methods so existing `Logger` satisfies `LibraryLogger` interface.

**Changes**:
- ✅ Add compile-time interface check: `var _ LibraryLogger = (*Logger)(nil)`
- ✅ Updated Info() to accept `format string, args ...any`
- ✅ Updated Debug() to accept `format string, args ...any`
- ✅ Updated Error() to accept `format string, args ...any`
- ✅ Added Warn() method (writes to results + debug logs)

**Validation**:
- [x] Logger satisfies LibraryLogger interface
- [x] Existing code still compiles (variadic args are backward-compatible)
- [x] Unit test: Logger.Info() works with formatting
- [x] Unit test: Logger.Warn() writes to correct files
- [x] All 44 existing tests pass

---

#### Task 1.3: Create Test Helper ✅ COMPLETE
**File**: `log/testing.go`, `log/testing_test.go`  
**Time**: 30 minutes  
**Status**: ✅ Complete

Create `MemoryLogger` for testing with message capture and query methods.

**Implementation**:
- ✅ Created `MemoryLogger` struct with thread-safe message capture
- ✅ Implements LibraryLogger interface
- ✅ Added `LogMessage` struct (Level + Message)
- ✅ Query methods: `GetMessages()`, `GetMessagesByLevel()`
- ✅ Search methods: `HasMessage()`, `HasMessageWithLevel()`
- ✅ Utility methods: `Clear()`, `Count()`, `CountByLevel()`, `String()`

**Validation**:
- [x] MemoryLogger implements LibraryLogger
- [x] Unit test: Can capture and query messages
- [x] Unit test: Formatting works correctly
- [x] Unit test: Thread-safe for concurrent use
- [x] All 11 tests pass

---

### Stage 1 Summary ✅ COMPLETE
**Total Time**: 1.5 hours  
**Files Created**: 
- `log/interface.go` (LibraryLogger interface + NoOpLogger + StdoutLogger)
- `log/testing.go` (MemoryLogger for tests)
- `log/testing_test.go` (11 tests)

**Files Modified**:
- `log/logger.go` (Updated to implement LibraryLogger)
- `log/logger_test.go` (Added 2 new tests)

**Status**: Foundation complete, ready for package migration

---

### Stage 2: Migration Package (1 hour) ✅ COMPLETE

**Why start here**: Smallest package (8 print statements), no complex dependencies

#### Task 2.1: Update migration.MigrateLegacyCRC ✅ COMPLETE
**File**: `migration/migration.go`  
**Time**: 30 minutes  
**Status**: ✅ Complete

**Changes**:
- ✅ Added logger parameter with minimal interface (Info + Warn)
- ✅ Replaced all 8 `fmt.Printf` / `fmt.Fprintf(os.Stderr)` calls with logger methods
- ✅ Updated `readLegacyCRCFile` to accept logger for warnings

**Validation**:
- [x] All 8 print statements replaced
- [x] Function compiles
- [x] Tests pass

---

#### Task 2.2: Update Callers ✅ COMPLETE
**Files**: `main.go` (doInit, doBuild)  
**Time**: 20 minutes  
**Status**: ✅ Complete

Pass logger to MigrateLegacyCRC calls in main.go.

**Changes**:
- ✅ doInit: Pass `log.StdoutLogger{}` (user-facing CLI, 2 call sites)
- ✅ doBuild: Pass `logger` (existing Logger instance, 2 call sites)

**Validation**:
- [x] main.go compiles
- [x] Migration still works (tested via existing tests)
- [x] No stdout/stderr output from migration package (only through logger)

---

#### Task 2.3: Add Tests ✅ COMPLETE
**File**: `migration/migration_test.go`  
**Time**: 10 minutes  
**Status**: ✅ Complete

Add test using MemoryLogger to verify log messages.

**Changes**:
- ✅ Created testLogger wrapper for *testing.T (logs via t.Logf)
- ✅ Updated all 7 existing tests to pass logger (NoOpLogger or testLogger)
- ✅ Added new test: `TestMigrateLegacyCRC_LogCapture` using MemoryLogger
- ✅ Test validates exact log message counts and content

**Validation**:
- [x] Test passes
- [x] Captures expected log messages (4 INFO, 2 WARN, 0 ERROR, 0 DEBUG)
- [x] All 8 migration tests pass

---

### Stage 2 Summary ✅ COMPLETE
**Total Time**: 1 hour  
**Print statements removed**: 8 (migration package now clean)

**Files Modified**:
- `migration/migration.go` - Added logger parameter to 2 functions
- `main.go` - Updated 4 call sites (2 with StdoutLogger, 2 with Logger)
- `migration/migration_test.go` - Updated 7 tests + added 1 new MemoryLogger test

**Status**: Migration package complete, no stdout/stderr remaining

---

### Stage 3: pkg Package (3 hours) ⏸️ PENDING

**Why next**: Most print statements (38), core dependency

#### Task 3.1: Update resolveDependencies
**Time**: 45 minutes  
**Status**: ⏸️ Pending

#### Task 3.2: Update buildDependencyGraph
**Time**: 30 minutes  
**Status**: ⏸️ Pending

#### Task 3.3: Update GetBuildOrder
**Time**: 30 minutes  
**Status**: ⏸️ Pending

#### Task 3.4: Update MarkPackagesNeedingBuild
**Time**: 45 minutes  
**Status**: ⏸️ Pending

#### Task 3.5: Update Public API Functions
**Time**: 30 minutes  
**Status**: ⏸️ Pending

---

### Stage 4: build Package (2 hours) ⏸️ PENDING

#### Task 4.1: Update DoBuild
**Time**: 45 minutes  
**Status**: ⏸️ Pending

#### Task 4.2: Update DoFetch
**Time**: 30 minutes  
**Status**: ⏸️ Pending

#### Task 4.3: Update Callers
**Time**: 45 minutes  
**Status**: ⏸️ Pending

---

### Stage 5: Remaining Packages (1.5 hours) ⏸️ PENDING

#### Task 5.1: util Package
**Time**: 30 minutes  
**Status**: ⏸️ Pending

**Decision needed**: Move AskYN to main.go (it's CLI-specific)

#### Task 5.2: mount Package
**Time**: 30 minutes  
**Status**: ⏸️ Pending

**Decision**: Skip (package is deprecated, will be removed)

#### Task 5.3: environment Package
**Time**: 30 minutes  
**Status**: ⏸️ Pending

Replace global log package usage in environment/bsd.

---

### Stage 6: Testing & Validation (1 hour) ⏸️ PENDING

#### Task 6.1: Unit Tests
**Time**: 30 minutes  
**Status**: ⏸️ Pending

#### Task 6.2: Integration Test
**Time**: 30 minutes  
**Status**: ⏸️ Pending

---

### Stage 7: Documentation & Cleanup (30 min) ⏸️ PENDING

#### Task 7.1: Update DEVELOPMENT.md
**Time**: 15 minutes  
**Status**: ⏸️ Pending

#### Task 7.2: Update INCONSISTENCIES.md
**Time**: 15 minutes  
**Status**: ⏸️ Pending

---

## Progress Summary

**Overall**: 0/7 stages complete (0%)  
**Current Stage**: Stage 1 (Foundation)  
**Current Task**: Task 1.1 (Define Logger Interface)

**Completed Tasks**: 0/19  
**Time Spent**: 0 hours  
**Time Remaining**: ~8 hours

---

## Acceptance Criteria

### Must Have ✅
- [ ] Zero `fmt.Print*` calls in pkg/, build/, migration/ packages
- [ ] All affected functions take `log.LibraryLogger` parameter
- [ ] Existing tests pass with logger parameter
- [ ] Manual build test (print/indexinfo) works silently

### Should Have 📋
- [ ] New unit tests for logger integration
- [ ] Documentation updated
- [ ] No global log package usage in libraries

### Nice to Have 🎯
- [ ] Configurable verbosity levels
- [ ] Structured logging (JSON output option)
- [ ] Performance: no string formatting unless logging enabled

---

## Risk Mitigation

### Risk 1: Breaking Changes
**Mitigation**: Add logger as last parameter, provide default in wrapper functions

### Risk 2: Missed Print Statements
**Mitigation**: Use grep to verify:
```bash
rg "fmt\.(Print|Fprintf\(os\.Std)" pkg/ build/ migration/ --type go
```

### Risk 3: Performance Impact
**Mitigation**: NoOpLogger has zero overhead, benchmark if needed

### Risk 4: Lost Progress Visibility
**Mitigation**: StdoutLogger for CLI, file logger for production

---

## Success Metrics

After completion:
- ✅ 71 print statements removed from library code
- ✅ 6 packages now reusable in any context
- ✅ Phase 5 REST API unblocked
- ✅ Test coverage maintained or improved
- ✅ Zero breaking changes to public APIs
