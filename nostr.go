package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// NostrKeys represents a Nostr key pair
type NostrKeys struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

// KeyManager handles Nostr key generation and storage
type KeyManager struct {
	keysFile string
	keys     *NostrKeys
}

// NewKeyManager creates a new key manager
func NewKeyManager(keysFile string) *KeyManager {
	return &KeyManager{
		keysFile: keysFile,
	}
}

// LoadKeys loads keys from file or generates new ones
func (km *KeyManager) LoadKeys() error {
	// Try to load existing keys
	if data, err := os.ReadFile(km.keysFile); err == nil {
		if err := json.Unmarshal(data, &km.keys); err != nil {
			return fmt.Errorf("failed to parse keys file: %v", err)
		}
		return nil
	}

	// Generate new keys if file doesn't exist
	return km.GenerateKeys()
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

	// Save keys to file
	return km.SaveKeys()
}

// SaveKeys saves keys to file
func (km *KeyManager) SaveKeys() error {
	data, err := json.MarshalIndent(km.keys, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keys: %v", err)
	}

	// Ensure directory exists
	if dir := filepath.Dir(km.keysFile); dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create keys directory: %v", err)
		}
	}

	if err := os.WriteFile(km.keysFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write keys file: %v", err)
	}

	return nil
}

// GetKeys returns the loaded keys
func (km *KeyManager) GetKeys() *NostrKeys {
	return km.keys
}

// CreateNostrEvent creates a Nostr event for a packet
func (km *KeyManager) CreateNostrEvent(packet *Packet, targetPubkey string) (*nostr.Event, error) {
	if km.keys == nil {
		return nil, fmt.Errorf("keys not loaded")
	}

	// Serialize packet to JSON
	packetJSON, err := packet.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize packet: %v", err)
	}

	// Create Nostr event
	event := &nostr.Event{
		Kind:      9999,               // Custom kind for TCP proxy packets
		Content:   string(packetJSON), // JSON packet as content
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Tags: nostr.Tags{
			{"p", targetPubkey}, // Tag the target (server or client)
			{"proxy", "tcp"},    // Identify as TCP proxy traffic
		},
	}

	// Sign the event
	if err := event.Sign(km.keys.PrivateKey); err != nil {
		return nil, fmt.Errorf("failed to sign event: %v", err)
	}

	return event, nil
}

// NostrRelayHandler handles communication with Nostr relays
type NostrRelayHandler struct {
	relay     *nostr.Relay
	relayURL  string
	keyMgr    *KeyManager
	verbose   bool
	ctx       context.Context
	cancel    context.CancelFunc
	eventChan chan *nostr.Event // Channel for received events
	mu        sync.RWMutex      // Protects shared state
}

// NewNostrRelayHandler creates a new Nostr relay handler
func NewNostrRelayHandler(relayURL string, keyMgr *KeyManager, verbose bool) (*NostrRelayHandler, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Connect to relay
	relay, err := nostr.RelayConnect(ctx, relayURL)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to relay %s: %v", relayURL, err)
	}

	handler := &NostrRelayHandler{
		relay:     relay,
		relayURL:  relayURL,
		keyMgr:    keyMgr,
		verbose:   verbose,
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan *nostr.Event, 100), // Buffered channel
	}

	if verbose {
		log.Printf("Connected to Nostr relay: %s", relayURL)
	}

	return handler, nil
}

// Close closes the relay connection and cleanup resources
func (nrh *NostrRelayHandler) Close() {
	nrh.cancel()
	if nrh.relay != nil {
		nrh.relay.Close()
	}
	close(nrh.eventChan)
}

// PublishEvent publishes a Nostr event to the relay
func (nrh *NostrRelayHandler) PublishEvent(event *nostr.Event) error {
	err := nrh.relay.Publish(nrh.ctx, *event)
	if err != nil {
		return fmt.Errorf("failed to publish event to relay: %v", err)
	}

	if nrh.verbose {
		log.Printf("Published event %s to relay %s", event.ID, nrh.relayURL)
	}

	return nil
}

// SubscribeToEvents subscribes to events for a specific pubkey
func (nrh *NostrRelayHandler) SubscribeToEvents(targetPubkey string) error {
	// Create subscription filter
	filter := nostr.Filter{
		Kinds: []int{9999},                               // TCP proxy events
		Tags:  nostr.TagMap{"p": []string{targetPubkey}}, // Events tagged for us
	}

	sub, err := nrh.relay.Subscribe(nrh.ctx, []nostr.Filter{filter})
	if err != nil {
		return fmt.Errorf("failed to subscribe to events: %v", err)
	}

	if nrh.verbose {
		log.Printf("Subscribed to events for pubkey %s", targetPubkey)
	}

	// Start goroutine to handle incoming events
	go func() {
		for event := range sub.Events {
			select {
			case nrh.eventChan <- event:
				if nrh.verbose {
					log.Printf("Received event %s from relay", event.ID)
				}
			case <-nrh.ctx.Done():
				return
			default:
				if nrh.verbose {
					log.Printf("Event channel full, dropping event %s", event.ID)
				}
			}
		}
	}()

	return nil
}

// GetEventChannel returns the channel for receiving events
func (nrh *NostrRelayHandler) GetEventChannel() <-chan *nostr.Event {
	return nrh.eventChan
}

// Helper functions for packet processing

// ParseNostrEvent parses a Nostr event content to extract the packet
func ParseNostrEvent(event *nostr.Event) (*Packet, error) {
	// Verify event kind
	if event.Kind != 9999 {
		return nil, fmt.Errorf("invalid event kind: %d", event.Kind)
	}

	var packet Packet
	if err := json.Unmarshal([]byte(event.Content), &packet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal packet: %v", err)
	}

	return &packet, nil
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
