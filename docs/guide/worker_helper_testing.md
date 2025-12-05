# Worker Helper VM Testing Guide

This guide provides step-by-step instructions for testing the worker helper implementation on DragonFly BSD.

## Prerequisites

- DragonFly BSD VM running (6.4.2 or later)
- SSH access to VM configured
- go-synth repository cloned at `/usr/home/build/go-synth`
- Root/doas access configured

## Quick Test (5 minutes)

### 1. Build Latest Version

```bash
# SSH into VM
ssh build@<vm-ip>

# Navigate to repository
cd /usr/home/build/go-synth

# Pull latest changes (if remote configured)
git pull origin master

# Build go-synth
doas make build

# Verify binary created
ls -lh go-synth
```

### 2. Test Worker Helper Directly

```bash
# Test basic worker helper functionality
doas sh test_worker_helper.sh
```

**Expected output:**
```
==> Testing worker helper mode
Test chroot created at: /tmp/test-worker-helper-12345

==> Test 1: Simple command execution
Hello from worker helper
✓ Test 1 PASSED

==> Test 2: Non-zero exit code propagation
✓ Test 2 PASSED (exit code: 42)

==> Test 3: Working directory
/test
✓ Test 3 PASSED

==> Test 4: Command timeout
✓ Test 4 PASSED (command timed out as expected)

==> All tests PASSED!

Worker helper is functioning correctly.
```

### 3. Test Build with Ctrl+C

```bash
# Start a build (will auto-answer 'yes' to prompt)
echo "y" | doas ./go-synth build devel/libedit

# Wait 5-10 seconds for build to start
# Then press Ctrl+C

# Check for clean exit
echo $?  # Should be 1 (interrupted)
```

**Verify cleanup:**

```bash
# No stale mounts
mount | grep /build/SL
# Output: (empty)

# No stale processes
ps aux | grep -E "make|cc1|cc1plus" | grep -v grep
# Output: (empty)

# Base directories cleaned up (or at least empty)
ls -la /build/SL* 2>/dev/null || echo "No directories found"
# Output: "No directories found" or empty directories
```

### 4. Test Build Completion

```bash
# Build a small package to completion
echo "y" | doas ./go-synth build devel/libedit

# Wait for completion (should take 1-2 minutes)

# Check results
doas ./go-synth logs results | tail -20
```

**Expected output:**
```
Initial queue: 1 packages
[...]
Build successful: devel/libedit
[...]
Successfully built 1 package(s)
```

## Detailed Testing (30 minutes)

### Test 1: Verify Reaper Status Acquisition

**Purpose:** Confirm worker helpers acquire reaper status successfully.

```bash
# Enable debug output (if needed)
doas ./go-synth -d build devel/libedit &
BUILD_PID=$!

# In another terminal, check worker helper processes
ps aux | grep "go-synth --worker-helper"

# Look for successful reaper acquisition in logs
doas ./go-synth logs devel/libedit | grep -i reaper

# Kill build after 10 seconds
sleep 10
kill -INT $BUILD_PID
```

### Test 2: Process Tree Cleanup

**Purpose:** Verify all descendants are cleaned up on worker exit.

**Before fix symptoms:**
- `make`, `cc1`, `cc1plus` processes survive Ctrl+C
- Mount points show "device busy" errors
- Process tree continues after parent death

**After fix expected:**
- All processes terminate within 2-3 seconds
- No "device busy" errors
- Clean mount point cleanup

**Test procedure:**

```bash
# Start build
echo "y" | doas ./go-synth build devel/git &
BUILD_PID=$!

# Wait for workers to spawn
sleep 15

# Check process tree BEFORE interrupt
echo "=== Process tree before interrupt ==="
ps aux | grep -E "go-synth|make|cc1" | grep -v grep

# Interrupt build
kill -INT $BUILD_PID
wait $BUILD_PID

# Wait a moment for cleanup
sleep 3

# Check process tree AFTER interrupt (should be empty)
echo "=== Process tree after interrupt ==="
ps aux | grep -E "go-synth|make|cc1" | grep -v grep
# Expected: (empty)

# Check mount points (should be empty)
echo "=== Mount points after interrupt ==="
mount | grep /build/SL
# Expected: (empty)
```

### Test 3: UI Functionality

**Purpose:** Verify UI displays correctly and exits cleanly.

```bash
# Start build (ncurses UI should display)
echo "y" | doas ./go-synth build devel/git

# Observe UI:
# - Header should display correctly (4 rows)
# - Progress bars should show (6 rows)
# - Text should wrap, not truncate
# - Stats should update in real-time

# Test 'q' key exit:
# Press 'q' - should exit immediately with exit code 1

# Test Ctrl+C exit:
# Start another build
echo "y" | doas ./go-synth build devel/git
# Press Ctrl+C - should exit immediately with exit code 1
```

### Test 4: Context Cancellation

**Purpose:** Verify context cancellation propagates correctly.

**Test with timeout:**

```bash
# Build with short timeout (will fail)
timeout --signal=INT 10 echo "y" | doas ./go-synth build devel/git

# Check logs for context cancellation (should be suppressed)
doas ./go-synth logs devel/git | grep -i "context"
# Expected: No "context canceled" errors visible to user
```

### Test 5: Multiple Workers

**Purpose:** Verify multiple worker helpers work concurrently.

```bash
# Build package with dependencies (uses multiple workers)
echo "y" | doas ./go-synth build editors/vim

# Wait 30 seconds, then check worker count
sleep 30
ps aux | grep "go-synth --worker-helper" | wc -l
# Expected: 2-8 workers (depending on config)

# Let build complete or interrupt with Ctrl+C

# Verify all workers cleaned up
ps aux | grep "go-synth --worker-helper"
# Expected: (empty)
```

## Troubleshooting

### Issue: Worker Helper Fails to Start

**Symptoms:**
```
phase failed with exit code -1
Failed to execute worker helper
```

**Diagnosis:**

```bash
# Check go-synth binary exists and is executable
ls -lh ./go-synth
file ./go-synth

# Try invoking worker helper directly
doas ./go-synth --worker-helper --chroot=/tmp/test -- /bin/echo "test"
```

**Common causes:**
- Binary not found or not executable
- Incorrect PATH in Execute() call
- Missing --worker-helper flag parsing

### Issue: Reaper Status Not Acquired

**Symptoms:**
```
Failed to acquire reaper status: connection required
```

**Diagnosis:**

This indicates PROC_REAP_* constants are incorrect for the platform.

```bash
# Check DragonFly BSD version
uname -a

# Verify procctl constants in source
grep -n "PROC_REAP" environment/bsd/procctl_dragonfly.go

# Should show:
# PROC_REAP_ACQUIRE = 0x0001
# PROC_REAP_RELEASE = 0x0002
# PROC_REAP_STATUS  = 0x0003
# PROC_REAP_GETPIDS = 0x0004
```

**Fix:**
- Ensure constants match DragonFly BSD procctl(2) man page
- Rebuild after changing constants: `doas make build`

### Issue: UI Not Displaying

**Symptoms:**
- Blank screen after starting build
- No progress updates
- Program appears hung

**Diagnosis:**

```bash
# Check terminal size
echo $TERM
stty size  # Should show rows/cols

# Test with debug output
doas ./go-synth -d build devel/libedit 2>&1 | tee debug.log

# Check for UI initialization errors
grep -i "ui\|screen\|ncurses" debug.log
```

**Common causes:**
- Terminal too small (needs minimum 24 rows)
- $TERM variable not set correctly
- ncurses initialization failure

### Issue: Stale Processes After Exit

**Symptoms:**
- `make` or `cc1` processes survive after Ctrl+C
- Worker helper processes become orphans

**Diagnosis:**

```bash
# Check process tree after interrupt
ps aux | grep -E "go-synth|make|cc1"

# Check if parent died but workers survived
ps aux | grep "go-synth --worker-helper"

# Check reaper status of surviving processes
# (This requires kernel debugging tools)
```

**This indicates:**
- Reaper mechanism not working correctly
- Worker helpers didn't acquire reaper status
- Context cancellation not propagating

## Success Criteria

✅ **Worker helper tests pass:** All 4 tests in `test_worker_helper.sh` pass

✅ **Clean Ctrl+C exit:** Build interrupts within 2-3 seconds, no errors

✅ **No stale mounts:** `mount | grep /build/SL` returns empty after exit

✅ **No stale processes:** `ps aux | grep make` returns empty after exit

✅ **UI displays correctly:** Progress updates, no truncation, clean layout

✅ **Exit keys work:** Both 'q' and Ctrl+C exit immediately (exit code 1)

✅ **Multiple workers:** Concurrent builds work without conflicts

✅ **Reaper status acquired:** No "connection required" errors in logs

## Reporting Results

After testing, please report:

1. **Test environment:**
   - DragonFly BSD version: `uname -a`
   - go-synth commit: `git rev-parse HEAD`
   - Go version: `go version`

2. **Test results:**
   - Which tests passed ✅
   - Which tests failed ❌
   - Any error messages or unexpected behavior

3. **Logs (if failures occurred):**
   ```bash
   doas ./go-synth logs results > results.log
   doas ./go-synth logs <portname> > port.log
   ```

## Next Steps After Testing

If all tests pass:
1. ✅ Mark task "Test worker helper implementation in VM" as completed
2. ✅ Update CLEANUP_CHILD_PROCESSES.md with VM test results
3. Consider removing debug output (optional cleanup)
4. Ready to close issue and move forward

If tests fail:
1. Capture detailed logs and error messages
2. Identify root cause using troubleshooting section
3. File bug report or create follow-up issue
4. Do NOT merge until tests pass

---

**Last Updated:** 2025-12-04  
**Related Commits:** 67fd365, 8939616, cd7b6ac, 8162134, c4efbf1, 9f53ff1  
**Issue Tracking:** docs/issues/CLEANUP_CHILD_PROCESSES.md
