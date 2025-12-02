package build

import (
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NcursesUI implements BuildUI using tview/tcell for a rich TUI
type NcursesUI struct {
	app           *tview.Application
	headerText    *tview.TextView
	progressText  *tview.TextView
	eventsText    *tview.TextView
	layout        *tview.Flex
	mu            sync.Mutex
	eventLines    []string
	maxEventLines int
	stopChan      chan struct{}
	stopped       bool
}

// NewNcursesUI creates a new ncurses-based UI
func NewNcursesUI() *NcursesUI {
	return &NcursesUI{
		maxEventLines: 100,
		stopChan:      make(chan struct{}),
	}
}

// Start initializes the ncurses UI
func (ui *NcursesUI) Start() error {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	ui.app = tview.NewApplication()

	// Header section (build summary)
	ui.headerText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	ui.headerText.SetBorder(true).SetTitle(" go-synth Build Status ").SetTitleAlign(tview.AlignLeft)
	ui.headerText.SetText("[yellow]Initializing build...[white]")

	// Progress section (statistics)
	ui.progressText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	ui.progressText.SetBorder(true).SetTitle(" Progress ").SetTitleAlign(tview.AlignLeft)
	ui.progressText.SetText("Waiting for build to start...")

	// Events section (worker logs)
	ui.eventsText = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			ui.app.Draw()
		})
	ui.eventsText.SetBorder(true).SetTitle(" Worker Events ").SetTitleAlign(tview.AlignLeft)
	ui.eventsText.SetText("No events yet...")

	// Layout: header (3 rows) + progress (5 rows) + events (rest)
	ui.layout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.headerText, 3, 0, false).
		AddItem(ui.progressText, 5, 0, false).
		AddItem(ui.eventsText, 0, 1, false)

	// Set up key bindings
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			ui.app.Stop()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q', 'Q':
				ui.app.Stop()
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
	ui.headerText.SetText(headerText)

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
	ui.progressText.SetText(progressText)

	ui.app.Draw()
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

	// Update the events text view
	eventsText := ""
	for _, line := range ui.eventLines {
		eventsText += line + "\n"
	}
	ui.eventsText.SetText(eventsText)

	// Scroll to bottom
	ui.eventsText.ScrollToEnd()

	ui.app.Draw()
}
