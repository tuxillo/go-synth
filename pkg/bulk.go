package pkg

import (
	"sync"

	"go-synth/config"
)

// BulkQueue implements a worker pool for parallel package information fetching.
// It's an internal implementation detail used by ParsePortList and
// ResolveDependencies to efficiently query port Makefiles in parallel.
//
// The queue maintains a fixed number of worker goroutines that process
// package queries concurrently. Work items are submitted via Queue() and
// results are retrieved via GetResult(). The queue must be closed with
// Close() when done to avoid goroutine leaks.
//
// # Concurrency
//
// BulkQueue is thread-safe. Multiple goroutines can call Queue() concurrently,
// and a single goroutine should call GetResult() to collect results.
//
// # Typical Usage
//
//	bq := newBulkQueue(cfg, cfg.MaxWorkers)
//	defer bq.Close()
//
//	// Queue work items
//	bq.Queue("editors", "vim", "", "")
//	bq.Queue("shells", "bash", "", "")
//
//	// Collect results
//	for bq.Pending() > 0 {
//	    pkg, initialFlags, parseFlags, ignoreReason, err := bq.GetResult()
//	    if err != nil {
//	        // handle error
//	    }
//	    // process pkg
//	}
type BulkQueue struct {
	cfg        *config.Config
	maxBulk    int
	workChan   chan *bulkWork
	resultChan chan *bulkResult
	wg         sync.WaitGroup
	mu         sync.Mutex
	active     int
}

type bulkWork struct {
	category string
	name     string
	flavor   string
	flags    string // "x" = not manual, "d" = debug stop
}

type bulkResult struct {
	pkg          *Package
	err          error
	initialFlags PackageFlags // Flags from manual selection, debug mode, etc.
	parseFlags   PackageFlags // Flags from queryMakefile (PkgFIgnored, PkgFMeta, PkgFCorrupt)
	ignoreReason string       // Ignore reason from queryMakefile
}

// newBulkQueue creates a new BulkQueue with the specified number of worker
// goroutines. If maxBulk is <= 0, uses cfg.MaxWorkers as the default.
//
// The workers are started immediately and begin processing items as soon
// as they are queued.
//
// Parameters:
//   - cfg: configuration for accessing port Makefiles
//   - maxBulk: maximum number of concurrent workers (0 = use cfg.MaxWorkers)
//
// Returns a BulkQueue ready to accept work items.
func newBulkQueue(cfg *config.Config, maxBulk int) *BulkQueue {
	if maxBulk <= 0 {
		maxBulk = cfg.MaxWorkers
	}

	bq := &BulkQueue{
		cfg:        cfg,
		maxBulk:    maxBulk,
		workChan:   make(chan *bulkWork, 1000),
		resultChan: make(chan *bulkResult, 1000),
	}

	// Start worker goroutines
	for i := 0; i < maxBulk; i++ {
		bq.wg.Add(1)
		go bq.worker()
	}

	return bq
}

func (bq *BulkQueue) worker() {
	defer bq.wg.Done()

	for work := range bq.workChan {
		pkg, parseFlags, ignoreReason, err := getPackageInfo(work.category, work.name, work.flavor, bq.cfg)

		var initialFlags PackageFlags
		if err == nil {
			// Add selection flags
			if work.flags != "x" {
				initialFlags |= PkgFManualSel
			}
			if work.flags == "d" {
				initialFlags |= PkgFManualSel // Debug stop mode
			}
		}

		bq.resultChan <- &bulkResult{
			pkg:          pkg,
			err:          err,
			initialFlags: initialFlags,
			parseFlags:   parseFlags,
			ignoreReason: ignoreReason,
		}
	}
}

// Queue adds a package to the work queue for parallel processing.
// The package is identified by category, name, and flavor, and will be
// processed by one of the worker goroutines.
//
// This method is thread-safe and can be called concurrently.
//
// Parameters:
//   - category: port category (e.g., "editors")
//   - name: port name (e.g., "vim")
//   - flavor: port flavor (e.g., "" or "python")
//   - flags: selection flags ("" = manually selected, "x" = dependency, "d" = debug)
func (bq *BulkQueue) Queue(category, name, flavor, flags string) {
	bq.mu.Lock()
	bq.active++
	bq.mu.Unlock()

	bq.workChan <- &bulkWork{
		category: category,
		name:     name,
		flavor:   flavor,
		flags:    flags,
	}
}

// GetResult retrieves one result from the queue, blocking until a result
// is available. This method should be called repeatedly until Pending()
// returns 0 to collect all results.
//
// Returns:
//   - pkg: the Package with populated metadata (may be nil if error occurred)
//   - initialFlags: flags from selection mode (PkgFManualSel, etc.)
//   - parseFlags: flags from Makefile parsing (PkgFIgnored, PkgFMeta, PkgFCorrupt)
//   - ignoreReason: reason string if port is ignored
//   - err: error if package info fetching failed
func (bq *BulkQueue) GetResult() (*Package, PackageFlags, PackageFlags, string, error) {
	result := <-bq.resultChan

	bq.mu.Lock()
	bq.active--
	bq.mu.Unlock()

	return result.pkg, result.initialFlags, result.parseFlags, result.ignoreReason, result.err
}

// Close shuts down the worker pool and waits for all workers to finish.
// Must be called when done with the queue to avoid goroutine leaks.
//
// After Close returns, no more results will be available from GetResult().
// It's safe (and recommended) to call Close via defer immediately after
// creating the queue.
func (bq *BulkQueue) Close() {
	close(bq.workChan)
	bq.wg.Wait()
	close(bq.resultChan)
}

// Pending returns the number of work items still queued or being processed.
// Returns 0 when all queued work has been completed and results retrieved.
func (bq *BulkQueue) Pending() int {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	return bq.active
}
