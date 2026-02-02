flowchart TD
  YAML[Agent Template YAML] --> Loader
  Loader --> Validator
  Validator --> Adapter
  Adapter --> Agent
  Agent --> Manager
  Manager --> Bus
