package build

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
	"unsafe"

	"go-synth/builddb"
	"go-synth/config"
	"go-synth/environment"
	"go-synth/log"
	"go-synth/pkg"
)

// setupTestBuildContext creates a minimal BuildContext for testing
func setupTestBuildContext(t *testing.T) (*BuildContext, context.CancelFunc, func()) {
	ctx, cancel := context.WithCancel(context.Background())

	tmpDir := t.TempDir()
	testCfg := &config.Config{
		LogsPath: filepath.Join(tmpDir, "logs"),
	}

	testLog, err := log.NewLogger(testCfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := builddb.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}

	registry := pkg.NewBuildStateRegistry()

	buildCtx := &BuildContext{
		ctx:      ctx,
		cancel:   cancel,
		logger:   testLog,
		registry: registry,
		buildDB:  db,
		queue:    make(chan *pkg.Package, 10),
		stats:    BuildStats{},
		cfg:      testCfg,
	}

	cleanup := func() {
		testLog.Close()
		db.Close()
	}

	return buildCtx, cancel, cleanup
}

// TestWorkerLoop_QueueDrain tests that workerLoop processes packages from queue
func TestWorkerLoop_QueueDrain(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cancel()
	defer cleanup()

	mockEnv := newMockEnv()
	worker := &Worker{
		ID:     0,
		Env:    mockEnv,
		Status: "idle",
	}

	// Create fake packages
	pkg1 := &pkg.Package{PortDir: "test/pkg1", Category: "test", Name: "pkg1"}
	pkg2 := &pkg.Package{PortDir: "test/pkg2", Category: "test", Name: "pkg2"}

	// Start worker in goroutine
	buildCtx.wg.Add(1)
	go buildCtx.workerLoop(worker)

	// Enqueue packages
	buildCtx.queue <- pkg1
	buildCtx.queue <- pkg2
	close(buildCtx.queue)

	// Wait for worker to finish
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - worker exited
	case <-time.After(2 * time.Second):
		t.Fatal("Worker did not exit after queue closed")
	}

	// Verify stats were updated (packages should have been processed)
	buildCtx.statsMu.Lock()
	defer buildCtx.statsMu.Unlock()

	total := buildCtx.stats.Success + buildCtx.stats.Failed
	if total != 2 {
		t.Errorf("Expected 2 packages processed, got %d (success=%d, failed=%d)",
			total, buildCtx.stats.Success, buildCtx.stats.Failed)
	}
}

// TestWorkerLoop_ContextCancellation tests that workerLoop exits when context is cancelled
func TestWorkerLoop_ContextCancellation(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cleanup()

	mockEnv := newMockEnv()
	worker := &Worker{
		ID:     0,
		Env:    mockEnv,
		Status: "idle",
	}

	// Start worker in goroutine
	buildCtx.wg.Add(1)
	go buildCtx.workerLoop(worker)

	// Cancel context immediately
	cancel()

	// Wait for worker to finish
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - worker exited due to cancellation
	case <-time.After(1 * time.Second):
		t.Fatal("Worker did not exit after context cancellation")
	}

	// Note: We can't easily verify log messages with real logger
	// but the important part is the worker exited cleanly
}

// TestWorkerLoop_CancellationMidQueue tests cancellation while packages are queued
func TestWorkerLoop_CancellationMidQueue(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cleanup()

	mockEnv := newMockEnv()
	// Set a delay to simulate slow builds
	mockEnv.executeDelay = 100 * time.Millisecond

	worker := &Worker{
		ID:     0,
		Env:    mockEnv,
		Status: "idle",
	}

	// Create fake packages
	pkg1 := &pkg.Package{PortDir: "test/pkg1", Category: "test", Name: "pkg1"}
	pkg2 := &pkg.Package{PortDir: "test/pkg2", Category: "test", Name: "pkg2"}
	pkg3 := &pkg.Package{PortDir: "test/pkg3", Category: "test", Name: "pkg3"}

	// Enqueue packages
	buildCtx.queue <- pkg1
	buildCtx.queue <- pkg2
	buildCtx.queue <- pkg3

	// Start worker in goroutine
	buildCtx.wg.Add(1)
	go buildCtx.workerLoop(worker)

	// Let worker start first package
	time.Sleep(50 * time.Millisecond)

	// Cancel context while worker is busy
	cancel()

	// Wait for worker to finish
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - worker exited
	case <-time.After(2 * time.Second):
		t.Fatal("Worker did not exit after context cancellation")
	}

	// Worker should have processed at most 1-2 packages before seeing cancellation
	buildCtx.statsMu.Lock()
	total := buildCtx.stats.Success + buildCtx.stats.Failed
	buildCtx.statsMu.Unlock()

	if total >= 3 {
		t.Errorf("Expected fewer than 3 packages processed after cancellation, got %d", total)
	}
}

// TestWorkerLoop_EmptyQueue tests that workerLoop exits cleanly with empty queue
func TestWorkerLoop_EmptyQueue(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cancel()
	defer cleanup()

	mockEnv := newMockEnv()
	worker := &Worker{
		ID:     0,
		Env:    mockEnv,
		Status: "idle",
	}

	// Start worker in goroutine
	buildCtx.wg.Add(1)
	go buildCtx.workerLoop(worker)

	// Close queue immediately without enqueueing anything
	close(buildCtx.queue)

	// Wait for worker to finish
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - worker exited cleanly
	case <-time.After(1 * time.Second):
		t.Fatal("Worker did not exit after empty queue closed")
	}

	// Verify no packages were processed
	buildCtx.statsMu.Lock()
	total := buildCtx.stats.Success + buildCtx.stats.Failed
	buildCtx.statsMu.Unlock()

	if total != 0 {
		t.Errorf("Expected 0 packages processed, got %d", total)
	}
}

// TestCleanupDoneChannel tests that cleanup's done channel closes when workers exit
func TestCleanupDoneChannel(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cleanup()

	// Create 2 workers
	for i := 0; i < 2; i++ {
		mockEnv := newMockEnv()
		worker := &Worker{
			ID:     i,
			Env:    mockEnv,
			Status: "idle",
		}
		buildCtx.workers = append(buildCtx.workers, worker)

		// Start worker goroutine
		buildCtx.wg.Add(1)
		go buildCtx.workerLoop(worker)
	}

	// Close queue to let workers exit
	close(buildCtx.queue)

	// Simulate the cleanup done channel behavior
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	// Verify done channel closes promptly
	select {
	case <-done:
		// Success - done channel closed
	case <-time.After(1 * time.Second):
		t.Fatal("Done channel did not close after workers exited")
	}

	// Cancel context for cleanup
	cancel()
}

// TestCleanupTimeout tests cleanup behavior when workers don't exit
func TestCleanupTimeout(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cleanup()

	// Create a "stuck" worker that won't exit immediately
	mockEnv := newMockEnv()
	mockEnv.executeDelay = 10 * time.Second // Very long delay

	worker := &Worker{
		ID:     0,
		Env:    mockEnv,
		Status: "idle",
	}
	buildCtx.workers = append(buildCtx.workers, worker)

	// Create a package that will take a long time
	pkg1 := &pkg.Package{PortDir: "test/slow", Category: "test", Name: "slow"}
	buildCtx.queue <- pkg1

	// Start worker
	buildCtx.wg.Add(1)
	go buildCtx.workerLoop(worker)

	// Let worker start processing
	time.Sleep(50 * time.Millisecond)

	// Close queue and cancel context (simulating cleanup start)
	close(buildCtx.queue)
	cancel()

	// Simulate the cleanup timeout behavior
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	// Verify we can timeout waiting for done channel
	timeout := 500 * time.Millisecond
	select {
	case <-done:
		// Worker exited (context cancellation worked)
	case <-time.After(timeout):
		// Expected - worker is taking too long, cleanup would proceed anyway
		t.Log("Timeout occurred as expected, cleanup would proceed")
	}

	// Note: In real cleanup, we'd proceed with environment cleanup here
	// even if workers haven't exited yet
}

// TestMultipleWorkersCleanup tests cleanup with multiple workers processing packages
func TestMultipleWorkersCleanup(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cleanup()

	numWorkers := 3
	numPackages := 6

	// Create workers
	for i := 0; i < numWorkers; i++ {
		mockEnv := newMockEnv()
		mockEnv.executeDelay = 10 * time.Millisecond // Small delay to simulate work

		worker := &Worker{
			ID:     i,
			Env:    mockEnv,
			Status: "idle",
		}
		buildCtx.workers = append(buildCtx.workers, worker)

		buildCtx.wg.Add(1)
		go buildCtx.workerLoop(worker)
	}

	// Enqueue packages
	for i := 0; i < numPackages; i++ {
		pkg := &pkg.Package{
			PortDir:  "test/pkg" + string(rune('0'+i)),
			Category: "test",
			Name:     "pkg" + string(rune('0'+i)),
		}
		buildCtx.queue <- pkg
	}

	// Close queue
	close(buildCtx.queue)

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - all workers exited
	case <-time.After(2 * time.Second):
		t.Fatal("Workers did not exit after processing all packages")
	}

	// Verify all packages were processed
	buildCtx.statsMu.Lock()
	total := buildCtx.stats.Success + buildCtx.stats.Failed
	buildCtx.statsMu.Unlock()

	if total != numPackages {
		t.Errorf("Expected %d packages processed, got %d", numPackages, total)
	}

	// Cleanup
	cancel()
}

// TestDoFetchOnly_QueueDrain tests that fetch workers process all packages
func TestDoFetchOnly_QueueDrain(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		MaxWorkers:    2,
		LogsPath:      filepath.Join(tmpDir, "logs"),
		DPortsPath:    "/usr/ports",
		DistFilesPath: filepath.Join(tmpDir, "distfiles"),
	}

	testLog, err := log.NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer testLog.Close()

	registry := pkg.NewBuildStateRegistry()

	// Create test packages
	packages := []*pkg.Package{
		{PortDir: "test/pkg1", Category: "test", Name: "pkg1"},
		{PortDir: "test/pkg2", Category: "test", Name: "pkg2"},
		{PortDir: "test/pkg3", Category: "test", Name: "pkg3"},
	}

	// Mock fetcher that always succeeds
	successFetcher := func(p *pkg.Package, cfg *config.Config) bool {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return true
	}

	stats, err := doFetchOnlyWithFetcher(packages, cfg, registry, testLog, successFetcher)
	if err != nil {
		t.Fatalf("DoFetchOnly failed: %v", err)
	}

	if stats.Total != 3 {
		t.Errorf("Expected 3 total packages, got %d", stats.Total)
	}
	if stats.Success != 3 {
		t.Errorf("Expected 3 successful fetches, got %d", stats.Success)
	}
	if stats.Failed != 0 {
		t.Errorf("Expected 0 failed fetches, got %d", stats.Failed)
	}
}

// TestDoFetchOnly_PartialFailure tests handling of some fetch failures
func TestDoFetchOnly_PartialFailure(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		MaxWorkers:    2,
		LogsPath:      filepath.Join(tmpDir, "logs"),
		DPortsPath:    "/usr/ports",
		DistFilesPath: filepath.Join(tmpDir, "distfiles"),
	}

	testLog, err := log.NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer testLog.Close()

	registry := pkg.NewBuildStateRegistry()

	// Create test packages
	packages := []*pkg.Package{
		{PortDir: "test/pkg1", Category: "test", Name: "pkg1"},
		{PortDir: "test/fail", Category: "test", Name: "fail"},
		{PortDir: "test/pkg3", Category: "test", Name: "pkg3"},
	}

	// Mock fetcher that fails for "fail" package
	mixedFetcher := func(p *pkg.Package, cfg *config.Config) bool {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return p.Name != "fail"
	}

	stats, err := doFetchOnlyWithFetcher(packages, cfg, registry, testLog, mixedFetcher)
	if err != nil {
		t.Fatalf("DoFetchOnly failed: %v", err)
	}

	if stats.Total != 3 {
		t.Errorf("Expected 3 total packages, got %d", stats.Total)
	}
	if stats.Success != 2 {
		t.Errorf("Expected 2 successful fetches, got %d", stats.Success)
	}
	if stats.Failed != 1 {
		t.Errorf("Expected 1 failed fetch, got %d", stats.Failed)
	}
}

// TestDoFetchOnly_EmptyQueue tests fetch with no packages
func TestDoFetchOnly_EmptyQueue(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		MaxWorkers:    2,
		LogsPath:      filepath.Join(tmpDir, "logs"),
		DPortsPath:    "/usr/ports",
		DistFilesPath: filepath.Join(tmpDir, "distfiles"),
	}

	testLog, err := log.NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer testLog.Close()

	registry := pkg.NewBuildStateRegistry()

	// Empty package list
	packages := []*pkg.Package{}

	// Mock fetcher (should never be called)
	neverCalledFetcher := func(p *pkg.Package, cfg *config.Config) bool {
		t.Error("Fetcher should not be called with empty package list")
		return false
	}

	stats, err := doFetchOnlyWithFetcher(packages, cfg, registry, testLog, neverCalledFetcher)
	if err != nil {
		t.Fatalf("DoFetchOnly failed: %v", err)
	}

	if stats.Total != 0 {
		t.Errorf("Expected 0 total packages, got %d", stats.Total)
	}
	if stats.Success != 0 {
		t.Errorf("Expected 0 successful fetches, got %d", stats.Success)
	}
	if stats.Failed != 0 {
		t.Errorf("Expected 0 failed fetches, got %d", stats.Failed)
	}
}

// TestDoFetchOnly_FilteredPackages tests that packages with certain flags are skipped
func TestDoFetchOnly_FilteredPackages(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		MaxWorkers:    2,
		LogsPath:      filepath.Join(tmpDir, "logs"),
		DPortsPath:    "/usr/ports",
		DistFilesPath: filepath.Join(tmpDir, "distfiles"),
	}

	testLog, err := log.NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer testLog.Close()

	registry := pkg.NewBuildStateRegistry()

	// Create test packages
	pkg1 := &pkg.Package{PortDir: "test/pkg1", Category: "test", Name: "pkg1"}
	pkg2 := &pkg.Package{PortDir: "test/notfound", Category: "test", Name: "notfound"}
	pkg3 := &pkg.Package{PortDir: "test/corrupt", Category: "test", Name: "corrupt"}
	pkg4 := &pkg.Package{PortDir: "test/pkg4", Category: "test", Name: "pkg4"}

	// Mark some packages with filter flags
	registry.SetFlags(pkg2, pkg.PkgFNotFound)
	registry.SetFlags(pkg3, pkg.PkgFCorrupt)

	packages := []*pkg.Package{pkg1, pkg2, pkg3, pkg4}

	// Mock fetcher that always succeeds
	successFetcher := func(p *pkg.Package, cfg *config.Config) bool {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return true
	}

	stats, err := doFetchOnlyWithFetcher(packages, cfg, registry, testLog, successFetcher)
	if err != nil {
		t.Fatalf("DoFetchOnly failed: %v", err)
	}

	// Should only process pkg1 and pkg4 (2 packages)
	if stats.Total != 2 {
		t.Errorf("Expected 2 total packages (filtered out 2), got %d", stats.Total)
	}
	if stats.Success != 2 {
		t.Errorf("Expected 2 successful fetches, got %d", stats.Success)
	}
	if stats.Failed != 0 {
		t.Errorf("Expected 0 failed fetches, got %d", stats.Failed)
	}
}

// TestDoFetchOnly_WorkerPoolSize tests that worker pool size is correctly limited
func TestDoFetchOnly_WorkerPoolSize(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		MaxWorkers:    20, // Large number
		LogsPath:      filepath.Join(tmpDir, "logs"),
		DPortsPath:    "/usr/ports",
		DistFilesPath: filepath.Join(tmpDir, "distfiles"),
	}

	testLog, err := log.NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer testLog.Close()

	registry := pkg.NewBuildStateRegistry()

	// Create many packages
	packages := make([]*pkg.Package, 20)
	for i := 0; i < 20; i++ {
		packages[i] = &pkg.Package{
			PortDir:  "test/pkg",
			Category: "test",
			Name:     "pkg",
		}
	}

	// Track concurrent workers
	var concurrentWorkers int32
	var maxConcurrent int32
	var mu sync.Mutex

	concurrencyFetcher := func(p *pkg.Package, cfg *config.Config) bool {
		mu.Lock()
		concurrentWorkers++
		if concurrentWorkers > maxConcurrent {
			maxConcurrent = concurrentWorkers
		}
		mu.Unlock()

		time.Sleep(50 * time.Millisecond) // Hold the worker busy

		mu.Lock()
		concurrentWorkers--
		mu.Unlock()

		return true
	}

	_, err = doFetchOnlyWithFetcher(packages, cfg, registry, testLog, concurrencyFetcher)
	if err != nil {
		t.Fatalf("DoFetchOnly failed: %v", err)
	}

	// Worker pool should be limited to 8 (as per fetch.go:46)
	if maxConcurrent > 8 {
		t.Errorf("Expected max 8 concurrent workers, got %d", maxConcurrent)
	}

	t.Logf("Max concurrent workers observed: %d", maxConcurrent)
}

// TestDoFetchOnly_StatsThreadSafety tests that stats are updated thread-safely
func TestDoFetchOnly_StatsThreadSafety(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		MaxWorkers:    4,
		LogsPath:      filepath.Join(tmpDir, "logs"),
		DPortsPath:    "/usr/ports",
		DistFilesPath: filepath.Join(tmpDir, "distfiles"),
	}

	testLog, err := log.NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer testLog.Close()

	registry := pkg.NewBuildStateRegistry()

	// Create many packages to stress test concurrent stat updates
	numPackages := 50
	packages := make([]*pkg.Package, numPackages)
	for i := 0; i < numPackages; i++ {
		packages[i] = &pkg.Package{
			PortDir:  "test/pkg",
			Category: "test",
			Name:     "pkg",
		}
	}

	// Mock fetcher that randomly succeeds/fails
	randomFetcher := func(p *pkg.Package, cfg *config.Config) bool {
		// Use a simple deterministic "random" based on package address
		// to ensure reproducible results
		return uintptr(unsafe.Pointer(p))%2 == 0
	}

	stats, err := doFetchOnlyWithFetcher(packages, cfg, registry, testLog, randomFetcher)
	if err != nil {
		t.Fatalf("DoFetchOnly failed: %v", err)
	}

	// Verify stats consistency
	if stats.Total != numPackages {
		t.Errorf("Expected %d total packages, got %d", numPackages, stats.Total)
	}

	if stats.Success+stats.Failed != stats.Total {
		t.Errorf("Stats inconsistent: success(%d) + failed(%d) != total(%d)",
			stats.Success, stats.Failed, stats.Total)
	}
}

// ========================================================================
// Integration-style tests: Multiple workers with dependencies
// ========================================================================

// TestIntegration_MultipleWorkersWithDependencies tests realistic build scenario
// with multiple workers processing packages that have dependencies
func TestIntegration_MultipleWorkersWithDependencies(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cancel()
	defer cleanup()

	// Create a dependency chain:
	// libA <- libB <- app1
	//      <- libC <- app2
	libA := &pkg.Package{PortDir: "libs/liba", Category: "libs", Name: "liba"}
	libB := &pkg.Package{PortDir: "libs/libb", Category: "libs", Name: "libb"}
	libC := &pkg.Package{PortDir: "libs/libc", Category: "libs", Name: "libc"}
	app1 := &pkg.Package{PortDir: "apps/app1", Category: "apps", Name: "app1"}
	app2 := &pkg.Package{PortDir: "apps/app2", Category: "apps", Name: "app2"}

	// Set up dependencies (in reality these would be set during ParsePortList)
	libB.IDependOn = []*pkg.PkgLink{{Pkg: libA}}
	libC.IDependOn = []*pkg.PkgLink{{Pkg: libA}}
	app1.IDependOn = []*pkg.PkgLink{{Pkg: libB}}
	app2.IDependOn = []*pkg.PkgLink{{Pkg: libC}}

	packages := []*pkg.Package{libA, libB, libC, app1, app2}

	// Create 3 workers
	numWorkers := 3
	for i := 0; i < numWorkers; i++ {
		mockEnv := newMockEnv()
		mockEnv.executeDelay = 50 * time.Millisecond // Simulate real build time

		worker := &Worker{
			ID:     i,
			Env:    mockEnv,
			Status: "idle",
		}
		buildCtx.workers = append(buildCtx.workers, worker)

		buildCtx.wg.Add(1)
		go buildCtx.workerLoop(worker)
	}

	// Queue packages in topological order (libA first, apps last)
	// In real scenario this would be done by topological sort
	for _, p := range packages {
		buildCtx.queue <- p
	}
	close(buildCtx.queue)

	// Wait for completion
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Workers did not complete processing dependency chain")
	}

	// Verify all packages were processed
	buildCtx.statsMu.Lock()
	total := buildCtx.stats.Success + buildCtx.stats.Failed
	buildCtx.statsMu.Unlock()

	if total != len(packages) {
		t.Errorf("Expected %d packages processed, got %d", len(packages), total)
	}

	t.Logf("Processed %d packages with %d workers", total, numWorkers)
}

// TestIntegration_CancellationDuringBuild tests canceling the build while workers
// are actively processing packages
func TestIntegration_CancellationDuringBuild(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cleanup()

	// Create multiple workers with long delays
	numWorkers := 3
	for i := 0; i < numWorkers; i++ {
		mockEnv := newMockEnv()
		mockEnv.executeDelay = 200 * time.Millisecond // Longer delay

		worker := &Worker{
			ID:     i,
			Env:    mockEnv,
			Status: "idle",
		}
		buildCtx.workers = append(buildCtx.workers, worker)

		buildCtx.wg.Add(1)
		go buildCtx.workerLoop(worker)
	}

	// Queue many packages
	numPackages := 15
	for i := 0; i < numPackages; i++ {
		pkg := &pkg.Package{
			PortDir:  "test/pkg",
			Category: "test",
			Name:     "pkg",
		}
		buildCtx.queue <- pkg
	}

	// Let workers start processing
	time.Sleep(100 * time.Millisecond)

	// Cancel while workers are busy
	cancel()
	close(buildCtx.queue)

	// Wait for workers to exit
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - workers responded to cancellation
	case <-time.After(3 * time.Second):
		t.Fatal("Workers did not exit after cancellation")
	}

	// Verify not all packages were processed (due to cancellation)
	buildCtx.statsMu.Lock()
	total := buildCtx.stats.Success + buildCtx.stats.Failed
	buildCtx.statsMu.Unlock()

	if total >= numPackages {
		t.Errorf("Expected fewer than %d packages processed due to cancellation, got %d",
			numPackages, total)
	}

	t.Logf("Processed %d/%d packages before cancellation", total, numPackages)
}

// TestIntegration_WorkerLoadBalancing tests that work is distributed among workers
func TestIntegration_WorkerLoadBalancing(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cancel()
	defer cleanup()

	numWorkers := 4
	numPackages := 20

	// Track which worker processed which package (count packages, not Execute calls)
	// Since buildPackage calls Execute multiple times per package, we track unique packages
	workerPackages := make(map[int]map[*pkg.Package]bool)
	var countMu sync.Mutex

	for i := 0; i < numWorkers; i++ {
		workerID := i // Capture for closure
		workerPackages[workerID] = make(map[*pkg.Package]bool)

		mockEnv := newMockEnv()
		mockEnv.executeDelay = 20 * time.Millisecond

		// Wrap execute to track worker activity
		workerRef := (*Worker)(nil) // Will be set below
		mockEnv.executeFunc = func(ctx context.Context, cmd *environment.ExecCommand) (*environment.ExecResult, error) {
			// Track which package this worker is processing (once per package)
			if workerRef != nil {
				workerRef.mu.Lock()
				currentPkg := workerRef.Current
				workerRef.mu.Unlock()

				if currentPkg != nil {
					countMu.Lock()
					workerPackages[workerID][currentPkg] = true
					countMu.Unlock()
				}
			}

			// Simulate work and check for cancellation
			delay := mockEnv.executeDelay
			if delay > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
					// Delay completed
				}
			}

			return &environment.ExecResult{
				ExitCode: 0,
				Duration: delay,
			}, nil
		}

		worker := &Worker{
			ID:     workerID,
			Env:    mockEnv,
			Status: "idle",
		}
		workerRef = worker
		buildCtx.workers = append(buildCtx.workers, worker)

		buildCtx.wg.Add(1)
		go buildCtx.workerLoop(worker)
	}

	// Queue packages
	for i := 0; i < numPackages; i++ {
		pkg := &pkg.Package{
			PortDir:  fmt.Sprintf("test/pkg%d", i), // Unique packages
			Category: "test",
			Name:     fmt.Sprintf("pkg%d", i),
		}
		buildCtx.queue <- pkg
	}
	close(buildCtx.queue)

	// Wait for completion
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Workers did not complete processing")
	}

	// Verify work distribution
	countMu.Lock()
	defer countMu.Unlock()

	workerCounts := make(map[int]int)
	for workerID, pkgs := range workerPackages {
		workerCounts[workerID] = len(pkgs)
		t.Logf("Worker %d processed %d packages", workerID, len(pkgs))
	}

	// Each worker should have processed some packages
	for i := 0; i < numWorkers; i++ {
		if workerCounts[i] == 0 {
			t.Errorf("Worker %d did not process any packages", i)
		}
	}

	// Check reasonable distribution (no worker should do more than 75% of work)
	maxExpected := int(float64(numPackages) * 0.75)
	for workerID, count := range workerCounts {
		if count > maxExpected {
			t.Errorf("Worker %d processed too many packages (%d/%d), poor load balancing",
				workerID, count, numPackages)
		}
	}
}

// TestIntegration_MixedSuccessFailure tests handling of both successful and failed builds
func TestIntegration_MixedSuccessFailure(t *testing.T) {
	buildCtx, cancel, cleanup := setupTestBuildContext(t)
	defer cancel()
	defer cleanup()

	// Create packages that will succeed and fail
	pkgSuccess1 := &pkg.Package{PortDir: "test/success1", Category: "test", Name: "success1"}
	pkgFail1 := &pkg.Package{PortDir: "test/fail1", Category: "test", Name: "fail1"}
	pkgSuccess2 := &pkg.Package{PortDir: "test/success2", Category: "test", Name: "success2"}
	pkgFail2 := &pkg.Package{PortDir: "test/fail2", Category: "test", Name: "fail2"}

	packages := []*pkg.Package{pkgSuccess1, pkgFail1, pkgSuccess2, pkgFail2}

	// Track which packages should fail
	failPackages := map[*pkg.Package]bool{
		pkgFail1: true,
		pkgFail2: true,
	}

	// Create workers
	numWorkers := 2
	for i := 0; i < numWorkers; i++ {
		mockEnv := newMockEnv()
		mockEnv.executeDelay = 30 * time.Millisecond

		// Make execute fail for packages marked as "fail"
		// Need to check worker.Current since package info isn't in cmd
		workerRef := (*Worker)(nil) // Will be set below
		mockEnv.executeFunc = func(ctx context.Context, cmd *environment.ExecCommand) (*environment.ExecResult, error) {
			// Simulate work and check for cancellation
			delay := mockEnv.executeDelay
			if delay > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
					// Delay completed
				}
			}

			// Check if current package should fail
			if workerRef != nil {
				workerRef.mu.Lock()
				currentPkg := workerRef.Current
				workerRef.mu.Unlock()

				if failPackages[currentPkg] {
					return &environment.ExecResult{
						ExitCode: 1,
						Duration: delay,
					}, fmt.Errorf("build failed")
				}
			}

			return &environment.ExecResult{
				ExitCode: 0,
				Duration: delay,
			}, nil
		}

		worker := &Worker{
			ID:     i,
			Env:    mockEnv,
			Status: "idle",
		}
		workerRef = worker // Set reference for closure
		buildCtx.workers = append(buildCtx.workers, worker)

		buildCtx.wg.Add(1)
		go buildCtx.workerLoop(worker)
	}

	// Queue packages
	for _, p := range packages {
		buildCtx.queue <- p
	}
	close(buildCtx.queue)

	// Wait for completion
	done := make(chan struct{})
	go func() {
		buildCtx.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(3 * time.Second):
		t.Fatal("Workers did not complete processing")
	}

	// Verify stats
	buildCtx.statsMu.Lock()
	defer buildCtx.statsMu.Unlock()

	if buildCtx.stats.Success != 2 {
		t.Errorf("Expected 2 successful builds, got %d", buildCtx.stats.Success)
	}

	if buildCtx.stats.Failed != 2 {
		t.Errorf("Expected 2 failed builds, got %d", buildCtx.stats.Failed)
	}

	total := buildCtx.stats.Success + buildCtx.stats.Failed
	if total != len(packages) {
		t.Errorf("Expected %d total packages processed, got %d", len(packages), total)
	}
}
