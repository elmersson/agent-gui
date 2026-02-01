sequenceDiagram
  Agent->>LLM_API: prompt
  LLM_API-->>Agent: stream chunk
  Agent->>Tokenizer: count tokens
  Tokenizer-->>Agent: token delta
  Agent->>Bus: agent.tokens.updated
  Bus-->>TUI: update display
  Agent->>Disk: persist usage
