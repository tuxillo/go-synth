//go:build manual
// +build manual

package stats

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestStatsCollectorDemo demonstrates StatsCollector behavior in real-time.
// Run with: go test -tags=manual -v -run TestStatsCollectorDemo ./stats/
//
// This test simulates a build workflow:
// 1. Creates collector with 4 workers
// 2. Simulates packages completing over time
// 3. Shows live rate/impulse updates every second
func TestStatsCollectorDemo(t *testing.T) {
	fmt.Println("\n=== StatsCollector Live Demo ===")
	fmt.Println("Simulating a build with 4 workers building 50 packages...\n")

	ctx := context.Background()
	sc := NewStatsCollector(ctx, 4)
	defer sc.Close()

	// Set initial queue
	sc.UpdateQueuedCount(50)

	// Register a demo consumer to see updates
	updates := make(chan TopInfo, 100)
	consumer := &demoConsumer{ch: updates}
	sc.AddConsumer(consumer)

	// Start a goroutine to print updates
	done := make(chan bool)
	go func() {
		for info := range updates {
			fmt.Printf("[%s] Workers: %d/%d  Rate: %.1f pkg/hr  Impulse: %.0f  Built: %d  Failed: %d  Remaining: %d\n",
				FormatDuration(info.Elapsed),
				info.ActiveWorkers, info.MaxWorkers,
				info.Rate, info.Impulse,
				info.Built, info.Failed, info.Remaining)
		}
		done <- true
	}()

	// Simulate build activity
	fmt.Println("Starting build simulation...\n")

	// Burst of completions (0-2s)
	for i := 0; i < 10; i++ {
		sc.RecordCompletion(BuildSuccess)
		time.Sleep(200 * time.Millisecond)
	}

	// Steady rate (2-5s)
	for i := 0; i < 15; i++ {
		sc.RecordCompletion(BuildSuccess)
		time.Sleep(200 * time.Millisecond)
	}

	// Some failures (5-7s)
	for i := 0; i < 5; i++ {
		if i%3 == 0 {
			sc.RecordCompletion(BuildFailed)
		} else {
			sc.RecordCompletion(BuildSuccess)
		}
		time.Sleep(400 * time.Millisecond)
	}

	// Skip some packages (shouldn't affect rate)
	for i := 0; i < 5; i++ {
		sc.RecordCompletion(BuildSkipped)
		time.Sleep(100 * time.Millisecond)
	}

	// Final burst (7-9s)
	for i := 0; i < 15; i++ {
		sc.RecordCompletion(BuildSuccess)
		time.Sleep(133 * time.Millisecond)
	}

	// Wait a few more seconds to see rate decay
	fmt.Println("\nWaiting for rate to stabilize...\n")
	time.Sleep(3 * time.Second)

	// Final snapshot
	snapshot := sc.GetSnapshot()
	fmt.Printf("\n=== Final Stats ===\n")
	fmt.Printf("Total Elapsed: %s\n", FormatDuration(snapshot.Elapsed))
	fmt.Printf("Built: %d\n", snapshot.Built)
	fmt.Printf("Failed: %d\n", snapshot.Failed)
	fmt.Printf("Skipped: %d\n", snapshot.Skipped)
	fmt.Printf("Final Rate: %.1f pkg/hr\n", snapshot.Rate)
	fmt.Printf("Final Impulse: %.0f\n", snapshot.Impulse)
	fmt.Printf("Remaining: %d\n", snapshot.Remaining)

	// Close and wait for printer
	close(updates)
	<-done

	fmt.Println("\nDemo complete!")
}

type demoConsumer struct {
	ch chan TopInfo
}

func (dc *demoConsumer) OnStatsUpdate(info TopInfo) {
	select {
	case dc.ch <- info:
	default:
		// Drop update if channel full
	}
}
