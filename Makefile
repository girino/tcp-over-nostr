# TCP-over-Nostr Makefile
# Builds with embedded version information

BINARY_NAME=tcp-proxy
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS=-ldflags "-X 'main.Version=$(VERSION)' -X 'main.BuildDate=$(BUILD_DATE)' -X 'main.GitCommit=$(GIT_COMMIT)'"

.PHONY: all build clean install test version help

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

# Show help
help:
	@echo "TCP-over-Nostr Build System"
	@echo ""
	@echo "Usage:"
	@echo "  make build      Build the binary with version information"
	@echo "  make build-race Build with race detection for debugging"
	@echo "  make clean      Remove build artifacts"
	@echo "  make install    Install to GOPATH/bin"
	@echo "  make test       Run tests"
	@echo "  make version    Show version information"
	@echo "  make help       Show this help message"
	@echo ""
	@echo "The binary will be built with embedded version information from git tags."
