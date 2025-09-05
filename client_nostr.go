package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

func runClientNostr(clientPort int, relayURLs []string, serverPubkey, privateKey string, verbose bool) {
	// Show startup banner
	fmt.Print(GetBanner())

	// Validate inputs
	if clientPort < 1 || clientPort > 65535 {
		log.Fatal("Client port must be between 1 and 65535")
	}

	if serverPubkey == "" {
		log.Fatal("Server public key is required for Nostr mode")
	}

	// Parse server public key (hex or npub format)
	serverPubkeyHex, err := ParsePublicKey(serverPubkey)
	if err != nil {
		log.Fatalf("Failed to parse server public key: %v", err)
	}

	fmt.Printf("Starting TCP proxy client (Nostr mode):\n")
	fmt.Printf("  Listen port: %d\n", clientPort)
	fmt.Printf("  Server pubkey: %s\n", serverPubkeyHex)
	fmt.Printf("  Relay URLs: %v\n", relayURLs)
	fmt.Printf("  Verbose logging: %t\n\n", verbose)

	// Initialize key manager
	keyMgr := NewKeyManager("")
	if privateKey != "" {
		// Use provided private key
		if err := keyMgr.LoadKeysFromPrivateKey(privateKey); err != nil {
			log.Fatalf("Failed to load keys from private key: %v", err)
		}
	} else {
		// Generate new keys
		if err := keyMgr.GenerateKeys(); err != nil {
			log.Fatalf("Failed to generate keys: %v", err)
		}
	}

	clientKeys := keyMgr.GetKeys()

	// Generate npub format for display
	clientNpub, err := EncodePublicKeyToNpub(clientKeys.PublicKey)
	if err != nil {
		fmt.Printf("Client Nostr pubkey (hex): %s\n\n", clientKeys.PublicKey)
	} else {
		fmt.Printf("Client Nostr pubkey (hex): %s\n", clientKeys.PublicKey)
		fmt.Printf("Client Nostr pubkey (npub): %s\n\n", clientNpub)
	}

	// Initialize relay handler
	relayHandler, err := NewNostrRelayHandler(relayURLs, keyMgr, verbose)
	if err != nil {
		log.Fatalf("Failed to connect to relays: %v", err)
	}
	defer relayHandler.Close()

	// Subscribe to encrypted gift wrap events from the server
	if err := relayHandler.SubscribeToGiftWrapEvents(clientKeys.PublicKey); err != nil {
		log.Fatalf("Failed to subscribe to encrypted events: %v", err)
	}

	// Start listening
	listenAddr := fmt.Sprintf(":%d", clientPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", listenAddr, err)
	}
	defer listener.Close()

	fmt.Printf("Client listening on %s\n", listenAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		if verbose {
			log.Printf("Client: Accepted connection from %s", conn.RemoteAddr())
		}

		// Handle each connection in a goroutine
		go handleClientConnectionNostr(conn, relayHandler, keyMgr, serverPubkeyHex, clientKeys.PublicKey, verbose)
	}
}

func sanitizeSessionID(sessionID string) string {
	// Replace problematic characters that might cause issues in filenames
	sessionID = strings.ReplaceAll(sessionID, ":", "_")
	sessionID = strings.ReplaceAll(sessionID, ".", "_")
	sessionID = strings.ReplaceAll(sessionID, "/", "_")
	sessionID = strings.ReplaceAll(sessionID, "\\", "_")
	return sessionID
}

func handleClientConnectionNostr(conn net.Conn, relayHandler *NostrRelayHandler, keyMgr *KeyManager, serverPubkeyHex, clientPubkey string, verbose bool) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	sessionID := fmt.Sprintf("session_%d_%s", time.Now().UnixNano(), clientAddr)
	sessionID = sanitizeSessionID(sessionID)

	if verbose {
		log.Printf("Client: Starting Nostr session %s for %s", sessionID, clientAddr)
	}

	// Send open packet synchronously to ensure it arrives first
	openPacket := CreateEmptyPacket()
	if err := SendNostrPacketSync(relayHandler, keyMgr, openPacket, serverPubkeyHex, PacketTypeOpen, sessionID, 0, "client_to_server", "", 0, clientAddr, "", verbose); err != nil {
		log.Printf("Client: Failed to send open packet: %v", err)
		return
	}

	// Start goroutine to read server responses
	done := make(chan bool, 2)
	go readServerNostrResponses(relayHandler, keyMgr, sessionID, clientPubkey, conn, done, verbose)

	// Read data from client connection and send as packets
	sequence := uint64(1)         // Start at 1 (open packet is 0)
	buffer := make([]byte, 32768) // Increased from 4KB to 32KB for better throughput
	// This reduces the number of Nostr events by 8x, significantly improving performance with remote relays

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				if verbose {
					log.Printf("Client: Session %s - Connection read error: %v", sessionID, err)
				}
			}
			break
		}

		if n > 0 {
			// Create data packet
			dataPacket := CreateDataPacket(buffer[:n])
			if err := SendNostrPacket(relayHandler, keyMgr, dataPacket, serverPubkeyHex, PacketTypeData, sessionID, sequence, "client_to_server", "", 0, clientAddr, "", verbose); err != nil {
				log.Printf("Client: Failed to send data packet: %v", err)
				break
			}

			if verbose {
				log.Printf("Client: Session %s - Sent %d bytes in packet (seq %d)", sessionID, n, sequence)
			}
			sequence++
		}
	}

	// Send close packet synchronously to ensure proper cleanup
	closePacket := CreateEmptyPacket()
	if err := SendNostrPacketSync(relayHandler, keyMgr, closePacket, serverPubkeyHex, PacketTypeClose, sessionID, sequence, "client_to_server", "", 0, clientAddr, "", verbose); err != nil {
		log.Printf("Client: Failed to send close packet: %v", err)
	}

	done <- true
	if verbose {
		log.Printf("Client: Session %s closed", sessionID)
	}
}

func readServerNostrResponses(relayHandler *NostrRelayHandler, keyMgr *KeyManager, sessionID, clientPubkey string, conn net.Conn, done chan bool, verbose bool) {
	defer func() { done <- true }()

	processedSequences := make(map[uint64]bool)
	nextExpectedSequence := uint64(0)
	pendingPackets := make(map[uint64]*ParsedPacket) // Buffer for out-of-order packets

	for {
		select {
		case <-done:
			return
		case event := <-relayHandler.GetEventChannel():
			if verbose {
				log.Printf("Client: Received event %s (kind %d) from relay", event.ID, event.Kind)
			}

			// Check if this event is for us
			if !IsEventForMe(event, clientPubkey) {
				if verbose {
					log.Printf("Client: Event %s not for us (our pubkey: %s)", event.ID, clientPubkey)
				}
				continue
			}

			if verbose {
				log.Printf("Client: Event %s is for us, attempting to unwrap", event.ID)
			}

			// Parse encrypted gift wrapped event
			parsedPacket, err := keyMgr.UnwrapEphemeralGiftWrap(event)
			if err != nil {
				if verbose {
					log.Printf("Client: Error unwrapping encrypted event %s: %v", event.ID, err)
				}
				continue
			}

			// Check if this packet belongs to our session
			if parsedPacket.SessionID != sessionID {
				continue
			}

			// Check direction - we want server_to_client packets
			if parsedPacket.Direction != "server_to_client" {
				continue
			}

			// Skip if already processed
			if processedSequences[parsedPacket.Sequence] {
				continue
			}

			// Check sequence order - if not the next expected, buffer it
			if parsedPacket.Sequence != nextExpectedSequence {
				pendingPackets[parsedPacket.Sequence] = parsedPacket
				if verbose {
					log.Printf("Client: Session %s - Buffering out-of-order packet seq %d (expecting %d)", sessionID, parsedPacket.Sequence, nextExpectedSequence)
				}
				continue
			}

			// Process this packet and any consecutive buffered packets
			packetsToProcess := []*ParsedPacket{parsedPacket}

			// Collect consecutive packets from buffer
			seq := nextExpectedSequence + 1
			for {
				if bufferedPacket, exists := pendingPackets[seq]; exists {
					packetsToProcess = append(packetsToProcess, bufferedPacket)
					delete(pendingPackets, seq)
					seq++
				} else {
					break
				}
			}

			// Process all packets in order
			for _, pkt := range packetsToProcess {
				// Mark as processed
				processedSequences[pkt.Sequence] = true

				// Process packet based on type
				switch pkt.Type {
				case PacketTypeData:
					// Write data to client connection
					if len(pkt.Packet.Data) > 0 {
						if _, writeErr := conn.Write(pkt.Packet.Data); writeErr != nil {
							log.Printf("Client: Session %s - Error writing to connection: %v", sessionID, writeErr)
							return
						}

						if verbose {
							log.Printf("Client: Session %s - Received %d bytes from server (seq %d)", sessionID, len(pkt.Packet.Data), pkt.Sequence)
						}
					}

				case PacketTypeClose:
					if verbose {
						log.Printf("Client: Session %s - Received close packet from server", sessionID)
					}
					return
				}

				// Update next expected sequence
				nextExpectedSequence = pkt.Sequence + 1
			}
		}
	}
}
