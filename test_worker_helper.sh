#!/bin/sh
# Simple test script to verify worker helper functionality

set -e

echo "==> Testing worker helper mode"

# Create a test chroot
TEST_ROOT="/tmp/test-worker-helper-$$"
mkdir -p "$TEST_ROOT/bin"
mkdir -p "$TEST_ROOT/usr/bin"
mkdir -p "$TEST_ROOT/lib"
mkdir -p "$TEST_ROOT/libexec"

# Copy essential binaries
cp /bin/sh "$TEST_ROOT/bin/"
cp /bin/echo "$TEST_ROOT/bin/"
cp /bin/ls "$TEST_ROOT/bin/"
cp /bin/sleep "$TEST_ROOT/bin/"
cp /usr/bin/true "$TEST_ROOT/usr/bin/" 2>/dev/null || cp /bin/true "$TEST_ROOT/usr/bin/"

# Copy required libraries for dynamically linked binaries
cp /lib/libc.so.8 "$TEST_ROOT/lib/" 2>/dev/null || true
cp /usr/libexec/ld-elf.so.2 "$TEST_ROOT/libexec/" 2>/dev/null || \
  cp /libexec/ld-elf.so.1 "$TEST_ROOT/libexec/" 2>/dev/null || true

# Create test directory structure
mkdir -p "$TEST_ROOT/test"

echo "Test chroot created at: $TEST_ROOT"

# Test 1: Simple command execution
echo ""
echo "==> Test 1: Simple command execution"
./go-synth --worker-helper --chroot="$TEST_ROOT" -- /bin/echo "Hello from worker helper"
if [ $? -eq 0 ]; then
    echo "✓ Test 1 PASSED"
else
    echo "✗ Test 1 FAILED"
    exit 1
fi

# Test 2: Command with exit code
echo ""
echo "==> Test 2: Non-zero exit code propagation"
./go-synth --worker-helper --chroot="$TEST_ROOT" -- /bin/sh -c "exit 42"
EXIT_CODE=$?
if [ $EXIT_CODE -eq 42 ]; then
    echo "✓ Test 2 PASSED (exit code: $EXIT_CODE)"
else
    echo "✗ Test 2 FAILED (expected 42, got $EXIT_CODE)"
    exit 1
fi

# Test 3: Working directory
echo ""
echo "==> Test 3: Working directory"
./go-synth --worker-helper --chroot="$TEST_ROOT" --workdir="/test" -- /bin/sh -c "pwd"
if [ $? -eq 0 ]; then
    echo "✓ Test 3 PASSED"
else
    echo "✗ Test 3 FAILED"
    exit 1
fi

# Test 4: Timeout (should timeout after 1 second)
echo ""
echo "==> Test 4: Command timeout"
timeout 3 ./go-synth --worker-helper --chroot="$TEST_ROOT" --timeout=1s -- /bin/sleep 10
EXIT_CODE=$?
if [ $EXIT_CODE -ne 0 ]; then
    echo "✓ Test 4 PASSED (command timed out as expected)"
else
    echo "✗ Test 4 FAILED (command should have timed out)"
    exit 1
fi

# Cleanup
rm -rf "$TEST_ROOT"

echo ""
echo "==> All tests PASSED!"
echo ""
echo "Worker helper is functioning correctly."
