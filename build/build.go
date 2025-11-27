package build

import (
	"fmt"
	"os"
	"sync"
	"time"

	"dsynth/config"
	"dsynth/log"
	"dsynth/mount"
	"dsynth/pkg"
)

// BuildStats tracks build statistics
type BuildStats struct {
	Total    int
	Success  int
	Failed   int
	Skipped  int
	Ignored  int
	Duration time.Duration
}

// Worker represents a build worker
type Worker struct {
	ID        int
	Mount     *mount.Worker
	Current   *pkg.Package
	Status    string
	StartTime time.Time
	mu        sync.Mutex
}

// BuildContext holds the build orchestration state
type BuildContext struct {
	cfg       *config.Config
	logger    *log.Logger
	registry  *pkg.BuildStateRegistry
	workers   []*Worker
	queue     chan *pkg.Package
	stats     BuildStats
	statsMu   sync.Mutex
	startTime time.Time
	wg        sync.WaitGroup
}

// DoBuild executes the main build process
// Returns stats, cleanup function, and error
func DoBuild(packages []*pkg.Package, cfg *config.Config, logger *log.Logger) (*BuildStats, func(), error) {
	// Get build order (topological sort)
	buildOrder := pkg.GetBuildOrder(packages)

	ctx := &BuildContext{
		cfg:       cfg,
		logger:    logger,
		registry:  pkg.NewBuildStateRegistry(),
		queue:     make(chan *pkg.Package, 100),
		startTime: time.Now(),
	}

	// Create cleanup function
	cleanup := func() {
		fmt.Fprintf(os.Stderr, "Cleaning up worker mounts...\n")
		for i, worker := range ctx.workers {
			if worker != nil {
				if err := mount.DoWorkerUnmounts(worker.Mount, cfg); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to unmount worker %d: %v\n", i, err)
				}
			}
		}
	}

	// Count packages that need building
	for _, p := range buildOrder {
		if ctx.registry.HasAnyFlags(p, pkg.PkgFSuccess|pkg.PkgFNoBuildIgnore|pkg.PkgFIgnored) {
			if ctx.registry.HasFlags(p, pkg.PkgFSuccess) {
				ctx.stats.Skipped++
			} else if ctx.registry.HasFlags(p, pkg.PkgFIgnored) {
				ctx.stats.Ignored++
			}
		} else {
			ctx.stats.Total++
		}
	}

	fmt.Printf("\nStarting build: %d packages (%d skipped, %d ignored)\n",
		ctx.stats.Total, ctx.stats.Skipped, ctx.stats.Ignored)

	// Create workers
	numWorkers := cfg.MaxWorkers
	if cfg.SlowStart > 0 && cfg.SlowStart < numWorkers {
		numWorkers = cfg.SlowStart
	}

	ctx.workers = make([]*Worker, numWorkers)
	for i := 0; i < numWorkers; i++ {
		ctx.workers[i] = &Worker{
			ID:     i,
			Status: "idle",
			Mount: &mount.Worker{
				Index:   i,
				BaseDir: fmt.Sprintf("%s/SL%02d", cfg.BuildBase, i),
			},
		}

		// Setup mounts for each worker
		if err := mount.DoWorkerMounts(ctx.workers[i].Mount, cfg); err != nil {
			logger.Error(fmt.Sprintf("Failed to setup mounts for worker %d: %v", i, err))
			cleanup() // Cleanup any workers we did create
			return nil, cleanup, fmt.Errorf("worker %d mount failed: %w", i, err)
		}

		// Start worker goroutine
		ctx.wg.Add(1)
		go ctx.workerLoop(ctx.workers[i])
	}

	// Queue packages in build order
	go func() {
		for _, p := range buildOrder {
			// Skip packages that don't need building
			if ctx.registry.HasAnyFlags(p, pkg.PkgFSuccess|pkg.PkgFNoBuildIgnore|pkg.PkgFIgnored) {
				continue
			}

			// Wait for dependencies
			if !ctx.waitForDependencies(p) {
				// Dependency failed, mark as skipped
				ctx.registry.AddFlags(p, pkg.PkgFSkipped)
				ctx.statsMu.Lock()
				ctx.stats.Skipped++
				ctx.statsMu.Unlock()
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

	// Don't call cleanup here - let the caller do it
	// This allows proper cleanup on signals
	return &ctx.stats, cleanup, nil
}

// workerLoop is the main loop for a build worker
func (ctx *BuildContext) workerLoop(worker *Worker) {
	defer ctx.wg.Done()

	for p := range ctx.queue {
		worker.mu.Lock()
		worker.Current = p
		worker.Status = "building"
		worker.StartTime = time.Now()
		worker.mu.Unlock()

		// Mark as running
		ctx.registry.AddFlags(p, pkg.PkgFRunning)

		// Build the package
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
		worker.Current = nil
		worker.Status = "idle"
		worker.mu.Unlock()

		// Print progress
		ctx.printProgress()
	}
}

// buildPackage builds a single package
func (ctx *BuildContext) buildPackage(worker *Worker, p *pkg.Package) bool {
	pkgLogger := log.NewPackageLogger(ctx.cfg, p.PortDir)
	defer pkgLogger.Close()

	pkgLogger.WriteHeader()

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

	startTime := time.Now()

	for _, phase := range phases {
		ctx.registry.SetLastPhase(p, phase)
		pkgLogger.WritePhase(phase)

		if err := executePhase(worker, p, phase, ctx.cfg, ctx.registry, pkgLogger); err != nil {
			duration := time.Since(startTime)
			pkgLogger.WriteFailure(duration, fmt.Sprintf("Phase %s failed: %v", phase, err))
			return false
		}
	}

	duration := time.Since(startTime)
	pkgLogger.WriteSuccess(duration)

	// TODO: Update CRC database after successful build
	// This requires:
	//   1. Add buildDB *builddb.DB to BuildContext
	//   2. Pass buildDB to DoBuild() function
	//   3. Compute CRC: builddb.ComputePortCRC(portPath)
	//   4. Update CRC: ctx.buildDB.UpdateCRC(p.PortDir, crc)
	//   5. Update package index: ctx.buildDB.UpdatePackageIndex(p.PortDir, p.Version, buildUUID)
	// This will be implemented in a future task (Task 6D or Task 7)

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

// printProgress prints current build progress
func (ctx *BuildContext) printProgress() {
	ctx.statsMu.Lock()
	defer ctx.statsMu.Unlock()

	elapsed := time.Since(ctx.startTime)
	done := ctx.stats.Success + ctx.stats.Failed

	fmt.Printf("\r[%s] Progress: %d/%d (S:%d F:%d) %s elapsed",
		time.Now().Format("15:04:05"),
		done, ctx.stats.Total,
		ctx.stats.Success, ctx.stats.Failed,
		formatDuration(elapsed))
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
