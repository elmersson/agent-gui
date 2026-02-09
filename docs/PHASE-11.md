PHASE 11 — Per-Pane Agent Instances and Conversation Isolation

You are implementing independent agent instances per pane, enabling truly isolated conversations in each pane.

## Background

Currently, all panes share a single agent adapter instance, which means:
- Conversation history is shared across all panes
- When you split a pane and chat in both, context bleeds between them
- There's no way to have independent parallel conversations

This phase refactors the architecture to give each pane its own agent instance with isolated conversation history.

## Goals

- Each pane maintains its own independent conversation history
- Splitting a pane creates a fresh agent instance for the new pane
- Panes can run different models independently
- Token usage is tracked per-pane
- Conversation state persists with pane lifecycle

## Architecture Changes

### Current Architecture

```
┌─────────────────────────────────────────────────────────┐
│                        TUI Model                         │
│                            │                             │
│                    ┌───────▼───────┐                     │
│                    │    Manager    │                     │
│                    │  (singleton)  │                     │
│                    └───────┬───────┘                     │
│                            │                             │
│                    ┌───────▼───────┐                     │
│                    │    Adapter    │                     │
│                    │  (singleton)  │                     │
│                    │   + history   │                     │
│                    └───────────────┘                     │
│                            │                             │
│         ┌──────────────────┼──────────────────┐          │
│         ▼                  ▼                  ▼          │
│    ┌─────────┐       ┌─────────┐        ┌─────────┐      │
│    │  Pane 1 │       │  Pane 2 │        │  Pane 3 │      │
│    │ (view)  │       │ (view)  │        │ (view)  │      │
│    └─────────┘       └─────────┘        └─────────┘      │
│         │                  │                  │          │
│         └──────────────────┴──────────────────┘          │
│                   Shared output stream                   │
└─────────────────────────────────────────────────────────┘
```

### Target Architecture

```
┌─────────────────────────────────────────────────────────┐
│                        TUI Model                         │
│                            │                             │
│                    ┌───────▼───────┐                     │
│                    │ AgentRegistry │                     │
│                    │ (pane→agent)  │                     │
│                    └───────┬───────┘                     │
│                            │                             │
│         ┌──────────────────┼──────────────────┐          │
│         ▼                  ▼                  ▼          │
│    ┌─────────┐       ┌─────────┐        ┌─────────┐      │
│    │ Agent 1 │       │ Agent 2 │        │ Agent 3 │      │
│    │+history │       │+history │        │+history │      │
│    │+adapter │       │+adapter │        │+adapter │      │
│    └────┬────┘       └────┬────┘        └────┬────┘      │
│         │                  │                  │          │
│         ▼                  ▼                  ▼          │
│    ┌─────────┐       ┌─────────┐        ┌─────────┐      │
│    │  Pane 1 │       │  Pane 2 │        │  Pane 3 │      │
│    │ (view)  │       │ (view)  │        │ (view)  │      │
│    └─────────┘       └─────────┘        └─────────┘      │
│         │                  │                  │          │
│    Independent        Independent        Independent     │
│    conversation       conversation       conversation    │
└─────────────────────────────────────────────────────────┘
```

## Implementation Plan

### Step 1: Create AgentInstance Type

Create a new type that encapsulates a complete agent instance with its own state:

```go
// pkg/agent/instance.go

type Instance struct {
    ID        string
    PaneID    string
    Adapter   Agent
    History   []Message
    Model     string
    State     string // idle, running, paused
    TokenUsage TokenUsage
    
    ctx       context.Context
    cancel    context.CancelFunc
    mu        sync.RWMutex
}

func NewInstance(paneID string, config Config) *Instance
func (i *Instance) Execute(ctx context.Context, input string) (<-chan string, <-chan error)
func (i *Instance) Stop()
func (i *Instance) Pause()
func (i *Instance) Resume()
func (i *Instance) ClearHistory()
func (i *Instance) GetHistory() []Message
func (i *Instance) SetModel(model string)
```

### Step 2: Create AgentRegistry

Create a registry that manages agent instances per pane:

```go
// pkg/agent/registry.go

type Registry struct {
    instances map[string]*Instance  // paneID -> instance
    bus       events.Bus
    mu        sync.RWMutex
}

func NewRegistry(bus events.Bus) *Registry
func (r *Registry) CreateInstance(paneID string, config Config) *Instance
func (r *Registry) GetInstance(paneID string) *Instance
func (r *Registry) RemoveInstance(paneID string)
func (r *Registry) GetOrCreate(paneID string, config Config) *Instance
```

### Step 3: Update Pane Manager Integration

Modify the pane manager to work with the agent registry:

```go
// pkg/pane/manager.go

type Manager struct {
    // ... existing fields
    agentRegistry *agent.Registry
}

func (m *Manager) SplitPane(paneID string, direction SplitDirection, newAgentName string) (*Pane, error) {
    // ... existing logic
    
    // Create new agent instance for the new pane
    m.agentRegistry.CreateInstance(newPaneID, agent.Config{
        Model: defaultModel,
    })
    
    return newPane, nil
}

func (m *Manager) ClosePane(paneID string) error {
    // ... existing logic
    
    // Clean up agent instance
    m.agentRegistry.RemoveInstance(paneID)
    
    return nil
}
```

### Step 4: Update TUI Message Routing

Modify the TUI to route messages to the correct agent instance:

```go
// pkg/tui/model.go

func (m Model) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    // ...
    case tea.KeyEnter:
        value := m.input.Value()
        if value != "" {
            // Get agent instance for focused pane
            focusedPaneID := m.paneManager.GetFocusedPaneID()
            agentInstance := m.agentRegistry.GetInstance(focusedPaneID)
            
            if agentInstance != nil {
                // Route input to pane's agent
                agentInstance.Execute(context.Background(), value)
            }
        }
    // ...
}
```

### Step 5: Update Event Handling

Modify event handling to include pane context:

```go
// pkg/events/bus.go

type Event struct {
    Type      EventType
    AgentName string
    PaneID    string  // NEW: Associate events with panes
    Data      any
    Timestamp time.Time
}
```

Update the TUI to filter events by pane:

```go
case OutputMsg:
    // Route output to the specific pane
    if msg.PaneID != "" {
        if pane := m.paneManager.GetPane(msg.PaneID); pane != nil {
            pane.AppendContent(msg.Content)
        }
    }
```

### Step 6: Per-Pane Model Selection

Allow each pane to use a different model:

```go
// New command: :model-pane <model>
case "model-pane":
    if len(args) > 0 {
        focusedPaneID := m.paneManager.GetFocusedPaneID()
        if instance := m.agentRegistry.GetInstance(focusedPaneID); instance != nil {
            instance.SetModel(args[0])
        }
    }
```

### Step 7: Conversation History UI

Add commands to view and manage conversation history per pane:

```go
// New commands
case "history":
    // Show conversation history for current pane
    focusedPaneID := m.paneManager.GetFocusedPaneID()
    if instance := m.agentRegistry.GetInstance(focusedPaneID); instance != nil {
        history := instance.GetHistory()
        // Display formatted history
    }

case "clear-history":
    // Clear conversation history for current pane
    focusedPaneID := m.paneManager.GetFocusedPaneID()
    if instance := m.agentRegistry.GetInstance(focusedPaneID); instance != nil {
        instance.ClearHistory()
    }
```

## New Files

```
pkg/
  agent/
    instance.go      # Agent instance with isolated state
    registry.go      # Registry managing instances per pane
```

## Modified Files

```
pkg/
  adapter/
    claude.go        # Remove global history (moved to Instance)
  agent/
    agent.go         # Add Instance interface
  events/
    bus.go           # Add PaneID to Event
  manager/
    manager.go       # Integrate with AgentRegistry
  pane/
    manager.go       # Create/destroy agent instances on pane lifecycle
    pane.go          # Add AgentInstanceID field
  tui/
    model.go         # Route messages to pane-specific agents
```

## Requirements

- Each pane MUST have its own agent instance
- Agent instances MUST be created when panes are created
- Agent instances MUST be destroyed when panes are closed
- Conversation history MUST NOT leak between panes
- Token usage MUST be tracked per-pane
- Events MUST include pane context for proper routing
- Model selection MUST work per-pane

## Constraints

- Maintain backward compatibility for single-pane mode
- Do not break existing session/replay functionality
- Ensure thread safety for concurrent pane operations
- Keep memory usage reasonable (lazy instantiation)

## Testing Plan

1. **Unit Tests**
   - AgentInstance creation and lifecycle
   - Registry add/remove/get operations
   - History isolation between instances

2. **Integration Tests**
   - Split pane creates new agent instance
   - Close pane destroys agent instance
   - Messages route to correct pane
   - Token usage tracked separately

3. **Manual Tests**
   - Split pane horizontally, verify independent conversations
   - Split pane vertically, verify independent conversations
   - Change model in one pane, verify other pane unchanged
   - Close pane, verify no memory leaks
   - Verify history command shows pane-specific history

## Exit Criteria

- [ ] Split a pane and have independent conversations in each
- [ ] Conversation history is isolated per pane
- [ ] Each pane can use a different model
- [ ] Token usage is tracked per pane
- [ ] Closing a pane cleans up its agent instance
- [ ] Events are properly routed to the correct pane
- [ ] No regression in single-pane mode
- [ ] Session replay still works correctly

## Future Considerations

- Per-pane system prompts
- Pane-to-pane context sharing (explicit)
- Agent instance pooling for performance
- Conversation forking (split pane with history copy)
- Cross-pane agent collaboration
