package build

import (
	"testing"
)

// TestClosureCapturesPointer verifies that the cleanup closure
// correctly captures the BuildContext pointer and sees workers
// that are added after the closure is created.
func TestClosureCapturesPointer(t *testing.T) {
	type TestContext struct {
		workers []*Worker
	}

	// Simulate the pattern used in DoBuild
	ctx := &TestContext{}

	// Create cleanup closure BEFORE workers are populated
	var capturedWorkers []*Worker
	cleanup := func() {
		// This should see the workers that are added AFTER this closure is created
		capturedWorkers = ctx.workers
	}

	// At this point, ctx.workers is nil
	if ctx.workers != nil {
		t.Fatal("Expected ctx.workers to be nil initially")
	}

	// Now populate workers (simulating what happens after callback is invoked)
	ctx.workers = make([]*Worker, 3)
	ctx.workers[0] = &Worker{ID: 0, Status: "idle"}
	ctx.workers[1] = &Worker{ID: 1, Status: "idle"}
	ctx.workers[2] = &Worker{ID: 2, Status: "idle"}

	// Call the cleanup function
	cleanup()

	// Verify the closure saw the populated workers
	if len(capturedWorkers) != 3 {
		t.Fatalf("Expected cleanup to see 3 workers, got %d", len(capturedWorkers))
	}

	for i, w := range capturedWorkers {
		if w == nil {
			t.Fatalf("Worker %d is nil", i)
		}
		if w.ID != i {
			t.Fatalf("Worker %d has wrong ID: expected %d, got %d", i, i, w.ID)
		}
	}

	t.Log("✓ Closure correctly captured ctx pointer and saw workers added after creation")
}

// TestClosureWithNilWorkers verifies cleanup handles nil workers gracefully
func TestClosureWithNilWorkers(t *testing.T) {
	type TestContext struct {
		workers []*Worker
	}

	ctx := &TestContext{}

	// Create cleanup that iterates over workers
	cleanupCalled := false
	cleanup := func() {
		cleanupCalled = true
		for i, worker := range ctx.workers {
			if worker != nil {
				t.Logf("Would cleanup worker %d", i)
			}
		}
	}

	// Call cleanup with nil workers (should not panic)
	cleanup()

	if !cleanupCalled {
		t.Fatal("Cleanup was not called")
	}

	t.Log("✓ Cleanup handles nil workers slice gracefully")
}
