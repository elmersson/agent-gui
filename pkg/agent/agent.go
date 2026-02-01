package agent

import (
	"context"
)

type Agent interface {
	Name() string
	Execute(ctx context.Context, input string) (<-chan string, <-chan error)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Config struct {
	APIKey string
	Model  string
}