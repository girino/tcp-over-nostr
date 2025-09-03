# Docker Deployment Guide

This guide explains how to deploy TCP-over-Nostr using Docker and Docker Compose.

## Quick Start

### Option 1: Server Only

```bash
# 1. Copy server environment template
cp env.server.example .env

# 2. Edit configuration
nano .env

# 3. Run server
docker compose -f docker-compose.server.yml up -d

# 4. Check logs
docker compose -f docker-compose.server.yml logs -f
```

### Option 2: Client Only

```bash
# 1. Copy client environment template
cp env.client.example .env

# 2. Edit configuration (set TON_SERVER_KEY from server output)
nano .env

# 3. Run client
docker compose -f docker-compose.client.yml up -d

# 4. Check logs
docker compose -f docker-compose.client.yml logs -f
```

### Option 3: Both Server and Client

```bash
# 1. Copy environment templates
cp env.server.example .env.server
cp env.client.example .env.client

# 2. Edit configurations
nano .env.server
nano .env.client

# 3. Run both
docker compose up -d

# 4. Check logs
docker compose logs -f
```

## Docker Files

### Dockerfile

- **Multi-stage build** for optimized image size
- **Alpine Linux** base for security and minimal footprint
- **Non-root user** for security
- **Health checks** for monitoring
- **Example environment files** included

### Docker Compose Files

#### `docker-compose.server.yml`
- Server-only configuration
- Uses `.env` file for configuration
- Exposes common server ports

#### `docker-compose.client.yml`
- Client-only configuration
- Uses `.env` file for configuration
- Exposes common client ports

#### `docker-compose.yml`
- Combined server and client configuration
- Uses `.env.server` and `.env.client` files
- Different port mappings to avoid conflicts

## Configuration

### Environment Files

Create environment files by copying the examples:

```bash
# Server configuration
cp env.server.example .env.server

# Client configuration
cp env.client.example .env.client
```

### Key Configuration Variables

#### Server Configuration (`.env.server`)
```bash
TON_MODE=server
TON_TARGET_HOST=192.168.1.100:22
TON_RELAY=wss://relay.damus.io
TON_VERBOSE=true
```

#### Client Configuration (`.env.client`)
```bash
TON_MODE=client
TON_CLIENT_PORT=2222
TON_SERVER_KEY=npub1abc123...
TON_RELAY=wss://relay.damus.io
TON_VERBOSE=true
```

## Port Mappings

### Server Ports
- `8080:8080` - Default client port
- `2222:2222` - Common SSH proxy port
- `80:80` - HTTP port
- `443:443` - HTTPS port

### Client Ports
- `8081:8080` - Client port (different from server)
- `2223:2222` - SSH proxy port (different from server)
- `3000:3000` - Alternative client port
- `5432:5432` - Database proxy port

## Common Use Cases

### SSH Proxy Setup

#### Server (`.env.server`)
```bash
TON_MODE=server
TON_TARGET_HOST=192.168.1.100:22
TON_RELAY=wss://relay.damus.io
TON_VERBOSE=true
```

#### Client (`.env.client`)
```bash
TON_MODE=client
TON_CLIENT_PORT=2222
TON_SERVER_KEY=npub1abc123...
TON_RELAY=wss://relay.damus.io
TON_VERBOSE=true
```

#### Usage
```bash
# Start server
docker compose -f docker-compose.server.yml up -d

# Start client
docker compose -f docker-compose.client.yml up -d

# Connect via SSH
ssh -p 2222 user@localhost
```

### HTTP Proxy Setup

#### Server (`.env.server`)
```bash
TON_MODE=server
TON_TARGET_HOST=httpbin.org:80
TON_RELAY=wss://relay.damus.io,wss://relay.primal.net
TON_VERBOSE=false
```

#### Client (`.env.client`)
```bash
TON_MODE=client
TON_CLIENT_PORT=8080
TON_SERVER_KEY=npub1abc123...
TON_RELAY=wss://relay.damus.io,wss://relay.primal.net
TON_VERBOSE=false
```

#### Usage
```bash
# Start both
docker compose up -d

# Test HTTP connection
curl http://localhost:8081
```

## Docker Commands

### Build and Run

```bash
# Build image
docker build -t tcp-over-nostr .

# Run server
docker run -d --name tcp-server --env-file .env.server tcp-over-nostr

# Run client
docker run -d --name tcp-client --env-file .env.client tcp-over-nostr
```

### Docker Compose

```bash
# Start services
docker compose up -d

# Start specific service
docker compose up -d tcp-over-nostr-server

# View logs
docker compose logs -f

# View logs for specific service
docker compose logs -f tcp-over-nostr-server

# Stop services
docker compose down

# Stop and remove volumes
docker compose down -v
```

### Monitoring

```bash
# Check container status
docker compose ps

# Check resource usage
docker stats

# Check logs
docker compose logs -f

# Execute commands in container
docker compose exec tcp-over-nostr-server sh
```

## Security Considerations

### Container Security
- **Non-root user** - Container runs as user `tcpnostr`
- **No new privileges** - Prevents privilege escalation
- **Resource limits** - Memory and CPU limits set
- **Health checks** - Automatic health monitoring

### Network Security
- **Port exposure** - Only necessary ports are exposed
- **Environment variables** - Sensitive data in environment files
- **Logging** - Log rotation configured

### Best Practices
- Use `.env` files for configuration
- Keep private keys secure
- Use trusted Nostr relays
- Monitor container logs
- Regular security updates

## Troubleshooting

### Common Issues

#### Container Won't Start
```bash
# Check logs
docker compose logs tcp-over-nostr-server

# Check environment variables
docker compose exec tcp-over-nostr-server env | grep TON_
```

#### Connection Issues
```bash
# Check if ports are exposed
docker compose ps

# Check network connectivity
docker compose exec tcp-over-nostr-server ping relay.damus.io
```

#### Environment Variables Not Working
```bash
# Verify .env file exists
ls -la .env*

# Check file contents
cat .env.server

# Test environment loading
docker compose exec tcp-over-nostr-server env | grep TON_
```

### Debug Mode

Enable verbose logging for troubleshooting:

```bash
# In .env file
TON_VERBOSE=true

# Restart container
docker compose restart tcp-over-nostr-server
```

### Health Checks

```bash
# Check health status
docker compose ps

# Manual health check
docker compose exec tcp-over-nostr-server pgrep tcp-proxy
```

## Production Deployment

### Environment Setup
1. Create production environment files
2. Set up proper Nostr relay infrastructure
3. Configure monitoring and logging
4. Set up backup and recovery procedures

### Scaling
- Use Docker Swarm or Kubernetes for orchestration
- Implement load balancing for multiple instances
- Set up monitoring and alerting

### Maintenance
- Regular security updates
- Monitor resource usage
- Rotate logs regularly
- Backup configuration files
