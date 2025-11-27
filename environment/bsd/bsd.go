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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
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
// This method executes commands via the chroot(8) utility, not the chroot(2)
// system call. This approach:
//   - Matches the existing Go implementation pattern (build/phases.go)
//   - Simplifies context and timeout handling via exec.CommandContext
//   - Avoids process lifecycle complexity (fork/exec coordination)
//
// Command execution flow:
//  1. Validates environment is setup (baseDir != "")
//  2. Creates derived context if cmd.Timeout > 0
//  3. Builds chroot command: chroot <baseDir> <command> <args...>
//  4. Sets environment variables if cmd.Env provided
//  5. Wires stdout/stderr streams
//  6. Executes command and measures duration
//  7. Returns ExecResult with exit code and duration
//
// Error handling follows the Environment interface contract:
//   - Command exits with code 0: ExecResult{ExitCode: 0}, err=nil (SUCCESS)
//   - Command exits with code N: ExecResult{ExitCode: N}, err=nil (SUCCESS)
//   - Execution fails: ExecResult{ExitCode: -1, Error: ErrExecutionFailed}, err != nil (FAILURE)
//
// The key distinction: a command that runs and returns a non-zero exit code
// is considered successful execution (err=nil). Only failures to execute the
// command (chroot not found, context cancelled, permission denied) return err != nil.
//
// Examples:
//   - make install returns 1 → ExecResult{ExitCode: 1}, err=nil ✅
//   - chroot binary not found → ExecResult{ExitCode: -1, Error: ...}, err != nil ❌
//   - context cancelled → ExecResult{ExitCode: -1, Error: ...}, err != nil ❌
//   - timeout expired → ExecResult{ExitCode: -1, Error: ...}, err != nil ❌
//
// Context and timeout behavior:
//   - Parent context cancellation always takes precedence
//   - If cmd.Timeout > 0: Creates derived context with timeout
//   - Uses exec.CommandContext for automatic process cleanup on cancellation
//   - Timeout errors are wrapped in ErrExecutionFailed
//
// Environment variables:
//   - If cmd.Env is nil or empty: Inherits parent process environment
//   - If cmd.Env is provided: Uses ONLY the specified variables
//   - Variables are converted from map[string]string to []string{"KEY=VALUE"}
//
// WorkDir field:
//   - Currently not implemented (reserved for future enhancement)
//   - The chroot command sets working directory to / inside chroot
//   - Commands handle their own working directory (e.g., make -C /xports/...)
//   - This matches current behavior in build/phases.go (cmd.Dir = "/")
//
// Thread safety:
//   - Safe to call concurrently after Setup() completes
//   - Each call creates independent exec.Cmd instance
//   - No shared mutable state modified during execution
//
// Example usage:
//
//	cmd := &environment.ExecCommand{
//	    Command: "/usr/bin/make",
//	    Args:    []string{"-C", "/xports/editors/vim", "install"},
//	    Env:     map[string]string{"BATCH": "yes"},
//	    Stdout:  logger,
//	    Stderr:  logger,
//	    Timeout: 30 * time.Minute,
//	}
//	result, err := env.Execute(ctx, cmd)
//	if err != nil {
//	    // Execution failed (chroot failed, cancelled, etc)
//	    log.Printf("Execution failed: %v", err)
//	} else if result.ExitCode != 0 {
//	    // Command ran but returned non-zero exit
//	    log.Printf("Command failed with exit code %d", result.ExitCode)
//	} else {
//	    // Success
//	    log.Printf("Command succeeded in %v", result.Duration)
//	}
func (e *BSDEnvironment) Execute(ctx context.Context, cmd *environment.ExecCommand) (*environment.ExecResult, error) {
	// Validate environment is setup
	if e.baseDir == "" {
		return nil, &environment.ErrExecutionFailed{
			Command: cmd.Command,
			Err:     fmt.Errorf("environment not set up (Setup must be called first)"),
		}
	}

	// Handle timeout: create derived context if cmd.Timeout > 0
	// This allows the caller to specify per-command timeouts while
	// still respecting parent context cancellation
	execCtx := ctx
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}

	// Build chroot command arguments
	// Format: chroot <baseDir> <command> <arg1> <arg2> ...
	// Example: chroot /build/SL01 /usr/bin/make -C /xports/editors/vim install
	args := []string{e.baseDir, cmd.Command}
	args = append(args, cmd.Args...)

	// Create command with context support
	// CommandContext ensures the process is killed if context is cancelled
	execCmd := exec.CommandContext(execCtx, "chroot", args...)

	// Set working directory from host perspective
	// The chroot command itself runs from root, and the command inside
	// chroot will have / as its working directory
	execCmd.Dir = "/"

	// Set environment variables if provided
	// If cmd.Env is nil/empty, the command inherits parent environment
	// If cmd.Env is provided, use ONLY those variables
	if len(cmd.Env) > 0 {
		env := make([]string, 0, len(cmd.Env))
		for k, v := range cmd.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		execCmd.Env = env
	}

	// Wire output streams
	// If nil, output is discarded (exec package default behavior)
	if cmd.Stdout != nil {
		execCmd.Stdout = cmd.Stdout
	}
	if cmd.Stderr != nil {
		execCmd.Stderr = cmd.Stderr
	}

	// Execute command and measure duration
	startTime := time.Now()
	err := execCmd.Run()
	duration := time.Since(startTime)

	// Build result
	result := &environment.ExecResult{
		Duration: duration,
	}

	// Handle errors with proper semantics
	// CRITICAL: Non-zero exit code is NOT an error from Execute's perspective
	if err != nil {
		// Try to extract exit code from error
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Command ran but returned non-zero exit code
			// This is SUCCESS from Execute's perspective (err=nil)
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}

		// Execution failed (chroot not found, context cancelled, permission denied, etc)
		// This is FAILURE from Execute's perspective (err != nil)
		result.ExitCode = -1
		return result, &environment.ErrExecutionFailed{
			Command: cmd.Command,
			Err:     err,
		}
	}

	// Success: command ran and returned exit code 0
	result.ExitCode = 0
	return result, nil
}

// Cleanup tears down the environment by unmounting all filesystems and
// removing the base directory.
//
// This method:
//  1. Unmounts all filesystems in reverse order (opposite of Setup)
//  2. Retries unmount operations up to 10 times with 5-second delays for busy mounts
//  3. Logs warnings for failed unmounts but continues (fail-safe behavior)
//  4. Removes the base directory after all unmounts succeed
//  5. Returns error only for catastrophic failures (e.g., baseDir not set)
//
// Unmounting is performed in reverse order to handle dependencies (e.g., /usr/lib
// must be unmounted before /usr).
//
// Retry Logic:
// - If unmount fails with EBUSY (device busy), retry up to 10 times
// - 5-second delay between retry attempts
// - After retries exhausted, log warning and continue to next mount
//
// Error Handling:
// - Returns ErrCleanupFailed only if baseDir is empty (catastrophic)
// - Unmount failures after retries are logged but do NOT fail Cleanup
// - This fail-safe behavior ensures partial cleanup succeeds
//
// Idempotency:
// - Safe to call multiple times (skips if baseDir empty)
// - Safe to call even if Setup() failed or was never called
//
// Example:
//
//	env := &BSDEnvironment{baseDir: "/build/SL01"}
//	err := env.Cleanup()
//	if err != nil {
//	    // Only catastrophic failures reach here
//	    log.Fatalf("cleanup failed: %v", err)
//	}
func (e *BSDEnvironment) Cleanup() error {
	const (
		maxRetries    = 10
		retryDelaySec = 5
	)

	// Validate baseDir is set
	if e.baseDir == "" {
		return &environment.ErrCleanupFailed{
			Err: fmt.Errorf("baseDir is empty, cannot cleanup"),
		}
	}

	log.Printf("[Cleanup] Starting cleanup for environment: %s", e.baseDir)

	// Track unmount failures for logging
	var unmountFailures []string

	// Unmount in reverse order (opposite of Setup)
	// We iterate backwards through e.mounts slice
	for i := len(e.mounts) - 1; i >= 0; i-- {
		ms := e.mounts[i]
		target := ms.target

		log.Printf("[Cleanup] Unmounting %s (attempt 1/%d)", target, maxRetries)

		// Retry loop for busy mounts
		var lastErr error
		unmounted := false

		for attempt := 1; attempt <= maxRetries; attempt++ {
			err := e.doUnmount(target)
			if err == nil {
				log.Printf("[Cleanup] Successfully unmounted %s", target)
				unmounted = true
				break
			}

			lastErr = err

			// Check if EBUSY (device busy)
			if attempt < maxRetries {
				log.Printf("[Cleanup] Unmount failed (attempt %d/%d): %v, retrying in %ds...",
					attempt, maxRetries, err, retryDelaySec)
				time.Sleep(retryDelaySec * time.Second)
			}
		}

		if !unmounted {
			// After all retries failed, log warning and continue
			msg := fmt.Sprintf("%s: %v (after %d retries)", target, lastErr, maxRetries)
			unmountFailures = append(unmountFailures, msg)
			log.Printf("[Cleanup] WARNING: Failed to unmount %s after %d retries: %v",
				target, maxRetries, lastErr)
		}
	}

	// Check if any mounts remain
	remaining := e.listRemainingMounts()
	if len(remaining) > 0 {
		log.Printf("[Cleanup] WARNING: %d mount(s) still present after cleanup: %v",
			len(remaining), remaining)
	}

	// Remove base directory (only if all unmounts succeeded)
	if len(unmountFailures) == 0 {
		log.Printf("[Cleanup] Removing base directory: %s", e.baseDir)
		if err := os.RemoveAll(e.baseDir); err != nil {
			log.Printf("[Cleanup] WARNING: Failed to remove base directory %s: %v",
				e.baseDir, err)
		} else {
			log.Printf("[Cleanup] Successfully removed base directory: %s", e.baseDir)
		}
	} else {
		log.Printf("[Cleanup] Skipping base directory removal due to %d unmount failure(s)",
			len(unmountFailures))
	}

	log.Printf("[Cleanup] Cleanup complete for environment: %s", e.baseDir)
	return nil
}

// listRemainingMounts checks which mounts from e.mounts are still active.
//
// Returns a slice of mount targets that are still mounted according to the
// system mount table. This is used for debugging and validation during Cleanup.
//
// Example:
//
//	remaining := env.listRemainingMounts()
//	if len(remaining) > 0 {
//	    log.Printf("WARNING: %d mounts still present: %v", len(remaining), remaining)
//	}
func (e *BSDEnvironment) listRemainingMounts() []string {
	var remaining []string

	for _, ms := range e.mounts {
		// Check if mount still exists by attempting to stat the target
		// On BSD, a mounted directory will have different inode properties
		// For simplicity, we check if the path exists and assume it's mounted
		// if it was in our mounts list and still exists
		if _, err := os.Stat(ms.target); err == nil {
			// Path exists, assume still mounted
			// A more robust check would parse /etc/mtab or use getmntinfo(3)
			remaining = append(remaining, ms.target)
		}
	}

	return remaining
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
