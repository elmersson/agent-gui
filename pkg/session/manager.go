package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
)

type Session struct {
	ID         string           `json:"id"`
	AgentName  string           `json:"agent_name"`
	Input      string           `json:"input"`
	Output     string           `json:"output"`
	StartTime  time.Time        `json:"start_time"`
	EndTime    *time.Time       `json:"end_time,omitempty"`
	Events     []events.Event   `json:"events"`
	TokenUsage agent.TokenUsage `json:"token_usage"`
}

type Manager struct {
	sessionsDir string
	current     *Session
}

func NewManager(sessionsDir string) (*Manager, error) {
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return nil, err
	}

	return &Manager{
		sessionsDir: sessionsDir,
	}, nil
}

func (m *Manager) StartSession(agentName, input string) {
	m.current = &Session{
		ID:        time.Now().Format("20060102-150405"),
		AgentName: agentName,
		Input:     input,
		Output:    "",
		StartTime: time.Now(),
		Events:    []events.Event{},
	}
}

func (m *Manager) AppendOutput(chunk string) {
	if m.current != nil {
		m.current.Output += chunk
	}
}

func (m *Manager) AddEvent(event events.Event) {
	if m.current != nil {
		m.current.Events = append(m.current.Events, event)
	}
}

// UpdateTokenUsage updates the token usage for the current session
func (m *Manager) UpdateTokenUsage(usage agent.TokenUsage) {
	if m.current != nil {
		m.current.TokenUsage = usage
	}
}

// GetTokenUsage returns the current session's token usage
func (m *Manager) GetTokenUsage() agent.TokenUsage {
	if m.current != nil {
		return m.current.TokenUsage
	}
	return agent.TokenUsage{}
}

func (m *Manager) EndSession() error {
	if m.current == nil {
		return nil
	}

	now := time.Now()
	m.current.EndTime = &now

	filePath := filepath.Join(m.sessionsDir, m.current.ID+".json")
	data, err := json.MarshalIndent(m.current, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	m.current = nil
	return nil
}

// ListSessions returns a list of all session IDs, sorted by most recent first
func (m *Manager) ListSessions() ([]string, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var sessions []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		// Remove .json extension to get session ID
		sessionID := strings.TrimSuffix(entry.Name(), ".json")
		sessions = append(sessions, sessionID)
	}

	// Sort by newest first (session IDs are timestamps)
	sort.Sort(sort.Reverse(sort.StringSlice(sessions)))

	return sessions, nil
}

// LoadSession loads a session by ID
func (m *Manager) LoadSession(sessionID string) (*Session, error) {
	filePath := filepath.Join(m.sessionsDir, sessionID+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// GetSessionsDir returns the sessions directory path
func (m *Manager) GetSessionsDir() string {
	return m.sessionsDir
}
