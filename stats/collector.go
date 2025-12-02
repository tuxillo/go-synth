// Package stats - StatsCollector implementation
package stats

import (
	"context"
	"sync"
	"time"
)

// StatsCollector collects real-time build statistics with 1 Hz sampling.
// It maintains a 60-second sliding window for rate calculation and notifies
// registered consumers (UI, BuildDB, monitor file) on each tick.
//
// Thread-safe for concurrent access from build workers and sampling goroutine.
type StatsCollector struct {
	mu            sync.RWMutex
	topInfo       TopInfo         // Current snapshot
	rateBuckets   [60]int         // Ring buffer: 1-second buckets for rate calculation
	currentBucket int             // Current bucket index (0-59)
	bucketStart   time.Time       // Start time of current bucket
	startTime     time.Time       // Build start timestamp
	ticker        *time.Ticker    // 1 Hz sampling ticker
	consumers     []StatsConsumer // Registered consumers (UI, monitor, etc.)
	ctx           context.Context // Cancellation context
	cancel        context.CancelFunc
	wg            sync.WaitGroup // Wait for goroutine to finish
}

// NewStatsCollector creates a new StatsCollector and starts the 1 Hz sampling loop.
// The collector runs until Close() is called or the context is cancelled.
//
// maxWorkers is the configured maximum number of build workers.
func NewStatsCollector(ctx context.Context, maxWorkers int) *StatsCollector {
	collectorCtx, cancel := context.WithCancel(ctx)
	now := time.Now()

	sc := &StatsCollector{
		topInfo: TopInfo{
			MaxWorkers: maxWorkers,
			StartTime:  now,
		},
		bucketStart: now,
		startTime:   now,
		ticker:      time.NewTicker(1 * time.Second),
		ctx:         collectorCtx,
		cancel:      cancel,
	}

	// Start sampling loop
	sc.wg.Add(1)
	go sc.run()

	return sc
}

// RecordCompletion records a package build completion event.
// Updates the current rate bucket and build totals based on status.
//
// BuildSkipped events do NOT count toward rate (not actual build work).
// BuildSuccess, BuildFailed, and BuildIgnored all count as completions.
func (sc *StatsCollector) RecordCompletion(status BuildStatus) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Ensure bucket is current (handles clock skew)
	sc.advanceBucketLocked(time.Now())

	// Update totals
	switch status {
	case BuildSuccess:
		sc.topInfo.Built++
	case BuildFailed:
		sc.topInfo.Failed++
	case BuildIgnored:
		sc.topInfo.Ignored++
	case BuildSkipped:
		sc.topInfo.Skipped++
		// Skip does NOT increment rate bucket (not actual work)
		return
	}

	// Increment current bucket for rate calculation
	sc.rateBuckets[sc.currentBucket]++
}

// UpdateWorkerCount updates the active worker count.
func (sc *StatsCollector) UpdateWorkerCount(active int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.topInfo.ActiveWorkers = active
}

// UpdateQueuedCount updates the total queued package count.
func (sc *StatsCollector) UpdateQueuedCount(queued int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.topInfo.Queued = queued
}

// GetSnapshot returns a thread-safe copy of the current TopInfo.
func (sc *StatsCollector) GetSnapshot() TopInfo {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.topInfo
}

// AddConsumer registers a stats consumer to receive updates on each tick.
// Consumers are notified in registration order.
func (sc *StatsCollector) AddConsumer(consumer StatsConsumer) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.consumers = append(sc.consumers, consumer)
}

// Close stops the sampling loop and waits for cleanup.
func (sc *StatsCollector) Close() error {
	sc.cancel()
	sc.ticker.Stop()
	sc.wg.Wait()
	return nil
}

// run is the 1 Hz sampling loop (goroutine).
// Each tick advances the bucket, calculates rate/impulse, and notifies consumers.
func (sc *StatsCollector) run() {
	defer sc.wg.Done()

	for {
		select {
		case <-sc.ticker.C:
			sc.tick()
		case <-sc.ctx.Done():
			return
		}
	}
}

// tick performs a single sampling iteration.
// Called every second by the sampling loop.
func (sc *StatsCollector) tick() {
	now := time.Now()

	sc.mu.Lock()

	// Advance to next bucket
	sc.advanceBucketLocked(now)

	// Calculate elapsed time
	sc.topInfo.Elapsed = now.Sub(sc.startTime)

	// Calculate rate (packages/hour over 60-second window)
	sc.topInfo.Rate = sc.calculateRateLocked()

	// Calculate impulse (completions in previous bucket)
	prevBucket := (sc.currentBucket + 59) % 60
	sc.topInfo.Impulse = float64(sc.rateBuckets[prevBucket])

	// Calculate remaining
	sc.topInfo.Remaining = sc.topInfo.Queued - (sc.topInfo.Built + sc.topInfo.Failed + sc.topInfo.Ignored)

	// Copy snapshot for consumers (outside lock)
	snapshot := sc.topInfo
	consumers := sc.consumers

	sc.mu.Unlock()

	// Notify consumers (outside lock to avoid blocking)
	for _, consumer := range consumers {
		consumer.OnStatsUpdate(snapshot)
	}
}

// advanceBucketLocked advances the bucket index, handling multi-second gaps.
// Must be called with lock held.
func (sc *StatsCollector) advanceBucketLocked(now time.Time) {
	// How many seconds elapsed since bucket start?
	elapsed := now.Sub(sc.bucketStart)

	// Advance buckets for each elapsed second
	for elapsed >= time.Second {
		// Move to next bucket
		sc.currentBucket = (sc.currentBucket + 1) % 60

		// Zero out the new bucket (clear old data)
		sc.rateBuckets[sc.currentBucket] = 0

		// Advance bucket start time by 1 second
		sc.bucketStart = sc.bucketStart.Add(time.Second)

		// Recalculate elapsed
		elapsed = now.Sub(sc.bucketStart)
	}
}

// calculateRateLocked calculates packages/hour from the 60-second window.
// Must be called with lock held.
func (sc *StatsCollector) calculateRateLocked() float64 {
	sum := 0
	for _, count := range sc.rateBuckets {
		sum += count
	}

	// Packages per hour: (completions in 60s) * 60 min/hr
	return float64(sum * 60)
}
