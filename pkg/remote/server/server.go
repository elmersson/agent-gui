package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/adapter"
	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
	"github.com/rasmuselmersson/opencode/pkg/remote/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Config contains configuration for the remote agent server
type Config struct {
	// Port is the port to listen on
	Port int

	// AuthTokens is a list of valid authentication tokens
	AuthTokens []string

	// AgentConfig contains configuration for creating local agents
	AgentConfig agent.Config

	// MaxConcurrentExecutions limits how many agents can run concurrently
	MaxConcurrentExecutions int

	// ExecutionTimeout is the maximum time allowed for a single execution
	ExecutionTimeout time.Duration
}

// Server implements the remote agent gRPC server
type Server struct {
	proto.UnimplementedAgentServiceServer
	config             Config
	grpcServer         *grpc.Server
	eventBus           events.Bus
	activeExecutions   map[string]context.CancelFunc
	executionsLock     sync.RWMutex
	executionSemaphore chan struct{}
}

// NewServer creates a new remote agent server
func NewServer(config Config, eventBus events.Bus) *Server {
	// Create semaphore for limiting concurrent executions
	var sem chan struct{}
	if config.MaxConcurrentExecutions > 0 {
		sem = make(chan struct{}, config.MaxConcurrentExecutions)
	}

	return &Server{
		config:             config,
		eventBus:           eventBus,
		activeExecutions:   make(map[string]context.CancelFunc),
		executionSemaphore: sem,
	}
}

// Start starts the gRPC server
func (s *Server) Start() error {
	// Create listener
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Create gRPC server with interceptors
	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(s.authInterceptor),
		grpc.StreamInterceptor(s.authStreamInterceptor),
	)

	// Register the AgentService implementation
	proto.RegisterAgentServiceServer(s.grpcServer, s)

	log.Printf("Remote agent server starting on port %d", s.config.Port)

	// Start serving (blocking)
	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop() {
	log.Println("Stopping remote agent server...")

	// Cancel all active executions
	s.executionsLock.Lock()
	for requestID, cancel := range s.activeExecutions {
		log.Printf("Cancelling execution: %s", requestID)
		cancel()
	}
	s.executionsLock.Unlock()

	// Stop gRPC server
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	log.Println("Remote agent server stopped")
}

// authInterceptor validates authentication tokens for unary RPCs
func (s *Server) authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if err := s.validateAuth(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

// authStreamInterceptor validates authentication tokens for streaming RPCs
func (s *Server) authStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := s.validateAuth(ss.Context()); err != nil {
		return err
	}
	return handler(srv, ss)
}

// validateAuth checks if the request has a valid auth token
func (s *Server) validateAuth(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// Extract token (format: "Bearer <token>")
	authHeader := authHeaders[0]
	if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		return status.Error(codes.Unauthenticated, "invalid authorization format")
	}

	token := authHeader[7:]

	// Validate token
	valid := false
	for _, validToken := range s.config.AuthTokens {
		if token == validToken {
			valid = true
			break
		}
	}

	if !valid {
		return status.Error(codes.Unauthenticated, "invalid auth token")
	}

	return nil
}

// Execute implements the Execute RPC (bidirectional streaming)
func (s *Server) Execute(stream grpc.BidiStreamingServer[proto.ExecuteRequest, proto.ExecuteResponse]) error {
	var requestID string
	var executionCtx context.Context
	var executionCancel context.CancelFunc

	// Acquire semaphore if limit is set
	if s.executionSemaphore != nil {
		select {
		case s.executionSemaphore <- struct{}{}:
			defer func() { <-s.executionSemaphore }()
		default:
			return status.Error(codes.ResourceExhausted, "max concurrent executions reached")
		}
	}

	// Receive first message (must be StartExecution)
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to receive request: %v", err)
	}

	startMsg, ok := req.Payload.(*proto.ExecuteRequest_Start)
	if !ok {
		return status.Error(codes.InvalidArgument, "first message must be StartExecution")
	}

	start := startMsg.Start
	requestID = start.RequestId

	log.Printf("Starting execution: %s (agent: %s)", requestID, start.AgentName)

	// Create execution context with timeout
	baseCtx := stream.Context()
	if s.config.ExecutionTimeout > 0 {
		executionCtx, executionCancel = context.WithTimeout(baseCtx, s.config.ExecutionTimeout)
	} else {
		executionCtx, executionCancel = context.WithCancel(baseCtx)
	}
	defer executionCancel()

	// Register active execution
	s.executionsLock.Lock()
	s.activeExecutions[requestID] = executionCancel
	s.executionsLock.Unlock()

	defer func() {
		s.executionsLock.Lock()
		delete(s.activeExecutions, requestID)
		s.executionsLock.Unlock()
	}()

	// Send status: STARTED
	if err := stream.Send(&proto.ExecuteResponse{
		RequestId: requestID,
		Payload: &proto.ExecuteResponse_Status{
			Status: &proto.ExecutionStatus{
				State:     proto.ExecutionStatus_STARTED,
				Message:   "Execution started",
				Timestamp: time.Now().UnixMilli(),
			},
		},
	}); err != nil {
		return err
	}

	// Create local agent for execution
	localAgent := adapter.NewClaudeAdapter(s.config.AgentConfig)

	// Start execution
	outputCh, errCh := localAgent.Execute(executionCtx, start.Input)

	// Get token tracking channel
	tokenCh := localAgent.GetTokenUsageChan()

	// Start goroutine to handle incoming messages (cancel, heartbeat)
	incomingDone := make(chan struct{})
	go func() {
		defer close(incomingDone)
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Printf("Error receiving from client: %v", err)
				return
			}

			switch req.Payload.(type) {
			case *proto.ExecuteRequest_Cancel:
				log.Printf("Received cancellation for: %s", requestID)
				executionCancel()
				return

			case *proto.ExecuteRequest_Heartbeat:
				// Send heartbeat ack
				stream.Send(&proto.ExecuteResponse{
					RequestId: requestID,
					Payload: &proto.ExecuteResponse_HeartbeatAck{
						HeartbeatAck: &proto.HeartbeatAck{
							Timestamp: time.Now().UnixMilli(),
						},
					},
				})
			}
		}
	}()

	// Send status: RUNNING
	stream.Send(&proto.ExecuteResponse{
		RequestId: requestID,
		Payload: &proto.ExecuteResponse_Status{
			Status: &proto.ExecutionStatus{
				State:     proto.ExecutionStatus_RUNNING,
				Message:   "Execution in progress",
				Timestamp: time.Now().UnixMilli(),
			},
		},
	})

	// Stream outputs, errors, and token usage
	var lastErr error
	for {
		select {
		case output, ok := <-outputCh:
			if !ok {
				outputCh = nil
				break
			}
			if err := stream.Send(&proto.ExecuteResponse{
				RequestId: requestID,
				Payload: &proto.ExecuteResponse_Output{
					Output: &proto.OutputChunk{
						Text:      output,
						Timestamp: time.Now().UnixMilli(),
					},
				},
			}); err != nil {
				return err
			}

		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				break
			}
			lastErr = err
			if err := stream.Send(&proto.ExecuteResponse{
				RequestId: requestID,
				Payload: &proto.ExecuteResponse_Error{
					Error: &proto.ErrorMessage{
						Message: err.Error(),
						Fatal:   false,
					},
				},
			}); err != nil {
				return err
			}

		case usage, ok := <-tokenCh:
			if !ok {
				tokenCh = nil
				break
			}
			if err := stream.Send(&proto.ExecuteResponse{
				RequestId: requestID,
				Payload: &proto.ExecuteResponse_Tokens{
					Tokens: &proto.TokenUsageUpdate{
						InputTokens:  int32(usage.InputTokens),
						OutputTokens: int32(usage.OutputTokens),
						CacheRead:    int32(usage.CacheRead),
						CacheWrite:   int32(usage.CacheWrite),
						TotalTokens:  int32(usage.TotalTokens),
						CostUsd:      usage.CostUSD,
						IsFinal:      true, // Assume final for simplicity
					},
				},
			}); err != nil {
				return err
			}

		case <-executionCtx.Done():
			// Execution cancelled or timed out
			finalState := proto.ExecutionStatus_CANCELLED
			finalMsg := "Execution cancelled"
			if executionCtx.Err() == context.DeadlineExceeded {
				finalState = proto.ExecutionStatus_FAILED
				finalMsg = "Execution timeout"
			}

			stream.Send(&proto.ExecuteResponse{
				RequestId: requestID,
				Payload: &proto.ExecuteResponse_Status{
					Status: &proto.ExecutionStatus{
						State:     finalState,
						Message:   finalMsg,
						Timestamp: time.Now().UnixMilli(),
					},
				},
			})
			return nil
		}

		// Check if all channels are closed
		if outputCh == nil && errCh == nil && tokenCh == nil {
			break
		}
	}

	// Wait for incoming message handler to finish
	<-incomingDone

	// Send final status
	finalState := proto.ExecutionStatus_COMPLETED
	finalMsg := "Execution completed successfully"
	if lastErr != nil {
		finalState = proto.ExecutionStatus_FAILED
		finalMsg = fmt.Sprintf("Execution failed: %v", lastErr)
	}

	if err := stream.Send(&proto.ExecuteResponse{
		RequestId: requestID,
		Payload: &proto.ExecuteResponse_Status{
			Status: &proto.ExecutionStatus{
				State:     finalState,
				Message:   finalMsg,
				Timestamp: time.Now().UnixMilli(),
			},
		},
	}); err != nil {
		return err
	}

	log.Printf("Execution completed: %s", requestID)
	return nil
}

// HealthCheck implements the HealthCheck RPC
func (s *Server) HealthCheck(ctx context.Context, req *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	return &proto.HealthCheckResponse{
		Status:  proto.HealthCheckResponse_SERVING,
		Message: "Remote agent server is healthy",
	}, nil
}
