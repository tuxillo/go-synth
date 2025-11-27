#!/bin/bash
# Restore VM from clean snapshot
set -euo pipefail

# Load VM configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.sh"

if [ ! -f "${VM_SNAPSHOT}" ]; then
    echo "❌ Clean snapshot not found: ${VM_SNAPSHOT}"
    echo "   Run initial setup first with: make vm-setup"
    echo "   Then after manual install: make vm-snapshot"
    exit 1
fi

# Stop VM if running
echo "🛑 Stopping VM..."
"${SCRIPT_DIR}/stop-vm.sh"

echo "♻️  Restoring VM from clean snapshot..."
rm -f "${VM_DISK}"

# Use copy-on-write backing
if ! qemu-img create -f qcow2 -F qcow2 -b "${VM_SNAPSHOT}" "${VM_DISK}"; then
    echo "❌ Failed to restore from snapshot"
    exit 1
fi

echo "✓ VM restored to clean state"
echo "  Run 'make vm-start' to boot"
