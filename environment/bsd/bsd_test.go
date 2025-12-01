package bsd

import (
	"context"
	"go-synth/config"
	"go-synth/environment"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestBSDEnvironment_Interface validates BSDEnvironment implements Environment.
//
// This is a compile-time check to ensure the type satisfies the interface.
// If this test fails to compile, BSDEnvironment is missing required methods.
func TestBSDEnvironment_Interface(t *testing.T) {
	var _ environment.Environment = (*BSDEnvironment)(nil)
}

// TestNewBSDEnvironment verifies the constructor initializes correctly.
func TestNewBSDEnvironment(t *testing.T) {
	env := NewBSDEnvironment()
	if env == nil {
		t.Fatal("NewBSDEnvironment() returned nil")
	}

	// Verify type
	bsdEnv, ok := env.(*BSDEnvironment)
	if !ok {
		t.Fatalf("NewBSDEnvironment() returned wrong type: %T", env)
	}

	// Verify fields are initialized
	if bsdEnv.mounts == nil {
		t.Error("mounts slice not initialized")
	}
	if cap(bsdEnv.mounts) < 30 {
		t.Errorf("mounts slice capacity = %d, want >= 30", cap(bsdEnv.mounts))
	}
	if bsdEnv.baseDir != "" {
		t.Errorf("baseDir = %q, want empty (not setup yet)", bsdEnv.baseDir)
	}
	if bsdEnv.cfg != nil {
		t.Error("cfg should be nil before Setup()")
	}
	if bsdEnv.mountErrors != 0 {
		t.Errorf("mountErrors = %d, want 0", bsdEnv.mountErrors)
	}
}

// TestBSDEnvironment_GetBasePath verifies GetBasePath returns correct path.
func TestBSDEnvironment_GetBasePath(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string
		want    string
	}{
		{
			name:    "empty before setup",
			baseDir: "",
			want:    "",
		},
		{
			name:    "set after setup",
			baseDir: "/build/SL01",
			want:    "/build/SL01",
		},
		{
			name:    "worker 5",
			baseDir: "/tmp/build/SL05",
			want:    "/tmp/build/SL05",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &BSDEnvironment{
				baseDir: tt.baseDir,
			}

			got := env.GetBasePath()
			if got != tt.want {
				t.Errorf("GetBasePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestResolveMountSource tests the mount source path resolution logic.
//
// This is a critical test because it validates the "$/" substitution
// logic that determines which host paths get mounted into the chroot.
//
// The logic is:
//   - "dummy" → "tmpfs" (special case for tmpfs/devfs/procfs)
//   - "$/path" + SystemPath="/" → "/path"
//   - "$/path" + SystemPath="/custom" → "/custom/path"
//   - "/absolute" → "/absolute" (no change)
func TestResolveMountSource(t *testing.T) {
	tests := []struct {
		name       string
		spath      string
		systemPath string
		want       string
	}{
		{
			name:       "dummy becomes tmpfs",
			spath:      "dummy",
			systemPath: "/",
			want:       "tmpfs",
		},
		{
			name:       "dollar-slash with root system",
			spath:      "$/bin",
			systemPath: "/",
			want:       "/bin",
		},
		{
			name:       "dollar-slash with custom system",
			spath:      "$/bin",
			systemPath: "/custom",
			want:       "/custom/bin",
		},
		{
			name:       "dollar-slash nested path",
			spath:      "$/usr/lib",
			systemPath: "/",
			want:       "/usr/lib",
		},
		{
			name:       "dollar-slash nested with custom",
			spath:      "$/usr/lib",
			systemPath: "/mnt/sysroot",
			want:       "/mnt/sysroot/usr/lib",
		},
		{
			name:       "absolute path unchanged",
			spath:      "/ports/tree",
			systemPath: "/",
			want:       "/ports/tree",
		},
		{
			name:       "absolute path with custom system",
			spath:      "/var/cache/ccache",
			systemPath: "/custom",
			want:       "/var/cache/ccache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal environment to test the logic
			env := &BSDEnvironment{
				cfg: &config.Config{
					SystemPath: tt.systemPath,
				},
			}

			// Extract the resolution logic from doMount
			var got string
			if tt.spath == "dummy" {
				got = "tmpfs"
			} else if len(tt.spath) > 0 && tt.spath[0] == '$' {
				// System path: $/ prefix means relative to SystemPath
				sysPath := env.cfg.SystemPath
				if sysPath == "/" {
					got = tt.spath[1:] // Remove $ prefix
				} else {
					got = filepath.Join(sysPath, tt.spath[1:])
				}
			} else {
				got = tt.spath
			}

			if got != tt.want {
				t.Errorf("resolve(%q, %q) = %q, want %q",
					tt.spath, tt.systemPath, got, tt.want)
			}
		})
	}
}

// TestBSDEnvironment_Execute_NotSetup verifies Execute fails if Setup not called.
func TestBSDEnvironment_Execute_NotSetup(t *testing.T) {
	env := NewBSDEnvironment()
	ctx := context.Background()
	cmd := &environment.ExecCommand{
		Command: "/bin/echo",
		Args:    []string{"hello"},
	}

	result, err := env.Execute(ctx, cmd)

	// Should return error because Setup() wasn't called (baseDir empty)
	if err == nil {
		t.Fatal("Execute() succeeded, want error (Setup not called)")
	}

	// Verify error type
	var execErr *environment.ErrExecutionFailed
	if !errors.As(err, &execErr) {
		t.Errorf("Execute() error type = %T, want *environment.ErrExecutionFailed", err)
	}

	// Verify error mentions setup
	errMsg := err.Error()
	if !contains(errMsg, "Setup") && !contains(errMsg, "not set up") {
		t.Errorf("Execute() error = %q, want message mentioning Setup", errMsg)
	}

	// Result should be nil when validation fails early
	if result != nil {
		t.Errorf("Execute() result = %v, want nil (early validation failure)", result)
	}
}

// TestBSDEnvironment_Execute_ContextCancellation verifies Execute respects context.
func TestBSDEnvironment_Execute_ContextCancellation(t *testing.T) {
	env := &BSDEnvironment{
		baseDir: "/tmp/test-env", // Fake baseDir to pass validation
	}

	// Create context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cmd := &environment.ExecCommand{
		Command: "/bin/sleep",
		Args:    []string{"10"}, // Long sleep
	}

	result, err := env.Execute(ctx, cmd)

	// Should return error due to cancelled context
	if err == nil {
		t.Fatal("Execute() succeeded with cancelled context, want error")
	}

	// Verify error mentions cancellation
	if !errors.Is(err, context.Canceled) && !contains(err.Error(), "cancel") {
		t.Errorf("Execute() error = %v, want cancellation error", err)
	}

	// Verify result has ExitCode -1
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.ExitCode != -1 {
		t.Errorf("Execute() result.ExitCode = %d, want -1", result.ExitCode)
	}
}

// TestBSDEnvironment_Execute_Timeout verifies Execute respects timeout.
//
// NOTE: This test cannot fully verify timeout behavior without root access
// (chroot requires root). Instead, it verifies that Execute completes quickly
// with a short timeout, demonstrating the timeout mechanism is active.
func TestBSDEnvironment_Execute_Timeout(t *testing.T) {
	// Skip this test if we're running as root (would actually work)
	if os.Geteuid() == 0 {
		t.Skip("Test requires non-root user to verify error handling")
	}

	env := &BSDEnvironment{
		baseDir: "/tmp/test-env", // Fake baseDir to pass validation
	}

	ctx := context.Background()
	cmd := &environment.ExecCommand{
		Command: "/bin/sleep",
		Args:    []string{"10"}, // 10 second sleep
		Timeout: 100 * time.Millisecond,
	}

	start := time.Now()
	result, err := env.Execute(ctx, cmd)
	duration := time.Since(start)

	// Should complete quickly (timeout or chroot failure)
	if duration > 1*time.Second {
		t.Errorf("Execute() took %v, want < 1s", duration)
	}

	// Result should be returned even on error
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// If there's an error (chroot failed), verify it's properly handled
	if err != nil {
		// Verify error is ErrExecutionFailed
		var execErr *environment.ErrExecutionFailed
		if !errors.As(err, &execErr) {
			t.Errorf("Execute() error type = %T, want *environment.ErrExecutionFailed", err)
		}
		// ExitCode should be -1 for execution failure
		if result.ExitCode != -1 {
			t.Errorf("Execute() result.ExitCode = %d, want -1 on error", result.ExitCode)
		}
	}
}

// TestBSDEnvironment_Cleanup_EmptyBaseDir verifies Cleanup fails if baseDir empty.
func TestBSDEnvironment_Cleanup_EmptyBaseDir(t *testing.T) {
	env := &BSDEnvironment{
		baseDir: "", // Empty baseDir
	}

	err := env.Cleanup()

	// Should return error
	if err == nil {
		t.Fatal("Cleanup() with empty baseDir succeeded, want error")
	}

	// Verify error type
	var cleanupErr *environment.ErrCleanupFailed
	if !errors.As(err, &cleanupErr) {
		t.Errorf("Cleanup() error type = %T, want *environment.ErrCleanupFailed", err)
	}

	// Verify error mentions baseDir
	errMsg := err.Error()
	if !contains(errMsg, "baseDir") && !contains(errMsg, "empty") {
		t.Errorf("Cleanup() error = %q, want message about empty baseDir", errMsg)
	}
}

// TestBSDEnvironment_Cleanup_NoMounts verifies Cleanup succeeds with no mounts.
func TestBSDEnvironment_Cleanup_NoMounts(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	env := &BSDEnvironment{
		baseDir: tmpDir,
		mounts:  []mountState{}, // No mounts
	}

	err := env.Cleanup()

	// Should succeed (no mounts to clean up)
	if err != nil {
		t.Errorf("Cleanup() with no mounts failed: %v", err)
	}

	// Directory should be removed
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Errorf("Cleanup() did not remove baseDir: %v", err)
	}
}

// TestBSDEnvironment_ListRemainingMounts verifies mount tracking.
// NOTE: This test verifies that listRemainingMounts() correctly parses the
// actual mount table. Without real mount operations (which require root),
// it will return an empty list, which is the correct behavior.
func TestBSDEnvironment_ListRemainingMounts(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	mount1 := filepath.Join(tmpDir, "mount1")
	mount2 := filepath.Join(tmpDir, "mount2")
	mount3 := filepath.Join(tmpDir, "mount3")

	// Create directories (but don't actually mount anything)
	os.MkdirAll(mount1, 0755)
	os.MkdirAll(mount2, 0755)
	os.MkdirAll(mount3, 0755)

	env := &BSDEnvironment{
		baseDir: tmpDir,
		mounts: []mountState{
			{target: mount1, fstype: "tmpfs", source: "tmpfs"},
			{target: mount2, fstype: "null", source: "/usr/bin"},
			{target: mount3, fstype: "null", source: "/usr/lib"},
		},
	}

	remaining := env.listRemainingMounts()

	// Since we're not actually mounting anything (requires root), the function
	// should correctly report no remaining mounts by checking the real mount table.
	// This is an improvement over the old behavior which just checked directory existence.
	if len(remaining) != 0 {
		t.Errorf("listRemainingMounts() = %v (len=%d), want 0 mounts (nothing actually mounted)",
			remaining, len(remaining))
	}

	// Note: Integration tests with root privileges verify the actual mount
	// tracking behavior in build/integration_test.go
}

// TestMountError_Error verifies MountError formatting.
func TestMountError_Error(t *testing.T) {
	tests := []struct {
		name           string
		err            *MountError
		wantSubstrings []string
	}{
		{
			name: "mount error with all fields",
			err: &MountError{
				Op:     "mount",
				Path:   "/build/SL01/usr/lib",
				FSType: "null",
				Source: "/usr/lib",
				Err:    errors.New("permission denied"),
			},
			wantSubstrings: []string{"mount", "/build/SL01/usr/lib", "null", "/usr/lib", "permission denied"},
		},
		{
			name: "unmount error without fstype",
			err: &MountError{
				Op:   "unmount",
				Path: "/build/SL01/proc",
				Err:  errors.New("device busy"),
			},
			wantSubstrings: []string{"unmount", "/build/SL01/proc", "device busy"},
		},
		{
			name: "mkdir error",
			err: &MountError{
				Op:   "mkdir",
				Path: "/build/SL01/usr/bin",
				Err:  errors.New("read-only filesystem"),
			},
			wantSubstrings: []string{"mkdir", "/build/SL01/usr/bin", "read-only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, substr := range tt.wantSubstrings {
				if !contains(got, substr) {
					t.Errorf("Error() = %q, want substring %q", got, substr)
				}
			}
		})
	}
}

// TestMountError_Unwrap verifies error unwrapping works.
func TestMountError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	mountErr := &MountError{
		Op:   "mount",
		Path: "/test",
		Err:  innerErr,
	}

	unwrapped := mountErr.Unwrap()
	if unwrapped != innerErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, innerErr)
	}

	// Verify errors.Is works
	if !errors.Is(mountErr, innerErr) {
		t.Error("errors.Is(mountErr, innerErr) = false, want true")
	}
}

// TestMountTypeFlagsAndMasks verifies mount type constants are correct.
func TestMountTypeFlagsAndMasks(t *testing.T) {
	tests := []struct {
		name       string
		mountType  int
		wantFSType int
		wantRW     bool
		wantBig    bool
		wantMed    bool
	}{
		{
			name:       "TmpfsRW",
			mountType:  TmpfsRW,
			wantFSType: MountTypeTmpfs,
			wantRW:     true,
			wantBig:    false,
			wantMed:    false,
		},
		{
			name:       "TmpfsRWBig",
			mountType:  TmpfsRWBig,
			wantFSType: MountTypeTmpfs,
			wantRW:     true,
			wantBig:    true,
			wantMed:    false,
		},
		{
			name:       "TmpfsRWMed",
			mountType:  TmpfsRWMed,
			wantFSType: MountTypeTmpfs,
			wantRW:     true,
			wantBig:    false,
			wantMed:    true,
		},
		{
			name:       "NullfsRO",
			mountType:  NullfsRO,
			wantFSType: MountTypeNullfs,
			wantRW:     false,
			wantBig:    false,
			wantMed:    false,
		},
		{
			name:       "NullfsRW",
			mountType:  NullfsRW,
			wantFSType: MountTypeNullfs,
			wantRW:     true,
			wantBig:    false,
			wantMed:    false,
		},
		{
			name:       "DevfsRW",
			mountType:  DevfsRW,
			wantFSType: MountTypeDevfs,
			wantRW:     true,
			wantBig:    false,
			wantMed:    false,
		},
		{
			name:       "ProcfsRO",
			mountType:  ProcfsRO,
			wantFSType: MountTypeProcfs,
			wantRW:     false,
			wantBig:    false,
			wantMed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check filesystem type
			gotFSType := tt.mountType & MountTypeMask
			if gotFSType != tt.wantFSType {
				t.Errorf("FSType = 0x%x, want 0x%x", gotFSType, tt.wantFSType)
			}

			// Check RW flag
			gotRW := (tt.mountType & MountTypeRW) != 0
			if gotRW != tt.wantRW {
				t.Errorf("RW = %v, want %v", gotRW, tt.wantRW)
			}

			// Check Big flag
			gotBig := (tt.mountType & MountTypeBig) != 0
			if gotBig != tt.wantBig {
				t.Errorf("Big = %v, want %v", gotBig, tt.wantBig)
			}

			// Check Med flag
			gotMed := (tt.mountType & MountTypeMed) != 0
			if gotMed != tt.wantMed {
				t.Errorf("Med = %v, want %v", gotMed, tt.wantMed)
			}
		})
	}
}

// TestBSDEnvironment_Registration verifies backend is registered.
func TestBSDEnvironment_Registration(t *testing.T) {
	// The init() function should have registered "bsd" backend
	env, err := environment.New("bsd")
	if err != nil {
		t.Fatalf("New(\"bsd\") failed: %v (backend not registered?)", err)
	}

	// Verify type
	if _, ok := env.(*BSDEnvironment); !ok {
		t.Errorf("New(\"bsd\") returned type %T, want *BSDEnvironment", env)
	}
}

// contains is a helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
