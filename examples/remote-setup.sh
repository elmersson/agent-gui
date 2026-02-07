#!/bin/bash

# Example script for setting up remote agent execution
# This demonstrates a complete remote setup from scratch

set -e

echo "=== OpenCode Remote Agent Setup Example ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Step 1: Build binaries
echo -e "${GREEN}Step 1: Building binaries...${NC}"
go build -o bin/opencode ./cmd/opencode
go build -o bin/opencode-remote ./cmd/remote-agent
go build -o bin/opencode-token ./cmd/remote-token
echo "✓ Binaries built"
echo ""

# Step 2: Generate authentication token
echo -e "${GREEN}Step 2: Generating authentication token...${NC}"
TOKEN=$(go run ./cmd/remote-token --length 32 | grep "Token:" | awk '{print $2}')
echo "✓ Token generated: ${TOKEN:0:20}..."
echo ""

# Step 3: Check for API key
echo -e "${GREEN}Step 3: Checking API key...${NC}"
if [ -z "$OPENCODE_API_KEY" ]; then
    echo -e "${YELLOW}⚠ Warning: OPENCODE_API_KEY not set${NC}"
    echo "The remote server needs an API key to call LLMs."
    echo "Set it with: export OPENCODE_API_KEY='your-key'"
    echo ""
    read -p "Enter API key (or press Enter to skip): " api_key
    if [ -n "$api_key" ]; then
        export OPENCODE_API_KEY="$api_key"
        echo "✓ API key set for this session"
    else
        echo "⚠ Skipping - server will fail to execute agents without API key"
    fi
else
    echo "✓ API key found"
fi
echo ""

# Step 4: Start remote server in background
echo -e "${GREEN}Step 4: Starting remote agent server...${NC}"
./bin/opencode-remote \
    --port 50051 \
    --tokens "$TOKEN" \
    --model "claude-sonnet-4-20250514" \
    --max-concurrent 5 \
    --timeout 10m \
    > remote-server.log 2>&1 &
    
SERVER_PID=$!
echo "✓ Server started (PID: $SERVER_PID)"
echo "  - Listening on port 50051"
echo "  - Logs: remote-server.log"
echo ""

# Wait for server to be ready
echo "Waiting for server to be ready..."
sleep 2

# Check if server is running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "✗ Server failed to start. Check remote-server.log for errors."
    exit 1
fi
echo "✓ Server is running"
echo ""

# Step 5: Configure client environment
echo -e "${GREEN}Step 5: Configuring client environment...${NC}"
export OPENCODE_REMOTE_ADDRESS="localhost:50051"
export OPENCODE_REMOTE_TOKEN="$TOKEN"
echo "✓ Client configured to use remote agent"
echo ""

# Step 6: Display setup summary
echo -e "${GREEN}=== Setup Complete ===${NC}"
echo ""
echo "Server Details:"
echo "  - PID: $SERVER_PID"
echo "  - Address: localhost:50051"
echo "  - Model: claude-sonnet-4-20250514"
echo "  - Max Concurrent: 5"
echo "  - Timeout: 10m"
echo ""
echo "Client Configuration:"
echo "  export OPENCODE_REMOTE_ADDRESS=\"localhost:50051\""
echo "  export OPENCODE_REMOTE_TOKEN=\"$TOKEN\""
echo ""
echo "Usage:"
echo "  # Run the client (will use remote mode automatically)"
echo "  ./bin/opencode"
echo ""
echo "  # To switch back to local mode:"
echo "  unset OPENCODE_REMOTE_ADDRESS"
echo "  unset OPENCODE_REMOTE_TOKEN"
echo ""
echo "  # To stop the server:"
echo "  kill $SERVER_PID"
echo ""
echo -e "${YELLOW}Note: The server is running in the background.${NC}"
echo "To view logs: tail -f remote-server.log"
echo ""

# Optional: Start client if requested
read -p "Start the client now? (y/N): " start_client
if [[ $start_client =~ ^[Yy]$ ]]; then
    echo ""
    echo "Starting client..."
    echo "(Press Ctrl+C to exit)"
    sleep 1
    ./bin/opencode
else
    echo ""
    echo "Setup complete! Start the client when ready with:"
    echo "  ./bin/opencode"
fi

# Cleanup on exit
trap "echo ''; echo 'Stopping server...'; kill $SERVER_PID 2>/dev/null; echo 'Server stopped.'" EXIT
