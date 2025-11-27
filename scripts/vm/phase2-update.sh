#!/bin/sh
# Phase 2: Update Packages and Install Dependencies
#
# This script runs on the freshly-installed DragonFlyBSD system. It:
#   - Updates the pkg database
#   - Upgrades all packages to latest versions
#   - Installs required packages for go-synth development
#
# Adapted from: https://github.com/golang/build/blob/master/env/dragonfly-amd64/phase2.sh
#
# IMPORTANT: Uses /bin/sh (not bash) - must be POSIX compatible
#
# Required packages:
#   - go: Go compiler for building go-synth
#   - bash: Required by various scripts
#   - git: Version control
#   - rsync: File synchronization
#   - curl: HTTP client for downloads
#   - doas: Privilege escalation (sudo alternative)

set -ex

echo "============================================"
echo "Phase 2: Package Updates Starting"
echo "============================================"

# CRITICAL: Make PFI look for CD again when booting for phase3
# Without this, phase3 won't auto-run!
echo "Step 0: Configuring PFI for phase3..."
echo '/REQUIRE/a
rm -f /etc/pfi.conf
.
w
q' | ed /etc/rc.d/pfi

# PFI startup does not have full PATH
export PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/games:/usr/local/sbin:/usr/local/bin:/usr/pkg/sbin:/usr/pkg/bin:/root/bin

# Give network time to initialize
echo "Waiting for network initialization..."
sleep 5

# Step 1: Update pkg repository metadata
echo "Step 1: Updating package repository..."
pkg update -f

# Step 2: Upgrade pkg itself first (in case of bugs)
echo "Step 2: Upgrading pkg..."
pkg upgrade -y pkg || true

# Step 3: Fix pkg 1.14 bug if needed
if [ ! -f /usr/local/etc/pkg/repos/df-latest.conf ]; then
    echo "Step 3: Fixing pkg configuration..."
    cp /usr/local/etc/pkg/repos/df-latest.conf.sample /usr/local/etc/pkg/repos/df-latest.conf
fi

# Step 4: Update package database again
echo "Step 4: Updating package database..."
pkg update

# Step 5: Upgrade all existing packages
echo "Step 5: Upgrading existing packages..."
pkg upgrade -fy

# Step 6: Install required packages for go-synth
echo "Step 6: Installing required packages..."
pkg install -y go bash git rsync curl wget doas

# Step 7: Verify installations
echo "Step 7: Verifying installations..."
echo "  Go version: $(go version)"
echo "  Bash version: $(/usr/local/bin/bash --version | head -n1)"
echo "  Git version: $(git --version)"
echo "  Rsync version: $(rsync --version | head -n1)"

# Step 8: Clean up package cache
echo "Step 8: Cleaning package cache..."
pkg clean -y
pkg autoremove -y

echo ""
echo "============================================"
echo "Phase 2: Package Updates Complete!"
echo "============================================"
echo "Packages installed:"
pkg info | grep -E '^(go|bash|git|rsync|curl|wget|doas)-' || true
echo ""
echo "DONE WITH PHASE 2."
sync
echo "Shutting down to proceed to Phase 3..."
sleep 2

# Power off so orchestrator can proceed to next phase
poweroff
sleep 86400
