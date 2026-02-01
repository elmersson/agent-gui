package session

import (
	"encoding/json"
	"os"
	"path/filepath"
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
