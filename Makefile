# TCP-over-Nostr Makefile
# Builds with embedded version information

BINARY_NAME=tcp-proxy
VERSION=$(shell grep 'Version.*=' version.go | sed 's/.*Version.*=.*"\(.*\)".*/\1/')
BUILD_DATE=$(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS=-ldflags "-X 'main.Version=$(VERSION)' -X 'main.BuildDate=$(BUILD_DATE)' -X 'main.GitCommit=$(GIT_COMMIT)'"

.PHONY: all build clean install test version help docker-build docker-run-server docker-run-client docker-compose-up docker-compose-down

all: build

# Build the binary with version information
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@rm -f $(BINARY_NAME)
	go build $(LDFLAGS) -o $(BINARY_NAME)
	@echo "Build complete: $(BINARY_NAME)"

# Build with race detection for debugging
build-race:
	@echo "Building $(BINARY_NAME) $(VERSION) with race detection..."
	@rm -f $(BINARY_NAME)
	go build -race $(LDFLAGS) -o $(BINARY_NAME)
	@echo "Build complete: $(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@echo "Clean complete"

# Install to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME) $(VERSION)..."
	go install $(LDFLAGS)
	@echo "Install complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Show version information that will be embedded
version:
	@echo "Version: $(VERSION)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Git Commit: $(GIT_COMMIT)"

# Docker operations
docker-build:
	@echo "Building Docker image..."
	docker build -t tcp-over-nostr:$(VERSION) .
	docker tag tcp-over-nostr:$(VERSION) tcp-over-nostr:latest
	@echo "Docker build complete: tcp-over-nostr:$(VERSION)"

docker-run-server:
	@echo "Running TCP-over-Nostr server in Docker..."
	@if [ ! -f .env ]; then echo "Error: .env file not found. Copy env.server.example to .env first."; exit 1; fi
	docker run -d --name tcp-over-nostr-server --env-file .env -p 8080:8080 -p 2222:2222 tcp-over-nostr:latest
	@echo "Server started. Check logs with: docker logs tcp-over-nostr-server"

docker-run-client:
	@echo "Running TCP-over-Nostr client in Docker..."
	@if [ ! -f .env ]; then echo "Error: .env file not found. Copy env.client.example to .env first."; exit 1; fi
	docker run -d --name tcp-over-nostr-client --env-file .env -p 8081:8080 -p 2223:2222 tcp-over-nostr:latest
	@echo "Client started. Check logs with: docker logs tcp-over-nostr-client"

docker-compose-up:
	@echo "Starting services with Docker Compose..."
	docker compose up -d
	@echo "Services started. Check logs with: docker compose logs -f"

docker-compose-down:
	@echo "Stopping services with Docker Compose..."
	docker compose down
	@echo "Services stopped."

# Show help
help:
	@echo "TCP-over-Nostr Build System"
	@echo ""
	@echo "Usage:"
	@echo "  make build              Build the binary with version information"
	@echo "  make build-race         Build with race detection for debugging"
	@echo "  make clean              Remove build artifacts"
	@echo "  make install            Install to GOPATH/bin"
	@echo "  make test               Run tests"
	@echo "  make version            Show version information"
	@echo "  make docker-build       Build Docker image"
	@echo "  make docker-run-server  Run server in Docker container"
	@echo "  make docker-run-client  Run client in Docker container"
	@echo "  make docker-compose-up  Start services with Docker Compose"
	@echo "  make docker-compose-down Stop services with Docker Compose"
	@echo "  make help               Show this help message"
	@echo ""
	@echo "The binary will be built with embedded version information from git tags."
	@echo ""
	@echo "Docker Usage:"
	@echo "  1. Copy env.server.example to .env (for server) or env.client.example to .env (for client)"
	@echo "  2. Edit .env with your configuration"
	@echo "  3. Run: make docker-build && make docker-run-server (or docker-run-client)"
	@echo "  4. Or use: make docker-compose-up (requires .env.server and .env.client files)"
