#!/bin/bash

# Script to generate gRPC code from proto files
# This script checks for required tools and provides helpful error messages

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Checking for required tools..."

# Check for protoc
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed"
    echo ""
    echo "Please install protoc:"
    echo "  macOS:   brew install protobuf"
    echo "  Ubuntu:  sudo apt install protobuf-compiler"
    echo "  Other:   https://grpc.io/docs/protoc-installation/"
    exit 1
fi

# Check for protoc-gen-go
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# Check for protoc-gen-go-grpc
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

echo "Generating gRPC code from proto files..."
cd "$PROJECT_ROOT"

protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    pkg/remote/proto/agent.proto

echo "✓ Proto generation complete"
echo "Generated files:"
echo "  - pkg/remote/proto/agent.pb.go"
echo "  - pkg/remote/proto/agent_grpc.pb.go"
