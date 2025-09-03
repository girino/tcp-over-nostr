package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/nbd-wtf/go-nostr/nip44"
)

// NostrKeys represents a Nostr key pair
type NostrKeys struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

// KeyManager handles Nostr key generation
type KeyManager struct {
	keys *NostrKeys
}

// NewKeyManager creates a new key manager
func NewKeyManager(keysFile string) *KeyManager {
	return &KeyManager{}
}

// GenerateKeys generates new Nostr keys
func (km *KeyManager) GenerateKeys() error {
	// Generate private key (32 random bytes)
	privateKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privateKeyBytes); err != nil {
		return fmt.Errorf("failed to generate random bytes: %v", err)
	}

	privateKeyHex := hex.EncodeToString(privateKeyBytes)
	publicKey, err := nostr.GetPublicKey(privateKeyHex)
	if err != nil {
		return fmt.Errorf("failed to derive public key: %v", err)
	}

	km.keys = &NostrKeys{
		PrivateKey: privateKeyHex,
		PublicKey:  publicKey,
	}

	return nil
}

// GetKeys returns the loaded keys
func (km *KeyManager) GetKeys() *NostrKeys {
	return km.keys
}

// ParsePrivateKey parses a private key from hex or nsec format
func ParsePrivateKey(privateKeyStr string) (string, error) {
	if privateKeyStr == "" {
		return "", fmt.Errorf("private key cannot be empty")
	}

	// Check if it's nsec format
	if strings.HasPrefix(privateKeyStr, "nsec") {
		return parseNsecKey(privateKeyStr)
	}

	// Assume hex format
	return parseHexKey(privateKeyStr)
}

// parseNsecKey parses a private key from nsec format
func parseNsecKey(nsec string) (string, error) {
	// Validate nsec format
	if len(nsec) < 5 {
		return "", fmt.Errorf("invalid nsec format: too short")
	}

	if !strings.HasPrefix(nsec, "nsec") {
		return "", fmt.Errorf("invalid nsec format: must start with 'nsec'")
	}

	// Decode nsec using nip19
	prefix, data, err := nip19.Decode(nsec)
	if err != nil {
		return "", fmt.Errorf("failed to decode nsec: %v", err)
	}

	// Validate prefix
	if prefix != "nsec" {
		return "", fmt.Errorf("invalid nsec format: expected 'nsec' prefix, got '%s'", prefix)
	}

	// Handle different possible return types from nip19.Decode
	var dataBytes []byte
	switch v := data.(type) {
	case []byte:
		dataBytes = v
	case string:
		// If it's a string, try to decode it as hex
		var err error
		dataBytes, err = hex.DecodeString(v)
		if err != nil {
			return "", fmt.Errorf("invalid nsec format: data string is not valid hex: %v", err)
		}
	default:
		return "", fmt.Errorf("invalid nsec format: unexpected data type %T", data)
	}

	// Validate length (should be 32 bytes for private key)
	if len(dataBytes) != 32 {
		return "", fmt.Errorf("invalid nsec format: expected 32 bytes, got %d bytes", len(dataBytes))
	}

	// Convert data to hex string
	hexKey := hex.EncodeToString(dataBytes)

	return hexKey, nil
}

// ParsePublicKey parses a public key from hex or npub format
func ParsePublicKey(publicKeyStr string) (string, error) {
	if publicKeyStr == "" {
		return "", fmt.Errorf("public key cannot be empty")
	}

	// Check if it's npub format
	if strings.HasPrefix(publicKeyStr, "npub") {
		return parseNpubKey(publicKeyStr)
	}

	// Assume hex format
	return parseHexPublicKey(publicKeyStr)
}

// parseNpubKey parses a public key from npub format
func parseNpubKey(npub string) (string, error) {
	// Validate npub format
	if len(npub) < 5 {
		return "", fmt.Errorf("invalid npub format: too short")
	}

	if !strings.HasPrefix(npub, "npub") {
		return "", fmt.Errorf("invalid npub format: must start with 'npub'")
	}

	// Decode npub using nip19
	prefix, data, err := nip19.Decode(npub)
	if err != nil {
		return "", fmt.Errorf("failed to decode npub: %v", err)
	}

	// Validate prefix
	if prefix != "npub" {
		return "", fmt.Errorf("invalid npub format: expected 'npub' prefix, got '%s'", prefix)
	}

	// Handle different possible return types from nip19.Decode
	var dataBytes []byte
	switch v := data.(type) {
	case []byte:
		dataBytes = v
	case string:
		// If it's a string, try to decode it as hex
		var err error
		dataBytes, err = hex.DecodeString(v)
		if err != nil {
			return "", fmt.Errorf("invalid npub format: data string is not valid hex: %v", err)
		}
	default:
		return "", fmt.Errorf("invalid npub format: unexpected data type %T", data)
	}

	// Validate length (should be 32 bytes for public key)
	if len(dataBytes) != 32 {
		return "", fmt.Errorf("invalid npub format: expected 32 bytes, got %d bytes", len(dataBytes))
	}

	// Convert data to hex string
	hexKey := hex.EncodeToString(dataBytes)
	return hexKey, nil
}

// parseHexPublicKey parses a public key from hex format
func parseHexPublicKey(hexKey string) (string, error) {
	// Remove any 0x prefix if present
	hexKey = strings.TrimPrefix(hexKey, "0x")

	// Decode hex to verify it's valid
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("invalid hex public key: %v", err)
	}

	// Check if it's 32 bytes (256 bits)
	if len(keyBytes) != 32 {
		return "", fmt.Errorf("public key must be 32 bytes (64 hex characters), got %d bytes", len(keyBytes))
	}

	return hexKey, nil
}

// EncodePublicKeyToNpub converts a hex public key to npub format
func EncodePublicKeyToNpub(hexKey string) (string, error) {
	// Decode hex to bytes
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("invalid hex public key: %v", err)
	}

	// Validate length
	if len(keyBytes) != 32 {
		return "", fmt.Errorf("public key must be 32 bytes, got %d bytes", len(keyBytes))
	}

	// Encode to npub using nip19
	npub, err := nip19.EncodePublicKey(hexKey)
	if err != nil {
		return "", fmt.Errorf("failed to encode npub: %v", err)
	}

	return npub, nil
}

// EncodePrivateKeyToNsec converts a hex private key to nsec format
func EncodePrivateKeyToNsec(hexKey string) (string, error) {
	// Decode hex to bytes
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("invalid hex private key: %v", err)
	}

	// Validate length
	if len(keyBytes) != 32 {
		return "", fmt.Errorf("private key must be 32 bytes, got %d bytes", len(keyBytes))
	}

	// Encode to nsec using nip19
	nsec, err := nip19.EncodePrivateKey(hexKey)
	if err != nil {
		return "", fmt.Errorf("failed to encode nsec: %v", err)
	}

	return nsec, nil
}

// parseHexKey parses a private key from hex format
func parseHexKey(hexKey string) (string, error) {
	// Remove any 0x prefix if present
	hexKey = strings.TrimPrefix(hexKey, "0x")

	// Decode hex to verify it's valid
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("invalid hex private key: %v", err)
	}

	// Check if it's 32 bytes (256 bits)
	if len(keyBytes) != 32 {
		return "", fmt.Errorf("private key must be 32 bytes (64 hex characters), got %d bytes", len(keyBytes))
	}

	return hexKey, nil
}

// LoadKeysFromPrivateKey loads keys using a provided private key string
func (km *KeyManager) LoadKeysFromPrivateKey(privateKeyStr string) error {
	privateKeyHex, err := ParsePrivateKey(privateKeyStr)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %v", err)
	}

	// Derive public key from private key
	publicKey, err := nostr.GetPublicKey(privateKeyHex)
	if err != nil {
		return fmt.Errorf("failed to derive public key: %v", err)
	}

	km.keys = &NostrKeys{
		PrivateKey: privateKeyHex,
		PublicKey:  publicKey,
	}

	return nil
}

// CreateNostrEvent creates a Nostr event for a packet with metadata in tags
func (km *KeyManager) CreateNostrEvent(packet *Packet, targetPubkey string, packetType PacketType, sessionID string, sequence uint64, direction string, targetHost string, targetPort int, clientAddr string, errorMsg string) (*nostr.Event, error) {
	if km.keys == nil {
		return nil, fmt.Errorf("keys not loaded")
	}

	// Encode packet data as base64 for content
	var content string
	if len(packet.Data) > 0 {
		content = base64.StdEncoding.EncodeToString(packet.Data)
	}

	// Create tags with all metadata
	tags := nostr.Tags{
		{"p", targetPubkey},                       // Tag the target (server or client)
		{"proxy", "tcp"},                          // Identify as TCP proxy traffic
		{"type", string(packetType)},              // Packet type (open, data, close, etc.)
		{"session", sessionID},                    // Session identifier
		{"sequence", fmt.Sprintf("%d", sequence)}, // Sequence number
		{"direction", direction},                  // Direction (client_to_server, server_to_client)
	}

	// Add optional tags based on packet type
	if targetHost != "" {
		tags = append(tags, nostr.Tag{"target_host", targetHost})
	}
	if targetPort > 0 {
		tags = append(tags, nostr.Tag{"target_port", fmt.Sprintf("%d", targetPort)})
	}
	if clientAddr != "" {
		tags = append(tags, nostr.Tag{"client_addr", clientAddr})
	}
	if errorMsg != "" {
		tags = append(tags, nostr.Tag{"error", errorMsg})
	}

	// Create Nostr event
	event := &nostr.Event{
		Kind:      20547,   // Ephemeral event for TCP proxy packets
		Content:   content, // Base64 encoded raw data only
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Tags:      tags,
	}

	// Sign the event
	if err := event.Sign(km.keys.PrivateKey); err != nil {
		return nil, fmt.Errorf("failed to sign event: %v", err)
	}

	return event, nil
}

// NostrRelayHandler handles communication with multiple Nostr relays
type NostrRelayHandler struct {
	pool      *nostr.SimplePool
	relayURLs []string
	keyMgr    *KeyManager
	verbose   bool
	ctx       context.Context
	cancel    context.CancelFunc
	eventChan chan *nostr.Event // Channel for received events
	mu        sync.RWMutex      // Protects shared state
}

// NewNostrRelayHandler creates a new Nostr relay handler with multiple relays
func NewNostrRelayHandler(relayURLs []string, keyMgr *KeyManager, verbose bool) (*NostrRelayHandler, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a simple pool of relays using the standard library
	pool := nostr.NewSimplePool(ctx)

	// Add all relays to the pool
	for _, relayURL := range relayURLs {
		_, err := pool.EnsureRelay(relayURL)
		if err != nil {
			if verbose {
				log.Printf("Failed to add relay %s to pool: %v", relayURL, err)
			}
			continue
		}
		if verbose {
			log.Printf("Added relay to pool: %s", relayURL)
		}
	}

	// Check if we have any relays in the pool
	if pool.Relays.Size() == 0 {
		cancel()
		return nil, fmt.Errorf("failed to add any relay to pool")
	}

	handler := &NostrRelayHandler{
		pool:      pool,
		relayURLs: relayURLs,
		keyMgr:    keyMgr,
		verbose:   verbose,
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan *nostr.Event, 100), // Buffered channel
	}

	if verbose {
		log.Printf("Created pool with %d relay(s): %v", pool.Relays.Size(), relayURLs)
	}

	return handler, nil
}

// Close closes all relay connections and cleanup resources
func (nrh *NostrRelayHandler) Close() {
	nrh.cancel()
	close(nrh.eventChan)
}

// PublishEvent publishes a Nostr event to all relays in the pool
func (nrh *NostrRelayHandler) PublishEvent(event *nostr.Event) error {
	// Use the pool's PublishMany method which handles multiple relays automatically
	results := nrh.pool.PublishMany(nrh.ctx, nrh.relayURLs, *event)

	successCount := 0
	var errors []string

	for result := range results {
		if result.Error != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", result.RelayURL, result.Error))
			if nrh.verbose {
				log.Printf("Failed to publish event %s to relay %s: %v", event.ID, result.RelayURL, result.Error)
			}
		} else {
			successCount++
			if nrh.verbose {
				log.Printf("Published event %s to relay %s", event.ID, result.RelayURL)
			}
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to publish event to any relay: %v", errors)
	}

	if len(errors) > 0 && nrh.verbose {
		log.Printf("Published to %d/%d relays, errors: %v", successCount, len(nrh.relayURLs), errors)
	}

	return nil
}

// SubscribeToEvents subscribes to events for a specific pubkey using the pool
func (nrh *NostrRelayHandler) SubscribeToEvents(targetPubkey string) error {
	// Create subscription filter
	filter := nostr.Filter{
		Kinds: []int{20547},                              // Ephemeral TCP proxy events
		Tags:  nostr.TagMap{"p": []string{targetPubkey}}, // Events tagged for us
	}

	// Use the pool's SubscribeMany method which handles multiple relays and deduplication automatically
	events := nrh.pool.SubscribeMany(nrh.ctx, nrh.relayURLs, filter)

	// Start goroutine to handle incoming events
	go func() {
		for relayEvent := range events {
			select {
			case nrh.eventChan <- relayEvent.Event:
				if nrh.verbose {
					log.Printf("Received event %s from relay %s", relayEvent.Event.ID, relayEvent.Relay)
				}
			case <-nrh.ctx.Done():
				return
			default:
				if nrh.verbose {
					log.Printf("Event channel full, dropping event %s from relay %s", relayEvent.Event.ID, relayEvent.Relay)
				}
			}
		}
	}()

	if nrh.verbose {
		log.Printf("Subscribed to events for pubkey %s using pool", targetPubkey)
	}

	return nil
}

// SubscribeToGiftWrapEvents subscribes to encrypted gift wrap events for a specific pubkey using the pool
func (nrh *NostrRelayHandler) SubscribeToGiftWrapEvents(targetPubkey string) error {
	// Create subscription filter for gift wrap events
	filter := nostr.Filter{
		Kinds: []int{21059},                              // Ephemeral gift wrap events
		Tags:  nostr.TagMap{"p": []string{targetPubkey}}, // Events tagged for us
	}

	// Use the pool's SubscribeMany method which handles multiple relays and deduplication automatically
	events := nrh.pool.SubscribeMany(nrh.ctx, nrh.relayURLs, filter)

	// Start goroutine to handle incoming events
	go func() {
		for relayEvent := range events {
			select {
			case nrh.eventChan <- relayEvent.Event:
				if nrh.verbose {
					log.Printf("Received encrypted gift wrap event %s from relay %s", relayEvent.Event.ID, relayEvent.Relay)
				}
			case <-nrh.ctx.Done():
				return
			default:
				if nrh.verbose {
					log.Printf("Event channel full, dropping gift wrap event %s from relay %s", relayEvent.Event.ID, relayEvent.Relay)
				}
			}
		}
	}()

	if nrh.verbose {
		log.Printf("Subscribed to encrypted gift wrap events for pubkey %s using pool", targetPubkey)
	}

	return nil
}

// GetEventChannel returns the channel for receiving events
func (nrh *NostrRelayHandler) GetEventChannel() <-chan *nostr.Event {
	return nrh.eventChan
}

// GetRelayURL returns the first relay URL (for backward compatibility)
func (nrh *NostrRelayHandler) GetRelayURL() string {
	if len(nrh.relayURLs) > 0 {
		return nrh.relayURLs[0]
	}
	return ""
}

// GetRelayURLs returns all relay URLs
func (nrh *NostrRelayHandler) GetRelayURLs() []string {
	return nrh.relayURLs
}

// Helper functions for packet processing

// ParsedPacket represents a packet with metadata extracted from Nostr event tags
type ParsedPacket struct {
	Packet       *Packet
	Type         PacketType
	SessionID    string
	Sequence     uint64
	Direction    string
	TargetHost   string
	TargetPort   int
	ClientAddr   string
	ErrorMsg     string
	ClientPubkey string // Real client pubkey from the rumor
}

// ParseNostrEvent parses a Nostr event to extract packet data and metadata from tags
func ParseNostrEvent(event *nostr.Event) (*ParsedPacket, error) {
	// Verify event kind
	if event.Kind != 20547 {
		return nil, fmt.Errorf("invalid event kind: %d", event.Kind)
	}

	// Decode base64 content to get raw data
	var data []byte
	if event.Content != "" {
		decoded, err := base64.StdEncoding.DecodeString(event.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 content: %v", err)
		}
		data = decoded
	}

	// Create packet with raw data
	packet := &Packet{Data: data}

	// Extract metadata from tags
	parsed := &ParsedPacket{Packet: packet}

	// Helper function to get tag value
	getTagValue := func(tagName string) string {
		for _, tag := range event.Tags {
			if len(tag) >= 2 && tag[0] == tagName {
				return tag[1]
			}
		}
		return ""
	}

	// Extract required metadata
	parsed.Type = PacketType(getTagValue("type"))
	parsed.SessionID = getTagValue("session")
	parsed.Direction = getTagValue("direction")

	// Parse sequence number
	if seqStr := getTagValue("sequence"); seqStr != "" {
		if _, err := fmt.Sscanf(seqStr, "%d", &parsed.Sequence); err != nil {
			return nil, fmt.Errorf("invalid sequence number: %s", seqStr)
		}
	}

	// Extract optional metadata
	parsed.TargetHost = getTagValue("target_host")
	parsed.ClientAddr = getTagValue("client_addr")
	parsed.ErrorMsg = getTagValue("error")

	// Parse target port
	if portStr := getTagValue("target_port"); portStr != "" {
		if _, err := fmt.Sscanf(portStr, "%d", &parsed.TargetPort); err != nil {
			return nil, fmt.Errorf("invalid target port: %s", portStr)
		}
	}

	return parsed, nil
}

// IsEventForMe checks if an event is tagged for the current public key
func IsEventForMe(event *nostr.Event, myPubkey string) bool {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "p" && tag[1] == myPubkey {
			return true
		}
	}
	return false
}

// Ephemeral Gift Wrap Implementation (NIP-59 with ephemeral kinds)

// CreateEphemeralGiftWrappedEvent creates an ephemeral gift wrapped event for secure transmission
// Uses ephemeral kinds (20000-29999) to ensure events are not stored permanently by relays
func (km *KeyManager) CreateEphemeralGiftWrappedEvent(packet *Packet, targetPubkey string, packetType PacketType, sessionID string, sequence uint64, direction string, targetHost string, targetPort int, clientAddr string, errorMsg string) (*nostr.Event, error) {
	if km.keys == nil {
		return nil, fmt.Errorf("keys not loaded")
	}

	// 1. Create the rumor (unsigned event with kind 20547)
	rumor, err := km.createEphemeralRumor(packet, packetType, sessionID, sequence, direction, targetHost, targetPort, clientAddr, errorMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to create rumor: %v", err)
	}

	// 2. Create ephemeral seal (kind 20013) with encrypted rumor
	seal, err := km.createEphemeralSeal(rumor, targetPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to create seal: %v", err)
	}

	// 3. Create ephemeral gift wrap (kind 21059) with encrypted seal
	giftWrap, err := km.createEphemeralGiftWrap(seal, targetPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to create gift wrap: %v", err)
	}

	return giftWrap, nil
}

// createEphemeralRumor creates an unsigned event (rumor) with kind 20547
func (km *KeyManager) createEphemeralRumor(packet *Packet, packetType PacketType, sessionID string, sequence uint64, direction string, targetHost string, targetPort int, clientAddr string, errorMsg string) (*nostr.Event, error) {
	// Encode packet data as base64 for content
	var content string
	if len(packet.Data) > 0 {
		content = base64.StdEncoding.EncodeToString(packet.Data)
	}

	// Create tags with all metadata
	tags := nostr.Tags{
		{"proxy", "tcp"},                          // Identify as TCP proxy traffic
		{"type", string(packetType)},              // Packet type (open, data, close, etc.)
		{"session", sessionID},                    // Session identifier
		{"sequence", fmt.Sprintf("%d", sequence)}, // Sequence number
		{"direction", direction},                  // Direction (client_to_server, server_to_client)
	}

	// Add optional tags based on packet type
	if targetHost != "" {
		tags = append(tags, nostr.Tag{"target_host", targetHost})
	}
	if targetPort > 0 {
		tags = append(tags, nostr.Tag{"target_port", fmt.Sprintf("%d", targetPort)})
	}
	if clientAddr != "" {
		tags = append(tags, nostr.Tag{"client_addr", clientAddr})
	}
	if errorMsg != "" {
		tags = append(tags, nostr.Tag{"error", errorMsg})
	}

	// Create unsigned rumor event
	rumor := &nostr.Event{
		Kind:      20547,   // Ephemeral event for TCP proxy packets
		Content:   content, // Base64 encoded raw data only
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Tags:      tags,
		PubKey:    km.keys.PublicKey,
	}

	// Calculate ID for the rumor (but don't sign it)
	rumor.ID = rumor.GetID()

	return rumor, nil
}

// createEphemeralSeal creates an ephemeral seal (kind 20013) with encrypted rumor
func (km *KeyManager) createEphemeralSeal(rumor *nostr.Event, targetPubkey string) (*nostr.Event, error) {
	// Serialize rumor to JSON
	rumorJSON, err := json.Marshal(rumor)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize rumor: %v", err)
	}

	// Generate conversation key for encryption
	conversationKey, err := nip44.GenerateConversationKey(targetPubkey, km.keys.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate conversation key: %v", err)
	}

	// Encrypt rumor using NIP-44
	encryptedRumor, err := nip44.Encrypt(string(rumorJSON), conversationKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt rumor: %v", err)
	}

	// Create ephemeral seal event (kind 20013 - ephemeral version of kind 13)
	seal := &nostr.Event{
		Kind:      20013,          // Ephemeral seal event kind
		Content:   encryptedRumor, // Encrypted rumor
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Tags:      nostr.Tags{},      // Tags MUST be empty in seal events
		PubKey:    km.keys.PublicKey, // Real author's pubkey
	}

	// Sign the seal
	if err := seal.Sign(km.keys.PrivateKey); err != nil {
		return nil, fmt.Errorf("failed to sign seal: %v", err)
	}

	return seal, nil
}

// createEphemeralGiftWrap creates an ephemeral gift wrap (kind 21059) with encrypted seal
func (km *KeyManager) createEphemeralGiftWrap(seal *nostr.Event, targetPubkey string) (*nostr.Event, error) {
	// Generate a random one-time-use keypair for the gift wrap FIRST
	oneTimePrivKey := nostr.GeneratePrivateKey()
	oneTimePubKey, err := nostr.GetPublicKey(oneTimePrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive one-time public key: %v", err)
	}

	// Serialize seal to JSON
	sealJSON, err := json.Marshal(seal)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize seal: %v", err)
	}

	// Generate conversation key between one-time key and target (NOT sender and target)
	conversationKey, err := nip44.GenerateConversationKey(targetPubkey, oneTimePrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate conversation key: %v", err)
	}

	// Encrypt seal using NIP-44 with one-time key conversation
	encryptedSeal, err := nip44.Encrypt(string(sealJSON), conversationKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt seal: %v", err)
	}

	// Create ephemeral gift wrap event (kind 21059 - ephemeral version of kind 1059)
	giftWrap := &nostr.Event{
		Kind:      21059,         // Ephemeral gift wrap event kind
		Content:   encryptedSeal, // Encrypted seal
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Tags: nostr.Tags{
			{"p", targetPubkey}, // Tag the recipient
		},
		PubKey: oneTimePubKey, // Random one-time-use pubkey
	}

	// Sign the gift wrap with the one-time key
	if err := giftWrap.Sign(oneTimePrivKey); err != nil {
		return nil, fmt.Errorf("failed to sign gift wrap: %v", err)
	}

	return giftWrap, nil
}

// UnwrapEphemeralGiftWrap unwraps an ephemeral gift wrapped event
func (km *KeyManager) UnwrapEphemeralGiftWrap(giftWrap *nostr.Event) (*ParsedPacket, error) {
	// Generate conversation key for decryption (recipient's private key + one-time public key)
	conversationKey, err := nip44.GenerateConversationKey(giftWrap.PubKey, km.keys.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate conversation key: %v", err)
	}

	// Decrypt the seal from the gift wrap
	sealJSON, err := nip44.Decrypt(giftWrap.Content, conversationKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt seal: %v", err)
	}

	// Parse the seal
	var seal nostr.Event
	if err := json.Unmarshal([]byte(sealJSON), &seal); err != nil {
		return nil, fmt.Errorf("failed to parse seal: %v", err)
	}

	// Verify seal signature
	if ok, _ := seal.CheckSignature(); !ok {
		return nil, fmt.Errorf("invalid seal signature")
	}

	// Generate conversation key for rumor decryption
	rumorConversationKey, err := nip44.GenerateConversationKey(seal.PubKey, km.keys.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate rumor conversation key: %v", err)
	}

	// Decrypt the rumor from the seal
	rumorJSON, err := nip44.Decrypt(seal.Content, rumorConversationKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt rumor: %v", err)
	}

	// Parse the rumor
	var rumor nostr.Event
	if err := json.Unmarshal([]byte(rumorJSON), &rumor); err != nil {
		return nil, fmt.Errorf("failed to parse rumor: %v", err)
	}

	// Parse the rumor as a ParsedPacket
	return km.parseRumorAsPacket(&rumor)
}

// parseRumorAsPacket parses a rumor event into a ParsedPacket
func (km *KeyManager) parseRumorAsPacket(rumor *nostr.Event) (*ParsedPacket, error) {
	// Verify event kind
	if rumor.Kind != 20547 {
		return nil, fmt.Errorf("invalid rumor kind: %d", rumor.Kind)
	}

	// Decode base64 content to get raw data
	var data []byte
	if rumor.Content != "" {
		decoded, err := base64.StdEncoding.DecodeString(rumor.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 content: %v", err)
		}
		data = decoded
	}

	// Create packet with raw data
	packet := &Packet{Data: data}

	// Extract metadata from tags
	parsed := &ParsedPacket{
		Packet:       packet,
		ClientPubkey: rumor.PubKey, // Extract real client pubkey from rumor
	}

	// Helper function to get tag value
	getTagValue := func(tagName string) string {
		for _, tag := range rumor.Tags {
			if len(tag) >= 2 && tag[0] == tagName {
				return tag[1]
			}
		}
		return ""
	}

	// Extract required metadata
	parsed.Type = PacketType(getTagValue("type"))
	parsed.SessionID = getTagValue("session")
	parsed.Direction = getTagValue("direction")

	// Parse sequence number
	if seqStr := getTagValue("sequence"); seqStr != "" {
		if _, err := fmt.Sscanf(seqStr, "%d", &parsed.Sequence); err != nil {
			return nil, fmt.Errorf("invalid sequence number: %s", seqStr)
		}
	}

	// Extract optional metadata
	parsed.TargetHost = getTagValue("target_host")
	parsed.ClientAddr = getTagValue("client_addr")
	parsed.ErrorMsg = getTagValue("error")

	// Parse target port
	if portStr := getTagValue("target_port"); portStr != "" {
		if _, err := fmt.Sscanf(portStr, "%d", &parsed.TargetPort); err != nil {
			return nil, fmt.Errorf("invalid target port: %s", portStr)
		}
	}

	return parsed, nil
}
