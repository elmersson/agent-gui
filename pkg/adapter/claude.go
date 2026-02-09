package adapter

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/tokenizer"
)

type OpenCodeAdapter struct {
	model     string
	tokenizer *tokenizer.Tokenizer
	// TokenUsageChan emits token usage updates during streaming
	TokenUsageChan chan agent.TokenUsage
	// Conversation history for maintaining context
	history []agent.Message
}

type OpenCodeEvent struct {
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	SessionID string          `json:"sessionID"`
	Part      json.RawMessage `json:"part"`
}

type TextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type StepFinishPart struct {
	Type   string  `json:"type"`
	Reason string  `json:"reason"`
	Cost   float64 `json:"cost"`
	Tokens struct {
		Input     int `json:"input"`
		Output    int `json:"output"`
		Reasoning int `json:"reasoning"`
		Cache     struct {
			Read  int `json:"read"`
			Write int `json:"write"`
		} `json:"cache"`
	} `json:"tokens"`
}

func NewClaudeAdapter(config agent.Config) *OpenCodeAdapter {
	return &OpenCodeAdapter{
		model:          config.Model,
		tokenizer:      tokenizer.NewTokenizer(),
		TokenUsageChan: make(chan agent.TokenUsage, 100),
		history:        make([]agent.Message, 0),
	}
}

// ClearHistory clears the conversation history
func (a *OpenCodeAdapter) ClearHistory() {
	a.history = make([]agent.Message, 0)
}

// GetHistory returns the current conversation history
func (a *OpenCodeAdapter) GetHistory() []agent.Message {
	return a.history
}

func (a *OpenCodeAdapter) Name() string {
	return "opencode"
}

func (a *OpenCodeAdapter) Execute(ctx context.Context, input string) (<-chan string, <-chan error) {
	outputCh := make(chan string, 100)
	errCh := make(chan error, 100)
	// Create a new token channel for this execution
	a.TokenUsageChan = make(chan agent.TokenUsage, 100)

	// Add user message to history
	a.history = append(a.history, agent.Message{Role: "user", Content: input})

	go func() {
		defer close(outputCh)
		defer close(errCh)
		defer close(a.TokenUsageChan)

		// Build full prompt with conversation history
		var fullPrompt string
		if len(a.history) > 1 {
			fullPrompt = "Continue the conversation. Here is the history:\n\n"
			for _, msg := range a.history[:len(a.history)-1] {
				if msg.Role == "user" {
					fullPrompt += "User: " + msg.Content + "\n\n"
				} else {
					fullPrompt += "Assistant: " + msg.Content + "\n\n"
				}
			}
			fullPrompt += "User: " + input + "\n\nPlease respond to the latest user message while considering the conversation context above."
		} else {
			fullPrompt = input
		}

		args := []string{"run", "--format", "json"}
		if a.model != "" {
			args = append(args, "--model", a.model)
		}
		args = append(args, fullPrompt)

		cmd := exec.CommandContext(ctx, "opencode", args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			errCh <- fmt.Errorf("failed to get stdout: %w", err)
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			errCh <- fmt.Errorf("failed to get stderr: %w", err)
			return
		}

		if err := cmd.Start(); err != nil {
			errCh <- fmt.Errorf("failed to start opencode: %w", err)
			return
		}

		// Read stderr in background for error reporting
		stderrDone := make(chan struct{})
		go func() {
			defer close(stderrDone)
			stderrScanner := bufio.NewScanner(stderr)
			var stderrOutput string
			for stderrScanner.Scan() {
				stderrOutput += stderrScanner.Text() + "\n"
			}
			if stderrOutput != "" {
				select {
				case errCh <- fmt.Errorf("opencode stderr: %s", stderrOutput):
				case <-ctx.Done():
				}
			}
		}()

		// Track token usage - use estimates during streaming, real values at step_finish
		var usage agent.TokenUsage
		var estimatedOutputTokens int
		var fullResponse strings.Builder

		scanner := bufio.NewScanner(stdout)
		// Increase buffer size to handle large JSON responses (default is 64KB, increase to 10MB)
		const maxScannerBuffer = 10 * 1024 * 1024
		scanner.Buffer(make([]byte, 0, 64*1024), maxScannerBuffer)

		for scanner.Scan() {
			line := scanner.Bytes()

			var event OpenCodeEvent
			if err := json.Unmarshal(line, &event); err != nil {
				continue
			}

			switch event.Type {
			case "text":
				var part TextPart
				if err := json.Unmarshal(event.Part, &part); err != nil {
					continue
				}

				// Collect full response for history
				fullResponse.WriteString(part.Text)

				// Estimate tokens during streaming for live updates
				chunkTokens := a.tokenizer.CountTokens(part.Text)
				estimatedOutputTokens += chunkTokens
				usage.OutputTokens = estimatedOutputTokens
				usage.TotalTokens = usage.InputTokens + usage.OutputTokens

				// Emit estimated token usage update (non-blocking)
				select {
				case a.TokenUsageChan <- usage:
				default:
				}

				select {
				case outputCh <- part.Text:
				case <-ctx.Done():
					cmd.Process.Kill()
					return
				}

			case "step_finish":
				// Parse real token counts and cost from API response
				var part StepFinishPart
				if err := json.Unmarshal(event.Part, &part); err != nil {
					continue
				}

				// Update with real values from the API
				usage.InputTokens = part.Tokens.Input
				usage.OutputTokens = part.Tokens.Output
				usage.CacheRead = part.Tokens.Cache.Read
				usage.CacheWrite = part.Tokens.Cache.Write
				usage.TotalTokens = usage.InputTokens + usage.OutputTokens + usage.CacheRead
				usage.CostUSD = part.Cost

				// Emit final accurate token usage
				select {
				case a.TokenUsageChan <- usage:
				default:
				}
			}
		}

		// Add assistant response to history
		if resp := fullResponse.String(); resp != "" {
			a.history = append(a.history, agent.Message{Role: "assistant", Content: resp})
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("scan error: %w", err)
		}

		// Wait for stderr goroutine to finish
		<-stderrDone

		cmd.Wait()
	}()

	return outputCh, errCh
}

// GetTokenUsageChan returns a channel that emits token usage updates during streaming
func (a *OpenCodeAdapter) GetTokenUsageChan() <-chan agent.TokenUsage {
	return a.TokenUsageChan
}

// SetModel changes the model used by the adapter
func (a *OpenCodeAdapter) SetModel(model string) {
	a.model = model
}

// GetModel returns the current model
func (a *OpenCodeAdapter) GetModel() string {
	return a.model
}
