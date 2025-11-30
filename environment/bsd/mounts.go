// Package bsd implements the Environment interface for FreeBSD/DragonFly BSD
// using chroot isolation with nullfs, tmpfs, devfs, and procfs mounts.
//
// This package extracts mount operations from the original mount package,
// refactoring them into methods on BSDEnvironment for better encapsulation
// and testability.
package bsd

import (
	"dsynth/config"
	"fmt"
	stdlog "log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// BSDEnvironment implements Environment interface using chroot with nullfs/tmpfs.
//
// This backend provides build isolation for FreeBSD and DragonFlyBSD systems.
// It creates an ephemeral filesystem hierarchy using tmpfs overlays and
// nullfs bind mounts, then executes commands via chroot.
//
// Thread safety: BSDEnvironment is safe for concurrent Execute() calls after
// Setup() completes. Setup() and Cleanup() must not be called concurrently.
//
// Fields:
//   - baseDir: Base directory (e.g., /build/SL01)
//   - cfg: Configuration reference
//   - mounts: Tracked mounts for cleanup
//   - mountErrors: Count of mount errors (for compatibility)
//   - workerID: Worker ID for logging/debugging
//   - activePIDs: PIDs of spawned processes (for cleanup)
//   - pidMu: Mutex protecting activePIDs access
type BSDEnvironment struct {
	baseDir     string
	cfg         *config.Config
	mounts      []mountState
	mountErrors int
	workerID    int
	activePIDs  []int      // Track spawned process PIDs for cleanup
	pidMu       sync.Mutex // Protect activePIDs from concurrent access
}

// Mount type flags use a bitmask design inherited from C dsynth.
//
// The low nibble (0x000F) specifies the filesystem type:
//   - 0x0001: tmpfs (temporary filesystem, memory-backed)
//   - 0x0002: nullfs (BSD null mount, similar to Linux bind mount)
//   - 0x0003: devfs (device filesystem)
//   - 0x0004: procfs (process filesystem)
//
// The high bits specify mount options:
//   - 0x0010: Read-write (default is read-only)
//   - 0x0020: Big tmpfs (64GB size limit)
//   - 0x0080: Medium tmpfs (16GB size limit)
//
// Example: TmpfsRWBig = 0x0001 | 0x0010 | 0x0020 = 0x0031
const (
	MountTypeMask   = 0x000F // Mask to extract filesystem type
	MountTypeTmpfs  = 0x0001 // tmpfs filesystem
	MountTypeNullfs = 0x0002 // nullfs (BSD null mount)
	MountTypeDevfs  = 0x0003 // devfs (device filesystem)
	MountTypeProcfs = 0x0004 // procfs (process filesystem)
	MountTypeRW     = 0x0010 // Read-write flag
	MountTypeBig    = 0x0020 // Big tmpfs (64GB)
	MountTypeMed    = 0x0080 // Medium tmpfs (16GB)
)

// Common mount type combinations used throughout worker setup.
const (
	TmpfsRW    = MountTypeTmpfs | MountTypeRW                // Read-write tmpfs (16GB default)
	TmpfsRWBig = MountTypeTmpfs | MountTypeRW | MountTypeBig // Large tmpfs (64GB)
	TmpfsRWMed = MountTypeTmpfs | MountTypeRW | MountTypeMed // Medium tmpfs (16GB)
	NullfsRO   = MountTypeNullfs                             // Read-only nullfs
	NullfsRW   = MountTypeNullfs | MountTypeRW               // Read-write nullfs
	DevfsRW    = MountTypeDevfs | MountTypeRW                // Read-write devfs
	ProcfsRO   = MountTypeProcfs                             // Read-only procfs
)

// mountState tracks a single mounted filesystem for cleanup and debugging.
//
// The BSDEnvironment maintains a slice of mountState entries to:
//   - Verify all mounts succeeded during setup
//   - Enable smart cleanup (unmount in reverse order)
//   - Provide debugging information for mount issues
type mountState struct {
	target string // Absolute path where filesystem is mounted
	fstype string // Filesystem type (tmpfs, null, devfs, procfs)
	source string // Source path (for nullfs) or filesystem name
}

// MountError represents a filesystem mount or unmount error.
//
// MountError provides structured error information including:
//   - The operation that failed (mount, unmount, mkdir)
//   - The target path
//   - Filesystem type and source (for mount operations)
//   - The underlying error
//
// This enables proper error handling and debugging compared to
// the original implementation which only printed to stderr.
type MountError struct {
	Op     string // Operation: "mount", "unmount", "mkdir"
	Path   string // Target path (absolute)
	FSType string // Filesystem type (optional, for mount)
	Source string // Source path (optional, for mount)
	Err    error  // Underlying error
}

func (e *MountError) Error() string {
	if e.FSType != "" {
		return fmt.Sprintf("%s failed for %s (type=%s, source=%s): %v",
			e.Op, e.Path, e.FSType, e.Source, e.Err)
	}
	return fmt.Sprintf("%s failed for %s: %v", e.Op, e.Path, e.Err)
}

func (e *MountError) Unwrap() error {
	return e.Err
}

// doMount performs a single filesystem mount operation.
//
// Parameters:
//   - mountType: Mount type flags (see constants above)
//   - spath: Source path
//   - "dummy": Placeholder for tmpfs/devfs/procfs (no real source)
//   - "$/" prefix: System path (e.g., "$/bin" → "/bin" or SystemPath+"/bin")
//   - Otherwise: Absolute path (e.g., cfg.DPortsPath)
//   - dpath: Target path relative to environment base directory
//
// The method:
//  1. Resolves source path based on spath prefix and cfg.SystemPath
//  2. Creates target directory (basedir + dpath)
//  3. Determines mount options (ro/rw, tmpfs size)
//  4. Executes mount command via exec.Command
//  5. Tracks successful mount in e.mounts slice
//
// Returns MountError if any step fails.
//
// Example calls:
//   - e.doMount(TmpfsRW, "dummy", "") → mount tmpfs on base directory
//   - e.doMount(NullfsRO, "$/bin", "/bin") → mount system /bin read-only
//   - e.doMount(NullfsRW, cfg.DPortsPath, "/xports") → mount ports tree
//
// Extracted from mount/mount.go doMount function (lines 189-278).
func (e *BSDEnvironment) doMount(mountType int, spath, dpath string) error {
	// Resolve source path
	var source string
	if spath == "dummy" {
		source = "tmpfs"
	} else if strings.HasPrefix(spath, "$") {
		// System path: $/ prefix means relative to SystemPath
		sysPath := e.cfg.SystemPath
		if sysPath == "/" {
			source = spath[1:] // Remove $ prefix
		} else {
			source = filepath.Join(sysPath, spath[1:])
		}
	} else {
		source = spath
	}

	// Resolve target path (basedir + relative dpath)
	target := filepath.Join(e.baseDir, dpath)

	// Create target directory
	if err := os.MkdirAll(target, 0755); err != nil {
		return &MountError{
			Op:   "mkdir",
			Path: target,
			Err:  err,
		}
	}

	// Verify directory exists (defensive check)
	if stat, err := os.Stat(target); err != nil {
		return &MountError{
			Op:   "mkdir",
			Path: target,
			Err:  fmt.Errorf("target directory does not exist after mkdir: %w", err),
		}
	} else if !stat.IsDir() {
		return &MountError{
			Op:   "mkdir",
			Path: target,
			Err:  fmt.Errorf("target is not a directory"),
		}
	}

	// Determine mount options based on type flags
	rwOpt := "ro"
	if mountType&MountTypeRW != 0 {
		rwOpt = "rw"
	}

	var fstype string
	var opts []string

	switch mountType & MountTypeMask {
	case MountTypeTmpfs:
		fstype = "tmpfs"
		opts = []string{rwOpt}
		// Tmpfs size based on flags
		if mountType&MountTypeBig != 0 {
			opts = append(opts, "size=64g")
		} else if mountType&MountTypeMed != 0 {
			opts = append(opts, "size=16g")
		} else {
			opts = append(opts, "size=16g") // Default size
		}

	case MountTypeNullfs:
		fstype = "null"
		opts = []string{rwOpt}

	case MountTypeDevfs:
		fstype = "devfs"
		opts = []string{rwOpt}

	case MountTypeProcfs:
		fstype = "procfs"
		opts = []string{rwOpt}

	default:
		return &MountError{
			Op:   "mount",
			Path: target,
			Err:  fmt.Errorf("unknown mount type: 0x%x", mountType),
		}
	}

	// Execute mount command
	optStr := strings.Join(opts, ",")
	cmd := exec.Command("mount", "-t", fstype, "-o", optStr, source, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &MountError{
			Op:     "mount",
			Path:   target,
			FSType: fstype,
			Source: source,
			Err:    fmt.Errorf("mount command failed: %w: %s", err, string(output)),
		}
	}

	// Track mounted filesystem for cleanup
	e.mounts = append(e.mounts, mountState{
		target: target,
		fstype: fstype,
		source: source,
	})

	return nil
}

// doUnmount unmounts a single filesystem.
//
// Parameters:
//   - rpath: Target path relative to environment base directory
//
// The method resolves the absolute target path (basedir + rpath) and
// calls unix.Unmount to unmount the filesystem.
//
// Expected errors (ignored):
//   - unix.EPERM: Permission denied (expected if not mounted)
//   - unix.ENOENT: Path doesn't exist (expected after unmount)
//   - unix.EINVAL: Invalid argument (expected if not a mount point)
//
// Returns MountError only for unexpected unmount failures.
//
// Example calls:
//   - e.doUnmount("/proc") → unmount /proc inside environment
//   - e.doUnmount("") → unmount root tmpfs
//
// Extracted from mount/mount.go doUnmount function (lines 280-292).
func (e *BSDEnvironment) doUnmount(rpath string) error {
	target := filepath.Join(e.baseDir, rpath)

	if err := unix.Unmount(target, 0); err != nil {
		switch err {
		case unix.EPERM, unix.ENOENT, unix.EINVAL:
			// Expected errors - filesystem not mounted or already unmounted
			return nil
		default:
			return &MountError{
				Op:   "unmount",
				Path: target,
				Err:  err,
			}
		}
	}

	return nil
}

// killActiveProcesses forcefully terminates all tracked child processes.
//
// This method is called during cleanup to ensure no processes remain
// that could prevent filesystem unmounting. It uses a two-phase approach:
//
//  1. SIGTERM phase: Send SIGTERM to all process groups, wait 2 seconds
//  2. SIGKILL phase: Send SIGKILL to any remaining process groups
//
// Process groups are killed using negative PIDs (-pid), which sends the
// signal to the entire process group. This ensures child processes (make,
// compiler, linker) are all terminated together.
//
// The method is thread-safe and can be called concurrently with Execute().
//
// Signal behavior:
//   - SIGTERM (15): Polite termination request, allows cleanup
//   - SIGKILL (9): Forceful termination, cannot be caught
//   - Negative PID: Kills entire process group (parent + children)
//
// Example:
//
//	e.killActiveProcesses()
//	// All tracked processes and their children are now terminated
func (e *BSDEnvironment) killActiveProcesses() {
	e.pidMu.Lock()
	pids := make([]int, len(e.activePIDs))
	copy(pids, e.activePIDs)
	e.pidMu.Unlock()

	if len(pids) == 0 {
		return
	}

	stdlog.Printf("[Cleanup] Killing %d active process(es) before unmount", len(pids))

	// Phase 1: Send SIGTERM to all process groups
	for _, pid := range pids {
		// Kill process group (negative PID)
		// This sends signal to the entire process tree
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
			stdlog.Printf("[Cleanup] WARNING: Failed to SIGTERM process group %d: %v", pid, err)
		} else {
			stdlog.Printf("[Cleanup] Sent SIGTERM to process group %d", pid)
		}
	}

	// Wait for processes to terminate gracefully
	time.Sleep(2 * time.Second)

	// Phase 2: Send SIGKILL to any remaining process groups
	for _, pid := range pids {
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			// ESRCH (no such process) is expected if process already exited
			if err != syscall.ESRCH {
				stdlog.Printf("[Cleanup] WARNING: Failed to SIGKILL process group %d: %v", pid, err)
			}
		} else {
			stdlog.Printf("[Cleanup] Sent SIGKILL to process group %d", pid)
		}
	}

	// Clear the PID list (all processes should be dead now)
	e.pidMu.Lock()
	e.activePIDs = nil
	e.pidMu.Unlock()
}
