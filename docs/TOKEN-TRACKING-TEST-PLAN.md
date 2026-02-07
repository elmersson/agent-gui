# Token Tracking Test Plan

## Test Environment Setup

```bash
# 1. Build the application
cd /Users/rasmus.elmersson/Documents/Personal/Github/agent-gui
make build

# 2. Verify build
./agent-gui --version

# 3. Ensure Claude API key is set
echo $ANTHROPIC_API_KEY
```

## Test Suite

### Test 1: Basic Token Display

**Objective**: Verify tokens appear in header after first response

**Steps**:
1. Launch `./agent-gui`
2. Enter prompt: `"Count to 5"`
3. Observe header during streaming

**Expected**:
```
┌─────────────────────────────────────────────────┐
│ opencode  RUNNING  model: claude-sonnet-4-...  │
│ in:15 out:25  $0.0006                          │
└─────────────────────────────────────────────────┘
```

**Validation**:
- [ ] Token counts appear during streaming
- [ ] Counts update in real-time
- [ ] Final count is accurate
- [ ] Cost displays with 4 decimal places

---

### Test 2: Input/Output Breakdown

**Objective**: Verify separate display of input and output tokens

**Steps**:
1. Enter a short prompt: `"Hello"`
2. Note input token count
3. Wait for response completion
4. Verify output token count

**Expected**:
- Display shows `in:X out:Y` format
- Input tokens: ~10-20 (short prompt)
- Output tokens: ~50-100 (greeting response)
- Ratio: Output > Input for generative tasks

**Validation**:
- [ ] Input tokens shown separately
- [ ] Output tokens shown separately
- [ ] Numbers use comma formatting if >1000
- [ ] Total = Input + Output

---

### Test 3: Number Formatting with Commas

**Objective**: Verify comma separators for large token counts

**Steps**:
1. Enter: `"Write a comprehensive guide to Python programming with many examples"`
2. Wait for long response
3. Check token display

**Expected**:
```
in:25 out:12,456  $0.1890
```

**Validation**:
- [ ] Numbers <1000: No commas (e.g., `456`)
- [ ] Numbers ≥1000: Commas (e.g., `1,234`)
- [ ] Numbers ≥10k: Commas (e.g., `12,456`)
- [ ] Numbers ≥100k: Commas (e.g., `123,456`)

---

### Test 4: Cache Display

**Objective**: Verify cache hits are displayed

**Setup**: Use repeated prompts to trigger caching

**Steps**:
1. Enter: `"Count to 10"` (first time)
2. Note tokens: `in:15 out:45  $0.0009`
3. Enter: `"Count to 10"` (same prompt)
4. Observe cache display

**Expected**:
```
in:15 out:45 cache:50  $0.0009
```

**Validation**:
- [ ] `cache:X` appears on second run
- [ ] Cache count > 0
- [ ] Cost is lower than first run
- [ ] Cache display hidden when CacheRead == 0

---

### Test 5: Cost Calculation Accuracy

**Objective**: Verify cost matches manual calculation

**Model**: `claude-sonnet-4-20250514` ($3/$15 per 1M tokens)

**Steps**:
1. Use model: `:model claude-sonnet-4-20250514`
2. Enter: `"Write 'hello world'"`
3. Note token counts from display
4. Calculate expected cost manually

**Manual Calculation**:
```
Input:  20 tokens × $3.00/1M  = 20 × 0.000003 = $0.0000600
Output: 50 tokens × $15.00/1M = 50 × 0.000015 = $0.0007500
                                              ─────────────
Total:                                          $0.0008100
```

**Expected Display**:
```
in:20 out:50  $0.0008
```

**Validation**:
- [ ] Cost matches calculation (±$0.0001)
- [ ] Cost updates with final token count
- [ ] Cost is never negative

---

### Test 6: Model Switching

**Objective**: Verify cost updates when switching models

**Steps**:
1. Start with Haiku: `:model claude-3-5-haiku-20241022`
2. Enter: `"Hello"`
3. Note cost (should be low)
4. Switch to Opus: `:model claude-opus-4-20250514`
5. Enter: `"Hello"` (same prompt)
6. Compare costs

**Expected**:
- Haiku cost: ~$0.0001 (cheap)
- Opus cost: ~$0.0015 (5x more expensive)
- Display updates model name in header

**Validation**:
- [ ] Haiku cost < Sonnet cost < Opus cost
- [ ] Model name updates in header
- [ ] Pricing table used correctly

---

### Test 7: Zero Token State

**Objective**: Verify display when no tokens consumed

**Steps**:
1. Launch fresh instance
2. Observe header before any agent runs

**Expected**:
```
┌─────────────────────────────────────────────────┐
│ opencode  IDLE  model: auto                    │
│                                                 │  ← No token display
└─────────────────────────────────────────────────┘
```

**Validation**:
- [ ] Token info hidden when `TotalTokens == 0`
- [ ] No display glitches or errors
- [ ] Clean header layout

---

### Test 8: Session Storage

**Objective**: Verify tokens saved to session files

**Steps**:
1. Run agent: `"Count to 10"`
2. Wait for completion
3. Check session directory

```bash
ls sessions/
# Find latest session file
LATEST=$(ls -t sessions/*.json | head -1)
cat $LATEST | jq .token_usage
```

**Expected Output**:
```json
{
  "input_tokens": 15,
  "output_tokens": 45,
  "cache_read": 0,
  "total_tokens": 60,
  "cost_usd": 0.0009
}
```

**Validation**:
- [ ] Session file created
- [ ] `token_usage` field present
- [ ] All token fields populated
- [ ] Cost matches TUI display

---

### Test 9: Multi-Turn Conversation

**Objective**: Verify tokens accumulate or reset correctly

**Note**: Current implementation shows **per-turn** tokens, not cumulative

**Steps**:
1. Enter: `"What is 2+2?"`
2. Note tokens: `in:15 out:20`
3. Enter: `"What is 3+3?"`
4. Check if tokens reset or accumulate

**Expected Behavior**:
- Tokens reset for each turn
- Each interaction tracked separately
- Session file stores last turn's tokens

**Validation**:
- [ ] Token display updates for new turn
- [ ] Previous turn tokens not shown
- [ ] Session stores final turn tokens

---

### Test 10: Remote Agent Tokens

**Objective**: Verify tokens tracked for remote agents

**Setup**: Requires remote agent server

**Steps**:
1. Start remote agent server
2. Connect: `./agent-gui --remote localhost:50051`
3. Run prompt: `"Hello"`
4. Check token display

**Expected**:
- Token tracking works same as local
- Display shows `● Remote` status
- Tokens emit across gRPC

**Validation**:
- [ ] Remote connection successful
- [ ] Tokens display in header
- [ ] Cost calculated correctly
- [ ] No token loss over network

---

### Test 11: Pipeline Token Aggregation

**Objective**: Verify tokens tracked across pipeline stages

**Steps**:
1. Run pipeline: `:run-pipeline planner-coder-reviewer "Add login"`
2. Watch token display update per stage
3. Check if tokens aggregate

**Expected**:
```
Stage 1 (Planner):  in:50 out:200   $0.0036
Stage 2 (Coder):    in:250 out:1000 $0.0165
Stage 3 (Reviewer): in:1250 out:500 $0.0113
```

**Validation**:
- [ ] Tokens update per stage
- [ ] Each stage shows separate counts
- [ ] Pipeline execution saved to file
- [ ] Total cost calculated

---

### Test 12: Window Resize

**Objective**: Verify token display adapts to terminal size

**Steps**:
1. Run agent with tokens displaying
2. Resize terminal to very narrow (40 cols)
3. Resize to very wide (200 cols)
4. Observe token display layout

**Expected**:
- Narrow: Token info wraps or truncates gracefully
- Wide: Token info right-aligned with spacing
- No overlap with other header elements

**Validation**:
- [ ] No visual glitches
- [ ] All info readable at 80+ cols
- [ ] Graceful degradation at <80 cols

---

### Test 13: Very Large Token Counts

**Objective**: Test formatting with extreme values

**Simulation**: Manually set large values (requires code modification or very long prompts)

**Test Values**:
- 999 tokens → `"999"` (no comma)
- 1,000 tokens → `"1,000"`
- 12,345 tokens → `"12,345"`
- 123,456 tokens → `"123,456"`
- 1,234,567 tokens → `"1,234,567"`

**Steps**:
1. Generate very long response (e.g., "Write a novel")
2. Check formatting at various thresholds

**Validation**:
- [ ] Comma placement correct
- [ ] No formatting errors
- [ ] Display width stays reasonable

---

### Test 14: Error Handling

**Objective**: Verify tokens display survives errors

**Steps**:
1. Run agent: `"Hello"`
2. Interrupt with `:stop`
3. Check token display persists
4. Run another prompt
5. Verify tokens update correctly

**Expected**:
- Tokens from interrupted run stay visible
- New run updates display
- No stale data

**Validation**:
- [ ] Tokens survive stop/pause
- [ ] Display updates on new run
- [ ] No token count corruption

---

### Test 15: Cost Edge Cases

**Objective**: Test extreme cost values

**Test Cases**:

**Very Small Cost** (<$0.0001):
```bash
> "Hi"  # 2-word response
Expected: $0.0000 (rounds down)
```

**Medium Cost** (~$0.10):
```bash
> "Write a comprehensive guide"
Expected: $0.1234
```

**Large Cost** (>$1.00):
```bash
> "Write a full e-commerce system"  # Very long output
Expected: $1.2345
```

**Validation**:
- [ ] Small costs don't show as negative
- [ ] 4 decimal places maintained
- [ ] Very small costs show as $0.0000
- [ ] Large costs display fully

---

## Automated Test Script

Save as `test-tokens.sh`:

```bash
#!/bin/bash

echo "Token Tracking Test Suite"
echo "========================="
echo ""

# Test 1: Basic functionality
echo "Test 1: Basic token display"
echo "Expected: Tokens appear after response"
echo -e 'Count to 5\nquit' | ./agent-gui
echo ""

# Test 2: Session storage
echo "Test 2: Session file creation"
LATEST=$(ls -t sessions/*.json | head -1)
if [[ -f "$LATEST" ]]; then
    echo "✓ Session file exists: $LATEST"
    cat "$LATEST" | jq .token_usage
else
    echo "✗ No session file found"
fi
echo ""

# Test 3: Number formatting
echo "Test 3: formatNumber function"
go test -run TestFormatNumber ./pkg/tui/
echo ""

# Test 4: Cost calculation
echo "Test 4: Cost calculation"
go test -run TestCalculateCost ./pkg/agent/
echo ""

echo "========================="
echo "Manual tests required:"
echo "- Visual display verification"
echo "- Multi-turn conversations"
echo "- Remote agent tokens"
echo "- Pipeline aggregation"
```

## Unit Tests

Add to `pkg/tui/model_test.go`:

```go
package tui

import "testing"

func TestFormatNumber(t *testing.T) {
    tests := []struct {
        input    int
        expected string
    }{
        {0, "0"},
        {1, "1"},
        {12, "12"},
        {123, "123"},
        {1234, "1,234"},
        {12345, "12,345"},
        {123456, "123,456"},
        {1234567, "1,234,567"},
    }

    for _, tt := range tests {
        result := formatNumber(tt.input)
        if result != tt.expected {
            t.Errorf("formatNumber(%d) = %s; want %s", 
                tt.input, result, tt.expected)
        }
    }
}
```

Add to `pkg/agent/agent_test.go`:

```go
package agent

import (
    "math"
    "testing"
)

func TestCalculateCost(t *testing.T) {
    tests := []struct {
        name         string
        model        string
        inputTokens  int
        outputTokens int
        expectedCost float64
    }{
        {
            name:         "Claude Sonnet basic",
            model:        "claude-sonnet-4-20250514",
            inputTokens:  1000,
            outputTokens: 1000,
            expectedCost: 0.018, // (1000*3 + 1000*15) / 1M
        },
        {
            name:         "Claude Haiku cheap",
            model:        "claude-3-5-haiku-20241022",
            inputTokens:  1000,
            outputTokens: 1000,
            expectedCost: 0.0048, // (1000*0.8 + 1000*4) / 1M
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            usage := TokenUsage{
                InputTokens:  tt.inputTokens,
                OutputTokens: tt.outputTokens,
            }
            cost := usage.CalculateCost(tt.model)
            
            if math.Abs(cost-tt.expectedCost) > 0.0001 {
                t.Errorf("CalculateCost() = %f; want %f", 
                    cost, tt.expectedCost)
            }
        })
    }
}
```

## Performance Tests

Test token update throughput:

```bash
# Measure update rate
> "Write a very long comprehensive guide to Go programming with 
   many examples and detailed explanations"

# Observe in logs or with profiler:
# - Token updates per second
# - Channel buffer usage
# - UI responsiveness
```

**Targets**:
- Token updates: >100/sec
- UI refresh: <16ms (60 FPS)
- No dropped updates

## Regression Tests

Re-run after any changes to:
- `pkg/agent/agent.go` (pricing, calculations)
- `pkg/adapter/claude.go` (token emission)
- `pkg/tui/model.go` (display logic)
- `pkg/manager/manager.go` (event routing)

## Test Completion Checklist

- [ ] All 15 manual tests passed
- [ ] Unit tests added and passing
- [ ] Session files validated
- [ ] Performance acceptable
- [ ] No visual regressions
- [ ] Documentation updated
- [ ] Edge cases handled

## Known Limitations

1. **Per-Turn Tracking**: Tokens don't accumulate across conversation turns
2. **Estimation Accuracy**: Streaming estimates within ~5% of final
3. **Cache Visibility**: Only shows cache reads, not writes
4. **Single Currency**: USD only, no currency conversion
5. **Manual Pricing**: Model prices must be updated in code

## Future Test Additions

- [ ] Cumulative session tracking
- [ ] Token rate display (tok/sec)
- [ ] Budget alerts
- [ ] Export functionality
- [ ] Detailed breakdown tooltip
