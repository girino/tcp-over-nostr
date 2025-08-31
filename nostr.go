package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
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

// NostrPacketEvent wraps our packet data in a Nostr event
type NostrPacketEvent struct {
	Event  *nostr.Event `json:"event"`
	Packet *Packet      `json:"-"` // Not serialized, computed from event content
}

// KeyManager handles Nostr key generation and storage
type KeyManager struct {
	keysFile string
	keys     *NostrKeys
	verbose  bool
}

// NewKeyManager creates a new key manager
func NewKeyManager(keysFile string, verbose bool) *KeyManager {
	return &KeyManager{
		keysFile: keysFile,
		verbose:  verbose,
	}
}

// LoadOrGenerateKeys loads existing keys or generates new ones
func (km *KeyManager) LoadOrGenerateKeys() error {
	// Try to load existing keys
	if _, err := os.Stat(km.keysFile); err == nil {
		data, err := os.ReadFile(km.keysFile)
		if err != nil {
			return fmt.Errorf("failed to read keys file: %v", err)
		}

		var keys NostrKeys
		if err := json.Unmarshal(data, &keys); err != nil {
			return fmt.Errorf("failed to parse keys file: %v", err)
		}

		km.keys = &keys
		if km.verbose {
			log.Printf("Loaded existing Nostr keys from %s (pubkey: %s)", km.keysFile, keys.PublicKey)
		}
		return nil
	}

	// Generate new keys
	if err := km.generateNewKeys(); err != nil {
		return fmt.Errorf("failed to generate new keys: %v", err)
	}

	if km.verbose {
		log.Printf("Generated new Nostr keys (pubkey: %s)", km.keys.PublicKey)
	}

	return nil
}

// generateNewKeys creates a new key pair and saves it
func (km *KeyManager) generateNewKeys() error {
	// Generate random private key (32 bytes)
	privateKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privateKeyBytes); err != nil {
		return fmt.Errorf("failed to generate random private key: %v", err)
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

	// Serialize packet to JSON and base64 encode
	packetJSON, err := packet.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize packet: %v", err)
	}

	// Create Nostr event
	event := &nostr.Event{
		Kind:      9999,               // Custom kind for TCP proxy packets
		Content:   string(packetJSON), // Base64 encoded packet JSON
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

// ParseNostrEvent parses a Nostr event back to a packet
func ParseNostrEvent(event *nostr.Event) (*Packet, error) {
	// Verify event kind
	if event.Kind != 9999 {
		return nil, fmt.Errorf("invalid event kind: %d", event.Kind)
	}

	// Parse the JSON content as packet
	packet, err := FromJSON([]byte(event.Content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse packet from event content: %v", err)
	}

	return packet, nil
}

// IsEventForMe checks if the event is tagged for this pubkey
func IsEventForMe(event *nostr.Event, myPubkey string) bool {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "p" && tag[1] == myPubkey {
			return true
		}
	}
	return false
}

// GetEventFilename generates a filename for a Nostr event (using event ID)
func GetEventFilename(event *nostr.Event, baseDir string) string {
	return filepath.Join(baseDir, fmt.Sprintf("%s.json", event.ID))
}

// NostrEventHandler handles reading/writing Nostr events to disk
type NostrEventHandler struct {
	baseDir       string
	keyMgr        *KeyManager
	verbose       bool
	cachedEvents  map[string]*nostr.Event // Cache for read events
	lastCacheTime time.Time               // Last time cache was updated
	cacheMutex    sync.RWMutex            // Protects cachedEvents map
}

// NewNostrEventHandler creates a new Nostr event handler
func NewNostrEventHandler(baseDir string, keyMgr *KeyManager, verbose bool) *NostrEventHandler {
	return &NostrEventHandler{
		baseDir:      baseDir,
		keyMgr:       keyMgr,
		verbose:      verbose,
		cachedEvents: make(map[string]*nostr.Event),
	}
}

// WriteEvent writes a Nostr event to disk
func (neh *NostrEventHandler) WriteEvent(event *nostr.Event) error {
	filename := GetEventFilename(event, neh.baseDir)

	// Ensure directory exists
	if err := os.MkdirAll(neh.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create events directory: %v", err)
	}

	// Serialize event to JSON
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal event: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write event file %s: %v", filename, err)
	}

	// Cache the written event
	neh.cacheMutex.Lock()
	neh.cachedEvents[filename] = event
	neh.cacheMutex.Unlock()

	if neh.verbose {
		log.Printf("NostrEventHandler: Wrote event %s to %s", event.ID, filename)
	}

	return nil
}

// ReadEvent reads a Nostr event from disk with caching
func (neh *NostrEventHandler) ReadEvent(filename string) (*nostr.Event, error) {
	// Check cache first (read lock)
	neh.cacheMutex.RLock()
	if event, exists := neh.cachedEvents[filename]; exists {
		neh.cacheMutex.RUnlock()
		return event, nil
	}
	neh.cacheMutex.RUnlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read event file %s: %v", filename, err)
	}

	var event nostr.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event from %s: %v", filename, err)
	}

	// Cache the event (write lock)
	neh.cacheMutex.Lock()
	neh.cachedEvents[filename] = &event
	neh.cacheMutex.Unlock()

	if neh.verbose {
		log.Printf("NostrEventHandler: Read event %s from %s", event.ID, filename)
	}

	return &event, nil
}

// GetEventFiles returns all event files in the directory for a specific pubkey
func (neh *NostrEventHandler) GetEventFiles(targetPubkey string, startupTime time.Time) ([]string, error) {
	var eventFiles []string

	err := filepath.WalkDir(neh.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		// Read and check event
		event, err := neh.ReadEvent(path)
		if err != nil {
			if neh.verbose {
				log.Printf("NostrEventHandler: Warning: failed to read event file %s: %v", path, err)
			}
			return nil
		}

		// Check if event is for target pubkey
		if !IsEventForMe(event, targetPubkey) {
			return nil
		}

		// Check if event is newer than startup time (for filtering old events)
		eventTime := time.Unix(int64(event.CreatedAt), 0)
		if eventTime.Before(startupTime) {
			if neh.verbose {
				log.Printf("NostrEventHandler: Ignoring old event %s (created %v, startup %v)", event.ID, eventTime, startupTime)
			}
			return nil
		}

		eventFiles = append(eventFiles, path)
		return nil
	})

	return eventFiles, err
}

// GetAllEventFiles returns all event files regardless of timestamp
func (neh *NostrEventHandler) GetAllEventFiles(targetPubkey string) ([]string, error) {
	return neh.GetEventFiles(targetPubkey, time.Unix(0, 0)) // Use epoch time to include all events
}
