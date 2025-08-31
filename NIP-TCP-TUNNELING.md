NIP-XX
======

TCP Tunneling over Nostr
-------------------------

`draft` `optional` `author:girino`

This NIP defines a protocol for tunneling TCP connections over Nostr relays, enabling decentralized network proxying and censorship-resistant communication.

## Motivation

Traditional TCP proxying and VPN solutions rely on centralized infrastructure that can be:
- Blocked or censored by governments or ISPs
- Subject to single points of failure
- Monitored or logged by service providers
- Expensive to operate and maintain

By leveraging Nostr's decentralized relay network, TCP tunneling becomes:
- Censorship-resistant through relay diversity
- Decentralized with no single point of control
- Cost-effective using existing Nostr infrastructure
- Verifiable through cryptographic signatures

## Description

This NIP introduces a new event kind `20547` for tunneling TCP connections through Nostr relays.

## Event Kind

| Kind    | Description        |
| ------- | ------------------ |
| `20547` | TCP Proxy Packet  |

Kind `20547` is an _ephemeral event_ as defined by [NIP-16](16.md).

## Event Format

TCP proxy events MUST be ephemeral events of kind `20547` with the following structure:

```json
{
  "kind": 20547,
  "content": "<base64-encoded-json-packet>",
  "tags": [
    ["p", "<recipient-pubkey>"],
    ["proxy", "tcp"],
    ["session", "<session-id>"],
    ["seq", "<sequence-number>"]
  ],
  "created_at": <unix-timestamp>,
  "pubkey": "<sender-pubkey>",
  "id": "<event-id>",
  "sig": "<signature>"
}
```

## Content Format

The `content` field contains a base64-encoded JSON packet:

```json
{
  "type": "open|data|close",
  "session_id": "<unique-session-identifier>",
  "sequence": <packet-sequence-number>,
  "data": "<base64-encoded-tcp-data>",
  "timestamp": <unix-timestamp-microseconds>
}
```

## Packet Types

### Open Packet
Initiates a new TCP session (`type: "open"`):
```json
{
  "type": "open",
  "session_id": "session_1234567890_client_identifier",
  "sequence": 0,
  "data": "",
  "timestamp": 1234567890123456
}
```

### Data Packet
Carries TCP payload data (`type: "data"`):
```json
{
  "type": "data", 
  "session_id": "session_1234567890_client_identifier",
  "sequence": 42,
  "data": "SGVsbG8gV29ybGQ=",
  "timestamp": 1234567890123456
}
```

### Close Packet
Terminates a TCP session (`type: "close"`):
```json
{
  "type": "close",
  "session_id": "session_1234567890_client_identifier", 
  "sequence": 100,
  "data": "",
  "timestamp": 1234567890123456
}
```

## Protocol Details

### Session Identifiers
Session IDs MUST be unique and SHOULD include:
- Timestamp for uniqueness
- Client identifier for disambiguation  
- Random component for unpredictability

Format: `session_<timestamp>_<client-id>_<random>`

### Sequence Numbers
- Each packet within a session MUST have a unique sequence number
- Sequence numbers MUST start at 0 for the open packet
- Sequence numbers MUST increment by 1 for each subsequent packet
- Recipients MUST buffer out-of-order packets and process them sequentially

### Event Subscription
Clients MUST subscribe to events with:
```json
{
  "kinds": [20547],
  "#p": ["<client-pubkey>"],
  "since": <startup-timestamp>
}
```

### Packet Processing
1. **Ordering**: Buffer out-of-order packets and process sequentially
2. **Deduplication**: Ignore packets with duplicate sequence numbers
3. **Timeout**: Implement timeouts for missing packets
4. **Error Handling**: Close sessions on protocol violations

## Security Considerations

⚠️ **CRITICAL**: This protocol does NOT encrypt TCP traffic. The `data` field contains plaintext (base64-encoded) TCP payloads.

Implementations MUST:
- Warn users that traffic is not encrypted
- Recommend using TLS/SSL for sensitive data
- Document that all traffic is visible to relay operators

Additional considerations:
- Session IDs and packet timing are visible to relay operators
- Traffic patterns may be analyzable  
- Consider using [NIP-59](59.md) giftwrap for enhanced privacy
- Relay operators can log, monitor, or censor traffic

## Implementation

### Reference Implementation
A proof-of-concept implementation is available at: https://github.com/girino/tcp-over-nostr

### Example Usage
```bash
# Start server
./tcp-proxy -mode server -target-host example.com -target-port 80

# Start client  
./tcp-proxy -mode client -server-key <server_pubkey> -client-port 8080

# Use tunneled connection
curl http://localhost:8080
```

## Use Cases

- SSH tunneling through censorship
- HTTP proxy for restricted content
- IoT device communication
- Decentralized VPN alternatives
- Development through firewalls

## Rationale

### Ephemeral Events
TCP traffic is naturally transient, making ephemeral events appropriate for automatic cleanup and reduced relay storage burden.

### Kind 20547
Selected from the ephemeral range (20000-29999), with "547" referencing the common relay port 10547 for easy identification.

### Base64 Encoding
Provides JSON-safe encoding for binary TCP data with reasonable size overhead and universal language support.
