# AGENTS.md - Development Guide for go-synth

This document provides essential information for agents and developers working with the go-synth codebase.

## ⚠️ CRITICAL SAFETY DIRECTIVES

### 1. GitHub Account Restrictions
**IMPORTANT**: All GitHub write operations (creating branches, PRs, issues, comments, etc.) must ONLY be performed on the `tuxillo` account.

- ✅ **Allowed**: Read operations on any repository
- ✅ **Allowed**: Write operations on `github.com/tuxillo/*` repositories
- ❌ **FORBIDDEN**: Write operations on any other GitHub accounts or organizations
- ❌ **FORBIDDEN**: Creating/modifying repositories outside `tuxillo` account

**Examples:**
```bash
# ✅ ALLOWED - Read from any repo
gh repo view someuser/somerepo

# ✅ ALLOWED - Write to tuxillo repos
gh pr create --repo tuxillo/go-synth

# ❌ FORBIDDEN - Write to other accounts
gh pr create --repo otheruser/somerepo
```

### 2. System Safety Restrictions
**IMPORTANT**: Do NOT execute commands that could potentially harm the host system or its installed operating system.

**Forbidden Operations:**
- ❌ Package manager operations that modify system packages (`apt install`, `pkg install`, `yum install`, etc.)
- ❌ System service modifications (`systemctl`, `service`, daemon operations)
- ❌ Kernel or bootloader modifications
- ❌ System-wide configuration changes outside the project directory
- ❌ Disk partitioning or formatting operations
- ❌ System user/group modifications
- ❌ Network configuration changes
- ❌ Firewall rule modifications
- ❌ Recursive deletions outside project directory (especially `rm -rf /`)
- ❌ chmod/chown on system directories

**Allowed Operations:**
- ✅ Building and testing within the project directory (`/home/antonioh/s/go-synth`)
- ✅ Reading system information (`uname`, `ps`, `df`, `mount` with no arguments)
- ✅ Go toolchain commands (`go build`, `go test`, `go mod`, etc.)
- ✅ Git operations within the project
- ✅ File operations within the project directory
- ✅ Project-specific make targets

**When in doubt, ASK before executing any system-level command.**

## Project Overview

**go-synth** is a Go implementation of dsynth, the DragonFly BSD ports build system. It's a parallel package building tool that:

- Builds FreeBSD/DragonFly BSD ports packages efficiently using parallel workers
- Uses chroot isolation for each build environment
- Implements CRC-based change detection for incremental builds
- Resolves dependencies automatically using topological sorting
- Provides comprehensive logging and progress tracking

### Project Location
**Repository Path**: `$HOME/s/go-synth` (typically `/home/antonioh/s/go-synth`)

### Essential Documentation Structure
- **`DEVELOPMENT.md`** (root) - **PRIMARY DEVELOPMENT GUIDE**: Phase tracking, task status, exit criteria, contribution workflow
- **`docs/design/`** - **PHASE DESIGN DOCUMENTS**: Detailed specifications for each phase
  - `PHASE_1_LIBRARY.md` - Library extraction (pkg package)
  - `PHASE_2_BUILDDB.md` - Build database with bbolt
  - `PHASE_3_BUILDER.md` - Builder orchestration
  - `PHASE_4_ENVIRONMENT.md` - Environment abstraction
  - `PHASE_5_MIN_API.md` - Minimal REST API
  - `PHASE_6_TESTING.md` - Testing strategy
  - `PHASE_7_INTEGRATION.md` - Integration & migration
  - Plus planning docs: `IDEAS.md`, `IDEAS_MVP.md`, `FUTURE_BACKLOG.md`

## Architecture Summary

### Core Components

| Package | Purpose | Key Files |
|---------|---------|-----------|
| `main.go` | CLI entry point, command parsing, orchestration | `main.go` |
| `config/` | INI-based configuration management | `config/config.go` |
| `pkg/` | Package metadata, dependency resolution | `pkg/*.go` |
| `builddb/` | Embedded bbolt database for build tracking and CRC-based incremental builds | `builddb/db.go` (562 lines) |
| `build/` | Build orchestration with worker pool management | `build/*.go` |
| `environment/` | ✅ Build isolation abstraction (Setup, Execute, Cleanup) with BSD backend - **Phase 4 Complete** | `environment/*.go`, `environment/bsd/*.go` |
| `mount/` | ⚠️ Deprecated - Filesystem operations (use `environment/` instead) | `mount/mount.go` |
| `log/` | Multi-file logging system (8 different log types) | `log/*.go` |
| `util/` | Helper utilities and common functions | `util/util.go` |
| `cmd/` | Additional command implementations | `cmd/build.go` |

### Key Data Structures

- **`pkg.Package`** - Represents a port/package with metadata, dependencies, and build status
- **`build.Worker`** - Represents a build worker with environment context
- **`build.BuildContext`** - Orchestrates the entire build process
- **`config.Config`** - Holds all configuration settings
- **`environment.Environment`** - Interface for build isolation (BSD, mock, future backends)
- **`environment.ExecCommand`** - Command execution specification with context support
- **`environment.ExecResult`** - Command execution result with exit code and duration

## Development Commands

### Contribution Workflow (Local Agents)

#### Mandatory Documentation Updates
**CRITICAL**: NO CODE CHANGES WITHOUT DOCUMENTATION UPDATES. Every commit that changes functionality MUST update all relevant documentation in the same commit.

**Required Documentation Updates for Every Commit:**

1. **DEVELOPMENT.md** (ALWAYS check this first)
   - Update task status (pending → in_progress → completed)
   - Update phase progress percentages
   - Update exit criteria status
   - Add commit hashes to completed tasks
   - Update "Current Status" and "Recent Milestones" sections

2. **Phase Design Document** (`docs/design/PHASE_N_*.md`)
   - Update task status in the relevant phase document
   - Mark deliverables as complete with commit references
   - Update implementation notes and decisions
   - Document any deviations from the plan

3. **README.md** (if user-facing changes)
   - Update features list
   - Update API examples
   - Update architecture diagrams or descriptions
   - Add/update usage examples

4. **AGENTS.md** (if architecture/workflow changes)
   - Update component descriptions
   - Update implementation status
   - Update code patterns and workflows

**The Rule**: If you wrote code → you MUST update docs. No exceptions. Documentation is not optional.

#### Commit Requirements
- Each completed step (e.g. API change, test addition, doc update) must be committed locally.
- **CRITICAL**: Every commit MUST include Co-authored-by trailer for the AI model used.
  - Format: `Co-authored-by: <Model Name> <ai-model@example.com>`
  - Example: `Co-authored-by: Claude 3.7 Sonnet <claude-3.7-sonnet@anthropic.com>`
  - Example: `Co-authored-by: GPT-4 <gpt-4@openai.com>`
  - The commit author is already set to Antonio Huete Jimenez with proper email
  - This properly attributes AI assistance in the git history
  - Use standard git trailer format (blank line before trailers)
- Do NOT push to any remote during iterative Phase work unless explicitly requested.
- Group related minimal changes per commit; avoid large mixed commits.
- Document rationale briefly in commit message (focus on why).
- Keep repository history clean for later Cobra/architecture migration.

### Building

```bash
# Using Makefile (recommended)
make build
make install    # Install to /usr/local/bin/

# Using build script
./build.sh

# Direct go build
go build -ldflags "-X main.Version=2.0.0" -o dsynth .
```

### Testing & Validation

```bash
# Run tests
make test
go test -v ./...

# Code quality checks
make fmt    # Format code
make vet    # Run go vet
go vet ./...
go fmt ./...
```

### Running

```bash
# Initialize configuration
sudo ./dsynth init

# Build packages
sudo ./dsynth build editors/vim
sudo ./dsynth build editors/vim shells/bash devel/git

# View logs
./dsynth logs editors/vim
./dsynth logs results
```

## Key Files & Directories

### Essential Files
- **`main.go`** - CLI interface and command routing (464 lines)
- **`go.mod`** - Go module definition and dependencies
- **`Makefile`** - Build system with standard targets
- **`build.sh`** - Alternative build script

### Configuration Files
- **`config/config.go`** - Configuration parsing and management
- **`/etc/dsynth/dsynth.ini`** - Runtime configuration (created by `init`)

### Documentation (Root Directory)
- **`DEVELOPMENT.md`** - **PRIMARY**: Phase tracking, task status, contribution workflow (646 lines)
- **`README.md`** - User guide with features, configuration, examples (411 lines)
- **`AGENTS.md`** - This file: Development guide for agents and developers (405 lines)
- **`QUICKSTART.md`** - Practical getting-started guide (408 lines)

### Design Documentation (docs/design/ Directory)
- **`IDEAS.md`** - Comprehensive architectural vision (729 lines)
- **`IDEAS_MVP.md`** - Minimum viable product scope
- **`PHASE_1_LIBRARY.md`** - Phase 1 specification and status
- **`PHASE_2_BUILDDB.md`** - Phase 2 specification and status
- **`PHASE_3_BUILDER.md` through `PHASE_7_INTEGRATION.md`** - Future phase specs
- **`FUTURE_BACKLOG.md`** - Deferred features

### Build Artifacts (Runtime)
- **`dsynth`** - Compiled binary
- **`/build/`** - Build base directory (configurable)
- **`*.db`** - CRC database files
- **`logs/`** - Build logs and results

## Configuration System

### Configuration Loading
Configuration is loaded from INI files with profile support:
- Default location: `/etc/dsynth/dsynth.ini`
- Override with `-C` flag
- Profile selection with `-p` flag

### Key Configuration Sections
```ini
[Global Configuration]
Number_of_builders=8          # Parallel worker count
Max_jobs=8                    # Make parallelism per worker
Directory_packages=/build/packages
Directory_buildbase=/build
Directory_portsdir=/usr/ports
Use_tmpfs=yes                 # Use tmpfs for speed
Tmpfs_worksize=64g           # Work directory size
```

### Important Config Fields
- `BuildBase`, `PackagesPath`, `RepositoryPath` - Directory paths
- `MaxWorkers`, `MaxJobs` - Concurrency settings
- `UseTmpfs`, `UseCCache` - Performance options
- `Debug`, `Force`, `YesAll` - Runtime behavior flags

## Implementation Status

### ✅ Implemented Features
- CLI interface with comprehensive command set
- Configuration parsing and management
- Package parsing and metadata extraction
- Dependency resolution with topological sort
- **BuildDB**: Embedded bbolt database with build tracking and content-based incremental builds
  - Build history with UUIDs and status tracking (running/success/failed)
  - CRC32-based change detection for automatic incremental builds
  - Package version indexing (latest successful build per port@version)
  - Crash-safe ACID transactions with automatic recovery
  - Single-open lifecycle pattern for thread safety
- **Environment Abstraction**: Clean isolation layer for build execution - **Phase 4 Complete ✅**
  - Interface-based design supporting multiple backends (BSD, mock, future: jails, containers)
  - BSD backend with 27-mount chroot environment (nullfs, tmpfs, devfs, procfs)
  - Context support for cancellation and timeout
  - Structured error types for debugging
  - Comprehensive test suite: 38 unit tests (91.6% coverage) + 8 integration tests (VM)
  - Critical bug fixed: Context timeout handling in Execute()
- Build orchestration with worker pools
- Multi-file logging system
- Signal handling and cleanup

### ⚠️ Partially Implemented
- Build phase execution (framework exists, some phases incomplete)
- Package installation and repository management
- Error handling and recovery

### ❌ Not Yet Implemented (TODO items)
- `status` command implementation
- `cleanup` command (stale mount/log cleanup)
- `configure` command (interactive configuration)
- `fetch-only` mode
- Package verification
- Repository rebuilding
- ncurses UI (disabled for now)

## Dependencies & Requirements

### Go Dependencies
```go
// go.mod
require (
    github.com/spf13/cobra v1.10.1    // CLI framework (imported but not used)
    golang.org/x/sys v0.15.0          // System calls
    gopkg.in/ini.v1 v1.67.0           // INI file parsing
)
```

### System Requirements
- Go 1.21 or later
- FreeBSD or DragonFly BSD (or compatible BSD)
- Root privileges (for chroot and mounts)
- pkg-static binary
- 50GB+ disk space for builds
- 8GB+ RAM recommended

### External Commands Used
- `make` - Port building
- `pkg` - Package management
- `mount`/`umount` - Filesystem operations
- `chroot` - Build isolation

## Common Workflows

### Adding New Commands
1. Add command case to `main.go` switch statement
2. Implement `doCommandName()` function
3. Add command to `usage()` function
4. Test with `./dsynth command-name`

### Modifying Build Process
1. Update `build/build.go` for orchestration changes
2. Modify `build/phases.go` for phase execution
3. Update `pkg/pkg.go` for package-related changes
4. Test with small port builds first

### Configuration Changes
1. Update `config/config.go` struct definition
2. Modify INI parsing logic
3. Update documentation in README.md
4. Test with `./dsynth init`

### Debugging Build Issues
1. Enable debug mode: `./dsynth -d build portname`
2. Check logs: `./dsynth logs portname`
3. Examine CRC database: `ls -la /build/dsynth.db`
4. Verify mounts: `mount | grep /build`

## Key Code Patterns

### Error Handling Pattern
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "Error message: %v\n", err)
    os.Exit(1)
}
```

### Configuration Access Pattern
```go
cfg, err := config.LoadConfig(*configDir, *profile)
if err != nil {
    // handle error
}
```

### Package Processing Pattern
```go
head, err := pkg.ParsePortList(portList, cfg)
if err != nil {
    // handle error
}

err = pkg.ResolveDependencies(head, cfg)
if err != nil {
    // handle error
}
```

### Logging Pattern
```go
logger, err := log.NewLogger(cfg)
if err != nil {
    // handle error
}
defer logger.Close()
```

## Testing Strategy

### Unit Tests
- Test individual package functions
- Mock external dependencies
- Focus on core logic (dependency resolution, CRC calculation)

### Integration Tests
- Test complete build workflows
- Use small, fast-building ports
- Verify package output and logs

### System Tests
- Test on actual BSD systems
- Use real ports tree
- Test with various configurations

## Performance Considerations

### Build Performance
- Worker count should be `CPU cores / 2`
- Use tmpfs for work directories if RAM available
- Enable ccache for faster rebuilds

### Memory Usage
- Each worker needs ~8-10GB with tmpfs
- Dependency graphs can be large for full ports tree
- CRC database grows with number of ports

### Disk I/O
- Package building is I/O intensive
- Use SSD storage for better performance
- Monitor `/build` directory usage

## Debugging Tips

### Common Issues
1. **Permission denied** - Run as root for chroot operations
2. **Mount errors** - Check kernel support for nullfs/tmpfs
3. **Package not found** - Verify ports tree exists and is up to date
4. **Out of space** - Increase tmpfs sizes or disable tmpfs

### Debug Commands
```bash
# Check configuration
./dsynth -d init  # Debug mode

# Reset state
sudo ./dsynth reset-db
sudo ./dsynth cleanup

# Verify ports tree
ls /usr/ports/editors/vim
make -C /usr/ports/editors/vim -V PKGORIGIN
```

### Log Analysis
- Check `/build/logs/00_last_results.log` for overall results
- Look at `/build/logs/02_failure_list.log` for failed builds
- Examine `/build/logs/logs/category/portname.log` for detailed errors

## Future Development Plans

See `IDEAS.md` for comprehensive architectural plans:
- Library-first design with reusable components
- REST API + WebSocket for web UI integration
- Advanced build tracking with bbolt database backend
- Multi-platform support (FreeBSD jails, Linux containers)
- Distributed builds across multiple servers

## Getting Help

### Documentation Reading Order for New Contributors
1. **`DEVELOPMENT.md`** - Start here! Understand phases, current status, and workflow
2. **`docs/design/PHASE_N_*.md`** - Read the current phase document for detailed tasks
3. **`AGENTS.md`** - This file: Development patterns and workflows
4. **`README.md`** - User-facing features and usage
5. **`QUICKSTART.md`** - Practical examples
6. **`docs/design/IDEAS.md`** - Deep dive into architectural vision

### Quick References
- Current phase status: Check **`DEVELOPMENT.md`** Phase Status section
- Task list: Check **`DEVELOPMENT.md`** and relevant phase document in `docs/design/`
- Code patterns: This file (AGENTS.md) "Common Workflows" section
- Architectural decisions: `docs/design/PHASE_N_*.md` "Key Decisions" sections
- Debug mode: Use `-d` flag for troubleshooting
- Build logs: `/build/logs/` directory

---

**Last Updated**: Based on codebase analysis  
**Project Version**: 2.0.0-dev  
**Go Version**: 1.21+