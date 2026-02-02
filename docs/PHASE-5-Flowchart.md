sequenceDiagram
  Disk->>ReplayEngine: load events
  ReplayEngine->>Bus: re-emit events
  Bus->>TUI: update UI
  TUI->>ReplayEngine: scrub timeline
