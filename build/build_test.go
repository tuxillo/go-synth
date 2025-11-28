package build

import (
	"testing"
	"time"
	
	"dsynth/pkg"
)

// TestFormatDuration tests duration formatting for display
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		dur  time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"seconds only", 45 * time.Second, "45s"},
		{"one minute", 1 * time.Minute, "1m00s"},
		{"minutes and seconds", 3*time.Minute + 30*time.Second, "3m30s"},
		{"one hour", 1 * time.Hour, "1h00m00s"},
		{"hours minutes seconds", 2*time.Hour + 15*time.Minute + 5*time.Second, "2h15m05s"},
		{"rounds to second", 1500 * time.Millisecond, "2s"},
		{"rounds down", 1499 * time.Millisecond, "1s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.dur)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.dur, got, tt.want)
			}
		})
	}
}

// TestBuildStats tests BuildStats zero initialization
func TestBuildStats_ZeroInitialization(t *testing.T) {
	stats := &BuildStats{}

	if stats.Total != 0 {
		t.Errorf("Total = %d, want 0", stats.Total)
	}
	if stats.Success != 0 {
		t.Errorf("Success = %d, want 0", stats.Success)
	}
	if stats.Failed != 0 {
		t.Errorf("Failed = %d, want 0", stats.Failed)
	}
	if stats.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", stats.Skipped)
	}
	if stats.Ignored != 0 {
		t.Errorf("Ignored = %d, want 0", stats.Ignored)
	}
	if stats.Duration != 0 {
		t.Errorf("Duration = %v, want 0", stats.Duration)
	}
}

// TestWorker tests Worker struct initialization
func TestWorker_InitialState(t *testing.T) {
	worker := &Worker{
		ID:     1,
		Status: "idle",
	}

	if worker.ID != 1 {
		t.Errorf("ID = %d, want 1", worker.ID)
	}
	if worker.Status != "idle" {
		t.Errorf("Status = %q, want %q", worker.Status, "idle")
	}
	if worker.Current != nil {
		t.Error("Current should be nil for new worker")
	}
	if worker.Env != nil {
		t.Error("Env should be nil before setup")
	}
}

// TestBuildContext tests BuildContext struct
func TestBuildContext_FieldsExist(t *testing.T) {
	// This test just verifies the struct can be created
	// We can't easily test the full functionality without mocks
	ctx := &BuildContext{
		queue: make(chan *pkg.Package, 10),
	}

	if ctx.queue == nil {
		t.Error("queue should not be nil")
	}
}
