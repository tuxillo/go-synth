#!/bin/bash
# Create snapshot of clean DragonFlyBSD installation
set -euo pipefail

VM_DIR="${HOME}/.go-synth/vm"
DISK_IMAGE="${VM_DIR}/dfly-test.qcow2"
SNAPSHOT_BASE="${VM_DIR}/dfly-clean-snapshot.qcow2"

if [ ! -f "${DISK_IMAGE}" ]; then
    echo "❌ Disk image not found: ${DISK_IMAGE}"
    echo "   Run 'make vm-setup' first"
    exit 1
fi

if [ -f "${SNAPSHOT_BASE}" ]; then
    echo "⚠️  Snapshot already exists: ${SNAPSHOT_BASE}"
    read -p "   Overwrite? [y/N]: " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Cancelled"
        exit 0
    fi
    rm -f "${SNAPSHOT_BASE}"
fi

echo "📸 Creating clean snapshot..."
echo "   This will preserve current VM state for instant restoration"

# Copy the disk image as the snapshot base
if ! cp "${DISK_IMAGE}" "${SNAPSHOT_BASE}"; then
    echo "❌ Failed to create snapshot"
    exit 1
fi

echo "✓ Snapshot created: ${SNAPSHOT_BASE}"
echo "  Size: $(du -h "${SNAPSHOT_BASE}" | cut -f1)"
echo ""
echo "  You can now:"
echo "  - make vm-restore  # Reset VM to this clean state"
echo "  - make vm-destroy  # Delete VM and snapshot"
