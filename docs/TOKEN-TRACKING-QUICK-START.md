# Token Tracking Quick Start

## What You'll See

When you run an agent, the TUI header displays real-time token usage:

```
┌─────────────────────────────────────────────────────────────┐
│ opencode  RUNNING  model: claude-sonnet-4-20250514          │
│ in:1,234 out:5,678 cache:890  $0.0234                       │
└─────────────────────────────────────────────────────────────┘
```

## Display Components

| Component | Example | Meaning |
|-----------|---------|---------|
| `in:1,234` | Input: 1,234 tokens | Tokens consumed from your prompt and context |
| `out:5,678` | Output: 5,678 tokens | Tokens generated in the response |
| `cache:890` | Cache: 890 tokens | Tokens read from cache (cost savings!) |
| `$0.0234` | Cost: $0.0234 | Total cost in USD for this interaction |

## Understanding Tokens

### What are tokens?
Tokens are chunks of text (roughly 4 characters or 0.75 words). Examples:
- "Hello" = 1 token
- "Hello world" = 2 tokens
- "Hello, how are you?" = 5 tokens

### Why do they matter?
- **Cost**: LLM APIs charge per token
- **Limits**: Models have maximum token limits (e.g., 200k tokens)
- **Performance**: More tokens = longer processing time

## Cost Breakdown

Costs vary by model and token type:

### Claude Models (per 1M tokens)

| Model | Input | Output |
|-------|-------|--------|
| Claude Opus 4 | $15.00 | $75.00 |
| Claude Sonnet 4 | $3.00 | $15.00 |
| Claude Haiku 3.5 | $0.80 | $4.00 |

**Example Calculation** (Claude Sonnet 4):
```
Input:  1,234 tokens @ $3.00/1M  = $0.0037
Output: 5,678 tokens @ $15.00/1M = $0.0852
─────────────────────────────────────────
Total:  6,912 tokens             = $0.0889
```

### OpenAI Models (per 1M tokens)

| Model | Input | Output |
|-------|-------|--------|
| GPT-4o | $2.50 | $10.00 |
| GPT-4o-mini | $0.15 | $0.60 |
| GPT-4-turbo | $10.00 | $30.00 |

## Cache Optimization

### What is caching?
Claude models cache repeated context to save costs. When you see `cache:890`:
- Those tokens were read from cache
- **No output cost** for cached tokens
- Significant savings on repeated prompts

### How to leverage caching:
1. **Reuse prompts**: Same prompt = cache hit
2. **System prompts**: Large system instructions get cached
3. **Context windows**: Repeated context across turns benefits

**Example Savings**:
```
Without cache: 10,000 tokens @ $3.00/1M = $0.0300
With cache:    10,000 tokens @ $0.00/1M = $0.0000 (90% cache hit)
```

## Monitoring Token Usage

### During Execution
Watch the header update in real-time:
```
in:45 out:123  $0.0025     # Streaming...
in:45 out:456  $0.0069     # Still going...
in:45 out:789  $0.0120     # Complete!
```

### After Execution
Check session files for detailed breakdown:
```bash
cat sessions/20260207-080428.json | jq .token_usage
```

Output:
```json
{
  "input_tokens": 1234,
  "output_tokens": 5678,
  "cache_read": 890,
  "total_tokens": 7802,
  "cost_usd": 0.0234
}
```

## Tips for Cost Optimization

### 1. Choose the Right Model
- **Quick tasks**: Use Haiku (cheaper, faster)
- **Complex reasoning**: Use Sonnet (balanced)
- **Critical tasks**: Use Opus (most capable, expensive)

### 2. Optimize Prompts
```
❌ Bad:  "Can you please help me write a function that takes a list 
         of numbers and returns the sum? I would really appreciate 
         it if you could show me how to do this."
         (28 tokens, verbose)

✅ Good: "Write a function to sum a list of numbers."
         (10 tokens, clear)
```

### 3. Limit Output Length
```bash
# Be specific about output length
> "Explain Python lists in 50 words"  # Not: "Explain Python lists"
```

### 4. Use Caching
```bash
# Define system prompts once
:start "You are a Python expert. [long instructions here]"

# Follow-up prompts reuse cached context
> "Write a list comprehension"  # Cache hit!
> "Write a dictionary"          # Cache hit!
```

### 5. Monitor as You Go
Watch the cost display. If it's climbing too fast:
- Stop with `:stop`
- Adjust your prompt
- Try a cheaper model

## Cost Estimates

Typical costs for common tasks:

| Task | Tokens (in/out) | Cost (Sonnet) |
|------|-----------------|---------------|
| Simple code snippet | 50/200 | $0.0033 |
| Code review | 500/300 | $0.0060 |
| Documentation | 200/1000 | $0.0156 |
| Refactoring | 1000/1500 | $0.0255 |
| Full feature | 2000/5000 | $0.0810 |

**Budget Example**: $10/month
- ~12,000 simple code snippets
- ~1,600 documentation tasks
- ~123 full feature implementations

## Command Reference

### Model Management
```bash
:models              # List available models
:model <name>        # Switch model
```

### Monitoring
```bash
# View header (always visible)
# in:X out:Y cache:Z  $cost

# Check session files
ls sessions/
cat sessions/<session-id>.json
```

## Troubleshooting

### Display shows "$0.0000"
- Very short interactions may round to zero
- Actual cost is still tracked in session file

### No token display
- Check that `tokenUsage.TotalTokens > 0`
- Tokens appear after first response chunk

### Cost seems wrong
- Verify model with `:model`
- Check pricing table in code
- Remember: output tokens cost more than input

### Cache not showing
- Caching requires repeated context
- Try the same prompt twice
- Only works with Claude models

## Additional Resources

- Full implementation: `docs/TOKEN-TRACKING-IMPLEMENTATION.md`
- Model pricing: `pkg/agent/agent.go:45-62`
- Session storage: `sessions/*.json`

## Example Session

```bash
$ ./agent-gui

# Start with cheap model
:model claude-3-5-haiku-20241022

# Simple task
> "Write hello world in Go"
# Display: in:23 out:67  $0.0003

# Complex task - watch cost climb
> "Write a comprehensive web server in Go with authentication"
# Display: in:45 out:2,345  $0.0098

# Switch to better model for quality
:model claude-sonnet-4-20250514

# Same task costs more but better quality
> "Write a comprehensive web server in Go with authentication"
# Display: in:45 out:2,456  $0.0374

# Check total cost
ls sessions/
cat sessions/<latest>.json | jq .token_usage.cost_usd
# Output: 0.0475
```

## Summary

✅ **Real-time visibility**: See tokens and cost as they happen  
✅ **Detailed breakdown**: Understand input, output, and cache  
✅ **Cost control**: Choose models and optimize prompts  
✅ **Historical tracking**: Review past sessions  
✅ **Cache optimization**: Save money on repeated context  

The token tracking system helps you stay within budget while maximizing agent effectiveness!
