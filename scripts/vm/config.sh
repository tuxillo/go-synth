#!/bin/bash
# VM Configuration - Centralized settings for all VM scripts
#
# This file is sourced by all VM management scripts to ensure consistent
# configuration. Update DFLY_VERSION when new DragonFlyBSD releases are
# available.
#
# Usage:
#   source "$(dirname "$0")/config.sh"
#
# Environment overrides:
#   DFLY_VERSION=6.6.0 make vm-setup    # Use specific version
#   VM_MEMORY=4G make vm-start          # Increase RAM
#   VM_CPUS=4 make vm-start             # Use 4 CPU cores

# DragonFlyBSD version to use (can be overridden via environment)
# Update this when new releases are available at:
# https://mirror-master.dragonflybsd.org/iso-images/
DFLY_VERSION="${DFLY_VERSION:-6.4.2}"

# VM directory (stores disk images, ISOs, snapshots)
VM_DIR="${VM_DIR:-${HOME}/.go-synth/vm}"

# VM resource allocation
VM_MEMORY="${VM_MEMORY:-8G}"      # RAM allocation (8G = 1/4 of 32GB host)
VM_CPUS="${VM_CPUS:-4}"           # Number of CPU cores
VM_DISK_SIZE="${VM_DISK_SIZE:-40G}"  # Disk size (qcow2, no preallocation)

# Network configuration
VM_SSH_PORT="${VM_SSH_PORT:-2222}"  # Host port for SSH forwarding

# Derived paths (do not modify these)
VM_IMAGE="${VM_DIR}/dfly-${DFLY_VERSION}.iso"
VM_DISK="${VM_DIR}/dfly-vm.qcow2"
VM_SNAPSHOT="${VM_DIR}/dfly-vm-clean.qcow2"
VM_DOWNLOAD_URL="https://mirror-master.dragonflybsd.org/iso-images/dfly-x86_64-${DFLY_VERSION}_REL.iso.bz2"

# SSH connection settings
VM_SSH_KEY="${VM_DIR}/id_ed25519"
VM_SSH_OPTS="-p ${VM_SSH_PORT} -i ${VM_SSH_KEY} -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
VM_SSH_HOST="root@localhost"

# Export variables for use in scripts
export DFLY_VERSION VM_DIR VM_MEMORY VM_CPUS VM_DISK_SIZE
export VM_SSH_PORT VM_SSH_KEY VM_IMAGE VM_DISK VM_SNAPSHOT VM_DOWNLOAD_URL
export VM_SSH_OPTS VM_SSH_HOST
