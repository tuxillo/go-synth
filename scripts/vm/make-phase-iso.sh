#!/bin/bash
# Creates a PFI (Platform Firmware Interface) ISO for automated DragonFlyBSD installation
#
# PFI is DragonFlyBSD's mechanism for automated installation scripts. When an ISO
# containing a pfi.conf file is mounted during boot, the installer automatically
# executes the specified script.
#
# Usage:
#   ./make-phase-iso.sh <script-path> <output-iso>
#
# Example:
#   ./make-phase-iso.sh phase1-install.sh phase1.iso
#
# References:
#   - https://github.com/golang/build/tree/master/env/dragonfly-amd64
#   - DragonFlyBSD PFI documentation

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load configuration
source "${SCRIPT_DIR}/config.sh"

# Parse arguments
if [ "$#" -lt 2 ]; then
    echo "Usage: $0 <script-path> <output-iso> [data-dir]" >&2
    echo "" >&2
    echo "Example: $0 phase1-install.sh phase1.iso" >&2
    echo "Example: $0 phase3-provision.sh phase3.iso /tmp/phase3-data" >&2
    exit 1
fi

SCRIPT_PATH="$1"
OUTPUT_ISO="$2"
DATA_DIR="${3:-}"  # Optional data directory to include in ISO

# Validate inputs
if [ ! -f "${SCRIPT_PATH}" ]; then
    echo "Error: Script not found: ${SCRIPT_PATH}" >&2
    exit 1
fi

if ! command -v genisoimage &> /dev/null; then
    echo "Error: genisoimage not found. Install it with:" >&2
    echo "  sudo apt-get install genisoimage" >&2
    exit 1
fi

# Get script basename for pfi.conf
SCRIPT_NAME="$(basename "${SCRIPT_PATH}")"

# Create temporary directory for ISO contents
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TEMP_DIR}"' EXIT

echo "Creating PFI ISO: ${OUTPUT_ISO}"
echo "  Script: ${SCRIPT_PATH}"

# Copy script to temp directory
cp "${SCRIPT_PATH}" "${TEMP_DIR}/${SCRIPT_NAME}"
chmod +x "${TEMP_DIR}/${SCRIPT_NAME}"

# Copy optional data directory if provided
if [ -n "${DATA_DIR}" ] && [ -d "${DATA_DIR}" ]; then
    echo "  Including data from: ${DATA_DIR}"
    cp -r "${DATA_DIR}"/* "${TEMP_DIR}/" 2>/dev/null || true
fi

# Create pfi.conf
cat > "${TEMP_DIR}/pfi.conf" <<EOF
# PFI Configuration
# This tells DragonFlyBSD installer to automatically execute the script
pfi_script=${SCRIPT_NAME}
EOF

# Generate ISO
genisoimage -r -o "${OUTPUT_ISO}" "${TEMP_DIR}" > /dev/null 2>&1

echo "✓ ISO created: ${OUTPUT_ISO}"
echo "  Size: $(du -h "${OUTPUT_ISO}" | cut -f1)"
