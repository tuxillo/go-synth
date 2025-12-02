package stats

import (
	"context"
	"testing"
	"time"
)

// TestRateCalculation verifies rate calculation from ring buffer
func TestRateCalculation(t *testing.T) {
	tests := []struct {
		name     string
		buckets  [60]int
		expected float64
	}{
		{
			name:     "empty buckets",
			buckets:  [60]int{},
			expected: 0.0,
		},
		{
			name: "burst in one bucket",
			buckets: func() [60]int {
				var b [60]int
				b[0] = 10
				return b
			}(),
			expected: 600.0, // 10 * 60 pkg/hr
		},
		{
			name: "sustained 1 per second",
			buckets: func() [60]int {
				var b [60]int
				for i := 0; i < 60; i++ {
					b[i] = 1
				}
				return b
			}(),
			expected: 3600.0, // 60 * 60 pkg/hr
		},
		{
			name: "partial window",
			buckets: func() [60]int {
				var b [60]int
				for i := 0; i < 30; i++ {
					b[i] = 1
				}
				return b
			}(),
			expected: 1800.0, // 30 * 60 pkg/hr
		},
		{
			name: "varying rates",
			buckets: func() [60]int {
				var b [60]int
				b[0] = 5
				b[10] = 3
				b[20] = 2
				b[59] = 1
				return b
			}(),
			expected: 660.0, // 11 * 60 pkg/hr
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &StatsCollector{rateBuckets: tt.buckets}
			rate := sc.calculateRateLocked()
			if rate != tt.expected {
				t.Errorf("calculateRateLocked() = %.1f, want %.1f", rate, tt.expected)
			}
		})
	}
}

// TestImpulseTracking verifies impulse reflects previous bucket
func TestImpulseTracking(t *testing.T) {
	ctx := context.Background()
	sc := NewStatsCollector(ctx, 4)
	defer sc.Close()

	// Record 5 completions in current bucket
	for i := 0; i < 5; i++ {
		sc.RecordCompletion(BuildSuccess)
	}

	// Current bucket should have 5
	sc.mu.RLock()
	currentCount := sc.rateBuckets[sc.currentBucket]
	currentIdx := sc.currentBucket
	sc.mu.RUnlock()
	if currentCount != 5 {
		t.Errorf("current bucket = %d, want 5", currentCount)
	}

	// Manipulate bucketStart to simulate 1 second passing
	sc.mu.Lock()
	sc.bucketStart = sc.bucketStart.Add(-1 * time.Second)
	sc.mu.Unlock()

	// Force a tick to advance bucket
	sc.tick()

	// Impulse should now reflect the previous bucket (5 completions)
	snapshot := sc.GetSnapshot()
	if snapshot.Impulse != 5.0 {
		t.Errorf("impulse = %.1f, want 5.0", snapshot.Impulse)
	}

	// Current bucket should have advanced
	sc.mu.RLock()
	newIdx := sc.currentBucket
	newCurrent := sc.rateBuckets[sc.currentBucket]
	sc.mu.RUnlock()

	expectedIdx := (currentIdx + 1) % 60
	if newIdx != expectedIdx {
		t.Errorf("current bucket index = %d, want %d", newIdx, expectedIdx)
	}

	if newCurrent != 0 {
		t.Errorf("new current bucket = %d, want 0", newCurrent)
	}
}

// TestBucketAdvance verifies bucket rollover and clearing
func TestBucketAdvance(t *testing.T) {
	ctx := context.Background()
	sc := NewStatsCollector(ctx, 4)
	defer sc.Close()

	// Fill some buckets
	sc.mu.Lock()
	sc.rateBuckets[0] = 10
	sc.rateBuckets[1] = 20
	sc.rateBuckets[59] = 5
	sc.currentBucket = 59 // Start at end
	// Simulate 1 second passing
	sc.bucketStart = sc.bucketStart.Add(-1 * time.Second)
	sc.mu.Unlock()

	// Advance bucket (should wrap to 0)
	sc.tick()

	sc.mu.RLock()
	currentBucket := sc.currentBucket
	bucketZero := sc.rateBuckets[0]
	sc.mu.RUnlock()

	if currentBucket != 0 {
		t.Errorf("currentBucket = %d, want 0 (wrapped)", currentBucket)
	}

	if bucketZero != 0 {
		t.Errorf("bucket[0] = %d, want 0 (cleared on advance)", bucketZero)
	}
}

// TestBucketAdvanceMultiSecondGap verifies handling of long pauses
func TestBucketAdvanceMultiSecondGap(t *testing.T) {
	ctx := context.Background()
	sc := NewStatsCollector(ctx, 4)
	defer sc.Close()

	// Fill all buckets
	sc.mu.Lock()
	for i := 0; i < 60; i++ {
		sc.rateBuckets[i] = 1
	}
	sc.currentBucket = 0
	sc.bucketStart = time.Now().Add(-5 * time.Second) // Simulate 5 second gap
	sc.mu.Unlock()

	// Advance should clear 5 buckets
	sc.advanceBucketLocked(time.Now())

	sc.mu.RLock()
	currentBucket := sc.currentBucket
	// Should have advanced 5 buckets: 0→1→2→3→4→5
	expectedBucket := 5
	sc.mu.RUnlock()

	if currentBucket != expectedBucket {
		t.Errorf("currentBucket = %d, want %d after 5s gap", currentBucket, expectedBucket)
	}

	// Buckets 1-5 should be cleared (0 was already current, then 1-5 were entered)
	sc.mu.RLock()
	for i := 1; i <= 5; i++ {
		if sc.rateBuckets[i] != 0 {
			t.Errorf("bucket[%d] = %d, want 0 (should be cleared)", i, sc.rateBuckets[i])
		}
	}
	sc.mu.RUnlock()
}

// TestSkippedNotCounted verifies SKIP status doesn't increment rate
func TestSkippedNotCounted(t *testing.T) {
	ctx := context.Background()
	sc := NewStatsCollector(ctx, 4)
	defer sc.Close()

	// Record various statuses
	sc.RecordCompletion(BuildSuccess)
	sc.RecordCompletion(BuildFailed)
	sc.RecordCompletion(BuildIgnored)
	sc.RecordCompletion(BuildSkipped) // Should NOT increment bucket
	sc.RecordCompletion(BuildSkipped) // Should NOT increment bucket

	// Current bucket should have 3 (not 5)
	sc.mu.RLock()
	count := sc.rateBuckets[sc.currentBucket]
	sc.mu.RUnlock()

	if count != 3 {
		t.Errorf("bucket count = %d, want 3 (skipped should not count)", count)
	}

	// Verify totals
	snapshot := sc.GetSnapshot()
	if snapshot.Built != 1 {
		t.Errorf("Built = %d, want 1", snapshot.Built)
	}
	if snapshot.Failed != 1 {
		t.Errorf("Failed = %d, want 1", snapshot.Failed)
	}
	if snapshot.Ignored != 1 {
		t.Errorf("Ignored = %d, want 1", snapshot.Ignored)
	}
	if snapshot.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", snapshot.Skipped)
	}
}

// TestUpdateMethods verifies helper update methods
func TestUpdateMethods(t *testing.T) {
	ctx := context.Background()
	sc := NewStatsCollector(ctx, 8)
	defer sc.Close()

	// Update worker count
	sc.UpdateWorkerCount(4)
	snapshot := sc.GetSnapshot()
	if snapshot.ActiveWorkers != 4 {
		t.Errorf("ActiveWorkers = %d, want 4", snapshot.ActiveWorkers)
	}

	// Update queued count
	sc.UpdateQueuedCount(100)
	snapshot = sc.GetSnapshot()
	if snapshot.Queued != 100 {
		t.Errorf("Queued = %d, want 100", snapshot.Queued)
	}

	// Verify max workers set in constructor
	if snapshot.MaxWorkers != 8 {
		t.Errorf("MaxWorkers = %d, want 8", snapshot.MaxWorkers)
	}
}

// TestElapsedTime verifies elapsed time calculation
func TestElapsedTime(t *testing.T) {
	ctx := context.Background()
	sc := NewStatsCollector(ctx, 4)
	defer sc.Close()

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Tick to update elapsed
	sc.tick()

	snapshot := sc.GetSnapshot()
	if snapshot.Elapsed < 100*time.Millisecond {
		t.Errorf("Elapsed = %v, want >= 100ms", snapshot.Elapsed)
	}
}

// TestRemainingCalculation verifies remaining count calculation
func TestRemainingCalculation(t *testing.T) {
	ctx := context.Background()
	sc := NewStatsCollector(ctx, 4)
	defer sc.Close()

	// Set queued count
	sc.UpdateQueuedCount(100)

	// Record some completions
	for i := 0; i < 10; i++ {
		sc.RecordCompletion(BuildSuccess)
	}
	for i := 0; i < 5; i++ {
		sc.RecordCompletion(BuildFailed)
	}
	for i := 0; i < 3; i++ {
		sc.RecordCompletion(BuildIgnored)
	}

	// Tick to update remaining
	sc.tick()

	snapshot := sc.GetSnapshot()
	// Remaining = 100 - (10 + 5 + 3) = 82
	expected := 82
	if snapshot.Remaining != expected {
		t.Errorf("Remaining = %d, want %d", snapshot.Remaining, expected)
	}
}

// TestConsumerNotification verifies consumers receive updates
func TestConsumerNotification(t *testing.T) {
	ctx := context.Background()
	sc := NewStatsCollector(ctx, 4)
	defer sc.Close()

	// Mock consumer
	received := make(chan TopInfo, 1)
	consumer := &mockConsumer{ch: received}
	sc.AddConsumer(consumer)

	// Trigger a tick
	sc.tick()

	// Should receive notification
	select {
	case info := <-received:
		if info.MaxWorkers != 4 {
			t.Errorf("received MaxWorkers = %d, want 4", info.MaxWorkers)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for consumer notification")
	}
}

// TestConcurrentAccess verifies thread safety
func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	sc := NewStatsCollector(ctx, 4)
	defer sc.Close()

	done := make(chan bool)

	// Goroutine 1: Record completions
	go func() {
		for i := 0; i < 100; i++ {
			sc.RecordCompletion(BuildSuccess)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 2: Update workers
	go func() {
		for i := 0; i < 100; i++ {
			sc.UpdateWorkerCount(i % 4)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 3: Read snapshots
	go func() {
		for i := 0; i < 100; i++ {
			_ = sc.GetSnapshot()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// Verify final state is consistent
	snapshot := sc.GetSnapshot()
	if snapshot.Built != 100 {
		t.Errorf("Built = %d, want 100", snapshot.Built)
	}
}

// mockConsumer implements StatsConsumer for testing
type mockConsumer struct {
	ch chan TopInfo
}

func (mc *mockConsumer) OnStatsUpdate(info TopInfo) {
	select {
	case mc.ch <- info:
	default:
		// Don't block if channel full
	}
}
