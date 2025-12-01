# Stub Commands Status

This document tracks the implementation status of all go-synth commands.

**Last Updated**: 2025-11-30  
**Status**: 10 implemented, 5 stubs remaining

---

## Implemented Commands (10/15) ✅

### Core Commands (Using Service Layer)

| Command | Status | Service Method | Description |
|---------|--------|----------------|-------------|
| `init` | ✅ Implemented | `service.Initialize()` | Initialize go-synth environment |
| `status` | ✅ Implemented | `service.GetStatus()` | Show build status for ports |
| `cleanup` | ✅ Implemented | `service.Cleanup()` | Clean up stale worker environments |
| `reset-db` | ✅ Implemented | `service.ResetDatabase()` | Reset build database |
| `build` | ✅ Implemented | `service.Build()` | Build specified ports |
| `just-build` | ✅ Implemented | `service.Build()` + flag | Build without pre-checks |
| `force` | ✅ Implemented | `service.Build()` + flag | Force rebuild all ports |
| `test` | ✅ Implemented | `service.Build()` + flag | Build with test mode |

### Utility Commands (Direct Implementation)

| Command | Status | Implementation | Description |
|---------|--------|----------------|-------------|
| `logs` | ✅ Implemented | `doLogs()` | Display build logs for a port |
| `version` | ✅ Implemented | Inline | Display version information |

---

## Stub Commands (5/15) ⏳

These commands are defined but not yet fully implemented. They print "not yet implemented" messages.

### 1. `configure` ⏳
**Function**: `doConfigure(cfg *config.Config)`  
**Status**: Stub (prints message)  
**Description**: Interactive configuration wizard

**Planned Implementation**:
- Prompt user for configuration values
- Validate inputs
- Write to dsynth.ini
- Could use service layer for validation

**Effort**: ~4 hours

---

### 2. `upgrade-system` ⏳
**Function**: `doUpgradeSystem(cfg *config.Config)`  
**Status**: Partially implemented (calls doBuild)  
**Description**: Upgrade all installed packages

**Current Implementation**:
```go
// Get list of installed packages
installed, err := pkg.GetInstalledPackages(cfg)
// Build the installed packages
doBuild(cfg, installed, false, false)
```

**Issues**:
- `pkg.GetInstalledPackages()` may not be implemented
- Should verify installed packages exist in ports tree
- Should handle package renames/removals

**Effort**: ~6 hours (requires pkg system integration)

---

### 3. `prepare-system` ⏳
**Function**: `doPrepareSystem(cfg *config.Config)`  
**Status**: Calls `doUpgradeSystem()` (alias)  
**Description**: Prepare system for builds

**Current Implementation**:
```go
func doPrepareSystem(cfg *config.Config) {
    fmt.Println("Preparing system...")
    doUpgradeSystem(cfg)
}
```

**Planned Implementation**:
- Verify system dependencies
- Check disk space
- Validate configuration
- Optionally run upgrade-system

**Effort**: ~3 hours

---

### 4. `rebuild-repository` ⏳
**Function**: `doRebuildRepo(cfg *config.Config)`  
**Status**: Stub (prints message)  
**Description**: Rebuild package repository metadata

**Planned Implementation**:
- Scan packages directory for built packages
- Generate pkg repository metadata
- Sign repository (if configured)
- Update repository catalog

**Dependencies**:
- Requires pkg repo tools (pkg-repo, pkg)
- May need to shell out to `pkg repo`

**Effort**: ~4 hours

**Note**: Currently called by `doBuild()` after successful builds (line 620)

---

### 5. `purge-distfiles` ⏳
**Function**: `doPurgeDistfiles(cfg *config.Config)`  
**Status**: Stub (prints message)  
**Description**: Remove unused distfiles to free disk space

**Planned Implementation**:
- Scan distfiles directory
- Compare against ports tree (find unreferenced files)
- Optionally keep recent downloads
- Interactive confirmation (unless -y flag)
- Report space freed

**Effort**: ~3 hours

---

### 6. `verify` ⏳
**Function**: `doVerify(cfg *config.Config)`  
**Status**: Stub (prints message)  
**Description**: Verify built packages

**Planned Implementation**:
- Check package integrity (checksums)
- Verify package metadata
- Test package installation (in temporary environment)
- Report corrupted packages

**Effort**: ~5 hours

---

### 7. `status-everything` ⏳
**Function**: `doStatusEverything(cfg *config.Config)`  
**Status**: Stub (prints message)  
**Description**: Show status for all ports in tree

**Planned Implementation**:
- Use `service.GetStatus()` with empty port list (all ports)
- Display summary statistics
- Group by category
- Show build coverage percentage

**Effort**: ~2 hours (easy - service layer already supports this)

---

### 8. `everything` ⏳
**Function**: `doEverything(cfg *config.Config)`  
**Status**: Partially implemented (calls doBuild)  
**Description**: Build all ports in tree

**Current Implementation**:
```go
// Get all ports from the ports tree
portList, err := pkg.GetAllPorts(cfg)
// Build all ports
doBuild(cfg, portList, false, false)
```

**Issues**:
- `pkg.GetAllPorts()` may not be implemented
- Should handle massive build queues gracefully
- Should provide progress updates
- May need special scheduling for all-port builds

**Effort**: ~4 hours

---

### 9. `fetch-only` ⏳
**Function**: `doFetchOnly(cfg *config.Config, portList []string)`  
**Status**: Stub (prints message)  
**Description**: Download distfiles without building

**Planned Implementation**:
- Use existing `build.DoFetchOnly()` function
- Parse port list
- Resolve dependencies
- Download all distfiles
- Report download statistics

**Effort**: ~2 hours (DoFetchOnly already exists in build/ package)

**Note**: The `build.DoFetchOnly()` function is already implemented but not wired to CLI

---

## Implementation Priority

### High Priority (User-Facing Features)
1. **fetch-only** (2h) - Already implemented in library, just wire to CLI
2. **status-everything** (2h) - Service layer ready, just display logic
3. **rebuild-repository** (4h) - Called by build, should work
4. **purge-distfiles** (3h) - Useful for disk space management

### Medium Priority (System Management)
5. **configure** (4h) - Nice-to-have for first-time setup
6. **prepare-system** (3h) - Wrapper around upgrade-system
7. **verify** (5h) - Quality assurance feature

### Low Priority (Complex/Niche)
8. **upgrade-system** (6h) - Requires pkg system integration
9. **everything** (4h) - Massive build operation, needs special handling

---

## Service Layer Opportunities

Commands that could benefit from service layer extraction:

### Already Using Service Layer ✅
- `init`, `status`, `cleanup`, `reset-db`, `build`, `just-build`, `force`, `test`

### Could Use Service Layer
- **fetch-only**: Add `service.FetchOnly()` method
- **rebuild-repository**: Add `service.RebuildRepository()` method
- **purge-distfiles**: Add `service.PurgeDistfiles()` method
- **verify**: Add `service.VerifyPackages()` method
- **status-everything**: Already supported by `service.GetStatus()`

### Should Stay in CLI
- **configure**: Interactive, CLI-specific
- **logs**: Simple file display, no business logic
- **version**: Trivial, no business logic

---

## Testing Strategy

### Unit Tests
Each service method should have comprehensive unit tests (like existing service layer tests).

### Integration Tests
- Test with real ports tree (requires VM/BSD environment)
- Validate with complex port dependencies
- Test error handling and edge cases

### End-to-End Tests
- Full workflows (fetch → build → repository → install)
- Multi-command sequences
- Failure recovery scenarios

---

## Future Work

### Phase 5.5: Complete Stub Commands (~30 hours)
1. Implement high-priority stubs (11 hours)
2. Add service layer methods where appropriate (8 hours)
3. Write tests for new functionality (8 hours)
4. Update documentation (3 hours)

### Phase 6: Advanced Features
- Concurrent fetching (parallel downloads)
- Build queuing and scheduling
- Remote build workers
- Build result caching
- Package signing and verification

---

## See Also

- [service/README.md](service/README.md) - Service layer API documentation
- [DEVELOPMENT.md](DEVELOPMENT.md) - Overall development roadmap
- [main.go](../main.go) - CLI command implementations
