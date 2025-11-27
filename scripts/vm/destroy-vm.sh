#!/bin/bash
# Destroy VM and all associated files
set -euo pipefail

# Load VM configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.sh"

# Stop VM first
echo "🛑 Stopping VM..."
"${SCRIPT_DIR}/stop-vm.sh"

echo "🗑️  Destroying VM disk images..."
rm -f "${VM_DISK}"

if [ -f "${VM_SNAPSHOT}" ]; then
    read -p "   Also delete clean snapshot? [y/N]: " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -f "${VM_SNAPSHOT}"
        echo "   ✓ Snapshot deleted"
    fi
fi

echo "✓ VM destroyed"
echo ""
echo "  To rebuild:"
echo "  - make vm-restore   # From snapshot (if kept)"
echo "  - make vm-setup     # From scratch"
