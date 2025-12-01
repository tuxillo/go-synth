# VM Testing Infrastructure

This document describes the DragonFlyBSD VM testing infrastructure for `go-synth`, designed to enable local, deterministic testing of Phase 4 mount operations that require BSD-specific system calls and root privileges.

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
- Guest: DragonFlyBSD 6.4.2 (x86_64, configurable)
- Provisioning: Shell scripts + SSH keys
- Management: Makefile + Bash scripts

---

## Why a VM?

Phase 4 of `go-synth` implements a complex worker environment with 27 mount points, requiring:

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
│  │  │ DragonFlyBSD 6.4.2                           │  │    │
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
│   • dfly-6.4.2.iso (installation media, configurable)     │
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

### First-Time Setup (Automated - Recommended)

**NEW**: Fully automated installation using DragonFlyBSD's PFI (Platform Firmware Interface) system, adapted from the [golang/build](https://github.com/golang/build/tree/master/env/dragonfly-amd64) automation approach.

**Total time: ~15 minutes, zero interaction required.**

#### Prerequisites Check

```bash
make vm-check-prereqs
```

Ensures you have:
- `qemu-system-x86_64` (already installed on Ubuntu 24.04)
- `genisoimage` (install with: `sudo apt-get install genisoimage`)

#### Step 1: Download ISO

```bash
cd /home/antonioh/s/go-synth
make vm-setup
```

Downloads DragonFlyBSD 6.4.2 ISO (~300MB) to `~/.go-synth/vm/`.

#### Step 2: Automated Installation

```bash
make vm-auto-install
```

This single command runs three automated phases:

**Phase 1: OS Installation** (~5 min)
- Partitions and formats disk (`fdisk`, `disklabel`, `newfs`)
- Installs DragonFlyBSD base system (`cpdup` from ISO)
- Configures boot loader with serial console
- Sets up networking (DHCP on em0)
- Enables SSH server with root login
- Powers off automatically

**Phase 2: Package Updates** (~3 min)
- Boots installed system
- Updates pkg repository metadata
- Upgrades all packages to latest versions
- Installs required packages:
  - `go` - Go compiler
  - `bash` - Bash shell
  - `git` - Version control
  - `rsync` - File synchronization
  - `curl`, `wget` - HTTP clients
  - `doas` - Privilege escalation
- Powers off automatically

**Phase 3: Provisioning** (~2 min)
- Boots system again
- Configures doas for passwordless root
- Creates directories:
  - `/build/synth/Workers` - Worker chroot environments
  - `/usr/dports` - Ports tree location
- Sets up Go environment (GOPATH, GOCACHE)
- Configures bash as default shell
- Verifies all configurations
- Creates snapshot for quick restoration
- Powers off automatically

**Result**: Clean, provisioned VM ready for Phase 4 testing.

#### Step 3: Start Testing

```bash
make vm-start   # Boot VM (30 seconds)
make vm-quick   # Run Phase 4 tests
```

---

### First-Time Setup (Manual - Alternative)

This takes ~15 minutes including manual OS installation.

#### Step 1: Download ISO and Create Disk

```bash
cd /home/antonioh/s/go-synth
make vm-setup
```

This will:
- Download DragonFlyBSD 6.4.2 ISO (~300MB) to `vm/` (version configurable)
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

### Setup & Lifecycle Management

| Target | Description |
|--------|-------------|
| `make vm-check-prereqs` | Check for QEMU and genisoimage (run first) |
| `make vm-setup` | Download DragonFlyBSD ISO (first-time only) |
| `make vm-auto-install` | **Fully automated installation** (15 min, zero interaction) |
| `make vm-install` | Boot VM for manual OS installation (alternative to auto-install) |
| `make vm-snapshot` | Save current VM state as clean snapshot |
| `make vm-start` | Start the VM |
| `make vm-stop` | Stop the VM gracefully |
| `make vm-destroy` | Delete VM and all data (prompts for confirmation) |
| `make vm-restore` | Restore VM to clean snapshot |
| `make vm-ssh` | SSH into the running VM |
| `make vm-status` | Show VM status and info |
| `make vm-clean-phases` | Remove temporary phase ISOs |

### Testing Targets

| Target | Description |
|--------|-------------|
| `make vm-sync` | Sync project files to VM |
| `make vm-build` | Build `go-synth` in VM |
| `make vm-test-unit` | Run unit tests |
| `make vm-test-integration` | Run integration tests (tags=integration) |
| `make vm-test-integration-e2e` | Run E2E integration tests (requires BSD) |
| `make vm-test-phase4` | Run Phase 4 tests (mount, chroot) |
| `make vm-test-e2e` | Run end-to-end tests (tags=e2e) |
| `make vm-test-all` | Run all tests (unit + integration + phase4) |
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
make vm-test-unit                # Unit tests only
make vm-test-integration         # Integration tests (tags=integration)
make vm-test-integration-e2e     # E2E integration tests (requires BSD)
make vm-test-phase4              # Phase 4 mount tests only

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

### Automated Installation Issues

#### Phase 1 (Installation) Fails

**Symptom**: `vm-auto-install` stops during Phase 1

**Common Causes**:
1. **ISO not found**: Run `make vm-setup` first
2. **Disk already exists**: Remove with `rm ~/.go-synth/vm/dfly-vm.qcow2` and retry
3. **Insufficient disk space**: Need 20GB free
4. **Device naming mismatch**: DragonFlyBSD version differences

**Debug**:
```bash
# Check phase1 script manually
cat scripts/vm/phase1-install.sh

# Run phase1 alone (advanced)
./scripts/vm/make-phase-iso.sh scripts/vm/phase1-install.sh /tmp/phase1.iso
./scripts/vm/run-phase.sh 1 /tmp/phase1.iso
```

#### Phase 2 (Package Updates) Fails

**Symptom**: `vm-auto-install` stops during Phase 2

**Common Causes**:
1. **Network issues**: Pkg repository unreachable
2. **Package unavailable**: Specific package version missing
3. **Disk full**: OS installation took more space than expected

**Debug**:
```bash
# Boot VM manually to inspect
qemu-system-x86_64 -hda ~/.go-synth/vm/dfly-vm.qcow2 -m 2G -nographic

# Inside VM:
pkg update -f
pkg search go
df -h
```

#### Phase 3 (Provisioning) Fails

**Symptom**: `vm-auto-install` stops during Phase 3

**Common Causes**:
1. **Doas configuration error**: Syntax issue in doas.conf
2. **Go not found**: Phase 2 didn't complete successfully
3. **Directory creation fails**: Permissions issue

**Debug**:
```bash
# Check provisioning script
cat scripts/vm/phase3-provision.sh

# Boot and check manually
make vm-start
make vm-ssh
# Inside VM:
command -v go
ls -ld /build/synth/Workers /usr/dports
cat /usr/local/etc/doas.conf
```

#### genisoimage Not Found

**Symptom**: `make vm-auto-install` fails with "genisoimage not found"

**Solution**:
```bash
sudo apt-get install genisoimage
```

#### Automation Hangs Indefinitely

**Symptom**: Phase script runs but never completes

**Possible Causes**:
1. **Waiting for user input**: Script expects interactive response
2. **Network timeout**: Pkg operations stalled
3. **Disk I/O issues**: Slow host system

**Solution**:
```bash
# Kill QEMU process
pkill -9 qemu-system-x86_64

# Check system resources
df -h
free -h
iotop

# Retry with more verbose output
bash -x scripts/vm/auto-install.sh
```

#### Clean Up After Failed Automation

```bash
# Remove temporary phase ISOs
make vm-clean-phases

# Remove incomplete VM disk
rm ~/.go-synth/vm/dfly-vm.qcow2

# Start fresh
make vm-setup
make vm-auto-install
```

---

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
4. Remove old build artifacts: `rm -rf /root/go-synth/go-synth`
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

### Updating DragonFlyBSD Version

When a new DragonFlyBSD release is available, update the version configuration:

**Method 1: Update config.sh (Recommended)**

1. Check latest release at: https://mirror-master.dragonflybsd.org/iso-images/

2. Edit `scripts/vm/config.sh`:
   ```bash
   # Change this line:
   DFLY_VERSION="${DFLY_VERSION:-6.4.2}"
   # To:
   DFLY_VERSION="${DFLY_VERSION:-6.6.0}"
   ```

3. Recreate VM:
   ```bash
   make vm-destroy
   make vm-setup
   make vm-install
   # ... provision ...
   make vm-snapshot
   ```

**Method 2: Environment Override (One-Time)**

Test a new version without modifying config:
```bash
DFLY_VERSION=6.6.0 make vm-setup
DFLY_VERSION=6.6.0 make vm-install
# ... provision ...
DFLY_VERSION=6.6.0 make vm-snapshot
DFLY_VERSION=6.6.0 make vm-start
```

**Version Compatibility**:
- Tested with: 6.4.2 (latest stable as of Nov 2025)
- Should work with: Any 6.x release
- May work with: Future 7.x releases (untested)

**After updating**, verify the version:
```bash
make vm-start
make vm-ssh
uname -a  # Should show new version
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

### Automated Installation Deep Dive

#### PFI (Platform Firmware Interface)

The automated installation uses DragonFlyBSD's **PFI** system, which allows unattended installation via ISO-embedded scripts.

**How it works**:

1. **Create PFI ISO**: Script + `pfi.conf` file
   ```bash
   ./scripts/vm/make-phase-iso.sh phase1-install.sh phase1.iso
   ```

2. **Boot with PFI ISO**: DragonFlyBSD installer detects `pfi.conf`
   ```
   pfi.conf contains: pfi_script=phase1-install.sh
   ```

3. **Automatic Execution**: Installer runs script, then powers off

**Architecture** (adapted from [golang/build](https://github.com/golang/build/tree/master/env/dragonfly-amd64)):

```
┌─────────────────────────────────────────────────────────┐
│ Host: Ubuntu 24.04                                       │
│                                                           │
│  make vm-auto-install                                    │
│         │                                                 │
│         ├──> Phase 1: OS Installation                    │
│         │    ├── make-phase-iso.sh → phase1.iso          │
│         │    ├── run-phase.sh 1 phase1.iso               │
│         │    │   ├── QEMU boots with:                    │
│         │    │   │   cd0: Installer ISO (boot)           │
│         │    │   │   cd1: phase1.iso (script)            │
│         │    │   │   cd2: Installer ISO (clean cpdup)    │
│         │    │   │   da0: Empty disk                     │
│         │    │   └── Installer auto-runs phase1-install.sh│
│         │    └── Result: Installed OS, powered off       │
│         │                                                 │
│         ├──> Phase 2: Package Updates                    │
│         │    ├── make-phase-iso.sh → phase2.iso          │
│         │    ├── run-phase.sh 2 phase2.iso               │
│         │    │   ├── QEMU boots with:                    │
│         │    │   │   Boot: Disk (installed OS)           │
│         │    │   │   cd0: phase2.iso (script)            │
│         │    │   └── Auto-runs phase2-update.sh          │
│         │    └── Result: Packages installed, powered off │
│         │                                                 │
│         ├──> Phase 3: Provisioning                       │
│         │    ├── make-phase-iso.sh → phase3.iso          │
│         │    ├── run-phase.sh 3 phase3.iso               │
│         │    │   ├── QEMU boots with:                    │
│         │    │   │   Boot: Disk (OS + packages)          │
│         │    │   │   cd0: phase3.iso (script)            │
│         │    │   └── Auto-runs phase3-provision.sh       │
│         │    └── Result: Configured, powered off         │
│         │                                                 │
│         └──> Create Snapshot                             │
│              cp dfly-vm.qcow2 dfly-vm-clean.qcow2        │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

**Phase Scripts**:

- **phase1-install.sh**: Disk partitioning, OS installation, boot config
- **phase2-update.sh**: `pkg update`, `pkg upgrade`, install Go/bash/git
- **phase3-provision.sh**: doas, directories, Go env, SSH keys

**Key Files**:

```
scripts/vm/
├── config.sh             # Centralized configuration (DFLY_VERSION, etc.)
├── make-phase-iso.sh     # Generic PFI ISO builder
├── phase1-install.sh     # OS installation script
├── phase2-update.sh      # Package update script
├── phase3-provision.sh   # Provisioning script
├── run-phase.sh          # QEMU boot helper for phases
└── auto-install.sh       # Orchestrator (runs all 3 phases)
```

**Benefits**:

1. **Zero Interaction**: No manual prompts or SSH required
2. **Reproducible**: Same result every time
3. **Fast**: 15 minutes vs 30+ minutes manual
4. **CI-Ready**: Can be automated in CI/CD pipelines
5. **Battle-Tested**: Go team uses this for their DragonFly builders

**Customization**:

Edit phase scripts to customize installation:

```bash
# Change Phase 3 to install additional packages
vim scripts/vm/phase3-provision.sh
# Add: pkg install -y vim tmux

# Re-run automation
make vm-destroy
make vm-setup
make vm-auto-install
```

#### Manual Phase Execution

Run individual phases for debugging:

```bash
# Phase 1 only
./scripts/vm/make-phase-iso.sh scripts/vm/phase1-install.sh /tmp/p1.iso
./scripts/vm/run-phase.sh 1 /tmp/p1.iso

# Phase 2 only (requires Phase 1 complete)
./scripts/vm/make-phase-iso.sh scripts/vm/phase2-update.sh /tmp/p2.iso
./scripts/vm/run-phase.sh 2 /tmp/p2.iso

# Phase 3 only (requires Phase 1+2 complete)
./scripts/vm/make-phase-iso.sh scripts/vm/phase3-provision.sh /tmp/p3.iso
./scripts/vm/run-phase.sh 3 /tmp/p3.iso
```

#### Version Overrides

Use different DragonFlyBSD versions:

```bash
# Use 6.6.0 instead of default 6.4.2
DFLY_VERSION=6.6.0 make vm-setup
DFLY_VERSION=6.6.0 make vm-auto-install
```

Or permanently change in `scripts/vm/config.sh`:

```bash
DFLY_VERSION="${DFLY_VERSION:-6.6.0}"
```

---

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

For `go-synth` specific issues, see `DEVELOPMENT.md`.
