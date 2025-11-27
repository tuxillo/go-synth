#!/bin/bash
# Destroy VM and all associated files
set -euo pipefail

VM_DIR="${HOME}/.go-synth/vm"
DISK_IMAGE="${VM_DIR}/dfly-test.qcow2"
SNAPSHOT_IMAGE="${VM_DIR}/dfly-clean-snapshot.qcow2"

# Stop VM first
echo "🛑 Stopping VM..."
./scripts/vm/stop-vm.sh

echo "🗑️  Destroying VM disk images..."
rm -f "${DISK_IMAGE}"

if [ -f "${SNAPSHOT_IMAGE}" ]; then
    read -p "   Also delete clean snapshot? [y/N]: " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -f "${SNAPSHOT_IMAGE}"
        echo "   ✓ Snapshot deleted"
    fi
fi

echo "✓ VM destroyed"
echo ""
echo "  To rebuild:"
echo "  - make vm-restore   # From snapshot (if kept)"
echo "  - make vm-setup     # From scratch"
