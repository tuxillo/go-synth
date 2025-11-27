#!/bin/bash
# Phase 3: go-synth Specific Provisioning
#
# This script configures the DragonFlyBSD VM for go-synth Phase 4 testing:
#   - Sets up doas for passwordless root access
#   - Creates required directories (/build/Workers, /usr/dports)
#   - Configures Go environment
#   - Sets up SSH key authentication
#   - Verifies configuration
#
# This is custom for go-synth, not based on golang/build

set -euxo pipefail

echo "============================================"
echo "Phase 3: go-synth Provisioning Starting"
echo "============================================"

# Step 1: Configure doas for passwordless root
echo "Step 1: Configuring doas..."
cat > /usr/local/etc/doas.conf <<EOF
# Allow root to execute commands without password
permit nopass root
# Persist environment variables needed for Go
permit nopass root env { GOPATH GOROOT GOCACHE PATH HOME }
EOF

chmod 600 /usr/local/etc/doas.conf

# Step 2: Create go-synth directories
echo "Step 2: Creating go-synth directories..."
mkdir -p /build/Workers
mkdir -p /usr/dports

# Set appropriate permissions
chown root:wheel /build
chown root:wheel /build/Workers
chown root:wheel /usr/dports
chmod 755 /build
chmod 755 /build/Workers
chmod 755 /usr/dports

# Step 3: Configure Go environment
echo "Step 3: Configuring Go environment..."
cat >> /root/.profile <<'EOF'

# Go environment
export GOROOT=/usr/local/go
export GOPATH=/root/go
export GOCACHE=/root/.cache/go-build
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
EOF

# Also set for current session
export GOROOT=/usr/local/go
export GOPATH=/root/go
export GOCACHE=/root/.cache/go-build
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin

# Create Go directories
mkdir -p "$GOPATH/bin"
mkdir -p "$GOPATH/src"
mkdir -p "$GOPATH/pkg"
mkdir -p "$GOCACHE"

# Step 4: Configure bash as default shell for root
echo "Step 4: Setting bash as default shell..."
if [ -f /usr/local/bin/bash ]; then
    # Add bash to valid shells if not present
    if ! grep -q '/usr/local/bin/bash' /etc/shells; then
        echo '/usr/local/bin/bash' >> /etc/shells
    fi
    # Change root's shell to bash
    chsh -s /usr/local/bin/bash root
fi

# Step 5: Create .bashrc for better shell experience
echo "Step 5: Creating .bashrc..."
cat > /root/.bashrc <<'EOF'
# .bashrc for root

# Source profile if it exists
if [ -f ~/.profile ]; then
    . ~/.profile
fi

# Aliases
alias ls='ls -G'
alias ll='ls -lh'
alias la='ls -lha'

# Prompt
PS1='\u@\h:\w\$ '

# History
HISTSIZE=1000
HISTFILESIZE=2000
EOF

# Step 6: Set up SSH authorized_keys (if SSH key provided via environment)
echo "Step 6: Configuring SSH..."
if [ -n "${SSH_PUBLIC_KEY:-}" ]; then
    echo "  Adding SSH public key to authorized_keys..."
    echo "${SSH_PUBLIC_KEY}" >> /root/.ssh/authorized_keys
    chmod 600 /root/.ssh/authorized_keys
else
    echo "  No SSH_PUBLIC_KEY provided, skipping key setup"
fi

# Ensure SSH directory has correct permissions
chmod 700 /root/.ssh
if [ -f /root/.ssh/authorized_keys ]; then
    chmod 600 /root/.ssh/authorized_keys
fi

# Step 7: Test doas configuration
echo "Step 7: Testing doas configuration..."
if doas -u root true 2>&1; then
    echo "  ✓ doas is working correctly"
else
    echo "  ✗ doas test failed!" >&2
    exit 1
fi

# Step 8: Verify Go installation
echo "Step 8: Verifying Go installation..."
if command -v go &> /dev/null; then
    echo "  ✓ Go version: $(go version)"
    echo "  ✓ GOROOT: $GOROOT"
    echo "  ✓ GOPATH: $GOPATH"
else
    echo "  ✗ Go not found!" >&2
    exit 1
fi

# Step 9: Verify directory structure
echo "Step 9: Verifying directory structure..."
for dir in /build/Workers /usr/dports "$GOPATH" "$GOCACHE"; do
    if [ -d "$dir" ]; then
        echo "  ✓ $dir exists"
    else
        echo "  ✗ $dir missing!" >&2
        exit 1
    fi
done

# Step 10: Create a marker file to indicate provisioning is complete
echo "Step 10: Creating provisioning marker..."
cat > /etc/gosynth-provisioned <<EOF
# go-synth VM provisioning completed
# Date: $(date)
# Phase 3 completed successfully
EOF

echo "============================================"
echo "Phase 3: Provisioning Complete!"
echo "============================================"
echo ""
echo "System Summary:"
echo "  OS: $(uname -sr)"
echo "  Go: $(go version)"
echo "  Doas: Configured (passwordless root)"
echo "  Directories:"
echo "    - /build/Workers"
echo "    - /usr/dports"
echo "    - $GOPATH"
echo ""
echo "The VM is now ready for go-synth Phase 4 testing!"
echo ""
echo "Shutting down..."
sleep 3

# Power off - orchestrator will create final snapshot
poweroff
