package build

import (
	"fmt"
	"sync"
)

// StdoutUI implements BuildUI using simple stdout output (current behavior)
type StdoutUI struct {
	mu sync.Mutex
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
