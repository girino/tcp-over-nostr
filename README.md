# TCP over Nostr

A Go implementation for tunneling TCP connections over the Nostr protocol.

## Description

This project implements a system that allows TCP connections to be tunneled through Nostr relays, enabling communication through decentralized infrastructure.

## Current Version: v1.0.0 - Basic TCP Proxy

The first version implements a simple TCP proxy that listens on a specified port and forwards connections to a target host and port.

### Usage

```bash
# Build the proxy
go build -o tcp-proxy

# Basic usage - proxy port 8080 to localhost:80
./tcp-proxy

# Custom configuration
./tcp-proxy -listen 9000 -target-host example.com -target-port 443 -verbose

# Show help
./tcp-proxy -h
```

### Command Line Options

- `-listen int`: Port to listen on (default 8080)
- `-target-host string`: Target host to proxy to (default "localhost")  
- `-target-port int`: Target port to proxy to (default 80)
- `-verbose`: Enable verbose logging

### Example

```bash
# Proxy local port 8080 to Google's HTTP server
./tcp-proxy -listen 8080 -target-host google.com -target-port 80 -verbose

# Test the proxy
curl http://localhost:8080
```

## Features

- ✅ Simple TCP proxy with configurable ports
- ✅ Command line interface with flags
- ✅ Bidirectional data forwarding
- ✅ Verbose logging option
- ✅ Clean connection handling
- 🚧 TCP connection tunneling over Nostr protocol (planned)
- 🚧 Decentralized communication infrastructure (planned)

## Development

This project uses semantic versioning and all working versions are tagged in git.

## License

TBD
