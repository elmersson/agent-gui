package remote

// Package remote provides types for remote agent execution.
// These types mirror the protobuf definitions in pkg/remote/proto/agent.proto
//
// NOTE: This file contains manual type definitions for development.
// In production, you should generate these from the proto file using:
//   make proto
//
// The generated files (agent.pb.go and agent_grpc.pb.go) will replace these manual definitions.

// RemoteConfig contains configuration for connecting to a remote agent server
type RemoteConfig struct {
	// Address is the gRPC server address (e.g., "localhost:50051")
	Address string

	// AuthToken is the bearer token for authentication
	AuthToken string

	// AgentName is the name of the agent to execute remotely
	AgentName string

	// TLSEnabled indicates whether to use TLS for the connection
	TLSEnabled bool

	// TLSCertPath is the path to the TLS certificate (if TLSEnabled)
	TLSCertPath string

	// MaxReconnectAttempts is the maximum number of reconnection attempts (0 = infinite)
	MaxReconnectAttempts int

	// ReconnectBackoffBase is the base duration for exponential backoff (default: 1s)
	ReconnectBackoffBase int64 // milliseconds
}

// ConnectionState represents the state of a remote connection
type ConnectionState int

const (
	// Disconnected means no connection exists
	Disconnected ConnectionState = iota

	// Connecting means attempting to establish connection
	Connecting

	// Connected means connection is active and healthy
	Connected

	// Reconnecting means attempting to restore a lost connection
	Reconnecting

	// Failed means connection failed and won't be retried
	Failed
)

// String returns the string representation of a ConnectionState
func (s ConnectionState) String() string {
	switch s {
	case Disconnected:
		return "Disconnected"
	case Connecting:
		return "Connecting"
	case Connected:
		return "Connected"
	case Reconnecting:
		return "Reconnecting"
	case Failed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// ExecutionState represents the state of a remote execution
type ExecutionState int

const (
	// StateUnknown means state is unknown
	StateUnknown ExecutionState = iota

	// StateStarted means execution has been initiated
	StateStarted

	// StateRunning means execution is in progress
	StateRunning

	// StateCompleted means execution finished successfully
	StateCompleted

	// StateFailed means execution failed with an error
	StateFailed

	// StateCancelled means execution was cancelled by the client
	StateCancelled
)

// String returns the string representation of an ExecutionState
func (s ExecutionState) String() string {
	switch s {
	case StateUnknown:
		return "Unknown"
	case StateStarted:
		return "Started"
	case StateRunning:
		return "Running"
	case StateCompleted:
		return "Completed"
	case StateFailed:
		return "Failed"
	case StateCancelled:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

// TokenUsage represents token consumption information
// This mirrors agent.TokenUsage but is defined here for proto compatibility
type TokenUsage struct {
	InputTokens  int32
	OutputTokens int32
	CacheRead    int32
	CacheWrite   int32
	TotalTokens  int32
	CostUSD      float64
	IsFinal      bool // True if this is the final count (not an estimate)
}

// ErrorInfo represents error information from remote execution
type ErrorInfo struct {
	Message string
	Code    string
	Fatal   bool // If true, execution has terminated
}
