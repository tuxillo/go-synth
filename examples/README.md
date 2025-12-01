# Examples - pkg Library Usage

This directory contains standalone, runnable examples demonstrating how to use the `pkg` library.

## Prerequisites

- Go 1.21 or later
- FreeBSD or DragonFly BSD with ports tree
- go-synth project configured

## Quick Start

Each example is self-contained and can be run with:

```bash
cd examples/01_simple_parse
go run main.go [args]
```

## Available Examples

### 01_simple_parse - Basic Parsing

**What it does:** Parses a single port specification and displays its metadata.

**Usage:**
```bash
cd 01_simple_parse
go run main.go [port-spec]
```

**Example:**
```bash
go run main.go editors/vim
```

**Output:**
```
Parsing port: editors/vim

Package Information:
  Port Directory:  editors/vim
  Version:         9.0.2189
  Package File:    vim-9.0.2189.pkg

Raw Dependency Strings (not resolved yet):
  BUILD_DEPENDS:   pkgconf>=1.3.0_1
  RUN_DEPENDS:     ...

Success! Parsed 1 package.
```

**Key Learning:** How to parse port specs with `ParsePortList()`

---

### 02_resolve_deps - Dependency Resolution

**What it does:** Resolves all dependencies of a port, building the complete dependency graph.

**Usage:**
```bash
cd 02_resolve_deps
go run main.go [port-spec]
```

**Example:**
```bash
go run main.go shells/bash
```

**Output:**
```
Resolving dependencies for: shells/bash

Parsed: shells/bash (v5.2.26)

Resolving dependencies...
Resolution complete! Found 8 total packages (including dependencies)

Direct Dependencies:
  BUILD dependencies: 3
  Reverse dependencies: 0 packages depend on this

First 10 dependencies:
  - devel/pkgconf (BUILD)
  - devel/gettext-runtime (BUILD)
  - devel/libiconv (LIB)

Success! Dependencies resolved.
```

**Key Learning:** How to use `ResolveDependencies()` to build the full graph

---

### 03_build_order - Topological Ordering

**What it does:** Computes the correct build order for packages using topological sort.

**Usage:**
```bash
cd 03_build_order
go run main.go [port-specs...]
```

**Examples:**
```bash
go run main.go editors/vim
go run main.go editors/vim shells/bash devel/git
```

**Output:**
```
Computing build order for 1 port(s):
  - editors/vim

Parsing ports...
Parsed 1 package(s)

Resolving dependencies...
Dependencies resolved

Computing topological build order...

Build Order (25 packages total):
======================================================================
   1. devel/pkgconf                          [0 deps, 5 dependents]
      ^ Build this first (no dependencies)
   2. devel/gettext-runtime                  [1 deps, 3 dependents]
   ...
  25. editors/vim                            [15 deps, 0 dependents]
      ^ Build this last (requested package)
======================================================================

Success! Build order computed with 25 packages.
```

**Key Learning:** How to use `GetBuildOrder()` for topological sorting

---

### 04_cycle_detection - Circular Dependencies

**What it does:** Demonstrates cycle detection and shows the difference between strict and permissive ordering.

**Usage:**
```bash
cd 04_cycle_detection
go run main.go [port-spec]
```

**Example:**
```bash
go run main.go editors/vim
```

**Output (no cycles):**
```
Checking for dependency cycles in: editors/vim

Parsing port...
Resolving dependencies...
Resolved 25 total packages

Attempting strict topological ordering (cycle detection)...

✓ SUCCESS! No cycles detected.

Strict ordering computed 25 packages in dependency order.

This dependency graph is cycle-free and can be built in strict order.
```

**Output (with cycles):**
```
❌ CYCLE DETECTED!

Details:
  Total packages:     50
  Successfully ordered: 42
  Stuck in cycle:     8

Packages involved in cycle:
  - www/package-a
  - www/package-b
  - devel/package-c

Cycles prevent strict ordering, but you can use permissive ordering for builds.

Falling back to permissive ordering (ignores cycles)...
✓ Permissive ordering succeeded with 50 packages

Note: Permissive ordering works around cycles by breaking them arbitrarily.
```

**Key Learning:** Difference between `TopoOrderStrict()` and `GetBuildOrder()`

---

### 05_dependency_tree - Tree Visualization

**What it does:** Prints a visual tree representation of a package's dependency hierarchy.

**Usage:**
```bash
cd 05_dependency_tree
go run main.go [port-spec] [max-depth]
```

**Examples:**
```bash
go run main.go editors/vim
go run main.go editors/vim 3
```

**Output:**
```
Dependency tree for: editors/vim (max depth: 5)

📦 editors/vim (v9.0.2189)
├─ [BUILD] devel/pkgconf
│  └─ [LIB] devel/libiconv
│     └─ [BUILD] devel/gettext-runtime
├─ [BUILD] devel/gettext-runtime (already shown)
├─ [LIB] devel/libiconv (already shown)
└─ [RUN] shells/bash
   ├─ [BUILD] devel/pkgconf (already shown)
   └─ [LIB] devel/gettext-runtime (already shown)

Statistics:
  Unique dependencies shown: 5
  Total unique dependencies: 5
```

**Key Learning:** How to traverse the dependency graph manually

---

## Common Patterns

### Running All Examples

```bash
#!/bin/bash
# Run all examples with default arguments

cd examples

echo "=== Example 01: Simple Parse ==="
cd 01_simple_parse && go run main.go editors/vim
cd ..

echo -e "\n=== Example 02: Resolve Dependencies ==="
cd 02_resolve_deps && go run main.go shells/bash
cd ..

echo -e "\n=== Example 03: Build Order ==="
cd 03_build_order && go run main.go devel/git
cd ..

echo -e "\n=== Example 04: Cycle Detection ==="
cd 04_cycle_detection && go run main.go www/nginx
cd ..

echo -e "\n=== Example 05: Dependency Tree ==="
cd 05_dependency_tree && go run main.go lang/python 3
cd ..
```

### Checking if Examples Compile

```bash
#!/bin/bash
# Verify all examples compile

cd examples
for dir in */; do
    if [ -f "$dir/main.go" ]; then
        echo "Checking $dir..."
        (cd "$dir" && go build -o /dev/null main.go)
        if [ $? -eq 0 ]; then
            echo "  ✓ $dir compiles"
        else
            echo "  ✗ $dir has errors"
        fi
    fi
done
```

## Learning Path

We recommend going through the examples in order:

1. **Start with 01** - Learn basic parsing
2. **Move to 02** - Understand dependency resolution
3. **Try 03** - See topological ordering in action
4. **Explore 04** - Learn about cycle detection
5. **Finish with 05** - Visualize the dependency tree

## Tips

- **Port specs:** Use format `category/portname` (e.g., `editors/vim`)
- **Flavors:** Use format `category/portname@flavor` (e.g., `lang/python@py39`)
- **Multiple ports:** Some examples accept multiple port specs
- **Configuration:** Examples use default config from `/etc/dsynth/dsynth.ini`

## Troubleshooting

### "Port not found" errors

Make sure your ports tree is checked out and the port exists:
```bash
ls /usr/ports/editors/vim
```

### Compilation errors

Ensure you're in the correct directory:
```bash
cd examples/01_simple_parse  # Not just examples/
go run main.go
```

### Import errors

Examples use relative imports. Run from within each example directory, not from the project root.

## See Also

- **[PHASE_1_DEVELOPER_GUIDE.md](../PHASE_1_DEVELOPER_GUIDE.md)** - Complete developer guide
- **[README.md](../README.md)** - Project overview
- **[pkg godoc](https://pkg.go.dev/)** - API documentation

## Contributing

Found an issue or want to add an example? Please submit a pull request!

---

**Last Updated:** 2025-11-26
