package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
	"github.com/rasmuselmersson/opencode/pkg/remote/server"
)

func main() {
	// Parse command-line flags
	port := flag.Int("port", 50051, "Port to listen on")
	authTokens := flag.String("tokens", "", "Comma-separated list of valid auth tokens")
	apiKey := flag.String("apikey", "", "API key for the agent (or use OPENCODE_API_KEY env var)")
	model := flag.String("model", "claude-sonnet-4-20250514", "Model to use for agents")
	maxConcurrent := flag.Int("max-concurrent", 10, "Maximum concurrent executions (0 = unlimited)")
	timeout := flag.Duration("timeout", 30*time.Minute, "Execution timeout (0 = no timeout)")

	flag.Parse()

	// Validate auth tokens
	if *authTokens == "" {
		log.Fatal("Error: --tokens is required (comma-separated list of auth tokens)")
	}

	tokenList := strings.Split(*authTokens, ",")
	for i, token := range tokenList {
		tokenList[i] = strings.TrimSpace(token)
	}

	// Get API key from flag or environment
	agentAPIKey := *apiKey
	if agentAPIKey == "" {
		agentAPIKey = os.Getenv("OPENCODE_API_KEY")
	}
	if agentAPIKey == "" {
		log.Fatal("Error: API key must be provided via --apikey flag or OPENCODE_API_KEY environment variable")
	}

	// Create event bus
	eventBus := events.NewEventBus()

	// Create server config
	config := server.Config{
		Port:                    *port,
		AuthTokens:              tokenList,
		MaxConcurrentExecutions: *maxConcurrent,
		ExecutionTimeout:        *timeout,
		AgentConfig: agent.Config{
			APIKey: agentAPIKey,
			Model:  *model,
		},
	}

	// Create and start server
	srv := server.NewServer(config, eventBus)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal...")
		srv.Stop()
		os.Exit(0)
	}()

	// Start server (blocking)
	log.Printf("Starting remote agent server on port %d", *port)
	log.Printf("Using model: %s", *model)
	log.Printf("Max concurrent executions: %d", *maxConcurrent)
	if *timeout > 0 {
		log.Printf("Execution timeout: %v", *timeout)
	} else {
		log.Printf("Execution timeout: none")
	}

	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
