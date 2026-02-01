flowchart LR
  TUI -->|commands| Manager
  Manager -->|Run| Agent
  Agent -->|events| Bus
  Bus -->|subscribe| TUI
  Agent -->|stream| ClaudeAPI
  Manager --> Disk[(Session JSON)]
