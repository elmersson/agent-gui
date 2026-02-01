You are implementing multi-agent coordination.

Goals:
- Support agent groups
- Support deterministic pipelines
- Enable agent-to-agent messaging

Requirements:
- Define pipeline schema
- Implement pipeline runner
- Implement agent messaging
- Emit pipeline lifecycle events
- Persist pipeline execution

Constraints:
- Pipelines must be deterministic
- Failures must halt execution unless configured otherwise

Exit Criteria:
- A planner → coder → reviewer pipeline executes correctly
- Pipeline progress is visible in the TUI
