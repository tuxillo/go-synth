package stats

import (
	"runtime"
	"testing"
)

// TestWorkerThrottler_NoThrottling verifies no throttling below thresholds
func TestWorkerThrottler_NoThrottling(t *testing.T) {
	wt := NewWorkerThrottler(8, false)

	tests := []struct {
		name    string
		load    float64
		swapPct int
		want    int
	}{
		{"zero metrics", 0, 0, 8},
		{"low load", 1.0, 0, 8},
		{"low swap", 0, 5, 8},
		{"both low", 1.0, 5, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wt.CalculateDynMax(tt.load, tt.swapPct)
			if got != tt.want {
				t.Errorf("CalculateDynMax(%v, %d) = %d, want %d", tt.load, tt.swapPct, got, tt.want)
			}
		})
	}
}

// TestWorkerThrottler_LoadThrottling tests load-based throttling
func TestWorkerThrottler_LoadThrottling(t *testing.T) {
	wt := NewWorkerThrottler(8, false)
	ncpus := float64(runtime.NumCPU())

	tests := []struct {
		name string
		load float64
		want int
	}{
		{"below threshold", 1.5*ncpus - 0.1, 8},
		{"at min threshold", 1.5 * ncpus, 8},
		{"mid range", 3.25 * ncpus, 5},         // ~middle of 1.5-5.0 range
		{"near max threshold", 4.9 * ncpus, 3}, // Close to 75% reduction (expect 3 due to rounding)
		{"at max threshold", 5.0 * ncpus, 2},   // 75% reduction
		{"above max threshold", 6.0 * ncpus, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wt.CalculateDynMax(tt.load, 0)
			if got != tt.want {
				t.Errorf("CalculateDynMax(%v, 0) = %d, want %d", tt.load, got, tt.want)
			}
		})
	}
}

// TestWorkerThrottler_SwapThrottling tests swap-based throttling
func TestWorkerThrottler_SwapThrottling(t *testing.T) {
	wt := NewWorkerThrottler(8, false)

	tests := []struct {
		name    string
		swapPct int
		want    int
	}{
		{"below threshold", 9, 8},
		{"at min threshold", 10, 8},
		{"mid range", 25, 5},          // Middle of 10-40% range
		{"near max threshold", 39, 3}, // Close to 75% reduction (expect 3 due to rounding)
		{"at max threshold", 40, 2},   // 75% reduction
		{"above max threshold", 50, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wt.CalculateDynMax(0, tt.swapPct)
			if got != tt.want {
				t.Errorf("CalculateDynMax(0, %d) = %d, want %d", tt.swapPct, got, tt.want)
			}
		})
	}
}

// TestWorkerThrottler_CombinedThrottling tests both caps simultaneously
func TestWorkerThrottler_CombinedThrottling(t *testing.T) {
	wt := NewWorkerThrottler(8, false)
	ncpus := float64(runtime.NumCPU())

	tests := []struct {
		name    string
		load    float64
		swapPct int
		want    int
	}{
		{
			name:    "high load, low swap - load wins",
			load:    4.0 * ncpus,
			swapPct: 5,
			want:    4, // Load cap ~4, swap cap 8 → min is 4
		},
		{
			name:    "low load, high swap - swap wins",
			load:    1.0,
			swapPct: 30,
			want:    4, // Load cap 8, swap cap ~4 → min is 4
		},
		{
			name:    "both high - most restrictive wins",
			load:    5.0 * ncpus,
			swapPct: 40,
			want:    2, // Both cap at 25% of 8 = 2
		},
		{
			name:    "both low - no throttling",
			load:    1.0,
			swapPct: 5,
			want:    8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wt.CalculateDynMax(tt.load, tt.swapPct)
			if got != tt.want {
				t.Errorf("CalculateDynMax(%v, %d) = %d, want %d", tt.load, tt.swapPct, got, tt.want)
			}
		})
	}
}

// TestWorkerThrottler_MinimumWorkers ensures at least 1 worker is always allowed
func TestWorkerThrottler_MinimumWorkers(t *testing.T) {
	wt := NewWorkerThrottler(1, false)

	// Even with extreme load/swap, should return at least 1
	got := wt.CalculateDynMax(1000, 100)
	if got < 1 {
		t.Errorf("CalculateDynMax(1000, 100) = %d, want >= 1", got)
	}
}

// TestWorkerThrottler_LargeWorkerCount tests throttling with many workers
func TestWorkerThrottler_LargeWorkerCount(t *testing.T) {
	wt := NewWorkerThrottler(64, false)
	ncpus := float64(runtime.NumCPU())

	// High load should reduce to 25%
	got := wt.CalculateDynMax(5.0*ncpus, 0)
	want := 16 // 25% of 64
	if got != want {
		t.Errorf("CalculateDynMax(high load, 0) = %d, want %d", got, want)
	}

	// High swap should reduce to 25%
	got = wt.CalculateDynMax(0, 40)
	want = 16 // 25% of 64
	if got != want {
		t.Errorf("CalculateDynMax(0, high swap) = %d, want %d", got, want)
	}
}

// TestWorkerThrottler_LinearInterpolation verifies smooth scaling
func TestWorkerThrottler_LinearInterpolation(t *testing.T) {
	wt := NewWorkerThrottler(100, false)
	ncpus := float64(runtime.NumCPU())

	// At exact midpoint of load range (3.25×ncpus between 1.5-5.0)
	midLoad := (1.5 + 5.0) / 2.0 * ncpus
	got := wt.CalculateDynMax(midLoad, 0)

	// Should be around 62-63 (midpoint between 100 and 25)
	// Linear: 100 - (75 * 0.5) = 62.5 → truncates to 62
	expectedMid := 62
	if got < expectedMid-1 || got > expectedMid+1 {
		t.Errorf("CalculateDynMax(mid load, 0) = %d, expected ~%d (±1)", got, expectedMid)
	}

	// At exact midpoint of swap range (25% between 10-40)
	midSwap := (10 + 40) / 2
	got = wt.CalculateDynMax(0, midSwap)

	// Should be around 62-63 (midpoint between 100 and 25)
	if got < expectedMid-1 || got > expectedMid+1 {
		t.Errorf("CalculateDynMax(0, mid swap) = %d, expected ~%d (±1)", got, expectedMid)
	}
}

// TestWorkerThrottler_Disabled verifies throttling is bypassed when disabled
func TestWorkerThrottler_Disabled(t *testing.T) {
	wt := NewWorkerThrottler(16, true) // disabled=true

	tests := []struct {
		name    string
		load    float64
		swapPct int
		want    int
	}{
		{"zero metrics", 0, 0, 16},
		{"high load", 100.0, 0, 16},      // Would normally throttle heavily
		{"high swap", 0, 80, 16},         // Would normally throttle heavily
		{"both high", 100.0, 80, 16},     // Would normally throttle to minimum
		{"extreme", 1000.0, 100, 16},     // Extreme values still return max
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wt.CalculateDynMax(tt.load, tt.swapPct)
			if got != tt.want {
				t.Errorf("Disabled throttler: CalculateDynMax(%v, %d) = %d, want %d (should always return maxWorkers)", 
					tt.load, tt.swapPct, got, tt.want)
			}
		})
	}
}

// TestWorkerThrottler_AutoDisableOnZeroMetrics verifies auto-disable when metrics unavailable
func TestWorkerThrottler_AutoDisableOnZeroMetrics(t *testing.T) {
	wt := NewWorkerThrottler(8, false) // enabled but metrics are zero

	// When both load and swap are zero, should auto-disable
	got := wt.CalculateDynMax(0.0, 0)
	want := 8
	if got != want {
		t.Errorf("CalculateDynMax(0.0, 0) with unavailable metrics = %d, want %d (auto-disable)", got, want)
	}

	// But if only one is zero, normal throttling applies
	// High swap with zero load should still throttle
	got = wt.CalculateDynMax(0.0, 50)
	if got >= 8 {
		t.Errorf("CalculateDynMax(0.0, 50) = %d, expected throttling with high swap even when load is zero", got)
	}
}
