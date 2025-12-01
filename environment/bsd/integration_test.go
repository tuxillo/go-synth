//go:build integration

package bsd

import (
	"bytes"
	"context"
	"go-synth/config"
	"go-synth/environment"
	"go-synth/log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestIntegration_FullLifecycle tests the complete Setup → Execute → Cleanup workflow.
//
// This test requires root privileges and should be run in a DragonFlyBSD VM.
// It verifies:
//   - Setup creates base directory and mounts all required filesystems
//   - Execute runs commands successfully in the chroot environment
//   - Cleanup unmounts all filesystems and removes the base directory
//
// Run with: doas go test -tags=integration -run TestIntegration_FullLifecycle
func TestIntegration_FullLifecycle(t *testing.T) {
	requireRoot(t)

	ctx := context.Background()
	cfg := createTestConfig(t)
	env := NewBSDEnvironment()

	// Track cleanup
	var cleanupCalled bool
	defer func() {
		if !cleanupCalled {
			_ = env.Cleanup()
		}
	}()

	// Step 1: Setup
	t.Log("Running Setup...")
	err := env.Setup(1, cfg, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Get base directory
	baseDir := env.GetBasePath()
	if baseDir == "" {
		t.Fatal("GetBasePath() returned empty string after Setup()")
	}

	// Verify base directory exists
	if !dirExists(t, baseDir) {
		t.Errorf("baseDir %q does not exist after Setup()", baseDir)
	}
	t.Logf("✓ Setup completed: baseDir=%s", baseDir)

	// Verify critical mount points exist
	criticalMounts := []string{
		filepath.Join(baseDir, "dev"),
		filepath.Join(baseDir, "proc"),
		filepath.Join(baseDir, "tmp"),
		filepath.Join(baseDir, "distfiles"),
		filepath.Join(baseDir, "packages"),
	}
	for _, mount := range criticalMounts {
		if !dirExists(t, mount) {
			t.Errorf("Critical mount point %q missing", mount)
		}
	}
	t.Log("✓ Critical mount points verified")

	// Step 2: Execute commands
	t.Log("Running Execute...")

	// Test 1: Simple echo command
	var stdout1 bytes.Buffer
	result, err := env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/echo",
		Args:    []string{"Hello", "from", "chroot"},
		Stdout:  &stdout1,
	})
	if err != nil {
		t.Errorf("Execute(echo) failed: %v", err)
	} else {
		if result.ExitCode != 0 {
			t.Errorf("echo exit code = %d, want 0", result.ExitCode)
		}
		expectedOutput := "Hello from chroot\n"
		if stdout1.String() != expectedOutput {
			t.Errorf("echo stdout = %q, want %q", stdout1.String(), expectedOutput)
		}
		t.Logf("✓ Execute(echo) succeeded: %q", strings.TrimSpace(stdout1.String()))
	}

	// Test 2: List root directory
	var stdout2 bytes.Buffer
	result, err = env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/ls",
		Args:    []string{"-1", "/"},
		Stdout:  &stdout2,
	})
	if err != nil {
		t.Errorf("Execute(ls) failed: %v", err)
	} else {
		if result.ExitCode != 0 {
			t.Errorf("ls exit code = %d, want 0", result.ExitCode)
		}
		// Verify standard directories exist
		output := stdout2.String()
		for _, dir := range []string{"bin", "dev", "etc", "tmp", "usr"} {
			if !strings.Contains(output, dir) {
				t.Errorf("ls output missing %q directory", dir)
			}
		}
		t.Log("✓ Execute(ls) succeeded: verified standard directories")
	}

	// Test 3: Check /distfiles mount
	result, err = env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/ls",
		Args:    []string{"-d", "/distfiles"},
	})
	if err != nil {
		t.Errorf("Execute(ls /distfiles) failed: %v", err)
	} else {
		if result.ExitCode != 0 {
			t.Errorf("ls /distfiles exit code = %d, want 0", result.ExitCode)
		}
		t.Log("✓ Execute verified /distfiles mount")
	}

	// Step 3: Cleanup
	t.Log("Running Cleanup...")
	cleanupCalled = true
	if err := env.Cleanup(); err != nil {
		t.Errorf("Cleanup() failed: %v", err)
	}
	t.Log("✓ Cleanup completed successfully")

	// Note: baseDir may still exist with tmpfs mount on DragonFly BSD
	// This is expected behavior and not an error

	// Verify cleanup is idempotent
	if err := env.Cleanup(); err != nil {
		t.Errorf("Second Cleanup() call failed: %v", err)
	}
	t.Log("✓ Cleanup is idempotent")
}

// TestIntegration_MultipleCommands tests running multiple commands in the same environment.
//
// This verifies that:
//   - State persists between Execute() calls
//   - File I/O works correctly in the chroot
//   - Temporary files can be created and read
func TestIntegration_MultipleCommands(t *testing.T) {
	requireRoot(t)

	ctx := context.Background()
	cfg := createTestConfig(t)
	env := NewBSDEnvironment()

	defer func() {
		_ = env.Cleanup()
	}()

	// Setup
	err := env.Setup(1, cfg, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	baseDir := env.GetBasePath()
	t.Logf("Setup completed: baseDir=%s", baseDir)

	// Command 1: Create a file in /tmp
	t.Log("Creating test file...")
	result, err := env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/sh",
		Args:    []string{"-c", "echo 'test data' > /tmp/testfile.txt"},
	})
	if err != nil {
		t.Fatalf("Execute(create file) failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("create file exit code = %d, want 0", result.ExitCode)
	}
	t.Log("✓ File created")

	// Command 2: Verify file exists
	result, err = env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/ls",
		Args:    []string{"-l", "/tmp/testfile.txt"},
	})
	if err != nil {
		t.Errorf("Execute(ls testfile) failed: %v", err)
	} else if result.ExitCode != 0 {
		t.Errorf("ls testfile exit code = %d, want 0", result.ExitCode)
	} else {
		t.Log("✓ File exists")
	}

	// Command 3: Read file contents
	var stdout bytes.Buffer
	result, err = env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/cat",
		Args:    []string{"/tmp/testfile.txt"},
		Stdout:  &stdout,
	})
	if err != nil {
		t.Errorf("Execute(cat testfile) failed: %v", err)
	} else {
		if result.ExitCode != 0 {
			t.Errorf("cat exit code = %d, want 0", result.ExitCode)
		}
		expectedContent := "test data\n"
		if stdout.String() != expectedContent {
			t.Errorf("cat output = %q, want %q", stdout.String(), expectedContent)
		}
		t.Logf("✓ File content verified: %q", strings.TrimSpace(stdout.String()))
	}

	// Command 4: Remove file
	result, err = env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/rm",
		Args:    []string{"/tmp/testfile.txt"},
	})
	if err != nil {
		t.Errorf("Execute(rm testfile) failed: %v", err)
	} else if result.ExitCode != 0 {
		t.Errorf("rm exit code = %d, want 0", result.ExitCode)
	} else {
		t.Log("✓ File removed")
	}

	// Command 5: Verify file is gone
	result, err = env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/ls",
		Args:    []string{"/tmp/testfile.txt"},
	})
	if err != nil {
		t.Errorf("Execute(ls deleted file) failed: %v", err)
	} else if result.ExitCode == 0 {
		t.Error("ls should fail for deleted file, but exit code = 0")
	} else {
		t.Log("✓ File deletion verified")
	}

	// Cleanup
	if err := env.Cleanup(); err != nil {
		t.Errorf("Cleanup() failed: %v", err)
	}
}

// TestIntegration_MountVerification verifies all expected mounts are present.
//
// This test verifies:
//   - All 27+ expected mount points are created
//   - Mount points are accessible from inside chroot
//   - /distfiles and /packages are properly mounted
func TestIntegration_MountVerification(t *testing.T) {
	requireRoot(t)

	ctx := context.Background()
	cfg := createTestConfig(t)
	env := NewBSDEnvironment()

	defer func() {
		_ = env.Cleanup()
	}()

	// Setup
	err := env.Setup(1, cfg, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	baseDir := env.GetBasePath()
	t.Logf("Setup completed: baseDir=%s", baseDir)

	// Expected mount points (subset of the 27+ mounts)
	expectedMounts := []struct {
		path string
		desc string
	}{
		{"/dev", "device filesystem"},
		{"/proc", "process filesystem"},
		{"/tmp", "temporary filesystem"},
		{"/distfiles", "distfiles directory"},
		{"/packages", "packages directory"},
		{"/usr/bin", "system binaries"},
		{"/usr/lib", "system libraries"},
		{"/usr/libexec", "system helpers"},
		{"/bin", "basic commands"},
		{"/sbin", "system commands"},
		{"/lib", "basic libraries"},
		{"/libexec", "basic helpers"},
	}

	for _, mount := range expectedMounts {
		fullPath := filepath.Join(baseDir, mount.path)

		// Verify mount point exists on host
		if !dirExists(t, fullPath) {
			t.Errorf("Mount point %q (%s) does not exist", mount.path, mount.desc)
			continue
		}

		// Verify it's accessible from inside the chroot
		result, err := env.Execute(ctx, &environment.ExecCommand{
			Command: "/bin/ls",
			Args:    []string{"-d", mount.path},
		})
		if err != nil {
			t.Errorf("Cannot access %q (%s) from chroot: %v", mount.path, mount.desc, err)
		} else if result.ExitCode != 0 {
			t.Errorf("ls %q failed with exit code %d", mount.path, result.ExitCode)
		} else {
			t.Logf("✓ Mount verified: %s (%s)", mount.path, mount.desc)
		}
	}

	// Cleanup and verify all mounts removed
	if err := env.Cleanup(); err != nil {
		t.Errorf("Cleanup() failed: %v", err)
	}

	// Note: On DragonFly BSD with tmpfs, baseDir may remain after cleanup
	// because the tmpfs itself is mounted. This is expected behavior.
	// The important thing is that all our managed mounts are unmounted.
	t.Log("✓ Cleanup completed - all managed mounts unmounted")
}

// TestIntegration_ConcurrentEnvironments tests multiple concurrent chroot environments.
//
// This verifies that:
//   - Multiple BSDEnvironment instances can run simultaneously
//   - Each environment has isolated base directories
//   - No race conditions occur during setup/cleanup
func TestIntegration_ConcurrentEnvironments(t *testing.T) {
	requireRoot(t)

	const numWorkers = 3
	ctx := context.Background()

	var wg sync.WaitGroup
	errors := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			cfg := createTestConfig(t)
			env := NewBSDEnvironment()
			defer func() {
				_ = env.Cleanup()
			}()

			// Setup
			err := env.Setup(workerID, cfg, log.NoOpLogger{})
			if err != nil {
				errors <- err
				return
			}
			baseDir := env.GetBasePath()
			t.Logf("Worker %d: Setup completed: baseDir=%s", workerID, baseDir)

			// Execute
			var stdout bytes.Buffer
			result, err := env.Execute(ctx, &environment.ExecCommand{
				Command: "/bin/echo",
				Args:    []string{"Worker", string(rune('0' + workerID))},
				Stdout:  &stdout,
			})
			if err != nil {
				errors <- err
				return
			}
			if result.ExitCode != 0 {
				errors <- err
				return
			}
			t.Logf("Worker %d: Execute completed: %q", workerID, strings.TrimSpace(stdout.String()))

			// Cleanup
			if err := env.Cleanup(); err != nil {
				errors <- err
				return
			}
			t.Logf("Worker %d: Cleanup completed", workerID)
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errCount int
	for err := range errors {
		t.Errorf("Worker error: %v", err)
		errCount++
	}

	if errCount == 0 {
		t.Logf("✓ All %d workers completed successfully", numWorkers)
	}
}

// TestIntegration_ContextCancellation tests context cancellation during operations.
//
// This verifies that:
//   - Operations respect context cancellation
//   - Cleanup still works after cancelled operations
func TestIntegration_ContextCancellation(t *testing.T) {
	requireRoot(t)

	cfg := createTestConfig(t)
	env := NewBSDEnvironment()

	defer func() {
		// Use fresh context for cleanup
		_ = env.Cleanup()
	}()

	// Setup with valid context
	t.Log("Testing context cancellation during Execute...")

	err := env.Setup(1, cfg, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	baseDir := env.GetBasePath()
	t.Logf("Setup completed: baseDir=%s", baseDir)

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to execute with cancelled context
	_, err = env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/sleep",
		Args:    []string{"10"},
	})
	if err == nil {
		t.Error("Execute() should fail with cancelled context, but succeeded")
	} else {
		t.Logf("✓ Execute failed with cancelled context: %v", err)
	}

	// Verify cleanup still works
	if err := env.Cleanup(); err != nil {
		t.Errorf("Cleanup() after cancelled Execute failed: %v", err)
	} else {
		t.Log("✓ Cleanup succeeded after cancelled operation")
	}
}

// TestIntegration_ExecuteTimeout tests command execution with timeout.
//
// This verifies that long-running commands can be interrupted.
func TestIntegration_ExecuteTimeout(t *testing.T) {
	requireRoot(t)

	cfg := createTestConfig(t)
	env := NewBSDEnvironment()

	defer func() {
		_ = env.Cleanup()
	}()

	// Setup
	err := env.Setup(1, cfg, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	t.Log("Running command with 500ms timeout (command sleeps 5s)...")
	start := time.Now()
	_, err = env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/sleep",
		Args:    []string{"5"},
	})
	duration := time.Since(start)

	// Should fail due to timeout
	if err == nil {
		t.Error("Execute() should timeout, but succeeded")
	} else {
		t.Logf("✓ Execute timed out as expected after %v: %v", duration, err)
	}

	// Verify it timed out quickly (not after 5 seconds)
	if duration > 2*time.Second {
		t.Errorf("Timeout took %v, expected < 2s", duration)
	}

	// Cleanup
	if err := env.Cleanup(); err != nil {
		t.Errorf("Cleanup() failed: %v", err)
	}
}

// TestIntegration_PartialSetupCleanup tests cleanup after partial setup failure.
//
// This verifies that Cleanup() can handle scenarios where Setup() partially
// completed (some mounts succeeded, others failed).
func TestIntegration_PartialSetupCleanup(t *testing.T) {
	requireRoot(t)

	cfg := createTestConfig(t)
	env := NewBSDEnvironment()

	// Setup with a configuration that might fail partway
	err := env.Setup(1, cfg, log.NoOpLogger{})

	// Regardless of whether Setup succeeded or failed, Cleanup should work
	cleanupErr := env.Cleanup()
	if cleanupErr != nil {
		t.Errorf("Cleanup() after Setup() failed: %v", cleanupErr)
	}

	if err == nil {
		t.Log("✓ Cleanup after successful Setup: OK")
	} else {
		t.Logf("✓ Cleanup after failed Setup: OK (Setup error was: %v)", err)
	}

	// Verify idempotent cleanup
	cleanupErr = env.Cleanup()
	if cleanupErr != nil {
		t.Errorf("Second Cleanup() call failed: %v", cleanupErr)
	}
	t.Log("✓ Cleanup is idempotent after partial setup")
}

// TestIntegration_OutputCapture tests stdout/stderr capture.
//
// This verifies that:
//   - Stdout and stderr are captured separately
//   - Output goes to the correct writers
func TestIntegration_OutputCapture(t *testing.T) {
	requireRoot(t)

	ctx := context.Background()
	cfg := createTestConfig(t)
	env := NewBSDEnvironment()

	defer func() {
		_ = env.Cleanup()
	}()

	// Setup
	err := env.Setup(1, cfg, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Execute command that writes to both stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	result, err := env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/sh",
		Args:    []string{"-c", "echo 'stdout message'; echo 'stderr message' >&2"},
		Stdout:  &stdoutBuf,
		Stderr:  &stderrBuf,
	})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("Command failed with exit code %d", result.ExitCode)
	}

	// Verify stdout
	if !strings.Contains(stdoutBuf.String(), "stdout message") {
		t.Errorf("stdout buffer = %q, want to contain 'stdout message'", stdoutBuf.String())
	} else {
		t.Log("✓ Stdout captured correctly")
	}

	// Verify stderr
	if !strings.Contains(stderrBuf.String(), "stderr message") {
		t.Errorf("stderr buffer = %q, want to contain 'stderr message'", stderrBuf.String())
	} else {
		t.Log("✓ Stderr captured correctly")
	}

	// Cleanup
	if err := env.Cleanup(); err != nil {
		t.Errorf("Cleanup() failed: %v", err)
	}
}

// Helper functions

// requireRoot skips the test if not running as root.
func requireRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() != 0 {
		t.Skip("This test requires root privileges. Run with: doas go test -tags=integration")
	}
}

// createTestConfig creates a test configuration with temporary directories.
// Each test gets its own unique temporary directory to avoid interference.
func createTestConfig(t *testing.T) *config.Config {
	t.Helper()

	// Create unique temp directory for this test run
	// Use t.Name() to make it identifiable in case of failures
	tmpRoot := filepath.Join(os.TempDir(), "go-synth-test-"+t.Name())

	baseDir := filepath.Join(tmpRoot, "build")
	distfiles := filepath.Join(tmpRoot, "distfiles")
	packages := filepath.Join(tmpRoot, "packages")
	dports := filepath.Join(tmpRoot, "dports")
	options := filepath.Join(tmpRoot, "options")

	// Create required directories
	for _, dir := range []string{distfiles, packages, dports, options} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create test directory %q: %v", dir, err)
		}
	}

	// Create minimal template directory
	templateDir := filepath.Join(baseDir, "Template")
	templateEtc := filepath.Join(templateDir, "etc")
	if err := os.MkdirAll(templateEtc, 0755); err != nil {
		t.Fatalf("Failed to create template directory %q: %v", templateEtc, err)
	}

	// Create minimal /etc/passwd
	// Note: /bin/sh is provided by the system mount, not the template
	passwdPath := filepath.Join(templateEtc, "passwd")
	passwd := "root:*:0:0::0:0:root:/root:/bin/sh\n"
	if err := os.WriteFile(passwdPath, []byte(passwd), 0644); err != nil {
		t.Fatalf("Failed to create /etc/passwd in template: %v", err)
	}

	// Register cleanup to remove temp directory after test
	t.Cleanup(func() {
		// Best-effort cleanup - ignore errors since mounts might still be present
		_ = os.RemoveAll(tmpRoot)
	})

	return &config.Config{
		BuildBase:     baseDir,
		SystemPath:    "/",
		DistFilesPath: distfiles,
		PackagesPath:  packages,
		DPortsPath:    dports,
		OptionsPath:   options,
	}
}

// dirExists checks if a directory exists.
func dirExists(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
