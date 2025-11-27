#!/bin/sh
# VM provisioning script - runs ON the DragonFlyBSD VM
# This configures the VM for Phase 4 testing after first boot
set -e

echo "📦 Installing required packages..."
pkg install -y \
    bash \
    go \
    git \
    gmake \
    rsync \
    vim \
    curl \
    wget

echo "🔧 Configuring doas (passwordless sudo)..."
cat > /usr/local/etc/doas.conf <<EOF
# Allow gosynth user to run commands as root without password
permit nopass gosynth as root
EOF

echo "📁 Creating test directories..."
mkdir -p ~/go-synth
mkdir -p /build/Workers
mkdir -p /build/logs
mkdir -p /usr/dports  # Placeholder for ports tree

echo "🔗 Setting up Go environment..."
cat >> ~/.profile <<'EOF'

# Go environment for testing
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin:/usr/local/go/bin
EOF

echo "🧪 Testing doas configuration..."
if doas whoami | grep -q root; then
    echo "  ✓ doas works correctly"
else
    echo "  ⚠️  doas may not be configured correctly"
fi

echo ""
echo "✓ Provisioning complete!"
echo ""
echo "VM is ready for Phase 4 testing:"
echo "  - Go installed"
echo "  - doas configured (passwordless root)"
echo "  - Test directories created"
echo "  - SSH keys ready"
echo ""
echo "Next steps:"
echo "  1. Exit VM: exit"
echo "  2. Create snapshot: make vm-snapshot"
echo "  3. Test: make vm-quick"
