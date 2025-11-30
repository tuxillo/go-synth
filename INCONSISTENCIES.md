# Inconsistencies and Issues

This document tracks inconsistencies, design problems, and notable issues discovered during code review. It is organized by top-level directory so we can incrementally expand coverage.

**Total Items**: 50 (7 bugs, 38 issues, 5 features)

## Quick Navigation

- **Active Tracking**: See [DEVELOPMENT.md - Active Development Tracking](DEVELOPMENT.md#-active-development-tracking)
- **Bugs**: See [DEVELOPMENT.md - Active Bugs](DEVELOPMENT.md#-active-bugs)
- **Issues**: See [DEVELOPMENT.md - Known Issues](DEVELOPMENT.md#%EF%B8%8F-known-issues)
- **Features**: See [DEVELOPMENT.md - Planned Features](DEVELOPMENT.md#-planned-features)

## Critical Patterns (Cross-Package)

These patterns affect multiple packages and represent the highest-priority architectural concerns:

### Pattern 1: Library Functions Write to stdout/stderr ✅ RESOLVED

**Affects**: pkg/, build/, environment/, migration/, mount/, util/  
**Priority**: ~~HIGH~~ **COMPLETED** - Phase 5 REST API unblocked  
**Items**: pkg/#2, build/#5, environment/#3, migration/#1, mount/#2, util/#2  
**Impact**: ~~Makes libraries unusable~~ Libraries now reusable in non-CLI contexts (API, GUI, services)  
**Solution**: ✅ Added LibraryLogger interface to all library functions (8h actual effort)  
**Status**: All 85 print statements removed across 6 packages (11/29/2025)  
**See**: [docs/refactoring/REFACTOR_ISSUE_FIXES.md](docs/refactoring/REFACTOR_ISSUE_FIXES.md) for implementation details

### Pattern 2: Split Responsibilities 🟠 MEDIUM-HIGH

**Affects**: pkg/, build/, mount/, main.go, cmd/  
**Priority**: MEDIUM-HIGH - Maintenance burden, risk of drift  
**Items**: 
- CRC logic split: pkg/#1, build/#1, build/#2
- Mount logic duplicated: mount/ vs environment/bsd
- Build flow duplicated: main.go vs cmd/

**Impact**: Code duplication, unclear source of truth, harder to maintain  
**Solution**: Consolidate to canonical locations (~12h effort)

### Pattern 3: Global Mutable State 🟡 MEDIUM

**Affects**: pkg/, config/, environment/  
**Priority**: MEDIUM - Complicates testing and concurrent usage  
**Items**: pkg/#5 (portsQuerier), config/#1 (globalConfig), environment/#4 (backend registry)  
**Impact**: Hard to test, race conditions, implicit dependencies  
**Solution**: Explicit dependency injection (~6h effort)

---

## pkg/

Library: package metadata, dependency resolution, and build-state helpers.

### 1. Duplicate / conflicting "needs build" logic

- `MarkPackagesNeedingBuild` (in `pkg/pkg.go`) computes CRCs and calls `builddb.NeedsBuild`, printing per-package status and a summary.
- `build.DoBuild` (in `build/build.go`) independently recomputes CRCs, calls `NeedsBuild` again, and applies its own skipping rules.
- Inside `DoBuild` a fresh `BuildStateRegistry` is created, so flags set in the registry passed into `MarkPackagesNeedingBuild` are not visible to the build workers.
- Effect: decisions about which packages need rebuilding are scattered and partially duplicated between `pkg` and `build`, and `MarkPackagesNeedingBuild` behaves more like a CLI/reporting function than a pure library primitive.

### 2. Library functions write directly to stdout/stderr

- `resolveDependencies`, `buildDependencyGraph`, and `GetBuildOrder` (in `pkg/deps.go`) use `fmt.Println` / `fmt.Printf` / `fmt.Fprintf(os.Stderr, ...)` for progress and debug output.
- `MarkPackagesNeedingBuild` (in `pkg/pkg.go`) prints a detailed checklist and per-package status.
- There is no logger dependency or verbosity control in the `pkg` API, which makes the package noisy and hard to embed in other contexts (APIs, GUIs, higher-level CLIs) without side effects on standard output.

### 3. Error types not surfaced consistently

- `ErrEmptySpec` and `ErrInvalidSpec` (in `pkg/errors.go`) are defined but not returned by public functions; callers cannot meaningfully observe them.
- `ErrPortNotFound` / `PortNotFoundError` exist, but `ParsePortList` treats per-port failures as warnings and silently drops those ports from the returned slice; callers get only a best-effort list or `ErrNoValidPorts` if *every* port failed.
- The package-level documentation describes these errors as core error types, but current APIs largely hide them rather than returning structured details to the caller.

### 4. Inconsistent handling of "port not found" vs. "no valid ports"

- `getPackageInfo` returns `PkgFNotFound` plus a `PortNotFoundError` when a port directory does not exist.
- `BulkQueue.worker` computes `initialFlags` only when `err == nil`, so for not-found ports, the `PkgFNotFound` flag is effectively discarded.
- `ParsePortList` logs a warning for any error from `BulkQueue.GetResult` and omits the failed port entirely from the `packages` slice.
- As a result, the only observable error at the API boundary is `ErrNoValidPorts` (when all ports fail); there is no way for callers to see which subset of ports were not found or to retrieve their associated flags.

### 5. Global mutable state for querying ports

- `portsQuerier` and `skipPortDirCheck` (in `pkg/ports_interface.go`) are package-level globals that control how port metadata is fetched.
- The `setTestQuerier` helper mutates these globals without synchronization and is intended for tests.
- In concurrent test runs (e.g., `go test -parallel`), changing this global state can cause races and unpredictable mixing of real ports and fixtures. A more robust design would pass a `PortsQuerier` explicitly (or via a context/struct) instead of relying on shared global state.

### 6. Fixture loading depends on process working directory

- `autoLoadTestFixtures` and `loadFixturesFromDir` use a relative path like `"testdata/fixtures"` when discovering fixtures.
- This assumes tests are always run with the package directory as the working directory; running from another cwd or embedding the package can break fixture discovery.
- While this primarily impacts tests, it is a brittle dependency on process context and could be hardened by resolving paths relative to the module root or the test file location.

### 7. Comment vs. implementation mismatch in dependency parsing

- `parseDependencyString` is documented as skipping `${NONEXISTENT}` dependencies, but the implementation actually checks for the hardcoded prefix `/nonexistent:`.
- This likely matches the current expansion of `${NONEXISTENT}` in BSD ports, but the discrepancy between comment and code is misleading and could cause a future maintainer to "fix" the code incorrectly.

### 8. `resolveDependencies` mutates its input slice unnecessarily

- `resolveDependencies` appends newly discovered dependencies to the `packages` slice argument while simultaneously populating a `PackageRegistry`.
- The public documentation for `ResolveDependencies` explains that callers should use `pkgRegistry.AllPackages()` to obtain the full graph, and call sites already do this.
- Mutating the input slice therefore provides little value and makes the function easier to misuse, since it is not obvious which of the two sources of truth a caller should rely on.

## build/

Library: build orchestration, worker lifecycle, and CRC-based incremental builds.

### 1. Build-state registry disconnect from caller

- `DoBuild` constructs a new `BuildContext` with its own fresh `pkg.BuildStateRegistry`, ignoring the registry that the caller used (e.g., in `main.go` with `MarkPackagesNeedingBuild`).
- As a result, flags recorded earlier (such as `PkgFSuccess`, `PkgFNoBuildIgnore`, `PkgFIgnored`) are not visible to the builder, and the initial per-package "needs build" analysis in `pkg` is effectively disconnected from the runtime state used by workers.
- The same logical problem appears from the `build/` side as from the `pkg/` side: responsibility for deciding what actually needs building is split across layers instead of flowing from a single source of truth.

### 2. CRC-based skip logic duplicates `pkg.MarkPackagesNeedingBuild`

- The queueing goroutine inside `DoBuild` recomputes CRCs (`builddb.ComputePortCRC`) and calls `buildDB.NeedsBuild` to decide whether to skip each package, even if `MarkPackagesNeedingBuild` was already run in the caller.
- Skipped packages are then annotated as `PkgFSuccess` in the *internal* registry and counted as `Skipped` in `BuildStats`, while `Stats.Total` was computed earlier under different assumptions.
- This leads to duplicated CRC work and overlapping policy between `pkg` and `build`, and makes it hard to reason about which layer is authoritative for incremental build decisions.

### 3. `BuildStats` semantics are slightly muddled

- `Stats.Total` is computed once at the start of `DoBuild` by counting packages that are not yet marked as success/ignored in the internal registry (which starts empty), so it effectively becomes "number of packages considered".
- Later, the CRC-based skip logic increments `Stats.Skipped` but never decrements `Total`, so skipped packages remain part of the total even though they were never actually built.
- This is not strictly incorrect, but it makes the relationship between `Total`, `Success`, `Failed`, and `Skipped` less intuitive, especially compared to the `needBuild` value returned from `pkg.MarkPackagesNeedingBuild`.

### 4. Hard-coded environment backend

- `DoBuild` always calls `environment.New("bsd")` to create worker environments.
- This hard-wires the BSD backend and makes it difficult to inject a mock or alternate backend from configuration, even though the `environment` package is designed to support multiple implementations.
- Integration tests work around this by importing the BSD backend for registration, but a cleaner design would allow the caller or configuration to choose the backend type.

### 5. Library functions write directly to stdout/stderr

- `DoBuild` prints a summary line to stdout at the start ("Starting build: ...") and progress updates via `printProgress` using `fmt.Printf`.
- `buildPackage` and other helpers write warnings directly to `os.Stderr` when database or CRC operations fail, instead of going through the structured logging system.
- As with `pkg`, this makes `build` harder to embed or reuse in non-CLI contexts where direct terminal output is undesirable.

### 6. Busy-wait dependency tracking

- `waitForDependencies` uses a simple polling loop that scans all `IDependOn` links and sleeps for 100ms between iterations.
- While this is functionally correct, it introduces unnecessary latency and CPU wakeups, and it relies on the shared registry as an implicit signalling mechanism.
- A more robust design would use explicit synchronization (e.g., channels or condition variables) to notify dependents when a package completes or fails.

### 7. Phase execution has some unused helpers and narrow coverage

- `build/phases.go` contains helpers like `extractPackage`, `copyFile`, `cleanupWorkDir`, and `installMissingPackages` that are not currently used by the main build path, suggesting an incomplete or partially migrated design.
- The `executePhase` switch short-circuits several `*-depends` phases by returning `nil` (relying on ports to pull them in implicitly) and only partially respects configuration (e.g., `check-plist` toggling).
- Integration tests validate the happy path for incremental builds on simple ports, but there is no automated coverage for failure-handling helpers or repository extraction behavior yet.

## builddb/

Library: bbolt-backed build database and CRC-based change tracking.

### 1. Partial use of bucket name constants

- The package defines bucket name constants (`BucketBuilds`, `BucketPackages`, `BucketCRCIndex`) and uses them consistently when creating buckets and in `Stats`.
- However, `LatestFor`, `UpdatePackageIndex`, `UpdateCRC`, and `GetCRC` use hard-coded string literals (`"builds"`, `"packages"`, `"crc_index"`) when opening buckets.
- This works today because the strings match, but it weakens the value of the constants and makes refactoring or typo-detection harder.

### 2. Error wrapping in `LatestFor` can obscure root causes

- `LatestFor` wraps *any* error from its internal read transaction in a new `PackageIndexError` with `Op: "lookup"`, even when the inner error is already a more specific `PackageIndexError` (e.g., with `Op: "validate"` for orphaned records).
- This double-wrapping preserves the full chain via `Unwrap`, but callers must dig through multiple `PackageIndexError` layers to see the underlying operation type.
- It’s not broken, but it’s a little noisy and makes inspection code more cumbersome than necessary.

### 3. No explicit guarding against use-after-close

- `DB.Close` simply calls `db.db.Close()` and leaves the `db.db` field non-nil; subsequent method calls on the same `*DB` will attempt to use a closed bbolt handle.
- The error types include `ErrDatabaseNotOpen` / `ErrDatabaseClosed`, but the methods don’t currently check or return them; they rely instead on bbolt’s own errors when used after close.
- In practice the repository opens the database once and defers close, so this is unlikely to bite normal usage, but it’s an inconsistency between the documented error vocabulary and runtime behavior.

### 4. `ComputePortCRC` uses `filepath.Walk` without context or cancellation

- `ComputePortCRC` walks the filesystem tree with `filepath.Walk` and has no way to observe context cancellation or timeouts, even though higher layers (environment and build orchestration) are context-aware.
- For normal ports this is fine, but on very large or malformed trees (or if pointed somewhere unexpected) CRC computation could become a long-running, uninterruptible operation.
- This is a minor architectural mismatch; a context-aware walk (or caller-provided context) would align better with the rest of the system.

## environment/

Library: abstraction and implementations for isolated build environments.

### 1. Cleanup idempotency vs. BSD implementation

- The `Environment` interface docs state that `Cleanup` must be idempotent and "must succeed even if Setup() failed or was never called".
- `BSDEnvironment.Cleanup` immediately returns an `ErrCleanupFailed` if `baseDir` is empty (i.e., if `Setup` was never called), which conflicts with the documented contract and makes it unsafe to unconditionally `defer env.Cleanup()` after `New`.
- This is a behavioral mismatch between the interface guarantee and the concrete backend’s implementation.

### 2. WorkDir semantics not honored by BSD backend

- `ExecCommand.WorkDir` is documented on the interface as the working directory inside the environment ("Must be absolute path inside environment"), and the comments for `Execute` in `environment.go` refer to it as part of command execution.
- `BSDEnvironment.Execute` explicitly documents that WorkDir is "currently not implemented" and always runs the chrooted command with `/` as the working directory, relying on callers to use `-C` flags instead.
- This discrepancy between the interface-level expectations and the concrete backend behavior can surprise callers relying on `WorkDir` for path resolution.

### 3. Mixed logging / output channels

- The `environment` package defines structured error types (`ErrSetupFailed`, `ErrExecutionFailed`, `ErrCleanupFailed`) and expects implementations to log transient issues rather than fail.
- The BSD backend logs many warnings directly to `os.Stderr` (e.g., mkdir and mount failures) and uses the global `log` package for cleanup logs, rather than going through a shared logger or caller-provided hook.
- This makes it harder to integrate environment operations into higher-level logging frameworks or non-CLI contexts, and creates side effects on global output similar to the issues already noted in `pkg/` and `build/`.

### 4. Global backend registry without synchronization

- `environment.Register` stores backends in a package-level `map[string]NewEnvironmentFunc` without any locking.
- In practice backends are registered from `init` functions at program startup, so concurrent mutation is unlikely, but the API itself does not prevent concurrent registration from multiple goroutines.
- This is a minor concurrency concern; either the registry should be documented as init-time only, or protected with a mutex if runtime registration is expected.

## log/

Library: multi-file logging for build results, status, and debugging.

### 1. Logger is tightly coupled to on-disk file layout

- `NewLogger` always creates eight specific files under `cfg.LogsPath` with hard-coded names and formats, mirroring the C dsynth layout.
- There is no abstraction for alternative sinks (stdout/stderr, structured logging, in-memory logs) or for turning specific streams on/off; any caller must accept this full layout even in contexts where only a subset is useful.
- This design is fine for the primary CLI tool, but makes it hard to reuse `log.Logger` in other frontends or embed it in tests/services without touching the filesystem.

### 2. Aggressive `Sync()` on every write

- Each logging method (`Success`, `Failed`, `Skipped`, `Ignored`, `Abnormal`, `Obsolete`, `Debug`, `Error`, `Info`, `WriteSummary`, and context variants) calls `Sync()` on the underlying files after every write.
- This provides robustness against crashes, but can be expensive on systems with slower storage or many small log entries, and there is no configuration knob to trade durability for throughput.
- The behavior is consistent with the current CLI expectations, but may be surprising for other uses of the logging package.

### 3. Partial integration with higher-level logging and environment

- The `Logger` type is used in `build/` via `WithContext` to create `ContextLogger`, but other packages (like `environment/bsd`) still use `fmt.Fprintf` and the global `log` package for diagnostics rather than routing messages through this logging layer.
- This results in a mixed logging story: some events go to the structured dsynth logs, while others go directly to stderr or the default logger, making it harder to get a unified view of system behavior.

### 4. ContextLogger success accounting vs. BuildStats

- `ContextLogger.Success` writes the contextual success message and unconditionally appends the port directory to `01_success_list.log`, independent of the build stats tracking done in `build.BuildContext`.
- There is no explicit guard against duplicate success entries if `Success` is called multiple times for the same port in a single run, and the log format does not encode the build UUID beyond the truncated context prefix.
- This is more a limitation than a bug, but it means the success list is a best-effort reflection of what happened, not a strictly de-duplicated, authoritative source.

## migration/

Library: helpers for migrating legacy CRC index files into BuildDB.

### 1. Mixed responsibility (library vs CLI output)

- `MigrateLegacyCRC` performs the core migration but also prints status messages directly to stdout/stderr (`fmt.Printf` / `fmt.Fprintf`), similar to the issues in `pkg/` and `build/`.
- This makes the function less reusable from non-CLI contexts (APIs, tests, GUIs) and ties it to a particular user interaction style rather than returning structured results for the caller to present.

### 2. No explicit dry-run or idempotency controls

- The function always imports any CRCs found and then renames the legacy file to `crc_index.bak` on success, with no option for a dry-run or for leaving the legacy file in place.
- If invoked multiple times without external guarding, it will see no further work after the first run (file renamed), but this behavior is implicit rather than enforced via versioning or explicit migration state in the database.

### 3. Limited validation of legacy data

- `readLegacyCRCFile` treats any parse failure on a line (missing colon, invalid hex) as a warning and continues, which is appropriate for best-effort migration but means the caller never learns how much of the legacy data was dropped.
- `MigrateLegacyCRC` reports only a global count of migrated vs total parsed records; it doesn’t expose a structured summary of invalid or failed entries that could be surfaced in a higher-level report.

## mount/

Deprecated: legacy worker mount/unmount helper retained alongside the newer `environment/bsd` backend.

### 1. Duplicate mount orchestration logic vs `environment/bsd`

- `DoWorkerMounts` and `DoWorkerUnmounts` reproduce the same mount layout and retry-unmount behavior that now lives in `environment/bsd.BSDEnvironment.Setup` and `Cleanup`.
- Keeping both implementations risks them drifting out of sync; comments and behavior in `environment/bsd` are now the canonical ones, while `mount/` still uses the older `Worker` type and a different error-reporting style.

### 2. Library functions write directly to stdout/stderr

- `DoWorkerMounts`, `doMount`, and `doUnmount` print errors and warnings directly to `os.Stderr` (mkdir failures, unknown mount types, mount/unmount failures) instead of integrating with the structured logging system.
- This follows the older C-style pattern but is inconsistent with the newer logging abstractions in `log/` and the environment interface.

### 3. Error handling via counters rather than structured types

- Mount failures increment `Worker.MountError` and, after retries in `DoWorkerUnmounts`, may return a generic `fmt.Errorf("unable to unmount all filesystems")`.
- There is no structured error conveying which mountpoints failed or why, unlike `environment.ErrSetupFailed` / `ErrCleanupFailed`, which carry operation and mount lists.

### 4. Unused parameters and flags

- `doMount` takes a `discreteFmt` parameter that is never used, and the `Worker` struct contains fields like `AccumError` and `Status` that are not referenced within this file.
- These leftovers reinforce that `mount/` is a partially-migrated, deprecated path and should not be extended further.

## util/

Library: assorted process, filesystem, and formatting helpers.

### 1. Shelling out for basic file operations

- `CopyFile`, `CopyDir`, and `RemoveAll` all invoke external commands (`cp`, `cp -Rp`, `rm -rf`) instead of using Go’s standard library (`io.Copy`, `os.RemoveAll`, etc.).
- This makes behavior dependent on the host’s userland (path to `cp`/`rm`, option semantics) and complicates testing or portability to non-BSD Unix-like systems.

### 2. Direct user interaction in a generic util package

- `AskYN` prints prompts directly to stdout and reads from stdin using `fmt.Scanln`, coupling it to an interactive TTY environment.
- Having this in a low-level util package encourages embedding user interaction deep in the call graph, rather than keeping prompts at the CLI boundary where they can be tested or bypassed (e.g., non-interactive modes).

### 3. Platform-specific stubs without clear contract

- `GetSwapUsage` currently returns `(0.0, false)` with a comment that it "would need platform-specific implementation".
- Callers must remember to inspect the boolean flag to know whether the metric is meaningful; the function name alone suggests real usage data, which can be misleading if the return value is accidentally used without checking the flag.

### 4. Thin wrappers around standard library functions

- Several helpers (`FileExists`, `DirExists`, `MkdirAll`, `WriteFile`, `ReadFile`, `Chdir`, `Getwd`, `Glob`) provide very small value over direct use of the `os`/`filepath` APIs.
- This is not inherently wrong, but it creates another layer of indirection that can obscure which calls are actually doing I/O and makes partial refactoring trickier.

## config/

Library: configuration loading, defaults, and global config access.

### 1. Global `Config` singleton

- `globalConfig` plus `GetConfig` / `SetConfig` introduce process-wide mutable state for configuration.
- This makes it easy to accidentally rely on implicit globals instead of passing configuration explicitly, and complicates concurrent tests or multiple-profile scenarios.

### 2. Asymmetric defaulting for Migration and Database settings

- Migration booleans are defaulted via conditional logic that infers "unset" based on combinations of `AutoMigrate` and `BackupLegacy`, which is subtle and can be hard to reason about if only one flag is set in the INI file.
- In contrast, `Database.AutoVacuum` is unconditionally forced to `true` at the end of `LoadConfig`, even if a section explicitly set it to false.
- This leads to surprising behavior where some INI values are respected and others are silently overridden by hard-coded defaults.

### 3. Boolean parsing and casing behavior

- `parseBool` first tries `strconv.ParseBool`, then falls back to a manual check for a limited set of string values (`yes`/`Yes`/`YES`/`1`/`on`/`On`/`ON`).
- Values like `true`/`True` are handled by `ParseBool`, but mixed or unexpected casings for the fallback set may not be; the reassignment `s = s` is also a no-op, suggesting previous refactoring left minor cruft.

### 4. Hard-coded default paths and environment probing

- `LoadConfig` hard-codes default paths like `/build`, `/usr/dports`, `/usr/ports`, and `/etc/dsynth/dsynth.ini`, and probes the filesystem (`os.Stat`) to decide between ports trees.
- This is consistent with the project’s BSD focus, but couples configuration logic tightly to specific host layouts and makes it harder to run in non-standard environments without a configuration file.

## main.go

Top-level CLI entrypoint and command dispatcher.

### 1. Mixed responsibilities and limited reuse

- `main.go` directly wires flag parsing, configuration loading, command dispatch, and core build orchestration (e.g., `doBuild`) instead of delegating to a separate CLI layer (like the Cobra `cmd/` package) or a reusable service layer.
- This makes it harder to reuse the core functionality (init, status, build, migration) from other entrypoints, tests, or tools without going through the `main` binary semantics.

### 2. Inconsistent prompting and user interaction

- Several commands (`doInit`, `doBuild`, `doResetDB`) implement their own interactive prompts using `fmt.Print` / `fmt.Scanln`, while `util.AskYN` is used only in `doBuild` for the main "Build N packages?" confirmation.
- Prompt wording, default behavior, and case handling vary slightly between locations, which can be confusing and makes it harder to centralize non-interactive/`-y` behavior.

### 3. Partial implementations and TODO-heavy commands

- Multiple subcommands (`configure`, `rebuild-repository`, `purge-distfiles`, `verify`, `status-everything`, `fetch-only`) are placeholders that only print "not yet implemented" messages or stubs.
- This is expected for an in-progress rewrite, but it means the surface area advertised by `usage()` is larger than the set of fully functional commands, which may surprise users.

### 4. Duplicate logic for CRC migration and cleanup

- Legacy CRC migration logic appears in both `doInit` and `doBuild`, with slightly different messaging and behavior (e.g., where warnings and summaries are printed).
- Worker cleanup behavior also appears in both `doCleanup` (with a custom `cleanupWorkerMounts`) and within the build pipeline via `build.DoBuild`’s returned cleanup function, increasing the chances of divergence over time.

## cmd/

CLI: experimental Cobra-based `build` command (not yet wired as root).

### 1. Duplicated CLI flow vs `main.go`

- `runBuild` reimplements the main build flow already present in `main.go` (argument validation, registry creation, dependency resolution, CRC marking, confirmation prompt, and build summary printing).
- Keeping both paths in sync increases maintenance cost and risks divergence in behavior (e.g., how errors are reported or how stats are printed).

### 2. Mixed responsibility and partial wiring

- The Cobra command is declared but not registered (`rootCmd` and `init` are commented out), so this code is effectively unused in the current binary.
- Despite that, `runBuild` performs real side effects (opens `builds.db`, starts a signal handler goroutine, runs builds) rather than being a thin adapter around a shared, testable function.

### 3. Inconsistent configuration and logger error handling

- `runBuild` calls `config.GetConfig()` and assumes a global config has been pre-populated, coupling the CLI subcommand tightly to global state.
- It also ignores the error from `log.NewLogger(cfg)` (`logger, _ := log.NewLogger(cfg)`), which can lead to nil dereferences later if logger creation fails, instead of failing fast with a clear message.

### 4. Inline user prompting instead of using shared utilities

- The command implements its own confirmation prompt using `fmt.Printf` and `fmt.Scanln`, rather than reusing `util.AskYN` or another centralized interaction helper.
- This contributes to slight inconsistencies in prompt style and behavior compared to other interactive points in the tool.

---

## Quick Wins (Low Effort, High Value)

These issues can be fixed quickly and provide immediate value:

### 1. builddb/#1 - Use bucket constants consistently
- **Effort**: 5 minutes
- **Files**: `builddb/db.go` (4 locations)
- **Change**: Replace `"builds"` with `BucketBuilds`, `"packages"` with `BucketPackages`, etc.
- **Benefit**: Compile-time safety, easier refactoring

### 2. config/#2 - Respect AutoVacuum INI setting
- **Effort**: 15 minutes
- **Files**: `config/config.go`
- **Change**: Don't force `AutoVacuum=true` at end of `LoadConfig`, respect user's INI value
- **Benefit**: User configuration actually works

### 3. pkg/#7 - Fix comment vs implementation mismatch
- **Effort**: 5 minutes
- **Files**: `pkg/deps.go` (parseDependencyString)
- **Change**: Update comment to say "/nonexistent:" prefix instead of "${NONEXISTENT}"
- **Benefit**: Documentation matches implementation

### 4. config/#3 - Simplify boolean parsing logic
- **Effort**: 10 minutes
- **Files**: `config/config.go` (parseBool function)
- **Change**: Remove no-op `s = s` assignment, clarify fallback logic
- **Benefit**: Cleaner code, less confusion

### 5. mount/#4 - Remove unused parameters (or skip if removing mount/)
- **Effort**: 15 minutes (or 0 if removing package)
- **Files**: `mount/mount.go`
- **Change**: Remove `discreteFmt` parameter, `AccumError` and `Status` fields
- **Benefit**: Cleaner API

**Total Quick Wins Time**: ~50 minutes for 5 fixes

---

## Critical Path for Phase 5 (REST API)

Before implementing Phase 5 REST API, these items **must** be addressed:

### Blockers (Required)

1. **Pattern 1: Remove stdout/stderr from library packages** (~8 hours)
   - Affects: pkg/#2, build/#5, environment/#3, migration/#1
   - Why: API cannot have libraries printing to terminal
   - Solution: Add logger parameter or return structured results

2. **main.go/#1: Extract service layer** (~4 hours)
   - Why: API needs reusable functions to call (not main.go CLI logic)
   - Solution: Create `service/` package with BuildService, StatusService, etc.

**Total Blocker Effort**: ~12 hours

### Strongly Recommended (Not Strict Blockers)

3. **Pattern 2: Consolidate split responsibilities** (~12 hours)
   - Why: Cleaner architecture before adding another layer
   - Items: CRC logic, mount logic, build flow
   - Solution: Pick canonical location, refactor others to use it

4. **log/#1: Configurable logging backends** (~4 hours)
   - Why: API needs different log format than file-based CLI logs
   - Solution: Logger interface with multiple implementations

**Total Recommended Effort**: ~16 hours

### Optional (Nice-to-Have)

5. **Pattern 3: Remove global mutable state** (~6 hours)
   - Why: API may need multiple concurrent configurations
   - Solution: Dependency injection

**Total Critical Path**: 12h (blockers) + 16h (recommended) = **28 hours**

---

## Statistics

### By Category
- 🐛 **Bugs**: 7 (14%)
  - High: 3 (contract violations)
  - Medium: 2 (error handling)
  - Low: 2 (race conditions)

- ⚠️ **Issues**: 38 (76%)
  - Architectural (Critical): 13
  - Architectural (Patterns): 6
  - Testing: 5
  - Code Quality: 8
  - Deprecated: 2
  - Performance: 2
  - Missing Functionality: 2

- ✨ **Features**: 5 (10%)
  - High: 1 (stdout/stderr removal)
  - Medium: 3 (context support, logging, swap usage)
  - Low: 1 (event-driven deps)

### By Package
- **pkg/**: 8 issues
- **build/**: 7 issues
- **builddb/**: 4 issues
- **environment/**: 4 issues
- **log/**: 4 issues
- **migration/**: 3 issues
- **mount/**: 4 issues (deprecated)
- **util/**: 4 issues
- **config/**: 4 issues
- **main.go**: 4 issues
- **cmd/**: 4 issues (mostly unused)

### Priority Distribution
- 🔴 **High**: 4 items (3 bugs + 1 feature)
- 🟠 **Medium-High**: 15 items (architecture)
- 🟡 **Medium**: 18 items (quality, testing)
- 🔵 **Low**: 13 items (polish, deprecated code)

---

## Migration to GitHub Issues

When ready to migrate to GitHub Issues (post-MVP, when project goes public):

1. Create labels: `bug`, `issue`, `feature`, `priority-high`, `priority-medium`, `priority-low`, `pattern-1`, `pattern-2`, `pattern-3`, `phase-5-blocker`
2. Create issues from high-priority items first
3. Reference this document in issue descriptions
4. Update DEVELOPMENT.md to link to GitHub Issues instead
5. Keep this document as historical reference

---

**Last Updated**: 2025-11-29  
**Review Completed By**: Code review during Phase 7  
**Status**: All 50 items catalogued and prioritized
