package main

import (
	"fmt"
	"log"
	"net"

	"github.com/nbd-wtf/go-nostr"
)

func runServerNostr(targetHost string, targetPort int, relayURLs []string, privateKey string, verbose bool) {
	// Show startup banner
	fmt.Print(GetBanner())

	// Validate inputs
	if targetPort < 1 || targetPort > 65535 {
		log.Fatal("Target port must be between 1 and 65535")
	}

	targetAddr := fmt.Sprintf("%s:%d", targetHost, targetPort)

	fmt.Printf("Starting TCP proxy server (Nostr mode):\n")
	fmt.Printf("  Target: %s\n", targetAddr)
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

	serverKeys := keyMgr.GetKeys()

	// Generate npub format for display
	npub, err := EncodePublicKeyToNpub(serverKeys.PublicKey)
	if err != nil {
		fmt.Printf("Server Nostr pubkey (hex): %s\n", serverKeys.PublicKey)
		fmt.Printf("Warning: Failed to generate npub format: %v\n", err)
	} else {
		fmt.Printf("Server Nostr pubkey (hex): %s\n", serverKeys.PublicKey)
		fmt.Printf("Server Nostr pubkey (npub): %s\n", npub)
	}
	fmt.Printf("Share this pubkey with clients using -server-key parameter\n\n")

	// Initialize relay handler
	relayHandler, err := NewNostrRelayHandler(relayURLs, keyMgr, verbose)
	if err != nil {
		log.Fatalf("Failed to connect to relays: %v", err)
	}
	defer relayHandler.Close()

	// Subscribe to encrypted gift wrap events for this server
	if err := relayHandler.SubscribeToGiftWrapEvents(serverKeys.PublicKey); err != nil {
		log.Fatalf("Failed to subscribe to encrypted events: %v", err)
	}

	fmt.Printf("TCP proxy server started successfully. Monitoring for Nostr events...\n\n")

	// Monitor for new session events
	monitorNostrSessionEvents(relayHandler, keyMgr, serverKeys.PublicKey, targetAddr, verbose)
}

func monitorNostrSessionEvents(relayHandler *NostrRelayHandler, keyMgr *KeyManager, serverPubkey, targetAddr string, verbose bool) {
	activeSessions := make(map[string]chan bool)            // sessionID -> done channel
	sessionEventChans := make(map[string]chan *nostr.Event) // sessionID -> event channel

	for {
		select {
		case event := <-relayHandler.GetEventChannel():
			// Check if this event is for us
			if !IsEventForMe(event, serverPubkey) {
				continue
			}

			// Version compatibility is now checked in UnwrapEphemeralGiftWrap

			// Parse encrypted gift wrapped event
			parsedPacket, err := keyMgr.UnwrapEphemeralGiftWrap(event)
			if err != nil {
				if verbose {
					log.Printf("Server: Error unwrapping encrypted event: %v", err)
				}
				continue
			}

			// Only handle client_to_server packets
			if parsedPacket.Direction != "client_to_server" {
				continue
			}

			// Check if this is an open packet for a new session
			if parsedPacket.Type == PacketTypeOpen {
				// Check if we already have this session
				if _, exists := activeSessions[parsedPacket.SessionID]; exists {
					continue // Session already active
				}

				if verbose {
					log.Printf("Server: New session %s from client", parsedPacket.SessionID)
				}

				// Create session-specific event channel
				sessionEventChan := make(chan *nostr.Event, 100)
				sessionEventChans[parsedPacket.SessionID] = sessionEventChan

				// Start new session handler with its own event channel
				// Use the real client pubkey from the rumor, not the one-time pubkey from gift wrap
				done := make(chan bool)
				activeSessions[parsedPacket.SessionID] = done
				go handleServerNostrSessionWithEvents(keyMgr, parsedPacket.SessionID, parsedPacket.ClientPubkey, targetAddr, relayHandler.GetRelayURLs(), sessionEventChan, done, verbose)

				// Clean up when session is done
				go func(sessionID string, doneChan chan bool) {
					<-doneChan
					delete(activeSessions, sessionID)
					if sessionEventChan, exists := sessionEventChans[sessionID]; exists {
						close(sessionEventChan)
						delete(sessionEventChans, sessionID)
					}
					if verbose {
						log.Printf("Server: Session %s completed and cleaned up", sessionID)
					}
				}(parsedPacket.SessionID, done)
			} else {
				// This is a data/close packet for an existing session
				if sessionEventChan, exists := sessionEventChans[parsedPacket.SessionID]; exists {
					select {
					case sessionEventChan <- event:
						// Successfully forwarded to session handler
					default:
						if verbose {
							log.Printf("Server: Session %s event channel full, dropping event", parsedPacket.SessionID)
						}
					}
				} else {
					if verbose {
						log.Printf("Server: Received event for unknown session %s", parsedPacket.SessionID)
					}
				}
			}
		}
	}
}

func handleServerNostrSessionWithEvents(keyMgr *KeyManager, sessionID, clientPubkey, targetAddr string, relayURLs []string, eventChan <-chan *nostr.Event, done chan bool, verbose bool) {
	defer func() { done <- true }()

	if verbose {
		log.Printf("Server: Starting session %s with client %s", sessionID, clientPubkey)
	}

	// Connect to target
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Server: Session %s - Failed to connect to target %s: %v", sessionID, targetAddr, err)
		return
	}
	defer targetConn.Close()

	if verbose {
		log.Printf("Server: Session %s - Connected to target %s", sessionID, targetAddr)
	}

	// Create relay handler for this session's responses
	relayHandler, err := NewNostrRelayHandler(relayURLs, keyMgr, verbose)
	if err != nil {
		log.Printf("Server: Session %s - Failed to create relay handler: %v", sessionID, err)
		return
	}
	defer relayHandler.Close()

	// Start goroutine to read responses from target
	targetDone := make(chan bool, 1)
	go readTargetNostrResponses(relayHandler, keyMgr, sessionID, clientPubkey, targetConn, targetDone, verbose)

	processedSequences := make(map[uint64]bool)
	nextExpectedSequence := uint64(1)                // Start at 1 since open packet (seq 0) was already handled
	pendingPackets := make(map[uint64]*ParsedPacket) // Buffer for out-of-order packets

	// Mark the open packet (seq 0) as already processed
	processedSequences[0] = true

	// Handle incoming packets for this session
	for {
		select {
		case <-targetDone:
			if verbose {
				log.Printf("Server: Session %s - Target connection closed", sessionID)
			}
			return
		case event := <-eventChan:
			// Events from this channel are already filtered for this session

			// Version compatibility is now checked in UnwrapEphemeralGiftWrap

			parsedPacket, err := keyMgr.UnwrapEphemeralGiftWrap(event)
			if err != nil {
				if verbose {
					log.Printf("Server: Session %s - Error unwrapping encrypted packet: %v", sessionID, err)
				}
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
					log.Printf("Server: Session %s - Buffering out-of-order packet seq %d (expecting %d)", sessionID, parsedPacket.Sequence, nextExpectedSequence)
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
					// Write data to target connection
					if len(pkt.Packet.Data) > 0 {
						if _, writeErr := targetConn.Write(pkt.Packet.Data); writeErr != nil {
							log.Printf("Server: Session %s - Error writing to target: %v", sessionID, writeErr)
							return
						}

						if verbose {
							log.Printf("Server: Session %s - Forwarded %d bytes to target (seq %d)", sessionID, len(pkt.Packet.Data), pkt.Sequence)
						}
					}

				case PacketTypeClose:
					if verbose {
						log.Printf("Server: Session %s - Received close packet from client", sessionID)
					}
					return
				}

				// Update next expected sequence
				nextExpectedSequence = pkt.Sequence + 1
			}
		}
	}
}

func readTargetNostrResponses(relayHandler *NostrRelayHandler, keyMgr *KeyManager, sessionID, clientPubkey string, targetConn net.Conn, done chan bool, verbose bool) {
	defer func() { done <- true }()

	sequence := uint64(0)         // Server starts its own sequence at 0
	buffer := make([]byte, 32768) // Increased from 4KB to 32KB for better throughput
	// This reduces the number of Nostr events by 8x, significantly improving performance with remote relays

	for {
		n, err := targetConn.Read(buffer)
		if err != nil {
			if verbose {
				log.Printf("Server: Session %s - Target connection closed: %v", sessionID, err)
			}
			break
		}

		if n > 0 {
			// Create data packet
			dataPacket := CreateDataPacket(buffer[:n])
			if err := SendNostrPacket(relayHandler, keyMgr, dataPacket, clientPubkey, PacketTypeData, sessionID, sequence, "server_to_client", "", 0, "", "", verbose); err != nil {
				log.Printf("Server: Session %s - Failed to send encrypted data packet: %v", sessionID, err)
				break
			}

			if verbose {
				log.Printf("Server: Session %s - Sent %d bytes to client via encrypted event (seq %d)", sessionID, n, sequence)
			}
			sequence++
		}
	}

	// Send close packet synchronously to ensure proper cleanup
	closePacket := CreateEmptyPacket()
	if err := SendNostrPacketSync(relayHandler, keyMgr, closePacket, clientPubkey, PacketTypeClose, sessionID, sequence, "server_to_client", "", 0, "", "", verbose); err != nil {
		log.Printf("Server: Session %s - Failed to send encrypted close packet: %v", sessionID, err)
	}

	if verbose {
		log.Printf("Server: Session %s - Sent encrypted close packet to client", sessionID)
	}
}
