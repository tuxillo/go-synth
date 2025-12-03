package stats

import "runtime"

// WorkerThrottler calculates dynamic worker limits based on system health.
// It implements the three-cap throttling algorithm from original dsynth:
//  1. Load-based cap: Linear interpolation between 1.5×ncpus and 5.0×ncpus
//  2. Swap-based cap: Linear interpolation between 10% and 40% swap usage
//  3. Final: Minimum of both caps (most restrictive wins)
//
// The throttling reduces worker count to prevent system overload during
// I/O-heavy builds that stress disk, memory, and swap.
type WorkerThrottler struct {
	maxWorkers int
	ncpus      int
	disabled   bool // When true, always return maxWorkers
}

// NewWorkerThrottler creates a throttler with the configured max workers.
// The ncpus value is determined automatically via runtime.NumCPU().
// If disabled is true, throttling is bypassed and maxWorkers is always returned.
func NewWorkerThrottler(maxWorkers int, disabled bool) *WorkerThrottler {
	return &WorkerThrottler{
		maxWorkers: maxWorkers,
		ncpus:      runtime.NumCPU(),
		disabled:   disabled,
	}
}

// CalculateDynMax computes the dynamic worker limit based on current system metrics.
// Returns a value between 1 and maxWorkers.
//
// Throttling rules (matching original dsynth):
//   - Load < 1.5×ncpus: No throttling (return maxWorkers)
//   - Load 1.5-5.0×ncpus: Linear reduction from 100% to 25% of maxWorkers
//   - Load > 5.0×ncpus: Hard cap at 25% of maxWorkers
//   - Swap < 10%: No swap throttling
//   - Swap 10-40%: Linear reduction from 100% to 25% of maxWorkers
//   - Swap > 40%: Hard cap at 25% of maxWorkers
//
// Returns the minimum of load-cap and swap-cap (most restrictive).
//
// Auto-disable: If both load and swap are zero (metrics not available),
// returns maxWorkers to avoid false throttling until metrics are implemented.
func (wt *WorkerThrottler) CalculateDynMax(load float64, swapPct int) int {
	// Explicit disable via config flag
	if wt.disabled {
		return wt.maxWorkers
	}

	// Auto-disable when metrics are unavailable (both zero)
	// This prevents false throttling until system metrics collection is implemented
	if load == 0.0 && swapPct == 0 {
		return wt.maxWorkers
	}

	// Calculate load-based cap
	loadCap := wt.calculateLoadCap(load)

	// Calculate swap-based cap
	swapCap := wt.calculateSwapCap(swapPct)

	// Return minimum (most restrictive)
	dynMax := loadCap
	if swapCap < dynMax {
		dynMax = swapCap
	}

	// Ensure at least 1 worker
	if dynMax < 1 {
		dynMax = 1
	}

	return dynMax
}

// calculateLoadCap computes the worker limit based on adjusted load average.
// Uses linear interpolation between thresholds:
//
//	minLoad = 1.5 × ncpus
//	maxLoad = 5.0 × ncpus
//
// If load < minLoad: Return maxWorkers (no throttling)
// If load >= maxLoad: Return 25% of maxWorkers (hard cap)
// If minLoad <= load < maxLoad: Linear interpolation
func (wt *WorkerThrottler) calculateLoadCap(load float64) int {
	minLoad := 1.5 * float64(wt.ncpus)
	maxLoad := 5.0 * float64(wt.ncpus)

	if load < minLoad {
		return wt.maxWorkers
	}

	if load >= maxLoad {
		return wt.maxWorkers / 4 // 75% reduction
	}

	// Linear interpolation: reduce from 100% to 25%
	ratio := (load - minLoad) / (maxLoad - minLoad)
	reduction := int(float64(wt.maxWorkers) * 0.75 * ratio)
	return wt.maxWorkers - reduction
}

// calculateSwapCap computes the worker limit based on swap usage percentage.
// Uses linear interpolation between thresholds:
//
//	minSwap = 10%
//	maxSwap = 40%
//
// If swap < minSwap: Return maxWorkers (no throttling)
// If swap >= maxSwap: Return 25% of maxWorkers (hard cap)
// If minSwap <= swap < maxSwap: Linear interpolation
func (wt *WorkerThrottler) calculateSwapCap(swapPct int) int {
	const minSwap = 10
	const maxSwap = 40

	if swapPct < minSwap {
		return wt.maxWorkers
	}

	if swapPct >= maxSwap {
		return wt.maxWorkers / 4 // 75% reduction
	}

	// Linear interpolation: reduce from 100% to 25%
	ratio := float64(swapPct-minSwap) / float64(maxSwap-minSwap)
	reduction := int(float64(wt.maxWorkers) * 0.75 * ratio)
	return wt.maxWorkers - reduction
}
