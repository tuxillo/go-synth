# Testing Guide

This document describes the testing strategy for go-synth, including how to run tests on Linux (using fixtures) and on BSD (using real ports).

## Table of Contents

- [Quick Start](#quick-start)
- [Testing Strategy](#testing-strategy)
- [Running Tests](#running-tests)
- [Test Types](#test-types)
- [Writing Tests](#writing-tests)
- [Fixture System](#fixture-system)
- [Troubleshooting](#troubleshooting)

## Quick Start

### On Linux (Development)

```bash
# Run all tests (uses fixtures for integration tests)
go test ./...

# Run with coverage
go test -cover ./...

# Run with verbose output
go test -v ./pkg

# Run only integration tests
go test -v ./pkg -run TestIntegration
```

### On BSD (FreeBSD/DragonFly)

```bash
# Run all tests including BSD-specific tests
go test ./...

# Run only BSD integration tests (requires ports tree)
go test -v ./pkg -run TestIntegrationBSD

# Run with real ports
go test -v ./pkg -run TestIntegrationBSD_RealPort
```

## Testing Strategy

go-synth uses a **multi-tier testing approach** to enable testing on both Linux (development) and BSD (production):

### Tier 1: Unit Tests (All Platforms)
- Test individual functions in isolation
- No dependencies on BSD ports or filesystem
- Run on any platform (Linux, BSD, macOS)

### Tier 2: Integration Tests with Fixtures (All Platforms)
- Test complete workflows (Parse → Resolve → TopoSort)
- Use text fixtures instead of real ports
- Run on any platform
- Primary development testing method

### Tier 3: BSD Integration Tests (BSD Only)
- Test against real BSD ports tree
- Use build tags: `//go:build freebsd || dragonfly`
- Validate fixture accuracy
- Final validation before deployment

## Running Tests

### All Tests

```bash
# Run everything
go test ./...

# With coverage
go test -cover ./...

# With race detection
go test -race ./...
```

### Specific Packages

```bash
# Test only pkg package
go test ./pkg

# Test only config package
go test ./config

# Test multiple packages
go test ./pkg ./config ./mount
```

### Specific Tests

```bash
# Run tests matching a pattern
go test ./pkg -run TestIntegration

# Run a specific test
go test ./pkg -run TestIntegration_SimpleWorkflow

# Run BSD tests only (on BSD systems)
go test ./pkg -run TestIntegrationBSD
```

### Coverage Reports

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage in terminal
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html
```

## Test Types

### Unit Tests

Example: `pkg/pkg_test.go`, `config/config_test.go`

```go
func TestPackageRegistry_Enter(t *testing.T) {
    registry := NewPackageRegistry()
    pkg := &Package{PortDir: "editors/vim"}
    
    result := registry.Enter(pkg)
    
    if result != pkg {
        t.Error("Expected first Enter to return same package")
    }
}
```

**Characteristics:**
- Fast execution (<1ms per test)
- No external dependencies
- Test single functions
- Use table-driven tests where appropriate

### Integration Tests (Fixtures)

Example: `pkg/integration_test.go`

```go
func TestIntegration_SimpleWorkflow(t *testing.T) {
    // Setup fixtures
    restore := setTestQuerier(newTestFixtureQuerier(map[string]string{
        "editors/vim": "testdata/fixtures/editors__vim.txt",
        "devel/gmake": "testdata/fixtures/devel__gmake.txt",
    }))
    defer restore()
    
    // Test complete workflow
    packages, _ := ParsePortList([]string{"editors/vim"}, cfg, bsReg, pkgReg)
    ResolveDependencies(packages, cfg, bsReg, pkgReg)
    buildOrder := GetBuildOrder(pkgReg.AllPackages())
    
    // Verify results
    // ...
}
```

**Characteristics:**
- Test complete workflows
- Use fixture files for port data
- Run on any platform
- Realistic but controlled environment

### BSD Integration Tests

Example: `pkg/integration_bsd_test.go`

```go
//go:build freebsd || dragonfly

func TestIntegrationBSD_RealPort(t *testing.T) {
    if _, err := os.Stat("/usr/ports"); os.IsNotExist(err) {
        t.Skip("Ports tree not found")
    }
    
    // Test against real ports tree
    packages, _ := ParsePortList([]string{"devel/gmake"}, cfg, bsReg, pkgReg)
    // ...
}
```

**Characteristics:**
- Only run on FreeBSD/DragonFly
- Require real ports tree at `/usr/ports`
- Validate fixture accuracy
- Slower execution (seconds)

## Writing Tests

### Test File Organization

```
pkg/
├── pkg.go              # Source code
├── pkg_test.go         # Unit tests (all platforms)
├── integration_test.go # Integration tests with fixtures (all platforms)
└── integration_bsd_test.go  # BSD-only tests (//go:build freebsd || dragonfly)
```

### Using Fixtures in Tests

1. **Create fixture files** in `pkg/testdata/fixtures/`:

```bash
# On BSD, capture real port data
./scripts/capture-fixtures.sh editors/vim
```

2. **Use fixtures in tests**:

```go
func TestMyFeature(t *testing.T) {
    // Setup test querier with fixtures
    restore := setTestQuerier(newTestFixtureQuerier(map[string]string{
        "editors/vim": "testdata/fixtures/editors__vim.txt",
        "devel/gmake": "testdata/fixtures/devel__gmake.txt",
    }))
    defer restore()
    
    // Your test code here
}
```

### Writing BSD-Only Tests

```go
//go:build freebsd || dragonfly
// +build freebsd dragonfly

package pkg

func TestIntegrationBSD_MyFeature(t *testing.T) {
    // Check ports tree exists
    if _, err := os.Stat("/usr/ports"); os.IsNotExist(err) {
        t.Skip("Ports tree not found")
    }
    
    // Test with real ports
}
```

### Test Naming Conventions

- **Unit tests:** `TestFunctionName` (e.g., `TestPackageRegistry_Enter`)
- **Integration tests (fixtures):** `TestIntegration_Feature` (e.g., `TestIntegration_SimpleWorkflow`)
- **BSD tests:** `TestIntegrationBSD_Feature` (e.g., `TestIntegrationBSD_RealPort`)

## Fixture System

### What Are Fixtures?

Fixtures are text files containing captured output from real BSD ports. They allow testing port parsing logic without requiring a BSD system or ports tree.

### Fixture Format

Each fixture is exactly 10 lines representing port metadata:

```
<PKGFILE>      # Line 1: Package filename (e.g., vim-9.0.1234.pkg)
<VERSION>      # Line 2: Version string (e.g., 9.0.1234)
<PKGFILE>      # Line 3: Package filename (duplicate)
<FETCH>        # Line 4: FETCH_DEPENDS
<EXTRACT>      # Line 5: EXTRACT_DEPENDS
<PATCH>        # Line 6: PATCH_DEPENDS
<BUILD>        # Line 7: BUILD_DEPENDS (e.g., gmake:devel/gmake)
<LIB>          # Line 8: LIB_DEPENDS (e.g., libintl.so:devel/gettext-runtime)
<RUN>          # Line 9: RUN_DEPENDS (e.g., python39:lang/python39)
<IGNORE>       # Line 10: IGNORE reason (empty if not ignored)
```

### Dependency Format in Fixtures

Dependencies use the BSD ports format `tool:category/port`:

```bash
# BUILD_DEPENDS examples
gmake:devel/gmake
msgfmt:devel/gettext-tools

# LIB_DEPENDS examples  
libintl.so:devel/gettext-runtime
libcurl.so:ftp/curl

# RUN_DEPENDS examples
python39:lang/python39
bash:shells/bash
```

### Creating Fixtures

#### On BSD Systems

```bash
# Capture a single port
./scripts/capture-fixtures.sh editors/vim

# Capture multiple ports
./scripts/capture-fixtures.sh editors/vim devel/gmake lang/python39

# Output goes to pkg/testdata/fixtures/
```

#### Manual Creation

Create a file `pkg/testdata/fixtures/category__portname.txt`:

```
vim-9.0.1234.pkg
9.0.1234
vim-9.0.1234.pkg


msgfmt:devel/gettext-tools gmake:devel/gmake
libintl.so:devel/gettext-runtime
python39:lang/python39

```

### Using Fixtures

```go
// Create querier with fixture mapping
querier := newTestFixtureQuerier(map[string]string{
    "editors/vim":           "testdata/fixtures/editors__vim.txt",
    "devel/gmake":           "testdata/fixtures/devel__gmake.txt",
    "devel/gettext-runtime": "testdata/fixtures/devel__gettext-runtime.txt",
})

// Set as active querier
restore := setTestQuerier(querier)
defer restore()

// Now ParsePortList will use fixtures instead of real ports
packages, err := ParsePortList([]string{"editors/vim"}, cfg, bsReg, pkgReg)
```

### Available Fixtures

Current fixture files in `pkg/testdata/fixtures/`:

- `editors__vim.txt` - Vim editor
- `editors__vim@python39.txt` - Vim with Python 3.9 flavor
- `devel__gmake.txt` - GNU Make
- `devel__git.txt` - Git version control
- `devel__gettext-runtime.txt` - Gettext runtime
- `devel__gettext-tools.txt` - Gettext tools
- `devel__libffi.txt` - libffi library
- `lang__python39.txt` - Python 3.9
- `ftp__curl.txt` - cURL
- `textproc__expat.txt` - Expat XML parser
- `x11__meta-gnome.txt` - GNOME meta-port

## Troubleshooting

### "Port not found" errors in tests

**Problem:** Tests fail with `port not found` errors

**Solution:** 
- Ensure you're using fixtures with `setTestQuerier()`
- Verify fixture files exist in `pkg/testdata/fixtures/`
- Check fixture paths are correct in test setup

### BSD tests skipped on Linux

**Problem:** `TestIntegrationBSD_*` tests don't run

**Explanation:** This is expected! BSD tests have build tags and only run on FreeBSD/DragonFly:

```go
//go:build freebsd || dragonfly
```

**To run BSD tests:** Use a FreeBSD or DragonFly system with ports tree.

### Fixture format errors

**Problem:** Tests fail with parsing errors

**Solution:**
- Verify fixture has exactly 10 lines
- Check dependency format: `tool:category/port` (not `category/port:type`)
- Ensure empty lines where appropriate (lines 4-6 often empty)
- Look at existing fixtures as examples

### Coverage too low

**Problem:** Coverage below target (60%)

**Solution:**
- Add more integration tests for uncovered workflows
- Add unit tests for utility functions
- Check coverage report: `go tool cover -func=coverage.out`
- Focus on high-value test cases first

### Tests pass on Linux but fail on BSD

**Problem:** Integration tests work with fixtures but fail on real BSD

**Solution:**
- Update fixtures to match current port versions
- Run fixture capture script on BSD: `./scripts/capture-fixtures.sh <port>`
- Check for BSD-specific behavior differences
- Verify ports tree is up to date

## Best Practices

### DO:
- ✅ Write unit tests for new functions
- ✅ Use fixtures for integration tests
- ✅ Test error conditions
- ✅ Use table-driven tests for multiple inputs
- ✅ Clean up resources with `defer`
- ✅ Skip tests gracefully when prerequisites missing

### DON'T:
- ❌ Depend on specific port versions in tests
- ❌ Assume ports tree location (use config)
- ❌ Create large fixture files (keep minimal)
- ❌ Test BSD-specific code without build tags
- ❌ Forget to restore state after mock setup

## Coverage Goals

**Target:** >60% overall coverage

**Current status:**
```bash
# Check current coverage
go test -cover ./...
```

**High-priority coverage areas:**
1. Core package parsing (`pkg/pkg.go`)
2. Dependency resolution (`pkg/deps.go`)
3. Topological ordering (`pkg/build_order.go`)
4. Configuration loading (`config/config.go`)

## Continuous Integration

### GitHub Actions (Example)

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test-linux:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - run: go test -v -cover ./...
  
  test-bsd:
    runs-on: freebsd-latest # Requires self-hosted runner
    steps:
      - uses: actions/checkout@v3
      - run: go test -v ./...
```

## Further Reading

- [pkg/testdata/README.md](pkg/testdata/README.md) - Detailed fixture documentation
- [examples/README.md](examples/README.md) - Example programs
- [PHASE_1_DEVELOPER_GUIDE.md](PHASE_1_DEVELOPER_GUIDE.md) - Developer guide
- [Go Testing Documentation](https://golang.org/pkg/testing/) - Official Go testing docs

---

**Last Updated:** 2025-11-26  
**Maintainers:** Antonio Huete Jimenez, OpenCode Agent
