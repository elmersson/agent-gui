You are implementing remote agent execution.

Goals:
- Allow agents to run outside the local process
- Maintain identical behavior to local agents

Requirements:
- Define a remote agent protocol
- Implement a remote adapter
- Support streaming output over the network
- Handle disconnects gracefully
- Display remote status in TUI

Constraints:
- Streaming must be bi-directional
- Remote failures must not crash the TUI
- Security must be token-based

Exit Criteria:
- Local and remote agents behave identically
- Remote agents stream output live
