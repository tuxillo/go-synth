#!/bin/bash
# Phase 2: Update Packages and Install Dependencies
#
# This script runs on the freshly-installed DragonFlyBSD system. It:
#   - Updates the pkg database
#   - Upgrades all packages to latest versions
#   - Installs required packages for go-synth development
#
# Adapted from: https://github.com/golang/build/blob/master/env/dragonfly-amd64/phase2.sh
#
# Required packages:
#   - go: Go compiler for building go-synth
#   - bash: Required by various scripts
#   - git: Version control
#   - rsync: File synchronization
#   - curl: HTTP client for downloads
#   - doas: Privilege escalation (sudo alternative)

set -euxo pipefail

# Logging setup
LOG_FILE="/tmp/phase2-update.log"
exec > >(tee -a "$LOG_FILE") 2>&1

# Error trap for debugging
trap 'echo "ERROR at line $LINENO: $BASH_COMMAND"; echo "Pausing for 60 seconds for inspection..."; sleep 60' ERR

echo "============================================"
echo "Phase 2: Package Updates Starting"
echo "============================================"
echo "Log file: $LOG_FILE"
echo ""

# Give network time to initialize
echo "Waiting for network initialization..."
sleep 5

# Step 1: Bootstrap pkg if needed
echo "Step 1: Bootstrapping pkg system..."
if ! command -v pkg &> /dev/null; then
    # Bootstrap pkg
    env ASSUME_ALWAYS_YES=YES pkg bootstrap
fi

# Step 2: Update pkg repository metadata
echo "Step 2: Updating package repository..."
pkg update -f

# Step 3: Upgrade all existing packages
echo "Step 3: Upgrading existing packages..."
pkg upgrade -fy

# Step 4: Install required packages for go-synth
echo "Step 4: Installing required packages..."
pkg install -y \
    go \
    bash \
    git \
    rsync \
    curl \
    wget \
    doas

# Step 5: Verify installations
echo "Step 5: Verifying installations..."
echo "  Go version: $(go version)"
echo "  Bash version: $(/usr/local/bin/bash --version | head -n1)"
echo "  Git version: $(git --version)"
echo "  Rsync version: $(rsync --version | head -n1)"

# Step 6: Clean up package cache
echo "Step 6: Cleaning package cache..."
pkg clean -y
pkg autoremove -y

# Step 7: Copy log to persistent location
echo "Step 7: Saving log..."
cp "$LOG_FILE" /root/phase2-update.log

echo ""
echo "============================================"
echo "Phase 2: Package Updates Complete!"
echo "============================================"
echo "Packages installed:"
pkg info | grep -E '^(go|bash|git|rsync|curl|wget|doas)-' || true
echo ""
echo "Log saved to /root/phase2-update.log"
echo "Shutting down to proceed to Phase 3..."
echo ""
sleep 3

# Power off so orchestrator can proceed to next phase
poweroff
