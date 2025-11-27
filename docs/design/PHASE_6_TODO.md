# Phase 6: Testing Strategy - Comprehensive Coverage

**Phase**: 6 of 7  
**Status**: 🟡 Partially Complete (78% coverage, needs completion)  
**Dependencies**: Phases 1-3 complete (✅), Phase 4 in progress  
**Estimated Effort**: ~8 hours (remaining work)  
**Priority**: High (Quality assurance)

## Overview

Phase 6 ensures comprehensive test coverage across all packages. **Important**: The project 
already has substantial test coverage (~78%, 4,875 test lines), but several packages lack 
tests entirely, and some test coverage gaps remain.

### Current State (Actual Analysis)

| Package | Implementation | Tests | Test Files | Coverage Status |
|---------|---------------|-------|------------|----------------|
| `pkg/` | 2,010 lines | 2,313 lines | 9 files | ✅ **Excellent** (115%) |
| `builddb/` | 842 lines | 2,120 lines | 3 files | ✅ **Excellent** (252%) |
| `build/` | 811 lines | 442 lines | 1 file | 🟡 **Good** (54%, needs more) |
| `mount/` | 293 lines | 0 lines | 0 files | ❌ **Missing** (Phase 4 → environment) |
| `config/` | 218 lines | 0 lines | 0 files | ❌ **Missing** |
| `log/` | 674 lines | 0 lines | 0 files | ❌ **Missing** |
| **Total** | **6,221 lines** | **4,875 lines** | **13 files** | 🟡 **78%** |

### Goals

1. **Achieve >80% coverage** across all packages
2. **Add missing tests** for config, log, and expanded build tests
3. **Maintain existing excellent coverage** in pkg and builddb
4. **Ensure CI/CD integration** with race detector
5. **Document testing approach** for future contributors

### Non-Goals (MVP)

- ❌ Performance benchmarks (optional enhancement)
- ❌ Chaos testing / fault injection
- ❌ Full E2E test matrix (covered by integration tests)
- ❌ 100% coverage (diminishing returns, target is >80%)

---

## Implementation Tasks

### Task 1: Add Build Package Tests ⚪

**Estimated Time**: 2 hours  
**Priority**: High  
**Current Coverage**: 54% (442/811 lines)  
**Target Coverage**: >80%

#### Description

Expand build package testing to cover worker lifecycle, concurrent builds, error propagation, 
and cleanup scenarios. Current integration tests cover happy path but lack edge cases.

#### Implementation Steps

1. **Create `build/build_test.go`** for unit tests:
   ```go
   package build_test
   
   import (
       "testing"
       "dsynth/build"
       "dsynth/builddb"
       "dsynth/config"
       "dsynth/log"
       "dsynth/pkg"
   )
   
   func TestBuildContext(t *testing.T) {
       tests := []struct {
           name string
           packages []*pkg.Package
           wantErr bool
       }{
           {
               name: "empty package list",
               packages: []*pkg.Package{},
               wantErr: false, // Should succeed with 0 builds
           },
           {
               name: "nil packages",
               packages: nil,
               wantErr: true,
           },
       }
       
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               cfg := &config.Config{/* test config */}
               logger, _ := log.NewLogger(cfg)
               db, _ := builddb.OpenDB(t.TempDir() + "/test.db")
               defer db.Close()
               
               _, cleanup, err := build.DoBuild(tt.packages, cfg, logger, db)
               defer cleanup()
               
               if (err != nil) != tt.wantErr {
                   t.Errorf("DoBuild() error = %v, wantErr %v", err, tt.wantErr)
               }
           })
       }
   }
   
   func TestBuildStats(t *testing.T) {
       // Test BuildStats accumulation
       // Test concurrent stats updates
       // Test final stats calculation
   }
   
   func TestWorkerLifecycle(t *testing.T) {
       // Test worker initialization
       // Test worker assignment
       // Test worker cleanup on error
   }
   ```

2. **Add error propagation tests** in `build/integration_test.go`:
   ```go
   func TestDependencyFailurePropagation(t *testing.T) {
       if os.Getuid() != 0 {
           t.Skip("requires root for mount operations")
       }
       
       // Setup: Create dependency chain A → B → C
       // Force B to fail
       // Assert C is marked as failed (dependency failed)
       // Assert A succeeds
   }
   
   func TestConcurrentBuilds(t *testing.T) {
       // Test multiple independent packages build in parallel
       // Assert worker pool utilization
       // Verify no race conditions
   }
   
   func TestCleanupOnPanic(t *testing.T) {
       // Simulate panic during build
       // Assert cleanup() still unmounts workers
       // Assert database consistency
   }
   ```

3. **Add CRC skip tests**:
   ```go
   func TestCRCSkipLogic(t *testing.T) {
       // First build: package builds and CRC stored
       // Second build: same package skipped (CRC match)
       // Modify package: rebuilds (CRC mismatch)
       // Assert stats.Skipped increments correctly
   }
   
   func TestForceRebuild(t *testing.T) {
       // Build once, CRC stored
       // Force rebuild bypasses CRC check
       // Assert package rebuilds despite CRC match
   }
   ```

#### Testing Checklist

- [ ] Worker lifecycle tested (create, assign, cleanup)
- [ ] Empty/nil package lists handled
- [ ] Dependency failure propagation verified
- [ ] Concurrent build correctness
- [ ] Cleanup on error/panic
- [ ] CRC skip logic validated
- [ ] Force rebuild bypasses CRC
- [ ] Stats accumulation correct
- [ ] Coverage >80% for build package

---

### Task 2: Add Config Package Tests ⚪

**Estimated Time**: 1.5 hours  
**Priority**: High  
**Current Coverage**: 0% (0/218 lines)  
**Target Coverage**: >80%

#### Description

Add comprehensive tests for configuration loading, validation, and defaults.

#### Implementation Steps

1. **Create `config/config_test.go`**:
   ```go
   package config_test
   
   import (
       "os"
       "path/filepath"
       "testing"
       
       "dsynth/config"
   )
   
   func TestLoadConfig(t *testing.T) {
       tests := []struct {
           name    string
           content string
           wantErr bool
       }{
           {
               name: "valid minimal config",
               content: `{
                   "profiles": {
                       "default": {
                           "num_workers": 4,
                           "packages_dir": "/usr/dports"
                       }
                   }
               }`,
               wantErr: false,
           },
           {
               name: "invalid JSON",
               content: `{invalid json}`,
               wantErr: true,
           },
           {
               name: "missing required fields",
               content: `{"profiles": {}}`,
               wantErr: true,
           },
       }
       
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               tmpDir := t.TempDir()
               cfgPath := filepath.Join(tmpDir, "config.json")
               os.WriteFile(cfgPath, []byte(tt.content), 0644)
               
               cfg, err := config.LoadConfig(cfgPath, "default")
               
               if (err != nil) != tt.wantErr {
                   t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
               }
               
               if !tt.wantErr && cfg == nil {
                   t.Error("Expected non-nil config")
               }
           })
       }
   }
   
   func TestConfigDefaults(t *testing.T) {
       // Test default values applied correctly
       cfg := &config.Config{}
       config.ApplyDefaults(cfg)
       
       if cfg.System.NumWorkers == 0 {
           t.Error("Expected default num_workers > 0")
       }
   }
   
   func TestProfileSelection(t *testing.T) {
       // Test loading specific profile
       // Test fallback to default profile
       // Test error when profile not found
   }
   
   func TestConfigValidation(t *testing.T) {
       // Test validation of required fields
       // Test validation of paths (must exist)
       // Test validation of numeric ranges (workers > 0)
   }
   ```

2. **Add environment variable tests**:
   ```go
   func TestEnvVarOverrides(t *testing.T) {
       // Test environment variables override config file
       os.Setenv("DSYNTH_NUM_WORKERS", "8")
       defer os.Unsetenv("DSYNTH_NUM_WORKERS")
       
       // Load config and assert num_workers = 8
   }
   ```

#### Testing Checklist

- [ ] LoadConfig with valid JSON succeeds
- [ ] LoadConfig with invalid JSON fails
- [ ] Missing required fields cause error
- [ ] Default values applied correctly
- [ ] Profile selection works
- [ ] Non-existent profile returns error
- [ ] Path validation (directories exist)
- [ ] Numeric validation (workers > 0)
- [ ] Environment variable overrides
- [ ] Coverage >80% for config package

---

### Task 3: Add Log Package Tests ⚪

**Estimated Time**: 1.5 hours  
**Priority**: Medium  
**Current Coverage**: 0% (0/674 lines)  
**Target Coverage**: >70%

#### Description

Add tests for log package functionality including file rotation, log levels, and formatting.

#### Implementation Steps

1. **Create `log/log_test.go`**:
   ```go
   package log_test
   
   import (
       "io/ioutil"
       "os"
       "path/filepath"
       "strings"
       "testing"
       
       "dsynth/config"
       "dsynth/log"
   )
   
   func TestNewLogger(t *testing.T) {
       tmpDir := t.TempDir()
       cfg := &config.Config{
           System: config.SystemConfig{
               LogDir: tmpDir,
           },
       }
       
       logger, err := log.NewLogger(cfg)
       if err != nil {
           t.Fatalf("NewLogger() failed: %v", err)
       }
       defer logger.Close()
       
       if logger == nil {
           t.Error("Expected non-nil logger")
       }
   }
   
   func TestLogLevels(t *testing.T) {
       tmpDir := t.TempDir()
       logFile := filepath.Join(tmpDir, "test.log")
       
       logger, _ := log.NewLogger(&config.Config{
           System: config.SystemConfig{LogDir: tmpDir},
       })
       defer logger.Close()
       
       // Write logs at different levels
       logger.Debug("debug message")
       logger.Info("info message")
       logger.Warn("warning message")
       logger.Error("error message")
       
       // Read log file
       content, _ := ioutil.ReadFile(logFile)
       logContent := string(content)
       
       // Assert messages present (or absent if level filtered)
       if !strings.Contains(logContent, "info message") {
           t.Error("Expected info message in log")
       }
   }
   
   func TestLogFormatting(t *testing.T) {
       // Test timestamp format
       // Test log level prefix
       // Test message formatting with printf-style args
   }
   
   func TestConcurrentLogging(t *testing.T) {
       tmpDir := t.TempDir()
       logger, _ := log.NewLogger(&config.Config{
           System: config.SystemConfig{LogDir: tmpDir},
       })
       defer logger.Close()
       
       // Launch multiple goroutines logging concurrently
       // Assert no data races
       // Assert all messages written
   }
   ```

2. **Add file rotation tests**:
   ```go
   func TestLogRotation(t *testing.T) {
       // Write logs exceeding rotation size
       // Assert old log renamed with timestamp
       // Assert new log created
   }
   ```

#### Testing Checklist

- [ ] NewLogger creates log file
- [ ] Log levels filter correctly
- [ ] Timestamp format correct
- [ ] Printf-style formatting works
- [ ] Concurrent logging safe (no races)
- [ ] Log file rotation works
- [ ] Close() flushes pending writes
- [ ] Coverage >70% for log package

---

### Task 4: Add Mount Package Tests (Integration with Phase 4) ⚪

**Estimated Time**: 1 hour  
**Priority**: Medium (Phase 4 creates environment package)  
**Current Coverage**: 0% (0/293 lines, will be moved to environment/)

#### Description

Since Phase 4 extracts mount logic into environment package, add tests for the legacy 
mount package and ensure the new environment package has proper test coverage.

#### Implementation Steps

1. **Create `mount/mount_test.go`** (temporary, for Phase 4 transition):
   ```go
   //go:build integration
   // +build integration
   
   package mount_test
   
   import (
       "os"
       "testing"
       
       "dsynth/config"
       "dsynth/mount"
   )
   
   func TestWorkerMountSetup(t *testing.T) {
       if os.Getuid() != 0 {
           t.Skip("requires root for mount operations")
       }
       
       cfg := &config.Config{
           System: config.SystemConfig{
               BuildBase:    t.TempDir(),
               PackagesDir:  "/usr/dports",
               DistfilesDir: "/usr/distfiles",
           },
       }
       
       worker, err := mount.SetupWorkerMounts(0, cfg)
       if err != nil {
           t.Fatalf("SetupWorkerMounts() failed: %v", err)
       }
       defer mount.DoWorkerUnmounts(worker, cfg)
       
       // Assert mount points exist
       // Assert template copied
   }
   
   func TestMountCleanup(t *testing.T) {
       // Test unmount success
       // Test unmount retry logic
       // Test cleanup with busy mount (should retry)
   }
   ```

2. **Add to Phase 4 environment tests** (in `environment/bsd/bsd_test.go`):
   ```go
   func TestEnvironmentSetup(t *testing.T) {
       // Test Setup() creates all mount points
       // Test Execute() runs command in chroot
       // Test Cleanup() removes all mounts
   }
   ```

#### Testing Checklist

- [ ] Worker mount setup creates all mount points
- [ ] Template copying works
- [ ] Cleanup retries on busy mounts
- [ ] Integration with Phase 4 environment tests
- [ ] Legacy mount package tests pass
- [ ] Coverage >70% for mount (pre-Phase 4)

---

### Task 5: CI/CD Integration ⚪

**Estimated Time**: 1.5 hours  
**Priority**: High  
**Dependencies**: Tasks 1-4

#### Description

Set up continuous integration to run tests on every PR with race detector and coverage reporting.

#### Implementation Steps

1. **Create `.github/workflows/test.yml`**:
   ```yaml
   name: Tests
   
   on:
     push:
       branches: [ main ]
     pull_request:
       branches: [ main ]
   
   jobs:
     unit-tests:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4
         
         - name: Set up Go
           uses: actions/setup-go@v4
           with:
             go-version: '1.22'
         
         - name: Run unit tests
           run: go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
         
         - name: Upload coverage
           uses: codecov/codecov-action@v3
           with:
             file: ./coverage.txt
             flags: unittests
     
     integration-tests:
       runs-on: ubuntu-latest
       # Note: Some integration tests require root (mount operations)
       # These may need to run in a VM or be skipped in CI
       steps:
         - uses: actions/checkout@v4
         
         - name: Set up Go
           uses: actions/setup-go@v4
           with:
             go-version: '1.22'
         
         - name: Run integration tests
           run: go test -v -tags=integration ./...
           # May need: sudo -E go test... for mount tests
     
     lint:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4
         
         - name: Set up Go
           uses: actions/setup-go@v4
           with:
             go-version: '1.22'
         
         - name: golangci-lint
           uses: golangci/golangci-lint-action@v3
           with:
             version: latest
   ```

2. **Create `.golangci.yml`** for linting:
   ```yaml
   run:
     timeout: 5m
   
   linters:
     enable:
       - gofmt
       - govet
       - errcheck
       - staticcheck
       - unused
       - gosimple
       - ineffassign
   
   linters-settings:
     govet:
       check-shadowing: true
   ```

3. **Add Makefile targets**:
   ```makefile
   .PHONY: test test-unit test-integration coverage lint
   
   test: test-unit
   
   test-unit:
   	go test -v -race ./...
   
   test-integration:
   	go test -v -tags=integration ./...
   
   coverage:
   	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
   	go tool cover -html=coverage.txt -o coverage.html
   	@echo "Coverage report: coverage.html"
   
   lint:
   	golangci-lint run
   ```

#### Testing Checklist

- [ ] CI runs on every PR
- [ ] Unit tests execute with race detector
- [ ] Integration tests run (with root if available)
- [ ] Coverage reported to Codecov (optional)
- [ ] Linting checks pass
- [ ] Failed tests block merge
- [ ] Make targets work locally

---

### Task 6: Testing Documentation ⚪

**Estimated Time**: 0.5 hours  
**Priority**: Medium  
**Dependencies**: Tasks 1-5

#### Description

Document testing approach, conventions, and how to run tests.

#### Implementation Steps

1. **Create `docs/testing/TESTING.md`**:
   ````markdown
   # Testing Guide
   
   ## Test Organization
   
   - **Unit Tests**: `*_test.go` files alongside implementation
   - **Integration Tests**: `*_test.go` with `//go:build integration` tag
   - **Test Coverage Target**: >80% across all packages
   
   ## Running Tests
   
   ### Unit Tests (No Root Required)
   
   ```bash
   # All packages
   go test -v ./...
   
   # With race detector
   go test -v -race ./...
   
   # With coverage
   go test -v -coverprofile=coverage.txt ./...
   go tool cover -html=coverage.txt
   
   # Single package
   go test -v ./pkg
   ```
   
   ### Integration Tests (May Require Root)
   
   ```bash
   # Run integration tests
   go test -v -tags=integration ./...
   
   # With root (for mount tests)
   sudo -E go test -v -tags=integration ./build ./mount
   ```
   
   ## Writing Tests
   
   ### Unit Test Example
   
   ```go
   package mypackage_test
   
   import "testing"
   
   func TestFunctionName(t *testing.T) {
       tests := []struct {
           name    string
           input   string
           want    string
           wantErr bool
       }{
           {"valid input", "test", "result", false},
           {"invalid input", "", "", true},
       }
       
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               got, err := FunctionName(tt.input)
               
               if (err != nil) != tt.wantErr {
                   t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
               }
               
               if got != tt.want {
                   t.Errorf("got %v, want %v", got, tt.want)
               }
           })
       }
   }
   ```
   
   ### Integration Test Example
   
   ```go
   //go:build integration
   // +build integration
   
   package mypackage_test
   
   import (
       "os"
       "testing"
   )
   
   func TestEndToEnd(t *testing.T) {
       if os.Getuid() != 0 {
           t.Skip("requires root")
       }
       
       // Setup
       tmpDir := t.TempDir()
       
       // Execute
       // ...
       
       // Assert
       // ...
       
       // Cleanup (automatic via t.TempDir())
   }
   ```
   
   ## Test Conventions
   
   1. **Use table-driven tests** for multiple test cases
   2. **Use t.Run()** for subtests
   3. **Use t.TempDir()** for temporary directories
   4. **Skip root tests** gracefully with t.Skip()
   5. **Clean up resources** in defer statements
   6. **Test package from outside** (mypackage_test) when possible
   
   ## Coverage Status
   
   Current coverage (as of 2025-11-27):
   
   | Package | Coverage | Status |
   |---------|----------|--------|
   | pkg | 115% | ✅ Excellent |
   | builddb | 252% | ✅ Excellent |
   | build | 54% → 80% | 🎯 Target |
   | config | 0% → 80% | 🎯 Target |
   | log | 0% → 70% | 🎯 Target |
   | mount | 0% → 70% | 🎯 Target |
   
   ## CI/CD
   
   Tests run automatically on every PR via GitHub Actions:
   
   - Unit tests with race detector
   - Integration tests (where root available)
   - Coverage reporting
   - Linting checks
   
   See `.github/workflows/test.yml` for details.
   ````

2. **Update README.md**:
   ```markdown
   ## Testing
   
   ```bash
   # Run all unit tests
   make test
   
   # Run with coverage
   make coverage
   
   # Run integration tests (may require root)
   make test-integration
   ```
   
   See [docs/testing/TESTING.md](docs/testing/TESTING.md) for detailed testing guide.
   ```

#### Testing Checklist

- [ ] Testing guide complete
- [ ] Examples clear and correct
- [ ] Conventions documented
- [ ] Coverage status up to date
- [ ] README updated with testing section

---

## Summary

### Estimated Time Breakdown

| Task | Estimated | Current Status | Critical Path |
|------|-----------|---------------|---------------|
| 1. Build Tests | 2h | 🟡 54% done | ✅ |
| 2. Config Tests | 1.5h | ❌ 0% done | ✅ |
| 3. Log Tests | 1.5h | ❌ 0% done | |
| 4. Mount Tests | 1h | ❌ 0% done (Phase 4) | |
| 5. CI/CD Setup | 1.5h | ❌ Not setup | ✅ |
| 6. Documentation | 0.5h | ❌ Not documented | |
| **Total** | **8h** | **78% base** | **5h critical** |

### Exit Criteria

- [ ] All packages have >80% test coverage (>70% for log)
- [ ] Integration test builds 1-3 ports end-to-end
- [ ] CI runs on every PR with race detector
- [ ] All tests pass without data races
- [ ] Failure tests validate error propagation
- [ ] Documentation explains testing approach
- [ ] Make targets work for local testing

### Current Strengths ✅

- **pkg package**: Excellent coverage (115%, 2,313 test lines)
  - ✅ Parsing, resolution, topo sort tested
  - ✅ Cycle detection covered
  - ✅ BSD integration tests exist
  - ✅ Fidelity tests comprehensive

- **builddb package**: Excellent coverage (252%, 2,120 test lines)
  - ✅ CRUD operations fully tested
  - ✅ CRC operations comprehensive
  - ✅ Error handling tested
  - ✅ Integration tests exist

### Gaps to Address ❌

- **build package**: Good base (54%) but needs expansion
  - ❌ Worker lifecycle not fully tested
  - ❌ Concurrent builds need tests
  - ❌ Error propagation needs validation
  - ❌ Cleanup on panic not tested

- **config package**: No tests (0%)
  - ❌ Loading, validation untested
  - ❌ Defaults not validated
  - ❌ Profile selection untested

- **log package**: No tests (0%)
  - ❌ Log levels not tested
  - ❌ Formatting not validated
  - ❌ Concurrency safety not proven

- **mount package**: No tests (0%)
  - ❌ Will be addressed in Phase 4 (environment package)
  - ❌ Legacy tests needed for transition

### Dependencies

**Requires**:
- ✅ Phase 1: pkg package (already well-tested)
- ✅ Phase 2: builddb package (already well-tested)
- ✅ Phase 3: build package (needs expansion)
- 🔄 Phase 4: environment package (will need new tests)

**Blocks**:
- Phase 7: Integration requires passing tests
- Production readiness depends on comprehensive coverage

### Code Impact

| Package | New Lines | Changes |
|---------|-----------|---------|
| `build/*_test.go` | +300 | Expand tests |
| `config/config_test.go` (new) | +250 | New tests |
| `log/log_test.go` (new) | +200 | New tests |
| `mount/mount_test.go` (new) | +150 | New tests |
| `.github/workflows/` | +150 | CI setup |
| `docs/testing/` | +300 | Documentation |
| **Total** | **~1,350** | **Minimal production code changes** |

---

## Notes

### Design Decisions

1. **Target >80% coverage**: Balance between quality and diminishing returns
2. **Separate integration tests**: Use build tags for tests requiring root
3. **Table-driven tests**: Standard Go testing pattern for readability
4. **No benchmarks in MVP**: Focus on correctness first, performance later
5. **CI uses race detector**: Catch concurrency bugs early

### Reality Check: Already Strong ✅

The project is in **much better shape than Phase 6 documentation suggests**:
- pkg: 2,313 test lines (vs 2,010 implementation) = 115% coverage
- builddb: 2,120 test lines (vs 842 implementation) = 252% coverage
- **Total: 4,875 test lines already written!**

Phase 6 should be reframed as "Complete Testing Coverage" rather than "Add Testing."

### Future Enhancements (Post-MVP)

- Performance benchmarks (BenchmarkDoBuild, BenchmarkTopoSort)
- Chaos testing (random failures, resource exhaustion)
- Fuzzing for parsers (pkg.ParseMakefile)
- Property-based testing (dependency graphs)
- E2E test matrix (different BSD versions)
- Test coverage gates in CI (fail if <80%)

### Testing Philosophy

1. **Unit tests**: Fast, no external dependencies, test logic
2. **Integration tests**: Real filesystem, database, maybe root
3. **Table-driven**: Multiple test cases in compact format
4. **Clear failure messages**: Easy debugging
5. **Avoid brittleness**: Don't test implementation details
6. **Test behavior**: Focus on observable outcomes

---

**Next Phase**: [Phase 7: Integration & Migration](PHASE_7_TODO.md)
