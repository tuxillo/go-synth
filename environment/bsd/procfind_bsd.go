//go:build dragonfly || freebsd
// +build dragonfly freebsd

// Package bsd implements process enumeration for BSD systems.
//
// This file provides process enumeration to find ALL processes running
// inside a worker's chroot, not just the ones we directly spawned. This
// solves the "cc1plus survival" problem where child/grandchild processes
// escape our PID tracking.
//
// Strategy:
//  1. Parse /proc to enumerate ALL processes on the system
//  2. Filter for processes whose working directory is inside our chroot
//  3. Kill those processes

package bsd

import (
	"bufio"
	"fmt"
	stdlog "log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// findProcessesInChroot finds all processes running inside a chroot.
//
// This works by:
//  1. Enumerating /proc/[pid] entries
//  2. Reading /proc/[pid]/file (DragonFly) or /proc/[pid]/cwd (FreeBSD)
//  3. Checking if the path starts with chrootPath
//
// Returns a list of PIDs (as integers).
//
// This is more reliable than PID tracking because it discovers:
//   - Background processes spawned by make
//   - Daemon processes started during build
//   - Orphaned/reparented processes
//   - Processes created after cleanup started
//
// Example:
//
//	pids := findProcessesInChroot("/build/SL01")
//	// Returns: [12345, 12346, 12347] (all processes in /build/SL01)
func findProcessesInChroot(chrootPath string) []int {
	var pids []int

	// Enumerate /proc
	entries, err := os.ReadDir("/proc")
	if err != nil {
		stdlog.Printf("[Cleanup] WARNING: Cannot read /proc: %v", err)
		return pids
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Parse PID from directory name
		pidStr := entry.Name()
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue // Not a numeric directory
		}

		// Skip our own process
		if pid == os.Getpid() {
			continue
		}

		// Check if process is inside our chroot
		if isProcessInChroot(pid, chrootPath) {
			pids = append(pids, pid)
		}
	}

	return pids
}

// isProcessInChroot checks if a process is running inside a chroot.
//
// DragonFly BSD: Read /proc/[pid]/file for executable path
// FreeBSD: Read /proc/[pid]/cwd for current working directory
//
// This is a heuristic - not perfect but catches most cases.
func isProcessInChroot(pid int, chrootPath string) bool {
	// Try reading cwd symlink (works on both DragonFly and FreeBSD)
	cwdPath := filepath.Join("/proc", strconv.Itoa(pid), "cwd")
	target, err := os.Readlink(cwdPath)
	if err == nil {
		// If cwd is inside chroot, process is likely inside chroot
		if strings.HasPrefix(target, chrootPath+"/") || target == chrootPath {
			return true
		}
	}

	// Try reading file (DragonFly: lists open files)
	filePath := filepath.Join("/proc", strconv.Itoa(pid), "file")
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Each line format: "filename" or similar
		// Check if any path references our chroot
		if strings.Contains(line, chrootPath) {
			return true
		}
	}

	return false
}

// killProcessesInChroot finds and kills all processes in a chroot.
//
// This is the ALTERNATIVE to procctl-based reaping for Go programs
// that don't fork separate worker processes.
//
// Strategy:
//  1. Find processes via /proc enumeration
//  2. Send SIGTERM, wait 2 seconds
//  3. Find remaining processes
//  4. Send SIGKILL, wait for exit
//
// This matches dsynth's two-phase kill strategy but uses /proc
// enumeration instead of reaper tracking.
//
// Thread safety:
//   - Safe to call concurrently (separate chroot paths)
//   - Each worker calls with its own chrootPath
//
// Example:
//
//	if err := killProcessesInChroot("/build/SL01"); err != nil {
//	    log.Printf("WARNING: Process killing incomplete: %v", err)
//	}
func killProcessesInChroot(chrootPath string) error {
	// Phase 1: Find and SIGTERM all processes
	pids := findProcessesInChroot(chrootPath)
	if len(pids) == 0 {
		return nil
	}

	stdlog.Printf("[Cleanup] Found %d process(es) in chroot %s", len(pids), chrootPath)

	for _, pid := range pids {
		// Kill process group (negative PID)
		// This sends signal to the entire process tree
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
			// ESRCH is expected if process already exited
			if err != syscall.ESRCH {
				stdlog.Printf("[Cleanup] WARNING: Failed to SIGTERM process group %d: %v", pid, err)
			}
		} else {
			stdlog.Printf("[Cleanup] Sent SIGTERM to process group %d", pid)
		}
	}

	// Wait for processes to terminate gracefully
	// time.Sleep(2 * time.Second)
	// REMOVED: Sleep is problematic during shutdown; rely on SIGKILL pass

	// Phase 2: Find remaining processes and SIGKILL
	pids = findProcessesInChroot(chrootPath)
	if len(pids) == 0 {
		stdlog.Printf("[Cleanup] All processes in %s terminated gracefully", chrootPath)
		return nil
	}

	stdlog.Printf("[Cleanup] Force-killing %d remaining process(es) in %s", len(pids), chrootPath)

	for _, pid := range pids {
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			// ESRCH is expected if process already exited
			if err != syscall.ESRCH {
				stdlog.Printf("[Cleanup] WARNING: Failed to SIGKILL process group %d: %v", pid, err)
			}
		} else {
			stdlog.Printf("[Cleanup] Sent SIGKILL to process group %d", pid)
		}
	}

	// Wait a moment for SIGKILL to take effect
	// time.Sleep(500 * time.Millisecond)
	// REMOVED: Rely on OS to clean up; we've done our part

	// Final check: report any survivors
	survivors := findProcessesInChroot(chrootPath)
	if len(survivors) > 0 {
		stdlog.Printf("[Cleanup] WARNING: %d process(es) survived SIGKILL in %s", len(survivors), chrootPath)
		for _, pid := range survivors {
			stdlog.Printf("[Cleanup] WARNING: Survivor PID %d", pid)
		}
		return fmt.Errorf("%d processes survived SIGKILL", len(survivors))
	}

	stdlog.Printf("[Cleanup] All processes in %s terminated", chrootPath)
	return nil
}
