PHASE 9 — Agent ↔ Agent Messaging

You are implementing structured communication between agents.

Goals
- Allow agents to collaborate
- Support task requests and context sharing
- Make all communication observable and replayable

Requirements
- Implement an in-process message bus
- Define structured message types (request, context, artifact, status)
- Emit events for all messages
- Persist messages as part of sessions

Constraints
- No direct agent references
- Messaging must go through the bus
- Messaging must respect permission boundaries
- Messages must not mutate agent state implicitly

Exit Criteria
- Agents can exchange messages
- Messages are visible in the TUI
- Messages replay deterministically