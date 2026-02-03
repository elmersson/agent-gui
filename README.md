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

## Project Structure

```
opencode/
├── cmd/opencode/      # Main application entry point
├── pkg/
│   ├── agent/         # Agent interface and types
│   ├── adapter/       # Agent adapter implementations
│   ├── events/        # Event bus for decoupled communication
│   ├── manager/       # Agent lifecycle management
│   ├── session/       # Session persistence
│   └── tui/           # Terminal UI
└── sessions/          # Saved session files
```

## Development

Run tests:

```bash
go test ./...
```

Build:

```bash
go build -o opencode ./cmd/opencode
```