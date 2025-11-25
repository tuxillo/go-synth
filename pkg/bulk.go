package pkg

import (
	"sync"

	"dsynth/config"
)

// Bulk operation queue for parallel package info fetching
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
	initialFlags int // Flags that should be set on this package
}

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
		pkg, err := getPackageInfo(work.category, work.name, work.flavor, bq.cfg)

		initialFlags := 0
		if err == nil && work.flags != "x" {
			initialFlags |= PkgFManualSel
		}

		if err == nil && work.flags == "d" {
			initialFlags |= PkgFManualSel // Debug stop mode
		}

		bq.resultChan <- &bulkResult{pkg: pkg, err: err, initialFlags: initialFlags}
	}
}

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

func (bq *BulkQueue) GetResult() (*Package, int, error) {
	result := <-bq.resultChan

	bq.mu.Lock()
	bq.active--
	bq.mu.Unlock()

	return result.pkg, result.initialFlags, result.err
}

func (bq *BulkQueue) Close() {
	close(bq.workChan)
	bq.wg.Wait()
	close(bq.resultChan)
}

func (bq *BulkQueue) Pending() int {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	return bq.active
}
