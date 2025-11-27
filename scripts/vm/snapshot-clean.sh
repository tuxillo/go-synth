#!/bin/bash
# Create snapshot of clean DragonFlyBSD installation
set -euo pipefail

# Load VM configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.sh"

if [ ! -f "${VM_DISK}" ]; then
    echo "❌ Disk image not found: ${VM_DISK}"
    echo "   Run 'make vm-setup' first"
    exit 1
fi

if [ -f "${VM_SNAPSHOT}" ]; then
    echo "⚠️  Snapshot already exists: ${VM_SNAPSHOT}"
    read -p "   Overwrite? [y/N]: " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Cancelled"
        exit 0
    fi
    rm -f "${VM_SNAPSHOT}"
fi

echo "📸 Creating clean snapshot..."
echo "   This will preserve current VM state for instant restoration"

# Copy the disk image as the snapshot base
if ! cp "${VM_DISK}" "${VM_SNAPSHOT}"; then
    echo "❌ Failed to create snapshot"
    exit 1
fi

echo "✓ Snapshot created: ${VM_SNAPSHOT}"
echo "  Size: $(du -h "${VM_SNAPSHOT}" | cut -f1)"
echo ""
echo "  You can now:"
echo "  - make vm-restore  # Reset VM to this clean state"
echo "  - make vm-destroy  # Delete VM and snapshot"
