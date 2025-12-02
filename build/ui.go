package build

import "go-synth/stats"

// BuildUI is the interface for displaying build progress
// Implementations can be stdout (current), ncurses, web UI, etc.
type BuildUI interface {
	// Start initializes the UI (e.g., setup ncurses screen)
	Start() error

	// Stop cleanly shuts down the UI (e.g., restore terminal)
	Stop()

	// UpdateProgress updates the progress display with current stats and elapsed time
	UpdateProgress(stats BuildStats, elapsed string)

	// LogEvent logs a worker event (e.g., "[worker 0] start build: vim")
	LogEvent(workerID int, message string)

	// OnStatsUpdate receives real-time stats updates (called every 1s by StatsCollector)
	// This is part of the stats.StatsConsumer interface
	OnStatsUpdate(info stats.TopInfo)
}
