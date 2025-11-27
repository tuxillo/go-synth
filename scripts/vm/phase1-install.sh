#!/bin/sh
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
# IMPORTANT: Uses /bin/sh (not bash) - must be POSIX compatible
#
# Device layout:
#   /dev/cd0  - DragonFlyBSD installer ISO (main boot)
#   /dev/cd1  - This PFI script ISO
#   /dev/cd2  - DragonFlyBSD installer ISO (for clean cpdup source)
#   /dev/da0  - Virtual disk (target)

set -ex

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

# Step 3: Create custom label for proper partition layout
echo "Step 3: Creating partition layout..."
disklabel da0s1 > /tmp/label
echo 'a: 1G 0 4.2BSD
d: * * HAMMER2' >> /tmp/label
disklabel -R -r da0s1 /tmp/label

# Step 4: Format partitions
echo "Step 4: Formatting partitions..."
newfs "${ROOT_PART}"
newfs_hammer2 -L ROOT "${HAMMER2_PART}"

# Step 5: Mount HAMMER2 partition
echo "Step 5: Mounting HAMMER2 partition..."
mount "${HAMMER2_PART}" /mnt
mkdir -p /mnt/boot
mount "${ROOT_PART}" /mnt/boot

# Step 6: Copy system from clean ISO
echo "Step 6: Copying DragonFlyBSD system from ISO..."

# Mount the clean ISO (cd2 as per golang/build convention)
mkdir -p /mnt/mnt
mount_cd9660 /dev/cd2 /mnt/mnt

# Copy boot files first
echo "  Copying /boot..."
cpdup /mnt/mnt/boot /mnt/boot

# Copy entire system
echo "  Copying system files (this takes 3-5 minutes)..."
cpdup /mnt/mnt /mnt

# Unmount and cleanup
umount /mnt/mnt

# Step 7: Configure /etc/fstab
echo "Step 7: Configuring /etc/fstab..."
cat > /mnt/etc/fstab <<'EOF'
# Device                Mountpoint      FStype  Options         Dump    Pass#
da0s1a                  /boot           ufs     rw              1       1
da0s1d                  /               hammer2 rw              1       1
proc                    /proc           procfs  rw              0       0
EOF

# Step 8: Configure /boot/loader.conf for serial console (minimal config)
echo "Step 8: Configuring boot loader..."
cat > /mnt/boot/loader.conf <<'EOF'
console=comconsole
vfs.root.mountfrom=hammer2:da0s1d
EOF

# Step 9: Configure /etc/rc.conf
echo "Step 9: Configuring system services..."
cat > /mnt/etc/rc.conf <<'EOF'
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

# Step 10: Enable root SSH login (needed for automation)
echo "Step 10: Configuring SSH..."
cat > /mnt/etc/ssh/sshd_config <<'EOF'
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

# Step 11: Set root password (default: 'root' for initial setup)
echo "Step 11: Setting root password..."
echo "root" | chroot /mnt pw usermod root -h 0

# Step 12: Configure resolv.conf for DNS
echo "Step 12: Configuring DNS..."
cat > /mnt/etc/resolv.conf <<'EOF'
nameserver 8.8.8.8
nameserver 8.8.4.4
EOF

# Step 13: Create necessary directories
echo "Step 13: Creating system directories..."
mkdir -p /mnt/root/.ssh
chmod 700 /mnt/root/.ssh

# Step 14: Unmount filesystems
echo "Step 14: Finalizing installation..."
umount /mnt/boot
umount /mnt

echo ""
echo "============================================"
echo "Phase 1: Installation Complete!"
echo "============================================"
echo "DONE WITH PHASE 1."
sync
echo "Shutting down to proceed to Phase 2..."
sleep 2

# Power off so orchestrator can proceed to next phase
poweroff
sleep 86400
