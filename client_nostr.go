package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

func runClientNostr(clientPort int, relayURL, serverPubkey, keysFile string, verbose bool) {
	// Show startup banner
	fmt.Print(GetBanner())

	// Validate inputs
	if clientPort < 1 || clientPort > 65535 {
		log.Fatal("Client port must be between 1 and 65535")
	}

	if serverPubkey == "" {
		log.Fatal("Server public key is required for Nostr mode")
	}

	fmt.Printf("Starting TCP proxy client (Nostr mode):\n")
	fmt.Printf("  Listen port: %d\n", clientPort)
	fmt.Printf("  Server pubkey: %s\n", serverPubkey)
	fmt.Printf("  Relay URL: %s\n", relayURL)
	fmt.Printf("  Keys file: %s\n", keysFile)
	fmt.Printf("  Verbose logging: %t\n\n", verbose)

	// Initialize key manager
	keyMgr := NewKeyManager(keysFile)
	if err := keyMgr.LoadKeys(); err != nil {
		log.Fatalf("Failed to load/generate keys: %v", err)
	}

	clientKeys := keyMgr.GetKeys()
	fmt.Printf("Client Nostr pubkey: %s\n\n", clientKeys.PublicKey)

	// Initialize relay handler
	relayHandler, err := NewNostrRelayHandler(relayURL, keyMgr, verbose)
	if err != nil {
		log.Fatalf("Failed to connect to relay: %v", err)
	}
	defer relayHandler.Close()

	// Subscribe to events from the server
	if err := relayHandler.SubscribeToEvents(clientKeys.PublicKey); err != nil {
		log.Fatalf("Failed to subscribe to events: %v", err)
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
		go handleClientConnectionNostr(conn, relayHandler, keyMgr, serverPubkey, clientKeys.PublicKey, verbose)
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

func handleClientConnectionNostr(conn net.Conn, relayHandler *NostrRelayHandler, keyMgr *KeyManager, serverPubkey, clientPubkey string, verbose bool) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	sessionID := fmt.Sprintf("session_%d_%s", time.Now().UnixNano(), clientAddr)
	sessionID = sanitizeSessionID(sessionID)

	if verbose {
		log.Printf("Client: Starting Nostr session %s for %s", sessionID, clientAddr)
	}

	// Send open packet
	openPacket := CreateOpenPacket(sessionID, "client_to_server", "", 0, clientAddr)
	if err := sendNostrPacket(relayHandler, keyMgr, openPacket, serverPubkey, verbose); err != nil {
		log.Printf("Client: Failed to send open packet: %v", err)
		return
	}

	// Start goroutine to read server responses
	done := make(chan bool, 2)
	go readServerNostrResponses(relayHandler, sessionID, clientPubkey, conn, done, verbose)

	// Read data from client connection and send as packets
	sequence := uint64(1) // Start at 1 (open packet is 0)
	buffer := make([]byte, 4096)

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
			dataPacket := CreateDataPacket(sessionID, "client_to_server", sequence, buffer[:n])
			if err := sendNostrPacket(relayHandler, keyMgr, dataPacket, serverPubkey, verbose); err != nil {
				log.Printf("Client: Failed to send data packet: %v", err)
				break
			}

			if verbose {
				log.Printf("Client: Session %s - Sent %d bytes in packet (seq %d)", sessionID, n, sequence)
			}
			sequence++
		}
	}

	// Send close packet
	closePacket := CreateClosePacket(sessionID, "client_to_server", sequence, "")
	if err := sendNostrPacket(relayHandler, keyMgr, closePacket, serverPubkey, verbose); err != nil {
		log.Printf("Client: Failed to send close packet: %v", err)
	}

	done <- true
	if verbose {
		log.Printf("Client: Session %s closed", sessionID)
	}
}

func readServerNostrResponses(relayHandler *NostrRelayHandler, sessionID, clientPubkey string, conn net.Conn, done chan bool, verbose bool) {
	defer func() { done <- true }()

	processedSequences := make(map[uint64]bool)
	nextExpectedSequence := uint64(0)
	pendingPackets := make(map[uint64]*Packet) // Buffer for out-of-order packets

	for {
		select {
		case <-done:
			return
		case event := <-relayHandler.GetEventChannel():
			// Check if this event is for us
			if !IsEventForMe(event, clientPubkey) {
				continue
			}

			// Parse packet from event
			packet, err := ParseNostrEvent(event)
			if err != nil {
				if verbose {
					log.Printf("Client: Error parsing packet from event: %v", err)
				}
				continue
			}

			// Check if this packet belongs to our session
			if packet.SessionID != sessionID {
				continue
			}

			// Check direction - we want server_to_client packets
			if packet.Direction != "server_to_client" {
				continue
			}

			// Skip if already processed
			if processedSequences[packet.Sequence] {
				continue
			}

			// Check sequence order - if not the next expected, buffer it
			if packet.Sequence != nextExpectedSequence {
				pendingPackets[packet.Sequence] = packet
				if verbose {
					log.Printf("Client: Session %s - Buffering out-of-order packet seq %d (expecting %d)", sessionID, packet.Sequence, nextExpectedSequence)
				}
				continue
			}

			// Process this packet and any consecutive buffered packets
			packetsToProcess := []*Packet{packet}

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
					if len(pkt.Data) > 0 {
						data, err := pkt.GetData()
						if err != nil {
							log.Printf("Client: Session %s - Error decoding packet data: %v", sessionID, err)
							continue
						}

						if _, writeErr := conn.Write(data); writeErr != nil {
							log.Printf("Client: Session %s - Error writing to connection: %v", sessionID, writeErr)
							return
						}

						if verbose {
							log.Printf("Client: Session %s - Received %d bytes from server (seq %d)", sessionID, len(data), pkt.Sequence)
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

func sendNostrPacket(relayHandler *NostrRelayHandler, keyMgr *KeyManager, packet *Packet, targetPubkey string, verbose bool) error {
	// Create Nostr event for the packet
	event, err := keyMgr.CreateNostrEvent(packet, targetPubkey)
	if err != nil {
		return fmt.Errorf("failed to create Nostr event: %v", err)
	}

	// Publish event to relay
	if err := relayHandler.PublishEvent(event); err != nil {
		return fmt.Errorf("failed to publish Nostr event: %v", err)
	}

	if verbose {
		log.Printf("Nostr: Sent packet %s as event %s", packet.ID, event.ID)
	}

	return nil
}
