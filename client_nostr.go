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

	// Read data from client connection with batching for better performance
	sequence := uint64(1) // Start at 1 (open packet is 0)
	readClientDataWithBatching(conn, relayHandler, keyMgr, serverPubkeyHex, sessionID, &sequence, clientAddr, verbose)

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

// readClientDataWithBatching reads data from client connection with intelligent batching
func readClientDataWithBatching(conn net.Conn, relayHandler *NostrRelayHandler, keyMgr *KeyManager, serverPubkeyHex, sessionID string, sequence *uint64, clientAddr string, verbose bool) {
	const (
		maxBatchSize = 16384 // 16KB batch size
		batchTimeout = 50 * time.Millisecond
	)
	
	buffer := make([]byte, 32768) // 32KB read buffer
	batchBuffer := make([]byte, 0, maxBatchSize) // Batch accumulation buffer
	timer := time.NewTimer(batchTimeout)
	timer.Stop() // Start stopped
	
	defer timer.Stop()
	
	for {
		// Set read deadline to allow for batching
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		
		n, err := conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout - send any accumulated data
				if len(batchBuffer) > 0 {
					sendBatchedData(relayHandler, keyMgr, serverPubkeyHex, sessionID, sequence, batchBuffer, clientAddr, verbose)
					batchBuffer = batchBuffer[:0] // Reset buffer
				}
				continue
			}
			
			if err != io.EOF {
				if verbose {
					log.Printf("Client: Session %s - Connection read error: %v", sessionID, err)
				}
			}
			break
		}
		
		if n > 0 {
			// Add data to batch
			batchBuffer = append(batchBuffer, buffer[:n]...)
			
			// Start timer if this is the first data in batch
			if len(batchBuffer) == n {
				timer.Reset(batchTimeout)
			}
			
			// Send batch if it's full
			if len(batchBuffer) >= maxBatchSize {
				sendBatchedData(relayHandler, keyMgr, serverPubkeyHex, sessionID, sequence, batchBuffer, clientAddr, verbose)
				batchBuffer = batchBuffer[:0] // Reset buffer
				timer.Stop()
			}
		}
		
		// Check for timeout
		select {
		case <-timer.C:
			if len(batchBuffer) > 0 {
				sendBatchedData(relayHandler, keyMgr, serverPubkeyHex, sessionID, sequence, batchBuffer, clientAddr, verbose)
				batchBuffer = batchBuffer[:0] // Reset buffer
			}
		default:
			// Continue reading
		}
	}
	
	// Send any remaining data
	if len(batchBuffer) > 0 {
		sendBatchedData(relayHandler, keyMgr, serverPubkeyHex, sessionID, sequence, batchBuffer, clientAddr, verbose)
	}
}

// sendBatchedData sends accumulated data as a single packet
func sendBatchedData(relayHandler *NostrRelayHandler, keyMgr *KeyManager, serverPubkeyHex, sessionID string, sequence *uint64, data []byte, clientAddr string, verbose bool) {
	if len(data) == 0 {
		return
	}
	
	// Create data packet with batched data
	dataPacket := CreateDataPacket(data)
	if err := SendNostrPacket(relayHandler, keyMgr, dataPacket, serverPubkeyHex, PacketTypeData, sessionID, *sequence, "client_to_server", "", 0, clientAddr, "", verbose); err != nil {
		log.Printf("Client: Failed to send batched data packet: %v", err)
		return
	}
	
	if verbose {
		log.Printf("Client: Session %s - Sent %d bytes in batched packet (seq %d)", sessionID, len(data), *sequence)
	}
	*sequence++
}
