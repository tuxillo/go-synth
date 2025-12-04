package build

import (
	"fmt"
	"sync"
	"time"

	"go-synth/stats"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NcursesUI implements BuildUI using tview/tcell for a rich TUI
type NcursesUI struct {
	app           *tview.Application
	screen        tcell.Screen // Optional injected screen (for testing)
	headerText    *tview.TextView
	progressText  *tview.TextView
	eventsText    *tview.TextView
	layout        *tview.Flex
	mu            sync.Mutex
	eventLines    []string
	maxEventLines int
	stopped       bool
	onInterrupt   func() // Callback for Ctrl+C handling
}

// NewNcursesUI creates a new ncurses-based UI
func NewNcursesUI() *NcursesUI {
	return &NcursesUI{
		maxEventLines: 100,
	}
}

// SetScreen injects a custom tcell.Screen for testing purposes
// Must be called before Start()
func (ui *NcursesUI) SetScreen(screen tcell.Screen) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	ui.screen = screen
}

// SetInterruptHandler sets a callback to be called when Ctrl+C is pressed
func (ui *NcursesUI) SetInterruptHandler(handler func()) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	ui.onInterrupt = handler
}

// Start initializes the ncurses UI
func (ui *NcursesUI) Start() error {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	ui.app = tview.NewApplication()

	// If a custom screen was injected (for testing), use it
	if ui.screen != nil {
		ui.app.SetScreen(ui.screen)
	}

	// Header section (system stats)
	ui.headerText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWordWrap(true)
	ui.headerText.SetBorder(true).SetTitle(" System Stats ").SetTitleAlign(tview.AlignLeft)
	ui.headerText.SetText("[yellow]Initializing build...[white]")

	// Progress section (statistics)
	ui.progressText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWordWrap(true)
	ui.progressText.SetBorder(true).SetTitle(" Progress ").SetTitleAlign(tview.AlignLeft)
	ui.progressText.SetText("Waiting for build to start...")

	// Events section (worker logs)
	ui.eventsText = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			ui.app.Draw()
		})
	ui.eventsText.SetBorder(true).SetTitle(" Worker Events ").SetTitleAlign(tview.AlignLeft)
	ui.eventsText.SetText("No events yet...")

	// Layout: header (4 rows fixed) + progress (6 rows fixed) + events (flexible)
	ui.layout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.headerText, 4, 0, false).
		AddItem(ui.progressText, 6, 0, false).
		AddItem(ui.eventsText, 0, 1, true)

	// Set up key bindings
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			// Trigger interrupt handler if set (for cleanup)
			ui.mu.Lock()
			handler := ui.onInterrupt
			ui.mu.Unlock()
			if handler != nil {
				// Use goroutine so handler runs async and doesn't block event loop
				// The handler will eventually call cleanup() which stops the UI
				go handler()
			}
			return nil
		case tcell.KeyRune:
			// Handle both 'q'/'Q' and Ctrl+C (rune 3)
			switch event.Rune() {
			case 3: // Ctrl+C as ETX (End of Text) ASCII code
				// Trigger interrupt handler if set (for cleanup)
				ui.mu.Lock()
				handler := ui.onInterrupt
				ui.mu.Unlock()
				if handler != nil {
					go handler()
				}
				return nil
			case 'q', 'Q':
				// Trigger interrupt handler if set (for cleanup)
				ui.mu.Lock()
				handler := ui.onInterrupt
				ui.mu.Unlock()
				if handler != nil {
					go handler()
				}
				return nil
			}
		}
		return event
	})

	// Start the application in a goroutine
	go func() {
		if err := ui.app.SetRoot(ui.layout, true).EnableMouse(true).Run(); err != nil {
			// Application stopped
		}
	}()

	// Give the UI a moment to initialize
	time.Sleep(100 * time.Millisecond)

	return nil
}

// Stop cleanly shuts down the ncurses UI
func (ui *NcursesUI) Stop() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.stopped {
		return
	}
	ui.stopped = true

	if ui.app != nil {
		ui.app.Stop()
	}

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)
}

// UpdateProgress updates the progress display
func (ui *NcursesUI) UpdateProgress(stats BuildStats, elapsed string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.app == nil || ui.stopped {
		return
	}

	done := stats.Success + stats.Failed + stats.SkippedPre + stats.Skipped

	// Update header
	headerText := fmt.Sprintf("[yellow]Building:[white] %d/%d packages | [green]Elapsed:[white] %s",
		done, stats.Total, elapsed)

	// Update progress section
	progressText := fmt.Sprintf(
		"[green]✓ Success:[white]     %3d\n"+
			"[red]✗ Failed:[white]      %3d\n"+
			"[yellow]⊙ Pre-skipped:[white] %3d\n"+
			"[yellow]⊙ Dep-skipped:[white] %3d",
		stats.Success,
		stats.Failed,
		stats.SkippedPre,
		stats.Skipped,
	)

	// Queue updates on the UI thread (thread-safe)
	ui.app.QueueUpdateDraw(func() {
		ui.headerText.SetText(headerText)
		ui.progressText.SetText(progressText)
	})
}

// LogEvent logs a worker event
func (ui *NcursesUI) LogEvent(workerID int, message string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.app == nil || ui.stopped {
		return
	}

	timestamp := time.Now().Format("15:04:05")
	event := fmt.Sprintf("[%s] [cyan][Worker %d][white] %s", timestamp, workerID, message)

	// Add to event lines buffer
	ui.eventLines = append(ui.eventLines, event)

	// Keep only the last N lines
	if len(ui.eventLines) > ui.maxEventLines {
		ui.eventLines = ui.eventLines[1:]
	}

	// Build events text
	eventsText := ""
	for _, line := range ui.eventLines {
		eventsText += line + "\n"
	}

	// Queue updates on the UI thread (thread-safe)
	ui.app.QueueUpdateDraw(func() {
		ui.eventsText.SetText(eventsText)
		ui.eventsText.ScrollToEnd()
	})
}

// OnStatsUpdate implements stats.StatsConsumer interface
// Called every 1 second by StatsCollector with fresh metrics
func (ui *NcursesUI) OnStatsUpdate(info stats.TopInfo) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.app == nil || ui.stopped {
		return
	}

	// Format stats header (2 lines)
	line1 := fmt.Sprintf("[yellow]Workers:[white] %2d/%2d  [yellow]Load:[white] %4.2f  [yellow]Swap:[white] %2d%%  [yellow][DynMax: %d][white]",
		info.ActiveWorkers, info.MaxWorkers, info.Load, info.SwapPct, info.DynMaxWorkers)

	line2 := fmt.Sprintf("[yellow]Elapsed:[white] %s  [yellow]Rate:[white] %s pkg/hr  [yellow]Impulse:[white] %.0f",
		stats.FormatDuration(info.Elapsed), stats.FormatRate(info.Rate), info.Impulse)

	headerText := line1 + "\n" + line2

	// Determine border color based on throttling
	borderColor := tcell.ColorWhite
	if info.DynMaxWorkers < info.MaxWorkers {
		borderColor = tcell.ColorYellow
		// Add throttle warning
		reason := stats.ThrottleReason(info)
		headerText += fmt.Sprintf("\n[yellow]⚠ Workers throttled: %s[white]", reason)
	}

	// Queue updates on the UI thread (thread-safe)
	ui.app.QueueUpdateDraw(func() {
		ui.headerText.SetText(headerText)
		ui.headerText.SetBorderColor(borderColor)
	})
}
