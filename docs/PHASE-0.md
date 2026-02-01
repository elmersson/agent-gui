You are a senior Go systems engineer.

Your task is to implement PHASE 0 of an agent runtime.

Goals:
- Implement a framework-agnostic agent runtime
- Support one Claude agent with streaming output
- Use an event-driven architecture
- Ensure the TUI never calls agent logic directly

Requirements:
- Define a clean Agent interface
- Implement a Claude adapter with streaming support
- Implement an event bus
- Implement an agent manager
- Implement context-based cancel/pause
- Persist session output to disk (JSON)
- Add a TUI with a command palette (:) to control agents

Constraints:
- Do not introduce tight coupling between UI and execution
- Emit events for all state changes
- Prefer clarity over optimization
- Write idiomatic Go

Exit Criteria:
- A Claude agent streams output live in the TUI
- Agent can be cancelled or paused
- A session file is written to disk

