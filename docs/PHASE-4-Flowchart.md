flowchart LR
  TUI --> Manager
  Manager --> RemoteAdapter
  RemoteAdapter -->|gRPC| RemoteRuntime
  RemoteRuntime -->|stream| RemoteAdapter
  RemoteAdapter --> Bus
  Bus --> TUI
