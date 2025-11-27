# dsynth Quick Start Guide

This guide will help you get started with dsynth-go quickly.

## Prerequisites

- FreeBSD or DragonFly BSD (or compatible BSD system)
- Root access (required for chroot operations)
- Go 1.21 or later (for building from source)
- At least 50GB free disk space for building
- 8GB+ RAM recommended (more for tmpfs)

## Installation

### From Source

```bash
# Install Go if needed
sudo pkg install go

# Clone the repository
git clone https://github.com/tuxillo/go-synth.git
cd go-synth

# Build
go build -o dsynth

# Install
sudo install -m 0755 dsynth /usr/local/bin/
```

### Using build.sh

```bash
./build.sh
sudo install -m 0755 dsynth /usr/local/bin/
```

## First-Time Setup

### 1. Initialize Configuration

```bash
sudo dsynth init
```

This creates:
- `/etc/dsynth/dsynth.ini` - Configuration file
- `/build/` - Default build base directory
- `/build/packages/` - Package repository
- `/build/distfiles/` - Downloaded source files
- `/build/logs/` - Build logs

### 2. Configure Settings

Edit the configuration file:

```bash
sudo vi /etc/dsynth/dsynth.ini
```

Important settings to adjust:

```ini
# Set based on your CPU cores (e.g., 8 cores = 4 builders)
Number_of_builders=4

# Set based on per-port parallelism needs
Max_jobs=4

# Adjust tmpfs sizes based on available RAM
# Rule of thumb: worksize = 8GB per builder
Tmpfs_worksize=32g
Tmpfs_localbasesize=8g
```

### 3. Ensure Ports Tree is Available

```bash
# For FreeBSD
sudo portsnap fetch extract
# or
sudo git clone https://git.FreeBSD.org/ports.git /usr/ports

# For DragonFly BSD
cd /usr
sudo make dports-create
```

## Basic Usage

### Building a Single Package

```bash
# Build vim and all its dependencies
sudo dsynth build editors/vim
```

This will:
1. Parse port Makefile
2. Resolve all dependencies
3. Check which packages need building (using BuildDB with CRC32 content-based detection)
4. Build packages in parallel
5. Save packages to repository and update BuildDB

### Building Multiple Packages

```bash
sudo dsynth build editors/vim shells/bash devel/git
```

### Building with Flavors

Some ports have multiple flavors:

```bash
# Build Python 3.9 flavor
sudo dsynth build lang/python@py39

# Build Python 3.11 flavor
sudo dsynth build lang/python@py311
```

### Force Rebuild

Ignore BuildDB cache and rebuild even if unchanged:

```bash
sudo dsynth force editors/vim
```

### Fetch Only (No Build)

Download distfiles without building:

```bash
sudo dsynth fetch-only editors/vim
```

### Build All Installed Packages

Update all packages currently installed on your system:

```bash
sudo dsynth upgrade-system
```

This queries `pkg` to get list of installed packages and builds them all.

## Monitoring Builds

### Real-time Progress

During build, dsynth shows progress:

```
[15:45:32] Progress: 45/100 (S:42 F:3) 25m30s elapsed
```

- S: Successful builds
- F: Failed builds

### Viewing Logs

View summary logs:

```bash
# Overall results
sudo dsynth logs results

# Successful builds
sudo dsynth logs success

# Failed builds
sudo dsynth logs failure
```

View per-package logs:

```bash
# View detailed log for a specific port
sudo dsynth logs editors/vim
```

Log files are stored in `/build/logs/`:
- `00_last_results.log` - Overall results
- `01_success_list.log` - Successful builds
- `02_failure_list.log` - Failed builds
- `logs/category/portname.log` - Per-package details

## Using Built Packages

### Configure pkg to Use Local Repository

Create `/usr/local/etc/pkg/repos/local.conf`:

```
local: {
    url: "file:///build/packages",
    enabled: yes
}

FreeBSD: {
    enabled: no
}
```

### Install Packages

```bash
# Update repository catalog
sudo pkg repo /build/packages

# Install from local repository
sudo pkg install vim
```

## Troubleshooting

### Build Fails with Mount Errors

```bash
# Ensure you're running as root
sudo -i

# Check mount points
mount | grep /build

# Clean up stuck mounts
sudo dsynth cleanup
```

### Out of Disk Space

```bash
# Check disk usage
df -h /build

# Clean up old builds
sudo dsynth cleanup

# Reduce tmpfs sizes in config
sudo vi /etc/dsynth/dsynth.ini
```

### Dependency Resolution Issues

```bash
# Reset BuildDB database
sudo dsynth reset-db

# Verify ports tree is up to date
cd /usr/ports
sudo git pull
```

### Package Not Found

```bash
# Verify port exists
ls /usr/ports/editors/vim

# Check port origin is correct
cd /usr/ports/editors/vim
make -V PKGORIGIN
```

## Performance Tuning

### Optimal Worker Count

Rule of thumb: `Number_of_builders = CPU_cores / 2`

```
 4 cores = 2 builders
 8 cores = 4 builders
16 cores = 8 builders
32 cores = 16 builders
```

### Memory Requirements

Minimum RAM needed:
- Base: 2GB
- Per builder with tmpfs: 8-10GB
- Example: 4 builders = 2GB + (4 × 10GB) = 42GB

Without tmpfs: 4GB total minimum

### Disk Space

Typical requirements:
- /build/distfiles: 20-50GB
- /build/packages: 10-100GB (varies)
- /build work dirs: 10GB per builder (if not tmpfs)

### Using ccache

Enable ccache for faster rebuilds:

```ini
Use_ccache=yes
Ccache_dir=/build/ccache
```

Install ccache:
```bash
sudo pkg install ccache
```

## Common Workflows

### Daily Port Rebuilds

```bash
#!/bin/sh
# rebuild-ports.sh

cd /usr/ports
git pull

dsynth upgrade-system
dsynth logs failure
```

### Building for Production

```bash
# Build with verification
sudo dsynth build -P editors/vim

# Verify packages
sudo dsynth verify

# Rebuild repository metadata
sudo pkg repo /build/packages
```

### Testing Port Changes

```bash
# Edit port
sudo vi /usr/ports/editors/vim/Makefile

# Force rebuild to test
sudo dsynth force editors/vim

# Check logs
sudo dsynth logs editors/vim
```

## Next Steps

1. Read the full README.md for detailed information
2. Explore all configuration options
3. Set up automated builds with cron
4. Configure a pkg repository server
5. Join the community for support

## Getting Help

- View all commands: `dsynth help`
- Check logs: `dsynth logs`
- Report issues: GitHub Issues
- Documentation: README.md

## Tips

1. **Start small**: Build a few packages first to verify setup
2. **Monitor resources**: Watch CPU, RAM, and disk usage
3. **Use tmpfs**: Much faster if you have RAM
4. **Keep ports updated**: Regular `git pull` in ports tree
5. **Clean regularly**: Run `dsynth cleanup` after builds
6. **Check logs**: Always review failure logs
7. **Backup packages**: Your built packages are valuable
8. **Test before production**: Use a separate build server

## Example Build Session

```bash
# Initial setup
sudo dsynth init
sudo vi /etc/dsynth/dsynth.ini

# Update ports tree
cd /usr/ports && sudo git pull

# Build some packages
sudo dsynth build editors/vim shells/bash devel/git

# Check results
sudo dsynth logs results
sudo dsynth logs success

# If failures, check what went wrong
sudo dsynth logs failure
sudo dsynth logs editors/vim

# Install locally
sudo pkg repo /build/packages
sudo pkg install vim bash git

# Clean up
sudo dsynth cleanup
```

Happy building!
