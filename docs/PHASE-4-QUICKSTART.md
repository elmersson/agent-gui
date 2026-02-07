# Phase 4: Remote Agent Execution - Quick Start Guide

This guide will help you set up and test remote agent execution in under 5 minutes.

## Prerequisites

- Go 1.23+ installed
- `protoc` and Go gRPC plugins installed (or run `make deps`)
- Access to Claude API (for agent execution)

## Quick Start

### Step 1: Generate a Token

```bash
# Generate an authentication token
./bin/remote-token

# Example output:
# Token:      abc123def456...
# Created:    2024-02-07T08:00:00Z
# Expires:    Never
```

Save the token - you'll need it for both server and client.

### Step 2: Start the Remote Server

```bash
# Start the remote agent server on port 50051
./bin/remote-agent --port 50051 --tokens "YOUR_TOKEN_HERE"

# Example:
# ./bin/remote-agent --port 50051 --tokens "abc123def456..."
```

The server will:
- Listen on port 50051
- Accept authenticated gRPC connections
- Execute agents locally using your Claude API key

### Step 3: Configure and Run the Client

```bash
# Set environment variables for remote mode
export OPENCODE_REMOTE_ADDRESS="localhost:50051"
export OPENCODE_REMOTE_TOKEN="YOUR_TOKEN_HERE"
export OPENCODE_REMOTE_TLS="false"  # Use true for production

# Run OpenCode in remote mode
./bin/opencode
```

### Step 4: Test It Out

Once the TUI launches, you'll see a remote status indicator in the header:

```
● Remote    [indicates connected]
```

Try running a simple command:
```
:start
Hello, can you tell me about remote agents?
```

The execution will happen on the remote server, and output will stream back to your client in real-time!

## Connection Status Indicators

The TUI displays the following connection states:

| Symbol | State | Description |
|--------|-------|-------------|
| `●` | Connected | Remote connection active |
| `◌` | Connecting | Establishing connection |
| `◌` | Reconnecting (attempt N) | Attempting to reconnect |
| `○` | Disconnected | Not connected |
| `✗` | Failed | Connection failed |

## Common Issues

### "Connection refused"
- Ensure the remote server is running
- Check the port number matches
- Verify firewall settings

### "Invalid auth token"
- Ensure token matches between client and server
- Check for whitespace in token strings
- Regenerate token if unsure

### "TLS handshake failed"
- Set `OPENCODE_REMOTE_TLS=false` for local testing
- For production, ensure valid TLS certificates

## Advanced Configuration

### Custom Port

```bash
# Server
./bin/remote-agent --port 8080 --tokens "YOUR_TOKEN"

# Client
export OPENCODE_REMOTE_ADDRESS="localhost:8080"
```

### Token Expiration

```bash
# Generate token that expires in 24 hours
./bin/remote-token --expires 24h
```

### Multiple Tokens

```bash
# Server accepts comma-separated tokens
./bin/remote-agent --port 50051 --tokens "token1,token2,token3"
```

### Max Concurrent Executions

```bash
# Limit to 3 concurrent executions
./bin/remote-agent --port 50051 --tokens "YOUR_TOKEN" --max-concurrent 3
```

### Execution Timeout

```bash
# Set 5-minute timeout per execution
./bin/remote-agent --port 50051 --tokens "YOUR_TOKEN" --timeout 5m
```

## Docker Deployment

See `examples/Dockerfile.remote` and `examples/docker-compose.yml` for containerized deployment.

```bash
# Build Docker image
docker build -f examples/Dockerfile.remote -t opencode-remote .

# Run container
docker run -p 50051:50051 \
  -e ANTHROPIC_API_KEY="your-key" \
  opencode-remote --tokens "YOUR_TOKEN"
```

## Production Checklist

Before deploying to production:

- [ ] Enable TLS (`OPENCODE_REMOTE_TLS=true`)
- [ ] Use environment variables for tokens (never hardcode)
- [ ] Set token expiration (`--expires` flag)
- [ ] Configure firewall rules
- [ ] Set up monitoring and logging
- [ ] Implement token rotation
- [ ] Test reconnection behavior
- [ ] Set appropriate execution timeouts
- [ ] Limit concurrent executions
- [ ] Use reverse proxy (nginx/traefik) for TLS termination

## Next Steps

- Read [PHASE-4.md](PHASE-4.md) for architectural details
- See [REMOTE-AGENTS.md](REMOTE-AGENTS.md) for design documentation
- Review `examples/` directory for deployment templates
- Explore Phase 5 for agent pools and load balancing

## Troubleshooting

### Enable Debug Logging

```bash
# Server
./bin/remote-agent --port 50051 --tokens "YOUR_TOKEN" --verbose

# Check logs for connection details
```

### Test Connection Without Client

```bash
# Use grpcurl to test server
grpcurl -plaintext \
  -H "authorization: Bearer YOUR_TOKEN" \
  localhost:50051 \
  remote.AgentService/HealthCheck
```

### Monitor Active Executions

The server logs show:
- Connection events
- Execution start/completion
- Error details
- Token validation failures

## Support

For issues:
1. Check logs on both client and server
2. Verify network connectivity
3. Test with `grpcurl` or similar tool
4. Review [GitHub Issues](https://github.com/rasmuselmersson/opencode/issues)

## Security Best Practices

1. **Never commit tokens** - Use environment variables or secret managers
2. **Enable TLS in production** - Plain TCP is only for development
3. **Rotate tokens regularly** - Set expiration times
4. **Use strong tokens** - Default 32-byte tokens are recommended
5. **Limit network access** - Use firewalls and VPNs
6. **Monitor authentication failures** - Set up alerts for suspicious activity
7. **Secure the server host** - Keep OS and dependencies updated

---

**Ready to scale?** Check out Phase 5 for agent pools and load balancing across multiple remote servers!
