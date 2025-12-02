// Package build provides parallel port building orchestration with CRC-based
// incremental builds. It manages worker pools, dependency ordering, and build
// lifecycle tracking through an embedded bbolt database.
//
// The build system automatically skips unchanged ports by computing CRC32
// checksums of port directories and comparing them with stored values from
// previous successful builds.
//
// # Build Workflow
//
// 1. Parse port specifications and resolve dependencies
// 2. Compute topological build order
// 3. For each port:
//   - Compute CRC32 of port directory
//   - Check if port needs building (NeedsBuild)
//   - Skip if CRC matches last successful build
//   - Otherwise, build and update CRC on success
//
// 4. Track all builds with UUIDs, status, and timestamps
//
// # Basic Usage
//
//	cfg, _ := config.LoadConfig("", "default")
//	logger, _ := log.NewLogger(cfg)
//	db, _ := builddb.OpenDB("~/.go-synth/builds.db")
//	defer db.Close()
//
//	pkgRegistry := pkg.NewPackageRegistry()
//	stateRegistry := pkg.NewBuildStateRegistry()
//	packages, _ := pkg.ParsePortList([]string{"editors/vim"}, cfg, stateRegistry, pkgRegistry)
//	pkg.ResolveDependencies(packages, cfg, stateRegistry, pkgRegistry)
//
//	stats, cleanup, _ := DoBuild(packages, cfg, logger, db)
//	defer cleanup()
//
//	fmt.Printf("Success: %d, Skipped: %d\n", stats.Success, stats.Skipped)
//
// # Incremental Builds
//
// The build system uses CRC-based change detection to skip unchanged ports:
//
//	First build:  editors/vim -> builds (no CRC stored)
//	Second build: editors/vim -> skipped (CRC match)
//	After edit:   editors/vim -> rebuilds (CRC mismatch)
//
// # Build Records
//
// Every build creates a record in the database with:
//   - Unique UUID for tracking
//   - Status: "running" → "success" or "failed"
//   - Timestamps: StartTime and EndTime
//   - Port directory and version
//
// Query build history:
//
//	rec, _ := db.LatestFor("editors/vim", "9.0.0")
//	fmt.Printf("Last build: %s at %s\n", rec.UUID, rec.StartTime)
package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-synth/builddb"
	"go-synth/config"
	"go-synth/environment"
	"go-synth/log"
	"go-synth/pkg"

	"github.com/google/uuid"
	"golang.org/x/term"
)

// BuildStats tracks build statistics
type BuildStats struct {
	Total      int
	Success    int
	Failed     int
	Skipped    int // Skipped due to dependency failures (runtime)
	SkippedPre int // Skipped because already built (CRC match, pre-queue)
	Ignored    int
	Duration   time.Duration
}

// Worker represents a build worker
type Worker struct {
	ID        int
	Env       environment.Environment // Environment for isolated execution
	Current   *pkg.Package
	Status    string
	StartTime time.Time
	mu        sync.Mutex
}

// BuildContext holds the build orchestration state.
// It manages worker pools, dependency tracking, and integrates with builddb
// for CRC-based incremental builds and build record lifecycle tracking.
//
// The context field supports cancellation for graceful shutdown when signals
// are received (SIGINT, SIGTERM). Workers check ctx.Done() to exit cleanly.
type BuildContext struct {
	ctx       context.Context
	cancel    context.CancelFunc // Cancel function for stopping build gracefully
	cfg       *config.Config
	logger    *log.Logger
	registry  *pkg.BuildStateRegistry
	buildDB   *builddb.DB
	workers   []*Worker
	queue     chan *pkg.Package
	stats     BuildStats
	statsMu   sync.Mutex
	startTime time.Time
	wg        sync.WaitGroup

	runID    string
	outputMu sync.Mutex
	ui       BuildUI // UI for progress and event display
}

func ensureBuildBaseInitialized(cfg *config.Config) error {
	required := []struct {
		name string
		path string
	}{
		{"Template", filepath.Join(cfg.BuildBase, "Template")},
		{"options", cfg.OptionsPath},
		{"distfiles", cfg.DistFilesPath},
		{"packages", cfg.PackagesPath},
		{"logs", cfg.LogsPath},
	}

	missing := make([]string, 0)
	for _, entry := range required {
		if entry.path == "" {
			continue
		}
		if _, err := os.Stat(entry.path); err != nil {
			missing = append(missing, fmt.Sprintf("%s (%s)", entry.name, entry.path))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("build base missing required directories: %s. Run 'go-synth init' to recreate the build base.", strings.Join(missing, ", "))
	}

	return nil
}

// DoBuild executes the main build process with CRC-based incremental builds.
//
// For each package in the build order:
//   - Computes CRC32 of port directory
//   - Checks if rebuild is needed (CRC comparison)
//   - Skips unchanged ports (increments stats.Skipped)
//   - Builds changed ports with full lifecycle tracking
//
// Returns build statistics, cleanup function, and error.
// The cleanup function must be called to unmount worker filesystems.
//
// The onCleanupReady callback (if provided) is called immediately when the cleanup
// function is created, before workers are started. This allows signal handlers to
// access the cleanup function as soon as workers exist.
//
// Build lifecycle for each port:
//  1. Generate UUID
//  2. SaveRecord with status="running"
//  3. Execute build phases
//  4. UpdateRecordStatus to "success" or "failed"
//  5. Update CRC and package index (on success only)
func DoBuild(packages []*pkg.Package, cfg *config.Config, logger *log.Logger, buildDB *builddb.DB, registry *pkg.BuildStateRegistry, onCleanupReady func(func()), runID string) (*BuildStats, func(), error) {
	// Get build order (topological sort)
	buildOrder := pkg.GetBuildOrder(packages, logger)

	// Create cancellable context for graceful shutdown
	// When cancelled (e.g., via signal handler), workers will exit their loops
	buildCtx, cancel := context.WithCancel(context.Background())

	// Use provided registry if given, otherwise create new one
	if registry == nil {
		registry = pkg.NewBuildStateRegistry()
	}

	ctx := &BuildContext{
		ctx:       buildCtx,
		cancel:    cancel,
		cfg:       cfg,
		logger:    logger,
		registry:  registry,
		buildDB:   buildDB,
		queue:     make(chan *pkg.Package, 100),
		startTime: time.Now(),
		runID:     runID,
	}

	// Initialize UI based on configuration and TTY detection
	// Use ncurses UI by default if stdout is a TTY and not disabled via -S flag
	useNcurses := !cfg.DisableUI && term.IsTerminal(int(os.Stdout.Fd()))
	if useNcurses {
		ctx.ui = NewNcursesUI()
	} else {
		ctx.ui = NewStdoutUI()
	}

	// Create cleanup function that will access ctx.workers when called
	// Note: ctx.workers will be populated after this function is created,
	// but the closure captures the ctx pointer, so it will see the workers
	// when cleanup is actually invoked
	//
	// Cleanup flow:
	//  1. Cancel context to signal workers to stop
	//  2. Wait for workers to exit their loops (respects ongoing work)
	//  3. Cleanup worker environments (unmount, remove directories)
	cleanup := func() {
		logger.Debug("Cleanup started")
		logger.Info("Stopping build workers...")

		// Cancel context to signal all workers to stop
		if ctx.cancel != nil {
			ctx.cancel()
			logger.Debug("Context cancelled")
		}

		logger.Debug("Waiting for workers (count=%d)...", len(ctx.workers))
		logger.Info("Waiting for workers to finish current operations...")
		// Wait for all worker goroutines to exit with timeout
		// Workers will see ctx.Done() and exit gracefully
		done := make(chan struct{})
		go func() {
			ctx.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logger.Debug("Workers exited gracefully")
			logger.Info("All workers exited gracefully")
		case <-time.After(5 * time.Second):
			logger.Debug("Worker timeout (5s), proceeding anyway")
			logger.Warn("Timeout waiting for workers to exit (5s), proceeding with cleanup anyway")
		}

		// Stop UI before cleaning up environments
		if ctx.ui != nil {
			ctx.ui.Stop()
		}

		logger.Debug("Starting environment cleanup")
		logger.Info("Cleaning up worker environments (total workers: %d)", len(ctx.workers))
		for i, worker := range ctx.workers {
			if worker != nil && worker.Env != nil {
				logger.Info("Cleaning up worker %d", i)
				if err := worker.Env.Cleanup(); err != nil {
					logger.Warn("Failed to cleanup worker %d: %v", i, err)
				}
			} else {
				logger.Warn("Worker %d is nil or has no environment", i)
			}
		}
		logger.Info("Cleanup complete")
	}

	// Notify caller that cleanup function is ready
	// The cleanup function captures ctx pointer, so it will see workers when they're created
	if onCleanupReady != nil {
		onCleanupReady(cleanup)
	}

	// Count all packages in the build order
	for _, p := range buildOrder {
		ctx.stats.Total++
		if ctx.registry.HasAnyFlags(p, pkg.PkgFSuccess|pkg.PkgFNoBuildIgnore|pkg.PkgFIgnored) {
			if ctx.registry.HasFlags(p, pkg.PkgFSuccess) {
				// Already built (CRC match from MarkPackagesNeedingBuild)
				ctx.stats.SkippedPre++
			} else if ctx.registry.HasFlags(p, pkg.PkgFIgnored) {
				ctx.stats.Ignored++
				now := time.Now()
				ctx.recordRunPackage(p, builddb.RunStatusIgnored, -1, now, now, "")
			}
		}
	}

	logger.Info("Starting build: %d packages (%d already built, %d ignored)",
		ctx.stats.Total, ctx.stats.SkippedPre, ctx.stats.Ignored)

	if err := ensureBuildBaseInitialized(cfg); err != nil {
		cancel()
		return nil, cleanup, err
	}

	// Bootstrap ports-mgmt/pkg before starting workers
	// This must succeed before any workers are created
	logger.Info("Checking for ports-mgmt/pkg bootstrap requirement...")
	pkgStatus, err := bootstrapPkg(buildCtx, buildOrder, cfg, logger, buildDB, ctx.registry, onCleanupReady, ctx.runID)
	if err != nil {
		cancel() // Cancel context
		return nil, cleanup, fmt.Errorf("pkg bootstrap failed: %w", err)
	}
	if pkgStatus != "" {
		// Note: Don't increment Total here - pkg is already counted in buildOrder loop above
		// Also note: ports-mgmt/pkg might have been counted in SkippedPre during the initial
		// buildOrder loop (lines 258-270) if it had PkgFSuccess flag. We need to adjust.
		// Find if pkg was pre-counted
		var pkgPkg *pkg.Package
		for _, p := range buildOrder {
			if p.PortDir == "ports-mgmt/pkg" {
				pkgPkg = p
				break
			}
		}
		wasPreSkipped := pkgPkg != nil && ctx.registry.HasFlags(pkgPkg, pkg.PkgFSuccess)

		switch pkgStatus {
		case builddb.RunStatusSuccess:
			// If pkg was already counted in SkippedPre, remove that count
			if wasPreSkipped {
				ctx.stats.SkippedPre--
			}
			ctx.stats.Success++
		case builddb.RunStatusSkipped:
			// Bootstrap pkg was already built (CRC match or template check)
			// Only increment if not already counted in the initial loop
			if !wasPreSkipped {
				ctx.stats.SkippedPre++
			}
		case builddb.RunStatusFailed:
			// If pkg was already counted in SkippedPre, remove that count
			if wasPreSkipped {
				ctx.stats.SkippedPre--
			}
			ctx.stats.Failed++
		}
	}

	// Create workers
	numWorkers := cfg.MaxWorkers
	if cfg.SlowStart > 0 && cfg.SlowStart < numWorkers {
		numWorkers = cfg.SlowStart
	}

	ctx.workers = make([]*Worker, numWorkers)
	for i := 0; i < numWorkers; i++ {
		// Create isolated environment for this worker
		env, err := environment.New("bsd")
		if err != nil {
			logger.Error(fmt.Sprintf("Worker %d: failed to create environment: %v", i, err))
			cleanup()
			return nil, cleanup, fmt.Errorf("worker %d environment creation failed: %w", i, err)
		}

		// Setup environment (mounts, directories, etc.)
		if err := env.Setup(i, cfg, logger); err != nil {
			logger.Error(fmt.Sprintf("Worker %d: environment setup failed: %v", i, err))
			cleanup()
			return nil, cleanup, fmt.Errorf("worker %d environment setup failed: %w", i, err)
		}

		ctx.workers[i] = &Worker{
			ID:     i,
			Env:    env,
			Status: "idle",
		}

		// Start worker goroutine
		ctx.wg.Add(1)
		go ctx.workerLoop(ctx.workers[i])
	}

	// Start UI after workers are created
	if err := ctx.ui.Start(); err != nil {
		logger.Warn("Failed to start UI: %v", err)
	}

	// Queue packages in build order
	go func() {
		for _, p := range buildOrder {
			// Skip packages that don't need building
			if ctx.registry.HasAnyFlags(p, pkg.PkgFSuccess|pkg.PkgFNoBuildIgnore|pkg.PkgFIgnored) {
				continue
			}

			// Skip ports-mgmt/pkg - already built in bootstrap phase
			if ctx.registry.HasFlags(p, pkg.PkgFPkgPkg) {
				continue
			}

			// Note: CRC-based incremental build check is already done in
			// MarkPackagesNeedingBuild() before we reach here. Packages that
			// don't need building are marked with PkgFSuccess and filtered
			// out at line 337 above. No need to check again here.

			// Wait for dependencies
			if !ctx.waitForDependencies(p) {
				// Dependency failed, mark as skipped
				ctx.registry.AddFlags(p, pkg.PkgFSkipped)
				ctx.statsMu.Lock()
				ctx.stats.Skipped++
				ctx.statsMu.Unlock()
				now := time.Now()
				ctx.recordRunPackage(p, builddb.RunStatusSkipped, -1, now, now, "")
				logger.Skipped(p.PortDir)
				continue
			}

			ctx.queue <- p
		}
		close(ctx.queue)
	}()

	// Wait for all workers to finish
	ctx.wg.Wait()

	// Calculate duration
	ctx.stats.Duration = time.Since(ctx.startTime)

	// Stop UI after all workers finish
	if ctx.ui != nil {
		ctx.ui.Stop()
	}

	// Don't call cleanup here - let the caller do it
	// This allows proper cleanup on signals
	return &ctx.stats, cleanup, nil
}

// workerLoop is the main loop for a build worker.
//
// The loop processes packages from the queue until either:
//   - The queue is closed (normal shutdown)
//   - The context is cancelled (signal-triggered shutdown)
//
// When context is cancelled, the worker exits immediately without processing
// more packages. Any ongoing buildPackage() call will be interrupted via
// context propagation to env.Execute().
func (ctx *BuildContext) workerLoop(worker *Worker) {
	defer ctx.wg.Done()

	for {
		// Check for cancellation before blocking on channel
		// This ensures workers exit promptly when signaled
		select {
		case <-ctx.ctx.Done():
			// Context cancelled (SIGINT, SIGTERM, etc.)
			ctx.logger.Info("Worker %d: stopping due to context cancellation", worker.ID)
			return

		case p, ok := <-ctx.queue:
			if !ok {
				// Channel closed, normal shutdown
				ctx.logger.Debug("Worker %d: queue closed, exiting", worker.ID)
				return
			}

			worker.mu.Lock()
			worker.Current = p
			worker.Status = "building"
			worker.StartTime = time.Now()
			startTime := worker.StartTime
			worker.mu.Unlock()

			// Mark as running
			ctx.registry.AddFlags(p, pkg.PkgFRunning)

			ctx.recordRunPackage(p, builddb.RunStatusRunning, worker.ID, startTime, time.Time{}, "")
			ctx.logWorkerEvent(worker.ID, fmt.Sprintf("start build: %s (deps: %d)", p.PortDir, len(p.IDependOn)))

			// Build the package (context will propagate to env.Execute())
			success := ctx.buildPackage(worker, p)

			// Update stats
			ctx.statsMu.Lock()
			if success {
				ctx.stats.Success++
				ctx.registry.AddFlags(p, pkg.PkgFSuccess)
				ctx.registry.ClearFlags(p, pkg.PkgFRunning)
				ctx.logger.Success(p.PortDir)
			} else {
				ctx.stats.Failed++
				ctx.registry.AddFlags(p, pkg.PkgFFailed)
				ctx.registry.ClearFlags(p, pkg.PkgFRunning)
				ctx.logger.Failed(p.PortDir, ctx.registry.GetLastPhase(p))
			}
			ctx.statsMu.Unlock()

			worker.mu.Lock()
			endTime := time.Now()
			duration := endTime.Sub(worker.StartTime)
			worker.Current = nil
			worker.Status = "idle"
			worker.mu.Unlock()

			if success {
				ctx.recordRunPackage(p, builddb.RunStatusSuccess, worker.ID, startTime, endTime, "")
				ctx.logWorkerEvent(worker.ID, fmt.Sprintf("build success: %s (%s)", p.PortDir, formatDuration(duration)))
			} else {
				lastPhase := ctx.registry.GetLastPhase(p)
				ctx.recordRunPackage(p, builddb.RunStatusFailed, worker.ID, startTime, endTime, lastPhase)
				ctx.logWorkerEvent(worker.ID, fmt.Sprintf("build failed: %s (phase: %s)", p.PortDir, lastPhase))
			}

			// Print progress
			ctx.printProgress()
		}
	}
}

// buildPackage builds a single package with full lifecycle tracking.
//
// Lifecycle:
//  1. Generate build UUID
//  2. Create build record (status="running")
//  3. Execute all build phases sequentially
//  4. Update record status to "success" or "failed"
//  5. On success: update CRC and package index
//
// Database operations are fail-safe - errors are logged but don't fail the build.
func (ctx *BuildContext) buildPackage(worker *Worker, p *pkg.Package) bool {
	pkgLogger := log.NewPackageLogger(ctx.cfg, p.PortDir)
	defer pkgLogger.Close()

	pkgLogger.WriteHeader()

	// Generate UUID for this build attempt
	p.BuildUUID = uuid.New().String()

	// Create context logger with UUID and worker info
	ctxLogger := ctx.logger.WithContext(log.LogContext{
		BuildID:  p.BuildUUID,
		PortDir:  p.PortDir,
		WorkerID: worker.ID,
	})

	ctxLogger.Info("Starting build")

	startTime := time.Now()

	buildRecord := &builddb.BuildRecord{
		UUID:      p.BuildUUID,
		PortDir:   p.PortDir,
		Version:   p.Version,
		Status:    "running",
		StartTime: startTime,
	}
	if err := ctx.buildDB.SaveRecord(buildRecord); err != nil {
		ctxLogger.Warn("Failed to save build record: %v", err)
	}

	// Execute all build phases
	phases := []string{
		"install-pkgs",
		"check-sanity",
		"fetch-depends",
		"fetch",
		"checksum",
		"extract-depends",
		"extract",
		"patch-depends",
		"patch",
		"build-depends",
		"lib-depends",
		"configure",
		"build",
		"run-depends",
		"stage",
		"check-plist",
		"package",
	}

	for _, phase := range phases {
		ctx.registry.SetLastPhase(p, phase)
		pkgLogger.WritePhase(phase)
		ctxLogger.Info("Starting phase: %s", phase)

		if err := executePhase(ctx.ctx, worker, p, phase, ctx.cfg, ctx.registry, pkgLogger); err != nil {
			duration := time.Since(startTime)
			pkgLogger.WriteFailure(duration, fmt.Sprintf("Phase %s failed: %v", phase, err))
			ctxLogger.Failed(phase, fmt.Sprintf("%v", err))

			if err := ctx.buildDB.UpdateRecordStatus(p.BuildUUID, "failed", time.Now()); err != nil {
				ctxLogger.Warn("Failed to update build record status: %v", err)
			}

			return false
		}
	}

	duration := time.Since(startTime)
	pkgLogger.WriteSuccess(duration)
	ctxLogger.Success(fmt.Sprintf("Build completed in %v", duration))

	// Update build record status to success
	if err := ctx.buildDB.UpdateRecordStatus(p.BuildUUID, "success", time.Now()); err != nil {
		ctxLogger.Warn("Failed to update build record status: %v", err)
	}

	// Update CRC database after successful build
	portPath := filepath.Join(ctx.cfg.DPortsPath, p.Category, p.Name)
	crc, err := builddb.ComputePortCRC(portPath)
	if err != nil {
		// Log warning but don't fail the build (CRC update is non-fatal)
		ctxLogger.Warn("Failed to compute CRC: %v", err)
	} else {
		if err := ctx.buildDB.UpdateCRC(p.PortDir, crc); err != nil {
			ctxLogger.Warn("Failed to update CRC: %v", err)
		}
	}

	if err := ctx.buildDB.UpdatePackageIndex(p.PortDir, p.Version, p.BuildUUID); err != nil {
		ctxLogger.Warn("Failed to update package index: %v", err)
	}

	return true
}

// waitForDependencies waits for all dependencies to complete
func (ctx *BuildContext) waitForDependencies(p *pkg.Package) bool {
	for {
		allDone := true
		anyFailed := false

		for _, link := range p.IDependOn {
			dep := link.Pkg

			if ctx.registry.HasFlags(dep, pkg.PkgFSuccess) {
				// Dependency succeeded
				continue
			}

			if ctx.registry.HasFlags(dep, pkg.PkgFFailed) {
				// Dependency failed
				anyFailed = true
				break
			}

			if ctx.registry.HasFlags(dep, pkg.PkgFSkipped) {
				// Dependency skipped
				anyFailed = true
				break
			}

			// Still running or not started
			allDone = false
		}

		if anyFailed {
			return false
		}

		if allDone {
			return true
		}

		// Wait a bit before checking again
		time.Sleep(100 * time.Millisecond)
	}
}

// printProgress updates UI with current build progress
func (ctx *BuildContext) printProgress() {
	ctx.statsMu.Lock()
	stats := ctx.stats // Copy stats to avoid holding lock during UI update
	elapsed := time.Since(ctx.startTime)
	ctx.statsMu.Unlock()

	if ctx.ui != nil {
		ctx.ui.UpdateProgress(stats, formatDuration(elapsed))
	}
}

func (ctx *BuildContext) logWorkerEvent(workerID int, message string) {
	if ctx.ui != nil {
		ctx.ui.LogEvent(workerID, message)
	}
}

func (ctx *BuildContext) recordRunPackage(p *pkg.Package, status string, workerID int, start, end time.Time, lastPhase string) {
	if ctx.runID == "" {
		return
	}

	rec := &builddb.RunPackageRecord{
		PortDir:   p.PortDir,
		Version:   p.Version,
		Status:    status,
		StartTime: start,
		EndTime:   end,
		WorkerID:  workerID,
		LastPhase: lastPhase,
	}

	if err := ctx.buildDB.PutRunPackage(ctx.runID, rec); err != nil {
		ctx.logger.Warn("Failed to record package %s for run %s: %v", p.PortDir, ctx.runID, err)
	}
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
