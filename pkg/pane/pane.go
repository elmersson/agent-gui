package pane

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/agent"
)

// PaneType indicates the type of content displayed in a pane
type PaneType string

const (
	PaneTypeAgent   PaneType = "agent"   // Displays agent output stream
	PaneTypeOutput  PaneType = "output"  // Generic output display
	PaneTypeReplay  PaneType = "replay"  // Replay mode
	PaneTypeStatus  PaneType = "status"  // Status/info display
	PaneTypeCommand PaneType = "command" // Command input pane
)

// Pane represents a single display pane that renders events from an agent stream.
// Panes are read-only views - they never control or mutate agent state.
type Pane struct {
	ID   string   `json:"id"`
	Type PaneType `json:"type"`

	// Agent binding (for agent panes)
	AgentName string `json:"agent_name,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`

	// Display state (not serialized - rebuilt during replay)
	Title       string           `json:"-"`
	Content     strings.Builder  `json:"-"`
	RawContent  string           `json:"-"`
	TokenUsage  agent.TokenUsage `json:"-"`
	State       string           `json:"-"` // idle, running, paused
	ScrollPos   int              `json:"-"` // Viewport scroll position
	ScrollMax   int              `json:"-"` // Maximum scroll position
	Focused     bool             `json:"-"` // Whether this pane is focused
	LastUpdated time.Time        `json:"-"` // Last content update time

	// Dimensions (set by layout manager)
	Rect Rect `json:"-"`

	mu sync.RWMutex
}

// NewPane creates a new pane with the given ID and type
func NewPane(id string, paneType PaneType) *Pane {
	return &Pane{
		ID:          id,
		Type:        paneType,
		State:       "idle",
		LastUpdated: time.Now(),
	}
}

// NewAgentPane creates a new pane bound to an agent
func NewAgentPane(id, agentName string) *Pane {
	p := NewPane(id, PaneTypeAgent)
	p.AgentName = agentName
	p.Title = agentName
	return p
}

// BindAgent binds this pane to an agent stream
func (p *Pane) BindAgent(agentName, agentID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.AgentName = agentName
	p.AgentID = agentID
	p.Title = agentName
	p.Type = PaneTypeAgent
}

// UnbindAgent removes the agent binding
func (p *Pane) UnbindAgent() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.AgentName = ""
	p.AgentID = ""
	p.Title = ""
}

// AppendContent adds content to the pane
func (p *Pane) AppendContent(text string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Content.WriteString(text)
	p.RawContent = p.Content.String()
	p.LastUpdated = time.Now()
}

// SetContent replaces the pane content
func (p *Pane) SetContent(text string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Content.Reset()
	p.Content.WriteString(text)
	p.RawContent = text
	p.LastUpdated = time.Now()
}

// GetContent returns the current pane content
func (p *Pane) GetContent() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.RawContent
}

// ClearContent clears the pane content
func (p *Pane) ClearContent() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Content.Reset()
	p.RawContent = ""
	p.LastUpdated = time.Now()
}

// SetState updates the pane state
func (p *Pane) SetState(state string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.State = state
}

// GetState returns the current pane state
func (p *Pane) GetState() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State
}

// SetTokenUsage updates the token usage display
func (p *Pane) SetTokenUsage(usage agent.TokenUsage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.TokenUsage = usage
}

// GetTokenUsage returns the current token usage
func (p *Pane) GetTokenUsage() agent.TokenUsage {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.TokenUsage
}

// SetRect updates the pane's display rectangle
func (p *Pane) SetRect(rect Rect) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Rect = rect
}

// GetRect returns the pane's display rectangle
func (p *Pane) GetRect() Rect {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Rect
}

// SetFocused sets the focus state
func (p *Pane) SetFocused(focused bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Focused = focused
}

// IsFocused returns true if this pane is focused
func (p *Pane) IsFocused() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Focused
}

// SetTitle sets the pane title
func (p *Pane) SetTitle(title string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Title = title
}

// GetTitle returns the pane title
func (p *Pane) GetTitle() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Title != "" {
		return p.Title
	}
	return p.AgentName
}

// ScrollUp scrolls the pane content up
func (p *Pane) ScrollUp(lines int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ScrollPos -= lines
	if p.ScrollPos < 0 {
		p.ScrollPos = 0
	}
}

// ScrollDown scrolls the pane content down
func (p *Pane) ScrollDown(lines int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ScrollPos += lines
	if p.ScrollPos > p.ScrollMax {
		p.ScrollPos = p.ScrollMax
	}
}

// ScrollToBottom scrolls to the bottom of the content
func (p *Pane) ScrollToBottom() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ScrollPos = p.ScrollMax
}

// ScrollToTop scrolls to the top of the content
func (p *Pane) ScrollToTop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ScrollPos = 0
}

// SetScrollMax sets the maximum scroll position
func (p *Pane) SetScrollMax(max int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ScrollMax = max
	if p.ScrollPos > max {
		p.ScrollPos = max
	}
}

// GetScrollPos returns the current scroll position
func (p *Pane) GetScrollPos() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ScrollPos
}

// Clone creates a copy of the pane (for serialization)
func (p *Pane) Clone() *Pane {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &Pane{
		ID:        p.ID,
		Type:      p.Type,
		AgentName: p.AgentName,
		AgentID:   p.AgentID,
	}
}

// PaneSnapshot represents a serializable snapshot of a pane's state
type PaneSnapshot struct {
	ID         string           `json:"id"`
	Type       PaneType         `json:"type"`
	AgentName  string           `json:"agent_name,omitempty"`
	AgentID    string           `json:"agent_id,omitempty"`
	Title      string           `json:"title,omitempty"`
	Content    string           `json:"content,omitempty"`
	State      string           `json:"state"`
	TokenUsage agent.TokenUsage `json:"token_usage,omitempty"`
}

// Snapshot creates a serializable snapshot of the pane
func (p *Pane) Snapshot() PaneSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PaneSnapshot{
		ID:         p.ID,
		Type:       p.Type,
		AgentName:  p.AgentName,
		AgentID:    p.AgentID,
		Title:      p.Title,
		Content:    p.RawContent,
		State:      p.State,
		TokenUsage: p.TokenUsage,
	}
}

// RestoreFromSnapshot restores pane state from a snapshot
func (p *Pane) RestoreFromSnapshot(snap PaneSnapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ID = snap.ID
	p.Type = snap.Type
	p.AgentName = snap.AgentName
	p.AgentID = snap.AgentID
	p.Title = snap.Title
	p.Content.Reset()
	p.Content.WriteString(snap.Content)
	p.RawContent = snap.Content
	p.State = snap.State
	p.TokenUsage = snap.TokenUsage
	p.LastUpdated = time.Now()
}

// Pane ID generation
var paneIDCounter int

func generatePaneID() string {
	paneIDCounter++
	return fmt.Sprintf("pane-%d", paneIDCounter)
}

// GeneratePaneID generates a unique pane ID
func GeneratePaneID() string {
	return generatePaneID()
}

// ResetPaneIDCounter resets the pane ID counter (mainly for testing)
func ResetPaneIDCounter() {
	paneIDCounter = 0
}
