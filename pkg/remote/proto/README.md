# Remote Agent Protocol Buffers

This directory contains the Protocol Buffer definitions for the remote agent execution protocol.

## Requirements

To generate the gRPC code, you need:

1. **Protocol Buffer Compiler (protoc)**:
   - macOS: `brew install protobuf`
   - Ubuntu: `sudo apt install protobuf-compiler`
   - Other: https://grpc.io/docs/protoc-installation/

2. **Go Protocol Buffer Plugins**:
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```

## Generating Code

After installing the requirements, run:

```bash
# From project root
make proto

# Or use the script directly
./scripts/generate-proto.sh
```

This will generate:
- `agent.pb.go` - Protocol Buffer message definitions
- `agent_grpc.pb.go` - gRPC service definitions

## Protocol Overview

The protocol supports:

- **Bidirectional Streaming**: Client can send cancellation signals, server streams output
- **Real-time Token Tracking**: Estimated and final token usage updates
- **Connection Health**: Heartbeat mechanism for detecting disconnects
- **Secure Authentication**: Token-based auth on each request
- **Error Handling**: Distinguishes between recoverable and fatal errors

## Message Types

### Client → Server
- `StartExecution`: Initiate agent execution with input and auth
- `CancelExecution`: Request termination of running execution
- `Heartbeat`: Keep connection alive

### Server → Client
- `OutputChunk`: Streaming text output from agent
- `TokenUsageUpdate`: Token consumption updates (estimated + final)
- `ErrorMessage`: Errors during execution
- `ExecutionStatus`: State transitions (STARTED, RUNNING, COMPLETED, etc.)
- `HeartbeatAck`: Acknowledge client heartbeat

## Security

All requests must include a valid `auth_token` in the `StartExecution` message. The server validates tokens before executing any agent code.
