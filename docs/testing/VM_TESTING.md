# VM Testing Infrastructure

This document describes the DragonFlyBSD VM testing infrastructure for `dsynth-go`, designed to enable local, deterministic testing of Phase 4 mount operations that require BSD-specific system calls and root privileges.

## Table of Contents

- [Overview](#overview)
- [Why a VM?](#why-a-vm)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Makefile Targets](#makefile-targets)
- [Development Workflow](#development-workflow)
- [Testing Workflow](#testing-workflow)
- [Troubleshooting](#troubleshooting)
- [Maintenance](#maintenance)
- [Advanced Usage](#advanced-usage)

---

## Overview

The VM testing infrastructure provides:

- **Local Testing**: VM runs on your laptop, OpenCode can access files
- **Programmatic Control**: Create, destroy, snapshot VMs via Makefile
- **Deterministic State**: Snapshot-based restoration for clean test runs
- **Fast Iteration**: Sync code + run tests in seconds
- **Root Access**: Test mount/chroot operations requiring privileges

**Technology Stack**:
- Host: Ubuntu 24.04 with QEMU/KVM
- Guest: DragonFlyBSD 6.4.0 (x86_64)
- Provisioning: Shell scripts + SSH keys
- Management: Makefile + Bash scripts

---

## Why a VM?

Phase 4 of `dsynth-go` implements a complex worker environment with 27 mount points, requiring:

1. **BSD-Specific System Calls**: `nullfs`, `tmpfs`, `devfs`, `procfs` mounts
2. **Root Privileges**: Cannot test mount/chroot without root
3. **Isolation**: Each worker needs its own chroot environment
4. **Cleanup**: Testing mount retry logic and error handling

**Existing E2E tests are comprehensive** (4,875 lines covering pkg, builddb, build), but **5 critical integration tests are SKIPPED** because they require:
- Root access
- BSD mount operations
- Real filesystem behavior

**Without VM testing, we cannot verify Phase 4 functionality**.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Ubuntu 24.04 Laptop (Host)                                  │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │ go-synth/ (Project Directory)                      │    │
│  │                                                     │    │
│  │  • Code edited locally (OpenCode access)           │    │
│  │  • Git repo                                        │    │
│  │  • Makefile targets                                │    │
│  └──────────────┬─────────────────────────────────────┘    │
│                 │                                            │
│                 │ make vm-sync (rsync over SSH)             │
│                 │ make vm-test-* (SSH commands)             │
│                 ▼                                            │
│  ┌────────────────────────────────────────────────────┐    │
│  │ QEMU/KVM Virtual Machine                           │    │
│  │                                                     │    │
│  │  ┌──────────────────────────────────────────────┐  │    │
│  │  │ DragonFlyBSD 6.4.0                           │  │    │
│  │  │                                               │  │    │
│  │  │  • /root/go-synth/ (synced from host)        │  │    │
│  │  │  • Go toolchain installed                    │  │    │
│  │  │  • doas configured (passwordless root)       │  │    │
│  │  │  • SSH server (port 2222 forwarded)          │  │    │
│  │  │  • 20GB disk (QCOW2 format)                  │  │    │
│  │  │  • Clean snapshot for restoration            │  │    │
│  │  │                                               │  │    │
│  │  │  ┌────────────────────────────────────────┐  │  │    │
│  │  │  │ Phase 4 Test Execution                 │  │  │    │
│  │  │  │                                         │  │  │    │
│  │  │  │  • Mount 27 filesystems                │  │  │    │
│  │  │  │  • Create chroot environments          │  │  │    │
│  │  │  │  • Test worker isolation               │  │  │    │
│  │  │  │  • Verify cleanup logic                │  │  │    │
│  │  │  └────────────────────────────────────────┘  │  │    │
│  │  └──────────────────────────────────────────────┘  │    │
│  └────────────────────────────────────────────────────┘    │
│                                                              │
│  vm/ directory:                                             │
│   • dfly-vm.qcow2 (VM disk image)                          │
│   • dfly-6.4.0.iso (installation media)                    │
│   • dfly-vm-clean.qcow2 (snapshot)                         │
└─────────────────────────────────────────────────────────────┘
```

**Key Design Decisions**:

1. **QEMU/KVM**: Native Linux virtualization, already installed on Ubuntu 24.04
2. **SSH Port Forwarding**: Host port 2222 → VM port 22
3. **Rsync**: Fast, incremental file sync (excludes `.git`, `vm/`)
4. **Snapshot-Based**: Clean state restoration in seconds
5. **Makefile Integration**: Simple, discoverable commands

---

## Prerequisites

**On your Ubuntu 24.04 laptop**:

1. **QEMU/KVM** (already installed):
   ```bash
   qemu-system-x86_64 --version  # Should show version 8.2.x
   ```

2. **Disk Space**:
   - 300MB for DragonFlyBSD ISO
   - 20GB for VM disk image
   - Total: ~21GB

3. **Network**: Internet access to download ISO (one-time)

4. **SSH**: OpenSSH client (already installed on Ubuntu)

**No system package installations required** - everything runs in project directory.

---

## Quick Start

### First-Time Setup (Run Once)

This takes ~15 minutes including manual OS installation.

#### Step 1: Download ISO and Create Disk

```bash
cd /home/antonioh/s/go-synth
make vm-setup
```

This will:
- Download DragonFlyBSD 6.4.0 ISO (~300MB) to `vm/`
- Create 20GB QCOW2 disk image at `vm/dfly-vm.qcow2`

#### Step 2: Install DragonFlyBSD

```bash
make vm-install
```

This boots the VM with the ISO attached. Follow the installation prompts:

1. **Installer Menu**: Select "Install DragonFly"
2. **Disk Selection**: Use `da0` (entire disk)
3. **Partitioning**: Accept defaults (automatic)
4. **Packages**: Install base system only (no X11)
5. **Root Password**: Set a password (you'll use this for SSH)
6. **Network**: Configure DHCP on `em0`
7. **Services**: Enable SSH server
8. **Reboot**: Remove ISO when prompted

**Installation takes ~10 minutes.**

#### Step 3: Provision the VM

After installation, SSH into the VM:

```bash
ssh -p 2222 root@localhost
```

Run the provisioning script (synced to VM during install):

```bash
cd /root
./scripts/vm/provision.sh
```

This script will:
- Configure `doas` for passwordless root access
- Install Go toolchain
- Create `/root/go-synth/` directory
- Set up SSH authorized keys (for passwordless login)
- Install development tools

**Provisioning takes ~5 minutes.**

#### Step 4: Create Clean Snapshot

After provisioning, exit the VM and create a snapshot:

```bash
exit  # Exit SSH session
make vm-snapshot
```

This saves the VM state to `vm/dfly-vm-clean.qcow2`. You can now restore to this clean state anytime with `make vm-restore`.

---

## Makefile Targets

### Lifecycle Management

| Target | Description |
|--------|-------------|
| `make vm-setup` | Download ISO, create disk (first-time only) |
| `make vm-install` | Boot VM for OS installation (first-time only) |
| `make vm-snapshot` | Save current VM state as clean snapshot |
| `make vm-start` | Start the VM |
| `make vm-stop` | Stop the VM gracefully |
| `make vm-destroy` | Delete VM and all data (prompts for confirmation) |
| `make vm-restore` | Restore VM to clean snapshot |
| `make vm-ssh` | SSH into the running VM |
| `make vm-status` | Show VM status and info |

### Testing Targets

| Target | Description |
|--------|-------------|
| `make vm-sync` | Sync project files to VM |
| `make vm-build` | Build `dsynth` in VM |
| `make vm-test-unit` | Run unit tests |
| `make vm-test-integration` | Run integration tests |
| `make vm-test-phase4` | Run Phase 4 tests (mount, chroot) |
| `make vm-test-e2e` | Run end-to-end tests |
| `make vm-test-all` | Run all tests |
| `make vm-quick` | Quick cycle: sync + Phase 4 tests |

### Help

| Target | Description |
|--------|-------------|
| `make vm-help` | Show all VM targets with descriptions |

---

## Development Workflow

### Daily Development Cycle

```bash
# 1. Start VM (30 seconds)
make vm-start

# 2. Edit code locally (OpenCode has full access)
# ... edit files in go-synth/ ...

# 3. Quick test cycle (sync + Phase 4 tests)
make vm-quick

# 4. Stop VM when done
make vm-stop
```

### Longer Testing Session

```bash
# Start VM
make vm-start

# Full test suite
make vm-test-all

# Or run specific test suites
make vm-test-unit           # Unit tests only
make vm-test-integration    # Integration tests only
make vm-test-phase4         # Phase 4 mount tests only

# Stop VM
make vm-stop
```

### Iterating on Phase 4 Code

```bash
# Start VM
make vm-start

# Edit internal/worker/*.go locally
# ... make changes ...

# Test immediately
make vm-quick

# Repeat: edit → test → edit → test
```

---

## Testing Workflow

### Phase 4 Testing Strategy

Phase 4 tests require:
1. **Root privileges** (for mount operations)
2. **BSD-specific filesystems** (nullfs, tmpfs, devfs, procfs)
3. **Chroot** (for worker isolation)

**Test Execution**:

```bash
# Run Phase 4 tests with root access
make vm-test-phase4
```

This executes:
```bash
doas go test -v ./internal/worker/...
```

**Test Coverage**:
- Mount point creation (27 mounts per worker)
- Nullfs overlays for `/usr/local`, `/usr/src`, etc.
- Tmpfs for `/tmp`, `/var/tmp`
- Devfs/Procfs mounting
- Chroot environment setup
- Concurrent worker isolation
- Cleanup retry logic
- Error handling for mount failures

### Full Test Suite

```bash
# Run everything (unit + integration + Phase 4)
make vm-test-all
```

This runs:
1. Unit tests (no root required)
2. Integration tests (builddb, pkg parsing)
3. Phase 4 tests (root + mount operations)

### Test-Driven Development

```bash
# 1. Write failing test locally
vim internal/worker/mount_test.go

# 2. Sync and run test
make vm-quick

# 3. Fix code
vim internal/worker/mount.go

# 4. Retest
make vm-quick

# 5. Repeat until passing
```

---

## Troubleshooting

### VM Won't Boot

**Symptom**: `make vm-start` hangs or fails

**Solutions**:
1. Check if VM is already running: `make vm-status`
2. Stop existing VM: `make vm-stop`
3. Check QEMU process: `pgrep -af qemu-system`
4. Kill stuck process: `pkill -9 qemu-system-x86_64`
5. Retry: `make vm-start`

### SSH Connection Refused

**Symptom**: Cannot SSH to VM (`ssh -p 2222 root@localhost` fails)

**Solutions**:
1. Wait longer (VM takes 30-60s to boot fully)
2. Check VM status: `make vm-status`
3. Verify SSH service in VM: `make vm-ssh` then `service sshd status`
4. Check port forwarding: `netstat -tln | grep 2222`

### VM Disk Corruption

**Symptom**: VM won't boot or filesystem errors

**Solution**: Restore from clean snapshot
```bash
make vm-restore
make vm-start
```

### Tests Fail in VM

**Symptom**: Tests pass on host but fail in VM

**Debugging**:
1. SSH into VM: `make vm-ssh`
2. Check Go version: `go version`
3. Run tests manually: `cd /root/go-synth && go test -v ./internal/worker/...`
4. Check system logs: `dmesg | tail -50`
5. Check mount points: `mount | grep worker`

### Out of Disk Space

**Symptom**: VM reports disk full

**Solutions**:
1. SSH into VM: `make vm-ssh`
2. Check usage: `df -h`
3. Clean Go cache: `go clean -cache -testcache -modcache`
4. Remove old build artifacts: `rm -rf /root/go-synth/dsynth`
5. If necessary, restore from clean snapshot: `make vm-restore`

### Slow VM Performance

**Symptom**: VM is sluggish or tests take too long

**Solutions**:
1. Check KVM is enabled: `lsmod | grep kvm`
2. Verify QEMU uses KVM: `ps aux | grep qemu | grep enable-kvm`
3. Increase VM memory: Edit `scripts/vm/start-vm.sh`, change `-m 2G` to `-m 4G`
4. Check host resources: `htop`

---

## Maintenance

### Updating DragonFlyBSD

When a new DragonFlyBSD release is available:

1. Download new ISO:
   ```bash
   cd vm/
   wget https://mirror-master.dragonflybsd.org/iso-images/dfly-x86_64-6.5.0_REL.iso.bz2
   bunzip2 dfly-x86_64-6.5.0_REL.iso.bz2
   ```

2. Update `scripts/vm/fetch-dfly-image.sh` with new URL/version

3. Recreate VM:
   ```bash
   make vm-destroy
   make vm-setup
   make vm-install
   # ... provision ...
   make vm-snapshot
   ```

### Updating Provisioning

To change VM provisioning (e.g., install new packages):

1. Edit `scripts/vm/provision.sh`
2. Restore clean snapshot: `make vm-restore`
3. Start VM: `make vm-start`
4. SSH and run provisioning: `make vm-ssh`, then `./scripts/vm/provision.sh`
5. Save new snapshot: `make vm-snapshot`

### Cleaning Up

Remove VM infrastructure completely:

```bash
# Delete VM and all data
make vm-destroy

# Remove ISO
rm vm/dfly-*.iso

# Remove snapshots
rm vm/dfly-vm-clean.qcow2
```

---

## Advanced Usage

### Manual VM Control

If you need finer control, use scripts directly:

```bash
# Start VM with custom options
./scripts/vm/start-vm.sh

# Stop VM
./scripts/vm/stop-vm.sh

# Create snapshot
./scripts/vm/snapshot-clean.sh

# Restore snapshot
./scripts/vm/restore-vm.sh
```

### SSH Without Makefile

```bash
ssh -p 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@localhost
```

### Rsync Without Makefile

```bash
rsync -avz --delete --exclude='.git' --exclude='vm/' \
  -e "ssh -p 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null" \
  . root@localhost:/root/go-synth/
```

### Run Tests Without Makefile

```bash
# SSH into VM
ssh -p 2222 root@localhost

# Run tests manually
cd /root/go-synth
make build
doas go test -v ./internal/worker/...
```

### Multiple VM Instances

To run multiple VMs (e.g., different DragonFlyBSD versions):

1. Copy VM scripts: `cp -r scripts/vm scripts/vm-6.4`
2. Edit `scripts/vm-6.4/start-vm.sh`: Change disk path and SSH port
3. Create separate disk: `scripts/vm-6.4/create-disk.sh`
4. Add Makefile targets with `vm2-*` prefix

### Performance Tuning

Edit `scripts/vm/start-vm.sh` to customize VM resources:

```bash
# Increase CPU cores
-smp 2  →  -smp 4

# Increase RAM
-m 2G  →  -m 4G

# Change disk cache mode
-drive ...,cache=writeback  →  -drive ...,cache=none
```

---

## Integration with Phase 4

Phase 4 implementation (`docs/design/PHASE_4_TODO.md`) requires this VM infrastructure as a **prerequisite**.

**Task 0 (VM Setup)** must complete before **Tasks 1-10 (Phase 4 Implementation)**.

Phase 4 tests will verify:
- Worker environment setup (`internal/worker/environment.go`)
- Mount point creation (`internal/worker/mount.go`)
- Chroot execution (`internal/worker/chroot.go`)
- Cleanup and error handling
- Concurrent worker isolation

Without this VM infrastructure, Phase 4 cannot be tested or verified.

---

## See Also

- `docs/design/PHASE_4_TODO.md` - Phase 4 implementation plan
- `DEVELOPMENT.md` - General development guide
- `scripts/vm/` - VM management scripts
- `Makefile` - VM target definitions

---

## Questions?

If you encounter issues not covered here:

1. Check VM status: `make vm-status`
2. Review logs: `make vm-ssh`, then `dmesg | tail -100`
3. Restore clean state: `make vm-restore`
4. Consult DragonFlyBSD documentation: https://www.dragonflybsd.org/docs/

For `dsynth-go` specific issues, see `DEVELOPMENT.md`.
