package adapter

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
	"github.com/rasmuselmersson/opencode/pkg/remote"
	"github.com/rasmuselmersson/opencode/pkg/remote/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// RemoteAdapter implements agent.Agent and agent.TokenTrackingAgent for remote execution
type RemoteAdapter struct {
	config         remote.RemoteConfig
	eventBus       events.Bus
	conn           *grpc.ClientConn
	connectionLock sync.RWMutex
	state          remote.ConnectionState
	tokenUsageChan chan agent.TokenUsage
}

// NewRemoteAdapter creates a new remote agent adapter
func NewRemoteAdapter(config remote.RemoteConfig, eventBus events.Bus) *RemoteAdapter {
	return &RemoteAdapter{
		config:         config,
		eventBus:       eventBus,
		state:          remote.Disconnected,
		tokenUsageChan: make(chan agent.TokenUsage, 100),
	}
}

// Name returns the name of the agent
func (r *RemoteAdapter) Name() string {
	return r.config.AgentName
}

// Execute runs the agent remotely and streams output back
func (r *RemoteAdapter) Execute(ctx context.Context, input string) (<-chan string, <-chan error) {
	outputCh := make(chan string, 100)
	errCh := make(chan error, 10)

	go func() {
		defer close(outputCh)
		defer close(errCh)
		defer close(r.tokenUsageChan)

		// Ensure connection is established
		if err := r.ensureConnection(ctx); err != nil {
			errCh <- fmt.Errorf("failed to connect to remote agent: %w", err)
			r.publishConnectionEvent(events.EventError, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		// Execute remotely with streaming
		if err := r.executeRemote(ctx, input, outputCh, errCh); err != nil {
			errCh <- err
		}
	}()

	return outputCh, errCh
}

// GetTokenUsageChan returns the token usage channel
func (r *RemoteAdapter) GetTokenUsageChan() <-chan agent.TokenUsage {
	return r.tokenUsageChan
}

// ensureConnection establishes a connection to the remote server if not already connected
func (r *RemoteAdapter) ensureConnection(ctx context.Context) error {
	r.connectionLock.Lock()
	defer r.connectionLock.Unlock()

	// If already connected, verify connection is healthy
	if r.conn != nil && r.state == remote.Connected {
		// Perform health check
		client := proto.NewAgentServiceClient(r.conn)
		md := metadata.New(map[string]string{
			"authorization": "Bearer " + r.config.AuthToken,
		})
		healthCtx := metadata.NewOutgoingContext(ctx, md)
		healthCtx, cancel := context.WithTimeout(healthCtx, 5*time.Second)
		defer cancel()

		_, err := client.HealthCheck(healthCtx, &proto.HealthCheckRequest{})
		if err == nil {
			return nil // Connection is healthy
		}
		// Connection unhealthy, need to reconnect
		r.conn.Close()
		r.conn = nil
	}

	// Establish new connection
	r.setState(remote.Connecting)
	r.publishConnectionEvent(events.EventRemoteConnecting, nil)

	var opts []grpc.DialOption
	if r.config.TLSEnabled {
		// TODO: Implement TLS credentials
		return fmt.Errorf("TLS not yet implemented")
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add timeout to connection attempt
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, r.config.Address, opts...)
	if err != nil {
		r.setState(remote.Failed)
		r.publishConnectionEvent(events.EventRemoteFailed, map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to dial remote server: %w", err)
	}

	r.conn = conn
	r.setState(remote.Connected)
	r.publishConnectionEvent(events.EventRemoteConnected, map[string]interface{}{
		"address": r.config.Address,
	})

	return nil
}

// executeRemote performs the actual remote execution with streaming
func (r *RemoteAdapter) executeRemote(ctx context.Context, input string, outputCh chan<- string, errCh chan<- error) error {
	// Add auth token to context metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + r.config.AuthToken,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Generate request ID
	requestID := fmt.Sprintf("%s-%d", r.config.AgentName, time.Now().UnixNano())

	// Create gRPC client
	client := proto.NewAgentServiceClient(r.conn)

	// Create bidirectional stream
	stream, err := client.Execute(ctx)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Send start execution request
	startReq := &proto.ExecuteRequest{
		Payload: &proto.ExecuteRequest_Start{
			Start: &proto.StartExecution{
				RequestId: requestID,
				SessionId: fmt.Sprintf("session-%d", time.Now().Unix()),
				AgentName: r.config.AgentName,
				Input:     input,
				AuthToken: r.config.AuthToken,
			},
		},
	}

	if err := stream.Send(startReq); err != nil {
		return fmt.Errorf("failed to send start request: %w", err)
	}

	// Start heartbeat goroutine
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go r.sendHeartbeats(ctx, stream, heartbeatDone)

	// Receive responses
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		switch payload := resp.Payload.(type) {
		case *proto.ExecuteResponse_Output:
			select {
			case outputCh <- payload.Output.Text:
			case <-ctx.Done():
				return ctx.Err()
			}

		case *proto.ExecuteResponse_Error:
			err := fmt.Errorf("%s", payload.Error.Message)
			select {
			case errCh <- err:
			case <-ctx.Done():
				return ctx.Err()
			}
			if payload.Error.Fatal {
				return err
			}

		case *proto.ExecuteResponse_Tokens:
			usage := agent.TokenUsage{
				InputTokens:  int(payload.Tokens.InputTokens),
				OutputTokens: int(payload.Tokens.OutputTokens),
				CacheRead:    int(payload.Tokens.CacheRead),
				CacheWrite:   int(payload.Tokens.CacheWrite),
				TotalTokens:  int(payload.Tokens.TotalTokens),
				CostUSD:      payload.Tokens.CostUsd,
			}
			select {
			case r.tokenUsageChan <- usage:
			case <-ctx.Done():
				return ctx.Err()
			}

		case *proto.ExecuteResponse_Status:
			// Handle status updates
			if payload.Status.State == proto.ExecutionStatus_COMPLETED {
				return nil
			}
			if payload.Status.State == proto.ExecutionStatus_FAILED {
				return fmt.Errorf("remote execution failed: %s", payload.Status.Message)
			}
		}
	}

	return nil
}

// sendHeartbeats sends periodic heartbeat messages to keep the connection alive
func (r *RemoteAdapter) sendHeartbeats(ctx context.Context, stream grpc.BidiStreamingClient[proto.ExecuteRequest, proto.ExecuteResponse], done <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			heartbeat := &proto.ExecuteRequest{
				Payload: &proto.ExecuteRequest_Heartbeat{
					Heartbeat: &proto.Heartbeat{
						Timestamp: time.Now().UnixMilli(),
					},
				},
			}
			if err := stream.Send(heartbeat); err != nil {
				return
			}
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// setState updates the connection state and publishes an event
func (r *RemoteAdapter) setState(state remote.ConnectionState) {
	r.state = state
}

// GetState returns the current connection state
func (r *RemoteAdapter) GetState() remote.ConnectionState {
	r.connectionLock.RLock()
	defer r.connectionLock.RUnlock()
	return r.state
}

// publishConnectionEvent publishes a connection-related event to the event bus
func (r *RemoteAdapter) publishConnectionEvent(eventType events.EventType, data map[string]interface{}) {
	if r.eventBus == nil {
		return
	}

	if data == nil {
		data = make(map[string]interface{})
	}
	data["agent_name"] = r.config.AgentName
	data["remote_address"] = r.config.Address
	data["connection_state"] = r.state.String()

	r.eventBus.Publish(events.Event{
		Type:      eventType,
		AgentName: r.config.AgentName,
		Data:      data,
	})
}

// Close closes the connection to the remote server
func (r *RemoteAdapter) Close() error {
	r.connectionLock.Lock()
	defer r.connectionLock.Unlock()

	if r.conn != nil {
		r.setState(remote.Disconnected)
		r.publishConnectionEvent(events.EventRemoteDisconnected, nil)
		return r.conn.Close()
	}
	return nil
}

// Reconnect attempts to reconnect to the remote server
func (r *RemoteAdapter) Reconnect(ctx context.Context) error {
	// Close existing connection
	r.Close()

	// Implement exponential backoff
	maxAttempts := r.config.MaxReconnectAttempts
	if maxAttempts == 0 {
		maxAttempts = -1 // infinite
	}

	backoff := time.Duration(r.config.ReconnectBackoffBase) * time.Millisecond
	if backoff == 0 {
		backoff = 1 * time.Second
	}

	attempt := 0
	for {
		attempt++
		if maxAttempts > 0 && attempt > maxAttempts {
			return fmt.Errorf("max reconnection attempts reached")
		}

		r.setState(remote.Reconnecting)
		r.publishConnectionEvent(events.EventRemoteReconnecting, map[string]interface{}{
			"attempt": attempt,
		})

		err := r.ensureConnection(ctx)
		if err == nil {
			return nil
		}

		// Exponential backoff
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}
