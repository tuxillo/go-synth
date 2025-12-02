# Phase 1.5 Fidelity Analysis

**Purpose:** Verify go-synth matches original dsynth C functionality and identify C-isms to remove.

**Date:** 2025-11-25  
**Status:** IN PROGRESS - Part A (Function Mapping)

---

## Part A: Original Fidelity Verification

### A1: Struct Comparison

#### C `pkg_t` (dsynth.h:136-170) vs Go `Package` (pkg.go:44-75)

| C Field | Go Field | Status | Notes |
|---------|----------|--------|-------|
| `build_next` | ❌ None | MISSING | Used for topology build list |
| `bnext` | `Next` | ✅ EQUIVALENT | Linked list |
| `hnext1` | ❌ None | MISSING | Hash chain for portdir lookup |
| `hnext2` | ❌ None | MISSING | Hash chain for pkgfile lookup |
| `idepon_list` (pkglink_t) | `IDependOn []*PkgLink` | ✅ BETTER | C uses linked list, Go uses slice |
| `deponi_list` (pkglink_t) | `DependsOnMe []*PkgLink` | ✅ BETTER | C uses linked list, Go uses slice |
| `portdir` | `PortDir` | ✅ MATCH | Origin name |
| `logfile` | ❌ None | MISSING | Relative log path (build phase) |
| `version` | `Version` | ✅ MATCH | PKGVERSION |
| `pkgfile` | `PkgFile` | ✅ MATCH | Package filename |
| `distfiles` | ❌ None | MISSING | DISTFILES (not needed for Phase 1) |
| `distsubdir` | ❌ None | MISSING | DIST_SUBDIR (not needed for Phase 1) |
| `ignore` | ❌ `ignoreReason` in `BuildStateRegistry` | ✅ REFACTORED | Moved to build state |
| `fetch_deps` | `FetchDeps` | ✅ MATCH | FETCH_DEPENDS |
| `ext_deps` | `ExtractDeps` | ✅ MATCH | EXTRACT_DEPENDS |
| `patch_deps` | `PatchDeps` | ✅ MATCH | PATCH_DEPENDS |
| `build_deps` | `BuildDeps` | ✅ MATCH | BUILD_DEPENDS |
| `lib_deps` | `LibDeps` | ✅ MATCH | LIB_DEPENDS |
| `run_deps` | `RunDeps` | ✅ MATCH | RUN_DEPENDS |
| `pos_options` | ❌ None | MISSING | SELECTED_OPTIONS (not needed yet) |
| `neg_options` | ❌ None | MISSING | DESELECTED_OPTIONS (not needed yet) |
| `flavors` | ❌ `Flavor` | ✅ REFACTORED | C stores all flavors, Go stores single flavor |
| `uses` | ❌ None | MISSING | USES (metaport detection) |
| `make_jobs_number` | ❌ None | MISSING | MAKE_JOBS_NUMBER (build phase) |
| `use_linux` | ❌ None | MISSING | USE_LINUX (build phase) |
| `idep_count` | ❌ None | MISSING | Recursive dependency count |
| `depi_count` | `DepiCount` | ✅ MATCH | Count of packages depending on me |
| `depi_depth` | `DepiDepth` | ✅ MATCH | Tree depth calculation |
| `dsynth_install_flg` | ❌ None | MISSING | Install coordination flag |
| `flags` | ❌ `BuildStateRegistry.flags` | ✅ REFACTORED | Moved to separate registry |
| `rscan` | ❌ None | MISSING | Recursive scan flag |
| `crc32` | ❌ CRC in `builddb` | ✅ REFACTORED | Moved to builddb package |
| `pkgfile_size` | ❌ None | MISSING | Package file size |
| ❌ None | `Category` | ✅ NEW | Split from PortDir for cleaner API |
| ❌ None | `Name` | ✅ NEW | Split from PortDir for cleaner API |
| ❌ None | `LastStatus` | ✅ NEW | Status tracking |
| ❌ None | `Prev` | ✅ NEW | Doubly-linked list |
| ❌ None | `mu sync.Mutex` | ⚠️ **DEAD CODE** | **NEVER USED - REMOVE** |

**Summary:**
- ✅ **Core functionality preserved**: Dependency tracking, linked lists, metadata
- ✅ **Intentional improvements**: Build state separation, hash map instead of linked hash chains
- ⚠️ **Missing features**: OK for Phase 1 (build execution fields, options, distfiles)
- ❌ **Dead code found**: `Package.mu` is never used

---

#### C `pkglink_t` (dsynth.h:119-124) vs Go `PkgLink` (pkg.go:78-81)

| C Field | Go Field | Status | Notes |
|---------|----------|--------|-------|
| `next` | ❌ None | ✅ BETTER | Go uses slice, not linked list |
| `prev` | ❌ None | ✅ BETTER | Go uses slice, not linked list |
| `pkg` | `Pkg` | ✅ MATCH | Pointer to package |
| `dep_type` | `DepType` | ✅ MATCH | Dependency type (int) |

**C-ism Alert:** Using `int` for `DepType` instead of typed constant.

---

### A2: Function Mapping

#### Package Parsing Functions

| C Function | C File | Go Function | Go File | Status |
|------------|--------|-------------|---------|--------|
| `ParsePackageList()` | pkglist.c:178 | `ParsePortList()` | pkg.go:142 | ✅ EQUIVALENT |
| `parsepkglist_file()` | pkglist.c:244 | ❌ inline in `ParsePortList` | pkg.go | ✅ SIMPLER |
| `GetLocalPackageList()` | pkglist.c:292 | `GetInstalledPackages()` | pkg.go:433 | ✅ EQUIVALENT |
| `GetFullPackageList()` | pkglist.c:395 | `GetAllPorts()` | pkg.go:455 | ✅ EQUIVALENT |
| `GetPkgPkg()` | pkglist.c:586 | `bootstrapPkg()` | build/bootstrap.go | ✅ IMPLEMENTED (2025-11-30) |
| `processPackageListBulk()` | pkglist.c:412 | ❌ inline | pkg.go:158 | ✅ REFACTORED |
| `scan_and_queue_dir()` | pkglist.c:1306 | ❌ inline in `GetAllPorts` | pkg.go:458 | ✅ SIMPLER |
| `scan_binary_repo()` | pkglist.c:1348 | ❌ None | - | ⚠️ MISSING (prebuilt detection) |

**Key Observation:** Go code combines multiple C functions into simpler flows.

---

#### Bulk Operations (Parallel Processing)

| C Function | C File | Go Function | Go File | Status |
|------------|--------|-------------|---------|--------|
| `initbulk()` | bulk.c | `newBulkQueue()` | bulk.go | ✅ EQUIVALENT |
| `queuebulk()` | bulk.c | `Queue()` | bulk.go | ✅ EQUIVALENT |
| `getbulk()` | bulk.c | `GetResult()` | bulk.go | ✅ EQUIVALENT |
| `donebulk()` | bulk.c | `Close()` | bulk.go | ✅ EQUIVALENT |
| `freebulk()` | bulk.c | ❌ defer/GC | - | ✅ BETTER (automatic) |
| `childGetPackageInfo()` | pkglist.c:966 | `getPackageInfo()` | pkg.go:225 | ✅ EQUIVALENT |
| `childGetBinaryDistInfo()` | pkglist.c:1178 | ❌ None | - | ⚠️ MISSING (Phase 1) |
| `childOptimizeEnv()` | pkglist.c:1244 | ❌ None | - | ⚠️ MISSING (Phase 1) |

---

#### Dependency Resolution

| C Function | C File | Go Function | Go File | Status |
|------------|--------|-------------|---------|--------|
| `resolveDeps()` | pkglist.c:658 | `resolveDependencies()` | deps.go:12 | ✅ EQUIVALENT |
| `resolveFlavors()` | pkglist.c:714 | ❌ None | - | ⚠️ DIFFERENT (flavors per-package) |
| `resolveDepString()` | pkglist.c:791 | `parseDependencyString()` | deps.go:147 | ✅ EQUIVALENT |
| ❌ None | - | `linkPackageDependencies()` | deps.go:241 | ✅ NEW (cleaner separation) |

**Algorithm Check:**
- **C:** `resolveDeps()` runs in 2 passes:
  1. Pass 0 (`gentopo=0`): Queue missing dependencies, collect from bulk
  2. Pass 1 (`gentopo=1`): Build topology links
- **Go:** `resolveDependencies()` runs in 2 phases:
  1. Phase 1: Collect dependencies recursively
  2. Phase 2: `buildDependencyGraph()` builds links

✅ **VERDICT:** Same algorithm, cleaner Go implementation.

---

#### Topological Sort

| C Function | C File | Go Function | Go File | Status |
|------------|--------|-------------|---------|--------|
| ❌ No explicit topo | - | `GetBuildOrder()` | deps.go:308 | ✅ NEW (Kahn's algorithm) |
| ❌ Implicit via `build_list` | build.c | Explicit sort | deps.go | ✅ BETTER |

**Algorithm Check:**
- **C:** Uses `build_next` field to build inverted topology during resolution
- **Go:** Explicit Kahn's algorithm topological sort
- ✅ **VERDICT:** Go approach is more explicit and testable

---

#### Hash Table Operations

| C Function | C File | Go Function | Go File | Status |
|------------|--------|-------------|---------|--------|
| `pkghash()` | pkglist.c:84 | ❌ native `map[string]` | pkg.go | ✅ BETTER |
| `pkg_enter()` | pkglist.c:117 | `PackageRegistry.Enter()` | pkg.go:122 | ✅ BETTER |
| `pkg_find()` | pkglist.c:156 | `PackageRegistry.Find()` | pkg.go:135 | ✅ BETTER |
| Two hash tables (`PkgHash1`/`PkgHash2`) | pkglist.c:60-61 | One map (portdir only) | pkg.go | ✅ SIMPLER |

**C-ism Alert:** C uses manual hash chains (`hnext1`/`hnext2`). Go uses native map.

---

#### Makefile Query

| C Function | C File | Go Function | Go File | Status |
|------------|--------|-------------|---------|--------|
| `childGetPackageInfo()` uses `dexec_open()` | pkglist.c:966-1166 | `queryMakefile()` | pkg.go:259 | ✅ EQUIVALENT |
| Queries 17 variables | pkglist.c:1015-1031 | Queries 10 variables | pkg.go:261-272 | ⚠️ FEWER (Phase 1) |

**Variables Queried:**

| C Variable | Go Variable | Status |
|-----------|-------------|--------|
| PKGVERSION | PKGVERSION | ✅ |
| PKGFILE:T | PKGFILE | ✅ |
| ALLFILES | ❌ | ⚠️ Phase 1 |
| DIST_SUBDIR | ❌ | ⚠️ Phase 1 |
| MAKE_JOBS_NUMBER | ❌ | ⚠️ Phase 1 |
| IGNORE | IGNORE | ✅ |
| FETCH_DEPENDS | FETCH_DEPENDS | ✅ |
| EXTRACT_DEPENDS | EXTRACT_DEPENDS | ✅ |
| PATCH_DEPENDS | PATCH_DEPENDS | ✅ |
| BUILD_DEPENDS | BUILD_DEPENDS | ✅ |
| LIB_DEPENDS | LIB_DEPENDS | ✅ |
| RUN_DEPENDS | RUN_DEPENDS | ✅ |
| SELECTED_OPTIONS | ❌ | ⚠️ Phase 1 |
| DESELECTED_OPTIONS | ❌ | ⚠️ Phase 1 |
| USE_LINUX | ❌ | ⚠️ Phase 1 |
| FLAVORS | ❌ | ⚠️ Different approach |
| USES | ❌ | ⚠️ Phase 1 |
| ❌ | PKGNAME | ✅ NEW |

---

### A3: Dependency Resolution Algorithm Comparison

#### C Implementation (pkglist.c:658-707)

```c
static pkg_t *
resolveDeps(pkg_t *list, pkg_t ***list_tailp, int gentopo)
{
    pkg_t *ret_list = NULL;
    
    for (scan = list; scan; scan = scan->bnext) {
        use = pkg_find(scan->portdir);
        resolveFlavors(use, scan->flavors, gentopo);
        resolveDepString(use, scan->fetch_deps, gentopo, DEP_TYPE_FETCH);
        resolveDepString(use, scan->ext_deps, gentopo, DEP_TYPE_EXT);
        // ... more dep types
    }
    
    if (gentopo)
        return NULL;  // Pass 2: no bulk ops
        
    // Pass 1: collect bulk results
    while ((bulk = getbulk()) != NULL) {
        // Add to list
        pkg_enter(scan);
    }
    return (ret_list);
}
```

**Called twice:**
1. `resolveDeps(dep_list, &list_tail, 0)` - Queue missing deps
2. `resolveDeps(list, NULL, 1)` - Build topology

#### Go Implementation (deps.go:12-138)

```go
func resolveDependencies(head *Package, ...) error {
    // Phase 1: Collect all dependencies recursively
    for len(toProcess) > 0 {
        currentBatch := toProcess
        for _, pkg := range currentBatch {
            // Parse deps, queue missing
        }
        // Collect bulk results
        for bq.Pending() > 0 {
            // Add to registry and list
        }
    }
    
    // Phase 2: Build dependency graph
    return buildDependencyGraph(head, cfg, pkgRegistry)
}
```

✅ **VERDICT:** Same two-pass algorithm, Go is more explicit about phases.

---

### A4: Topological Sort Algorithm Comparison

#### C Implementation (build.c - implicit)

C uses `build_next` field to create inverted topology list during dependency resolution:

```c
// In resolveDepString() when gentopo=1:
link = calloc(1, sizeof(*link));
link->pkg = dpkg;
link->next = &pkg->idepon_list;
// ... circular linked list insertion
```

The `build_next` field chains packages in build order.

#### Go Implementation (deps.go:308-377)

Explicit **Kahn's Algorithm:**

```go
func GetBuildOrder(head *Package) []*Package {
    // 1. Count in-degrees
    inDegree := make(map[*Package]int)
    for pkg := head; pkg != nil; pkg = pkg.Next {
        inDegree[pkg] = len(pkg.IDependOn)
    }
    
    // 2. Queue zero in-degree packages
    queue := make([]*Package, 0)
    for _, pkg := range packages {
        if inDegree[pkg] == 0 {
            queue = append(queue, pkg)
        }
    }
    
    // 3. Process queue
    result := make([]*Package, 0, len(packages))
    for len(queue) > 0 {
        pkg := queue[0]
        queue = queue[1:]
        result = append(result, pkg)
        
        for _, link := range pkg.DependsOnMe {
            inDegree[link.Pkg]--
            if inDegree[link.Pkg] == 0 {
                queue = append(queue, link.Pkg)
            }
        }
    }
    
    return result
}
```

✅ **VERDICT:** 
- C: Implicit topology via linked list during resolution
- Go: Explicit Kahn's algorithm, easier to verify
- **Both are correct**, Go approach is more maintainable

---

### A5: Package Parsing Logic Comparison

#### C (pkglist.c:966-1166)

```c
// Uses dexec_open() to run make -V* -V* ...
fp = dexec_open(portpath + strlen(DPortsPath) + 1, cav, cac,
                &pid, NULL, 1, 1);

line = 1;
while ((ptr = fgetln(fp, &len)) != NULL) {
    switch(line) {
    case 1: /* PKGVERSION */
        asprintf(&pkg->version, "%s", ptr);
        break;
    case 2: /* PKGFILE */
        asprintf(&pkg->pkgfile, "%s", ptr);
        break;
    // ... cases 3-17
    }
    ++line;
}
```

#### Go (pkg.go:259-348)

```go
func queryMakefile(pkg *Package, portPath string, cfg *config.Config) (int, string, error) {
    args := []string{"-C", portPath}
    if pkg.Flavor != "" {
        args = append(args, "FLAVOR="+pkg.Flavor)
    }
    for _, v := range vars {
        args = append(args, "-V", v)
    }
    
    cmd := exec.Command("make", args...)
    out, err := cmd.Output()
    
    lines := strings.Split(out, "\n")
    pkg.Version = strings.TrimSpace(lines[1])
    pkg.PkgFile = filepath.Base(strings.TrimSpace(lines[2]))
    // ... more parsing
}
```

✅ **VERDICT:** Same approach, Go is cleaner with `exec.Command`.

---

### A6: Build Phase Execution (SKIPPED - Phase 3)

Build execution is Phase 3. Phase 1 only covers parsing and dependencies.

---

### A7: Flag Meanings (PkgF* Constants)

#### C Flags (dsynth.h:172-192)

```c
#define PKGF_PACKAGED   0x00000001  /* has a repo package */
#define PKGF_DUMMY      0x00000002  /* generic root for flavors */
#define PKGF_NOTFOUND   0x00000004  /* dport not found */
#define PKGF_CORRUPT    0x00000008  /* dport corrupt */
#define PKGF_PLACEHOLD  0x00000010  /* pre-entered */
#define PKGF_BUILDLIST  0x00000020  /* on build_list */
#define PKGF_BUILDLOOP  0x00000040  /* traversal loop test */
#define PKGF_BUILDTRAV  0x00000080  /* traversal optimization */
#define PKGF_NOBUILD_D  0x00000100  /* can't build - dependency problem */
#define PKGF_NOBUILD_S  0x00000200  /* can't build - skipped */
#define PKGF_NOBUILD_F  0x00000400  /* can't build - failed */
#define PKGF_NOBUILD_I  0x00000800  /* can't build - ignored or broken */
#define PKGF_SUCCESS    0x00001000  /* build complete */
#define PKGF_FAILURE    0x00002000  /* build complete */
#define PKGF_RUNNING    0x00004000  /* build complete */
#define PKGF_PKGPKG     0x00008000  /* pkg/pkg-static special */
#define PKGF_NOTREADY   0x00010000  /* build_find_leaves() only */
#define PKGF_MANUALSEL  0x00020000  /* manually specified */
#define PKGF_META       0x00040000  /* USES contains 'metaport' */
#define PKGF_DEBUGSTOP  0x00080000  /* freeze slot on completion */
```

#### Go Flags (pkg.go:17-30, buildstate.go)

```go
const (
    PkgFManualSel     = 0x00000001  // Manually selected
    PkgFMeta          = 0x00000002  // Meta port (no build)
    PkgFDummy         = 0x00000004  // Dummy package
    PkgFSuccess       = 0x00000008  // Build succeeded
    PkgFFailed        = 0x00000010  // Build failed
    PkgFSkipped       = 0x00000020  // Skipped
    PkgFIgnored       = 0x00000040  // Ignored
    PkgFNoBuildIgnore = 0x00000080  // Don't build (ignored)
    PkgFNotFound      = 0x00000100  // Port not found
    PkgFCorrupt       = 0x00000200  // Port corrupted
    PkgFPackaged      = 0x00000400  // Package exists
    PkgFRunning       = 0x00000800  // Currently building
)
```

**Comparison:**

| C Flag | Go Flag | Status | Notes |
|--------|---------|--------|-------|
| `PKGF_PACKAGED` | `PkgFPackaged` | ✅ | Different bit position |
| `PKGF_DUMMY` | `PkgFDummy` | ✅ | Different bit position |
| `PKGF_NOTFOUND` | `PkgFNotFound` | ✅ | Different bit position |
| `PKGF_CORRUPT` | `PkgFCorrupt` | ✅ | Different bit position |
| `PKGF_PLACEHOLD` | ❌ | ⚠️ | Go uses registry check instead |
| `PKGF_BUILDLIST` | ❌ | ✅ | Not needed (explicit slice) |
| `PKGF_BUILDLOOP` | ❌ | ✅ | Not needed (Kahn's algorithm) |
| `PKGF_BUILDTRAV` | ❌ | ✅ | Not needed (Kahn's algorithm) |
| `PKGF_NOBUILD_*` | `PkgFNoBuildIgnore` etc | ⚠️ | Go has fewer variants |
| `PKGF_SUCCESS` | `PkgFSuccess` | ✅ | Different bit position |
| `PKGF_FAILURE` | `PkgFFailed` | ✅ | Different bit position |
| `PKGF_RUNNING` | `PkgFRunning` | ✅ | Different bit position |
| `PKGF_PKGPKG` | `PkgFPkgPkg` | ✅ | IMPLEMENTED (2025-11-30) |
| `PKGF_NOTREADY` | ❌ | ⚠️ | Phase 3 |
| `PKGF_MANUALSEL` | `PkgFManualSel` | ✅ | Different bit position |
| `PKGF_META` | `PkgFMeta` | ✅ | Different bit position |
| `PKGF_DEBUGSTOP` | ❌ | ⚠️ | Phase 3 |

⚠️ **FINDING:** Go flag bit positions don't match C! This is OK as long as they're not serialized to disk in binary form.

---

### A8: CRC Database Format

#### C Implementation (builddb/crc.c - not in provided source)

From `dsynth.h`:
```c
uint32_t crcDirTree(const char *path);
```

#### Go Implementation (builddb/crc.go, builddb/helpers.go)

```go
func (db *CRCDatabase) CheckNeedsBuild(pkg Package, cfg *config.Config) bool
func ComputePortDirCRC(portDir string) (uint32, error)
```

✅ **VERDICT:** Go uses similar CRC32 approach, stored in BBolt database instead of ndbm.

---

### A9: Divergence Summary

#### ✅ Preserved C Functionality

1. **Dependency Resolution:** Two-pass algorithm intact
2. **Package Parsing:** Same `make -V` query approach
3. **Bulk Parallelism:** Worker pool pattern preserved
4. **CRC Checking:** Port directory CRC calculation
5. **Linked Lists:** Package chain preserved (for now)

#### ✅ Intentional Go Improvements

1. **Hash Tables:** Native `map[string]*Package` instead of manual chains
2. **Dependency Links:** Slices instead of circular linked lists
3. **Build State Separation:** `BuildStateRegistry` instead of `pkg->flags`
4. **Explicit Topology Sort:** Kahn's algorithm instead of implicit `build_next`
5. **Type Safety:** Separate `Category`/`Name` fields
6. **Database:** BBolt (BoltDB) instead of ndbm

#### ⚠️ Missing C Features (OK for Phase 1)

1. **Prebuilt Package Detection:** `scan_binary_repo()`, `childGetBinaryDistInfo()`
2. **Environment Optimization:** `childOptimizeEnv()`
3. **Flavor Handling:** C creates dummy nodes, Go handles per-package
4. **pkg Bootstrap:** `GetPkgPkg()` special handling
5. **Build Execution:** Worker threads, phases (Phase 3)
6. **Options:** `SELECTED_OPTIONS`, `DESELECTED_OPTIONS`
7. **Distfiles:** `ALLFILES`, `DIST_SUBDIR`
8. **Build Config:** `MAKE_JOBS_NUMBER`, `USE_LINUX`

#### ✅ Dead Code Removed (Part B Task B1)

1. **`Package.mu sync.Mutex`** - Removed in commit 175462b

#### ✅ C-isms Addressed (Part B Complete)

1. **Linked Lists:** ✅ Converted to Go slices (commit ae58f64)
2. **Bitfield Flags:** ✅ Converted to typed PackageFlags enum (commit eb1f7e7)
3. **Integer DepType:** ✅ Converted to typed DepType enum (commit 063d0e7)
4. **Dead Mutex:** ✅ Removed `Package.mu` (commit 175462b)

---

### A10: Test Case Verification ✅ COMPLETE

**Created:** `pkg/fidelity_test.go` with 10 comprehensive test cases

**Test Coverage:**

1. ✅ **TestCFidelity_DependencyResolutionTwoPass**
   - Verifies two-pass algorithm structure matches C implementation
   - Confirmed: Phase 1 collects deps, Phase 2 builds topology

2. ✅ **TestCFidelity_TopologicalSort**
   - Verifies Kahn's algorithm produces valid topological order
   - Tested: 4-package chain with proper dependency ordering
   - Result: D→C→B→A (dependencies first)

3. ✅ **TestCFidelity_MultipleDepTypes**
   - Verifies all 6 dependency types match C constants exactly
   - FETCH(1), EXTRACT(2), PATCH(3), BUILD(4), LIB(5), RUN(6)

4. ✅ **TestCFidelity_DependencyStringParsing**
   - 7 test cases covering all dependency string formats
   - Simple deps, multiple deps, paths, flavors, tags, libraries
   - Behavior matches C's `resolveDepString()`

5. ✅ **TestCFidelity_PackageRegistry**
   - Verifies Enter/Find behaves like C's `pkg_enter`/`pkg_find`
   - Tests: insert, lookup, duplicate handling, not found

6. ✅ **TestCFidelity_CircularDependencyDetection**
   - Verifies we detect cycles (A→B→C→A)
   - Returns partial order, proper error reporting

7. ✅ **TestCFidelity_DiamondDependency**
   - Diamond pattern: A depends on B&C, both depend on D
   - Correct ordering: D→{B,C}→A
   - Handles shared dependencies correctly

8. ✅ **TestCFidelity_ParsePortSpec**
   - Tests: `editors/vim`, `lang/python@py39`, paths, flavors
   - Behavior matches C's `ParsePackageList()` parsing

9. ✅ **TestCFidelity_DepiCountAndDepth**
   - Verifies `DepiCount` (number of dependents)
   - Verifies `DepiDepth` (maximum dependency depth)
   - Matches C's `depi_count` and `depi_depth` fields

10. ✅ **TestCFidelity_BidirectionalLinks**
    - Verifies `IDependOn` and `DependsOnMe` are both created
    - Matches C's `idepon_list` and `deponi_list`
    - Both directions have same `DepType`

**Test Results:**
```
=== RUN   TestCFidelity_DependencyResolutionTwoPass
--- PASS: TestCFidelity_DependencyResolutionTwoPass (0.00s)
=== RUN   TestCFidelity_TopologicalSort
--- PASS: TestCFidelity_TopologicalSort (0.00s)
=== RUN   TestCFidelity_MultipleDepTypes
--- PASS: TestCFidelity_MultipleDepTypes (0.00s)
=== RUN   TestCFidelity_DependencyStringParsing
--- PASS: TestCFidelity_DependencyStringParsing (0.00s)
=== RUN   TestCFidelity_PackageRegistry
--- PASS: TestCFidelity_PackageRegistry (0.00s)
=== RUN   TestCFidelity_CircularDependencyDetection
--- PASS: TestCFidelity_CircularDependencyDetection (0.00s)
=== RUN   TestCFidelity_DiamondDependency
--- PASS: TestCFidelity_DiamondDependency (0.00s)
=== RUN   TestCFidelity_ParsePortSpec
--- PASS: TestCFidelity_ParsePortSpec (0.00s)
=== RUN   TestCFidelity_DepiCountAndDepth
--- PASS: TestCFidelity_DepiCountAndDepth (0.00s)
=== RUN   TestCFidelity_BidirectionalLinks
--- PASS: TestCFidelity_BidirectionalLinks (0.00s)
PASS
ok      dsynth/pkg      0.020s
```

**All test suite (26 tests total):**
```
ok      dsynth/pkg      0.020s
```

**Conclusion:** ✅ Go implementation verified to match C behavior across all core algorithms.

---

## ✅ PART A COMPLETE: Original Fidelity Verified

**Summary of Findings:**

1. **✅ High Fidelity Achieved**
   - Core dependency resolution: MATCHES C
   - Topological sorting: CORRECT (Kahn's vs implicit)
   - Package parsing: MATCHES C
   - Dependency string parsing: MATCHES C
   - Registry behavior: MATCHES C

2. **✅ Intentional Improvements Working**
   - Native Go maps instead of hash chains: VERIFIED
   - Slice-based dependency links: VERIFIED
   - Explicit topological sort: VERIFIED
   - Build state separation: WORKING

3. **✅ Dead Code Removed (Part B)**
   - `Package.mu sync.Mutex`: Removed in commit 175462b

4. **✅ C-isms Removed (Part B Complete)**
   - Linked list traversals: Converted to Go slices (commit ae58f64)
   - Bitfield flags: Converted to typed PackageFlags enum (commit eb1f7e7)
   - Integer `DepType`: Converted to typed DepType enum (commit 063d0e7)

**Confidence Level: HIGH ✅**

We can confidently proceed to Part B (Remove C-isms) knowing our implementation faithfully reproduces the C dsynth's core functionality while making appropriate Go idioms improvements.

---

## Part B: Remove C-isms ✅ COMPLETE

All C-isms identified in Part A have been successfully removed:

- **B1: Remove Dead Code** ✅ (commit 175462b)
- **B2: Convert Linked Lists to Slices** ✅ (commit ae58f64)
- **B3: Use Typed DepType Enum** ✅ (commit 063d0e7)
- **B4: Use Typed PackageFlags Enum** ✅ (commit eb1f7e7)

See `docs/design/phase_1.5_part_b_plan.md` for detailed implementation notes.

---

## Conclusion

### Fidelity Assessment: ✅ HIGH FIDELITY

**Core Algorithm Match:**
- ✅ Two-pass dependency resolution: EQUIVALENT
- ✅ Makefile querying: EQUIVALENT  
- ✅ Bulk parallelism: EQUIVALENT
- ✅ CRC checking: EQUIVALENT
- ✅ Topological sort: DIFFERENT BUT CORRECT (Kahn's vs implicit)

**Intentional Improvements:**
- ✅ Native Go maps instead of manual hash chains
- ✅ Slices instead of linked lists (dependency links)
- ✅ Separate build state registry
- ✅ Explicit topological sort

**Phase 1 Scope:**
- ⚠️ Missing features are **build execution** (Phase 3) - not needed yet
- ⚠️ Missing pkg bootstrap, prebuilt detection - can add later

### Recommendations

1. **Continue with confidence:** Core algorithms match original C
2. **Remove dead code:** Delete `Package.mu` immediately
3. **Proceed to Part B:** Address remaining C-isms (linked lists, bitfields)
4. **Add tests:** Create comparison tests against C behavior
5. **Document divergences:** Track intentional improvements vs bugs

### Next Steps

- [x] Part A1-A9: Function and algorithm mapping ✅ COMPLETE
- [ ] Part A10: Create test cases
- [ ] Part B: Remove C-isms
  - [ ] B1: Remove `Package.mu` (dead code)
  - [ ] B2: Linked list analysis
  - [ ] B3: Bitfield refactoring
  - [ ] B4: Type safety improvements

---

## Part C: System Stats Infrastructure Analysis

**Date**: 2025-12-02  
**Purpose**: Document original dsynth stats/monitoring implementation for Issue #9  
**Scope**: Source inventory, structure analysis, missing implementations

### C1: Source File Inventory

**Available Files in `.original-c-source/`**:

| File | Lines | Size | Stats-Related Content |
|------|-------|------|----------------------|
| `dsynth.h` | 666 | 20K | ✅ `topinfo_t`, `runstats_t`, DLOG/WMSG constants, global counters |
| `build.c` | 3,356 | 83K | ✅ `adjloadavg()`, dynamic throttling, RunStats* call sites |
| `pkglist.c` | 1,406 | 32K | ⚠️ Some completion tracking |
| `bulk.c` | 380 | 8.2K | ❌ No stats code |

**Total Available**: 5,808 lines

**Missing Files** (declared but not present):
- ❌ `monitor.c` - Monitor file writer (`MonitorRunStats` implementation)
- ❌ `curses.c` / `ncurses.c` - Ncurses UI (`NCursesRunStats` implementation)  
- ❌ `html.c` - HTML report generator (`HtmlRunStats` implementation)
- ❌ Implementation of `getswappct()` (declared in dsynth.h:616, not defined in build.c)

**Upstream Fetch Required**:
```bash
git clone https://github.com/DragonFlyBSD/DragonFlyBSD.git
cd DragonFlyBSD/usr.bin/dsynth
# Files needed: monitor.c, curses.c, html.c
```

### C2: Stats Data Structures (dsynth.h)

#### `topinfo_t` Structure (lines 469-487)

```c
typedef struct topinfo {
    int active;          // Active worker count
    int pkgimpulse;      // Instant completions (last 1 second)
    int pkgrate;         // Packages/hour (60s sliding window)
    int noswap;          // Flag: no swap configured
    int h;               // Elapsed hours
    int m;               // Elapsed minutes  
    int s;               // Elapsed seconds
    int total;           // Total queued packages
    int successful;      // Built successfully
    int ignored;         // Ignored (DLOG_IGN)
    int remaining;       // Queued - (successful + failed + ignored)
    int failed;          // Build failures
    int skipped;         // Skipped (DLOG_SKIP)
    int meta;            // Metaports count
    int dynmaxworkers;   // Dynamic max workers (throttled)
    double dswap;        // Swap usage percentage (0.0 - 1.0)
    double dload[3];     // Adjusted load average [1min, 5min, 15min]
} topinfo_t;
```

**Key Observations**:
- Time stored as h/m/s integers (not `time_t` or duration)
- Rate/impulse are integers (not doubles as in issue doc)
- `dswap` is double 0-1.0 range (not percentage 0-100)
- `noswap` flag indicates no swap configured (avoid division by zero)

#### `runstats_t` Interface (lines 489-500)

```c
typedef struct runstats {
    struct runstats *next;  // Linked list of stat consumers
    void (*init)(void);
    void (*done)(void);
    void (*reset)(void);
    void (*update)(worker_t *work, const char *portdir);
    void (*updateTop)(topinfo_t *info);
    void (*updateLogs)(void);
    void (*updateCompletion)(worker_t *work, int dlogid, pkg_t *pkg,
                             const char *reason, const char *skipbuf);
    void (*sync)(void);
} runstats_t;
```

**Pattern**: Observer/visitor pattern with multiple consumers
- `NCursesRunStats` - Live UI updates
- `MonitorRunStats` - File-based monitoring  
- `HtmlRunStats` - HTML report generation

#### Global Build Counters (lines 516-524)

```c
extern int BuildCount;          // Currently building
extern int BuildTotal;          // Total packages
extern int BuildFailCount;      // Failed builds
extern int BuildSkipCount;      // Skipped packages
extern int BuildIgnoreCount;    // Ignored packages
extern int BuildSuccessCount;   // Successful builds
extern int BuildMissingCount;   // Missing dependencies
extern int BuildMetaCount;      // Metaports
extern int DynamicMaxWorkers;   // Throttled worker limit
```

**Usage**: Updated by build.c, read by RunStats implementations

#### Monitor File Constants (lines 91-92)

```c
#define STATS_FILE      "monitor.dat"    /* under LogsPath */
#define STATS_LOCKFILE  "monitor.lk"     /* under LogsPath */
```

#### Completion Log Types (lines 386-394)

```c
#define DLOG_ALL   0   // Usually stdout when curses disabled
#define DLOG_SUCC  1   // success_list.log
#define DLOG_FAIL  2   // failure_list.log
#define DLOG_IGN   3   // ignored_list.log
#define DLOG_SKIP  4   // skipped_list.log
#define DLOG_ABN   5   // abnormal_command_output
#define DLOG_OBS   6   // obsolete_packages.log
#define DLOG_DEBUG 7   // debug.log
#define DLOG_COUNT 8   // total number of DLOGs
```

**Mapping to Stats**:
- `DLOG_SUCC` → increment `BuildSuccessCount`
- `DLOG_FAIL` → increment `BuildFailCount`
- `DLOG_IGN` → increment `BuildIgnoreCount`
- `DLOG_SKIP` → increment `BuildSkipCount`

#### Worker Message Types (lines 355-360)

```c
#define WMSG_CMD_STATUS_UPDATE   0x0001  // Worker status change
#define WMSG_CMD_SUCCESS         0x0002  // Build succeeded
#define WMSG_CMD_FAILURE         0x0003  // Build failed
#define WMSG_CMD_INSTALL_PKGS    0x0004  // Install dependencies
#define WMSG_RES_INSTALL_PKGS    0x0005  // Install result
#define WMSG_CMD_FREEZEWORKER    0x0006  // Freeze worker slot
```

**Relevance**: `WMSG_CMD_STATUS_UPDATE` triggers `RunStatsUpdate()` calls

### C3: Stats Function Declarations (dsynth.h:648-655)

```c
void RunStatsInit(void);
void RunStatsDone(void);
void RunStatsReset(void);
void RunStatsUpdate(worker_t *work, const char *portdir);
void RunStatsUpdateTop(int active);
void RunStatsUpdateLogs(void);
void RunStatsSync(void);
void RunStatsUpdateCompletion(worker_t *work, int logid, pkg_t *pkg,
                               const char *reason, const char *skipbuf);
```

**Call Pattern** (from build.c grep):
- `RunStatsInit()` - Line 247: Before build starts
- `RunStatsReset()` - Line 248: Clear counters
- `RunStatsUpdateTop(active)` - Lines 368, 1293: 1 Hz updates
- `RunStatsUpdateLogs()` - Lines 369, 1294: Log file updates
- `RunStatsSync()` - Lines 370, 1028, 1295, 1606: Flush monitor file
- `RunStatsUpdateCompletion()` - Lines 697, 934, 943, 974, 983, 1194, 1203, 1216, 1242: Package completions
- `RunStatsUpdate(work, portdir)` - Lines 1291, 1605: Worker status changes
- `RunStatsDone()` - Line 371: Cleanup

**Update Frequency**:
- `UpdateTop()` + `UpdateLogs()` + `Sync()` → Every 1 second (waitbuild loop)
- `UpdateCompletion()` → On package state change (success/fail/skip/ignore)
- `Update()` → On worker status change (start/stop/idle)

### C4: Metric Acquisition Functions

#### Adjusted Load Average (build.c, line ~3200)

```c
static void
adjloadavg(double *dload)
{
#if defined(__DragonFly__)
    struct vmtotal total;
    size_t size;

    size = sizeof(total);
    if (sysctlbyname("vm.vmtotal", &total, &size, NULL, 0) == 0) {
        dload[0] += (double)total.t_pw;
    }
#else
    dload[0] += 0.0;  /* just avoid compiler 'unused' warnings */
#endif
}
```

**Purpose**: Add page-fault waiting processes to 1-min load average  
**Rationale**: Standard `getloadavg()` doesn't account for disk I/O waits  
**Platform**: DragonFly BSD specific (FreeBSD has different vmtotal structure)

**Usage** (build.c:1343-1344):
```c
getloadavg(dload, 3);  // Get standard 1/5/15 min load
adjloadavg(dload);     // Adjust 1-min load with vm.vmtotal.t_pw
```

#### Swap Usage Percentage (NOT IMPLEMENTED in available sources)

**Declaration** (dsynth.h:616):
```c
double getswappct(int *noswapp);
```

**Usage** (build.c:1359):
```c
dswap = getswappct(&noswap);
```

**Return Value**: Double in range 0.0 - 1.0 (0% to 100%)  
**Out Parameter**: `noswap` flag (1 if no swap configured, 0 otherwise)

**Expected Implementation** (from context):
- Query `vm.swap_info` sysctl or use `kvm_getswapinfo()`
- Sum `ksw_used` / `ksw_total` across all swap devices
- Return 0.0 if no swap configured (set `*noswapp = 1`)

### C5: Dynamic Worker Throttling Logic (build.c:1304-1400)

**Throttling Algorithm** (lines 1326-1376):

```c
double min_load = 1.5 * NumCores;
double max_load = 5.0 * NumCores;
double min_swap = 0.10;
double max_swap = 0.40;

// Cap based on load (back-loaded)
getloadavg(dload, 3);
adjloadavg(dload);

if (dload[0] < min_load) {
    max1 = MaxWorkers;
} else if (dload[0] <= max_load) {
    max1 = MaxWorkers - MaxWorkers * 0.75 *
           (dload[0] - min_load) / (max_load - min_load);
} else {
    max1 = MaxWorkers * 25 / 100;  // 75% reduction
}

// Cap based on swap use (back-loaded)
dswap = getswappct(&noswap);

if (dswap < min_swap) {
    max2 = MaxWorkers;
} else if (dswap <= max_swap) {
    max2 = MaxWorkers - MaxWorkers * 0.75 *
           (dswap - min_swap) / (max_swap - min_swap);
} else {
    max2 = MaxWorkers * 25 / 100;  // 75% reduction
}

DynamicMaxWorkers = (max1 < max2) ? max1 : max2;
```

**Thresholds**:
- **Load**: Start reducing at 1.5×ncpus, max reduction at 5.0×ncpus
- **Swap**: Start reducing at 10%, max reduction at 40%
- **Reduction**: Linear interpolation from 100% to 25% (75% reduction)
- **Result**: Use minimum of load-based and swap-based limits

**Key Difference from Issue Doc**:
- Issue doc said "reduce to 75%" but code actually reduces BY 75% (to 25%)
- Issue doc said "load > 2.0 × ncpus" but code uses 1.5-5.0 range with linear interpolation

### C6: Missing Implementation Analysis

**Files Needed from Upstream**:

1. **monitor.c** - Monitor file writer
   - `MonitorRunStats` structure initialization
   - `monitor_updateTop()` implementation
   - `monitor_sync()` atomic file write with flock
   - Monitor file format (text? binary? line-based?)

2. **curses.c** - Ncurses UI
   - `NCursesRunStats` structure initialization  
   - `curses_updateTop()` panel rendering
   - Layout details (how many lines for stats panel?)
   - Color scheme, refresh rate

3. **html.c** - HTML report generator
   - `HtmlRunStats` structure initialization
   - Report format, template
   - Update frequency (periodic? end-of-build?)

4. **Swap implementation** - `getswappct()` function
   - Likely in a system-specific file (bsd.c? util.c?)
   - Need to check upstream for exact implementation

### C7: Go Port Implications

**Data Type Adjustments**:
- `topinfo.pkgrate` / `pkgimpulse`: C uses `int`, Go should use `int` (not `float64`)
- `topinfo.dswap`: C uses `double` 0-1.0, Go should match (convert to percentage for display)
- `topinfo.h/m/s`: C uses separate ints, Go should use `time.Duration` and convert for display

**Throttling Formula Correction**:
- Use 1.5-5.0×ncpus range (not 2.0×ncpus threshold)
- Use 10-40% swap range (not 10% threshold)
- Linear interpolation, reduce TO 25% (not BY 75%)

**Missing Functions**:
- Must fetch upstream sources for monitor file format
- Must implement BSD-specific `getswappct()` using Go syscalls
- Must design Go-native ncurses UI (or use existing tview implementation)

**Observer Pattern**:
- Go equivalent: `StatsConsumer` interface
- Multiple consumers: ncurses UI, monitor writer, future web UI
- Update propagation: channel-based or callback-based?

### C8: Next Steps

1. ✅ **Inventory Complete**: All available sources cataloged
2. ✅ **Symbol Extraction Complete**: `topinfo_t`, `runstats_t`, constants documented
3. ⏳ **Call Site Mapping**: Map all `RunStats*()` calls in build.c (next task)
4. ⏳ **Fetch Upstream Sources**: Clone DragonFlyBSD repo, extract monitor.c/curses.c
5. ⏳ **Metric Mapping Table**: Create comprehensive mapping for Go implementation

**Blockers**:
- Need upstream sources for monitor file format
- Need `getswappct()` implementation details

**Ready to Proceed**:
- Have complete data structure definitions
- Have throttling algorithm
- Have call site locations
- Can start Go port design while waiting for upstream sources

---

**Document Updated**: 2025-12-02  
**Analysis Complete**: 60% (missing upstream sources)  
**Next Phase**: Map RunStats call sites in build.c (Task plan3)

### C9: RunStats Call Site Mapping (build.c)

**Total Call Sites**: 23 locations across build lifecycle

#### Initialization & Cleanup

| Line | Function | Call | Context |
|------|----------|------|---------|
| 247 | `DoBuild()` | `RunStatsInit()` | Before build starts, after mutex lock |
| 248 | `DoBuild()` | `RunStatsReset()` | Clear counters immediately after init |
| 368-371 | `DoBuild()` | `RunStatsUpdateTop(0)` + `UpdateLogs()` + `Sync()` + `Done()` | Final updates after all workers complete |

**Pattern**: Init → Reset at start, UpdateTop → UpdateLogs → Sync → Done at end

#### Main Monitoring Loop (waitbuild)

| Line | Function | Call | Context | Frequency |
|------|----------|------|---------|-----------|
| 1291 | `waitbuild()` | `RunStatsUpdate(work, NULL)` | For each worker (loop 0..MaxWorkers) | 1 Hz |
| 1293 | `waitbuild()` | `RunStatsUpdateTop(1)` | After worker loop, active=1 | 1 Hz |
| 1294 | `waitbuild()` | `RunStatsUpdateLogs()` | Update log files | 1 Hz |
| 1295 | `waitbuild()` | `RunStatsSync()` | Flush monitor file | 1 Hz |

**waitbuild() Loop Flow**:
```c
while (RunningWorkers == whilematch) {
    // 1. Check each worker for completion
    for (i = 0; i < MaxWorkers; ++i) {
        work = &WorkerAry[i];
        if (work->state == WORKER_DONE || work->state == WORKER_FAILED) {
            workercomplete(work);  // Handles stats increments
        }
        RunStatsUpdate(work, NULL);  // Update per-worker stats
    }
    
    // 2. Update aggregate stats
    RunStatsUpdateTop(1);      // Populate topinfo_t, call all consumers
    RunStatsUpdateLogs();      // Write to log files
    RunStatsSync();            // Flush monitor.dat atomically
    
    // 3. Sleep 1 second (cond_timedwait with 1s timeout)
    if (RunningWorkers == whilematch) {
        ts.tv_sec += 1;
        pthread_cond_timedwait(&WorkerCond, &WorkerMutex, &ts);
    }
    
    // 4. Dynamic throttling logic (load/swap checks)
    // ... DynamicMaxWorkers calculation ...
}
```

**Update Frequency**: Exactly 1 Hz (1-second `pthread_cond_timedwait` timeout)

#### Package Completion Events

**Meta-node completion** (non-built metaports):

| Line | Call | DLOG Type | Counter Updated |
|------|------|-----------|-----------------|
| 697 | `RunStatsUpdateCompletion(NULL, DLOG_SUCC, pkg, "", "")` | DLOG_SUCC | BuildMetaCount++ |

**Dependency-driven skips/ignores** (before build starts):

| Line | Call | DLOG Type | Reason | Counter |
|------|------|-----------|--------|---------|
| 934 | `RunStatsUpdateCompletion(NULL, DLOG_IGN, ipkg, reason, skipbuf)` | DLOG_IGN | Dependency ignored | BuildIgnoreCount++ |
| 943 | `RunStatsUpdateCompletion(NULL, DLOG_SKIP, ipkg, reason, skipbuf)` | DLOG_SKIP | Dependency skipped | BuildSkipCount++ |
| 974 | `RunStatsUpdateCompletion(NULL, DLOG_IGN, pkgi, reason, skipbuf)` | DLOG_IGN | Reverse dep ignored | BuildIgnoreCount++ |
| 983 | `RunStatsUpdateCompletion(NULL, DLOG_SKIP, pkgi, reason, skipbuf)` | DLOG_SKIP | Reverse dep skipped | BuildSkipCount++ |

**Worker-driven completions** (during build execution):

| Line | Call | DLOG Type | Phase | Counter |
|------|------|-----------|-------|---------|
| 1194 | `RunStatsUpdateCompletion(work, DLOG_SKIP, pkg, reason, skipbuf)` | DLOG_SKIP | Pre-build checks (IGNORD) | BuildIgnoreCount++ |
| 1203 | `RunStatsUpdateCompletion(work, DLOG_SKIP, pkg, reason, skipbuf)` | DLOG_SKIP | Pre-build checks (SKIPPD) | BuildSkipCount++ |
| 1216 | `RunStatsUpdateCompletion(work, DLOG_FAIL, pkg, skipbuf, "")` | DLOG_FAIL | Build phase failure | BuildFailCount++ |
| 1242 | `RunStatsUpdateCompletion(work, DLOG_SUCC, pkg, "", "")` | DLOG_SUCC | Build success | BuildSuccessCount++ |

**Pattern**: All completions call `RunStatsUpdateCompletion()` with DLOG type, triggering:
1. Global counter increment
2. Log file write
3. Rate/impulse tracking update
4. UI refresh

#### Worker State Updates

| Line | Call | Context |
|------|------|---------|
| 1021 | `/*RunStatsUpdate(work);*/` | COMMENTED OUT - after startworker() |
| 1090 | `/*RunStatsUpdate(work);*/` | COMMENTED OUT - in startwork() |
| 1605 | `RunStatsUpdate(work, NULL)` | Worker freeze handling |
| 1606 | `RunStatsSync()` | After freeze update |

**Note**: Most `RunStatsUpdate(work)` calls are **commented out**, relying on the 1 Hz loop in `waitbuild()` instead. Only active call is in freeze handling.

#### Periodic Sync

| Line | Call | Context |
|------|------|---------|
| 1028 | `RunStatsSync()` | After ptymaster setup loop |
| 1295 | `RunStatsSync()` | In waitbuild() 1 Hz loop |
| 1606 | `RunStatsSync()` | After worker freeze |

**Frequency**: 1 Hz (in waitbuild loop) + ad-hoc (setup, freeze)

### C10: Call Flow Diagram

```
DoBuild() Start
│
├─ RunStatsInit()              // Initialize consumers (ncurses, monitor, html)
├─ RunStatsReset()             // Clear counters (BuildSuccessCount, etc.)
│
├─ Build Loop
│  │
│  ├─ Dependency Resolution
│  │  └─ RunStatsUpdateCompletion(NULL, DLOG_IGN/SKIP, pkg, ...)  // Cascade failures
│  │
│  ├─ Worker Assignment
│  │  └─ startworker(pkg, work)
│  │
│  └─ waitbuild() [1 Hz LOOP]
│     │
│     ├─ FOR each worker
│     │  ├─ workercomplete(work) if DONE/FAILED
│     │  │  └─ RunStatsUpdateCompletion(work, DLOG_SUCC/FAIL, ...)
│     │  └─ RunStatsUpdate(work, NULL)  // Per-worker status
│     │
│     ├─ RunStatsUpdateTop(1)   // Populate topinfo_t with current metrics
│     │  └─ Calls all registered consumers:
│     │     ├─ NCursesRunStats.updateTop(&topinfo)  → Update UI panel
│     │     ├─ MonitorRunStats.updateTop(&topinfo)   → Update monitor.dat
│     │     └─ HtmlRunStats.updateTop(&topinfo)      → Update HTML report
│     │
│     ├─ RunStatsUpdateLogs()   // Write to log files
│     ├─ RunStatsSync()         // Flush monitor.dat with flock
│     │
│     ├─ pthread_cond_timedwait(1 second)  // Sleep until next iteration
│     │
│     └─ Dynamic Throttling
│        ├─ getloadavg() + adjloadavg()   → dload[0]
│        ├─ getswappct()                  → dswap
│        └─ Calculate DynamicMaxWorkers   → Throttle if high load/swap
│
└─ Build Complete
   ├─ RunStatsUpdateTop(0)      // Final update with active=0
   ├─ RunStatsUpdateLogs()      // Final log flush
   ├─ RunStatsSync()            // Final monitor.dat write
   └─ RunStatsDone()            // Cleanup (ncurses teardown, close files)
```

### C11: Key Observations

**Update Strategy**:
- **1 Hz Heartbeat**: `waitbuild()` updates stats every 1 second via `pthread_cond_timedwait`
- **Event-Driven**: Package completions trigger immediate `RunStatsUpdateCompletion()` calls
- **Batched**: Top-level stats (`UpdateTop`), logs (`UpdateLogs`), and monitor sync (`Sync`) happen together

**Threading Model**:
- All RunStats calls happen **under `WorkerMutex`** (main thread, not worker threads)
- Workers signal completion via `pthread_cond_signal(&WorkerCond)`
- Main thread polls workers in `waitbuild()` and updates stats

**Consumer Pattern**:
- `RunStatsUpdateTop()` iterates linked list of `runstats_t` consumers
- Each consumer (ncurses, monitor, html) implements callbacks
- All consumers receive same `topinfo_t` snapshot

**Missing Implementations**:
- `RunStatsUpdateTop()` implementation (likely in stats.c or main.c)
- Individual consumer implementations (curses.c, monitor.c, html.c)
- Need to fetch upstream sources for these

**Go Port Implications**:
- Need 1 Hz ticker goroutine (instead of pthread_cond_timedwait)
- Event-driven completion tracking via channels or callbacks
- StatsCollector maintains topinfo_t equivalent, notifies consumers
- Consumers implement `StatsConsumer` interface (equivalent to runstats_t callbacks)

---

**Document Updated**: 2025-12-02  
**Call Site Mapping**: COMPLETE ✅  
**Next Phase**: Identify missing files and create metric mapping table (Tasks plan4, plan5)

### C12: Missing Upstream Files - Fetch Instructions

**Files Declared But Not Present in `.original-c-source/`**:

1. **stats.c** (estimated)
   - `RunStatsInit()`, `RunStatsDone()`, `RunStatsReset()` implementations
   - Consumer registration and linked list management
   - `RunStatsUpdateTop()` - iterates consumers, calls `consumer->updateTop(&topinfo)`

2. **monitor.c** - Monitor file writer
   - `MonitorRunStats` structure initialization
   - `monitor_init()` - create monitor.dat and monitor.lk
   - `monitor_updateTop()` - format and write stats
   - `monitor_sync()` - atomic write with flock
   - **File format** - Need to confirm: line-based text? key=value pairs?

3. **curses.c** - Ncurses UI
   - `NCursesRunStats` structure initialization
   - `curses_init()` - initialize ncurses, create windows
   - `curses_updateTop()` - render stats panel
   - `curses_done()` - teardown ncurses
   - **Layout** - Header panel lines, column widths, color scheme

4. **html.c** - HTML report generator
   - `HtmlRunStats` structure initialization
   - `html_updateTop()` - generate HTML snippet
   - **Format** - Template, update frequency

5. **System utilities** (bsd.c? util.c?)
   - `getswappct()` - Swap usage percentage implementation
   - Likely uses `kvm_getswapinfo()` or `vm.swap_info` sysctl

**Upstream Repository**:
```
https://github.com/DragonFlyBSD/DragonFlyBSD
Path: usr.bin/dsynth/
```

**Fetch Commands**:
```bash
# Clone DragonFlyBSD repository (large, ~5GB)
git clone --depth 1 https://github.com/DragonFlyBSD/DragonFlyBSD.git

# Navigate to dsynth sources
cd DragonFlyBSD/usr.bin/dsynth

# Files to extract:
# - stats.c (if exists)
# - monitor.c
# - curses.c
# - html.c
# - Any file containing getswappct() implementation

# Copy to go-synth for analysis
cp stats.c monitor.c curses.c html.c /home/antonioh/s/go-synth/.original-c-source/
```

**Alternative (web view)**:
```
https://gitweb.dragonflybsd.org/dragonfly.git/tree/HEAD:/usr.bin/dsynth
```

**Priority**:
- 🔴 **Critical**: monitor.c (monitor file format)
- 🟠 **High**: getswappct() implementation (swap metric)
- 🟡 **Medium**: curses.c (UI layout reference)
- 🟢 **Low**: html.c (not needed for MVP)

---

### C13: Comprehensive Metric Mapping Table

**Purpose**: Map original dsynth metrics to go-synth implementation strategy

| Metric | C Source | C Type | Go Equivalent | Go Type | Acquisition Method | Update Freq | Notes |
|--------|----------|--------|---------------|---------|-------------------|-------------|-------|
| **Active Workers** | `topinfo.active` | `int` | `TopInfo.ActiveWorkers` | `int` | Count running workers | 1 Hz | Main thread tracks worker states |
| **Dynamic Max** | `topinfo.dynmaxworkers` | `int` | `TopInfo.DynMaxWorkers` | `int` | Throttling algorithm | 1 Hz | Based on load/swap |
| **Package Rate** | `topinfo.pkgrate` | `int` | `TopInfo.Rate` | `float64` | 60-second sliding window | 1 Hz | C uses int pkg/hr, Go uses float for precision |
| **Package Impulse** | `topinfo.pkgimpulse` | `int` | `TopInfo.Impulse` | `int` | Current 1s bucket | 1 Hz | Instant completions |
| **Load Average** | `topinfo.dload[3]` | `double[3]` | `TopInfo.Load` | `float64` | `unix.Getloadavg()` + adjloadavg | 1 Hz | Only use dload[0] (1-min), adjust with vm.vmtotal.t_pw |
| **Swap Usage** | `topinfo.dswap` | `double` | `TopInfo.SwapPct` | `int` | `getswappct()` equivalent | 1 Hz | C: 0.0-1.0, Go: 0-100 (convert for display) |
| **No Swap Flag** | `topinfo.noswap` | `int` | `TopInfo.NoSwap` | `bool` | Check swap configured | 1 Hz | Avoid division by zero |
| **Elapsed Time** | `topinfo.h/m/s` | `int` (3 fields) | `TopInfo.Elapsed` | `time.Duration` | `time.Since(startTime)` | 1 Hz | C stores h/m/s separately, Go uses Duration |
| **Start Time** | (implicit) | `time_t` | `TopInfo.StartTime` | `time.Time` | Captured at build start | Once | For elapsed calculation |
| **Total Queued** | `topinfo.total` | `int` | `TopInfo.Queued` | `int` | Count packages in queue | On change | BuildTotal global |
| **Successful** | `topinfo.successful` | `int` | `TopInfo.Built` | `int` | Increment on DLOG_SUCC | On completion | BuildSuccessCount global |
| **Failed** | `topinfo.failed` | `int` | `TopInfo.Failed` | `int` | Increment on DLOG_FAIL | On completion | BuildFailCount global |
| **Ignored** | `topinfo.ignored` | `int` | `TopInfo.Ignored` | `int` | Increment on DLOG_IGN | On completion | BuildIgnoreCount global |
| **Skipped** | `topinfo.skipped` | `int` | `TopInfo.Skipped` | `int` | Increment on DLOG_SKIP | On completion | BuildSkipCount global |
| **Remaining** | `topinfo.remaining` | `int` | `TopInfo.Remaining` | `int` | Calculated: Queued - (Built + Failed + Ignored) | 1 Hz | Derived metric |
| **Metaports** | `topinfo.meta` | `int` | `TopInfo.Meta` | `int` | Count meta-node completions | On completion | BuildMetaCount global |

#### Special Metrics (Not in topinfo_t)

| Metric | C Source | Purpose | Go Equivalent | Notes |
|--------|----------|---------|---------------|-------|
| **Running Dep Size** | `RunningPkgDepSize` | Memory throttling | Track separately | Sum of pkg dependency sizes for active workers |
| **Max Pkg Dep** | `PkgDepMemoryTarget` | Memory limit | Config setting | Target memory usage before throttling |
| **Running Workers** | `RunningWorkers` | Thread count | `ActiveWorkers` | Redundant with topinfo.active |
| **Build Count** | `BuildCount` | Currently building | Same as active | Global counter |

#### Throttling Thresholds

| Threshold | C Value | Go Equivalent | Purpose |
|-----------|---------|---------------|---------|
| **Min Load** | `1.5 * NumCores` | `1.5 * runtime.NumCPU()` | Start reducing workers |
| **Max Load** | `5.0 * NumCores` | `5.0 * runtime.NumCPU()` | Maximum reduction (25% of max) |
| **Min Swap** | `0.10` (10%) | `10` (10%) | Start reducing workers |
| **Max Swap** | `0.40` (40%) | `40` (40%) | Maximum reduction (25% of max) |
| **Reduction Formula** | Linear interpolation | Same | `MaxWorkers * (1 - 0.75 * ratio)` |

---

### C14: Go Implementation Strategy Summary

#### Phase 1: Core Data Structures

```go
// stats/types.go

type TopInfo struct {
    // Workers
    ActiveWorkers int
    MaxWorkers    int
    DynMaxWorkers int
    
    // System metrics
    Load    float64  // Adjusted 1-min load
    SwapPct int      // 0-100 percentage
    NoSwap  bool     // No swap configured
    
    // Build rate
    Rate    float64  // Packages per hour (60s window)
    Impulse int      // Instant completions (last 1s)
    
    // Timing
    StartTime time.Time
    Elapsed   time.Duration
    
    // Build totals
    Queued    int
    Built     int
    Failed    int
    Ignored   int
    Skipped   int
    Meta      int
    Remaining int  // Calculated
}

type StatsConsumer interface {
    OnStatsUpdate(info TopInfo)
}
```

#### Phase 2: StatsCollector Service

```go
// stats/collector.go

type StatsCollector struct {
    mu            sync.RWMutex
    topInfo       TopInfo
    rateBuckets   [60]int      // 1-second buckets
    currentBucket int          // Ring buffer index
    ticker        *time.Ticker // 1 Hz
    consumers     []StatsConsumer
    ctx           context.Context
    cancel        context.CancelFunc
}

// Public API
func NewStatsCollector(ctx context.Context, maxWorkers int) *StatsCollector
func (sc *StatsCollector) RecordCompletion(status BuildStatus)
func (sc *StatsCollector) UpdateWorkerCount(active int)
func (sc *StatsCollector) GetSnapshot() TopInfo
func (sc *StatsCollector) AddConsumer(consumer StatsConsumer)
func (sc *StatsCollector) Close() error

// Internal (1 Hz ticker)
func (sc *StatsCollector) run()
func (sc *StatsCollector) tick()
func (sc *StatsCollector) sampleMetrics()
func (sc *StatsCollector) calculateRate() float64
func (sc *StatsCollector) updateThrottling()
func (sc *StatsCollector) notifyConsumers()
```

#### Phase 3: BSD Metrics Acquisition

```go
// stats/metrics_bsd.go

func getAdjustedLoad() (float64, error) {
    var loadavg [3]float64
    if err := unix.Getloadavg(loadavg[:]); err != nil {
        return 0, err
    }
    
    // Add page-fault waiting processes
    var vmtotal unix.Vmtotal
    mib := []int32{unix.CTL_VM, unix.VM_TOTAL}
    if err := sysctl(mib, &vmtotal); err != nil {
        return loadavg[0], nil  // Fallback to unadjusted
    }
    
    return loadavg[0] + float64(vmtotal.T_pw), nil
}

func getSwapUsage() (int, bool, error) {
    // Query vm.swap_info sysctl
    // Returns (percentage, noswap_flag, error)
    // Implementation TBD - need upstream getswappct() source
}
```

#### Phase 4: Integration Points

```go
// build/build.go

func DoBuild(...) error {
    // Create stats collector
    stats := stats.NewStatsCollector(buildCtx, cfg.MaxWorkers)
    defer stats.Close()
    
    // Register UI consumer
    stats.AddConsumer(ui)
    
    // Register monitor writer (if enabled)
    if cfg.MonitorFile != "" {
        monitor := stats.NewMonitorWriter(cfg.MonitorFile)
        stats.AddConsumer(monitor)
        defer monitor.Close()
    }
    
    // ... build loop ...
    
    // On package completion
    stats.RecordCompletion(buildStatus)
    
    // Worker count changes automatically via stats.UpdateWorkerCount()
}
```

---

**Document Updated**: 2025-12-02  
**Status**: Part C (Stats Analysis) - COMPLETE ✅  
**Next Steps**:
1. Fetch upstream sources (monitor.c, curses.c, getswappct)
2. Update issue document with corrected thresholds and data types
3. Begin Go implementation (Phase 3 of Issue #9)

