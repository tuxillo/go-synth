#!/bin/sh
#
# Capture BSD sysctl fixtures for testing
#
# This script captures raw binary output from BSD sysctls that we parse
# in stats/metrics_bsd.go. The fixtures allow us to test parsing logic
# on any platform without needing actual BSD syscalls.
#
# Usage:
#   On DragonFly/FreeBSD:
#     ./scripts/capture-bsd-fixtures.sh
#
#   Creates: stats/testdata/fixtures/*.bin

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FIXTURE_DIR="$PROJECT_ROOT/stats/testdata/fixtures"

# Check we're on BSD
OS=$(uname -s)
if [ "$OS" != "DragonFly" ] && [ "$OS" != "FreeBSD" ]; then
    echo "ERROR: This script must be run on DragonFly BSD or FreeBSD"
    echo "Current OS: $OS"
    exit 1
fi

# Create fixture directory
mkdir -p "$FIXTURE_DIR"

echo "Capturing BSD sysctl fixtures..."
echo "Output directory: $FIXTURE_DIR"
echo ""

# Capture vm.loadavg
echo "Capturing vm.loadavg..."
sysctl -b vm.loadavg > "$FIXTURE_DIR/vm.loadavg.bin"
SIZE=$(stat -f%z "$FIXTURE_DIR/vm.loadavg.bin" 2>/dev/null || stat -c%s "$FIXTURE_DIR/vm.loadavg.bin")
echo "  → vm.loadavg.bin ($SIZE bytes)"

# Also capture text version for reference
sysctl vm.loadavg > "$FIXTURE_DIR/vm.loadavg.txt"
echo "  → vm.loadavg.txt (reference)"

# Capture vm.vmtotal
echo "Capturing vm.vmtotal..."
sysctl -b vm.vmtotal > "$FIXTURE_DIR/vm.vmtotal.bin"
SIZE=$(stat -f%z "$FIXTURE_DIR/vm.vmtotal.bin" 2>/dev/null || stat -c%s "$FIXTURE_DIR/vm.vmtotal.bin")
echo "  → vm.vmtotal.bin ($SIZE bytes)"

sysctl vm.vmtotal > "$FIXTURE_DIR/vm.vmtotal.txt"
echo "  → vm.vmtotal.txt (reference)"

# Capture vm.swap_info
echo "Capturing vm.swap_info..."
sysctl -b vm.swap_info > "$FIXTURE_DIR/vm.swap_info.bin" 2>/dev/null || {
    echo "  → vm.swap_info not available (no swap configured)"
    touch "$FIXTURE_DIR/vm.swap_info.bin"  # Create empty file
}
SIZE=$(stat -f%z "$FIXTURE_DIR/vm.swap_info.bin" 2>/dev/null || stat -c%s "$FIXTURE_DIR/vm.swap_info.bin")
echo "  → vm.swap_info.bin ($SIZE bytes)"

sysctl vm.swap_info > "$FIXTURE_DIR/vm.swap_info.txt" 2>/dev/null || {
    echo "vm.swap_info: not available" > "$FIXTURE_DIR/vm.swap_info.txt"
}
echo "  → vm.swap_info.txt (reference)"

echo ""
echo "Capturing system information..."

# Capture system info for reference
{
    echo "# System Information"
    echo "# Generated: $(date)"
    echo ""
    uname -a
    echo ""
    sysctl kern.ostype kern.osrelease kern.version
    echo ""
    sysctl hw.ncpu hw.physmem
    echo ""
    echo "# Current load average:"
    uptime
} > "$FIXTURE_DIR/system_info.txt"

echo "  → system_info.txt"

echo ""
echo "✓ Fixtures captured successfully!"
echo ""
echo "Files created:"
ls -lh "$FIXTURE_DIR"

echo ""
echo "Next steps:"
echo "  1. Review fixtures: cat $FIXTURE_DIR/*.txt"
echo "  2. Add fixtures to git: git add stats/testdata/"
echo "  3. Run tests: go test ./stats/"
