package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine("test-sessions")

	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}

	if engine.sessionsDir != "test-sessions" {
		t.Errorf("Expected sessionsDir 'test-sessions', got '%s'", engine.sessionsDir)
	}

	if engine.state != StateStopped {
		t.Errorf("Expected initial state StateStopped, got %v", engine.state)
	}

	if engine.speed != 1.0 {
		t.Errorf("Expected initial speed 1.0, got %v", engine.speed)
	}
}

func TestListSessions(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "replay-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engine := NewEngine(tmpDir)

	// Initially empty
	sessions, err := engine.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(sessions))
	}

	// Create test sessions
	now := time.Now()
	session1 := Session{
		ID:        "20260201-100000",
		AgentName: "test",
		Input:     "hello",
		Output:    "world",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   func() *time.Time { t := now.Add(-50 * time.Minute); return &t }(),
	}

	session2 := Session{
		ID:        "20260201-110000",
		AgentName: "test",
		Input:     "foo",
		Output:    "bar",
		StartTime: now.Add(-30 * time.Minute),
		EndTime:   func() *time.Time { t := now.Add(-20 * time.Minute); return &t }(),
	}

	// Write sessions to disk
	for _, s := range []Session{session1, session2} {
		data, _ := json.Marshal(s)
		os.WriteFile(filepath.Join(tmpDir, s.ID+".json"), data, 0644)
	}

	// List again
	sessions, err = engine.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}

	// Should be sorted newest first
	if sessions[0].ID != "20260201-110000" {
		t.Errorf("Expected newest session first, got %s", sessions[0].ID)
	}
}

func TestLoadSession(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "replay-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engine := NewEngine(tmpDir)

	// Create test session
	now := time.Now()
	endTime := now.Add(10 * time.Minute)
	testSession := Session{
		ID:        "test-session-001",
		AgentName: "test-agent",
		Input:     "test input",
		Output:    "test output response",
		StartTime: now,
		EndTime:   &endTime,
		Events: []events.Event{
			{Type: events.EventAgentStarted, Timestamp: now},
			{Type: events.EventOutputChunk, Timestamp: now.Add(5 * time.Second), Data: "test output"},
		},
		TokenUsage: agent.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}

	// Write session to disk
	data, _ := json.MarshalIndent(testSession, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, testSession.ID+".json"), data, 0644)

	// Load session
	err = engine.LoadSession("test-session-001")
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	session := engine.GetSession()
	if session == nil {
		t.Fatal("GetSession returned nil after LoadSession")
	}

	if session.ID != "test-session-001" {
		t.Errorf("Expected ID 'test-session-001', got '%s'", session.ID)
	}

	if session.Output != "test output response" {
		t.Errorf("Expected output 'test output response', got '%s'", session.Output)
	}

	// Check duration
	duration := engine.GetDuration()
	expectedDuration := 10 * time.Minute
	if duration != expectedDuration {
		t.Errorf("Expected duration %v, got %v", expectedDuration, duration)
	}
}

func TestLoadSessionNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engine := NewEngine(tmpDir)

	err = engine.LoadSession("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent session, got nil")
	}
}

func TestPlaybackState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engine := NewEngine(tmpDir)

	// Initial state
	if engine.GetState() != StateStopped {
		t.Errorf("Expected initial state StateStopped, got %v", engine.GetState())
	}

	// Cannot play without session
	err = engine.Play()
	if err == nil {
		t.Error("Expected error when playing without session")
	}

	// Create and load a session
	now := time.Now()
	endTime := now.Add(5 * time.Second)
	testSession := Session{
		ID:        "play-test",
		AgentName: "test",
		Input:     "test",
		Output:    "output",
		StartTime: now,
		EndTime:   &endTime,
	}

	data, _ := json.MarshalIndent(testSession, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, testSession.ID+".json"), data, 0644)

	engine.LoadSession("play-test")

	// Play
	err = engine.Play()
	if err != nil {
		t.Fatalf("Play failed: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	if engine.GetState() != StatePlaying {
		t.Errorf("Expected state StatePlaying, got %v", engine.GetState())
	}

	// Pause
	engine.Pause()
	time.Sleep(50 * time.Millisecond)
	if engine.GetState() != StatePaused {
		t.Errorf("Expected state StatePaused, got %v", engine.GetState())
	}

	// Stop
	engine.Stop()
	time.Sleep(50 * time.Millisecond)
	if engine.GetState() != StateStopped {
		t.Errorf("Expected state StateStopped, got %v", engine.GetState())
	}
}

func TestSeek(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engine := NewEngine(tmpDir)

	// Create and load a session
	now := time.Now()
	endTime := now.Add(1 * time.Minute)
	testSession := Session{
		ID:        "seek-test",
		AgentName: "test",
		Input:     "test",
		Output:    "Hello World",
		StartTime: now,
		EndTime:   &endTime,
	}

	data, _ := json.MarshalIndent(testSession, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, testSession.ID+".json"), data, 0644)

	engine.LoadSession("seek-test")

	// Seek to middle
	engine.Seek(30 * time.Second)
	if engine.GetPosition() != 30*time.Second {
		t.Errorf("Expected position 30s, got %v", engine.GetPosition())
	}

	// Seek beyond end
	engine.Seek(2 * time.Minute)
	if engine.GetPosition() != 1*time.Minute {
		t.Errorf("Expected position clamped to 1m, got %v", engine.GetPosition())
	}

	// Seek before start
	engine.Seek(-10 * time.Second)
	if engine.GetPosition() != 0 {
		t.Errorf("Expected position clamped to 0, got %v", engine.GetPosition())
	}
}

func TestSeekPercent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engine := NewEngine(tmpDir)

	now := time.Now()
	endTime := now.Add(100 * time.Second)
	testSession := Session{
		ID:        "seek-percent-test",
		AgentName: "test",
		Input:     "test",
		Output:    "output",
		StartTime: now,
		EndTime:   &endTime,
	}

	data, _ := json.MarshalIndent(testSession, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, testSession.ID+".json"), data, 0644)

	engine.LoadSession("seek-percent-test")

	// Seek to 50%
	engine.SeekPercent(0.5)
	if engine.GetPosition() != 50*time.Second {
		t.Errorf("Expected position 50s at 50%%, got %v", engine.GetPosition())
	}

	// Seek to 100%
	engine.SeekPercent(1.0)
	if engine.GetPosition() != 100*time.Second {
		t.Errorf("Expected position 100s at 100%%, got %v", engine.GetPosition())
	}
}

func TestSetSpeed(t *testing.T) {
	engine := NewEngine("test")

	// Initial speed
	if engine.GetSpeed() != 1.0 {
		t.Errorf("Expected initial speed 1.0, got %v", engine.GetSpeed())
	}

	// Set speed
	engine.SetSpeed(2.0)
	if engine.GetSpeed() != 2.0 {
		t.Errorf("Expected speed 2.0, got %v", engine.GetSpeed())
	}

	// Clamp to max
	engine.SetSpeed(20.0)
	if engine.GetSpeed() != 10.0 {
		t.Errorf("Expected speed clamped to 10.0, got %v", engine.GetSpeed())
	}

	// Clamp to min
	engine.SetSpeed(0.01)
	if engine.GetSpeed() != 0.1 {
		t.Errorf("Expected speed clamped to 0.1, got %v", engine.GetSpeed())
	}
}

func TestObserver(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engine := NewEngine(tmpDir)

	eventCount := 0
	engine.AddObserver(func(event ReplayEvent) {
		eventCount++
	})

	// Create and load a session
	now := time.Now()
	endTime := now.Add(1 * time.Second)
	testSession := Session{
		ID:        "observer-test",
		AgentName: "test",
		Input:     "test",
		Output:    "output",
		StartTime: now,
		EndTime:   &endTime,
	}

	data, _ := json.MarshalIndent(testSession, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, testSession.ID+".json"), data, 0644)

	engine.LoadSession("observer-test")

	// Should have received session_loaded event
	if eventCount == 0 {
		t.Error("Expected at least one event from observer")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "00:00.00"},
		{1 * time.Second, "00:01.00"},
		{1*time.Minute + 30*time.Second, "01:30.00"},
		{5*time.Minute + 45*time.Second + 500*time.Millisecond, "05:45.50"},
	}

	for _, tt := range tests {
		result := FormatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("FormatDuration(%v) = %s, expected %s", tt.duration, result, tt.expected)
		}
	}
}

func TestFormatProgress(t *testing.T) {
	current := 1*time.Minute + 30*time.Second
	total := 5 * time.Minute

	result := FormatProgress(current, total)
	expected := "01:30.00 / 05:00.00"

	if result != expected {
		t.Errorf("FormatProgress(%v, %v) = %s, expected %s", current, total, result, expected)
	}
}

func TestOutputAtPosition(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engine := NewEngine(tmpDir)

	now := time.Now()
	endTime := now.Add(10 * time.Second)
	testSession := Session{
		ID:        "output-position-test",
		AgentName: "test",
		Input:     "test",
		Output:    "Hello World!", // 12 characters
		StartTime: now,
		EndTime:   &endTime,
	}

	data, _ := json.MarshalIndent(testSession, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, testSession.ID+".json"), data, 0644)

	engine.LoadSession("output-position-test")

	// At beginning (0%)
	output := engine.outputAtPosition(0)
	if len(output) != 0 {
		t.Errorf("Expected empty output at beginning, got '%s'", output)
	}

	// At 50%
	output = engine.outputAtPosition(5 * time.Second)
	expectedLen := 6 // Approximately half of 12
	if len(output) != expectedLen {
		t.Errorf("Expected ~%d characters at 50%%, got %d ('%s')", expectedLen, len(output), output)
	}

	// At end (100%)
	output = engine.outputAtPosition(10 * time.Second)
	if output != "Hello World!" {
		t.Errorf("Expected full output at end, got '%s'", output)
	}
}
