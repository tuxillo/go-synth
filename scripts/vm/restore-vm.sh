#!/bin/bash
# Restore VM from clean snapshot
set -euo pipefail

VM_DIR="${HOME}/.go-synth/vm"
SNAPSHOT_BASE="${VM_DIR}/dfly-clean-snapshot.qcow2"
DISK_IMAGE="${VM_DIR}/dfly-test.qcow2"

if [ ! -f "${SNAPSHOT_BASE}" ]; then
    echo "❌ Clean snapshot not found: ${SNAPSHOT_BASE}"
    echo "   Run initial setup first with: make vm-setup"
    echo "   Then after manual install: make vm-snapshot"
    exit 1
fi

# Stop VM if running
echo "🛑 Stopping VM..."
./scripts/vm/stop-vm.sh

echo "♻️  Restoring VM from clean snapshot..."
rm -f "${DISK_IMAGE}"

# Use copy-on-write backing
if ! qemu-img create -f qcow2 -F qcow2 -b "${SNAPSHOT_BASE}" "${DISK_IMAGE}"; then
    echo "❌ Failed to restore from snapshot"
    exit 1
fi

echo "✓ VM restored to clean state"
echo "  Run 'make vm-start' to boot"
