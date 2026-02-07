package server

import (
	"testing"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
)

func TestNewServer(t *testing.T) {
	eventBus := events.NewEventBus()

	config := Config{
		Port:                    50051,
		AuthTokens:              []string{"test-token"},
		AgentConfig:             agent.Config{Model: "claude-3-5-sonnet-20241022"},
		MaxConcurrentExecutions: 5,
		ExecutionTimeout:        10 * time.Minute,
	}

	server := NewServer(config, eventBus)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.config.Port != 50051 {
		t.Errorf("Expected port 50051, got %d", server.config.Port)
	}

	if len(server.config.AuthTokens) != 1 {
		t.Errorf("Expected 1 auth token, got %d", len(server.config.AuthTokens))
	}

	if server.config.MaxConcurrentExecutions != 5 {
		t.Errorf("Expected max concurrent executions 5, got %d", server.config.MaxConcurrentExecutions)
	}

	if server.executionSemaphore == nil {
		t.Error("Expected non-nil execution semaphore")
	}

	if cap(server.executionSemaphore) != 5 {
		t.Errorf("Expected semaphore capacity 5, got %d", cap(server.executionSemaphore))
	}
}

func TestNewServer_UnlimitedConcurrency(t *testing.T) {
	eventBus := events.NewEventBus()

	config := Config{
		Port:                    50051,
		AuthTokens:              []string{"test-token"},
		AgentConfig:             agent.Config{Model: "claude-3-5-sonnet-20241022"},
		MaxConcurrentExecutions: 0, // unlimited
		ExecutionTimeout:        10 * time.Minute,
	}

	server := NewServer(config, eventBus)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.executionSemaphore != nil {
		t.Error("Expected nil execution semaphore for unlimited concurrency")
	}
}

func TestServer_Stop(t *testing.T) {
	eventBus := events.NewEventBus()

	config := Config{
		Port:                    50052,
		AuthTokens:              []string{"test-token"},
		AgentConfig:             agent.Config{Model: "claude-3-5-sonnet-20241022"},
		MaxConcurrentExecutions: 5,
		ExecutionTimeout:        10 * time.Minute,
	}

	server := NewServer(config, eventBus)

	// Stop should not panic even if server wasn't started
	server.Stop()
}
