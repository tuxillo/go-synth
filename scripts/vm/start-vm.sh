#!/bin/bash
# Start DragonFlyBSD VM
set -euo pipefail

# Load VM configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.sh"

PID_FILE="${VM_DIR}/vm.pid"

# Check if disk exists
if [ ! -f "${VM_DISK}" ]; then
    echo "❌ Disk image not found: ${VM_DISK}"
    echo "   Run 'make vm-setup' first"
    exit 1
fi

# Check if VM already running
if [ -f "${PID_FILE}" ] && kill -0 $(cat "${PID_FILE}") 2>/dev/null; then
    echo "✓ VM already running (PID: $(cat ${PID_FILE}))"
    echo "  SSH: ssh -p ${VM_SSH_PORT} root@localhost"
    exit 0
fi

echo "🚀 Starting DragonFlyBSD ${DFLY_VERSION} VM..."
echo "   Disk: ${VM_DISK}"
echo "   SSH: localhost:${VM_SSH_PORT}"
echo "   Memory: ${VM_MEMORY} / CPUs: ${VM_CPUS}"

# Start VM in background
# Use virtio-scsi to match installation (creates da0 device)
qemu-system-x86_64 \
    -enable-kvm \
    -m "${VM_MEMORY}" \
    -smp "${VM_CPUS}" \
    -device virtio-scsi-pci,id=scsi0 \
    -drive file="${VM_DISK}",if=none,format=qcow2,cache=none,id=myscsi \
    -device scsi-hd,drive=myscsi,bus=scsi0.0 \
    -net nic,model=virtio \
    -net user,hostfwd=tcp::${VM_SSH_PORT}-:22 \
    -daemonize \
    -pidfile "${PID_FILE}" \
    -display none

if [ ! -f "${PID_FILE}" ]; then
    echo "❌ VM failed to start"
    exit 1
fi

echo "⏳ Waiting for SSH (may take 30s)..."
for i in {1..60}; do
    if ssh ${VM_SSH_OPTS} -o ConnectTimeout=1 \
           ${VM_SSH_HOST} "echo connected" >/dev/null 2>&1; then
        echo "✓ VM ready! (PID: $(cat ${PID_FILE}))"
        echo ""
        echo "  Connect: make vm-ssh"
        echo "  Test:    make vm-quick"
        exit 0
    fi
    sleep 1
    if [ $((i % 10)) -eq 0 ]; then
        echo "   Still waiting... (${i}s)"
    fi
done

echo "❌ VM failed to become ready after 60s"
echo "   Check with: make vm-status"
exit 1
