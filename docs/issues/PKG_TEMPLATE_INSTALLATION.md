# Issue #3: pkg Not Installed into Template

**Status**: 🔴 Open  
**Priority**: P0 - Critical (Blocks all builds)  
**Discovered**: 2025-11-30  
**Component**: `build/bootstrap.go`, `build/phases.go`  
**Affects**: All packages with dependencies (99% of ports)

---

## Problem Statement

While Issue #2 successfully implemented pkg *building* first (before workers start), it does not *install* the built pkg package into the Template directory. This causes a critical failure: workers copy from Template, so without pkg in Template, workers have no `/usr/local/sbin/pkg` binary to install dependencies.

Additionally, the build continues after dependency installation failures instead of stopping immediately, masking the root cause.

---

## Observed Behavior

### Test Case: Building print/indexinfo

```bash
$ cd /root/go-synth
$ rm -rf /build/synth/Template /build/synth/packages /build/synth/builds.db
$ echo "y" | doas ./go-synth -C /nonexistent build print/indexinfo

# Output shows:
Build Plan:
  Total packages: 2
  To build: 2
  To skip: 0

Packages to build:
  - ports-mgmt/pkg        ← Shows in list
  - print/indexinfo

# But check Template after build:
$ ls /build/synth/Template/usr/local/sbin/pkg
ls: /build/synth/Template/usr/local/sbin/pkg: No such file or directory

# Worker slots also missing pkg:
$ ls /build/synth/SL00/usr/local/sbin/pkg
ls: /build/synth/SL00/usr/local/sbin/pkg: No such file or directory
```

### Dependency Installation Failure (Silent)

When building a port with dependencies, the build output would show:

```
[Worker 0] misc/help2man: Installing dependencies...
pkg: command not found           ← ERROR but logged as warning
[Worker 0] misc/help2man: build  ← Shouldn't reach here!
```

**The build continues** because `installPackages()` logs warnings instead of returning errors.

---

## Root Causes

### 1. Missing Template Installation (build/bootstrap.go)

**Current Code** (lines 165-169):
```go
// Step 9: Mark success
registry.AddFlags(pkgPkg, pkg.PkgFSuccess|pkg.PkgFPackaged)
logger.Success("ports-mgmt/pkg (bootstrap build succeeded)")

return nil  // ← Exits without installing pkg into Template!
```

**What's Missing**: After building pkg, we need to extract the `.pkg` file into Template using tar.

**C dsynth Reference** (build.c:266-286):
```c
/*
 * Install pkg/pkg-static into the template
 */
if (newtemplate && FetchOnlyOpt == 0) {
    char *buf;
    int rc;

    asprintf(&buf,
         "cd %s/Template; "
         "tar --exclude '+*' --exclude '*/man/*' "
         "-xvzpf %s/%s > /dev/null 2>&1",
         BuildBase,
         RepositoryPath,
         scan->pkgfile);
    rc = system(buf);
    if (rc)
        dfatal("Command failed: %s\n", buf);  // ← FATAL on failure
    freestrp(&buf);
}
```

---

### 2. No Template Check Before Bootstrap (build/bootstrap.go)

**Current Code** (lines 48-67):
```go
logger.Info("Bootstrap phase: checking ports-mgmt/pkg...")

// Step 2: Compute CRC of pkg port directory
portPath := filepath.Join(cfg.DPortsPath, pkgPkg.Category, pkgPkg.Name)
currentCRC, err := builddb.ComputePortCRC(portPath)
// ... proceeds to check CRC and potentially rebuild
```

**Problem**: Doesn't check if `/build/synth/Template/usr/local/sbin/pkg` already exists before computing CRC and potentially rebuilding.

**C dsynth Reference** (build.c:230-237):
```c
scan = GetPkgPkg(&pkgs);

/*
 * Create our template.  The template will be missing pkg
 * and pkg-static.
 */
if (FetchOnlyOpt) {
    newtemplate = DoCreateTemplate(0);
} else if ((scan->flags & (PKGF_SUCCESS | PKGF_PACKAGED)) == 0) {
    /* force a fresh template */
    newtemplate = DoCreateTemplate(1);
} else {
    newtemplate = DoCreateTemplate(0);
}

// Later: Build pkg only if not already successful
if ((scan->flags & (PKGF_SUCCESS | PKGF_PACKAGED)) == 0 && FetchOnlyOpt == 0) {
    build_list = scan;
    // ... build pkg
}
```

---

### 3. Wrong Error Handling (build/phases.go)

**Problem 1**: `installPackages()` continues on failure (lines 172-183)

**Current Code**:
```go
result, err := worker.Env.Execute(ctx, execCmd)
if err != nil {
    // Execution failed - log warning but don't fail build
    logger.WriteWarning(fmt.Sprintf("Package install execution failed for %s: %v", pkgFile, err))
    continue  // ← WRONG: Should return error
}

if result.ExitCode != 0 {
    // pkg add failed, but this might be acceptable (already installed)
    // Output already captured by logger, just log warning
    logger.WriteWarning(fmt.Sprintf("Package install returned exit code %d for %s", result.ExitCode, pkgFile))
    // ← WRONG: Should return error
}
```

**Problem 2**: `installMissingPackages()` continues on exec error (lines 250-256)

**Current Code**:
```go
result, err := worker.Env.Execute(ctx, installCmd)
if err != nil {
    logger.WriteWarning(fmt.Sprintf("Failed to execute install for %s: %v", pkgFile, err))
    continue  // ← WRONG: Should return error
}

if result.ExitCode != 0 {
    logger.WriteWarning(fmt.Sprintf("Failed to install %s: exit code %d", pkgFile, result.ExitCode))
    return fmt.Errorf("failed to install required package %s: exit code %d", pkgFile, result.ExitCode)
    // ← This one is correct, but inconsistent
}
```

---

## Impact Analysis

### Severity: P0 - Critical

- **Blocks 99% of ports**: Almost all ports have dependencies that require pkg to install
- **Silent failures**: Builds continue after "pkg: command not found", hiding the root cause
- **Cascading errors**: Ports fail during build phase with cryptic errors about missing dependencies
- **Production blocker**: Makes the tool unusable for any real-world port building

### Affected Components

1. **build/bootstrap.go**: Missing Template installation after pkg build
2. **build/phases.go**: Wrong error handling in dependency installation
3. **All port builds**: Workers cannot install dependencies without pkg binary

---

## Solution Plan (3 Steps)

### Step 1: Check Template for Existing pkg (Skip if Present)

**File**: `build/bootstrap.go`  
**Location**: After line 46 (after finding pkgPkg, before computing CRC)

**Implementation**:
```go
// Step 1.5: Check if pkg is already installed in Template
// If pkg binary exists in Template AND package file exists, skip bootstrap
templatePkg := filepath.Join(cfg.BuildBase, "Template/usr/local/sbin/pkg")
pkgFilePath := filepath.Join(cfg.PackagesPath, "All", pkgPkg.PkgFile)

if _, err := os.Stat(templatePkg); err == nil {
    // pkg binary exists in Template
    if _, err := os.Stat(pkgFilePath); err == nil {
        // Package file also exists
        registry.AddFlags(pkgPkg, pkg.PkgFSuccess|pkg.PkgFPackaged)
        logger.Success("ports-mgmt/pkg (already in Template, using existing)")
        return nil
    }
}

logger.Info("Bootstrap phase: pkg not in Template or package missing, will build...")
```

**Rationale**:
- C dsynth checks `(scan->flags & (PKGF_SUCCESS | PKGF_PACKAGED))` before building
- We verify Template has the binary AND package file exists
- Avoids rebuilding pkg on every run (performance improvement)

---

### Step 2: Install pkg into Template After Building

**File**: `build/bootstrap.go`  
**Location**: After line 167 (after marking PkgFSuccess, before return statement)

**Implementation**:
```go
// Step 10: Install pkg into Template directory
// This is CRITICAL - other ports need /usr/local/sbin/pkg to install their dependencies
// C dsynth does this at build.c:273-285
logger.Info("Installing ports-mgmt/pkg into Template...")

templateDir := filepath.Join(cfg.BuildBase, "Template")
pkgFilePath := filepath.Join(cfg.PackagesPath, "All", pkgPkg.PkgFile)

// Verify package file exists
if _, err := os.Stat(pkgFilePath); err != nil {
    return fmt.Errorf("bootstrap: package file not found after build: %s (%w)", pkgFilePath, err)
}

// Verify Template directory exists
if _, err := os.Stat(templateDir); err != nil {
    return fmt.Errorf("bootstrap: Template directory not found: %s (%w)", templateDir, err)
}

// Extract pkg package into Template using tar
// Exclude metadata (+* files) and man pages like C dsynth does (build.c:273)
// Command: tar --exclude '+*' --exclude '*/man/*' -xzpf <pkgfile> -C <template>
cmd := exec.CommandContext(ctx, "tar",
    "--exclude", "+*",
    "--exclude", "*/man/*",
    "-xzpf", pkgFilePath,
    "-C", templateDir)

output, err := cmd.CombinedOutput()
if err != nil {
    return fmt.Errorf("bootstrap: failed to install pkg into Template: %w (output: %s)", err, string(output))
}

logger.Success("ports-mgmt/pkg installed into Template at /usr/local/sbin/pkg")
```

**Rationale**:
- Matches C dsynth behavior exactly (build.c:273-285)
- Uses same tar exclusions: `+*` (metadata) and `*/man/*` (man pages)
- FATAL error if extraction fails (just like C dsynth)
- Workers will copy Template with pkg included

**Expected Result**:
```bash
$ ls -la /build/synth/Template/usr/local/sbin/
total 16
drwxr-xr-x  2 root  wheel   512 Nov 30 23:00 .
drwxr-xr-x  8 root  wheel   512 Nov 30 23:00 ..
-rwxr-xr-x  1 root  wheel  1234 Nov 30 23:00 pkg         ← MUST EXIST
-rwxr-xr-x  1 root  wheel  5678 Nov 30 23:00 pkg-static  ← MUST EXIST
```

---

### Step 3: Fix Error Handling - Stop on Failure

**File**: `build/phases.go`

#### Fix 3.1: installPackages() - Lines 172-183

**Current (WRONG)**:
```go
result, err := worker.Env.Execute(ctx, execCmd)
if err != nil {
    logger.WriteWarning(fmt.Sprintf("Package install execution failed for %s: %v", pkgFile, err))
    continue  // ← WRONG
}

if result.ExitCode != 0 {
    logger.WriteWarning(fmt.Sprintf("Package install returned exit code %d for %s", result.ExitCode, pkgFile))
    // ← WRONG: No return
}
```

**Fixed (CORRECT)**:
```go
result, err := worker.Env.Execute(ctx, execCmd)
if err != nil {
    return fmt.Errorf("failed to install dependency %s: %w", pkgFile, err)
}

if result.ExitCode != 0 {
    return fmt.Errorf("failed to install dependency %s: exit code %d", pkgFile, result.ExitCode)
}
```

#### Fix 3.2: installMissingPackages() - Lines 250-253

**Current (WRONG)**:
```go
result, err := worker.Env.Execute(ctx, installCmd)
if err != nil {
    logger.WriteWarning(fmt.Sprintf("Failed to execute install for %s: %v", pkgFile, err))
    continue  // ← WRONG
}
```

**Fixed (CORRECT)**:
```go
result, err := worker.Env.Execute(ctx, installCmd)
if err != nil {
    return fmt.Errorf("failed to execute install for %s: %w", pkgFile, err)
}
```

**Rationale**:
- ANY command failure should stop the build immediately
- Continuing after "pkg: command not found" hides the root cause
- Matches principle: fail fast with clear error messages
- C dsynth uses `dfatal()` for command failures

---

## Testing Plan

### Test 1: Fresh Bootstrap Build

**Setup**:
```bash
ssh root@vm 'rm -rf /build/synth/Template /build/synth/packages /build/synth/builds.db /build/synth/SL*'
```

**Test**:
```bash
ssh root@vm 'cd /root/go-synth && echo "y" | doas ./go-synth -C /nonexistent build print/indexinfo'
```

**Expected Output**:
```
Bootstrap phase: pkg not in Template or package missing, will build...
Building ports-mgmt/pkg (bootstrap)...
  [bootstrap] ports-mgmt/pkg: fetch
  [bootstrap] ports-mgmt/pkg: checksum
  [bootstrap] ports-mgmt/pkg: extract
  [bootstrap] ports-mgmt/pkg: patch
  [bootstrap] ports-mgmt/pkg: configure
  [bootstrap] ports-mgmt/pkg: build
  [bootstrap] ports-mgmt/pkg: stage
  [bootstrap] ports-mgmt/pkg: package
Installing ports-mgmt/pkg into Template...
ports-mgmt/pkg installed into Template at /usr/local/sbin/pkg
ports-mgmt/pkg (bootstrap build succeeded)
Starting build: 1 packages (1 skipped, 0 ignored)
[Worker 0] Building print/indexinfo...
[Worker 0] print/indexinfo: SUCCESS
```

**Verification**:
```bash
# pkg binary in Template
ssh root@vm 'ls -la /build/synth/Template/usr/local/sbin/pkg'
-rwxr-xr-x  1 root  wheel  123456 Nov 30 23:00 /build/synth/Template/usr/local/sbin/pkg

# pkg package created
ssh root@vm 'ls -la /build/synth/packages/All/pkg-*.pkg'
-rw-r--r--  1 root  wheel  456789 Nov 30 23:00 /build/synth/packages/All/pkg-1.21.3.pkg

# indexinfo package created
ssh root@vm 'ls -la /build/synth/packages/All/indexinfo-*.pkg'
-rw-r--r--  1 root  wheel  12345 Nov 30 23:00 /build/synth/packages/All/indexinfo-0.3.1.pkg
```

---

### Test 2: Second Build (Should Skip Bootstrap)

**Test**:
```bash
# Run again WITHOUT cleaning
ssh root@vm 'cd /root/go-synth && echo "y" | doas ./go-synth -C /nonexistent build print/indexinfo'
```

**Expected Output**:
```
Bootstrap phase: checking ports-mgmt/pkg...
ports-mgmt/pkg (already in Template, using existing)
Starting build: 1 packages (1 skipped, 0 ignored)
[Worker 0] Building print/indexinfo...
[Worker 0] print/indexinfo: SUCCESS
```

**Key Difference**: 
- No "Building ports-mgmt/pkg" message
- Immediate worker start
- Same successful result

---

### Test 3: Build Port with Dependencies

**Test**:
```bash
ssh root@vm 'cd /root/go-synth && rm -rf /build/synth/packages /build/synth/builds.db && echo "y" | doas ./go-synth -C /nonexistent build misc/help2man'
```

**Expected**:
- pkg bootstraps first
- All 11 dependencies build successfully
- misc/help2man builds last
- NO "pkg: command not found" errors
- NO "failed to install dependency" errors that continue

---

### Test 4: Error Handling - Force pkg Build Failure

**Setup**:
```bash
# Corrupt pkg Makefile
ssh root@vm 'echo "BROKEN_BUILD=yes" >> /usr/dports/ports-mgmt/pkg/Makefile'
```

**Test**:
```bash
ssh root@vm 'cd /root/go-synth && rm -rf /build/synth/Template /build/synth/packages /build/synth/builds.db && echo "y" | doas ./go-synth -C /nonexistent build print/indexinfo'
```

**Expected Output**:
```
Bootstrap phase: pkg not in Template or package missing, will build...
Building ports-mgmt/pkg (bootstrap)...
  [bootstrap] ports-mgmt/pkg: fetch
  [bootstrap] ports-mgmt/pkg: checksum
  [bootstrap] ports-mgmt/pkg: extract
  [bootstrap] ports-mgmt/pkg: patch
  [bootstrap] ports-mgmt/pkg: configure
  [bootstrap] ports-mgmt/pkg: build
Error: pkg bootstrap failed: bootstrap build failed: phase build failed: phase failed with exit code 1

Build failed: pkg bootstrap failed: bootstrap build failed: phase build failed: phase failed with exit code 1
```

**Key Points**:
- Build STOPS immediately
- NO workers start
- Clear error message chain
- Exit code 1

---

## Implementation Checklist

### Step 1: Check Template (5-10 min)
- [ ] Add Template check after finding pkgPkg
- [ ] Check both `/build/synth/Template/usr/local/sbin/pkg` and package file
- [ ] Return early if both exist
- [ ] Log "already in Template, using existing"

### Step 2: Install pkg (10-15 min)
- [ ] Add Template installation after marking PkgFSuccess
- [ ] Verify package file exists
- [ ] Verify Template directory exists
- [ ] Run tar with correct exclusions
- [ ] Return error if tar fails
- [ ] Log success message
- [ ] Import `os/exec` package

### Step 3: Fix Error Handling (5-10 min)
- [ ] Fix `installPackages()` line 174: return error instead of continue
- [ ] Fix `installPackages()` line 179: return error for non-zero exit
- [ ] Fix `installMissingPackages()` line 252: return error instead of continue
- [ ] Review all error paths for consistency

### Testing (30-45 min)
- [ ] Run Test 1: Fresh bootstrap build
- [ ] Verify Template has pkg binary
- [ ] Run Test 2: Second build (skip bootstrap)
- [ ] Run Test 3: Build with dependencies
- [ ] Run Test 4: Force failure test
- [ ] Verify no "pkg: command not found" errors

### Documentation (10-15 min)
- [ ] Update DEVELOPMENT.md with commit hashes
- [ ] Mark Issue #3 as RESOLVED
- [ ] Update PHASE_1.5_FIDELITY_ANALYSIS.md if needed

---

## Commit Plan

### Commit 1: Check Template for Existing pkg
```
build: check Template for existing pkg before bootstrap

Before building ports-mgmt/pkg, check if it's already installed
in the Template directory. If pkg binary and package file both
exist, skip the bootstrap entirely.

This matches C dsynth behavior which checks PKGF_SUCCESS flag
before deciding to build pkg.

Fixes: Issue #3 (pkg Template installation) - Part 1/3

Co-authored-by: <AI Model> <email>
```

### Commit 2: Install pkg into Template
```
build: install pkg into Template after bootstrap build

After successfully building ports-mgmt/pkg, extract the package
into the Template directory using tar. This is CRITICAL because
all worker slots copy from Template, and ports need /usr/local/sbin/pkg
to install their dependencies.

Matches C dsynth build.c:273-285 behavior:
- Extract pkg package to Template with tar
- Exclude +* metadata files and man pages
- Fatal error if extraction fails

Without this, workers have no pkg binary and dependency installation
fails with "pkg: command not found".

Fixes: Issue #3 (pkg Template installation) - Part 2/3

Co-authored-by: <AI Model> <email>
```

### Commit 3: Fix Error Handling in Dependency Installation
```
build: stop build immediately on dependency install failure

Changed installPackages() and installMissingPackages() to return
errors instead of logging warnings and continuing. ANY pkg command
failure now stops the build immediately.

Previously, if pkg installation failed (e.g. "pkg: command not found"),
the code would log a warning and continue, leading to cryptic build
failures later.

Now:
- pkg exec error → return error (was: continue)
- pkg exit code != 0 → return error (was: log warning)
- Build stops immediately with clear error message

Fixes: Issue #3 (pkg Template installation) - Part 3/3

Co-authored-by: <AI Model> <email>
```

---

## Related Issues

- **Issue #2**: Missing ports-mgmt/pkg Bootstrap ✅ RESOLVED
  - Fixed pkg *building* first, but not installation
  - This issue (Issue #3) completes the bootstrap implementation

---

## References

### C dsynth Source Code

**File**: `usr.bin/dsynth/build.c`

**Lines 220-290** - pkg bootstrap and Template installation:
```c
scan = GetPkgPkg(&pkgs);

// Create Template
if (FetchOnlyOpt) {
    newtemplate = DoCreateTemplate(0);
} else if ((scan->flags & (PKGF_SUCCESS | PKGF_PACKAGED)) == 0) {
    newtemplate = DoCreateTemplate(1);
} else {
    newtemplate = DoCreateTemplate(0);
}

// Build pkg if needed
if ((scan->flags & (PKGF_SUCCESS | PKGF_PACKAGED)) == 0 && FetchOnlyOpt == 0) {
    build_list = scan;
    build_tail = &scan->build_next;
    startbuild(&build_list, &build_tail);
    while (RunningWorkers == 1)
        waitbuild(1, 0);

    if (scan->flags & PKGF_NOBUILD)
        dfatal("Unable to build 'pkg'");
    if (scan->flags & PKGF_ERROR)
        dfatal("Error building 'pkg'");
    if ((scan->flags & PKGF_SUCCESS) == 0)
        dfatal("Error building 'pkg'");
    newtemplate = 1;
}

// Install pkg into Template
if (newtemplate && FetchOnlyOpt == 0) {
    char *buf;
    int rc;

    asprintf(&buf,
         "cd %s/Template; "
         "tar --exclude '+*' --exclude '*/man/*' "
         "-xvzpf %s/%s > /dev/null 2>&1",
         BuildBase,
         RepositoryPath,
         scan->pkgfile);
    rc = system(buf);
    if (rc)
        dfatal("Command failed: %s\n", buf);
    freestrp(&buf);
}
```

---

## Expected Final State

After all fixes, the build structure should be:

```
/build/synth/
├── Template/
│   ├── usr/
│   │   ├── local/
│   │   │   ├── sbin/
│   │   │   │   ├── pkg           ← MUST EXIST
│   │   │   │   ├── pkg-static    ← MUST EXIST
│   │   │   ├── lib/
│   │   │   │   ├── libpkg.so.*   ← MUST EXIST
├── packages/
│   ├── All/
│   │   ├── pkg-*.pkg              ← MUST EXIST
│   │   ├── indexinfo-*.pkg        ← Created by test
├── builds.db                      ← Build records
├── SL00/ ... SL03/                ← Worker slots (cleaned after build)
```

**Worker slot verification** (during active build):
```bash
$ ls -la /build/synth/SL00/usr/local/sbin/pkg
lrwxr-xr-x  1 root  wheel  32 Nov 30 23:00 /build/synth/SL00/usr/local/sbin/pkg -> ../../Template/usr/local/sbin/pkg
```

---

## Timeline

**Total Estimated Time**: 1-1.5 hours

- Step 1 Implementation: 5-10 min
- Step 2 Implementation: 10-15 min
- Step 3 Implementation: 5-10 min
- Testing (all 4 tests): 30-45 min
- Documentation updates: 10-15 min
- Code review & cleanup: 10 min

---

## Success Criteria

- [ ] `/build/synth/Template/usr/local/sbin/pkg` exists after bootstrap
- [ ] Second build skips bootstrap with "already in Template" message
- [ ] Port with dependencies builds successfully (misc/help2man)
- [ ] No "pkg: command not found" errors in any logs
- [ ] Forced pkg build failure stops entire build immediately
- [ ] Dependency install failure stops build immediately
- [ ] All 3 commits include Co-authored-by trailers
- [ ] DEVELOPMENT.md Issue #3 marked RESOLVED with commit hashes
- [ ] VM testing shows 100% success rate

---

**End of Issue #3 Documentation**
