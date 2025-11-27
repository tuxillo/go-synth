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

# Logging setup - write to both console and log file
LOG_FILE="/tmp/phase1-install.log"
exec > >(tee -a "$LOG_FILE") 2>&1

# Error trap for debugging
trap 'echo "ERROR at line $LINENO: $BASH_COMMAND"; echo "Pausing for 60 seconds for inspection..."; sleep 60' ERR

echo "============================================"
echo "Phase 1: Automated OS Installation Starting"
echo "============================================"
echo "Log file: $LOG_FILE"
echo ""

# Target disk device
DISK="/dev/da0"
ROOT_PART="${DISK}s1a"
HAMMER2_PART="${DISK}s1d"

# Step 0: Detect available CD devices
echo "Step 0: Detecting CD devices..."
ls -l /dev/cd* || echo "Warning: No /dev/cd* devices found"
echo ""

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

# Step 5: Copy system from clean ISO
echo "Step 5: Copying DragonFlyBSD system from ISO..."

# Try to find the correct ISO device
ISO_SOURCE=""
for dev in /dev/cd2 /dev/cd1 /dev/cd0; do
    if [ -e "$dev" ]; then
        echo "  Trying to mount $dev..."
        mkdir -p /mnt/cd_tmp
        if mount_cd9660 "$dev" /mnt/cd_tmp 2>/dev/null; then
            # Check if this looks like an installer ISO (has /boot, /bin, etc)
            if [ -d "/mnt/cd_tmp/boot" ] && [ -d "/mnt/cd_tmp/bin" ]; then
                ISO_SOURCE="$dev"
                echo "  Found installer ISO at $dev"
                umount /mnt/cd_tmp
                break
            fi
            umount /mnt/cd_tmp
        fi
    fi
done

if [ -z "$ISO_SOURCE" ]; then
    echo "ERROR: Could not find installer ISO!"
    echo "Available devices:"
    ls -l /dev/cd* || true
    exit 1
fi

# Mount the ISO and copy
echo "  Mounting $ISO_SOURCE for cpdup..."
mkdir -p /mnt/cd_source
mount_cd9660 "$ISO_SOURCE" /mnt/cd_source

# Copy system files (this takes a few minutes)
echo "  Running cpdup (this may take 3-5 minutes)..."
cpdup /mnt/cd_source /mnt

# Unmount and cleanup
umount /mnt/cd_source
rmdir /mnt/cd_source

# Step 6: Configure /etc/fstab
echo "Step 6: Configuring /etc/fstab..."
cat > /mnt/etc/fstab <<EOF
# Device                Mountpoint      FStype  Options         Dump    Pass#
${HAMMER2_PART}        /               hammer2 rw              1       1
${ROOT_PART}           /boot           ufs     rw              1       1
proc                    /proc           procfs  rw              0       0
EOF

# Step 7: Configure /boot/loader.conf for serial console
echo "Step 7: Configuring boot loader (serial console only)..."
cat > /mnt/boot/loader.conf <<'EOF'
# Boot configuration - Serial console only to avoid character doubling
console="comconsole"
comconsole_speed="115200"
comconsole_port="0x3F8"
boot_serial="-h"
autoboot_delay="3"
# Kernel console output
kern.console="comconsole"
EOF

# Step 7b: Configure /etc/ttys for serial console
echo "Step 7b: Configuring /etc/ttys for serial console..."
# Enable serial console on ttyd0
if grep -q '^ttyd0' /mnt/etc/ttys; then
    sed -i '' 's|^ttyd0.*|ttyd0   "/usr/libexec/getty std.115200"   vt100   on  secure|' /mnt/etc/ttys
else
    echo 'ttyd0   "/usr/libexec/getty std.115200"   vt100   on  secure' >> /mnt/etc/ttys
fi

# Step 8: Configure /etc/rc.conf
echo "Step 8: Configuring system services..."
cat > /mnt/etc/rc.conf <<'EOF'
# Network configuration
hostname="dragonfly-gosynth"
ifconfig_em0="DHCP"
sshd_enable="YES"

# Serial console
dumpdev="AUTO"

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

# Step 10: Set root password (default: 'root' for initial setup)
echo "Step 10: Setting root password..."
echo "root" | chroot /mnt pw usermod root -h 0

# Step 11: Configure resolv.conf for DNS
echo "Step 11: Configuring DNS..."
cat > /mnt/etc/resolv.conf <<'EOF'
nameserver 8.8.8.8
nameserver 8.8.4.4
EOF

# Step 12: Create necessary directories
echo "Step 12: Creating system directories..."
mkdir -p /mnt/root/.ssh
chmod 700 /mnt/root/.ssh

# Step 13: Copy log file to installed system
echo "Step 13: Copying installation log..."
cp "$LOG_FILE" /mnt/root/phase1-install.log

# Step 14: Sync and unmount
echo "Step 14: Finalizing installation..."
sync
sleep 2
umount /mnt

echo ""
echo "============================================"
echo "Phase 1: Installation Complete!"
echo "============================================"
echo "Installation log saved to /root/phase1-install.log"
echo "Shutting down to proceed to Phase 2..."
echo ""
sleep 3

# Power off so orchestrator can proceed to next phase
poweroff
