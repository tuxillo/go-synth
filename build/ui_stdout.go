package build

import (
	"fmt"
	"sync"
	"time"

	"go-synth/stats"
)

// StdoutUI implements BuildUI using simple stdout output (current behavior)
type StdoutUI struct {
	mu        sync.Mutex
	lastPrint time.Time // Last time stats were printed (throttle to every 5s)
}

// NewStdoutUI creates a new stdout-based UI
func NewStdoutUI() *StdoutUI {
	return &StdoutUI{}
}

// Start initializes the stdout UI (no-op)
func (ui *StdoutUI) Start() error {
	return nil
}

// Stop cleanly shuts down the stdout UI (no-op)
func (ui *StdoutUI) Stop() {
	// Print final newline to avoid leaving cursor on progress line
	fmt.Println()
}

// UpdateProgress updates the progress display
func (ui *StdoutUI) UpdateProgress(stats BuildStats, elapsed string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	done := stats.Success + stats.Failed + stats.SkippedPre + stats.Skipped
	progressMsg := fmt.Sprintf("Progress: %d/%d (success: %d, failed: %d, pre-skipped: %d, dep-skipped: %d) %s elapsed",
		done, stats.Total, stats.Success, stats.Failed, stats.SkippedPre, stats.Skipped, elapsed)

	fmt.Printf("\r%-80s", progressMsg)
}

// LogEvent logs a worker event
func (ui *StdoutUI) LogEvent(workerID int, message string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	event := fmt.Sprintf("[worker %d] %s", workerID, message)
	fmt.Printf("\r%-80s\n", event)
}

// OnStatsUpdate implements stats.StatsConsumer interface
// Prints condensed status line every 5 seconds to reduce spam
func (ui *StdoutUI) OnStatsUpdate(info stats.TopInfo) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	// Throttle output to every 5 seconds
	now := time.Now()
	if now.Sub(ui.lastPrint) < 5*time.Second {
		return
	}
	ui.lastPrint = now

	// Print condensed status line
	statusLine := fmt.Sprintf("\r[%s] Load %.2f Swap %d%% Rate %s/hr Built %d Failed %d",
		stats.FormatDuration(info.Elapsed), info.Load, info.SwapPct,
		stats.FormatRate(info.Rate), info.Built, info.Failed)

	// Add throttle warning if applicable
	if info.DynMaxWorkers < info.MaxWorkers {
		reason := stats.ThrottleReason(info)
		statusLine += fmt.Sprintf(" [THROTTLED: %s]", reason)
	}

	fmt.Printf("%-100s\n", statusLine)
}
