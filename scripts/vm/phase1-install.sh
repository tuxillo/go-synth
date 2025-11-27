#!/bin/bash
# Phase 1: Automated DragonFlyBSD OS Installation
#
# This script is executed automatically by the DragonFlyBSD installer via PFI
# (Platform Firmware Interface). It performs a fully automated OS installation:
#   - Partitions and formats the disk
#   - Copies system files from the installer ISO
#   - Configures boot loader, networking, and SSH
#   - Sets up serial console for automation
#
# Adapted from: https://github.com/golang/build/blob/master/env/dragonfly-amd64/phase1.sh
#
# Device layout:
#   /dev/cd0  - DragonFlyBSD installer ISO (main boot)
#   /dev/cd1  - This PFI script ISO
#   /dev/cd2  - DragonFlyBSD installer ISO (for clean cpdup source)
#   /dev/da0  - Virtual disk (target)

set -euxo pipefail

echo "============================================"
echo "Phase 1: Automated OS Installation Starting"
echo "============================================"

# Target disk device
DISK="/dev/da0"
ROOT_PART="${DISK}s1a"
HAMMER2_PART="${DISK}s1d"

# Step 1: Partition the disk
echo "Step 1: Partitioning disk ${DISK}..."
fdisk -IB "${DISK}"

# Step 2: Create disklabel with root (UFS) and main (HAMMER2) partitions
echo "Step 2: Creating disklabel..."
disklabel -r -w -B "${DISK}s1" auto

# Step 3: Format partitions
echo "Step 3: Formatting partitions..."
newfs "${ROOT_PART}"
newfs_hammer2 -L ROOT "${HAMMER2_PART}"

# Step 4: Mount HAMMER2 partition
echo "Step 4: Mounting HAMMER2 partition..."
mount "${HAMMER2_PART}" /mnt

# Step 5: Copy system from clean ISO (cd2)
echo "Step 5: Copying DragonFlyBSD system from ISO..."
# Mount the clean ISO
mkdir -p /mnt/cd2
mount_cd9660 /dev/cd2 /mnt/cd2

# Copy system files (this takes a few minutes)
cpdup /mnt/cd2 /mnt

# Unmount the ISO
umount /mnt/cd2
rmdir /mnt/cd2

# Step 6: Configure /etc/fstab
echo "Step 6: Configuring /etc/fstab..."
cat > /mnt/etc/fstab <<EOF
# Device                Mountpoint      FStype  Options         Dump    Pass#
${HAMMER2_PART}        /               hammer2 rw              1       1
${ROOT_PART}           /boot           ufs     rw              1       1
proc                    /proc           procfs  rw              0       0
EOF

# Step 7: Configure /boot/loader.conf
echo "Step 7: Configuring boot loader..."
cat > /mnt/boot/loader.conf <<EOF
# Boot configuration
console="comconsole,vidconsole"
comconsole_speed="115200"
boot_serial="-D -h"
autoboot_delay="1"
EOF

# Step 8: Configure /etc/rc.conf
echo "Step 8: Configuring system services..."
cat > /mnt/etc/rc.conf <<EOF
# Network configuration
hostname="dragonfly-gosynth"
ifconfig_em0="DHCP"
sshd_enable="YES"

# Time synchronization
ntpd_enable="YES"
ntpd_sync_on_start="YES"

# Disable sendmail
sendmail_enable="NO"
sendmail_submit_enable="NO"
sendmail_outbound_enable="NO"
sendmail_msp_queue_enable="NO"

# Performance tuning
powerd_enable="YES"
EOF

# Step 9: Enable root SSH login (needed for automation)
echo "Step 9: Configuring SSH..."
cat > /mnt/etc/ssh/sshd_config <<EOF
# SSH Configuration for VM automation
PermitRootLogin yes
PasswordAuthentication yes
ChallengeResponseAuthentication no
UsePAM yes
X11Forwarding no
PrintMotd yes
AcceptEnv LANG LC_*
Subsystem sftp /usr/libexec/sftp-server
EOF

# Step 10: Set root password (default: 'root' for initial setup)
echo "Step 10: Setting root password..."
echo 'root:$6$rounds=5000$DragonFly$7QvI8xvXN.K9Q3K3O9xZ9K3Q3K3O9xZ9K3Q3K3O9xZ9K3Q3K3O9xZ9K3Q3K3O9xZ9K3Q3K3O9xZ9K3' | chroot /mnt chpass -u root -l

# Alternative: Set a simple password for testing
echo "root" | chroot /mnt pw usermod root -h 0

# Step 11: Configure resolv.conf for DNS
echo "Step 11: Configuring DNS..."
cat > /mnt/etc/resolv.conf <<EOF
nameserver 8.8.8.8
nameserver 8.8.4.4
EOF

# Step 12: Create necessary directories
echo "Step 12: Creating system directories..."
mkdir -p /mnt/root/.ssh
chmod 700 /mnt/root/.ssh

# Step 13: Sync and unmount
echo "Step 13: Finalizing installation..."
sync
umount /mnt

echo "============================================"
echo "Phase 1: Installation Complete!"
echo "============================================"
echo "Shutting down to proceed to Phase 2..."
sleep 2

# Power off so orchestrator can proceed to next phase
poweroff
