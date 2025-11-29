package log

import (
	"fmt"
	"strings"
	"sync"
)

// MemoryLogger captures all log messages in memory for testing.
// Thread-safe for concurrent use.
type MemoryLogger struct {
	mu       sync.Mutex
	messages []LogMessage
}

// LogMessage represents a captured log entry
type LogMessage struct {
	Level   string // "INFO", "DEBUG", "WARN", "ERROR"
	Message string
}

// NewMemoryLogger creates a new MemoryLogger for testing
func NewMemoryLogger() *MemoryLogger {
	return &MemoryLogger{
		messages: make([]LogMessage, 0),
	}
}

func (m *MemoryLogger) Info(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, LogMessage{
		Level:   "INFO",
		Message: fmt.Sprintf(format, args...),
	})
}

func (m *MemoryLogger) Debug(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, LogMessage{
		Level:   "DEBUG",
		Message: fmt.Sprintf(format, args...),
	})
}

func (m *MemoryLogger) Warn(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, LogMessage{
		Level:   "WARN",
		Message: fmt.Sprintf(format, args...),
	})
}

func (m *MemoryLogger) Error(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, LogMessage{
		Level:   "ERROR",
		Message: fmt.Sprintf(format, args...),
	})
}

// GetMessages returns a copy of all captured messages
func (m *MemoryLogger) GetMessages() []LogMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to prevent race conditions
	result := make([]LogMessage, len(m.messages))
	copy(result, m.messages)
	return result
}

// GetMessagesByLevel returns all messages of a specific level
func (m *MemoryLogger) GetMessagesByLevel(level string) []LogMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []LogMessage
	for _, msg := range m.messages {
		if msg.Level == level {
			result = append(result, msg)
		}
	}
	return result
}

// HasMessage checks if any message contains the given substring
func (m *MemoryLogger) HasMessage(substring string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range m.messages {
		if strings.Contains(msg.Message, substring) {
			return true
		}
	}
	return false
}

// HasMessageWithLevel checks if any message at the given level contains the substring
func (m *MemoryLogger) HasMessageWithLevel(level, substring string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range m.messages {
		if msg.Level == level && strings.Contains(msg.Message, substring) {
			return true
		}
	}
	return false
}

// Clear removes all captured messages
func (m *MemoryLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]LogMessage, 0)
}

// Count returns the total number of captured messages
func (m *MemoryLogger) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

// CountByLevel returns the number of messages at a specific level
func (m *MemoryLogger) CountByLevel(level string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, msg := range m.messages {
		if msg.Level == level {
			count++
		}
	}
	return count
}

// String returns a formatted string of all messages (useful for debugging tests)
func (m *MemoryLogger) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sb strings.Builder
	for i, msg := range m.messages {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, msg.Level, msg.Message))
	}
	return sb.String()
}
