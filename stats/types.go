// Package stats provides real-time build statistics collection and monitoring
// for go-synth. It tracks metrics like worker counts, system load, swap usage,
// build rates, and package completion totals.
//
// The stats system uses a 1 Hz sampling loop to collect metrics and notify
// registered consumers (UI displays, BuildDB writers, monitor file exporters).
package stats

import (
	"fmt"
	"time"
)

// TopInfo contains real-time build statistics.
// This is the unified payload shared across all stats consumers (UI, CLI, monitor).
//
// Data types chosen based on dsynth C source analysis with Go adaptations:
//   - Rate/Impulse: float64 for precision (double in C topinfo_t)
//   - Swap: int 0-100 percentage for clarity (converted from double 0-1.0 in C)
//   - Elapsed: time.Duration (convert to H:M:S for display)
type TopInfo struct {
	// Worker Metrics
	ActiveWorkers int // Currently building
	MaxWorkers    int // Configured max
	DynMaxWorkers int // Dynamic max (throttled by load/swap/memory)

	// System Metrics
	Load    float64 // Adjusted 1-min load average (includes vm.vmtotal.t_pw)
	SwapPct int     // Swap usage percentage (0-100)
	NoSwap  bool    // True if no swap configured

	// Build Rate Metrics
	Rate    float64 // Packages/hour (60s sliding window)
	Impulse float64 // Instant completions/sec (last 1s bucket)

	// Timing
	Elapsed   time.Duration // Time since build start
	StartTime time.Time     // Build start timestamp

	// Build Totals
	Queued    int // Total packages to build
	Built     int // Successfully built
	Failed    int // Build failures
	Ignored   int // Ignored/skipped by user
	Skipped   int // Skipped due to dependencies
	Meta      int // Metaports (no actual build)
	Remaining int // Calculated: Queued - (Built + Failed + Ignored)
}

// BuildStatus replaces C's DLOG_* bitwise flags with typed enum.
// Used to record package build outcomes for rate calculation and totals.
type BuildStatus int

const (
	BuildSuccess BuildStatus = iota // DLOG_SUCC - Successfully built
	BuildFailed                     // DLOG_FAIL - Build failed
	BuildIgnored                    // DLOG_IGN - Ignored/skipped by user
	BuildSkipped                    // DLOG_SKIP - Skipped due to dependencies
)

// String returns the string representation of BuildStatus
func (bs BuildStatus) String() string {
	switch bs {
	case BuildSuccess:
		return "success"
	case BuildFailed:
		return "failed"
	case BuildIgnored:
		return "ignored"
	case BuildSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// StatsConsumer interface for UI/monitor writer (replaces runstats_t callbacks).
// Consumers receive OnStatsUpdate() calls every 1 second with fresh TopInfo snapshot.
type StatsConsumer interface {
	OnStatsUpdate(info TopInfo)
}

// FormatDuration formats a duration as HH:MM:SS for display
func FormatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// FormatRate formats a rate (packages/hour) for display
func FormatRate(rate float64) string {
	if rate < 0.1 {
		return "0.0"
	}
	return fmt.Sprintf("%.1f", rate)
}

// ThrottleReason returns a human-readable reason for worker throttling
// based on current system metrics. Returns empty string if not throttled.
func ThrottleReason(info TopInfo) string {
	if info.DynMaxWorkers >= info.MaxWorkers {
		return "" // Not throttled
	}

	// Check thresholds (from Phase 2 throttling analysis)
	// Load: 1.5-5.0×ncpus triggers throttling
	// Swap: 10-40% triggers throttling
	// These are heuristics - actual throttling logic is in WorkerThrottler

	// Estimate ncpus from load threshold (rough heuristic)
	// If load > 2×ncpus, likely throttled by load
	estimatedNCPUs := info.MaxWorkers // Rough approximation

	if info.Load > float64(estimatedNCPUs)*2.0 {
		return "high load"
	}

	if info.SwapPct > 10 {
		return "high swap"
	}

	// Memory pressure would be checked here if we had that metric
	// For now, assume "system resources" as generic fallback
	return "system resources"
}
