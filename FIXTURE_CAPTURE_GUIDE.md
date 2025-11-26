# Fixture Capture Guide

This guide explains how to capture BSD port fixtures for comprehensive integration testing.

## Quick Start (On BSD)

```bash
cd /path/to/go-synth
./scripts/capture-fixtures.sh
```

This will automatically capture ~40 port fixtures covering:
- Basic dependencies (gmake, pkgconf, gettext, etc.)
- Network libraries (curl, expat, ca_root_nss)
- Language runtimes (python39, perl5, ruby31)
- Basic applications (vim, git, bash)
- Complex applications (firefox, chromium, ffmpeg)
- X11/graphics stack (xorg-server, mesa-libs, cairo)
- Desktop environments (gnome-shell, i3)
- Meta ports (xorg, gnome, kde5)

## Why Capture More Fixtures?

### Current Coverage (11 fixtures)
The existing 11 fixtures enable basic integration tests:
- ✅ Simple workflow (Parse→Resolve→TopoOrder)
- ✅ Shared dependencies
- ✅ Flavored ports
- ✅ Error handling
- ✅ Meta ports
- ✅ Basic large graphs

### Enhanced Coverage (40+ fixtures)
Additional complex fixtures enable thorough testing of:
- 🎯 **Deep dependency resolution** (100+ transitive dependencies)
- 🎯 **Large graph handling** (multiple complex roots)
- 🎯 **Real-world scenarios** (actual desktop/browser builds)
- 🎯 **Performance testing** (stress test with chromium/firefox)
- 🎯 **Duplicate detection** (many ports sharing common deps)
- 🎯 **Ordering correctness** (complex dependency chains)

## Test Coverage by Fixture Set

### Minimal Set (Currently Available - 11 fixtures)
**Tests Enabled:**
- `TestIntegration_SimpleWorkflow` ✅
- `TestIntegration_SharedDependencies` ✅
- `TestIntegration_FlavoredPackage` ✅
- `TestIntegration_ErrorPortNotFound` ✅
- `TestIntegration_MetaPort` ✅
- `TestIntegration_LargeGraph` ✅ (limited)
- `TestIntegration_DeepDependencies` ⏭️ (skipped)

**Coverage:** ~72% - Covers basic workflows

### Extended Set (After Capture - 40+ fixtures)
**Tests Enabled:**
- All minimal tests ✅
- `TestIntegration_DeepDependencies` ✅ (firefox/chromium)
- `TestIntegration_LargeGraph` ✅ (full test with complex ports)

**Coverage:** ~75-80% - Covers real-world scenarios

## What Gets Captured

### Tier 1: Basic Dependencies (Always Available)
**Purpose:** Enable basic tests
- `devel/gmake`, `devel/pkgconf`, `devel/gettext-runtime`, `devel/gettext-tools`
- `devel/libffi`, `devel/libiconv`

### Tier 2: Common Libraries (Usually Available)
**Purpose:** Enable realistic application testing
- `ftp/curl`, `textproc/expat`, `security/ca_root_nss`, `dns/libidn2`
- `lang/python39`, `lang/perl5`, `lang/ruby31`

### Tier 3: Basic Applications (Usually Available)
**Purpose:** Test moderate dependency chains
- `editors/vim`, `devel/git`, `shells/bash`

### Tier 4: Complex Applications (May Not Exist)
**Purpose:** Test deep dependency resolution
- `www/firefox` - ~100+ dependencies (if available)
- `www/chromium` - ~200+ dependencies (if available)
- `multimedia/ffmpeg` - ~50+ dependencies
- `multimedia/gstreamer1` - Media framework

### Tier 5: X11/Graphics (May Not Exist)
**Purpose:** Test large interconnected dependency graphs
- `x11/xorg-server`, `x11/xorg-libs`, `x11/libX11`, `x11/libxcb`
- `graphics/mesa-libs`, `graphics/cairo`

### Tier 6: Desktop Environments (May Not Exist)
**Purpose:** Test meta-ports and large dependency sets
- `x11-wm/i3`, `x11/gnome-shell`
- `x11/xorg` (meta), `x11/gnome` (meta), `x11/kde5` (meta)

## Graceful Degradation

The capture script and tests are designed to **gracefully handle missing ports**:

### Script Behavior
```bash
# If a port doesn't exist, script continues
./scripts/capture-fixtures.sh
  Capturing: www/firefox
    WARNING: Port not found: www/firefox (skipping)
  Capturing: www/chromium
    ✓ www__chromium.txt (10 lines)
```

### Test Behavior
```go
// Tests automatically skip if fixtures unavailable
func TestIntegration_DeepDependencies(t *testing.T) {
    // Checks if complex fixtures exist
    if !foundComplex {
        t.Skip("Skipping: no complex port fixtures found")
    }
    // Test runs with whatever is available
}
```

## Capture Workflow

### On DragonFly BSD (Recommended)

```bash
# 1. Clone repository
cd /home/you/projects
git clone https://example.com/go-synth.git
cd go-synth

# 2. Run capture script
./scripts/capture-fixtures.sh

# 3. Check results
ls -l pkg/testdata/fixtures/*.txt | wc -l
# Should show 30-45 fixtures depending on ports tree

# 4. Commit new fixtures
git add pkg/testdata/fixtures/*.txt
git commit -m "Update fixtures with complex ports from DragonFly"
git push
```

### On FreeBSD

```bash
# Same as DragonFly, script auto-detects FreeBSD
./scripts/capture-fixtures.sh
# Uses /usr/ports by default
```

### Remote Capture (From Linux)

```bash
# 1. Copy script to BSD system
scp scripts/capture-fixtures.sh user@bsd-host:

# 2. SSH and run
ssh user@bsd-host
cd go-synth
./capture-fixtures.sh

# 3. Copy fixtures back
exit
scp user@bsd-host:go-synth/pkg/testdata/fixtures/*.txt pkg/testdata/fixtures/

# 4. Commit
git add pkg/testdata/fixtures/*.txt
git commit -m "Add complex port fixtures"
```

## Expected Output

### Successful Capture

```
Project root: /home/you/go-synth
Capturing port fixtures from /usr/dports...
Output directory: /home/you/go-synth/pkg/testdata/fixtures

=== Basic Dependencies ===
  Capturing: devel/gmake
    Port path: /usr/dports/devel/gmake
    Output: /home/you/go-synth/pkg/testdata/fixtures/devel__gmake.txt
    ✓ devel__gmake.txt (10 lines)
  ...

=== Complex Ports with Deep Dependencies ===
  Capturing: www/firefox
    Port path: /usr/dports/www/firefox
    Output: /home/you/go-synth/pkg/testdata/fixtures/www__firefox.txt
    ✓ www__firefox.txt (10 lines)
  ...

=========================================
Fixture capture complete!
=========================================

Captured fixtures:
  devel__gettext-runtime.txt
  devel__gettext-tools.txt
  ...
  www__firefox.txt
  www__chromium.txt

Total: 42 fixture files

Next steps:
  1. Review captured fixtures for correctness
  2. Verify line counts (should all be exactly 10 lines)
  3. If on remote BSD system, copy fixtures back:
     scp pkg/testdata/fixtures/*.txt user@devmachine:go-synth/pkg/testdata/fixtures/
  4. Commit fixtures: git add pkg/testdata/fixtures/*.txt
  5. Run tests: go test ./pkg -run TestIntegration
```

### With Missing Ports

```
=== Complex Ports with Deep Dependencies ===
  Capturing: www/firefox
    WARNING: Port not found: www/firefox (skipping)
  Capturing: www/chromium
    ✓ www__chromium.txt (10 lines)
```

This is **fine**! Tests will work with whatever fixtures are available.

## Verification

### After Capture

```bash
# Verify fixture count
ls pkg/testdata/fixtures/*.txt | wc -l

# Verify all have 10 lines
for f in pkg/testdata/fixtures/*.txt; do
    lines=$(wc -l < "$f")
    if [ "$lines" -ne 10 ]; then
        echo "ERROR: $f has $lines lines (expected 10)"
    fi
done

# Run tests
go test ./pkg -run TestIntegration -v

# Check which tests run
# - All 7 tests should run (not skip) if complex fixtures exist
# - 6/7 tests if complex fixtures missing
```

### Expected Test Results

**With Extended Fixtures:**
```
=== RUN   TestIntegration_SimpleWorkflow
--- PASS: TestIntegration_SimpleWorkflow (0.00s)
=== RUN   TestIntegration_SharedDependencies
--- PASS: TestIntegration_SharedDependencies (0.00s)
=== RUN   TestIntegration_FlavoredPackage
--- PASS: TestIntegration_FlavoredPackage (0.00s)
=== RUN   TestIntegration_ErrorPortNotFound
--- PASS: TestIntegration_ErrorPortNotFound (0.00s)
=== RUN   TestIntegration_MetaPort
--- PASS: TestIntegration_MetaPort (0.00s)
=== RUN   TestIntegration_DeepDependencies      ← NEW!
    integration_test.go: Deep dependency test passed: 127 packages
--- PASS: TestIntegration_DeepDependencies (0.01s)
=== RUN   TestIntegration_LargeGraph
    integration_test.go: Large graph validated: 231 packages
--- PASS: TestIntegration_LargeGraph (0.01s)
PASS
ok      dsynth/pkg      0.025s
```

## Fixture Maintenance

### When to Update
- **Port format changes:** If DragonFly/FreeBSD changes Makefile format
- **New ports added:** When adding tests for new ports
- **Quarterly:** Good practice to refresh fixtures every 3-4 months
- **Before releases:** Always refresh fixtures before major releases

### What to Watch For
- ⚠️ **Version numbers change** - This is normal, tests don't check versions
- ⚠️ **Dependencies change** - Port dependencies evolve, update fixtures to match
- ⚠️ **Ports removed** - If a port is removed from tree, remove fixture and update tests
- ⚠️ **Line count != 10** - This is an error, fixture is malformed

## Troubleshooting

### "Port not found" Warnings
**Cause:** Port doesn't exist in your ports tree (especially tier 4-6 ports)  
**Fix:** This is normal, script continues with other ports

### "Expected 10 lines, got X"
**Cause:** Make output changed or error occurred  
**Fix:** Check the fixture file manually, may need to regenerate

### Script Won't Run on Linux
**Cause:** Script requires BSD make's `-V` flag  
**Fix:** Must run on actual BSD system (see "Remote Capture" above)

### Tests Still Skip After Capture
**Cause:** Fixture files aren't in correct location  
**Fix:** Ensure files are in `pkg/testdata/fixtures/` with correct naming

## Summary

- 🎯 **Goal:** Capture 30-45 fixtures for comprehensive testing
- 📦 **Required:** Tier 1-3 (basic deps, libs, apps) - Always available
- 🌟 **Optional:** Tier 4-6 (complex ports) - Test deep dependencies
- ✅ **Graceful:** Script and tests handle missing ports elegantly
- 🚀 **Impact:** Enables thorough real-world testing without requiring BSD for development

Run `./scripts/capture-fixtures.sh` on your DragonFly system and commit the results!

---

**Last Updated:** 2025-11-26
