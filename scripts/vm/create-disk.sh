#!/bin/bash
# Create QCOW2 disk image for DragonFlyBSD VM
set -euo pipefail

VM_DIR="${HOME}/.go-synth/vm"
DISK_IMAGE="${VM_DIR}/dfly-test.qcow2"
DISK_SIZE="20G"

mkdir -p "${VM_DIR}"

if [ -f "${DISK_IMAGE}" ]; then
    echo "⚠️  Disk image already exists: ${DISK_IMAGE}"
    echo "   Size: $(du -h "${DISK_IMAGE}" | cut -f1)"
    echo "   Delete it first or use: make vm-destroy"
    exit 1
fi

echo "💾 Creating ${DISK_SIZE} disk image..."
if ! qemu-img create -f qcow2 "${DISK_IMAGE}" "${DISK_SIZE}"; then
    echo "❌ Failed to create disk image"
    exit 1
fi

echo "✓ Disk created: ${DISK_IMAGE}"
echo "  Format: QCOW2 (thin provisioned)"
echo "  Size: ${DISK_SIZE}"
