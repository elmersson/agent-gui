PHASE 10 — Real Parallel TUI Demo

You are implementing a real end-to-end demonstration of the system.

Goals
- Prove the architecture works in practice
- Demonstrate parallel agents, panes, and streaming
- Validate determinism and observability

Requirements
- Run 3 OpenCode agents in parallel:
 - Code generator
 - Test writer
 - Documentation writer
- Display each agent in its own pane
- Use the scheduler for execution
- Enable agent-to-agent messaging
- Track tokens and cost live

Constraints
- No mocked components
- No shortcuts around the scheduler
- UI must remain event-driven
- Demo must be reproducible

Exit Criteria
- Demo runs via a single command
- All agents stream concurrently
- Panes behave correctly
- Sessions are replayable end-to-end