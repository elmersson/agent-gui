PHASE 8 — Multi-Stream Routing

You are implementing stream routing between agents and panes.

Goals
- Route multiple streaming agents to different panes
- Support simultaneous streaming
- Preserve stream ordering
- Enable replay-safe streaming

Requirements
- Implement a stream router
- Route agent output events to specific panes
- Handle backpressure without data loss
- Buffer streams for replay

Constraints
- No direct agent → pane coupling
- Routing decisions must be event-driven
- Replay must reproduce exact routing
- No dropped or reordered events

Exit Criteria
- Multiple agents stream simultaneously
- Each pane receives the correct stream
- Replay reproduces streams exactly