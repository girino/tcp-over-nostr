# Environment Variable Configuration

TCP-over-Nostr supports configuration via environment variables with the `TON_` prefix. This allows for easy configuration management and deployment.

## Quick Start

1. **Copy the appropriate example file:**
   ```bash
   # For server configuration
   cp env.server.example .env
   
   # For client configuration  
   cp env.client.example .env
   ```

2. **Edit the `.env` file with your values:**
   ```bash
   nano .env
   ```

3. **Load the environment variables and run:**
   ```bash
   # Load environment variables
   source .env
   
   # Run the application
   ./tcp-proxy
   ```

   **Note:** These environment files are designed for use with Docker containers where environment variables are automatically loaded.

## Environment Variables

### Common Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `TON_MODE` | Operation mode | `server` or `client` |
| `TON_RELAY` | Nostr relay URL(s) | `wss://relay.damus.io` or `wss://relay1.io,wss://relay2.io` |
| `TON_PRIVATE_KEY` | Private key (hex or nsec) | `4c2800f5a0a4fb6d09afce6ec470f09f29250abe09e6558029fad0691c857721` |
| `TON_VERBOSE` | Enable verbose logging | `true` or `false` |

### Server Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `TON_TARGET_HOST` | Target host | `192.168.1.100` or `192.168.1.100:22` |
| `TON_TARGET_PORT` | Target port (if not in host) | `22` |

### Client Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `TON_CLIENT_PORT` | Client listening port | `2222` |
| `TON_SERVER_KEY` | Server's public key (hex or npub) | `npub1abc123...` |

## Configuration Methods

### Method 1: Environment File (Recommended for Docker)

```bash
# Create .env file
cp env.server.example .env

# Edit configuration
nano .env

# Load and run
source .env
./tcp-proxy
```

### Method 2: Export Variables

```bash
export TON_MODE=server
export TON_TARGET_HOST=192.168.1.100:22
export TON_RELAY=wss://relay.damus.io
./tcp-proxy
```

### Method 3: Inline Variables

```bash
TON_MODE=server TON_TARGET_HOST=192.168.1.100:22 ./tcp-proxy
```

### Method 4: Docker Environment Files

```bash
# Docker will automatically load .env files
docker run --env-file .env tcp-over-nostr
```

## Precedence Order

1. **Command line flags** (highest priority)
2. **Environment variables** (fallback)
3. **Default values** (lowest priority)

Example:
```bash
# Environment variable sets default
export TON_TARGET_HOST=192.168.1.100:22

# Command line flag overrides environment variable
./tcp-proxy -target-host localhost:80
# Result: Uses localhost:80 (command line wins)
```

## Common Use Cases

### SSH Proxy Setup

**Server (.env):**
```bash
TON_MODE=server
TON_TARGET_HOST=192.168.1.100:22
TON_RELAY=wss://relay.damus.io
TON_VERBOSE=true
```

**Client (.env):**
```bash
TON_MODE=client
TON_CLIENT_PORT=2222
TON_SERVER_KEY=npub1abc123...
TON_RELAY=wss://relay.damus.io
TON_VERBOSE=true
```

**Usage:**
```bash
# On server
source .env && ./tcp-proxy

# On client  
source .env && ./tcp-proxy

# Connect via SSH
ssh -p 2222 user@localhost
```

### HTTP Proxy Setup

**Server (.env):**
```bash
TON_MODE=server
TON_TARGET_HOST=httpbin.org:80
TON_RELAY=wss://relay.damus.io,wss://relay.snort.social
TON_VERBOSE=false
```

**Client (.env):**
```bash
TON_MODE=client
TON_CLIENT_PORT=8080
TON_SERVER_KEY=npub1abc123...
TON_RELAY=wss://relay.damus.io,wss://relay.snort.social
TON_VERBOSE=false
```

**Usage:**
```bash
# On server
source .env && ./tcp-proxy

# On client
source .env && ./tcp-proxy

# Test HTTP connection
curl http://localhost:8080
```

## Security Notes

- **Private Keys**: Store private keys securely and never commit them to version control
- **Environment Files**: Add `.env` to `.gitignore` to prevent accidental commits
- **Relay Selection**: Choose reliable, trusted Nostr relays for production use
- **Network Security**: Consider the security implications of tunneling traffic over Nostr

## Troubleshooting

### Environment Variables Not Working

1. **Check variable names**: Must use `TON_` prefix and uppercase
2. **Check syntax**: No spaces around `=` in `.env` files
3. **Load variables**: Use `source .env` to load the file
4. **Verify precedence**: Command line flags override environment variables

### Common Issues

- **Wrong mode**: Ensure `TON_MODE` is set to `server` or `client`
- **Missing server key**: Client requires `TON_SERVER_KEY` from server output
- **Invalid relay**: Check relay URLs are accessible and valid
- **Port conflicts**: Ensure ports are not already in use

### Debug Mode

Enable verbose logging to troubleshoot:
```bash
export TON_VERBOSE=true
./tcp-proxy
```
