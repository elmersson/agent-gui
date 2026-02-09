package pane

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LayoutPersistence handles saving and loading layout state
type LayoutPersistence struct {
	layoutDir string
}

// NewLayoutPersistence creates a new layout persistence handler
func NewLayoutPersistence(layoutDir string) (*LayoutPersistence, error) {
	if err := os.MkdirAll(layoutDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create layout directory: %w", err)
	}

	return &LayoutPersistence{
		layoutDir: layoutDir,
	}, nil
}

// PersistedLayout represents a layout that can be saved to disk
type PersistedLayout struct {
	Version       int             `json:"version"`
	Timestamp     time.Time       `json:"timestamp"`
	Name          string          `json:"name"`
	Layout        *LayoutTree     `json:"layout"`
	Panes         []PersistedPane `json:"panes"`
	FocusedPaneID string          `json:"focused_pane_id"`
}

// PersistedPane represents a pane in persisted form
type PersistedPane struct {
	ID        string   `json:"id"`
	Type      PaneType `json:"type"`
	AgentName string   `json:"agent_name,omitempty"`
	AgentID   string   `json:"agent_id,omitempty"`
	Title     string   `json:"title,omitempty"`
}

// Save saves a layout snapshot to disk
func (p *LayoutPersistence) Save(name string, snapshot LayoutSnapshot) error {
	persisted := PersistedLayout{
		Version:       1,
		Timestamp:     time.Now(),
		Name:          name,
		Layout:        snapshot.Layout,
		FocusedPaneID: snapshot.FocusedPaneID,
		Panes:         make([]PersistedPane, 0, len(snapshot.Panes)),
	}

	for id, paneSnap := range snapshot.Panes {
		persisted.Panes = append(persisted.Panes, PersistedPane{
			ID:        id,
			Type:      paneSnap.Type,
			AgentName: paneSnap.AgentName,
			AgentID:   paneSnap.AgentID,
			Title:     paneSnap.Title,
		})
	}

	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal layout: %w", err)
	}

	filePath := p.getLayoutPath(name)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write layout file: %w", err)
	}

	return nil
}

// Load loads a layout from disk
func (p *LayoutPersistence) Load(name string) (*PersistedLayout, error) {
	filePath := p.getLayoutPath(name)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("layout not found: %s", name)
		}
		return nil, fmt.Errorf("failed to read layout file: %w", err)
	}

	var persisted PersistedLayout
	if err := json.Unmarshal(data, &persisted); err != nil {
		return nil, fmt.Errorf("failed to unmarshal layout: %w", err)
	}

	// Rebuild parent references
	if persisted.Layout != nil {
		persisted.Layout.RebuildParentRefs()
	}

	return &persisted, nil
}

// Delete removes a layout from disk
func (p *LayoutPersistence) Delete(name string) error {
	filePath := p.getLayoutPath(name)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete layout: %w", err)
	}
	return nil
}

// List returns all saved layout names
func (p *LayoutPersistence) List() ([]string, error) {
	entries, err := os.ReadDir(p.layoutDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read layout directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > 5 && name[len(name)-5:] == ".json" {
			names = append(names, name[:len(name)-5])
		}
	}

	return names, nil
}

// Exists checks if a layout exists
func (p *LayoutPersistence) Exists(name string) bool {
	filePath := p.getLayoutPath(name)
	_, err := os.Stat(filePath)
	return err == nil
}

// getLayoutPath returns the file path for a layout
func (p *LayoutPersistence) getLayoutPath(name string) string {
	return filepath.Join(p.layoutDir, name+".json")
}

// SaveDefault saves the layout as the default layout
func (p *LayoutPersistence) SaveDefault(snapshot LayoutSnapshot) error {
	return p.Save("default", snapshot)
}

// LoadDefault loads the default layout
func (p *LayoutPersistence) LoadDefault() (*PersistedLayout, error) {
	return p.Load("default")
}

// HasDefault checks if a default layout exists
func (p *LayoutPersistence) HasDefault() bool {
	return p.Exists("default")
}

// RestoreToManager restores a persisted layout to a manager
func (p *LayoutPersistence) RestoreToManager(persisted *PersistedLayout, manager *Manager) error {
	// Convert persisted panes to pane snapshots
	paneSnaps := make(map[string]PaneSnapshot)
	for _, pp := range persisted.Panes {
		paneSnaps[pp.ID] = PaneSnapshot{
			ID:        pp.ID,
			Type:      pp.Type,
			AgentName: pp.AgentName,
			AgentID:   pp.AgentID,
			Title:     pp.Title,
			State:     "idle",
		}
	}

	snapshot := LayoutSnapshot{
		Layout:        persisted.Layout,
		Panes:         paneSnaps,
		FocusedPaneID: persisted.FocusedPaneID,
	}

	return manager.Restore(snapshot)
}

// SessionLayout represents layout state for a session (used in session persistence)
type SessionLayout struct {
	Layout        *LayoutTree     `json:"layout"`
	Panes         []PersistedPane `json:"panes"`
	FocusedPaneID string          `json:"focused_pane_id"`
}

// ToSessionLayout converts a LayoutSnapshot to a SessionLayout
func ToSessionLayout(snapshot LayoutSnapshot) SessionLayout {
	panes := make([]PersistedPane, 0, len(snapshot.Panes))
	for id, paneSnap := range snapshot.Panes {
		panes = append(panes, PersistedPane{
			ID:        id,
			Type:      paneSnap.Type,
			AgentName: paneSnap.AgentName,
			AgentID:   paneSnap.AgentID,
			Title:     paneSnap.Title,
		})
	}

	return SessionLayout{
		Layout:        snapshot.Layout,
		Panes:         panes,
		FocusedPaneID: snapshot.FocusedPaneID,
	}
}

// FromSessionLayout converts a SessionLayout to a LayoutSnapshot
func FromSessionLayout(sl SessionLayout) LayoutSnapshot {
	paneSnaps := make(map[string]PaneSnapshot)
	for _, pp := range sl.Panes {
		paneSnaps[pp.ID] = PaneSnapshot{
			ID:        pp.ID,
			Type:      pp.Type,
			AgentName: pp.AgentName,
			AgentID:   pp.AgentID,
			Title:     pp.Title,
			State:     "idle",
		}
	}

	return LayoutSnapshot{
		Layout:        sl.Layout,
		Panes:         paneSnaps,
		FocusedPaneID: sl.FocusedPaneID,
	}
}
