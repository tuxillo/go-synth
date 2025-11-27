#!/bin/bash
# Create QCOW2 disk image for DragonFlyBSD VM
set -euo pipefail

# Load VM configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.sh"

mkdir -p "${VM_DIR}"

if [ -f "${VM_DISK}" ]; then
    echo "⚠️  Disk image already exists: ${VM_DISK}"
    echo "   Size: $(du -h "${VM_DISK}" | cut -f1)"
    echo "   Delete it first or use: make vm-destroy"
    exit 1
fi

echo "💾 Creating ${VM_DISK_SIZE} disk image..."
if ! qemu-img create -f qcow2 "${VM_DISK}" "${VM_DISK_SIZE}"; then
    echo "❌ Failed to create disk image"
    exit 1
fi

echo "✓ Disk created: ${VM_DISK}"
echo "  Format: QCOW2 (thin provisioned)"
echo "  Size: ${VM_DISK_SIZE}"
