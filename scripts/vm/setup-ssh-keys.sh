#!/bin/bash
# Setup SSH keys for passwordless access to VM
set -euo pipefail

SSH_PORT="2222"
VM_USER="gosynth"
VM_HOST="localhost"

echo "🔑 Setting up passwordless SSH..."

# Generate SSH key if not exists
if [ ! -f ~/.ssh/id_ed25519 ]; then
    echo "  Generating SSH key..."
    ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -N "" -C "go-synth-vm-testing"
fi

echo "  Copying public key to VM..."
echo "  (You may need to enter VM password: gosynth)"

if ! ssh-copy-id -p ${SSH_PORT} -o StrictHostKeyChecking=no ${VM_USER}@${VM_HOST}; then
    echo "❌ Failed to copy SSH key"
    echo "   Make sure VM is running: make vm-start"
    echo "   And that you can SSH manually: ssh -p ${SSH_PORT} ${VM_USER}@${VM_HOST}"
    exit 1
fi

# Test passwordless access
if ssh -p ${SSH_PORT} -o PasswordAuthentication=no ${VM_USER}@${VM_HOST} "echo test" >/dev/null 2>&1; then
    echo "✓ SSH keys configured successfully"
    echo "  Test: ssh -p ${SSH_PORT} ${VM_USER}@${VM_HOST}"
else
    echo "⚠️  Key copied but passwordless access not working"
    echo "   Try connecting manually to debug"
fi
