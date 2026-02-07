# Remote Agent Execution - User Guide

Phase 4 introduces the ability to run agents remotely over gRPC, enabling distributed execution, load balancing, and horizontal scaling of agent workloads.

## Overview

Remote agents behave identically to local agents from the user's perspective, but execute on a separate server. This allows you to:

- **Distribute load**: Run multiple agents across different machines
- **Centralize resources**: Keep expensive GPU/CPU resources on dedicated servers
- **Scale horizontally**: Add more agent servers as demand grows
- **Isolate execution**: Separate agent execution from the client UI

## Architecture

```
┌─────────────┐           gRPC            ┌──────────────────┐
│   Client    │   ◄───────────────────►   │  Remote Server   │
│  (opencode) │     (bidirectional)       │ (opencode-remote)│
│             │                            │                  │
│ RemoteAdapter                            │  LocalAgent      │
│  - Handles streaming                     │  - Executes code │
│  - Reconnection                          │  - LLM calls     │
│  - Auth tokens                           │  - Token tracking│
└─────────────┘                            └──────────────────┘
```

## Quick Start

### 1. Generate an Authentication Token

```bash
# Generate a secure token
go run ./cmd/remote-token --length 32

# Output:
# Token:      dGVzdC10b2tlbi0xMjM0NTY3ODkwMTIzNDU2
# Created:    2026-02-07T10:30:00Z
# Expires:    Never
```

**Important**: Store this token securely. Never commit it to version control.

### 2. Start the Remote Agent Server

```bash
# Set your API key (required for agents to call LLMs)
export OPENCODE_API_KEY="your-api-key-here"

# Start the server
go run ./cmd/remote-agent \
  --port 50051 \
  --tokens "dGVzdC10b2tlbi0xMjM0NTY3ODkwMTIzNDU2" \
  --model "claude-sonnet-4-20250514" \
  --max-concurrent 10 \
  --timeout 30m

# Output:
# Starting remote agent server on port 50051
# Using model: claude-sonnet-4-20250514
# Max concurrent executions: 10
# Execution timeout: 30m0s
```

**Server Options:**
- `--port`: Port to listen on (default: 50051)
- `--tokens`: Comma-separated list of valid auth tokens (required)
- `--apikey`: API key for agents (or use `OPENCODE_API_KEY` env var)
- `--model`: Default model for agents (default: claude-sonnet-4-20250514)
- `--max-concurrent`: Max simultaneous executions (0 = unlimited)
- `--timeout`: Execution timeout (0 = no timeout)

### 3. Configure the Client for Remote Mode

Set environment variables to enable remote execution:

```bash
# Required: Remote server address
export OPENCODE_REMOTE_ADDRESS="localhost:50051"

# Required: Authentication token
export OPENCODE_REMOTE_TOKEN="dGVzdC10b2tlbi0xMjM0NTY3ODkwMTIzNDU2"

# Optional: Enable TLS (recommended for production)
export OPENCODE_REMOTE_TLS="false"
```

### 4. Run the Client

```bash
# Start opencode as normal - it will automatically use remote mode
go run ./cmd/opencode
```

You'll see a remote connection status indicator in the TUI:

```
┌─────────────────────────────────────────────────────────────┐
│  opencode  RUNNING  model: claude-sonnet-4-20250514  ● Remote
└─────────────────────────────────────────────────────────────┘
```

## Connection States

The TUI displays the remote connection status:

- **● Remote** (green) - Connected and healthy
- **◌ Connecting...** (yellow) - Establishing connection
- **◌ Reconnecting (attempt N)...** (yellow) - Attempting to restore connection
- **○ Disconnected** (gray) - No connection
- **✗ Connection Failed** (red) - Connection permanently failed

## Token Management

### Generating Tokens

```bash
# Generate a single token
go run ./cmd/remote-token

# Generate multiple tokens
go run ./cmd/remote-token --count 5

# Generate with expiration
go run ./cmd/remote-token --expires 24h

# Generate for a specific user
go run ./cmd/remote-token --user "alice@example.com"

# Show SHA256 hash (for secure storage)
go run ./cmd/remote-token --hash
```

### Token Rotation

To rotate tokens without downtime:

1. Generate a new token
2. Add it to the server's `--tokens` list (comma-separated)
3. Update clients to use the new token
4. Remove the old token from the server after all clients have switched

```bash
# Server with multiple tokens
go run ./cmd/remote-agent \
  --tokens "old-token,new-token" \
  --port 50051
```

## Security Best Practices

### Development
- Use generated tokens (don't make up simple tokens)
- Store tokens in environment variables
- Use `.env` files (but add to `.gitignore`)

### Production
- **Always use TLS** (`OPENCODE_REMOTE_TLS=true`)
- Use a secret manager (AWS Secrets Manager, HashiCorp Vault, etc.)
- Rotate tokens regularly (monthly recommended)
- Use short expiration times for tokens
- Monitor authentication failures
- Use firewall rules to restrict server access
- Run the server behind a reverse proxy (nginx, Caddy)

### TLS Setup (Production)

Generate TLS certificates:

```bash
# Self-signed for development
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

# For production, use Let's Encrypt or your organization's CA
```

Start server with TLS:

```bash
go run ./cmd/remote-agent \
  --port 50051 \
  --tokens "..." \
  --tls-cert cert.pem \
  --tls-key key.pem
```

Client configuration:

```bash
export OPENCODE_REMOTE_TLS="true"
export OPENCODE_REMOTE_CERT="cert.pem"  # If using self-signed
```

## Error Handling

### Connection Failures

The client automatically handles:
- **Initial connection failures**: Retries with exponential backoff
- **Mid-execution disconnects**: Buffers output and reconnects
- **Server crashes**: Attempts to reconnect up to 5 times
- **Network timeouts**: Detects stale connections via heartbeat

### Manual Reconnection

If connection fails persistently:

1. Check server is running: `curl http://localhost:50051` (should refuse connection, not timeout)
2. Verify token is correct
3. Check firewall rules
4. Review server logs for authentication errors

## Advanced Configuration

### Load Balancing

For multiple remote servers, use a load balancer (nginx, HAProxy, or cloud load balancer):

```nginx
upstream agents {
    server agent1.example.com:50051;
    server agent2.example.com:50051;
    server agent3.example.com:50051;
}

server {
    listen 50051 http2;
    location / {
        grpc_pass grpc://agents;
    }
}
```

Client configuration:

```bash
export OPENCODE_REMOTE_ADDRESS="loadbalancer.example.com:50051"
```

### Docker Deployment

Server:

```dockerfile
FROM golang:1.25 AS builder
WORKDIR /app
COPY . .
RUN go build -o opencode-remote ./cmd/remote-agent

FROM debian:bookworm-slim
COPY --from=builder /app/opencode-remote /usr/local/bin/
ENV OPENCODE_API_KEY=""
EXPOSE 50051
CMD ["opencode-remote", "--port", "50051", "--tokens", "${AUTH_TOKENS}"]
```

Run:

```bash
docker run -d \
  -e OPENCODE_API_KEY="your-key" \
  -e AUTH_TOKENS="your-token" \
  -p 50051:50051 \
  opencode-remote
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: opencode-remote
spec:
  replicas: 3
  selector:
    matchLabels:
      app: opencode-remote
  template:
    metadata:
      labels:
        app: opencode-remote
    spec:
      containers:
      - name: opencode-remote
        image: opencode-remote:latest
        ports:
        - containerPort: 50051
        env:
        - name: OPENCODE_API_KEY
          valueFrom:
            secretKeyRef:
              name: opencode-secrets
              key: api-key
        - name: AUTH_TOKENS
          valueFrom:
            secretKeyRef:
              name: opencode-secrets
              key: auth-tokens
---
apiVersion: v1
kind: Service
metadata:
  name: opencode-remote
spec:
  type: LoadBalancer
  ports:
  - port: 50051
    targetPort: 50051
    protocol: TCP
  selector:
    app: opencode-remote
```

## Troubleshooting

### "Failed to connect to remote agent"

**Cause**: Server not reachable or not running

**Solution**:
```bash
# Verify server is running
netstat -an | grep 50051

# Test connectivity
telnet localhost 50051

# Check server logs
```

### "Invalid auth token"

**Cause**: Token mismatch or expired

**Solution**:
```bash
# Verify token matches server
echo $OPENCODE_REMOTE_TOKEN

# Generate new token if needed
go run ./cmd/remote-token

# Update both server and client
```

### "Connection keeps reconnecting"

**Cause**: Network instability or server overload

**Solution**:
```bash
# Check server resource usage
top  # or htop

# Increase max concurrent executions
go run ./cmd/remote-agent --max-concurrent 20

# Check network latency
ping remote-server
```

### "Execution timeout"

**Cause**: Long-running agent exceeded timeout

**Solution**:
```bash
# Increase timeout on server
go run ./cmd/remote-agent --timeout 60m

# Or disable timeout
go run ./cmd/remote-agent --timeout 0
```

## Monitoring

### Server Health

Check if server is healthy:

```bash
# Health check endpoint (requires proto generation)
grpcurl -plaintext localhost:50051 remote.AgentService/HealthCheck
```

### Metrics to Monitor

1. **Active Connections**: Number of concurrent client connections
2. **Execution Count**: Total agent executions over time
3. **Error Rate**: Failed authentications, timeouts, crashes
4. **Latency**: Time from request to first output chunk
5. **Resource Usage**: CPU, memory, network bandwidth

## Migration Guide

### From Local to Remote

1. **Start remote server** on a dedicated machine
2. **Generate token** and distribute securely
3. **Set environment variables** on clients
4. **Test with one client** before rolling out
5. **Monitor** for performance/stability
6. **Scale** by adding more servers if needed

### From Remote to Local

Simply unset the environment variables:

```bash
unset OPENCODE_REMOTE_ADDRESS
unset OPENCODE_REMOTE_TOKEN
unset OPENCODE_REMOTE_TLS
```

The client will automatically fall back to local mode.

## Limitations

Current limitations (to be addressed in future phases):

- No automatic load balancing (requires external load balancer)
- No built-in request queuing (use `--max-concurrent`)
- No execution state persistence (server restart loses active executions)
- No multi-region support (single server endpoint per client)

## Next Steps

- **Phase 5**: Agent pools with automatic load balancing
- **Phase 6**: Distributed execution across multiple regions
- **Phase 7**: Persistent execution state with resumability

## Support

For issues or questions:

1. Check this documentation
2. Review server logs
3. Test with local mode to isolate issues
4. Open an issue on GitHub with detailed logs
