//go:build dragonfly
// +build dragonfly

// Package bsd implements DragonFly BSD-specific process control via procctl(2).
//
// This file provides Go bindings for the procctl(2) system call, specifically
// the PROC_REAP_* commands used for process reaping. The reaper mechanism
// allows a process to automatically inherit ALL orphaned descendants and
// enumerate/kill them at cleanup time.
//
// This matches the original dsynth C implementation which uses:
//   - procctl(PROC_REAP_ACQUIRE) to become reaper
//   - procctl(PROC_REAP_KILL) to enumerate and kill all descendants
//   - procctl(PROC_REAP_STATUS) for older DragonFly versions
//
// References:
//   - DragonFly BSD procctl(2) man page
//   - .original-c-source/build.c:2868 (phaseReapAll function)
//   - .original-c-source/build.c:2105 (PROC_REAP_ACQUIRE setup)

package bsd

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// procctl constants from DragonFly BSD <sys/procctl.h>
// These values are stable ABI and match the C implementation.
const (
	// idtype_t values for procctl(2)
	P_PID = 0 // Process ID

	// PROC_REAP_* commands for procctl(2)
	PROC_REAP_ACQUIRE = 2 // Acquire reaper status
	PROC_REAP_RELEASE = 3 // Release reaper status
	PROC_REAP_STATUS  = 4 // Get reaper status (older DragonFly)
	PROC_REAP_KILL    = 5 // Kill all descendants (DragonFly >= 6.0.5)
)

// reaperStatus matches struct reaper_status from DragonFly BSD.
// Used with PROC_REAP_STATUS to enumerate descendants (pre-6.0.5).
type reaperStatus struct {
	Flags   uint32 // PROC_REAP_ACQUIRE flag
	Refs    uint32 // Number of reaper refs
	PidHead int32  // First PID in reaper's subtree
}

// reaperKill matches struct reaper_kill from DragonFly BSD.
// Used with PROC_REAP_KILL to kill all descendants (6.0.5+).
type reaperKill struct {
	Signal int32  // Signal to send (SIGKILL)
	Flags  uint32 // Reserved (must be 0)
	Killed int32  // OUT: Number of processes killed
	_      int32  // Padding
}

// procctl wraps the raw procctl(2) system call.
//
// Arguments:
//   - idtype: Process ID type (P_PID for process ID)
//   - id: Process ID (use getpid() for self)
//   - cmd: Command (PROC_REAP_ACQUIRE, PROC_REAP_KILL, etc)
//   - data: Pointer to command-specific data structure (may be nil)
//
// Returns:
//   - error: nil on success, syscall.Errno on failure
//
// Example:
//
//	err := procctl(P_PID, unix.Getpid(), PROC_REAP_ACQUIRE, nil)
//	if err != nil {
//	    return fmt.Errorf("cannot become reaper: %w", err)
//	}
func procctl(idtype int, id int, cmd int, data unsafe.Pointer) error {
	_, _, errno := unix.Syscall6(
		unix.SYS_PROCCTL,
		uintptr(idtype),
		uintptr(id),
		uintptr(cmd),
		uintptr(data),
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// BecomeReaper makes the current process a reaper.
//
// A reaper automatically inherits ALL orphaned descendant processes,
// even if they're reparented or escape process groups. This ensures
// we can enumerate and kill every single process in our subtree at
// cleanup time.
//
// This matches the original dsynth behavior:
//   - .original-c-source/build.c:2105
//   - Called once per worker process at startup
//
// Thread safety:
//   - Must be called from the main worker goroutine/thread
//   - Should be called BEFORE any processes are spawned
//   - Once set, reaper status persists until process exit
//
// Example:
//
//	if err := BecomeReaper(); err != nil {
//	    return fmt.Errorf("failed to become reaper: %w", err)
//	}
func BecomeReaper() error {
	err := procctl(P_PID, unix.Getpid(), PROC_REAP_ACQUIRE, nil)
	if err != nil {
		return fmt.Errorf("procctl(PROC_REAP_ACQUIRE) failed: %w", err)
	}
	return nil
}

// ReapAll kills and reaps all descendant processes.
//
// This method:
//  1. Enumerates ALL processes in the reaper subtree (via OS)
//  2. Sends SIGKILL to each process
//  3. Waits for all processes to exit
//  4. Repeats until no processes remain
//
// This matches the original dsynth implementation:
//   - .original-c-source/build.c:2868 (phaseReapAll function)
//   - Uses PROC_REAP_KILL on DragonFly >= 6.0.5
//   - Falls back to PROC_REAP_STATUS on older versions
//
// The key advantage over manual PID tracking:
//   - Discovers processes spawned AFTER tracking started
//   - Finds processes that escaped process groups
//   - Handles orphaned/reparented processes
//   - No race conditions with process creation
//
// Thread safety:
//   - Safe to call concurrently (procctl is thread-safe)
//   - Should be called from cleanup path (once per worker)
//
// Example:
//
//	if err := ReapAll(); err != nil {
//	    log.Printf("WARNING: Process reaping incomplete: %v", err)
//	}
func ReapAll() error {
	// Try modern PROC_REAP_KILL first (DragonFly >= 6.0.5)
	// This is the most efficient method: one syscall per batch
	for {
		var rk reaperKill
		rk.Signal = int32(syscall.SIGKILL)
		rk.Flags = 0

		err := procctl(P_PID, unix.Getpid(), PROC_REAP_KILL, unsafe.Pointer(&rk))
		if err != nil {
			// If PROC_REAP_KILL not supported, fall back to PROC_REAP_STATUS
			if errno, ok := err.(syscall.Errno); ok && errno == syscall.EINVAL {
				return reapAllLegacy()
			}
			return fmt.Errorf("procctl(PROC_REAP_KILL) failed: %w", err)
		}

		// If no processes were killed, we're done
		if rk.Killed == 0 {
			break
		}

		// Reap all killed processes (wait for them to exit)
		// wait3(2) with NULL rusage reaps any child
		for {
			var status syscall.WaitStatus
			pid, err := syscall.Wait4(-1, &status, 0, nil)
			if err != nil {
				// ECHILD means no more children (expected)
				if errno, ok := err.(syscall.Errno); ok && errno == syscall.ECHILD {
					break
				}
				return fmt.Errorf("wait4 failed: %w", err)
			}
			if pid <= 0 {
				break
			}
		}
	}

	// Final cleanup: reap any stragglers (race condition mitigation)
	// This matches dsynth's final while(wait3(...) > 0) loop
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, 0, nil)
		if err != nil || pid <= 0 {
			break
		}
	}

	return nil
}

// reapAllLegacy is the fallback for older DragonFly versions (< 6.0.5).
//
// Uses PROC_REAP_STATUS to enumerate descendants one at a time.
// This is slower but achieves the same result.
//
// Matches .original-c-source/build.c:2884 (#else branch).
func reapAllLegacy() error {
	for {
		var rs reaperStatus
		err := procctl(P_PID, unix.Getpid(), PROC_REAP_STATUS, unsafe.Pointer(&rs))
		if err != nil {
			return fmt.Errorf("procctl(PROC_REAP_STATUS) failed: %w", err)
		}

		// Check if we're actually a reaper
		const PROC_REAP_ACQUIRE_FLAG = 0x0001
		if rs.Flags&PROC_REAP_ACQUIRE_FLAG == 0 {
			// Not a reaper (should not happen)
			break
		}

		// No more descendants
		if rs.PidHead < 0 {
			break
		}

		// Kill the first process in the subtree
		if err := syscall.Kill(int(rs.PidHead), syscall.SIGKILL); err != nil {
			// Process may have exited between PROC_REAP_STATUS and kill
			if errno, ok := err.(syscall.Errno); ok && errno == syscall.ESRCH {
				continue
			}
			return fmt.Errorf("kill(%d) failed: %w", rs.PidHead, err)
		}

		// Wait for it to exit
		for {
			var status syscall.WaitStatus
			pid, err := syscall.Wait4(int(rs.PidHead), &status, 0, nil)
			if err != nil {
				// ECHILD or other error, move on
				break
			}
			if pid == int(rs.PidHead) {
				break
			}
		}
	}

	// Final cleanup
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, 0, nil)
		if err != nil || pid <= 0 {
			break
		}
	}

	return nil
}
