package pane

import (
	"encoding/json"
	"testing"
)

func TestNewLayoutTree(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")

	if tree.Root == nil {
		t.Fatal("expected root node to be created")
	}

	if !tree.Root.IsLeaf() {
		t.Error("expected root to be a leaf node")
	}

	if tree.Root.PaneID != "pane-1" {
		t.Errorf("expected pane ID 'pane-1', got '%s'", tree.Root.PaneID)
	}
}

func TestLayoutTreeSplit(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")
	tree.SetDimensions(100, 50)

	// Split horizontally
	err := tree.SplitPane("pane-1", SplitHorizontal, "pane-2", 0.5)
	if err != nil {
		t.Fatalf("failed to split: %v", err)
	}

	// Root should now be a split node
	if tree.Root.IsLeaf() {
		t.Error("expected root to be a split node after split")
	}

	if tree.Root.Direction != SplitHorizontal {
		t.Errorf("expected horizontal split, got %v", tree.Root.Direction)
	}

	// Should have 2 children
	if len(tree.Root.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(tree.Root.Children))
	}

	// Both children should be leaves
	for i, child := range tree.Root.Children {
		if !child.IsLeaf() {
			t.Errorf("child %d should be a leaf", i)
		}
	}

	// First child should have pane-1, second should have pane-2
	if tree.Root.Children[0].PaneID != "pane-1" {
		t.Errorf("expected first child to be pane-1, got %s", tree.Root.Children[0].PaneID)
	}
	if tree.Root.Children[1].PaneID != "pane-2" {
		t.Errorf("expected second child to be pane-2, got %s", tree.Root.Children[1].PaneID)
	}
}

func TestLayoutTreeCalculateLayout(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")
	tree.SetDimensions(100, 50)

	// Split horizontally with 50/50
	tree.SplitPane("pane-1", SplitHorizontal, "pane-2", 0.5)

	rects := tree.CalculateLayout()

	// Check pane-1 rect
	rect1, ok := rects["pane-1"]
	if !ok {
		t.Fatal("pane-1 rect not found")
	}
	if rect1.X != 0 || rect1.Y != 0 {
		t.Errorf("pane-1 should start at (0,0), got (%d,%d)", rect1.X, rect1.Y)
	}
	if rect1.Width != 50 {
		t.Errorf("pane-1 width should be 50, got %d", rect1.Width)
	}
	if rect1.Height != 50 {
		t.Errorf("pane-1 height should be 50, got %d", rect1.Height)
	}

	// Check pane-2 rect
	rect2, ok := rects["pane-2"]
	if !ok {
		t.Fatal("pane-2 rect not found")
	}
	if rect2.X != 50 || rect2.Y != 0 {
		t.Errorf("pane-2 should start at (50,0), got (%d,%d)", rect2.X, rect2.Y)
	}
	if rect2.Width != 50 {
		t.Errorf("pane-2 width should be 50, got %d", rect2.Width)
	}
}

func TestLayoutTreeVerticalSplit(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")
	tree.SetDimensions(100, 100)

	// Split vertically with 50/50
	tree.SplitPane("pane-1", SplitVertical, "pane-2", 0.5)

	rects := tree.CalculateLayout()

	// Check pane-1 rect (top half)
	rect1, ok := rects["pane-1"]
	if !ok {
		t.Fatal("pane-1 rect not found")
	}
	if rect1.Height != 50 {
		t.Errorf("pane-1 height should be 50, got %d", rect1.Height)
	}

	// Check pane-2 rect (bottom half)
	rect2, ok := rects["pane-2"]
	if !ok {
		t.Fatal("pane-2 rect not found")
	}
	if rect2.Y != 50 {
		t.Errorf("pane-2 Y should be 50, got %d", rect2.Y)
	}
	if rect2.Height != 50 {
		t.Errorf("pane-2 height should be 50, got %d", rect2.Height)
	}
}

func TestLayoutTreeNestedSplit(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")
	tree.SetDimensions(100, 100)

	// Split horizontally first
	tree.SplitPane("pane-1", SplitHorizontal, "pane-2", 0.5)

	// Split pane-2 vertically
	tree.SplitPane("pane-2", SplitVertical, "pane-3", 0.5)

	// Should have 3 panes now
	count := tree.CountPanes()
	if count != 3 {
		t.Errorf("expected 3 panes, got %d", count)
	}

	rects := tree.CalculateLayout()

	// pane-1 should be on the left half
	rect1 := rects["pane-1"]
	if rect1.Width != 50 {
		t.Errorf("pane-1 width should be 50, got %d", rect1.Width)
	}

	// pane-2 should be top-right quadrant
	rect2 := rects["pane-2"]
	if rect2.X != 50 {
		t.Errorf("pane-2 X should be 50, got %d", rect2.X)
	}
	if rect2.Height != 50 {
		t.Errorf("pane-2 height should be 50, got %d", rect2.Height)
	}

	// pane-3 should be bottom-right quadrant
	rect3 := rects["pane-3"]
	if rect3.X != 50 || rect3.Y != 50 {
		t.Errorf("pane-3 should be at (50,50), got (%d,%d)", rect3.X, rect3.Y)
	}
}

func TestLayoutTreeRemovePane(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")
	tree.SetDimensions(100, 50)

	// Split to create 2 panes
	tree.SplitPane("pane-1", SplitHorizontal, "pane-2", 0.5)

	// Remove pane-2
	err := tree.RemovePane("pane-2")
	if err != nil {
		t.Fatalf("failed to remove pane: %v", err)
	}

	// Should have 1 pane now
	count := tree.CountPanes()
	if count != 1 {
		t.Errorf("expected 1 pane after removal, got %d", count)
	}

	// pane-1 should take full space
	rects := tree.CalculateLayout()
	rect1 := rects["pane-1"]
	if rect1.Width != 100 {
		t.Errorf("pane-1 should take full width after removal, got %d", rect1.Width)
	}
}

func TestLayoutTreeCannotRemoveLastPane(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")

	err := tree.RemovePane("pane-1")
	if err == nil {
		t.Error("expected error when removing last pane")
	}
}

func TestLayoutTreeFindNode(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")
	tree.SplitPane("pane-1", SplitHorizontal, "pane-2", 0.5)

	// Find by pane ID
	node := tree.FindNodeByPaneID("pane-2")
	if node == nil {
		t.Fatal("expected to find node for pane-2")
	}
	if node.PaneID != "pane-2" {
		t.Errorf("expected pane-2, got %s", node.PaneID)
	}
}

func TestLayoutTreeGetAllPaneIDs(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")
	tree.SplitPane("pane-1", SplitHorizontal, "pane-2", 0.5)
	tree.SplitPane("pane-2", SplitVertical, "pane-3", 0.5)

	paneIDs := tree.GetAllPaneIDs()

	if len(paneIDs) != 3 {
		t.Errorf("expected 3 pane IDs, got %d", len(paneIDs))
	}

	// Check all IDs are present
	found := make(map[string]bool)
	for _, id := range paneIDs {
		found[id] = true
	}

	for _, expected := range []string{"pane-1", "pane-2", "pane-3"} {
		if !found[expected] {
			t.Errorf("expected to find %s in pane IDs", expected)
		}
	}
}

func TestLayoutTreeSerialization(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")
	tree.SetDimensions(100, 50)
	tree.SplitPane("pane-1", SplitHorizontal, "pane-2", 0.5)

	// Serialize
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Deserialize
	var restored LayoutTree
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Rebuild parent refs
	restored.RebuildParentRefs()

	// Verify structure
	if restored.Root == nil {
		t.Fatal("restored tree has nil root")
	}

	if restored.Root.Direction != SplitHorizontal {
		t.Errorf("expected horizontal split, got %v", restored.Root.Direction)
	}

	if len(restored.Root.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(restored.Root.Children))
	}
}

func TestSplitDirection(t *testing.T) {
	tests := []struct {
		dir      SplitDirection
		expected string
	}{
		{SplitNone, "none"},
		{SplitHorizontal, "horizontal"},
		{SplitVertical, "vertical"},
	}

	for _, tt := range tests {
		if tt.dir.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.dir.String())
		}
	}
}

func TestSplitDirectionMarshal(t *testing.T) {
	tests := []struct {
		dir      SplitDirection
		expected string
	}{
		{SplitNone, `"none"`},
		{SplitHorizontal, `"horizontal"`},
		{SplitVertical, `"vertical"`},
	}

	for _, tt := range tests {
		data, err := json.Marshal(tt.dir)
		if err != nil {
			t.Errorf("failed to marshal %v: %v", tt.dir, err)
			continue
		}
		if string(data) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(data))
		}
	}
}

func TestSplitDirectionUnmarshal(t *testing.T) {
	tests := []struct {
		input    string
		expected SplitDirection
	}{
		{`"none"`, SplitNone},
		{`"horizontal"`, SplitHorizontal},
		{`"vertical"`, SplitVertical},
	}

	for _, tt := range tests {
		var dir SplitDirection
		err := json.Unmarshal([]byte(tt.input), &dir)
		if err != nil {
			t.Errorf("failed to unmarshal %s: %v", tt.input, err)
			continue
		}
		if dir != tt.expected {
			t.Errorf("expected %v, got %v", tt.expected, dir)
		}
	}
}

func TestLayoutTreeResize(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")
	tree.SetDimensions(100, 100)
	tree.SplitPane("pane-1", SplitHorizontal, "pane-2", 0.5)

	// Resize to 70/30 split
	err := tree.Resize(tree.Root.ID, []float64{0.7})
	if err != nil {
		t.Fatalf("failed to resize: %v", err)
	}

	rects := tree.CalculateLayout()

	// pane-1 should now be 70% width
	rect1 := rects["pane-1"]
	if rect1.Width != 70 {
		t.Errorf("pane-1 width should be 70, got %d", rect1.Width)
	}

	// pane-2 should be 30% width
	rect2 := rects["pane-2"]
	if rect2.Width != 30 {
		t.Errorf("pane-2 width should be 30, got %d", rect2.Width)
	}
}

func TestLayoutTreeCustomRatio(t *testing.T) {
	ResetNodeIDCounter()

	tree := NewLayoutTree("pane-1")
	tree.SetDimensions(100, 100)

	// Split with 30/70 ratio
	tree.SplitPane("pane-1", SplitHorizontal, "pane-2", 0.3)

	rects := tree.CalculateLayout()

	rect1 := rects["pane-1"]
	if rect1.Width != 30 {
		t.Errorf("pane-1 width should be 30, got %d", rect1.Width)
	}

	rect2 := rects["pane-2"]
	if rect2.Width != 70 {
		t.Errorf("pane-2 width should be 70, got %d", rect2.Width)
	}
}
