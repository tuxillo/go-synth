# Phase 3: Builder Orchestration - Task Breakdown

**Status**: 🟡 In Progress  
**Last Updated**: 2025-11-27  
**Estimated Total**: 16 hours (5 hours remaining: integration tests + docs)

## Overview

Phase 3 integrates builddb (CRC-based incremental builds) with the existing builder orchestration code. The existing builder (~705 lines) already has worker pool, dependency ordering, and mount management. Phase 3 adds CRC checking, build record tracking, and database updates.

**Note**: Tasks 2-4 were already implemented during Phase 2 (commits 65ccadd, b9d9d41). Phase 3 Task 1 completes the integration by adding pre-build CRC checking.

## Task Progress: 4/6 Complete (67%)

### ✅ Completed: 4 tasks
- Task 1: Pre-Build CRC Check Integration (502fae3)
- Task 2: Build Record Lifecycle Tracking (65ccadd - Phase 2)
- Task 3: CRC and Package Index Update (65ccadd, b9d9d41 - Phase 2)
- Task 4: Error Handling and Logging (Phase 2)

### 🚧 In Progress: 0 tasks
- None

### ❌ Remaining: 2 tasks
1. Pre-Build CRC Check Integration (3h)
2. Build Record Lifecycle Tracking (4h)
3. CRC and Package Index Update (2h)
4. Error Handling and Logging (2h)
5. Integration Tests (3h)
6. Documentation and Examples (2h)

---

## Task 1: Pre-Build CRC Check Integration ✅

**Priority**: 🔴 High  
**Effort**: 3 hours  
**Status**: ✅ Complete  
**Commit**: 502fae3

### Objective
Skip unchanged ports before queuing them for build by checking CRC.

### Implementation Steps

1. **Add CRC check in DoBuild()** (build/build.go, after line 127)
   ```go
   // After getting buildOrder, before queueing packages
   for _, p := range buildOrder {
       // Skip if already successful/ignored (existing logic)
       if ctx.registry.HasAnyFlags(p, pkg.PkgFSuccess|pkg.PkgFNoBuildIgnore|pkg.PkgFIgnored) {
           // ... existing skip logic
           continue
       }
       
       // NEW: Compute CRC and check if build needed
       portPath := filepath.Join(cfg.DPortsPath, p.PortDir)
       currentCRC, err := builddb.ComputePortCRC(portPath)
       if err != nil {
           logger.Error(fmt.Sprintf("Failed to compute CRC for %s: %v", p.PortDir, err))
           // Continue with build (fail-safe)
           ctx.queue <- p
           continue
       }
       
       needsBuild, err := ctx.buildDB.NeedsBuild(p.PortDir, currentCRC)
       if err != nil {
           logger.Error(fmt.Sprintf("Failed to check NeedsBuild for %s: %v", p.PortDir, err))
           // Continue with build (fail-safe)
           ctx.queue <- p
           continue
       }
       
       if !needsBuild {
           // CRC matches, skip this build
           ctx.registry.AddFlags(p, pkg.PkgFSuccess)
           ctx.statsMu.Lock()
           ctx.stats.Skipped++
           ctx.statsMu.Unlock()
           logger.Success(fmt.Sprintf("%s (CRC match, skipped)", p.PortDir))
           continue
       }
       
       // CRC mismatch or no stored CRC, queue for build
       ctx.queue <- p
   }
   ```

2. **Import required packages** (if not already imported)
   - `filepath` (already imported)
   - `dsynth/builddb` (already imported)

3. **Update stats initialization**
   - Ensure `ctx.stats.Total` only counts packages that need building
   - Adjust counting logic to account for CRC-skipped packages

### Files to Modify
- `build/build.go` - Lines 125-147 (queueing logic in DoBuild)

### Testing Checklist
- [ ] Build a port successfully
- [ ] Run build again without changes
- [ ] Verify port is skipped (log shows "CRC match, skipped")
- [ ] Verify stats.Skipped increments correctly
- [ ] Verify stats.Total reflects actual builds (not skips)
- [ ] Modify port Makefile and rebuild
- [ ] Verify port is rebuilt (CRC mismatch detected)

### Success Criteria
- ✅ Unchanged ports are skipped before queueing
- ✅ Stats.Skipped counter accurately reflects CRC-based skips
- ✅ Logs clearly indicate when ports are skipped
- ✅ CRC computation errors don't cause build orchestration to fail

### Dependencies
- ✅ Phase 2 complete (builddb.ComputePortCRC, builddb.NeedsBuild available)

---

## Task 2: Build Record Lifecycle Tracking ✅

**Priority**: 🔴 High  
**Effort**: 4 hours  
**Status**: ✅ Complete (Phase 2)  
**Commit**: 65ccadd

### Objective
Track each build with UUID, status transitions, and timestamps in builddb.

### Implementation Steps

1. **Add UUID generation in buildPackage()** (build/build.go, line 202)
   ```go
   func (ctx *BuildContext) buildPackage(worker *Worker, p *pkg.Package) bool {
       // Generate UUID for this build
       buildUUID := uuid.New().String()
       
       // Create package logger
       logger := log.NewPackageLogger(ctx.cfg.LogDir, p.PortDir)
       defer logger.Close()
       
       logger.Info(fmt.Sprintf("Starting build %s for %s@%s", buildUUID, p.PortDir, p.Version))
       
       // Save initial record with status="running"
       startTime := time.Now()
       rec := &builddb.BuildRecord{
           UUID:      buildUUID,
           PortDir:   p.PortDir,
           Version:   p.Version,
           Status:    "running",
           StartTime: startTime,
       }
       
       if err := ctx.buildDB.SaveRecord(rec); err != nil {
           logger.Error(fmt.Sprintf("Failed to save build record: %v", err))
           // Continue anyway (fail-safe)
       }
       
       // ... existing build phase execution ...
   ```

2. **Update status at end of buildPackage()** (build/build.go, after line 270)
   ```go
       // After all phases complete
       endTime := time.Now()
       finalStatus := "success"
       if !success {
           finalStatus = "failed"
       }
       
       // Update record status
       if err := ctx.buildDB.UpdateRecordStatus(buildUUID, finalStatus, endTime); err != nil {
           logger.Error(fmt.Sprintf("Failed to update build record: %v", err))
           // Continue anyway (fail-safe)
       }
       
       logger.Info(fmt.Sprintf("Build %s completed with status: %s (duration: %s)",
           buildUUID, finalStatus, formatDuration(endTime.Sub(startTime))))
       
       return success
   }
   ```

3. **Add buildUUID to context or worker** (optional, for tracking)
   - Consider adding `CurrentBuildUUID string` to `Worker` struct
   - Allows monitoring and debugging of in-progress builds

### Files to Modify
- `build/build.go` - buildPackage() method (lines 202-280)

### Testing Checklist
- [ ] Build a port successfully
- [ ] Check database for record with status="success"
- [ ] Verify UUID is logged in package log
- [ ] Verify timestamps are accurate
- [ ] Force a build to fail (invalid patch)
- [ ] Check database for record with status="failed"
- [ ] Verify EndTime is set for both success and failed
- [ ] Run multiple builds concurrently
- [ ] Verify each has unique UUID

### Success Criteria
- Every build creates a record in builddb
- Status transitions correctly: running → success/failed
- Timestamps accurately reflect build start and end times
- UUIDs are unique and properly logged
- Database queries return correct records

### Dependencies
- ✅ Phase 2 complete (builddb.SaveRecord, builddb.UpdateRecordStatus available)
- ✅ uuid package already imported in build/build.go

---

## Task 3: CRC and Package Index Update on Success ✅

**Priority**: 🔴 High  
**Effort**: 2 hours  
**Status**: ✅ Complete (Phase 2)  
**Commits**: 65ccadd, b9d9d41

### Objective
Update CRC and package index after successful builds to enable future CRC-based skips.

### Implementation Steps

1. **Add CRC update after successful build** (build/build.go, in buildPackage after line 181)
   ```go
       // After marking success (around line 181)
       if success {
           ctx.stats.Success++
           ctx.registry.AddFlags(p, pkg.PkgFSuccess)
           ctx.registry.ClearFlags(p, pkg.PkgFRunning)
           ctx.logger.Success(p.PortDir)
           
           // NEW: Update CRC and package index
           portPath := filepath.Join(ctx.cfg.DPortsPath, p.PortDir)
           finalCRC, err := builddb.ComputePortCRC(portPath)
           if err != nil {
               logger.Error(fmt.Sprintf("Failed to compute final CRC for %s: %v", p.PortDir, err))
           } else {
               // Update CRC index
               if err := ctx.buildDB.UpdateCRC(p.PortDir, finalCRC); err != nil {
                   logger.Error(fmt.Sprintf("Failed to update CRC for %s: %v", p.PortDir, err))
               } else {
                   logger.Info(fmt.Sprintf("Updated CRC for %s: %08x", p.PortDir, finalCRC))
               }
               
               // Update package index
               if err := ctx.buildDB.UpdatePackageIndex(p.PortDir, p.Version, buildUUID); err != nil {
                   logger.Error(fmt.Sprintf("Failed to update package index for %s: %v", p.PortDir, err))
               } else {
                   logger.Info(fmt.Sprintf("Updated package index: %s@%s -> %s", p.PortDir, p.Version, buildUUID))
               }
           }
       } else {
           // Failed build - do NOT update CRC or package index
           ctx.stats.Failed++
           ctx.registry.AddFlags(p, pkg.PkgFFailed)
           ctx.registry.ClearFlags(p, pkg.PkgFRunning)
           ctx.logger.Failed(p.PortDir, ctx.registry.GetLastPhase(p))
       }
   ```

2. **Ensure failed builds don't update CRC**
   - Verify CRC update only happens in success branch
   - Add explicit logging for failed builds (no CRC update)

### Files to Modify
- `build/build.go` - buildPackage() method, success/failure branches (lines 178-190)

### Testing Checklist
- [ ] Build a port successfully
- [ ] Check CRC is stored in database
- [ ] Check package index points to build UUID
- [ ] Build same port again
- [ ] Verify it's skipped (Task 1 integration)
- [ ] Force a build to fail
- [ ] Verify CRC is NOT updated
- [ ] Verify package index is NOT updated
- [ ] Fix the build and rebuild
- [ ] Verify CRC and index are updated on success

### Success Criteria
- CRC updated only on successful builds
- Package index points to latest successful build
- Failed builds don't corrupt CRC or index
- Next build of unchanged port is skipped (Task 1 validation)
- Logs clearly show CRC updates

### Dependencies
- ✅ Phase 2 complete (builddb.UpdateCRC, builddb.UpdatePackageIndex available)
- ⚠️ Task 2 (buildUUID variable needed for package index)

---

## Task 4: Error Handling and Logging ✅

**Priority**: 🟡 Medium  
**Effort**: 2 hours  
**Status**: ✅ Complete (Phase 2)  
**Note**: Implemented alongside Tasks 2-3

### Objective
Add structured error handling for all builddb operations with fail-safe behavior.

### Implementation Steps

1. **Review builddb error types** (from Phase 2)
   - DatabaseError
   - RecordError
   - PackageIndexError
   - CRCError
   - ValidationError

2. **Add error inspection and logging** (all builddb calls)
   ```go
   if err := ctx.buildDB.SaveRecord(rec); err != nil {
       // Check for specific error types
       var dbErr *builddb.DatabaseError
       var recErr *builddb.RecordError
       
       if errors.As(err, &dbErr) {
           logger.Error(fmt.Sprintf("Database error saving record: %v", dbErr))
       } else if errors.As(err, &recErr) {
           logger.Error(fmt.Sprintf("Record error: %v", recErr))
       } else {
           logger.Error(fmt.Sprintf("Failed to save build record: %v", err))
       }
       
       // Continue anyway (fail-safe)
   }
   ```

3. **Add debug logging for CRC values**
   ```go
   logger.Debug(fmt.Sprintf("Computed CRC for %s: %08x", p.PortDir, currentCRC))
   
   if !needsBuild {
       logger.Debug(fmt.Sprintf("CRC match for %s (stored: %08x, current: %08x)", 
           p.PortDir, storedCRC, currentCRC))
   }
   ```

4. **Ensure no panics on builddb errors**
   - Wrap all builddb calls in error checks
   - Never panic on builddb failures
   - Log errors but continue with build orchestration

5. **Add error metrics** (optional)
   - Track builddb error counts in BuildStats
   - Add `DBErrors int` field to BuildStats

### Files to Modify
- `build/build.go` - All builddb call sites (Tasks 1, 2, 3)

### Testing Checklist
- [ ] Simulate database error (invalid path)
- [ ] Verify build continues despite error
- [ ] Verify errors are logged appropriately
- [ ] Check for structured error types in logs
- [ ] Verify no panics occur
- [ ] Test with missing database file
- [ ] Test with corrupted database
- [ ] Verify fail-safe behavior in all cases

### Success Criteria
- All builddb errors are caught and logged
- Build orchestration never fails due to builddb errors
- Logs provide useful debugging information
- Structured error types are properly handled
- No panics or crashes on builddb failures

### Dependencies
- ✅ Phase 2 complete (builddb structured errors available)
- ⚠️ Tasks 1-3 (builddb call sites)

---

## Task 5: Integration Tests

**Priority**: 🔴 High  
**Effort**: 3 hours  
**Status**: ❌ Not Started

### Objective
Validate end-to-end CRC skip mechanism with integration tests.

### Test Scenarios

#### Test 1: First Build Workflow
```go
func TestIntegration_FirstBuildWorkflow(t *testing.T) {
    // Build a small port (e.g., misc/help2man)
    // Verify:
    // - Build succeeds
    // - Build record created in database (status="success")
    // - CRC stored in database
    // - Package index updated (points to build UUID)
    // - Logs show build phases executed
}
```

#### Test 2: Incremental Build (Skip)
```go
func TestIntegration_IncrementalBuildSkip(t *testing.T) {
    // Build port once (establish baseline)
    // Build same port again without changes
    // Verify:
    // - Port is skipped (CRC match)
    // - stats.Skipped increments
    // - No new build record created
    // - Log shows "CRC match, skipped"
    // - Build completes very quickly
}
```

#### Test 3: Rebuild After Change
```go
func TestIntegration_RebuildAfterChange(t *testing.T) {
    // Build port once
    // Modify port Makefile (add comment)
    // Build port again
    // Verify:
    // - Port is rebuilt (CRC mismatch)
    // - New build record created
    // - New CRC stored in database
    // - Package index updated to new UUID
    // - Logs show build phases executed
}
```

#### Test 4: Failed Build Handling
```go
func TestIntegration_FailedBuildHandling(t *testing.T) {
    // Force a build to fail (invalid patch, missing dependency)
    // Verify:
    // - Build record shows status="failed"
    // - CRC is NOT updated
    // - Package index is NOT updated
    // - Stats.Failed increments
    // - Fix the build and rebuild
    // - Verify successful build updates CRC and index
}
```

#### Test 5: Multi-Port Dependency Chain
```go
func TestIntegration_MultiPortDependencyChain(t *testing.T) {
    // Build port A with dependency port B
    // Verify both build successfully
    // Build again without changes
    // Verify both are skipped (CRC match)
    // Modify port A only
    // Verify:
    // - Port A rebuilds (CRC mismatch)
    // - Port B is skipped (CRC match)
    // - Dependency relationship respected
}
```

### Test Infrastructure

1. **Test fixtures**
   - Use small, fast-building ports (misc/help2man, textproc/jq, etc.)
   - Or create minimal test ports in testdata/
   - Ensure ports have few/no dependencies

2. **Test database**
   - Create temporary database for each test
   - Clean up after test completion
   - Use t.TempDir() for isolation

3. **Mock configuration**
   - Minimal config for test environment
   - Point to test ports tree
   - Single worker to avoid concurrency issues in tests

4. **Helper functions**
   - `setupTestBuild(t *testing.T) (*BuildContext, func())`
   - `buildPort(t *testing.T, ctx *BuildContext, portDir string)`
   - `modifyPortFile(t *testing.T, portDir, filename string)`
   - `assertBuildStats(t *testing.T, stats *BuildStats, expected BuildStats)`

### Files to Create
- `build/integration_test.go` (new file, ~400-500 lines)

### Testing Checklist
- [ ] All 5 test scenarios pass
- [ ] Tests run with `-race` (no race conditions)
- [ ] Tests are isolated (no interference between tests)
- [ ] Tests clean up properly (no leftover files)
- [ ] Tests are deterministic (no flaky behavior)
- [ ] Test execution time is reasonable (<30s total)

### Success Criteria
- All 5 integration test scenarios pass
- CRC skip mechanism validated end-to-end
- Build statistics are accurate in all scenarios
- No race conditions detected
- Tests are reliable and maintainable

### Dependencies
- ✅ Phase 2 complete (builddb fully functional)
- ⚠️ Tasks 1-3 complete (CRC integration implemented)

---

## Task 6: Documentation and Examples

**Priority**: 🟡 Medium  
**Effort**: 2 hours  
**Status**: ❌ Not Started

### Objective
Update documentation and provide usage examples for Phase 3 integration.

### Documentation Updates

1. **DEVELOPMENT.md**
   - Mark Phase 3 as complete
   - Update progress: 0% → 100%
   - Add Tasks 1-6 completion status with commit hashes
   - Update "Recent Milestones" section
   - Update "Next Milestones" to Phase 4

2. **PHASE_3_BUILDER.md**
   - Add "Implementation Details" section
   - Document actual integration points
   - Add code examples from Tasks 1-3
   - Update status to "Complete"
   - Add performance notes (CRC computation time, skip rates)

3. **Godoc comments in build/build.go**
   - Update `BuildContext` struct comment
   - Update `DoBuild()` function comment (mention CRC skip)
   - Update `buildPackage()` method comment (mention record tracking)
   - Add example usage in package-level comment

4. **Create PHASE_3_EXAMPLES.md**
   - Example 1: Simple build with CRC skip
   - Example 2: Query build records
   - Example 3: View build history
   - Example 4: Debug CRC mismatches
   - Example 5: Force rebuild (bypass CRC check)

### Code Examples

#### Example 1: Simple Build with CRC Skip
```go
// Build a port with automatic CRC-based skip
packages, _ := pkg.Parse([]string{"editors/vim"}, cfg)
pkg.Resolve(packages, cfg)

db, _ := builddb.OpenDB("~/.go-synth/builds.db")
defer db.Close()

stats, cleanup, _ := build.DoBuild(packages, cfg, logger, db)
defer cleanup()

fmt.Printf("Total: %d, Success: %d, Skipped: %d\n",
    stats.Total, stats.Success, stats.Skipped)
// Output: Total: 1, Success: 0, Skipped: 1 (on second run)
```

#### Example 2: Query Build Records
```go
// Find latest successful build for a port
rec, _ := db.LatestFor("editors/vim", "9.0.0")
if rec != nil {
    fmt.Printf("Last build: %s at %s (status: %s)\n",
        rec.UUID, rec.StartTime, rec.Status)
}
```

#### Example 3: Check if Port Needs Building
```go
// Manually check if a port needs building
crc, _ := builddb.ComputePortCRC("/usr/dports/editors/vim")
needsBuild, _ := db.NeedsBuild("editors/vim", crc)

if needsBuild {
    fmt.Println("Port has changed, rebuild needed")
} else {
    fmt.Println("Port unchanged, can skip")
}
```

### Files to Modify/Create
- `DEVELOPMENT.md` - Phase 3 status update
- `docs/design/PHASE_3_BUILDER.md` - Implementation details
- `docs/design/PHASE_3_EXAMPLES.md` - New file with examples
- `build/build.go` - Enhanced godoc comments

### Documentation Checklist
- [ ] DEVELOPMENT.md updated with Phase 3 completion
- [ ] PHASE_3_BUILDER.md updated with implementation details
- [ ] PHASE_3_EXAMPLES.md created with 5 examples
- [ ] Godoc comments enhanced in build/build.go
- [ ] All examples are tested and accurate
- [ ] Links between documents are correct
- [ ] Formatting is consistent

### Success Criteria
- Documentation clearly explains CRC integration
- Examples are practical and tested
- Phase 3 marked as complete in DEVELOPMENT.md
- Godoc is comprehensive and accurate
- Users can understand how to use Phase 3 features

### Dependencies
- ⚠️ Tasks 1-5 complete (implementation and testing done)

---

## Summary

### Total Effort: 16 hours
- Task 1: Pre-Build CRC Check (3h)
- Task 2: Build Record Lifecycle (4h)
- Task 3: CRC/Index Update (2h)
- Task 4: Error Handling (2h)
- Task 5: Integration Tests (3h)
- Task 6: Documentation (2h)

### Critical Path
Tasks 1-3 must be done sequentially (each depends on previous)
Tasks 4-6 can be done alongside or after Tasks 1-3

### Recommended Order
1. **Week 1**: Tasks 1-3 (9 hours) - Core integration
2. **Week 2**: Tasks 4-6 (7 hours) - Polish and validation

### Success Metrics
- Build same port twice → second build skipped (CRC match)
- Modify port → rebuild triggered (CRC mismatch)
- Failed builds don't update CRC/index
- All 5 integration tests pass
- Documentation complete and accurate

---

**Last Updated**: 2025-11-27  
**Phase Status**: 🔵 Ready to Start
