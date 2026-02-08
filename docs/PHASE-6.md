PHASE 6 — Pane Manager (tmux-like Layout Engine)

You are implementing a tmux-like pane manager for the TUI.

Goals
- Allow multiple panes to exist simultaneously
- Support horizontal and vertical splits
- Bind panes to agents and streams
- Persist and replay layout state

Requirements
- Implement a layout tree model (split nodes + leaf panes)
- Support pane creation, split, focus, resize, close
- Emit events for all layout changes
- Persist pane layout as part of the session
- Restore layout during replay

Constraints
- Panes must never own or control agents
- Panes render events only
- UI must not mutate layout state directly
- Layout changes must be event-driven

Exit Criteria
- Multiple panes visible simultaneously
- Each pane can display a different agent stream
- Layout can be saved and restored
- Layout changes replay deterministically