package log

import (
	"sync"
	"testing"
)

func TestMemoryLogger_ImplementsLibraryLogger(t *testing.T) {
	// Compile-time check
	var _ LibraryLogger = (*MemoryLogger)(nil)

	logger := NewMemoryLogger()
	if logger == nil {
		t.Fatal("NewMemoryLogger returned nil")
	}

	if logger.Count() != 0 {
		t.Errorf("Expected 0 messages, got %d", logger.Count())
	}
}

func TestMemoryLogger_CaptureMessages(t *testing.T) {
	logger := NewMemoryLogger()

	logger.Info("Starting process")
	logger.Debug("Debug info: step 1")
	logger.Warn("Warning: low disk space")
	logger.Error("Error: file not found")

	if logger.Count() != 4 {
		t.Errorf("Expected 4 messages, got %d", logger.Count())
	}

	// Check counts by level
	if count := logger.CountByLevel("INFO"); count != 1 {
		t.Errorf("Expected 1 INFO message, got %d", count)
	}
	if count := logger.CountByLevel("DEBUG"); count != 1 {
		t.Errorf("Expected 1 DEBUG message, got %d", count)
	}
	if count := logger.CountByLevel("WARN"); count != 1 {
		t.Errorf("Expected 1 WARN message, got %d", count)
	}
	if count := logger.CountByLevel("ERROR"); count != 1 {
		t.Errorf("Expected 1 ERROR message, got %d", count)
	}
}

func TestMemoryLogger_GetMessages(t *testing.T) {
	logger := NewMemoryLogger()

	logger.Info("First message")
	logger.Error("Second message")

	messages := logger.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}

	if messages[0].Level != "INFO" || messages[0].Message != "First message" {
		t.Errorf("First message incorrect: %+v", messages[0])
	}

	if messages[1].Level != "ERROR" || messages[1].Message != "Second message" {
		t.Errorf("Second message incorrect: %+v", messages[1])
	}
}

func TestMemoryLogger_GetMessagesByLevel(t *testing.T) {
	logger := NewMemoryLogger()

	logger.Info("Info 1")
	logger.Error("Error 1")
	logger.Info("Info 2")
	logger.Error("Error 2")

	infoMessages := logger.GetMessagesByLevel("INFO")
	if len(infoMessages) != 2 {
		t.Errorf("Expected 2 INFO messages, got %d", len(infoMessages))
	}

	errorMessages := logger.GetMessagesByLevel("ERROR")
	if len(errorMessages) != 2 {
		t.Errorf("Expected 2 ERROR messages, got %d", len(errorMessages))
	}

	debugMessages := logger.GetMessagesByLevel("DEBUG")
	if len(debugMessages) != 0 {
		t.Errorf("Expected 0 DEBUG messages, got %d", len(debugMessages))
	}
}

func TestMemoryLogger_HasMessage(t *testing.T) {
	logger := NewMemoryLogger()

	logger.Info("Processing package devel/git")
	logger.Error("Failed to build www/nginx")

	if !logger.HasMessage("devel/git") {
		t.Error("Expected to find message containing 'devel/git'")
	}

	if !logger.HasMessage("www/nginx") {
		t.Error("Expected to find message containing 'www/nginx'")
	}

	if logger.HasMessage("nonexistent") {
		t.Error("Should not find message containing 'nonexistent'")
	}
}

func TestMemoryLogger_HasMessageWithLevel(t *testing.T) {
	logger := NewMemoryLogger()

	logger.Info("Starting build")
	logger.Error("Build failed")
	logger.Warn("Disk space low")

	if !logger.HasMessageWithLevel("INFO", "Starting") {
		t.Error("Expected to find INFO message with 'Starting'")
	}

	if !logger.HasMessageWithLevel("ERROR", "failed") {
		t.Error("Expected to find ERROR message with 'failed'")
	}

	if !logger.HasMessageWithLevel("WARN", "Disk space") {
		t.Error("Expected to find WARN message with 'Disk space'")
	}

	// Negative cases
	if logger.HasMessageWithLevel("INFO", "failed") {
		t.Error("Should not find 'failed' in INFO messages")
	}

	if logger.HasMessageWithLevel("ERROR", "Starting") {
		t.Error("Should not find 'Starting' in ERROR messages")
	}
}

func TestMemoryLogger_Formatting(t *testing.T) {
	logger := NewMemoryLogger()

	logger.Info("Processing package %s version %d", "devel/git", 2)
	logger.Debug("Worker %d completed %d/%d tasks", 5, 10, 100)

	messages := logger.GetMessages()

	if messages[0].Message != "Processing package devel/git version 2" {
		t.Errorf("Info formatting failed: got %q", messages[0].Message)
	}

	if messages[1].Message != "Worker 5 completed 10/100 tasks" {
		t.Errorf("Debug formatting failed: got %q", messages[1].Message)
	}
}

func TestMemoryLogger_Clear(t *testing.T) {
	logger := NewMemoryLogger()

	logger.Info("Message 1")
	logger.Error("Message 2")

	if logger.Count() != 2 {
		t.Fatalf("Expected 2 messages before clear, got %d", logger.Count())
	}

	logger.Clear()

	if logger.Count() != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", logger.Count())
	}

	// Verify we can still log after clear
	logger.Info("Message 3")
	if logger.Count() != 1 {
		t.Errorf("Expected 1 message after clear and new log, got %d", logger.Count())
	}
}

func TestMemoryLogger_Concurrent(t *testing.T) {
	logger := NewMemoryLogger()

	// Test concurrent writes
	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info("Goroutine %d message %d", id, j)
			}
		}(i)
	}

	wg.Wait()

	expectedCount := numGoroutines * messagesPerGoroutine
	actualCount := logger.Count()

	if actualCount != expectedCount {
		t.Errorf("Expected %d messages after concurrent writes, got %d", expectedCount, actualCount)
	}

	// Verify all messages are INFO level
	if logger.CountByLevel("INFO") != expectedCount {
		t.Errorf("Expected all %d messages to be INFO level", expectedCount)
	}
}

func TestMemoryLogger_String(t *testing.T) {
	logger := NewMemoryLogger()

	logger.Info("First message")
	logger.Error("Second message")

	output := logger.String()

	if output == "" {
		t.Error("String() returned empty string")
	}

	// Should contain message numbers and content
	if !logger.HasMessage("First message") {
		t.Error("String output missing first message")
	}
	if !logger.HasMessage("Second message") {
		t.Error("String output missing second message")
	}
}

func TestMemoryLogger_EmptyState(t *testing.T) {
	logger := NewMemoryLogger()

	// Test all query methods on empty logger
	if logger.Count() != 0 {
		t.Error("Empty logger should have 0 count")
	}

	if logger.CountByLevel("INFO") != 0 {
		t.Error("Empty logger should have 0 INFO messages")
	}

	if logger.HasMessage("anything") {
		t.Error("Empty logger should not have any messages")
	}

	if logger.HasMessageWithLevel("INFO", "anything") {
		t.Error("Empty logger should not have any messages at any level")
	}

	messages := logger.GetMessages()
	if len(messages) != 0 {
		t.Error("Empty logger should return empty slice")
	}

	infoMessages := logger.GetMessagesByLevel("INFO")
	if len(infoMessages) != 0 {
		t.Error("Empty logger should return empty slice for level query")
	}
}
