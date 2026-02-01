You are implementing YAML-based agent templates.

Goals:
- Allow agents to be declared declaratively
- Instantiate agents via adapters
- Enforce limits from templates

Requirements:
- Define YAML schema for agent templates
- Implement template loader and validator
- Instantiate agents from templates
- Add :spawn-template command
- List templates in the TUI

Constraints:
- Templates must be framework-agnostic
- Validation errors must be explicit
- No hardcoded LLM-specific assumptions outside adapter

Exit Criteria:
- Agents can be spawned from YAML
- Limits are enforced correctly
