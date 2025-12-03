package build

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// TestNcursesUI_CtrlC_SimulationScreen tests that Ctrl+C triggers interrupt handler
// using tcell's SimulationScreen for automated testing without requiring a real terminal
func TestNcursesUI_CtrlC_SimulationScreen(t *testing.T) {
	// Create simulation screen for testing
	simScreen := tcell.NewSimulationScreen("UTF-8")
	err := simScreen.Init()
	if err != nil {
		t.Fatalf("Failed to init simulation screen: %v", err)
	}
	// Note: Don't defer simScreen.Fini() - ui.Stop() will finalize it via tview

	// Set screen size
	simScreen.SetSize(80, 24)

	// Create UI and inject simulation screen
	ui := NewNcursesUI()
	ui.SetScreen(simScreen)

	// Track if interrupt handler was called
	interruptCalled := make(chan bool, 1)
	ui.SetInterruptHandler(func() {
		interruptCalled <- true
	})

	// Start UI (will use injected screen)
	err = ui.Start()
	if err != nil {
		t.Fatalf("Failed to start UI: %v", err)
	}

	// Give UI time to start
	time.Sleep(100 * time.Millisecond)

	// Inject Ctrl+C as KeyRune with rune 3 (same pattern as 'q' test)
	// SimulationScreen seems to only reliably deliver KeyRune events
	simScreen.InjectKey(tcell.KeyRune, rune(3), tcell.ModNone)

	// Wait for interrupt handler to be called (with timeout)
	select {
	case <-interruptCalled:
		t.Log("✓ Interrupt handler was called successfully")
	case <-time.After(2 * time.Second):
		// Get screen contents to debug
		cells, _, _ := simScreen.GetContents()
		screenText := ""
		for _, c := range cells {
			for _, r := range c.Runes {
				screenText += string(r)
			}
		}
		t.Logf("Screen content sample: %s", screenText[:min(200, len(screenText))])
		t.Fatal("✗ Timeout waiting for interrupt handler - Ctrl+C not detected")
	}

	// Cleanup
	ui.Stop()
}

// TestNcursesUI_QuitKey_SimulationScreen tests that 'q' triggers interrupt handler
func TestNcursesUI_QuitKey_SimulationScreen(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	err := simScreen.Init()
	if err != nil {
		t.Fatalf("Failed to init simulation screen: %v", err)
	}
	// Note: Don't defer simScreen.Fini() - ui.Stop() will finalize it via tview

	simScreen.SetSize(80, 24)

	ui := NewNcursesUI()
	ui.SetScreen(simScreen)

	interruptCalled := make(chan bool, 1)
	ui.SetInterruptHandler(func() {
		interruptCalled <- true
	})

	// Start UI
	err = ui.Start()
	if err != nil {
		t.Fatalf("Failed to start UI: %v", err)
	}

	// Give UI time to initialize
	time.Sleep(100 * time.Millisecond)

	// Inject 'q' key
	simScreen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)

	select {
	case <-interruptCalled:
		t.Log("✓ Interrupt handler called via 'q' key")
	case <-time.After(2 * time.Second):
		t.Fatal("✗ Timeout waiting for interrupt handler - 'q' key not detected")
	}

	ui.Stop()
}

// TestNcursesUI_UpdateProgress_SimulationScreen tests that UI updates work
func TestNcursesUI_UpdateProgress_SimulationScreen(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	err := simScreen.Init()
	if err != nil {
		t.Fatalf("Failed to init simulation screen: %v", err)
	}
	// Note: Don't defer simScreen.Fini() - ui.Stop() will finalize it via tview

	simScreen.SetSize(80, 24)

	ui := NewNcursesUI()
	ui.SetScreen(simScreen)

	// Start UI
	err = ui.Start()
	if err != nil {
		t.Fatalf("Failed to start UI: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Update progress
	stats := BuildStats{
		Total:      10,
		Success:    3,
		Failed:     1,
		SkippedPre: 2,
		Skipped:    0,
	}
	ui.UpdateProgress(stats, "00:05:30")

	// Log an event
	ui.LogEvent(1, "Building editors/vim")

	// Give time for draw operations
	time.Sleep(200 * time.Millisecond)

	// Verify screen contents contain expected text
	cells, width, height := simScreen.GetContents()
	t.Logf("Screen size: %dx%d, cells: %d", width, height, len(cells))

	// Extract text from cells for verification
	screenText := ""
	for _, cell := range cells {
		screenText += string(cell.Runes)
	}

	// Check for expected content
	if !contains(screenText, "Success") {
		t.Error("Screen should contain 'Success' in progress display")
	}
	if !contains(screenText, "editors/vim") {
		t.Error("Screen should contain logged event 'editors/vim'")
	}

	ui.Stop()
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
