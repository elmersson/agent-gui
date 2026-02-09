package pane

import (
	"encoding/json"
	"fmt"
)

// SplitDirection indicates how a container is split
type SplitDirection int

const (
	SplitNone       SplitDirection = iota // Leaf node (contains a pane)
	SplitHorizontal                       // Children arranged left-to-right
	SplitVertical                         // Children arranged top-to-bottom
)

// String returns the string representation of SplitDirection
func (s SplitDirection) String() string {
	switch s {
	case SplitNone:
		return "none"
	case SplitHorizontal:
		return "horizontal"
	case SplitVertical:
		return "vertical"
	default:
		return "unknown"
	}
}

// MarshalJSON implements json.Marshaler
func (s SplitDirection) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON implements json.Unmarshaler
func (s *SplitDirection) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "none":
		*s = SplitNone
	case "horizontal":
		*s = SplitHorizontal
	case "vertical":
		*s = SplitVertical
	default:
		return fmt.Errorf("unknown split direction: %s", str)
	}
	return nil
}

// LayoutNode represents a node in the layout tree.
// It can be either a split container (with children) or a leaf (with a pane).
type LayoutNode struct {
	ID        string         `json:"id"`
	Direction SplitDirection `json:"direction"`

	// For split nodes - the ratio of space each child takes (0.0 to 1.0)
	// Only the first N-1 ratios are stored; the last child gets remaining space
	Ratios []float64 `json:"ratios,omitempty"`

	// For split nodes
	Children []*LayoutNode `json:"children,omitempty"`

	// For leaf nodes - the pane ID
	PaneID string `json:"pane_id,omitempty"`

	// Parent reference (not serialized)
	parent *LayoutNode
}

// Rect represents a rectangle for layout calculations
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// LayoutTree represents the entire pane layout
type LayoutTree struct {
	Root *LayoutNode `json:"root"`

	// Dimensions
	Width  int `json:"width"`
	Height int `json:"height"`

	// Cached layout calculations
	paneRects map[string]Rect
}

// NewLayoutTree creates a new layout tree with a single pane
func NewLayoutTree(rootPaneID string) *LayoutTree {
	return &LayoutTree{
		Root: &LayoutNode{
			ID:        generateNodeID(),
			Direction: SplitNone,
			PaneID:    rootPaneID,
		},
		paneRects: make(map[string]Rect),
	}
}

// IsLeaf returns true if this node is a leaf (contains a pane)
func (n *LayoutNode) IsLeaf() bool {
	return n.Direction == SplitNone
}

// IsSplit returns true if this node is a split container
func (n *LayoutNode) IsSplit() bool {
	return n.Direction != SplitNone
}

// FindNode finds a node by its ID
func (t *LayoutTree) FindNode(id string) *LayoutNode {
	return findNodeRecursive(t.Root, id)
}

func findNodeRecursive(node *LayoutNode, id string) *LayoutNode {
	if node == nil {
		return nil
	}
	if node.ID == id {
		return node
	}
	for _, child := range node.Children {
		if found := findNodeRecursive(child, id); found != nil {
			return found
		}
	}
	return nil
}

// FindNodeByPaneID finds the leaf node containing a specific pane
func (t *LayoutTree) FindNodeByPaneID(paneID string) *LayoutNode {
	return findNodeByPaneIDRecursive(t.Root, paneID)
}

func findNodeByPaneIDRecursive(node *LayoutNode, paneID string) *LayoutNode {
	if node == nil {
		return nil
	}
	if node.IsLeaf() && node.PaneID == paneID {
		return node
	}
	for _, child := range node.Children {
		if found := findNodeByPaneIDRecursive(child, paneID); found != nil {
			return found
		}
	}
	return nil
}

// Split splits a leaf node into two panes
func (t *LayoutTree) Split(nodeID string, direction SplitDirection, newPaneID string, ratio float64) error {
	node := t.FindNode(nodeID)
	if node == nil {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	if !node.IsLeaf() {
		return fmt.Errorf("can only split leaf nodes")
	}

	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}

	// Convert leaf to split container
	existingPaneID := node.PaneID

	// Create child nodes
	leftChild := &LayoutNode{
		ID:        generateNodeID(),
		Direction: SplitNone,
		PaneID:    existingPaneID,
		parent:    node,
	}

	rightChild := &LayoutNode{
		ID:        generateNodeID(),
		Direction: SplitNone,
		PaneID:    newPaneID,
		parent:    node,
	}

	// Transform current node into a split container
	node.Direction = direction
	node.PaneID = ""
	node.Children = []*LayoutNode{leftChild, rightChild}
	node.Ratios = []float64{ratio}

	return nil
}

// SplitPane is a convenience method to split by pane ID
func (t *LayoutTree) SplitPane(paneID string, direction SplitDirection, newPaneID string, ratio float64) error {
	node := t.FindNodeByPaneID(paneID)
	if node == nil {
		return fmt.Errorf("pane not found: %s", paneID)
	}
	return t.Split(node.ID, direction, newPaneID, ratio)
}

// RemovePane removes a pane from the layout
func (t *LayoutTree) RemovePane(paneID string) error {
	node := t.FindNodeByPaneID(paneID)
	if node == nil {
		return fmt.Errorf("pane not found: %s", paneID)
	}

	// If this is the root node, can't remove it
	if node == t.Root {
		return fmt.Errorf("cannot remove the last pane")
	}

	parent := node.parent
	if parent == nil {
		return fmt.Errorf("parent not found")
	}

	// Find sibling
	var sibling *LayoutNode
	for _, child := range parent.Children {
		if child != node {
			sibling = child
			break
		}
	}

	if sibling == nil {
		return fmt.Errorf("sibling not found")
	}

	// Replace parent with sibling (collapse the tree)
	if parent == t.Root {
		// Special case: sibling becomes new root
		sibling.parent = nil
		t.Root = sibling
	} else {
		// Replace parent in grandparent's children
		grandparent := parent.parent
		for i, child := range grandparent.Children {
			if child == parent {
				sibling.parent = grandparent
				grandparent.Children[i] = sibling
				break
			}
		}
	}

	return nil
}

// Resize adjusts the ratio between children at a split node
func (t *LayoutTree) Resize(nodeID string, ratios []float64) error {
	node := t.FindNode(nodeID)
	if node == nil {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	if node.IsLeaf() {
		return fmt.Errorf("cannot resize a leaf node")
	}

	if len(ratios) != len(node.Children)-1 {
		return fmt.Errorf("expected %d ratios, got %d", len(node.Children)-1, len(ratios))
	}

	// Validate ratios
	sum := 0.0
	for _, r := range ratios {
		if r <= 0 || r >= 1 {
			return fmt.Errorf("invalid ratio: %f", r)
		}
		sum += r
	}
	if sum >= 1 {
		return fmt.Errorf("ratios sum to >= 1")
	}

	node.Ratios = ratios
	return nil
}

// CalculateLayout computes the rectangle for each pane
func (t *LayoutTree) CalculateLayout() map[string]Rect {
	t.paneRects = make(map[string]Rect)

	if t.Root == nil || t.Width == 0 || t.Height == 0 {
		return t.paneRects
	}

	t.calculateNodeLayout(t.Root, Rect{
		X:      0,
		Y:      0,
		Width:  t.Width,
		Height: t.Height,
	})

	return t.paneRects
}

func (t *LayoutTree) calculateNodeLayout(node *LayoutNode, rect Rect) {
	if node.IsLeaf() {
		t.paneRects[node.PaneID] = rect
		return
	}

	// Calculate children rectangles
	numChildren := len(node.Children)
	if numChildren == 0 {
		return
	}

	// Build ratios list (add implicit last ratio)
	ratios := make([]float64, numChildren)
	sum := 0.0
	for i, r := range node.Ratios {
		ratios[i] = r
		sum += r
	}
	ratios[numChildren-1] = 1.0 - sum

	// Calculate child rects
	if node.Direction == SplitHorizontal {
		x := rect.X
		for i, child := range node.Children {
			childWidth := int(float64(rect.Width) * ratios[i])
			// Last child gets remaining width to avoid rounding errors
			if i == numChildren-1 {
				childWidth = rect.Width - (x - rect.X)
			}
			childRect := Rect{
				X:      x,
				Y:      rect.Y,
				Width:  childWidth,
				Height: rect.Height,
			}
			t.calculateNodeLayout(child, childRect)
			x += childWidth
		}
	} else { // SplitVertical
		y := rect.Y
		for i, child := range node.Children {
			childHeight := int(float64(rect.Height) * ratios[i])
			// Last child gets remaining height to avoid rounding errors
			if i == numChildren-1 {
				childHeight = rect.Height - (y - rect.Y)
			}
			childRect := Rect{
				X:      rect.X,
				Y:      y,
				Width:  rect.Width,
				Height: childHeight,
			}
			t.calculateNodeLayout(child, childRect)
			y += childHeight
		}
	}
}

// SetDimensions sets the layout dimensions and recalculates
func (t *LayoutTree) SetDimensions(width, height int) {
	t.Width = width
	t.Height = height
	t.CalculateLayout()
}

// GetPaneRect returns the rectangle for a specific pane
func (t *LayoutTree) GetPaneRect(paneID string) (Rect, bool) {
	rect, ok := t.paneRects[paneID]
	return rect, ok
}

// GetAllPaneIDs returns all pane IDs in the layout
func (t *LayoutTree) GetAllPaneIDs() []string {
	var paneIDs []string
	collectPaneIDs(t.Root, &paneIDs)
	return paneIDs
}

func collectPaneIDs(node *LayoutNode, ids *[]string) {
	if node == nil {
		return
	}
	if node.IsLeaf() {
		*ids = append(*ids, node.PaneID)
		return
	}
	for _, child := range node.Children {
		collectPaneIDs(child, ids)
	}
}

// CountPanes returns the number of panes in the layout
func (t *LayoutTree) CountPanes() int {
	return countPanesRecursive(t.Root)
}

func countPanesRecursive(node *LayoutNode) int {
	if node == nil {
		return 0
	}
	if node.IsLeaf() {
		return 1
	}
	count := 0
	for _, child := range node.Children {
		count += countPanesRecursive(child)
	}
	return count
}

// RebuildParentRefs rebuilds parent references after deserialization
func (t *LayoutTree) RebuildParentRefs() {
	rebuildParentRefs(t.Root, nil)
}

func rebuildParentRefs(node, parent *LayoutNode) {
	if node == nil {
		return
	}
	node.parent = parent
	for _, child := range node.Children {
		rebuildParentRefs(child, node)
	}
}

// Node ID generation
var nodeIDCounter int

func generateNodeID() string {
	nodeIDCounter++
	return fmt.Sprintf("node-%d", nodeIDCounter)
}

// ResetNodeIDCounter resets the node ID counter (mainly for testing)
func ResetNodeIDCounter() {
	nodeIDCounter = 0
}
