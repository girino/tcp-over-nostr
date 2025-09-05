# Multi-stage build for TCP-over-Nostr
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy git directory for commit hash (useful for debugging and support)
COPY .git .git

# Copy source code
COPY . .

# Build the application
RUN make build

# Final stage - minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S tcpnostr && \
    adduser -u 1001 -S tcpnostr -G tcpnostr

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/tcp-proxy /app/tcp-proxy

# Change ownership to non-root user
RUN chown -R tcpnostr:tcpnostr /app

# Switch to non-root user
USER tcpnostr

# Expose common ports (will be overridden by environment variables)
EXPOSE 8080 2222 80 443

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep tcp-proxy || exit 1

# Default command
CMD ["./tcp-proxy"]
