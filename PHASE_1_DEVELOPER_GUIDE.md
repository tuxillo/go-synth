# Phase 1 Developer Guide - Using the pkg Library

**Version:** 1.0  
**Last Updated:** 2025-11-26  
**Status:** Phase 1 Complete

---

## Table of Contents

1. [Overview](#overview)
2. [Installation & Setup](#installation--setup)
3. [Quick Start](#quick-start)
4. [Core Concepts](#core-concepts)
5. [API Reference](#api-reference)
6. [Error Handling](#error-handling)
7. [Advanced Usage](#advanced-usage)
8. [Common Patterns](#common-patterns)
9. [Troubleshooting](#troubleshooting)
10. [Examples](#examples)

---

## Overview

### What is the pkg Library?

The `pkg` package is a pure Go library for parsing FreeBSD/DragonFly BSD port specifications, resolving their dependencies, and computing valid build orders. It was extracted from the dsynth build system to provide a clean, reusable API for working with ports metadata.

### What Does It Do?

The pkg library provides three core operations:

1. **Parse** - Parse port specifications (e.g., `editors/vim`, `lang/python@py39`)
2. **Resolve** - Build complete dependency graph with all 6 dependency types
3. **TopoOrder** - Compute topological build order using Kahn's algorithm

### What Does It NOT Do?

The pkg library is focused purely on metadata and dependency analysis. It does NOT:

- Build packages (use the `build` package)
- Manage chroot environments (use the `mount` package)
- Track build state or CRCs (use the `builddb` package)
- Execute make commands
- Manage configuration files

### When to Use This Library?

Use the pkg library when you need to:

- Parse port specifications programmatically
- Analyze dependency relationships
- Compute build orders for custom build systems
- Validate port dependencies
- Generate dependency graphs or reports
- Build tools that work with FreeBSD/DragonFly ports

### Key Features

- **Pure Metadata** - No side effects, no global state
- **Thread-Safe** - Safe for concurrent use with separate registries
- **Comprehensive** - Handles all 6 dependency types (DEPEND, BUILD_DEPEND, RUN_DEPEND, etc.)
- **Type-Safe** - Structured error types, typed enums
- **Well-Documented** - Comprehensive godoc comments
- **Tested** - 39 tests including fidelity tests against original C implementation

---

## Installation & Setup

### Prerequisites

- **Go 1.21 or later**
- **FreeBSD or DragonFly BSD** (or compatible system with ports tree)
- **Ports tree** checked out at a known location (e.g., `/usr/ports`)

### Import the Package

```go
import (
    "dsynth/config"
    "dsynth/pkg"
)
```

### Configuration Setup

The pkg library requires a `config.Config` object that specifies the ports tree location:

```go
// Load configuration from file
cfg, err := config.LoadConfig("", "default")
if err != nil {
    log.Fatal(err)
}

// Or create configuration manually
cfg := &config.Config{
    DPortsPath: "/usr/ports",
}
```

---

## Quick Start

Here's a complete working example that parses a port, resolves its dependencies, and computes the build order:

```go
package main

import (
    "fmt"
    "log"
    
    "dsynth/config"
    "dsynth/pkg"
)

func main() {
    // 1. Load configuration
    cfg, err := config.LoadConfig("", "default")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }
    
    // 2. Create package and build state registries
    pkgRegistry := pkg.NewPackageRegistry()
    bsRegistry := pkg.NewBuildStateRegistry()
    
    // 3. Parse port specifications
    ports := []string{"editors/vim"}
    packages, err := pkg.ParsePortList(ports, cfg, pkgRegistry, bsRegistry)
    if err != nil {
        log.Fatalf("Failed to parse ports: %v", err)
    }
    
    fmt.Printf("Parsed %d package(s)\n", len(packages))
    
    // 4. Resolve dependencies
    err = pkg.ResolveDependencies(packages, cfg, pkgRegistry, bsRegistry)
    if err != nil {
        log.Fatalf("Failed to resolve dependencies: %v", err)
    }
    
    // 5. Get build order (must use ALL packages from registry)
    allPackages := pkgRegistry.AllPackages()
    buildOrder := pkg.GetBuildOrder(allPackages)
    
    // 6. Print build order
    fmt.Printf("\nBuild order (%d packages):\n", len(buildOrder))
    for i, p := range buildOrder {
        fmt.Printf("%3d. %s\n", i+1, p.PortDir)
    }
}
```

**Expected Output:**
```
Parsed 1 package(s)

Build order (25 packages):
  1. devel/pkgconf
  2. devel/gettext-runtime
  3. ...
 24. editors/vim-console
 25. editors/vim
```

---

## Core Concepts

### The Package Struct

The `Package` struct represents a single port and contains only metadata (no build state):

```go
type Package struct {
    // Identification
    PortDir     string   // e.g., "editors/vim"
    PkgVersion  string   // e.g., "9.0.2189"
    PkgFile     string   // e.g., "vim-9.0.2189.pkg"
    Flavor      string   // e.g., "py39" for python flavors
    
    // Dependencies (6 types)
    Depend       []*PkgLink  // Runtime dependencies
    BuildDepend  []*PkgLink  // Build-time dependencies
    RunDepend    []*PkgLink  // Run-time dependencies
    LibDepend    []*PkgLink  // Library dependencies
    FetchDepend  []*PkgLink  // Fetch-phase dependencies
    ExtDepend    []*PkgLink  // Extract-phase dependencies
    
    // Dependency Graph (bidirectional)
    Depi      []*PkgLink  // Packages that depend on this
    DepiCount int         // Number of forward dependencies
    DepiDepth int         // Depth in dependency tree
}
```

**Key Points:**
- Package is **pure metadata** - no locks, no build state
- All dependency arrays use Go slices (not linked lists)
- Bidirectional links enable both forward and reverse traversal

### PackageRegistry

The `PackageRegistry` stores all parsed packages and enables lookups:

```go
type PackageRegistry struct {
    packages map[string]*Package  // Key: portdir[@flavor]
    mu       sync.Mutex            // Thread-safe access
}

// Create a new registry
registry := pkg.NewPackageRegistry()

// Enter a package (idempotent)
p := registry.Enter(portdir, flavor)

// Find a package
p, exists := registry.Find(portdir, flavor)
```

**Key Points:**
- Thread-safe for concurrent access
- `Enter()` is idempotent (safe to call multiple times)
- Key format: `"portdir"` or `"portdir@flavor"`

### BuildStateRegistry

The `BuildStateRegistry` tracks build-time state separately from Package:

```go
type BuildState struct {
    Pkg          *Package
    Flags        PackageFlags  // PkgFlagDummy, PkgFlagMetaNode, etc.
    IgnoreReason string
    LastPhase    string
}
```

**Why Separate?**
- Keeps Package pure (reusable for different builds)
- Enables concurrent builds with different states
- Clean separation of concerns

### Dependency Types (DepType)

The library supports all 6 FreeBSD dependency types:

```go
const (
    DEPEND_TYPE        DepType = 0  // Runtime dependency
    BUILD_DEPEND_TYPE  DepType = 1  // Build-time only
    RUN_DEPEND_TYPE    DepType = 2  // Runtime only
    LIB_DEPEND_TYPE    DepType = 3  // Shared library
    FETCH_DEPEND_TYPE  DepType = 4  // Fetch phase
    EXTRACT_DEPEND_TYPE DepType = 5 // Extract phase
)
```

### Topological Ordering

The library uses **Kahn's algorithm** for topological sorting:

```go
// IMPORTANT: Must use ALL packages from registry after ResolveDependencies()
allPackages := pkgRegistry.AllPackages()

// Get build order (permissive - ignores cycles)
order := pkg.GetBuildOrder(allPackages)

// Strict ordering (fails on cycles)
order, err := pkg.TopoOrderStrict(allPackages)
```

**Key Points:**
- `GetBuildOrder()` is permissive (used for building)
- `TopoOrderStrict()` fails fast on cycles (used for validation)
- Order respects all 6 dependency types
- **Must pass allPackages from registry, not just root packages**

---

## API Reference

### Main Functions

#### ParsePortList

```go
func ParsePortList(
    portSpecs []string,
    cfg *config.Config,
    pkgRegistry *PackageRegistry,
    bsRegistry *BuildStateRegistry,
) ([]*Package, error)
```

Parses a list of port specifications and returns Package objects.

**Parameters:**
- `portSpecs` - Port specifications (e.g., `["editors/vim", "shells/bash"]`)
- `cfg` - Configuration with ports tree path
- `pkgRegistry` - Package registry for lookups
- `bsRegistry` - Build state registry

**Returns:**
- Slice of parsed packages
- Error if parsing fails

**Errors:**
- `ErrNoValidPorts` - No valid ports in the list
- `ErrInvalidSpec` - Malformed port specification
- `*PortNotFoundError` - Port not found in tree

#### ResolveDependencies

```go
func ResolveDependencies(
    packages []*Package,
    cfg *config.Config,
    pkgRegistry *PackageRegistry,
    bsRegistry *BuildStateRegistry,
) error
```

Resolves all dependencies using a two-pass algorithm.

**Algorithm:**
1. **Pass 1:** Recursively fetch all dependencies (6 types)
2. **Pass 2:** Build bidirectional links (forward and reverse edges)

**Returns:**
- Error if resolution fails

#### GetBuildOrder

```go
func GetBuildOrder(packages []*Package) ([]*Package, error)
```

Computes topological build order using Kahn's algorithm (permissive).

**Returns:**
- Ordered slice where dependencies come before dependents
- Error on failure (should not happen with permissive mode)

#### TopoOrderStrict

```go
func TopoOrderStrict(packages []*Package) ([]*Package, error)
```

Strict topological ordering that detects cycles.

**Returns:**
- Ordered slice if no cycles
- `*CycleError` if cycles detected

### Helper Functions

#### MarkPackagesNeedingBuild

```go
func MarkPackagesNeedingBuild(packages []*Package, bsRegistry *BuildStateRegistry)
```

Marks packages that need to be built (no PkgFlagDummy).

#### GetInstalledPackages / GetAllPorts

```go
func GetInstalledPackages(cfg *config.Config) ([]string, error)
func GetAllPorts(cfg *config.Config) ([]string, error)
```

Utility functions for listing installed packages or all ports.

---

## Error Handling

The pkg library uses **structured error types** for better error handling.

### Sentinel Errors

```go
var (
    ErrCycleDetected = errors.New("dependency cycle detected")
    ErrInvalidSpec   = errors.New("invalid port specification")
    ErrPortNotFound  = errors.New("port not found")
    ErrNoValidPorts  = errors.New("no valid ports provided")
    ErrEmptySpec     = errors.New("empty port specification")
)
```

### Structured Errors

#### PortNotFoundError

```go
type PortNotFoundError struct {
    PortSpec string  // e.g., "editors/vim"
    Path     string  // e.g., "/usr/ports/editors/vim"
}
```

**Usage:**
```go
packages, err := pkg.ParsePortList(ports, cfg, pkgRegistry, bsRegistry)
if err != nil {
    var pnfErr *pkg.PortNotFoundError
    if errors.As(err, &pnfErr) {
        fmt.Printf("Port not found: %s (path: %s)\n", pnfErr.PortSpec, pnfErr.Path)
    }
}
```

#### CycleError

```go
type CycleError struct {
    TotalPackages   int
    ProcessedCount  int
    RemainingCount  int
    SamplePackages  []string  // First few packages in cycle
}
```

**Usage:**
```go
order, err := pkg.TopoOrderStrict(packages)
if err != nil {
    var cycleErr *pkg.CycleError
    if errors.As(err, &cycleErr) {
        fmt.Printf("Cycle detected: %d packages involved\n", cycleErr.RemainingCount)
        fmt.Printf("Sample packages: %v\n", cycleErr.SamplePackages)
    }
}
```

### Error Handling Patterns

#### Check for Specific Errors

```go
if errors.Is(err, pkg.ErrNoValidPorts) {
    // Handle no valid ports
}

if errors.Is(err, pkg.ErrCycleDetected) {
    // Handle cycle
}
```

#### Extract Error Details

```go
var pnfErr *pkg.PortNotFoundError
if errors.As(err, &pnfErr) {
    // Access pnfErr.PortSpec, pnfErr.Path
}
```

#### Graceful Degradation

```go
packages, err := pkg.ParsePortList(ports, cfg, pkgRegistry, bsRegistry)
if err != nil {
    // Log error but continue with valid packages
    log.Printf("Warning: Some ports failed to parse: %v", err)
    if len(packages) > 0 {
        // Continue with successfully parsed packages
    }
}
```

---

## Advanced Usage

### Working with Flavors

Flavors are variants of a port (e.g., different Python versions):

```go
// Parse port with flavor
ports := []string{"lang/python@py39", "lang/python@py310"}
packages, err := pkg.ParsePortList(ports, cfg, pkgRegistry, bsRegistry)

// Check flavor
for _, p := range packages {
    if p.Flavor != "" {
        fmt.Printf("%s has flavor: %s\n", p.PortDir, p.Flavor)
    }
}
```

**Key format:** `portdir@flavor`

### Handling Large Dependency Graphs

For large ports trees (e.g., building everything):

```go
// Get all ports
allPorts, err := pkg.GetAllPorts(cfg)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Total ports: %d\n", len(allPorts))

// Parse in batches (optional, for memory control)
batchSize := 100
for i := 0; i < len(allPorts); i += batchSize {
    end := i + batchSize
    if end > len(allPorts) {
        end = len(allPorts)
    }
    
    batch := allPorts[i:end]
    packages, err := pkg.ParsePortList(batch, cfg, pkgRegistry, bsRegistry)
    // ... process batch
}
```

### Concurrent Usage

The library is thread-safe when using separate registries:

```go
// Create registries per goroutine
func buildPort(portSpec string) {
    pkgRegistry := pkg.NewPackageRegistry()  // Per-goroutine registry
    bsRegistry := pkg.NewBuildStateRegistry()
    
    packages, err := pkg.ParsePortList([]string{portSpec}, cfg, pkgRegistry, bsRegistry)
    // ... process
}

// Safe to run concurrently
go buildPort("editors/vim")
go buildPort("shells/bash")
```

**Important:** Don't share registries across goroutines without additional synchronization.

### Custom Dependency Analysis

Walk the dependency graph manually:

```go
// Find all dependencies of a package
func getAllDeps(p *pkg.Package, visited map[*pkg.Package]bool) []*pkg.Package {
    if visited[p] {
        return nil
    }
    visited[p] = true
    
    deps := []*pkg.Package{p}
    
    // Walk all dependency types
    for _, link := range p.Depend {
        deps = append(deps, getAllDeps(link.Pkg, visited)...)
    }
    for _, link := range p.BuildDepend {
        deps = append(deps, getAllDeps(link.Pkg, visited)...)
    }
    // ... other types
    
    return deps
}

// Usage
visited := make(map[*pkg.Package]bool)
allDeps := getAllDeps(myPackage, visited)
fmt.Printf("Total dependencies: %d\n", len(allDeps))
```

---

## Common Patterns

### Pattern 1: Safe Build Order with Error Recovery

```go
func safeBuildOrder(ports []string, cfg *config.Config) ([]*pkg.Package, error) {
    pkgRegistry := pkg.NewPackageRegistry()
    bsRegistry := pkg.NewBuildStateRegistry()
    
    // Parse
    packages, err := pkg.ParsePortList(ports, cfg, pkgRegistry, bsRegistry)
    if err != nil {
        // Check if we got any valid packages
        if len(packages) == 0 {
            return nil, fmt.Errorf("no valid packages: %w", err)
        }
        log.Printf("Warning: Some packages failed to parse: %v", err)
    }
    
    // Resolve
    err = pkg.ResolveDependencies(packages, cfg, pkgRegistry, bsRegistry)
    if err != nil {
        return nil, fmt.Errorf("dependency resolution failed: %w", err)
    }
    
    // Get all packages from registry (includes transitive dependencies)
    allPackages := pkgRegistry.AllPackages()
    
    // Try strict ordering first
    order, err := pkg.TopoOrderStrict(allPackages)
    if err != nil {
        // Fall back to permissive ordering
        log.Printf("Warning: Cycles detected, using permissive ordering: %v", err)
        order = pkg.GetBuildOrder(allPackages)
    }
    
    return order, nil
}
```

### Pattern 2: Dependency Tree Printer

```go
func printDependencyTree(p *pkg.Package, indent int, visited map[*pkg.Package]bool) {
    if visited[p] {
        fmt.Printf("%s%s (already shown)\n", strings.Repeat("  ", indent), p.PortDir)
        return
    }
    visited[p] = true
    
    fmt.Printf("%s%s\n", strings.Repeat("  ", indent), p.PortDir)
    
    // Print dependencies
    for _, link := range p.BuildDepend {
        printDependencyTree(link.Pkg, indent+1, visited)
    }
}

// Usage
visited := make(map[*pkg.Package]bool)
printDependencyTree(myPackage, 0, visited)
```

### Pattern 3: Filter Packages by Criteria

```go
// Find packages matching a pattern
func filterPackages(packages []*pkg.Package, pattern string) []*pkg.Package {
    var filtered []*pkg.Package
    for _, p := range packages {
        if strings.Contains(p.PortDir, pattern) {
            filtered = append(filtered, p)
        }
    }
    return filtered
}

// Find packages with specific dependency
func packagesWithDep(packages []*pkg.Package, depPort string) []*pkg.Package {
    var result []*pkg.Package
    for _, p := range packages {
        for _, link := range p.BuildDepend {
            if link.Pkg.PortDir == depPort {
                result = append(result, p)
                break
            }
        }
    }
    return result
}
```

### Pattern 4: Build Progress Tracking

```go
type BuildProgress struct {
    Total     int
    Completed int
    Failed    int
    Skipped   int
}

func trackBuildProgress(packages []*pkg.Package) *BuildProgress {
    progress := &BuildProgress{Total: len(packages)}
    
    // Simulate build process
    for _, p := range packages {
        fmt.Printf("Building %s...\n", p.PortDir)
        
        // Your build logic here
        success := true // Replace with actual build
        
        if success {
            progress.Completed++
        } else {
            progress.Failed++
        }
        
        // Print progress
        fmt.Printf("Progress: %d/%d (%.1f%% complete, %d failed)\n",
            progress.Completed, progress.Total,
            float64(progress.Completed)/float64(progress.Total)*100,
            progress.Failed)
    }
    
    return progress
}
```

---

## Troubleshooting

### Common Issues

#### Issue: "Port not found" errors

**Symptoms:**
```
Error: port not found: editors/vim (path: /usr/ports/editors/vim)
```

**Solutions:**
1. Verify ports tree is checked out:
   ```bash
   ls /usr/ports/editors/vim
   ```

2. Check configuration path:
   ```go
   fmt.Printf("Ports path: %s\n", cfg.DPortsPath)
   ```

3. Verify port specification format:
   - Correct: `"editors/vim"`
   - Wrong: `"vim"`, `"/usr/ports/editors/vim"`

#### Issue: Cycle detection errors

**Symptoms:**
```
Error: dependency cycle detected (12 packages involved)
```

**Solutions:**
1. Use permissive ordering for builds:
   ```go
   allPackages := pkgRegistry.AllPackages()
   order := pkg.GetBuildOrder(allPackages)  // Permissive
   ```

2. Identify cycle participants:
   ```go
   var cycleErr *pkg.CycleError
   if errors.As(err, &cycleErr) {
       fmt.Printf("Packages in cycle: %v\n", cycleErr.SamplePackages)
   }
   ```

3. Check for incorrect DEPEND vs BUILD_DEPEND:
   - Some cycles are due to ports specifying the wrong dependency type

#### Issue: "No valid ports" error

**Symptoms:**
```
Error: no valid ports provided
```

**Solutions:**
1. Verify input slice is not empty:
   ```go
   if len(portSpecs) == 0 {
       // Handle empty input
   }
   ```

2. Check for all-invalid specifications:
   ```go
   packages, err := pkg.ParsePortList(ports, cfg, pkgRegistry, bsRegistry)
   if errors.Is(err, pkg.ErrNoValidPorts) {
       // All ports were invalid
   }
   ```

#### Issue: Out of memory with large graphs

**Symptoms:**
- Process killed by OOM
- Slow parsing

**Solutions:**
1. Parse in batches:
   ```go
   batchSize := 100
   for i := 0; i < len(allPorts); i += batchSize {
       // Process batch
   }
   ```

2. Use filters to reduce scope:
   ```go
   // Only parse specific categories
   ports := []string{"editors/*", "devel/*"}
   ```

3. Increase system memory or use streaming approach

### Debugging Tips

#### Enable Verbose Logging

```go
import "log"

log.Printf("Parsing %d ports...\n", len(ports))
packages, err := pkg.ParsePortList(ports, cfg, pkgRegistry, bsRegistry)
log.Printf("Parsed %d packages\n", len(packages))
```

#### Inspect Package Contents

```go
func dumpPackage(p *pkg.Package) {
    fmt.Printf("Package: %s\n", p.PortDir)
    fmt.Printf("  Version: %s\n", p.PkgVersion)
    fmt.Printf("  Flavor: %s\n", p.Flavor)
    fmt.Printf("  Build deps: %d\n", len(p.BuildDepend))
    fmt.Printf("  Run deps: %d\n", len(p.RunDepend))
    fmt.Printf("  DepiCount: %d\n", p.DepiCount)
}
```

#### Check Registry Contents

```go
// After parsing, check what's in the registry
func dumpRegistry(registry *pkg.PackageRegistry) {
    // Note: This requires exposing registry contents or using reflection
    // For debugging, you can add a Debug() method to PackageRegistry
}
```

### Q&A

**Q: Can I use the pkg library without a ports tree?**  
A: No, the library requires access to port Makefiles to parse metadata. You need a checked-out ports tree.

**Q: Is the library safe for concurrent use?**  
A: Yes, but create separate registries per goroutine. Don't share registries without synchronization.

**Q: What's the difference between GetBuildOrder and TopoOrderStrict?**  
A: `GetBuildOrder()` is permissive (ignores cycles, used for building). `TopoOrderStrict()` fails on cycles (used for validation).

**Q: How do I handle flavors?**  
A: Specify flavors with `@` syntax: `"lang/python@py39"`. The library treats different flavors as separate packages.

**Q: Can I modify Package objects?**  
A: Package objects are meant to be read-only after resolution. Modifying them may break dependency links.

**Q: How do I get only direct dependencies (not transitive)?**  
A: Access the dependency arrays directly: `p.BuildDepend`, `p.RunDepend`, etc. These contain only direct dependencies.

---

## Examples

This section provides complete, runnable examples. See the `examples/` directory for standalone programs.

### Example 1: Simple Dependency Analyzer

Analyze a port's dependencies and print statistics:

```go
package main

import (
    "fmt"
    "log"
    
    "dsynth/config"
    "dsynth/pkg"
)

func main() {
    cfg, _ := config.LoadConfig("", "default")
    pkgRegistry := pkg.NewPackageRegistry()
    bsRegistry := pkg.NewBuildStateRegistry()
    
    // Parse and resolve
    packages, err := pkg.ParsePortList([]string{"editors/vim"}, cfg, pkgRegistry, bsRegistry)
    if err != nil {
        log.Fatal(err)
    }
    
    err = pkg.ResolveDependencies(packages, cfg, pkgRegistry, bsRegistry)
    if err != nil {
        log.Fatal(err)
    }
    
    // Analyze first package
    p := packages[0]
    fmt.Printf("Package: %s\n", p.PortDir)
    fmt.Printf("Version: %s\n", p.PkgVersion)
    fmt.Printf("\nDependency Statistics:\n")
    fmt.Printf("  Build dependencies:   %d\n", len(p.BuildDepend))
    fmt.Printf("  Runtime dependencies: %d\n", len(p.RunDepend))
    fmt.Printf("  Library dependencies: %d\n", len(p.LibDepend))
    fmt.Printf("  Total direct deps:    %d\n", 
        len(p.BuildDepend)+len(p.RunDepend)+len(p.LibDepend))
    
    // Compute total (including transitive)
    allPackages := pkgRegistry.AllPackages()
    order := pkg.GetBuildOrder(allPackages)
    fmt.Printf("  Total transitive:     %d\n", len(order)-1)
}
```

### Example 2: Bulk Port Validator

Validate multiple ports and report errors:

```go
package main

import (
    "fmt"
    "log"
    
    "dsynth/config"
    "dsynth/pkg"
)

func main() {
    cfg, _ := config.LoadConfig("", "default")
    
    ports := []string{
        "editors/vim",
        "shells/bash",
        "devel/git",
        "www/nginx",
    }
    
    fmt.Printf("Validating %d ports...\n\n", len(ports))
    
    for _, port := range ports {
        fmt.Printf("Checking %s... ", port)
        
        pkgRegistry := pkg.NewPackageRegistry()
        bsRegistry := pkg.NewBuildStateRegistry()
        
        packages, err := pkg.ParsePortList([]string{port}, cfg, pkgRegistry, bsRegistry)
        if err != nil {
            fmt.Printf("FAIL - %v\n", err)
            continue
        }
        
        err = pkg.ResolveDependencies(packages, cfg, pkgRegistry, bsRegistry)
        if err != nil {
            fmt.Printf("FAIL - %v\n", err)
            continue
        }
        
        _, err = pkg.TopoOrderStrict(packages)
        if err != nil {
            fmt.Printf("WARNING - %v\n", err)
            continue
        }
        
        fmt.Printf("OK (%d dependencies)\n", len(packages)-1)
    }
}
```

### Example 3: Dependency Graph Exporter

Export dependency graph to DOT format for visualization:

```go
package main

import (
    "fmt"
    "log"
    "os"
    
    "dsynth/config"
    "dsynth/pkg"
)

func main() {
    cfg, _ := config.LoadConfig("", "default")
    pkgRegistry := pkg.NewPackageRegistry()
    bsRegistry := pkg.NewBuildStateRegistry()
    
    packages, err := pkg.ParsePortList([]string{"editors/vim"}, cfg, pkgRegistry, bsRegistry)
    if err != nil {
        log.Fatal(err)
    }
    
    err = pkg.ResolveDependencies(packages, cfg, pkgRegistry, bsRegistry)
    if err != nil {
        log.Fatal(err)
    }
    
    // Export to DOT format
    f, err := os.Create("deps.dot")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()
    
    fmt.Fprintln(f, "digraph dependencies {")
    fmt.Fprintln(f, "  rankdir=LR;")
    fmt.Fprintln(f, "  node [shape=box];")
    
    visited := make(map[*pkg.Package]bool)
    for _, p := range packages {
        exportPackage(f, p, visited)
    }
    
    fmt.Fprintln(f, "}")
    fmt.Println("Wrote deps.dot (render with: dot -Tpng deps.dot -o deps.png)")
}

func exportPackage(f *os.File, p *pkg.Package, visited map[*pkg.Package]bool) {
    if visited[p] {
        return
    }
    visited[p] = true
    
    for _, link := range p.BuildDepend {
        fmt.Fprintf(f, "  \"%s\" -> \"%s\";\n", p.PortDir, link.Pkg.PortDir)
        exportPackage(f, link.Pkg, visited)
    }
}
```

### Example 4: Find Outdated Dependencies

Find packages that depend on an old version:

```go
package main

import (
    "fmt"
    "log"
    "strings"
    
    "dsynth/config"
    "dsynth/pkg"
)

func main() {
    cfg, _ := config.LoadConfig("", "default")
    
    // Get all installed packages
    installed, err := pkg.GetInstalledPackages(cfg)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Checking %d installed packages...\n\n", len(installed))
    
    // Find packages that need updates
    needsUpdate := []string{}
    
    for _, port := range installed {
        pkgRegistry := pkg.NewPackageRegistry()
        bsRegistry := pkg.NewBuildStateRegistry()
        
        packages, err := pkg.ParsePortList([]string{port}, cfg, pkgRegistry, bsRegistry)
        if err != nil {
            continue
        }
        
        p := packages[0]
        
        // Check if any dependency has "old" in version (simple heuristic)
        for _, link := range p.BuildDepend {
            if strings.Contains(link.Pkg.PkgVersion, "old") {
                needsUpdate = append(needsUpdate, port)
                break
            }
        }
    }
    
    fmt.Printf("Found %d packages with potentially outdated dependencies:\n", len(needsUpdate))
    for _, port := range needsUpdate {
        fmt.Printf("  - %s\n", port)
    }
}
```

---

## See Also

- **[README.md](README.md)** - Project overview and installation
- **[DEVELOPMENT.md](DEVELOPMENT.md)** - Phase tracking and development status
- **[AGENTS.md](AGENTS.md)** - Development guidelines and commit conventions
- **[Phase 1 TODO](docs/design/PHASE_1_TODO.md)** - Detailed task list
- **[godoc](https://pkg.go.dev/)** - Run `godoc -http=:6060` for API documentation

---

## Contributing

Found an issue or have a suggestion? Please:

1. Check [Phase 1 TODO](docs/design/PHASE_1_TODO.md) for known issues
2. Read [AGENTS.md](AGENTS.md) for contribution guidelines
3. Submit an issue or pull request

---

**Document Version:** 1.0  
**Last Updated:** 2025-11-26  
**Maintainer:** dsynth-go team
