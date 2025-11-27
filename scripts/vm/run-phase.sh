#!/bin/bash
# Helper script to run a specific phase with QEMU
#
# This script boots QEMU with the appropriate configuration for each phase:
#   Phase 1: Needs installer ISO + phase1 ISO + clean ISO (for cpdup)
#   Phase 2: Needs disk + phase2 ISO
#   Phase 3: Needs disk + phase3 ISO
#
# Usage:
#   ./run-phase.sh <phase-number> <phase-iso>
#
# Example:
#   ./run-phase.sh 1 phase1.iso

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load configuration
source "${SCRIPT_DIR}/config.sh"

# Parse arguments
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <phase-number> <phase-iso>" >&2
    echo "" >&2
    echo "Example: $0 1 phase1.iso" >&2
    exit 1
fi

PHASE="$1"
PHASE_ISO="$2"

# Validate inputs
if [ ! -f "${PHASE_ISO}" ]; then
    echo "Error: Phase ISO not found: ${PHASE_ISO}" >&2
    exit 1
fi

if [ "${PHASE}" != "1" ] && [ ! -f "${VM_DISK}" ]; then
    echo "Error: VM disk not found: ${VM_DISK}" >&2
    echo "Phase ${PHASE} requires an existing disk from Phase 1" >&2
    exit 1
fi

echo "============================================"
echo "Running Phase ${PHASE}"
echo "============================================"
echo "Phase ISO: ${PHASE_ISO}"
if [ "${PHASE}" != "1" ]; then
    echo "VM Disk: ${VM_DISK}"
fi
echo ""

# Build QEMU command based on phase
# Following golang/build approach: virtio-scsi disk, multiple -drive for CDs
QEMU_CMD="qemu-system-x86_64"
QEMU_ARGS=(
    -display none
    -serial stdio
    -monitor none
    -m "${VM_MEMORY}"
    -smp "${VM_CPUS}"
    -machine accel=kvm
    -cpu host
    -net nic,model=virtio
    -net user
    -device virtio-scsi-pci,id=scsi0
)

case "${PHASE}" in
    1)
        # Phase 1: OS Installation
        # Need: installer ISO (boot), phase1 ISO (script), clean ISO (for cpdup)
        if [ ! -f "${VM_IMAGE}" ]; then
            echo "Error: Installer ISO not found: ${VM_IMAGE}" >&2
            echo "Run 'make vm-setup' first to download the ISO" >&2
            exit 1
        fi
        
        # Create empty disk for installation
        echo "Creating VM disk: ${VM_DISK}..."
        qemu-img create -f qcow2 "${VM_DISK}" "${VM_DISK_SIZE}"
        
        QEMU_ARGS+=(
            -drive "file=${VM_DISK},if=none,format=qcow2,cache=none,id=myscsi"
            -device scsi-hd,drive=myscsi,bus=scsi0.0
            -drive "file=${VM_IMAGE},media=cdrom"      # cd0: Installer ISO (bootable)
            -drive "file=${PHASE_ISO},media=cdrom"     # cd1: Phase1 script
            -drive "file=${VM_IMAGE},media=cdrom"      # cd2: Clean ISO for cpdup
        )
        ;;
        
    2)
        # Phase 2: Package Updates
        # Need: disk (boot), phase2 ISO (script)
        QEMU_ARGS+=(
            -drive "file=${VM_DISK},if=none,format=qcow2,cache=none,id=myscsi"
            -device scsi-hd,drive=myscsi,bus=scsi0.0
            -drive "file=${PHASE_ISO},media=cdrom"     # cd0: Phase2 script
        )
        ;;
        
    3)
        # Phase 3: Provisioning
        # Need: disk (boot), phase3 ISO (script)
        QEMU_ARGS+=(
            -drive "file=${VM_DISK},if=none,format=qcow2,cache=none,id=myscsi"
            -device scsi-hd,drive=myscsi,bus=scsi0.0
            -drive "file=${PHASE_ISO},media=cdrom"     # cd0: Phase3 script
        )
        ;;
        
    *)
        echo "Error: Invalid phase: ${PHASE}" >&2
        echo "Valid phases: 1, 2, 3" >&2
        exit 1
        ;;
esac

echo "Starting QEMU..."
echo "Command: ${QEMU_CMD} ${QEMU_ARGS[*]}"
echo ""
echo "The VM will automatically execute the phase script and power off."
echo "This may take several minutes..."
echo ""
echo "----------------------------------------"

if [ "${PHASE}" = "1" ]; then
    # Phase 1: Need to set console in boot loader
    # Pipe boot commands like golang/build does
    (sleep 1; echo 9; echo "set console=comconsole"; echo boot) | \
        "${QEMU_CMD}" "${QEMU_ARGS[@]}"
else
    # Phase 2 & 3: Just boot normally
    "${QEMU_CMD}" "${QEMU_ARGS[@]}"
fi

EXIT_CODE=$?

echo "----------------------------------------"
echo ""

if [ ${EXIT_CODE} -eq 0 ]; then
    echo "✓ Phase ${PHASE} completed successfully"
else
    echo "✗ Phase ${PHASE} failed with exit code ${EXIT_CODE}" >&2
    exit ${EXIT_CODE}
fi
