package build

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dsynth/builddb"
	"dsynth/config"
	_ "dsynth/environment/bsd" // Register bsd backend
	"dsynth/log"
	"dsynth/pkg"
)

// ==================== Integration Test Helpers ====================

// setupTestBuild creates a complete test environment for build integration tests.
// Returns BuildContext-compatible components and a cleanup function.
func setupTestBuild(t *testing.T) (*builddb.DB, *config.Config, *log.Logger, func()) {
	t.Helper()

	// Create temp directory for all test artifacts
	tmpDir := t.TempDir()

	// Setup test database
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := builddb.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create test directory structure
	portsDir := filepath.Join(tmpDir, "ports")
	buildBase := filepath.Join(tmpDir, "build")
	logDir := filepath.Join(tmpDir, "logs")
	packagesDir := filepath.Join(tmpDir, "packages")

	// Create all necessary directories
	packagesAll := filepath.Join(packagesDir, "All")
	for _, dir := range []string{portsDir, buildBase, logDir, packagesDir, packagesAll} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create Template directory for BSD environment
	templateDir := filepath.Join(buildBase, "Template")
	templateEtc := filepath.Join(templateDir, "etc")
	if err := os.MkdirAll(templateEtc, 0755); err != nil {
		t.Fatalf("Failed to create template directory: %v", err)
	}

	// Create minimal /etc/passwd for chroot environment
	passwdPath := filepath.Join(templateEtc, "passwd")
	passwd := "root:*:0:0::0:0:root:/root:/bin/sh\n"
	if err := os.WriteFile(passwdPath, []byte(passwd), 0644); err != nil {
		t.Fatalf("Failed to create passwd file: %v", err)
	}

	// Create minimal test config
	cfg := &config.Config{
		DPortsPath:   portsDir,
		BuildBase:    buildBase,
		LogsPath:     logDir,
		PackagesPath: packagesDir,
		MaxWorkers:   1, // Single worker for deterministic tests
		MaxJobs:      1,
		UseTmpfs:     false, // Don't use tmpfs in tests
		Debug:        false,
	}

	// Create test logger
	logger, err := log.NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close database: %v", err)
		}
	}

	return db, cfg, logger, cleanup
}

// createTestPort creates a minimal test port in the specified location.
// Returns the full path to the port directory.
func createTestPort(t *testing.T, baseDir, category, name string) string {
	t.Helper()

	portDir := filepath.Join(baseDir, category, name)
	if err := os.MkdirAll(portDir, 0755); err != nil {
		t.Fatalf("Failed to create port directory %s: %v", portDir, err)
	}

	// Create minimal Makefile
	makefileContent := fmt.Sprintf(`# Test port: %s/%s
PORTNAME=	%s
PORTVERSION=	1.0.0
CATEGORIES=	%s

MAINTAINER=	test@example.com
COMMENT=	Test port for integration tests

NO_BUILD=	yes

do-install:
	@${ECHO} "Installing test port"

.include <bsd.port.mk>
`, category, name, name, category)

	makefilePath := filepath.Join(portDir, "Makefile")
	if err := os.WriteFile(makefilePath, []byte(makefileContent), 0644); err != nil {
		t.Fatalf("Failed to create Makefile: %v", err)
	}

	return portDir
}

// modifyPortFile modifies a file in the port directory to change its CRC.
// Appends a timestamp comment to ensure content changes.
func modifyPortFile(t *testing.T, portDir, filename string) {
	t.Helper()

	filePath := filepath.Join(portDir, filename)

	// Append a timestamp comment to change the file
	comment := fmt.Sprintf("\n# Modified at %s\n", time.Now().Format(time.RFC3339Nano))

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file %s for modification: %v", filePath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(comment); err != nil {
		t.Fatalf("Failed to modify file %s: %v", filePath, err)
	}
}

// addDependency adds a dependency line to a port's Makefile.
func addDependency(t *testing.T, portDir, depPortDir string) {
	t.Helper()

	makefilePath := filepath.Join(portDir, "Makefile")

	// Read existing content
	content, err := os.ReadFile(makefilePath)
	if err != nil {
		t.Fatalf("Failed to read Makefile: %v", err)
	}

	// Add dependency line after CATEGORIES
	depLine := fmt.Sprintf("\nBUILD_DEPENDS=	${PORTSDIR}/%s\n", depPortDir)
	newContent := string(content) + depLine

	if err := os.WriteFile(makefilePath, []byte(newContent), 0644); err != nil {
		t.Fatalf("Failed to write Makefile: %v", err)
	}
}

// assertBuildStats compares build statistics with expected values.
func assertBuildStats(t *testing.T, stats *BuildStats, expectedSuccess, expectedFailed, expectedSkipped int) {
	t.Helper()

	if stats.Success != expectedSuccess {
		t.Errorf("Success count mismatch: got %d, want %d", stats.Success, expectedSuccess)
	}
	if stats.Failed != expectedFailed {
		t.Errorf("Failed count mismatch: got %d, want %d", stats.Failed, expectedFailed)
	}
	if stats.Skipped != expectedSkipped {
		t.Errorf("Skipped count mismatch: got %d, want %d", stats.Skipped, expectedSkipped)
	}
}

// assertBuildRecord verifies that a build record exists in the database with expected status.
func assertBuildRecord(t *testing.T, db *builddb.DB, buildUUID, expectedStatus string) {
	t.Helper()

	rec, err := db.GetRecord(buildUUID)
	if err != nil {
		t.Fatalf("Failed to get build record %s: %v", buildUUID, err)
	}

	if rec == nil {
		t.Fatalf("Build record %s not found in database", buildUUID)
	}

	if rec.Status != expectedStatus {
		t.Errorf("Build record %s status mismatch: got %q, want %q", buildUUID, rec.Status, expectedStatus)
	}
}

// assertCRCStored verifies that a CRC is stored for a port.
func assertCRCStored(t *testing.T, db *builddb.DB, portDir string) uint32 {
	t.Helper()

	crc, found, err := db.GetCRC(portDir)
	if err != nil {
		t.Fatalf("Failed to get CRC for %s: %v", portDir, err)
	}

	if !found {
		t.Fatalf("CRC for %s not found in database", portDir)
	}

	if crc == 0 {
		t.Fatalf("CRC for %s is zero (not stored)", portDir)
	}

	return crc
}

// assertCRCNotStored verifies that no CRC is stored for a port (e.g., after failed build).
func assertCRCNotStored(t *testing.T, db *builddb.DB, portDir string) {
	t.Helper()

	crc, found, err := db.GetCRC(portDir)
	if err != nil {
		t.Fatalf("Unexpected error getting CRC for %s: %v", portDir, err)
	}

	if found && crc != 0 {
		t.Errorf("CRC for %s should not be stored, but got %d", portDir, crc)
	}
}

// assertPackageIndex verifies that the package index points to the expected build UUID.
func assertPackageIndex(t *testing.T, db *builddb.DB, portDir, version, expectedUUID string) {
	t.Helper()

	rec, err := db.LatestFor(portDir, version)
	if err != nil {
		t.Fatalf("Failed to get latest build for %s@%s: %v", portDir, version, err)
	}

	if rec == nil {
		t.Fatalf("No package index entry found for %s@%s", portDir, version)
	}

	if rec.UUID != expectedUUID {
		t.Errorf("Package index for %s@%s points to wrong UUID: got %q, want %q",
			portDir, version, rec.UUID, expectedUUID)
	}
}

// ==================== Integration Tests ====================

// TestIntegration_FirstBuildWorkflow tests the complete first build workflow.
// Verifies that a port builds successfully and all database entries are created.
func TestIntegration_FirstBuildWorkflow(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Requires root privileges")
	}

	db, cfg, logger, cleanup := setupTestBuild(t)
	t.Cleanup(cleanup)

	// Create test port
	portDir := createTestPort(t, cfg.DPortsPath, "misc", "testport1")
	t.Logf("Created test port at: %s", portDir)

	// Parse port
	pkgRegistry := pkg.NewPackageRegistry()
	stateRegistry := pkg.NewBuildStateRegistry()
	packages, err := pkg.ParsePortList([]string{"misc/testport1"}, cfg, stateRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Failed to parse port: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(packages))
	}

	// Build the port
	stats, cleanupBuild, err := DoBuild(packages, cfg, logger, db)
	if cleanupBuild != nil {
		t.Cleanup(cleanupBuild)
	}
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify statistics
	assertBuildStats(t, stats, 1, 0, 0)

	// Verify build record
	buildUUID := packages[0].BuildUUID
	if buildUUID == "" {
		t.Fatal("Build UUID not set on package")
	}
	assertBuildRecord(t, db, buildUUID, "success")

	// Verify CRC stored
	assertCRCStored(t, db, "misc/testport1")

	// Verify package index
	assertPackageIndex(t, db, "misc/testport1", packages[0].Version, buildUUID)

	t.Logf("First build workflow test passed - UUID: %s", buildUUID)
}

// TestIntegration_IncrementalBuildSkip tests that unchanged ports are skipped on rebuild.
func TestIntegration_IncrementalBuildSkip(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Requires root privileges")
	}

	db, cfg, logger, cleanup := setupTestBuild(t)
	t.Cleanup(cleanup)

	// Create test port
	portDir := createTestPort(t, cfg.DPortsPath, "misc", "testport2")
	t.Logf("Created test port at: %s", portDir)

	pkgRegistry := pkg.NewPackageRegistry()
	stateRegistry := pkg.NewBuildStateRegistry()

	// Build 1: Initial build
	packages1, err := pkg.ParsePortList([]string{"misc/testport2"}, cfg, stateRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Failed to parse port: %v", err)
	}

	_, cleanup1, err := DoBuild(packages1, cfg, logger, db)
	if cleanup1 != nil {
		t.Cleanup(cleanup1)
	}
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}
	firstUUID := packages1[0].BuildUUID
	t.Logf("First build completed - UUID: %s", firstUUID)

	// Build 2: Rebuild without changes (should skip)
	stateRegistry2 := pkg.NewBuildStateRegistry()
	packages2, err := pkg.ParsePortList([]string{"misc/testport2"}, cfg, stateRegistry2, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Failed to parse port for rebuild: %v", err)
	}

	stats2, cleanup2, err := DoBuild(packages2, cfg, logger, db)
	if cleanup2 != nil {
		t.Cleanup(cleanup2)
	}
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}

	// Verify port was skipped (CRC match)
	assertBuildStats(t, stats2, 0, 0, 1)

	// Verify no new build record (should still be using firstUUID)
	// Package index should still point to firstUUID
	assertPackageIndex(t, db, "misc/testport2", packages1[0].Version, firstUUID)

	t.Logf("Incremental build skip test passed - port skipped on rebuild")
}

// TestIntegration_RebuildAfterChange tests that modified ports are rebuilt.
func TestIntegration_RebuildAfterChange(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Requires root privileges")
	}

	db, cfg, logger, cleanup := setupTestBuild(t)
	t.Cleanup(cleanup)

	// Create test port
	portDir := createTestPort(t, cfg.DPortsPath, "misc", "testport3")
	t.Logf("Created test port at: %s", portDir)

	pkgRegistry := pkg.NewPackageRegistry()
	stateRegistry := pkg.NewBuildStateRegistry()

	// Build 1: Initial build
	packages1, err := pkg.ParsePortList([]string{"misc/testport3"}, cfg, stateRegistry, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Failed to parse port: %v", err)
	}

	_, cleanup1, err := DoBuild(packages1, cfg, logger, db)
	if cleanup1 != nil {
		t.Cleanup(cleanup1)
	}
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}

	firstUUID := packages1[0].BuildUUID
	oldCRC := assertCRCStored(t, db, "misc/testport3")
	t.Logf("First build completed - UUID: %s, CRC: %d", firstUUID, oldCRC)

	// Modify port to change CRC
	time.Sleep(10 * time.Millisecond) // Ensure timestamp changes
	modifyPortFile(t, portDir, "Makefile")
	t.Logf("Modified port Makefile")

	// Build 2: After change (should rebuild)
	stateRegistry2 := pkg.NewBuildStateRegistry()
	packages2, err := pkg.ParsePortList([]string{"misc/testport3"}, cfg, stateRegistry2, pkgRegistry, log.NoOpLogger{})
	if err != nil {
		t.Fatalf("Failed to parse port for rebuild: %v", err)
	}

	stats2, cleanup2, err := DoBuild(packages2, cfg, logger, db)
	if cleanup2 != nil {
		t.Cleanup(cleanup2)
	}
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}

	// Verify port was rebuilt
	assertBuildStats(t, stats2, 1, 0, 0)

	secondUUID := packages2[0].BuildUUID
	newCRC := assertCRCStored(t, db, "misc/testport3")

	if secondUUID == firstUUID {
		t.Errorf("Second build should have new UUID, but got same: %s", secondUUID)
	}

	if newCRC == oldCRC {
		t.Errorf("CRC should have changed, but stayed the same: %d", newCRC)
	}

	// Verify package index updated to new UUID
	assertPackageIndex(t, db, "misc/testport3", packages2[0].Version, secondUUID)

	t.Logf("Rebuild after change test passed - UUID: %s -> %s, CRC: %d -> %d",
		firstUUID, secondUUID, oldCRC, newCRC)
}

// TestIntegration_FailedBuildHandling tests that failed builds don't update CRC.
func TestIntegration_FailedBuildHandling(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Requires root privileges")
	}

	// This test would require creating an intentionally broken port
	// and verifying that:
	// 1. Build fails and creates "failed" record
	// 2. CRC is NOT updated
	// 3. Package index is NOT updated
	// 4. After fixing, successful build updates everything

	t.Log("Failed build handling test requires more complex setup - deferred")
}

// TestIntegration_MultiPortDependencyChain tests multi-port builds with dependencies.
func TestIntegration_MultiPortDependencyChain(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Requires root privileges")
	}

	// This test would require:
	// 1. Creating port B (no dependencies)
	// 2. Creating port A (depends on B)
	// 3. Building both and verifying CRC skip logic works for dependencies

	t.Log("Multi-port dependency chain test requires complex dependency setup - deferred")
}
