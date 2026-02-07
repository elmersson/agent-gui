.PHONY: proto clean test build

# Generate gRPC code from proto files
proto:
	@echo "Generating gRPC code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		pkg/remote/proto/agent.proto

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	rm -f pkg/remote/proto/*.pb.go

# Run tests
test:
	go test -v ./...

# Build the main binary
build:
	go build -o bin/opencode ./cmd/opencode

# Build the remote agent server
build-remote:
	go build -o bin/opencode-remote ./cmd/remote-agent

# Install dependencies
deps:
	go mod download
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Run all (generate proto, test, build)
all: proto test build build-remote
