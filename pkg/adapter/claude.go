package adapter

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/rasmuselmersson/opencode/pkg/agent"
)

type OpenCodeAdapter struct {
	model string
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

func NewClaudeAdapter(config agent.Config) *OpenCodeAdapter {
	return &OpenCodeAdapter{model: config.Model}
}

func (a *OpenCodeAdapter) Name() string {
	return "opencode"
}

func (a *OpenCodeAdapter) Execute(ctx context.Context, input string) (<-chan string, <-chan error) {
	outputCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(outputCh)
		defer close(errCh)

		args := []string{"run", "--format", "json"}
		if a.model != "" {
			args = append(args, "-m", a.model)
		}
		args = append(args, input)

		cmd := exec.CommandContext(ctx, "opencode", args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			errCh <- fmt.Errorf("failed to get stdout: %w", err)
			return
		}

		if err := cmd.Start(); err != nil {
			errCh <- fmt.Errorf("failed to start opencode: %w", err)
			return
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Bytes()

			var event OpenCodeEvent
			if err := json.Unmarshal(line, &event); err != nil {
				continue
			}

			if event.Type == "text" {
				var part TextPart
				if err := json.Unmarshal(event.Part, &part); err != nil {
					continue
				}
				select {
				case outputCh <- part.Text:
				case <-ctx.Done():
					cmd.Process.Kill()
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("scan error: %w", err)
			return
		}

		cmd.Wait()
	}()

	return outputCh, errCh
}
