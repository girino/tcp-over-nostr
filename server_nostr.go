package main

import (
	"fmt"
	"log"
	"net"
)

func runServerNostr(targetHost string, targetPort int, relayURL, keysFile string, verbose bool) {
	// Validate inputs
	if targetPort < 1 || targetPort > 65535 {
		log.Fatal("Target port must be between 1 and 65535")
	}

	targetAddr := fmt.Sprintf("%s:%d", targetHost, targetPort)

	fmt.Printf("Starting TCP proxy server (Nostr mode):\n")
	fmt.Printf("  Target: %s\n", targetAddr)
	fmt.Printf("  Relay URL: %s\n", relayURL)
	fmt.Printf("  Keys file: %s\n", keysFile)
	fmt.Printf("  Verbose logging: %t\n\n", verbose)

	// Initialize key manager
	keyMgr := NewKeyManager(keysFile)
	if err := keyMgr.LoadKeys(); err != nil {
		log.Fatalf("Failed to load/generate keys: %v", err)
	}

	serverKeys := keyMgr.GetKeys()
	fmt.Printf("Server Nostr pubkey: %s\n", serverKeys.PublicKey)
	fmt.Printf("Share this pubkey with clients using -server-key parameter\n\n")

	// Initialize relay handler
	relayHandler, err := NewNostrRelayHandler(relayURL, keyMgr, verbose)
	if err != nil {
		log.Fatalf("Failed to connect to relay: %v", err)
	}
	defer relayHandler.Close()

	// Subscribe to events for this server
	if err := relayHandler.SubscribeToEvents(serverKeys.PublicKey); err != nil {
		log.Fatalf("Failed to subscribe to events: %v", err)
	}

	fmt.Printf("TCP proxy server started successfully. Monitoring for Nostr events...\n\n")

	// Monitor for new session events
	monitorNostrSessionEvents(relayHandler, keyMgr, serverKeys.PublicKey, targetAddr, verbose)
}

func monitorNostrSessionEvents(relayHandler *NostrRelayHandler, keyMgr *KeyManager, serverPubkey, targetAddr string, verbose bool) {
	activeSessions := make(map[string]chan bool) // sessionID -> done channel

	for {
		select {
		case event := <-relayHandler.GetEventChannel():
			// Check if this event is for us
			if !IsEventForMe(event, serverPubkey) {
				continue
			}

			// Parse packet from event
			packet, err := ParseNostrEvent(event)
			if err != nil {
				if verbose {
					log.Printf("Server: Error parsing packet from event: %v", err)
				}
				continue
			}

			// Only handle client_to_server packets
			if packet.Direction != "client_to_server" {
				continue
			}

			// Check if this is an open packet for a new session
			if packet.Type == PacketTypeOpen {
				// Check if we already have this session
				if _, exists := activeSessions[packet.SessionID]; exists {
					continue // Session already active
				}

				if verbose {
					log.Printf("Server: New session %s from client", packet.SessionID)
				}

				// Start new session handler
				done := make(chan bool)
				activeSessions[packet.SessionID] = done
				go handleServerNostrSession(relayHandler, keyMgr, packet.SessionID, event.PubKey, targetAddr, done, verbose)

				// Clean up when session is done
				go func(sessionID string, doneChan chan bool) {
					<-doneChan
					delete(activeSessions, sessionID)
					if verbose {
						log.Printf("Server: Session %s completed and cleaned up", sessionID)
					}
				}(packet.SessionID, done)
			}
		}
	}
}

func handleServerNostrSession(relayHandler *NostrRelayHandler, keyMgr *KeyManager, sessionID, clientPubkey, targetAddr string, done chan bool, verbose bool) {
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

	// Start goroutine to read responses from target
	targetDone := make(chan bool, 1)
	go readTargetNostrResponses(relayHandler, keyMgr, sessionID, clientPubkey, targetConn, targetDone, verbose)

	processedSequences := make(map[uint64]bool)

	// Handle incoming packets for this session
	for {
		select {
		case <-targetDone:
			if verbose {
				log.Printf("Server: Session %s - Target connection closed", sessionID)
			}
			return
		case event := <-relayHandler.GetEventChannel():
			// Check if this event is for us
			if !IsEventForMe(event, keyMgr.GetKeys().PublicKey) {
				continue
			}

			// Parse packet from event
			packet, err := ParseNostrEvent(event)
			if err != nil {
				continue
			}

			// Check if this packet belongs to our session
			if packet.SessionID != sessionID {
				continue
			}

			// Check direction - we want client_to_server packets
			if packet.Direction != "client_to_server" {
				continue
			}

			// Skip if already processed
			if processedSequences[packet.Sequence] {
				continue
			}

			// Mark as processed
			processedSequences[packet.Sequence] = true

			// Process packet based on type
			switch packet.Type {
			case PacketTypeData:
				// Write data to target connection
				if len(packet.Data) > 0 {
					data, err := packet.GetData()
					if err != nil {
						log.Printf("Server: Session %s - Error decoding packet data: %v", sessionID, err)
						continue
					}

					if _, writeErr := targetConn.Write(data); writeErr != nil {
						log.Printf("Server: Session %s - Error writing to target: %v", sessionID, writeErr)
						return
					}

					if verbose {
						log.Printf("Server: Session %s - Forwarded %d bytes to target (seq %d)", sessionID, len(data), packet.Sequence)
					}
				}

			case PacketTypeClose:
				if verbose {
					log.Printf("Server: Session %s - Received close packet from client", sessionID)
				}
				return
			}
		}
	}
}

func readTargetNostrResponses(relayHandler *NostrRelayHandler, keyMgr *KeyManager, sessionID, clientPubkey string, targetConn net.Conn, done chan bool, verbose bool) {
	defer func() { done <- true }()

	sequence := uint64(1) // Start at 1 (open packet is 0)
	buffer := make([]byte, 4096)

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
			dataPacket := CreateDataPacket(sessionID, "server_to_client", sequence, buffer[:n])
			if err := sendNostrPacket(relayHandler, keyMgr, dataPacket, clientPubkey, verbose); err != nil {
				log.Printf("Server: Session %s - Failed to send data packet: %v", sessionID, err)
				break
			}

			if verbose {
				log.Printf("Server: Session %s - Sent %d bytes to client via event (seq %d)", sessionID, n, sequence)
			}
			sequence++
		}
	}

	// Send close packet
	closePacket := CreateClosePacket(sessionID, "server_to_client", sequence, "")
	if err := sendNostrPacket(relayHandler, keyMgr, closePacket, clientPubkey, verbose); err != nil {
		log.Printf("Server: Session %s - Failed to send close packet: %v", sessionID, err)
	}

	if verbose {
		log.Printf("Server: Session %s - Sent close packet to client", sessionID)
	}
}


