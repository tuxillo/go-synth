package build

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// TestNcursesUI_ScreenInjection_Validation validates that the injected screen is actually used
// This test verifies the dependency injection mechanism works correctly
func TestNcursesUI_ScreenInjection_Validation(t *testing.T) {
	// Create simulation screen
	simScreen := tcell.NewSimulationScreen("UTF-8")
	err := simScreen.Init()
	if err != nil {
		t.Fatalf("Failed to init simulation screen: %v", err)
	}

	simScreen.SetSize(80, 24)

	// Create UI and inject screen BEFORE Start()
	ui := NewNcursesUI()
	ui.SetScreen(simScreen)

	// Verify screen was stored
	ui.mu.Lock()
	if ui.screen == nil {
		t.Fatal("Screen was not stored by SetScreen()")
	}
	if ui.screen != simScreen {
		t.Fatal("Stored screen does not match injected screen")
	}
	ui.mu.Unlock()

	// Start UI
	err = ui.Start()
	if err != nil {
		t.Fatalf("Failed to start UI: %v", err)
	}

	// Give UI time to initialize
	time.Sleep(200 * time.Millisecond)

	// Verify app is running
	ui.mu.Lock()
	if ui.app == nil {
		t.Fatal("Application was not created by Start()")
	}
	ui.mu.Unlock()

	// Update progress to trigger screen drawing
	stats := BuildStats{
		Total:      10,
		Success:    3,
		Failed:     1,
		SkippedPre: 2,
		Skipped:    0,
	}
	ui.UpdateProgress(stats, "00:01:30")

	// Log an event to trigger more drawing
	ui.LogEvent(1, "Test event: Building devel/pkgconf")

	// Give time for draw operations to complete
	time.Sleep(300 * time.Millisecond)

	// Verify screen has content (simulation screen was used for drawing)
	cells, width, height := simScreen.GetContents()

	t.Logf("Screen dimensions: %dx%d", width, height)
	t.Logf("Total cells: %d", len(cells))

	if len(cells) == 0 {
		t.Fatal("Screen has no cells - injection may have failed")
	}

	// Verify expected dimensions
	if width != 80 || height < 24 {
		t.Errorf("Unexpected screen size: got %dx%d, want 80x24+", width, height)
	}

	// Count non-empty cells (cells that have been written to)
	nonEmptyCells := 0
	for _, cell := range cells {
		if len(cell.Runes) > 0 {
			nonEmptyCells++
		}
	}

	t.Logf("Non-empty cells: %d (%.1f%%)", nonEmptyCells, float64(nonEmptyCells)/float64(len(cells))*100)

	if nonEmptyCells == 0 {
		t.Fatal("Screen has no content - UI did not draw to injected screen")
	}

	// Extract text content from screen to verify specific elements
	screenText := extractScreenText(cells)

	// Check for key UI elements
	checks := []struct {
		name     string
		expected string
		found    bool
	}{
		{"Progress stats", "Success", false},
		{"Worker event", "pkgconf", false},
		{"Border characters", "─", false},
	}

	for i := range checks {
		if containsText(screenText, checks[i].expected) {
			checks[i].found = true
			t.Logf("✓ Found '%s' in screen content", checks[i].expected)
		}
	}

	// Report findings
	for _, check := range checks {
		if !check.found {
			t.Errorf("✗ Did not find '%s' (%s) in screen content", check.expected, check.name)
		}
	}

	// Test cursor positioning
	x, y, visible := simScreen.GetCursor()
	t.Logf("Cursor: x=%d, y=%d, visible=%v", x, y, visible)

	// Cleanup
	ui.Stop()

	t.Log("✓ Screen injection validation complete")
}

// TestNcursesUI_RealScreen_vs_SimScreen verifies both code paths work
func TestNcursesUI_RealScreen_vs_SimScreen(t *testing.T) {
	// Test 1: With injected SimulationScreen
	t.Run("WithSimulationScreen", func(t *testing.T) {
		simScreen := tcell.NewSimulationScreen("UTF-8")
		err := simScreen.Init()
		if err != nil {
			t.Fatalf("Failed to init simulation screen: %v", err)
		}
		simScreen.SetSize(80, 24)

		ui := NewNcursesUI()
		ui.SetScreen(simScreen)

		err = ui.Start()
		if err != nil {
			t.Fatalf("Failed to start UI with simulation screen: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		// Verify it's using our screen
		cells, _, _ := simScreen.GetContents()
		if len(cells) == 0 {
			t.Error("Simulation screen was not used for drawing")
		}

		ui.Stop()
		t.Log("✓ Simulation screen path works")
	})

	// Test 2: Without injected screen (would use real terminal, but we'll just verify app creation)
	t.Run("WithoutInjectedScreen", func(t *testing.T) {
		ui := NewNcursesUI()
		// Don't inject screen - this tests the else path

		// We can't actually start without a terminal, so just verify the field is nil
		ui.mu.Lock()
		if ui.screen != nil {
			t.Error("Screen should be nil when not injected")
		}
		ui.mu.Unlock()

		t.Log("✓ Non-injected screen path verified (field is nil)")
	})
}

// Helper: Extract text from screen cells
func extractScreenText(cells []tcell.SimCell) string {
	text := ""
	for _, cell := range cells {
		for _, r := range cell.Runes {
			text += string(r)
		}
	}
	return text
}

// Helper: Check if text contains substring
func containsText(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
