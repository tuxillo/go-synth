#!/bin/bash
# Start DragonFlyBSD VM
set -euo pipefail

VM_DIR="${HOME}/.go-synth/vm"
DISK_IMAGE="${VM_DIR}/dfly-test.qcow2"
PID_FILE="${VM_DIR}/vm.pid"
SSH_PORT="2222"

# Check if disk exists
if [ ! -f "${DISK_IMAGE}" ]; then
    echo "❌ Disk image not found: ${DISK_IMAGE}"
    echo "   Run 'make vm-setup' first"
    exit 1
fi

# Check if VM already running
if [ -f "${PID_FILE}" ] && kill -0 $(cat "${PID_FILE}") 2>/dev/null; then
    echo "✓ VM already running (PID: $(cat ${PID_FILE}))"
    echo "  SSH: ssh -p ${SSH_PORT} gosynth@localhost"
    exit 0
fi

echo "🚀 Starting DragonFlyBSD VM..."
echo "   Disk: ${DISK_IMAGE}"
echo "   SSH: localhost:${SSH_PORT}"
echo "   Memory: 2GB / CPUs: 2"

# Start VM in background
qemu-system-x86_64 \
    -enable-kvm \
    -m 2048 \
    -smp 2 \
    -drive file="${DISK_IMAGE}",format=qcow2 \
    -netdev user,id=net0,hostfwd=tcp::${SSH_PORT}-:22 \
    -device e1000,netdev=net0 \
    -daemonize \
    -pidfile "${PID_FILE}" \
    -display none

if [ ! -f "${PID_FILE}" ]; then
    echo "❌ VM failed to start"
    exit 1
fi

echo "⏳ Waiting for SSH (may take 30s)..."
for i in {1..60}; do
    if ssh -p ${SSH_PORT} -o StrictHostKeyChecking=no -o ConnectTimeout=1 \
           -o UserKnownHostsFile=/dev/null \
           gosynth@localhost "echo connected" >/dev/null 2>&1; then
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
