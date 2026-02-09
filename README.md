# opencode

A terminal-based agent runtime with event-driven architecture and TUI interface.

## Prerequisites

- Go 1.25+
- `opencode` CLI installed and configured

## Installation

```bash
go build -o opencode ./cmd/opencode
```

## Usage

Run the application:

```bash
./opencode
```

### Commands

Press `:` to enter command mode, then type:

- `start <prompt>` - Start the agent with your prompt
- `stop` - Stop the running agent
- `pause` - Pause the agent execution
- `resume` - Resume a paused agent

### Keyboard Shortcuts

- `Ctrl+C` - Quit the application
- `:` - Enter command mode
- `Enter` - Submit input or command
- `Esc` - Exit command mode

## Features

- **Streaming Output**: See the agent's response in real-time
- **Event-Driven Architecture**: Clean separation between UI and agent logic
- **Session Persistence**: All sessions saved to `sessions/` directory as JSON
- **Control**: Cancel, pause, and resume agent execution
- **TUI**: Clean terminal interface with command palette
- **Pipeline Execution**: Run multi-stage agent workflows
- **Template System**: Reusable agent configurations
- **Remote Execution** (Phase 4): Run agents on remote servers with gRPC

## Project Structure

```
opencode/
├── cmd/
│   ├── opencode/       # Main application entry point
│   ├── remote-agent/   # Remote agent server (Phase 4)
│   └── remote-token/   # Token generation utility (Phase 4)
├── pkg/
│   ├── agent/          # Agent interface and types
│   ├── adapter/        # Agent adapter implementations (local + remote)
│   ├── events/         # Event bus for decoupled communication
│   ├── manager/        # Agent lifecycle management
│   ├── pipeline/       # Pipeline execution (Phase 3)
│   ├── remote/         # Remote execution infrastructure (Phase 4)
│   ├── session/        # Session persistence
│   ├── template/       # Template system (Phase 2)
│   └── tui/            # Terminal UI
├── sessions/           # Saved session files
├── templates/          # Agent templates
└── pipelines/          # Pipeline definitions
```

## Development

Run tests:

```bash
go test ./...
```

Build:

```bash
# Build main client
go build -o opencode ./cmd/opencode

# Build remote agent server (Phase 4)
go build -o opencode-remote ./cmd/remote-agent

# Build token generator (Phase 4)
go build -o opencode-token ./cmd/remote-token

# Or use Make
make build        # Build client
make build-remote # Build server
make all          # Build everything
```

## Remote Agent Execution (Phase 4)

Run agents on remote servers for distributed execution, load balancing, and scaling.

### Features

- **Bidirectional gRPC Streaming**: Real-time output and token usage updates
- **Token-Based Authentication**: Secure server access with cryptographic tokens
- **Automatic Reconnection**: Exponential backoff for network resilience
- **TUI Integration**: Live connection status display
- **Health Checks**: Verify server availability before execution
- **Concurrent Execution Limits**: Control server resource usage
- **Execution Timeouts**: Prevent runaway agents

### Quick Start

1. **Generate a token**:
```bash
./bin/remote-token --length 32
```

2. **Start the remote server**:
```bash
./bin/remote-agent --port 50051 --tokens "your-token"
```

3. **Configure the client**:
```bash
export OPENCODE_REMOTE_ADDRESS="localhost:50051"
export OPENCODE_REMOTE_TOKEN="your-token"
export OPENCODE_REMOTE_TLS="false"  # Set to "true" for production
./bin/opencode
```

The TUI will show connection status in the header:
- `● Remote` - Connected
- `◌ Connecting...` - Establishing connection
- `◌ Reconnecting (attempt N)...` - Automatic reconnection
- `✗ Connection Failed` - Connection error

### Documentation

- **[PHASE-4-QUICKSTART.md](docs/PHASE-4-QUICKSTART.md)** - 5-minute setup guide
- **[PHASE-4.md](docs/PHASE-4.md)** - Detailed implementation documentation
- **[REMOTE-AGENTS.md](docs/REMOTE-AGENTS.md)** - Architecture and design
- **[examples/.env.remote](examples/.env.remote)** - Configuration template

### Production Deployment

For production use:

```bash
# Enable TLS
export OPENCODE_REMOTE_TLS="true"

# Use strong tokens with expiration
./bin/remote-token --expires 24h

# Set execution limits
./bin/remote-agent \
  --port 50051 \
  --tokens "$TOKEN" \
  --max-concurrent 10 \
  --timeout 15m
```

See `examples/` directory for Docker and Kubernetes deployment templates.

## Implemented Phases

- ✅ **Phase 0**: Core foundation (event bus, agent interface)
- ✅ **Phase 1**: Session persistence and TUI
- ✅ **Phase 2**: Template system for reusable agent configs
- ✅ **Phase 3**: Pipeline execution for multi-stage workflows
- ✅ **Phase 4**: Remote agent execution with gRPC
- ✅ **Phase 5+**: Agent pools, distributed execution (planned)