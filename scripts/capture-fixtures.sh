#!/bin/sh
# capture-fixtures.sh - Generate test fixtures from BSD ports tree
#
# IMPORTANT: This script MUST be run on FreeBSD or DragonFly BSD.
# It will NOT work on Linux because it requires BSD make's -V flag.
#
# Purpose:
#   Captures the output of 'make -V' commands for selected ports and saves them
#   as test fixtures for use in integration tests on all platforms.
#
# Usage:
#   ./scripts/capture-fixtures.sh [ports-dir]
#
# Arguments:
#   ports-dir  Path to ports tree (default: /usr/ports on FreeBSD, /usr/dports on DragonFly)
#
# Output:
#   Fixtures are written to pkg/testdata/fixtures/ with naming pattern: category__port.txt
#   Each fixture is exactly 10 lines containing port metadata variables.
#
# Example on FreeBSD/DragonFly:
#   cd /path/to/go-synth
#   ./scripts/capture-fixtures.sh
#   ./scripts/capture-fixtures.sh /usr/dports
#
# To update fixtures from Linux:
#   1. Copy this script to a BSD system
#   2. Run it there with ports tree installed
#   3. Copy generated pkg/testdata/fixtures/*.txt files back to Linux

set -e

# Check if we're on a BSD system
OS="$(uname -s)"
case "$OS" in
    FreeBSD|DragonFly)
        # Supported BSD systems
        ;;
    *)
        echo "Error: This script must be run on FreeBSD or DragonFly BSD"
        echo "Current OS: $OS"
        echo ""
        echo "This script uses BSD make's -V flag to extract port variables."
        echo "GNU make (used on Linux) does not support this flag."
        echo ""
        echo "To generate fixtures:"
        echo "  1. Copy this script to a FreeBSD/DragonFly system"
        echo "  2. Run it there with a ports tree installed"
        echo "  3. Copy the generated fixtures back to this system"
        exit 1
        ;;
esac

# Determine default ports directory based on OS
if [ "$OS" = "DragonFly" ]; then
    DEFAULT_PORTS_DIR="/usr/dports"
else
    DEFAULT_PORTS_DIR="/usr/ports"
fi

PORTS_DIR="${1:-$DEFAULT_PORTS_DIR}"

# Verify we're in go-synth project root
if [ ! -f "go.mod" ] || ! grep -q "module github.com/tuxillo/go-synth" go.mod; then
    echo "Error: Must run from go-synth project root"
    exit 1
fi

# Get absolute path to project root
# Use realpath if available, otherwise pwd
if command -v realpath >/dev/null 2>&1; then
    PROJECT_ROOT="$(realpath .)"
else
    PROJECT_ROOT="$(pwd)"
fi
FIXTURE_DIR="$PROJECT_ROOT/pkg/testdata/fixtures"

# Verify ports directory exists
if [ ! -d "$PORTS_DIR" ]; then
    echo "Error: Ports directory not found: $PORTS_DIR"
    echo "Usage: $0 [ports-dir]"
    exit 1
fi

# Create fixture directory if needed
mkdir -p "$FIXTURE_DIR"

echo "Project root: $PROJECT_ROOT"
echo "Capturing port fixtures from $PORTS_DIR..."
echo "Output directory: $FIXTURE_DIR"
echo ""

# Verify fixture directory is absolute path
case "$FIXTURE_DIR" in
    /*) ;; # Absolute path, good
    *)
        echo "ERROR: FIXTURE_DIR is not absolute: $FIXTURE_DIR"
        exit 1
        ;;
esac

# Function to capture a single port's make output
# Usage: capture_port category port [flavor]
capture_port() {
    category="$1"
    port="$2"
    flavor="$3"
    
    port_path="$PORTS_DIR/$category/$port"
    
    if [ ! -d "$port_path" ]; then
        echo "  WARNING: Port not found: $category/$port (skipping)"
        return
    fi
    
    # Determine output filename
    if [ -n "$flavor" ]; then
        output_file="$FIXTURE_DIR/${category}__${port}@${flavor}.txt"
        flavor_arg="FLAVOR=$flavor"
        display_name="$category/$port@$flavor"
    else
        output_file="$FIXTURE_DIR/${category}__${port}.txt"
        flavor_arg=""
        display_name="$category/$port"
    fi
    
    echo "  Capturing: $display_name"
    echo "    Port path: $port_path"
    echo "    Output: $output_file"
    
    # Verify output file path is absolute
    case "$output_file" in
        /*) ;; # Absolute, good
        *)
            echo "    ✗ ERROR: Output path is not absolute: $output_file"
            return 1
            ;;
    esac
    
    # Capture make output
    # Note: We use 'cd' instead of -C for better compatibility
    # BSD make's -V flag prints one variable per invocation, so we get 10 lines
    (
        cd "$port_path" && \
        make $flavor_arg \
            -V PKGFILE \
            -V PKGVERSION \
            -V PKGFILE \
            -V FETCH_DEPENDS \
            -V EXTRACT_DEPENDS \
            -V PATCH_DEPENDS \
            -V BUILD_DEPENDS \
            -V LIB_DEPENDS \
            -V RUN_DEPENDS \
            -V IGNORE \
            > "$output_file" 2>&1
    )
    
    if [ $? -eq 0 ]; then
        # Verify fixture has exactly 10 lines
        line_count=$(wc -l < "$output_file")
        if [ "$line_count" -eq 10 ]; then
            echo "    ✓ $output_file ($line_count lines)"
        else
            echo "    ⚠ WARNING: Expected 10 lines, got $line_count"
            echo "    → $output_file"
        fi
    else
        echo "    ✗ ERROR: Failed to capture $display_name"
        rm -f "$output_file"
    fi
}

# Capture core ports used in tests
echo "Capturing test fixtures..."
echo ""

echo "=== Basic Dependencies ==="
# Core ports for basic tests
capture_port "devel" "gmake"
capture_port "devel" "gettext-runtime"
capture_port "devel" "gettext-tools"
capture_port "devel" "libffi"
capture_port "devel" "pkgconf"
capture_port "devel" "libiconv"

echo ""
echo "=== Network Libraries ==="
capture_port "ftp" "curl"
capture_port "textproc" "expat"
capture_port "security" "ca_root_nss"
capture_port "dns" "libidn2"

echo ""
echo "=== Language Runtimes ==="
capture_port "lang" "python39"
capture_port "lang" "perl5"
capture_port "lang" "ruby31"

echo ""
echo "=== Basic Applications ==="
capture_port "editors" "vim"
capture_port "devel" "git"
capture_port "shells" "bash"

echo ""
echo "=== Flavored Ports ==="
# Flavored port example
capture_port "editors" "vim" "python39"

echo ""
echo "=== Complex Ports with Deep Dependencies ==="
# X11 and graphics stack (moderate complexity)
capture_port "x11" "xorg-server"
capture_port "x11" "xorg-libs"
capture_port "x11" "libX11"
capture_port "x11" "libxcb"
capture_port "graphics" "mesa-libs"
capture_port "graphics" "cairo"

# Desktop environment components
capture_port "x11-wm" "i3"
capture_port "x11" "gnome-shell"

# Large applications with many dependencies
capture_port "www" "firefox"
capture_port "www" "chromium"

# Multimedia (deep dependency trees)
capture_port "multimedia" "ffmpeg"
capture_port "multimedia" "gstreamer1"

echo ""
echo "=== Meta Ports ==="
# Meta port examples
if [ -d "$PORTS_DIR/x11/xorg" ]; then
    capture_port "x11" "xorg"
fi
if [ -d "$PORTS_DIR/x11/gnome" ]; then
    capture_port "x11" "gnome"
elif [ -d "$PORTS_DIR/x11/meta-gnome" ]; then
    capture_port "x11" "meta-gnome"
fi
if [ -d "$PORTS_DIR/x11/kde5" ]; then
    capture_port "x11" "kde5"
fi

echo ""
echo "========================================="
echo "Fixture capture complete!"
echo "========================================="
echo ""
echo "Captured fixtures:"
ls -1 "$FIXTURE_DIR" | sed 's/^/  /'
echo ""
echo "Total: $(ls -1 "$FIXTURE_DIR" | wc -l) fixture files"
echo ""
echo "Fixture format: Each file has exactly 10 lines:"
echo "  Line 1:  PKGFILE (e.g., vim-9.0.1234.pkg)"
echo "  Line 2:  PKGVERSION (e.g., 9.0.1234)"
echo "  Line 3:  PKGFILE (duplicate)"
echo "  Line 4:  FETCH_DEPENDS"
echo "  Line 5:  EXTRACT_DEPENDS"
echo "  Line 6:  PATCH_DEPENDS"
echo "  Line 7:  BUILD_DEPENDS (e.g., gmake:devel/gmake)"
echo "  Line 8:  LIB_DEPENDS (e.g., libintl.so:devel/gettext-runtime)"
echo "  Line 9:  RUN_DEPENDS (e.g., python39:lang/python39)"
echo "  Line 10: IGNORE (empty if not ignored)"
echo ""
echo "Next steps:"
echo "  1. Review captured fixtures for correctness"
echo "  2. Verify line counts (should all be exactly 10 lines)"
echo "  3. If on remote BSD system, copy fixtures back:"
echo "     scp pkg/testdata/fixtures/*.txt user@devmachine:go-synth/pkg/testdata/fixtures/"
echo "  4. Commit fixtures: git add pkg/testdata/fixtures/*.txt"
echo "  5. Run tests: go test ./pkg -run TestIntegration"
