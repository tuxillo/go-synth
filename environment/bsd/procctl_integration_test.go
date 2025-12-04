//go:build dragonfly && integration

package bsd

import (
	"bufio"
	"context"
	"fmt"
	"go-synth/config"
	"go-synth/environment"
	"go-synth/log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestIntegration_ProcctlReaping tests procctl-based process reaping.
//
// This test requires:
//   - DragonFly BSD (procctl(2) with PROC_REAP_ACQUIRE/PROC_REAP_KILL)
//   - Root privileges (for chroot and mounts)
//   - Integration test tag
//
// The test verifies that procctl-based reaping kills ALL descendant
// processes, including orphaned and reparented processes, solving the
// "cc1plus survival" bug.
//
// Test strategy:
//  1. Setup BSDEnvironment (creates chroot with mounts)
//  2. Call BecomeReaper() to enable procctl reaping
//  3. Execute spawn_children.sh in chroot (spawns 3 background sleeps)
//  4. Cancel context and trigger cleanup
//  5. Verify NO processes remain in the chroot
//
// Run with:
//
//	doas go test -tags=integration -run TestIntegration_ProcctlReaping ./environment/bsd
func TestIntegration_ProcctlReaping(t *testing.T) {
	requireRoot(t)
	requireDragonFly(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := createTestConfig(t)
	env := NewBSDEnvironment()

	// Track cleanup
	var cleanupCalled bool
	defer func() {
		if !cleanupCalled {
			_ = env.Cleanup()
		}
	}()

	// Step 1: Setup environment
	t.Log("Setting up BSD environment...")
	err := env.Setup(99, cfg, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	baseDir := env.GetBasePath()
	t.Logf("✓ Setup complete: baseDir=%s", baseDir)

	// Step 2: Become reaper BEFORE spawning any processes
	t.Log("Calling BecomeReaper()...")
	if err := BecomeReaper(); err != nil {
		t.Fatalf("BecomeReaper() failed: %v", err)
	}
	t.Log("✓ BecomeReaper() succeeded")

	// Step 3: Copy spawn_children.sh into chroot
	helperScript := filepath.Join(baseDir, "tmp", "spawn_children.sh")
	sourceScript := "testdata/spawn_children.sh"

	// Read source
	content, err := os.ReadFile(sourceScript)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", sourceScript, err)
	}

	// Write to chroot
	if err := os.WriteFile(helperScript, content, 0755); err != nil {
		t.Fatalf("Failed to write script to chroot: %v", err)
	}
	t.Logf("✓ Copied spawn_children.sh to %s", helperScript)

	// Step 4: Execute spawn_children.sh (spawns 3 background processes)
	t.Log("Executing spawn_children.sh (spawns 3 background sleeps)...")

	// Use a short-lived context for Execute (we'll cancel it)
	execCtx, execCancel := context.WithTimeout(ctx, 2*time.Second)
	defer execCancel()

	var stdout strings.Builder
	result, err := env.Execute(execCtx, &environment.ExecCommand{
		Command: "/bin/sh",
		Args:    []string{"/tmp/spawn_children.sh", "3", "9999"},
		Stdout:  &stdout,
		Stderr:  &stdout,
	})

	// We EXPECT this to fail with context deadline (script waits forever)
	if err == nil {
		t.Logf("WARNING: spawn_children.sh completed unexpectedly (exit code %d)", result.ExitCode)
	} else {
		t.Logf("✓ spawn_children.sh timed out as expected: %v", err)
	}
	t.Logf("Script output:\n%s", stdout.String())

	// Step 5: Give processes time to spawn
	time.Sleep(500 * time.Millisecond)

	// Step 6: Count processes in chroot BEFORE cleanup
	pidsBefore := findProcessesInChroot(baseDir)
	t.Logf("Found %d processes in chroot before cleanup: %v", len(pidsBefore), pidsBefore)

	if len(pidsBefore) == 0 {
		t.Fatal("Expected child processes to be running before cleanup")
	}

	// Step 7: Trigger cleanup (should kill all processes via procctl)
	t.Log("Calling ReapAll() to kill descendants...")
	if err := ReapAll(); err != nil {
		t.Errorf("ReapAll() failed: %v", err)
	}

	// Step 8: Verify NO processes remain
	pidsAfter := findProcessesInChroot(baseDir)
	t.Logf("Found %d processes in chroot after cleanup: %v", len(pidsAfter), pidsAfter)

	if len(pidsAfter) > 0 {
		t.Errorf("FAIL: %d processes survived procctl reaping", len(pidsAfter))
		for _, pid := range pidsAfter {
			cmdline := getProcessCmdline(pid)
			t.Errorf("  Survivor PID %d: %s", pid, cmdline)
		}
	} else {
		t.Log("✓ SUCCESS: All processes killed by procctl reaping")
	}

	// Step 9: Cleanup environment
	t.Log("Calling Cleanup()...")
	cleanupCalled = true
	if err := env.Cleanup(); err != nil {
		t.Errorf("Cleanup() failed: %v", err)
	}
	t.Log("✓ Cleanup complete")
}

// TestIntegration_ProcfindReaping tests /proc-based process enumeration.
//
// This test verifies the FALLBACK mechanism (used when procctl isn't available
// or when we're using goroutine-based workers instead of forked processes).
//
// Test strategy:
//  1. Setup BSDEnvironment (creates chroot with mounts)
//  2. Execute spawn_children.sh in chroot (spawns 3 background sleeps)
//  3. Call killProcessesInChroot() to enumerate and kill via /proc
//  4. Verify NO processes remain in the chroot
//
// Run with:
//
//	doas go test -tags=integration -run TestIntegration_ProcfindReaping ./environment/bsd
func TestIntegration_ProcfindReaping(t *testing.T) {
	requireRoot(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := createTestConfig(t)
	env := NewBSDEnvironment()

	// Track cleanup
	var cleanupCalled bool
	defer func() {
		if !cleanupCalled {
			_ = env.Cleanup()
		}
	}()

	// Step 1: Setup environment
	t.Log("Setting up BSD environment...")
	err := env.Setup(98, cfg, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	baseDir := env.GetBasePath()
	t.Logf("✓ Setup complete: baseDir=%s", baseDir)

	// Step 2: Copy spawn_children.sh into chroot
	helperScript := filepath.Join(baseDir, "tmp", "spawn_children.sh")
	sourceScript := "testdata/spawn_children.sh"

	content, err := os.ReadFile(sourceScript)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", sourceScript, err)
	}

	if err := os.WriteFile(helperScript, content, 0755); err != nil {
		t.Fatalf("Failed to write script to chroot: %v", err)
	}
	t.Logf("✓ Copied spawn_children.sh to %s", helperScript)

	// Step 3: Execute spawn_children.sh (spawns 3 background processes)
	t.Log("Executing spawn_children.sh (spawns 3 background sleeps)...")

	execCtx, execCancel := context.WithTimeout(ctx, 2*time.Second)
	defer execCancel()

	var stdout strings.Builder
	result, err := env.Execute(execCtx, &environment.ExecCommand{
		Command: "/bin/sh",
		Args:    []string{"/tmp/spawn_children.sh", "3", "9999"},
		Stdout:  &stdout,
		Stderr:  &stdout,
	})

	if err == nil {
		t.Logf("WARNING: spawn_children.sh completed unexpectedly (exit code %d)", result.ExitCode)
	} else {
		t.Logf("✓ spawn_children.sh timed out as expected: %v", err)
	}
	t.Logf("Script output:\n%s", stdout.String())

	// Step 4: Give processes time to spawn
	time.Sleep(500 * time.Millisecond)

	// Step 5: Count processes BEFORE cleanup
	pidsBefore := findProcessesInChroot(baseDir)
	t.Logf("Found %d processes in chroot before cleanup: %v", len(pidsBefore), pidsBefore)

	if len(pidsBefore) == 0 {
		t.Fatal("Expected child processes to be running before cleanup")
	}

	// Step 6: Kill processes via /proc enumeration
	t.Log("Calling killProcessesInChroot() to enumerate and kill via /proc...")
	if err := killProcessesInChroot(baseDir); err != nil {
		t.Errorf("killProcessesInChroot() failed: %v", err)
	}

	// Step 7: Verify NO processes remain
	pidsAfter := findProcessesInChroot(baseDir)
	t.Logf("Found %d processes in chroot after cleanup: %v", len(pidsAfter), pidsAfter)

	if len(pidsAfter) > 0 {
		t.Errorf("FAIL: %d processes survived /proc enumeration", len(pidsAfter))
		for _, pid := range pidsAfter {
			cmdline := getProcessCmdline(pid)
			t.Errorf("  Survivor PID %d: %s", pid, cmdline)
		}
	} else {
		t.Log("✓ SUCCESS: All processes killed by /proc enumeration")
	}

	// Step 8: Cleanup environment
	t.Log("Calling Cleanup()...")
	cleanupCalled = true
	if err := env.Cleanup(); err != nil {
		t.Errorf("Cleanup() failed: %v", err)
	}
	t.Log("✓ Cleanup complete")
}

// TestIntegration_OrphanedProcesses tests the "cc1plus survival" scenario.
//
// This test simulates the exact bug we observed:
//  1. Parent process (chroot/make) exits normally
//  2. Child processes (cc1plus) are reparented to init
//  3. Old approach: Lost track of children (PID tracking failed)
//  4. New approach: Finds them via /proc enumeration
//
// Run with:
//
//	doas go test -tags=integration -run TestIntegration_OrphanedProcesses ./environment/bsd
func TestIntegration_OrphanedProcesses(t *testing.T) {
	requireRoot(t)

	ctx := context.Background()
	cfg := createTestConfig(t)
	env := NewBSDEnvironment()

	defer env.Cleanup()

	// Setup environment
	err := env.Setup(97, cfg, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	baseDir := env.GetBasePath()
	t.Logf("Setup complete: baseDir=%s", baseDir)

	// Create a script that spawns children then IMMEDIATELY exits (orphaning them)
	orphanScript := filepath.Join(baseDir, "tmp", "spawn_and_exit.sh")
	orphanContent := `#!/bin/sh
# Spawn background children, then exit immediately (orphaning them)
for i in 1 2 3; do
    ( sleep 9999 & )
done
echo "Parent exiting, children orphaned"
exit 0
`
	if err := os.WriteFile(orphanScript, []byte(orphanContent), 0755); err != nil {
		t.Fatalf("Failed to write orphan script: %v", err)
	}

	// Execute script (should complete quickly, leaving orphans)
	t.Log("Executing script that orphans children...")
	var stdout strings.Builder
	result, err := env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/sh",
		Args:    []string{"/tmp/spawn_and_exit.sh"},
		Stdout:  &stdout,
		Stderr:  &stdout,
	})

	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("Script exit code = %d, want 0. Output:\n%s", result.ExitCode, stdout.String())
	}
	t.Logf("✓ Script completed (orphaned children): %s", stdout.String())

	// Wait for children to spawn
	time.Sleep(1 * time.Second)

	// Find orphaned processes
	orphans := findProcessesInChroot(baseDir)
	t.Logf("Found %d orphaned processes: %v", len(orphans), orphans)

	if len(orphans) == 0 {
		t.Fatal("Expected orphaned processes to be running")
	}

	// Kill them via /proc enumeration
	t.Log("Killing orphans via /proc enumeration...")
	if err := killProcessesInChroot(baseDir); err != nil {
		t.Errorf("killProcessesInChroot() failed: %v", err)
	}

	// Verify cleanup
	survivors := findProcessesInChroot(baseDir)
	if len(survivors) > 0 {
		t.Errorf("FAIL: %d orphaned processes survived", len(survivors))
		for _, pid := range survivors {
			cmdline := getProcessCmdline(pid)
			t.Errorf("  Survivor PID %d: %s", pid, cmdline)
		}
	} else {
		t.Log("✓ SUCCESS: All orphaned processes killed")
	}
}

// ==================== Helper Functions ====================

// requireDragonFly skips the test if not running on DragonFly BSD
func requireDragonFly(t *testing.T) {
	t.Helper()

	// Check if we're on DragonFly by reading /proc/version or uname
	cmd := exec.Command("uname", "-s")
	output, err := cmd.Output()
	if err != nil {
		t.Skipf("Cannot determine OS: %v", err)
	}

	os := strings.TrimSpace(string(output))
	if os != "DragonFly" {
		t.Skipf("Test requires DragonFly BSD (this is %s)", os)
	}
}

// getProcessCmdline reads /proc/<pid>/cmdline for debugging
func getProcessCmdline(pid int) string {
	path := fmt.Sprintf("/proc/%d/cmdline", pid)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("<error reading cmdline: %v>", err)
	}

	// cmdline is null-terminated, replace nulls with spaces
	cmdline := string(data)
	cmdline = strings.ReplaceAll(cmdline, "\x00", " ")
	cmdline = strings.TrimSpace(cmdline)

	if cmdline == "" {
		return "<empty cmdline>"
	}

	return cmdline
}

// countProcessesInProc counts processes by scanning /proc (for verification)
func countProcessesInProc() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return -1
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err == nil {
			count++
		}
	}

	return count
}

// verifyProcctlAvailable checks if procctl(2) is available
func verifyProcctlAvailable(t *testing.T) {
	t.Helper()

	// Try to become reaper (will fail if procctl not available)
	err := BecomeReaper()
	if err != nil {
		t.Skipf("procctl(PROC_REAP_ACQUIRE) not available: %v", err)
	}
}

// dumpProcessTree dumps the process tree for debugging
func dumpProcessTree(t *testing.T, label string) {
	t.Helper()

	cmd := exec.Command("ps", "auxww")
	output, err := cmd.Output()
	if err != nil {
		t.Logf("Failed to run ps: %v", err)
		return
	}

	t.Logf("\n===== %s =====\n%s\n=============", label, string(output))
}

// isProcessRunning checks if a PID exists
func isProcessRunning(pid int) bool {
	// Send signal 0 (no-op) to check if process exists
	err := syscall.Kill(pid, 0)
	return err == nil
}

// getProcStats reads /proc/<pid>/stat for detailed process info
func getProcStats(pid int) (ppid int, state string, err error) {
	path := fmt.Sprintf("/proc/%d/stat", pid)
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, "", fmt.Errorf("empty stat file")
	}

	// Parse: PID (comm) state ppid ...
	line := scanner.Text()
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return 0, "", fmt.Errorf("invalid stat format")
	}

	state = fields[2]
	ppid, err = strconv.Atoi(fields[3])
	if err != nil {
		return 0, "", fmt.Errorf("invalid ppid: %v", err)
	}

	return ppid, state, nil
}
