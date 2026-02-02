You are implementing token and cost accounting.

Goals:
- Track input/output tokens per agent
- Emit token usage updates during streaming
- Estimate cost in USD
- Persist token usage with sessions

Requirements:
- Define TokenUsage model
- Integrate Claude tokenizer
- Emit token update events incrementally
- Display tokens and cost in the TUI
- Persist token usage per session

Constraints:
- Token tracking must not block streaming
- Token data must be replayable later

Exit Criteria:
- Token usage updates live in the TUI
- Cost estimates are visible and persisted
