#!/bin/sh
# capture-fixtures.sh - Generate test fixtures from BSD ports tree
#
# This script must be run on FreeBSD or DragonFly BSD with a ports tree installed.
# It captures the output of 'make -V' commands for selected ports and saves them
# as test fixtures for use in integration tests on all platforms.
#
# Usage:
#   ./scripts/capture-fixtures.sh [ports-dir]
#
# Arguments:
#   ports-dir  Path to ports tree (default: /usr/ports on FreeBSD, /usr/dports on DragonFly)
#
# Output:
#   Fixtures are written to pkg/testdata/fixtures/ with naming pattern: category__port.txt
#
# Example:
#   ./scripts/capture-fixtures.sh
#   ./scripts/capture-fixtures.sh /usr/dports

set -e

# Determine default ports directory based on OS
if [ "$(uname -s)" = "DragonFly" ]; then
    DEFAULT_PORTS_DIR="/usr/dports"
else
    DEFAULT_PORTS_DIR="/usr/ports"
fi

PORTS_DIR="${1:-$DEFAULT_PORTS_DIR}"
FIXTURE_DIR="pkg/testdata/fixtures"

# Verify ports directory exists
if [ ! -d "$PORTS_DIR" ]; then
    echo "Error: Ports directory not found: $PORTS_DIR"
    echo "Usage: $0 [ports-dir]"
    exit 1
fi

# Verify we're in go-synth project root
if [ ! -f "go.mod" ] || ! grep -q "module dsynth" go.mod; then
    echo "Error: Must run from go-synth project root"
    exit 1
fi

# Create fixture directory if needed
mkdir -p "$FIXTURE_DIR"

echo "Capturing port fixtures from $PORTS_DIR..."
echo "Output directory: $FIXTURE_DIR"
echo ""

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
    
    # Capture make output
    # Note: We use 'cd' instead of -C for better compatibility
    (
        cd "$port_path" && \
        make $flavor_arg \
            -V PKGNAME \
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
        echo "    → $output_file"
    else
        echo "    ERROR: Failed to capture $display_name"
        rm -f "$output_file"
    fi
}

# Capture core ports used in tests
echo "Capturing core test fixtures..."

# Simple ports with minimal dependencies
capture_port "devel" "gmake"
capture_port "lang" "python39"

# Common ports with typical dependencies
capture_port "editors" "vim"
capture_port "devel" "git"

# Flavored port example
capture_port "editors" "vim" "python39"

# Meta port example (if it exists)
if [ -d "$PORTS_DIR/misc" ]; then
    # Note: You may need to adjust this to an actual meta port in your tree
    # This is just a placeholder
    echo "  Note: Add meta port capture if needed"
fi

echo ""
echo "Fixture capture complete!"
echo ""
echo "Captured fixtures:"
ls -1 "$FIXTURE_DIR" | sed 's/^/  /'
echo ""
echo "Total fixtures: $(ls -1 "$FIXTURE_DIR" | wc -l)"
echo ""
echo "Next steps:"
echo "  1. Review captured fixtures for correctness"
echo "  2. Commit fixtures to repository"
echo "  3. Run tests: go test ./pkg"
