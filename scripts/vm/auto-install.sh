#!/bin/bash
# Automated DragonFlyBSD VM Installation and Provisioning
#
# This script orchestrates the complete automated installation of a DragonFlyBSD VM
# for go-synth Phase 4 testing. It runs three phases:
#
#   Phase 1: OS Installation (fdisk, disklabel, newfs, cpdup, configure boot)
#   Phase 2: Package Updates (pkg update/upgrade, install go, bash, git, etc.)
#   Phase 3: Provisioning (doas, directories, Go environment, SSH)
#
# After completion, it creates a clean snapshot that can be quickly restored.
#
# Usage:
#   ./auto-install.sh
#
# Prerequisites:
#   - QEMU/KVM installed and working
#   - genisoimage installed
#   - DragonFlyBSD ISO downloaded (run 'make vm-setup' first)
#   - At least 20GB free disk space
#
# Adapted from: https://github.com/golang/build/tree/master/env/dragonfly-amd64

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load configuration
source "${SCRIPT_DIR}/config.sh"

# Temporary files for phase ISOs
PHASE1_ISO="${VM_DIR}/phase1.iso"
PHASE2_ISO="${VM_DIR}/phase2.iso"
PHASE3_ISO="${VM_DIR}/phase3.iso"

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up temporary ISOs..."
    rm -f "${PHASE1_ISO}" "${PHASE2_ISO}" "${PHASE3_ISO}"
}
trap cleanup EXIT

echo "========================================"
echo "DragonFlyBSD VM Automated Installation"
echo "========================================"
echo ""
echo "This will create a fully provisioned DragonFlyBSD VM for go-synth testing."
echo "The process takes approximately 15-20 minutes and requires no interaction."
echo ""
echo "Configuration:"
echo "  Version: ${DFLY_VERSION}"
echo "  Memory: ${VM_MEMORY}"
echo "  CPUs: ${VM_CPUS}"
echo "  Disk: ${VM_DISK_SIZE}"
echo "  VM Directory: ${VM_DIR}"
echo ""

# Check prerequisites
echo "Checking prerequisites..."

if ! command -v qemu-system-x86_64 &> /dev/null; then
    echo "Error: qemu-system-x86_64 not found" >&2
    echo "Install it with: sudo apt-get install qemu-system-x86" >&2
    exit 1
fi

if ! command -v genisoimage &> /dev/null; then
    echo "Error: genisoimage not found" >&2
    echo "Install it with: sudo apt-get install genisoimage" >&2
    exit 1
fi

if [ ! -f "${VM_IMAGE}" ]; then
    echo "Error: DragonFlyBSD ISO not found: ${VM_IMAGE}" >&2
    echo "Run 'make vm-setup' first to download the ISO" >&2
    exit 1
fi

# Check if VM disk already exists
if [ -f "${VM_DISK}" ]; then
    echo ""
    echo "WARNING: VM disk already exists: ${VM_DISK}"
    echo "This will be OVERWRITTEN by Phase 1 installation."
    echo ""
    read -p "Continue? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
    rm -f "${VM_DISK}"
fi

echo "✓ All prerequisites met"
echo ""

# ==============================================================================
# Phase 1: OS Installation
# ==============================================================================

echo "========================================"
echo "Phase 1: OS Installation"
echo "========================================"
echo "  Creating installer ISO with automated script..."
echo "  This phase will:"
echo "    - Partition and format the disk"
echo "    - Install DragonFlyBSD base system"
echo "    - Configure boot loader and networking"
echo "    - Set up SSH for remote access"
echo ""

# Create Phase 1 ISO
"${SCRIPT_DIR}/make-phase-iso.sh" \
    "${SCRIPT_DIR}/phase1-install.sh" \
    "${PHASE1_ISO}"

echo ""
echo "Starting Phase 1 installation..."
echo "This will take approximately 5-7 minutes..."
echo ""

# Run Phase 1
"${SCRIPT_DIR}/run-phase.sh" 1 "${PHASE1_ISO}"

echo ""
echo "✓ Phase 1 complete: OS installed"
echo ""
sleep 2

# ==============================================================================
# Phase 2: Package Updates
# ==============================================================================

echo "========================================"
echo "Phase 2: Package Updates"
echo "========================================"
echo "  Creating package update ISO..."
echo "  This phase will:"
echo "    - Update pkg repository metadata"
echo "    - Upgrade all packages to latest versions"
echo "    - Install Go, bash, git, rsync, curl, doas"
echo ""

# Create Phase 2 ISO
"${SCRIPT_DIR}/make-phase-iso.sh" \
    "${SCRIPT_DIR}/phase2-update.sh" \
    "${PHASE2_ISO}"

echo ""
echo "Starting Phase 2 package updates..."
echo "This will take approximately 3-5 minutes..."
echo ""

# Run Phase 2
"${SCRIPT_DIR}/run-phase.sh" 2 "${PHASE2_ISO}"

echo ""
echo "✓ Phase 2 complete: Packages updated"
echo ""
sleep 2

# ==============================================================================
# Phase 3: go-synth Provisioning
# ==============================================================================

echo "========================================"
echo "Phase 3: go-synth Provisioning"
echo "========================================"
echo "  Creating provisioning ISO..."
echo "  This phase will:"
echo "    - Configure doas for passwordless root"
echo "    - Create /build/Workers and /usr/dports"
echo "    - Set up Go environment"
echo "    - Configure bash as default shell"
echo "    - Verify all configurations"
echo ""

# Create Phase 3 ISO
"${SCRIPT_DIR}/make-phase-iso.sh" \
    "${SCRIPT_DIR}/phase3-provision.sh" \
    "${PHASE3_ISO}"

echo ""
echo "Starting Phase 3 provisioning..."
echo "This will take approximately 2-3 minutes..."
echo ""

# Run Phase 3
"${SCRIPT_DIR}/run-phase.sh" 3 "${PHASE3_ISO}"

echo ""
echo "✓ Phase 3 complete: System provisioned"
echo ""
sleep 2

# ==============================================================================
# Create Clean Snapshot
# ==============================================================================

echo "========================================"
echo "Creating Clean Snapshot"
echo "========================================"
echo "  Creating snapshot for quick restoration..."
echo ""

# Create snapshot
if [ -f "${VM_SNAPSHOT}" ]; then
    rm -f "${VM_SNAPSHOT}"
fi

cp "${VM_DISK}" "${VM_SNAPSHOT}"

echo "✓ Snapshot created: ${VM_SNAPSHOT}"
echo ""

# ==============================================================================
# Success Summary
# ==============================================================================

echo "========================================"
echo "Installation Complete!"
echo "========================================"
echo ""
echo "Your DragonFlyBSD VM is ready for go-synth Phase 4 testing."
echo ""
echo "VM Details:"
echo "  Disk: ${VM_DISK}"
echo "  Snapshot: ${VM_SNAPSHOT}"
echo "  Size: $(du -h "${VM_DISK}" | cut -f1)"
echo ""
echo "Next Steps:"
echo "  1. Start the VM:"
echo "     make vm-start"
echo ""
echo "  2. Connect via SSH:"
echo "     make vm-ssh"
echo ""
echo "  3. Run Phase 4 tests:"
echo "     make vm-quick"
echo ""
echo "  4. Restore clean state anytime:"
echo "     make vm-restore"
echo ""
echo "For more information, see: docs/testing/VM_TESTING.md"
echo ""
