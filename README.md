# TCP-over-Nostr

[![License](https://img.shields.io/badge/license-Girino%20License-blue.svg)](LICENSE.md)
[![Release](https://img.shields.io/github/v/release/girino/tcp-over-nostr?include_prereleases)](https://github.com/girino/tcp-over-nostr/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/girino/tcp-over-nostr)](go.mod)

**Decentralized TCP Proxy over Nostr Protocol**

TCP-over-Nostr enables secure, decentralized TCP tunneling using the Nostr protocol. Route any TCP traffic through Nostr relays, creating censorship-resistant network tunnels without traditional VPN infrastructure.

```
┌─────────────────────────────────────────────────────────────┐
│                   TCP-over-Nostr v1.1.0                    │
│                                                             │
│  Decentralized TCP Proxy over Nostr Protocol               │
│  Author: Girino Vey                                        │
│  Copyright © 2025                                           │
│  License: https://license.girino.org                       │
└─────────────────────────────────────────────────────────────┘
```

## 🔐 **SECURITY FEATURES (v1.1.0+)**

✅ **END-TO-END ENCRYPTION**: All TCP traffic is now encrypted using NIP-59 Gift Wrap with NIP-44 encryption before transmission through Nostr events.

✅ **EPHEMERAL EVENTS**: Uses ephemeral event kinds (20013, 21059) that are not stored permanently by relays.

✅ **ONE-TIME KEYS**: Each message uses unique one-time keypairs to prevent correlation attacks.

## 🚨 **IMPORTANT SECURITY WARNINGS**

⚠️ **PUBLIC RELAY RISKS**: When using public Nostr relays:
- **Rate limiting** may throttle your connection
- **Event size limits** may fragment large packets  
- **Metadata visibility** - session IDs and packet counts are visible
- **Potential logging** by relay operators

⚠️ **PRODUCTION USAGE**: This software was "vibecoded" to v1.1.0. See [Development Notes](#development-notes) for important implications.

## 📋 **Table of Contents**

- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Usage](#usage)
- [Local Development Setup](#local-development-setup)
- [Configuration](#configuration)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)
- [Development Notes](#development-notes)
- [TODO / Future Development](#todo--future-development)
- [Acknowledgments](#acknowledgments)
- [License](#license)

## ✨ **Features**

- 🌐 **Decentralized**: No central servers - uses Nostr relay network
- 🔧 **Universal**: Tunnel any TCP service (SSH, HTTP, databases, etc.)
- 🚀 **Real-time**: Low-latency packet delivery via WebSocket relays
- 🔐 **End-to-End Encrypted**: NIP-59 Gift Wrap with NIP-44 encryption
- 🔑 **Cryptographically Signed**: All events signed with Ed25519 keys
- 📦 **Ephemeral Events**: Uses kinds 20013/21059 for automatic cleanup
- 🎲 **One-Time Keys**: Unique keypairs prevent correlation attacks
- 🎯 **Packet Ordering**: Handles out-of-order delivery automatically
- 🔍 **Session Management**: Multiple concurrent connections supported
- 📊 **Verbose Logging**: Detailed debugging and monitoring

## 🚀 **Quick Start**

### 1. Download and Build
```bash
git clone https://github.com/girino/tcp-over-nostr.git
cd tcp-over-nostr
make build
```

### 2. Start Local Relay (Recommended for Testing)
```bash
# Install nak tool by fiatjaf
go install github.com/fiatjaf/nak@latest

# Start local relay
nak serve
# Relay will run on ws://localhost:10547
```

### 3. Start Server (on target machine)
```bash
./tcp-proxy -mode server -target-host httpbin.org -target-port 80
# Note the server's public key from output
```

### 4. Start Client (on local machine)
```bash
./tcp-proxy -mode client -server-key <server_pubkey> -client-port 8080
# Connect via: curl http://localhost:8080
```

## 📦 **Installation**

### From Source
```bash
git clone https://github.com/girino/tcp-over-nostr.git
cd tcp-over-nostr
make build
```

### Build Requirements
- Go 1.21+ 
- Git (for version embedding)
- Make (optional but recommended)

### Binary Installation
```bash
# Build for current platform
make build

# Build with race detection (debugging)
make build-race

# Install to GOPATH/bin
make install

# Show embedded version info
./tcp-proxy --version
```

## 🔧 **Usage**

### Basic Syntax
```bash
tcp-proxy -mode <client|server> [options]
```

### Server Mode
```bash
# Basic server
tcp-proxy -mode server -target-host example.com -target-port 80

# With custom relay
tcp-proxy -mode server -target-host 192.168.1.100 -target-port 22 \
  -relay wss://relay.damus.io

# With verbose logging
tcp-proxy -mode server -target-host localhost -target-port 3306 \
  -verbose -keys-file mysql-server-keys.json
```

### Client Mode  
```bash
# Basic client (requires server's public key)
tcp-proxy -mode client -server-key <server_pubkey> -client-port 8080

# SSH tunnel example
tcp-proxy -mode client -server-key <server_pubkey> -client-port 2222 \
  -relay wss://relay.damus.io
ssh -p 2222 user@localhost

# Custom configuration
tcp-proxy -mode client -server-key <server_pubkey> -client-port 5432 \
  -keys-file postgres-client-keys.json -verbose
```

### Available Options
```
Nostr Options:
  -relay string        Nostr relay URL (default "ws://localhost:10547")
  -keys-file string    File to store key pair (auto-generated)
  -server-key string   Server's public key (required for client)

Client Options:
  -client-port int     Local port to listen on (default 8080)
  
Server Options:
  -target-host string  Target host to proxy to (default "localhost")
  -target-port int     Target port to proxy to (default 80)

General Options:
  -verbose            Enable verbose logging
  -version            Show version information
```

## 🏠 **Local Development Setup**

For testing and development, use a local Nostr relay:

### 1. Install nak tool
```bash
go install github.com/fiatjaf/nak@latest
```
*nak is an excellent Nostr toolkit by [fiatjaf](https://github.com/fiatjaf/nak)*

### 2. Start local relay
```bash
nak serve
# Listening on :10547 (WebSocket), :10548 (HTTP)
```

### 3. Test with HTTP service
```bash
# Terminal 1: Start server
./tcp-proxy -mode server -target-host httpbin.org -target-port 80

# Terminal 2: Start client (use server's pubkey from Terminal 1)
./tcp-proxy -mode client -server-key <server_pubkey> -client-port 8080

# Terminal 3: Test connection
curl http://localhost:8080/get
```

### 4. Test with SSH tunnel
```bash
# Terminal 1: Start server pointing to SSH host
./tcp-proxy -mode server -target-host 192.168.1.100 -target-port 22

# Terminal 2: Start client
./tcp-proxy -mode client -server-key <server_pubkey> -client-port 2222

# Terminal 3: Connect via SSH
ssh -p 2222 user@localhost
```

## ⚙️ **Configuration**

### Key Management
- Keys are automatically generated and stored in JSON files
- Default locations: `client-keys.json`, `server-keys.json`
- Keys are Ed25519 keypairs for Nostr event signing
- Reuse keys across sessions for consistent identity

### Relay Selection
```bash
# Local development (recommended)
-relay ws://localhost:10547

# Public relays (with limitations)
-relay wss://relay.damus.io
-relay wss://nos.lol
-relay wss://relay.nostr.band
```

### Performance Tuning
```bash
# Enable verbose logging for debugging
-verbose

# Use dedicated key files for multiple services
-keys-file service-specific-keys.json
```

## 🔐 **Encryption Implementation (v1.1.0+)**

TCP-over-Nostr v1.1.0 implements **NIP-59 Gift Wrap** encryption for secure transmission:

### Encryption Flow
1. **TCP Data** → **Rumor** (kind 20547, unsigned, contains raw data)
2. **Rumor** → **Seal** (kind 20013, encrypted with sender↔recipient key)
3. **Seal** → **Gift Wrap** (kind 21059, encrypted with one-time↔recipient key)
4. **Gift Wrap** → **Relay** → **Recipient**
5. **Recipient** unwraps: Gift Wrap → Seal → Rumor → TCP Data

### Security Features
- **NIP-44 Encryption**: Uses `secp256k1 ECDH, HKDF, ChaCha20, HMAC-SHA256`
- **One-Time Keys**: Each gift wrap uses unique ephemeral keypairs
- **Ephemeral Events**: Kinds 20013/21059 are not stored permanently by relays
- **HMAC Validation**: Ensures message integrity and authenticity
- **Forward Secrecy**: One-time keys prevent correlation attacks

### Compatibility
- **Requires**: NIP-44 and NIP-59 compatible relays
- **Breaking Change**: Events now use kind 21059 instead of 20547
- **Backward Incompatible**: v1.1.0+ cannot communicate with v1.0.x

## 🔒 **Security Considerations**

### ⚠️ **Critical Security Warnings**

1. **NO ENCRYPTION**: TCP traffic is transmitted in plaintext through Nostr events
   - Always use TLS/SSL for HTTP traffic
   - Use SSH for terminal access
   - Never transmit sensitive data without additional encryption

2. **Public Relay Risks**:
   - Your traffic metadata is visible to relay operators
   - Events may be logged or stored indefinitely
   - Rate limiting can disrupt connections
   - Event size limits may affect large transfers

3. **Packet Visibility**: 
   - Nostr events contain your TCP packet data
   - Anyone can subscribe to your events if they know your pubkey
   - Use firewall rules and access controls on target services

### 🛡️ **Security Best Practices**

- **Use local relays** for sensitive development work
- **Layer encryption** - never rely on Nostr alone for security  
- **Monitor logs** for unusual connection patterns
- **Rotate keys** periodically for long-term deployments
- **Validate target services** - ensure they use proper encryption

### 🔍 **Privacy Considerations**

- Server and client public keys are visible in events
- Connection timing and packet sizes leak traffic patterns  
- Relay operators can potentially correlate sessions
- Consider using Tor or VPN for additional privacy layers

## 🚨 **Troubleshooting**

### Common Issues

**Connection Refused**
```bash
# Check if relay is running
curl -I ws://localhost:10547
# For nak: nak serve should show "Listening on :10547"
```

**Events Not Received**
```bash
# Verify server pubkey is correct
./tcp-proxy -mode server [...] # Shows pubkey in output

# Check relay connectivity with verbose logging
./tcp-proxy -mode client -verbose [...]
```

**Rate Limiting on Public Relays**
```bash
# Switch to local relay
nak serve
./tcp-proxy -relay ws://localhost:10547 [...]

# Or try different public relay
./tcp-proxy -relay wss://nos.lol [...]
```

**Large File Transfers Fail**
```bash
# Public relays may have event size limits
# Use local relay for large transfers
# Consider breaking large transfers into smaller chunks
```

### Debug Mode
```bash
# Enable verbose logging on both sides
./tcp-proxy -mode server -verbose [...]
./tcp-proxy -mode client -verbose [...]

# Build with race detection for development
make build-race
```

### Log Analysis
```bash
# Server logs show:
# - New session establishment  
# - Packet sequence numbers
# - Target connection status

# Client logs show:
# - Relay connection status
# - Packet ordering/buffering
# - Local connection handling
```

## 📝 **Development Notes**

### ⚠️ **"Vibecoded" Software Disclaimer**

This software was **"vibecoded"** to version 1.0.0, meaning it was developed rapidly with AI assistance (Cursor) based on inspiration and experimentation rather than formal software engineering processes.

**Important Implications:**

**Legal:**
- No warranties or guarantees of fitness for any purpose
- Use at your own risk in production environments  
- See [LICENSE.md](LICENSE.md) for full legal terms
- Consider additional liability insurance for commercial use

**Security:**
- Limited security auditing has been performed
- Cryptographic implementations use standard libraries but integration is untested
- Event handling and session management may have race conditions
- Input validation and error handling may be incomplete

**Performance:**
- No formal performance testing or optimization
- Memory usage and connection limits are untested  
- Packet ordering and relay handling are experimental
- Scalability characteristics are unknown

**Recommended Actions:**
- **Test thoroughly** in non-production environments
- **Security audit** before any production deployment
- **Performance testing** for your specific use cases  
- **Code review** by experienced Go developers
- **Monitoring and alerting** for any production usage

### Development Process
- Built using Cursor AI-powered development environment
- Rapid prototyping with iterative testing
- Community inspiration from BitDevs Brasília meetings
- Based on Nostr protocol experimentation and learning

## 📋 **TODO / Future Development**

### High Priority

- [ ] **giftwrap Encryption**: Implement NIP-59 giftwrap for traffic encryption
- [ ] **Connection Pooling**: Reuse relay connections for better performance
- [ ] **Packet Fragmentation**: Handle large packets with event size limits
- [ ] **Security Audit**: Professional security review and testing
- [ ] **Performance Testing**: Benchmarks and optimization

### Medium Priority

- [ ] **Multiple Relay Support**: Use multiple relays for redundancy  
- [ ] **Automatic Relay Discovery**: NIP-65 relay list support
- [ ] **Connection Management**: Better handling of dropped connections
- [ ] **Bandwidth Monitoring**: Track usage and performance metrics
- [ ] **Configuration Files**: YAML/TOML config file support

### Low Priority

- [ ] **GUI Client**: Desktop application for non-technical users
- [ ] **Mobile Support**: Android/iOS implementation
- [ ] **Protocol Extensions**: Custom Nostr kinds for optimization
- [ ] **Relay Operator Tools**: Monitoring and management utilities
- [ ] **Docker Images**: Containerized deployment options

### Research Ideas

- [ ] **Payment Integration**: Lightning Network payments for relay usage
- [ ] **Identity Integration**: NIP-05 identity verification
- [ ] **P2P Discovery**: Direct peer-to-peer connections via Nostr
- [ ] **Mesh Networking**: Multi-hop routing through Nostr network
- [ ] **Load Balancing**: Distribute traffic across multiple servers

## 🙏 **Acknowledgments**

### Core Dependencies

- **[go-nostr](https://github.com/nbd-wtf/go-nostr)** by nbd-wtf - Essential Nostr protocol implementation
- **[Go](https://golang.org/)** - The Go programming language and toolchain
- **Standard Library** - Go's excellent networking and cryptography packages

### Development Tools

- **[Cursor](https://cursor.sh/)** - AI-powered development environment that made rapid prototyping possible
- **[GitHub](https://github.com/)** - Source code hosting and collaboration platform
- **Git** - Version control system for project management

### Inspiration and Community

- **[fiatjaf](https://github.com/fiatjaf)** - Creator of the Nostr protocol and excellent tools like [nak](https://github.com/fiatjaf/nak)
- **[Anthony Accioly](https://primal.net/p/nprofile1qqswa8vhnelpgx9f7arjhtuzmjtqs2sdgfgmw77tzu9xankf87kl7eqxcqfnm)** - Inspiration for creative Nostr applications and protocol experimentation
- **BitDevs Brasília** - Local Bitcoin development community meetings that sparked ideas for decentralized networking solutions
- **Nostr Community** - Global community of developers building the decentralized future

### Special Thanks

The rapid development of this project was made possible by:
- The vibrant Nostr ecosystem and its innovative developers
- AI-assisted development tools that enabled rapid prototyping
- The open-source community providing robust, reusable components
- Local tech communities fostering experimentation and learning

*This project stands on the shoulders of giants in the open-source and decentralized technology communities.*

## 📄 **License**

This project is licensed under **The Girino License** - see the [LICENSE.md](LICENSE.md) file for details.

**TL;DR**: You can use this software freely, but if you modify it, you might need to wear a He-man costume in public. 😄

Full license: https://license.girino.org

---

## 🔗 **Links**

- **Repository**: https://github.com/girino/tcp-over-nostr
- **Issues**: https://github.com/girino/tcp-over-nostr/issues  
- **Releases**: https://github.com/girino/tcp-over-nostr/releases
- **License**: https://license.girino.org
- **Nostr Protocol**: https://github.com/nostr-protocol/nostr
- **nak tool**: https://github.com/fiatjaf/nak

---

**Made with ❤️ and AI assistance by [Girino Vey](https://github.com/girino)**

*"Decentralizing the internet, one TCP packet at a time."*