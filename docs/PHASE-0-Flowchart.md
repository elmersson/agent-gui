flowchart LR
  TUI -->|commands| Manager
  Manager -->|Run| Agent
  Agent -->|events| Bus
  Bus -->|subscribe| TUI
  Agent -->|stream| LLM_API
  Manager --> Disk[(Session JSON)]
