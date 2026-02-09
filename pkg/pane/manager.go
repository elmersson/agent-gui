package pane

import (
	"fmt"
	"sync"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/events"
)

// Manager manages panes and their layout.
// It provides the primary interface for pane operations and emits events for all changes.
type Manager struct {
	// Layout tree
	layout *LayoutTree

	// Pane storage
	panes map[string]*Pane

	// Currently focused pane
	focusedPaneID string

	// Event bus for emitting layout events
	bus events.Bus

	mu sync.RWMutex
}

// NewManager creates a new pane manager
func NewManager(bus events.Bus) *Manager {
	return &Manager{
		panes: make(map[string]*Pane),
		bus:   bus,
	}
}

// Initialize creates the initial layout with a single pane
func (m *Manager) Initialize(agentName string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create the initial pane
	paneID := GeneratePaneID()
	pane := NewAgentPane(paneID, agentName)
	m.panes[paneID] = pane

	// Create the layout tree
	m.layout = NewLayoutTree(paneID)
	m.focusedPaneID = paneID
	pane.SetFocused(true)

	// Emit event
	m.emitEvent(events.Event{
		Type:      events.EventPaneCreated,
		AgentName: agentName,
		Data: map[string]any{
			"pane_id":   paneID,
			"pane_type": string(PaneTypeAgent),
		},
	})

	m.emitEvent(events.Event{
		Type: events.EventLayoutChanged,
		Data: map[string]any{
			"action":    "initialize",
			"pane_id":   paneID,
			"num_panes": 1,
		},
	})

	return paneID
}

// CreatePane creates a new pane (not added to layout yet)
func (m *Manager) CreatePane(paneType PaneType, agentName string) *Pane {
	m.mu.Lock()
	defer m.mu.Unlock()

	paneID := GeneratePaneID()
	var pane *Pane

	if paneType == PaneTypeAgent && agentName != "" {
		pane = NewAgentPane(paneID, agentName)
	} else {
		pane = NewPane(paneID, paneType)
	}

	m.panes[paneID] = pane

	// Emit event
	m.emitEvent(events.Event{
		Type:      events.EventPaneCreated,
		AgentName: agentName,
		Data: map[string]any{
			"pane_id":   paneID,
			"pane_type": string(paneType),
		},
	})

	return pane
}

// SplitPane splits an existing pane horizontally or vertically
func (m *Manager) SplitPane(paneID string, direction SplitDirection, newAgentName string) (*Pane, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.layout == nil {
		return nil, fmt.Errorf("layout not initialized")
	}

	// Verify source pane exists
	if _, ok := m.panes[paneID]; !ok {
		return nil, fmt.Errorf("pane not found: %s", paneID)
	}

	// Create the new pane
	newPaneID := GeneratePaneID()
	newPane := NewAgentPane(newPaneID, newAgentName)
	m.panes[newPaneID] = newPane

	// Split the layout
	if err := m.layout.SplitPane(paneID, direction, newPaneID, 0.5); err != nil {
		delete(m.panes, newPaneID)
		return nil, err
	}

	// Recalculate layout
	m.layout.CalculateLayout()
	m.updatePaneRects()

	// Emit events
	m.emitEvent(events.Event{
		Type:      events.EventPaneCreated,
		AgentName: newAgentName,
		Data: map[string]any{
			"pane_id":   newPaneID,
			"pane_type": string(PaneTypeAgent),
		},
	})

	m.emitEvent(events.Event{
		Type: events.EventPaneSplit,
		Data: map[string]any{
			"source_pane_id": paneID,
			"new_pane_id":    newPaneID,
			"direction":      direction.String(),
			"num_panes":      m.layout.CountPanes(),
		},
	})

	m.emitEvent(events.Event{
		Type: events.EventLayoutChanged,
		Data: map[string]any{
			"action":    "split",
			"pane_id":   newPaneID,
			"num_panes": m.layout.CountPanes(),
		},
	})

	return newPane, nil
}

// SplitHorizontal splits a pane horizontally (left/right)
func (m *Manager) SplitHorizontal(paneID string, newAgentName string) (*Pane, error) {
	return m.SplitPane(paneID, SplitHorizontal, newAgentName)
}

// SplitVertical splits a pane vertically (top/bottom)
func (m *Manager) SplitVertical(paneID string, newAgentName string) (*Pane, error) {
	return m.SplitPane(paneID, SplitVertical, newAgentName)
}

// ClosePane removes a pane from the layout
func (m *Manager) ClosePane(paneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.layout == nil {
		return fmt.Errorf("layout not initialized")
	}

	pane, ok := m.panes[paneID]
	if !ok {
		return fmt.Errorf("pane not found: %s", paneID)
	}

	// Can't close the last pane
	if m.layout.CountPanes() <= 1 {
		return fmt.Errorf("cannot close the last pane")
	}

	// Remove from layout
	if err := m.layout.RemovePane(paneID); err != nil {
		return err
	}

	// If this was the focused pane, focus another one
	if m.focusedPaneID == paneID {
		paneIDs := m.layout.GetAllPaneIDs()
		if len(paneIDs) > 0 {
			m.focusedPaneID = paneIDs[0]
			if p, ok := m.panes[m.focusedPaneID]; ok {
				p.SetFocused(true)
			}
		}
	}

	// Remove from storage
	delete(m.panes, paneID)

	// Recalculate layout
	m.layout.CalculateLayout()
	m.updatePaneRects()

	// Emit events
	m.emitEvent(events.Event{
		Type:      events.EventPaneClosed,
		AgentName: pane.AgentName,
		Data: map[string]any{
			"pane_id":   paneID,
			"num_panes": m.layout.CountPanes(),
		},
	})

	m.emitEvent(events.Event{
		Type: events.EventLayoutChanged,
		Data: map[string]any{
			"action":    "close",
			"pane_id":   paneID,
			"num_panes": m.layout.CountPanes(),
		},
	})

	return nil
}

// FocusPane sets the focused pane
func (m *Manager) FocusPane(paneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pane, ok := m.panes[paneID]
	if !ok {
		return fmt.Errorf("pane not found: %s", paneID)
	}

	// Unfocus current
	if current, ok := m.panes[m.focusedPaneID]; ok && m.focusedPaneID != paneID {
		current.SetFocused(false)
	}

	// Focus new
	pane.SetFocused(true)
	m.focusedPaneID = paneID

	// Emit event
	m.emitEvent(events.Event{
		Type:      events.EventPaneFocused,
		AgentName: pane.AgentName,
		Data: map[string]any{
			"pane_id": paneID,
		},
	})

	return nil
}

// FocusNext moves focus to the next pane
func (m *Manager) FocusNext() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.layout == nil {
		return fmt.Errorf("layout not initialized")
	}

	paneIDs := m.layout.GetAllPaneIDs()
	if len(paneIDs) == 0 {
		return fmt.Errorf("no panes")
	}

	// Find current index
	currentIdx := 0
	for i, id := range paneIDs {
		if id == m.focusedPaneID {
			currentIdx = i
			break
		}
	}

	// Move to next
	nextIdx := (currentIdx + 1) % len(paneIDs)
	nextPaneID := paneIDs[nextIdx]

	// Unfocus current
	if current, ok := m.panes[m.focusedPaneID]; ok {
		current.SetFocused(false)
	}

	// Focus next
	m.focusedPaneID = nextPaneID
	if next, ok := m.panes[nextPaneID]; ok {
		next.SetFocused(true)

		m.emitEvent(events.Event{
			Type:      events.EventPaneFocused,
			AgentName: next.AgentName,
			Data: map[string]any{
				"pane_id": nextPaneID,
			},
		})
	}

	return nil
}

// FocusPrev moves focus to the previous pane
func (m *Manager) FocusPrev() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.layout == nil {
		return fmt.Errorf("layout not initialized")
	}

	paneIDs := m.layout.GetAllPaneIDs()
	if len(paneIDs) == 0 {
		return fmt.Errorf("no panes")
	}

	// Find current index
	currentIdx := 0
	for i, id := range paneIDs {
		if id == m.focusedPaneID {
			currentIdx = i
			break
		}
	}

	// Move to previous
	prevIdx := (currentIdx - 1 + len(paneIDs)) % len(paneIDs)
	prevPaneID := paneIDs[prevIdx]

	// Unfocus current
	if current, ok := m.panes[m.focusedPaneID]; ok {
		current.SetFocused(false)
	}

	// Focus previous
	m.focusedPaneID = prevPaneID
	if prev, ok := m.panes[prevPaneID]; ok {
		prev.SetFocused(true)

		m.emitEvent(events.Event{
			Type:      events.EventPaneFocused,
			AgentName: prev.AgentName,
			Data: map[string]any{
				"pane_id": prevPaneID,
			},
		})
	}

	return nil
}

// ResizePane adjusts the split ratio at a pane's parent node
func (m *Manager) ResizePane(paneID string, delta float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.layout == nil {
		return fmt.Errorf("layout not initialized")
	}

	node := m.layout.FindNodeByPaneID(paneID)
	if node == nil {
		return fmt.Errorf("pane not found in layout: %s", paneID)
	}

	// Find parent and adjust ratio
	// This is a simplified implementation - a full version would need
	// to find the parent node and adjust ratios properly
	parent := node.parent
	if parent == nil {
		return fmt.Errorf("cannot resize root pane")
	}

	if len(parent.Ratios) == 0 {
		return fmt.Errorf("parent has no ratios to adjust")
	}

	// Adjust first ratio by delta
	newRatio := parent.Ratios[0] + delta
	if newRatio < 0.1 {
		newRatio = 0.1
	}
	if newRatio > 0.9 {
		newRatio = 0.9
	}
	parent.Ratios[0] = newRatio

	// Recalculate layout
	m.layout.CalculateLayout()
	m.updatePaneRects()

	// Emit event
	m.emitEvent(events.Event{
		Type: events.EventPaneResized,
		Data: map[string]any{
			"pane_id":   paneID,
			"new_ratio": newRatio,
		},
	})

	m.emitEvent(events.Event{
		Type: events.EventLayoutChanged,
		Data: map[string]any{
			"action":    "resize",
			"pane_id":   paneID,
			"num_panes": m.layout.CountPanes(),
		},
	})

	return nil
}

// SetDimensions updates the overall layout dimensions
func (m *Manager) SetDimensions(width, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.layout == nil {
		return
	}

	m.layout.SetDimensions(width, height)
	m.updatePaneRects()
}

// updatePaneRects updates all pane rectangles from the layout
func (m *Manager) updatePaneRects() {
	if m.layout == nil {
		return
	}

	rects := m.layout.CalculateLayout()
	for paneID, rect := range rects {
		if pane, ok := m.panes[paneID]; ok {
			pane.SetRect(rect)
		}
	}
}

// GetPane returns a pane by ID
func (m *Manager) GetPane(paneID string) *Pane {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.panes[paneID]
}

// GetFocusedPane returns the currently focused pane
func (m *Manager) GetFocusedPane() *Pane {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.panes[m.focusedPaneID]
}

// GetFocusedPaneID returns the ID of the currently focused pane
func (m *Manager) GetFocusedPaneID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.focusedPaneID
}

// GetAllPanes returns all panes
func (m *Manager) GetAllPanes() []*Pane {
	m.mu.RLock()
	defer m.mu.RUnlock()

	panes := make([]*Pane, 0, len(m.panes))
	for _, pane := range m.panes {
		panes = append(panes, pane)
	}
	return panes
}

// GetPaneByAgent returns the pane bound to a specific agent
func (m *Manager) GetPaneByAgent(agentName string) *Pane {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pane := range m.panes {
		if pane.AgentName == agentName {
			return pane
		}
	}
	return nil
}

// GetLayout returns the layout tree
func (m *Manager) GetLayout() *LayoutTree {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.layout
}

// CountPanes returns the number of panes
func (m *Manager) CountPanes() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.layout == nil {
		return 0
	}
	return m.layout.CountPanes()
}

// BindPaneToAgent binds a pane to an agent
func (m *Manager) BindPaneToAgent(paneID, agentName, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pane, ok := m.panes[paneID]
	if !ok {
		return fmt.Errorf("pane not found: %s", paneID)
	}

	pane.BindAgent(agentName, agentID)

	m.emitEvent(events.Event{
		Type:      events.EventPaneBound,
		AgentName: agentName,
		Data: map[string]any{
			"pane_id":  paneID,
			"agent_id": agentID,
		},
	})

	return nil
}

// emitEvent publishes an event to the event bus
func (m *Manager) emitEvent(event events.Event) {
	if m.bus == nil {
		return
	}
	event.Timestamp = time.Now()
	m.bus.Publish(event)
}

// LayoutSnapshot represents a serializable layout state
type LayoutSnapshot struct {
	Layout        *LayoutTree             `json:"layout"`
	Panes         map[string]PaneSnapshot `json:"panes"`
	FocusedPaneID string                  `json:"focused_pane_id"`
}

// Snapshot creates a serializable snapshot of the layout state
func (m *Manager) Snapshot() LayoutSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	paneSnaps := make(map[string]PaneSnapshot)
	for id, pane := range m.panes {
		paneSnaps[id] = pane.Snapshot()
	}

	return LayoutSnapshot{
		Layout:        m.layout,
		Panes:         paneSnaps,
		FocusedPaneID: m.focusedPaneID,
	}
}

// Restore restores the layout from a snapshot
func (m *Manager) Restore(snap LayoutSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.layout = snap.Layout
	if m.layout != nil {
		m.layout.RebuildParentRefs()
	}

	m.panes = make(map[string]*Pane)
	for id, paneSnap := range snap.Panes {
		pane := NewPane(id, paneSnap.Type)
		pane.RestoreFromSnapshot(paneSnap)
		m.panes[id] = pane
	}

	m.focusedPaneID = snap.FocusedPaneID
	if pane, ok := m.panes[m.focusedPaneID]; ok {
		pane.SetFocused(true)
	}

	// Recalculate layout
	if m.layout != nil {
		m.layout.CalculateLayout()
		m.updatePaneRects()
	}

	m.emitEvent(events.Event{
		Type: events.EventLayoutRestored,
		Data: map[string]any{
			"num_panes":       len(m.panes),
			"focused_pane_id": m.focusedPaneID,
		},
	})

	return nil
}
