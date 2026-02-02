flowchart LR
  Planner -->|message| Coder
  Coder -->|output| Reviewer

  subgraph Pipeline
    Planner --> Coder --> Reviewer
  end

  Pipeline --> Bus
  Bus --> TUI
