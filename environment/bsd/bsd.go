// Package bsd implements the Environment interface for FreeBSD and DragonFlyBSD
// using chroot isolation with nullfs/tmpfs mounts.
//
// This implementation mirrors the mount strategy from the original C dsynth:
//   - Root tmpfs for ephemeral build state
//   - Read-only nullfs mounts for system directories
//   - Read-write tmpfs/nullfs for work areas
//   - devfs and procfs for device/process access
//
// Mount layout (27 mount points total):
//
//	/           tmpfs (rw)                # Root overlay
//	/boot       tmpfs (rw)                # Boot files
//	/dev        devfs (rw)                # Device files
//	/proc       procfs (ro)               # Process info
//	/bin        nullfs → $/bin (ro)       # System binaries
//	/sbin       nullfs → $/sbin (ro)      # System admin binaries
//	/lib        nullfs → $/lib (ro)       # System libraries
//	/libexec    nullfs → $/libexec (ro)   # System utilities
//	/usr/bin    nullfs → $/usr/bin (ro)   # User binaries
//	/usr/include nullfs → $/usr/include (ro) # Headers
//	/usr/lib    nullfs → $/usr/lib (ro)   # User libraries
//	/usr/libdata nullfs → $/usr/libdata (ro) # Library data
//	/usr/libexec nullfs → $/usr/libexec (ro) # User utilities
//	/usr/sbin   nullfs → $/usr/sbin (ro)  # User admin binaries
//	/usr/share  nullfs → $/usr/share (ro) # Shared data
//	/usr/games  nullfs → $/usr/games (ro) # Games
//	/usr/src    nullfs → $/usr/src (ro)   # Source (optional)
//	/xports     nullfs → DPortsPath (ro)  # Ports tree
//	/options    nullfs → OptionsPath (rw) # Port options
//	/packages   nullfs → PackagesPath (rw) # Built packages
//	/distfiles  nullfs → DistFilesPath (rw) # Source tarballs
//	/construction tmpfs (rw, 64GB)        # Build workspace
//	/usr/local  tmpfs (rw, 16GB)          # Installed files
//	/ccache     nullfs → CCachePath (rw)  # Compiler cache (optional)
//
// Note: $/ prefix indicates SystemPath substitution (typically "/" or
// custom system root).
//
// Design decisions:
//   - Follow existing dsynth mount layout exactly (proven in production)
//   - Track all mounts for verification and cleanup
//   - Return errors instead of stderr + counters (structured errors)
//   - Fail-safe: log mount errors but continue (compatibility with dsynth C)
//   - Copy template after mounts (provides shell, utilities, etc.)
package bsd

import (
	"context"
	"dsynth/config"
	"dsynth/environment"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// NewBSDEnvironment creates a new BSD environment instance.
//
// This constructor is registered with the environment package to handle
// the "bsd" backend type.
func NewBSDEnvironment() environment.Environment {
	return &BSDEnvironment{
		mounts: make([]mountState, 0, 30), // Pre-allocate for ~27 mounts
	}
}

func init() {
	// Register this backend with the environment package
	environment.Register("bsd", NewBSDEnvironment)
}

// Setup prepares the build environment for a worker.
//
// This method:
//  1. Creates the base directory at cfg.BuildBase/SL{workerID:02d}
//  2. Creates all mount point directories
//  3. Mounts root tmpfs overlay
//  4. Mounts system directories (bin, lib, usr/bin, etc.) as read-only nullfs
//  5. Mounts work directories (distfiles, packages, construction) as read-write
//  6. Copies template directory (provides shell, basic utilities, etc.)
//
// Mount failures are logged but do not immediately abort setup (fail-safe mode
// for compatibility). If any mounts fail, an aggregate error is returned.
//
// If Setup() returns an error, the caller must still call Cleanup() to release
// any partially-mounted filesystems.
//
// Example:
//
//	env := NewBSDEnvironment()
//	err := env.Setup(1, cfg)
//	if err != nil {
//	    env.Cleanup() // Still cleanup partial state
//	    return err
//	}
//	defer env.Cleanup()
func (e *BSDEnvironment) Setup(workerID int, cfg *config.Config) error {
	e.cfg = cfg
	e.workerID = workerID
	e.baseDir = filepath.Join(cfg.BuildBase, fmt.Sprintf("SL%02d", workerID))
	e.mountErrors = 0

	// Create base directory
	if err := os.MkdirAll(e.baseDir, 0755); err != nil {
		return &environment.ErrSetupFailed{
			Err: fmt.Errorf("cannot create basedir: %w", err),
		}
	}

	// Mount root tmpfs overlay (provides ephemeral root filesystem)
	if err := e.doMount(TmpfsRW, "dummy", ""); err != nil {
		e.mountErrors++
		fmt.Fprintf(os.Stderr, "WARNING: root tmpfs mount failed: %v\n", err)
	}

	// Create all mount point directories upfront
	// This matches the original dsynth behavior
	mountPoints := []string{
		"usr",
		"usr/packages",
		"boot",
		"boot/modules.local",
		"bin",
		"sbin",
		"lib",
		"libexec",
		"usr/bin",
		"usr/include",
		"usr/lib",
		"usr/libdata",
		"usr/libexec",
		"usr/sbin",
		"usr/share",
		"usr/games",
		"usr/src",
		"xports",
		"options",
		"packages",
		"distfiles",
		"construction",
		"usr/local",
		"ccache",
		"tmp",
		"dev",
		"proc",
	}

	for _, mp := range mountPoints {
		dir := filepath.Join(e.baseDir, mp)
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir %s failed: %v\n", dir, err)
			e.mountErrors++
		}
	}

	// System pseudo-filesystems
	// These provide device access, process info, and boot files
	if err := e.doMount(TmpfsRW, "dummy", "/boot"); err != nil {
		e.mountErrors++
		fmt.Fprintf(os.Stderr, "WARNING: /boot mount failed: %v\n", err)
	}
	if err := e.doMount(DevfsRW, "dummy", "/dev"); err != nil {
		e.mountErrors++
		fmt.Fprintf(os.Stderr, "WARNING: /dev mount failed: %v\n", err)
	}
	if err := e.doMount(ProcfsRO, "dummy", "/proc"); err != nil {
		e.mountErrors++
		fmt.Fprintf(os.Stderr, "WARNING: /proc mount failed: %v\n", err)
	}

	// Read-only nullfs mounts from system
	// These provide the base system binaries and libraries
	// $/ prefix means "system path" (typically "/" or custom system root)
	systemMounts := []struct {
		src string
		dst string
	}{
		{"$/bin", "/bin"},
		{"$/sbin", "/sbin"},
		{"$/lib", "/lib"},
		{"$/libexec", "/libexec"},
		{"$/usr/bin", "/usr/bin"},
		{"$/usr/include", "/usr/include"},
		{"$/usr/lib", "/usr/lib"},
		{"$/usr/libdata", "/usr/libdata"},
		{"$/usr/libexec", "/usr/libexec"},
		{"$/usr/sbin", "/usr/sbin"},
		{"$/usr/share", "/usr/share"},
		{"$/usr/games", "/usr/games"},
	}

	for _, m := range systemMounts {
		if err := e.doMount(NullfsRO, m.src, m.dst); err != nil {
			e.mountErrors++
			fmt.Fprintf(os.Stderr, "WARNING: mount %s failed: %v\n", m.dst, err)
		}
	}

	// Optional: /usr/src (source tree)
	// Only mounted if configured (saves resources when not needed)
	if cfg.UseUsrSrc {
		if err := e.doMount(NullfsRO, "$/usr/src", "/usr/src"); err != nil {
			e.mountErrors++
			fmt.Fprintf(os.Stderr, "WARNING: /usr/src mount failed: %v\n", err)
		}
	}

	// Ports tree (read-only)
	// Provides port specifications and Makefiles
	if err := e.doMount(NullfsRO, cfg.DPortsPath, "/xports"); err != nil {
		e.mountErrors++
		fmt.Fprintf(os.Stderr, "WARNING: /xports mount failed: %v\n", err)
	}

	// Build-related directories (read-write)
	// These are shared across workers for coordination
	if err := e.doMount(NullfsRW, cfg.OptionsPath, "/options"); err != nil {
		e.mountErrors++
		fmt.Fprintf(os.Stderr, "WARNING: /options mount failed: %v\n", err)
	}
	if err := e.doMount(NullfsRW, cfg.PackagesPath, "/packages"); err != nil {
		e.mountErrors++
		fmt.Fprintf(os.Stderr, "WARNING: /packages mount failed: %v\n", err)
	}
	if err := e.doMount(NullfsRW, cfg.DistFilesPath, "/distfiles"); err != nil {
		e.mountErrors++
		fmt.Fprintf(os.Stderr, "WARNING: /distfiles mount failed: %v\n", err)
	}

	// Work areas (large tmpfs)
	// /construction: Main build workspace (64GB for large ports)
	// /usr/local: Installed files during build (16GB)
	if err := e.doMount(TmpfsRWBig, "dummy", "/construction"); err != nil {
		e.mountErrors++
		fmt.Fprintf(os.Stderr, "WARNING: /construction mount failed: %v\n", err)
	}
	if err := e.doMount(TmpfsRWMed, "dummy", "/usr/local"); err != nil {
		e.mountErrors++
		fmt.Fprintf(os.Stderr, "WARNING: /usr/local mount failed: %v\n", err)
	}

	// Optional: ccache (compiler cache)
	// Speeds up repeated builds of same source
	if cfg.UseCCache {
		if err := e.doMount(NullfsRW, cfg.CCachePath, "/ccache"); err != nil {
			e.mountErrors++
			fmt.Fprintf(os.Stderr, "WARNING: /ccache mount failed: %v\n", err)
		}
	}

	// Copy template directory
	// Provides essential files: /bin/sh, /etc/passwd, /etc/resolv.conf, etc.
	// These files enable basic shell functionality inside chroot
	templatePath := filepath.Join(cfg.BuildBase, "Template")
	cmd := exec.Command("cp", "-Rp", templatePath+"/.", e.baseDir)
	if err := cmd.Run(); err != nil {
		return &environment.ErrSetupFailed{
			Err: fmt.Errorf("template copy failed: %w", err),
		}
	}

	// Return aggregate error if any mounts failed
	if e.mountErrors > 0 {
		return &environment.ErrSetupFailed{
			Err: fmt.Errorf("mount errors occurred: %d", e.mountErrors),
		}
	}

	return nil
}

// Execute runs a command in the chroot environment.
//
// This method will be implemented in Task 4. For now, return not implemented.
func (e *BSDEnvironment) Execute(ctx context.Context, cmd *environment.ExecCommand) (*environment.ExecResult, error) {
	return nil, &environment.ErrExecutionFailed{
		Command: cmd.Command,
		Err:     fmt.Errorf("Execute() not yet implemented (Task 4)"),
	}
}

// Cleanup tears down the environment.
//
// This method will be implemented in Task 5. For now, return not implemented.
func (e *BSDEnvironment) Cleanup() error {
	return &environment.ErrCleanupFailed{
		Err: fmt.Errorf("Cleanup() not yet implemented (Task 5)"),
	}
}

// GetBasePath returns the root path of the chroot environment.
//
// This path is used for:
//   - Logging and debugging
//   - Copying files into the environment before execution
//   - Compatibility with existing code during migration
//
// Example return: "/build/SL01"
func (e *BSDEnvironment) GetBasePath() string {
	return e.baseDir
}
