You are implementing deterministic session replay.

Goals:
- Replay sessions exactly as they occurred
- Provide timeline-based inspection in the TUI

Requirements:
- Persist all events with timestamps
- Implement replay engine
- Add replay mode to TUI
- Support timeline scrubbing and speed control

Constraints:
- Replay must be read-only
- Replay must not invoke live agents

Exit Criteria:
- Sessions replay deterministically
- Developers can debug using replay alone
