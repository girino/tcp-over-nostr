package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

func runClientNostr(clientPort int, nostrDir, serverPubkey, keysFile string, verbose bool) {
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
	fmt.Printf("  Events directory: %s\n", nostrDir)
	fmt.Printf("  Keys file: %s\n", keysFile)
	fmt.Printf("  Verbose logging: %t\n\n", verbose)

	// Initialize key manager
	keyMgr := NewKeyManager(keysFile, verbose)
	if err := keyMgr.LoadOrGenerateKeys(); err != nil {
		log.Fatalf("Failed to initialize keys: %v", err)
	}

	clientKeys := keyMgr.GetKeys()
	fmt.Printf("Client Nostr pubkey: %s\n\n", clientKeys.PublicKey)

	// Initialize Nostr event handler
	eventHandler := NewNostrEventHandler(nostrDir, keyMgr, verbose)

	// Process old events on startup - mark events older than startup time as processed
	startupTime := time.Now()
	if verbose {
		log.Printf("Client: Processing old events from before startup at %v", startupTime)
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
		go handleClientConnectionNostr(conn, eventHandler, serverPubkey, clientKeys.PublicKey, startupTime, verbose)
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

func handleClientConnectionNostr(conn net.Conn, eventHandler *NostrEventHandler, serverPubkey, clientPubkey string, startupTime time.Time, verbose bool) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	sessionID := fmt.Sprintf("session_%d_%s", time.Now().UnixNano(), clientAddr)
	sessionID = sanitizeSessionID(sessionID)

	if verbose {
		log.Printf("Client: Starting Nostr session %s for %s", sessionID, clientAddr)
	}

	// Send open packet
	openPacket := CreateOpenPacket(sessionID, "client_to_server", "", 0, clientAddr)
	if err := sendNostrPacket(eventHandler, openPacket, serverPubkey, verbose); err != nil {
		log.Printf("Client: Failed to send open packet: %v", err)
		return
	}

	// Start goroutine to read server responses
	done := make(chan bool, 2)
	go readServerNostrResponses(eventHandler, sessionID, clientPubkey, conn, startupTime, done, verbose)

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
			if err := sendNostrPacket(eventHandler, dataPacket, serverPubkey, verbose); err != nil {
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
	if err := sendNostrPacket(eventHandler, closePacket, serverPubkey, verbose); err != nil {
		log.Printf("Client: Failed to send close packet: %v", err)
	}

	done <- true
	if verbose {
		log.Printf("Client: Session %s closed", sessionID)
	}
}

func readServerNostrResponses(eventHandler *NostrEventHandler, sessionID, clientPubkey string, conn net.Conn, startupTime time.Time, done chan bool, verbose bool) {
	defer func() { done <- true }()

	processedSequences := make(map[uint64]bool)
	processedFiles := make(map[string]bool) // Track processed filenames
	nextExpectedSequence := uint64(0)

	for {
		select {
		case <-done:
			return
		default:
			// Check for new events
			eventFiles, err := eventHandler.GetEventFiles(clientPubkey, startupTime)
			if err != nil {
				if verbose {
					log.Printf("Client: Error getting event files: %v", err)
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			processedAny := false
			sessionClosed := false

			for _, filename := range eventFiles {
				// Skip if this file has already been processed
				if processedFiles[filename] {
					continue
				}

				if processedSequences[nextExpectedSequence] {
					nextExpectedSequence++
					continue
				}

				event, err := eventHandler.ReadEvent(filename)
				if err != nil {
					if verbose {
						log.Printf("Client: Error reading event file %s: %v", filename, err)
					}
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

				// Check sequence order
				if packet.Sequence != nextExpectedSequence {
					continue
				}

				// Process packet based on type
				switch packet.Type {
				case PacketTypeData:
					// Write data to client connection
					if len(packet.Data) > 0 {
						data, err := packet.GetData()
						if err != nil {
							log.Printf("Client: Session %s - Error decoding packet data: %v", sessionID, err)
							continue
						}

						if _, writeErr := conn.Write(data); writeErr != nil {
							if verbose {
								log.Printf("Client: Session %s - Error writing to client: %v", sessionID, writeErr)
							}
							return
						}

						if verbose {
							log.Printf("Client: Session %s - Wrote %d bytes to client (seq %d)", sessionID, len(data), packet.Sequence)
						}
					}

				case PacketTypeClose:
					if verbose {
						log.Printf("Client: Session %s - Received server close packet", sessionID)
					}
					sessionClosed = true

				case PacketTypeHeartbeat:
					if verbose {
						log.Printf("Client: Session %s - Received server heartbeat", sessionID)
					}
				}

				// Mark sequence as processed
				processedSequences[packet.Sequence] = true
				processedFiles[filename] = true // Mark file as processed
				if packet.Type != PacketTypeHeartbeat {
					nextExpectedSequence++
				}
				processedAny = true

				// Event processed successfully (keeping file for history)

				if sessionClosed {
					return
				}
			}

			if !processedAny {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
}

func sendNostrPacket(eventHandler *NostrEventHandler, packet *Packet, targetPubkey string, verbose bool) error {
	// Create Nostr event for the packet
	event, err := eventHandler.keyMgr.CreateNostrEvent(packet, targetPubkey)
	if err != nil {
		return fmt.Errorf("failed to create Nostr event: %v", err)
	}

	// Write event to disk
	if err := eventHandler.WriteEvent(event); err != nil {
		return fmt.Errorf("failed to write Nostr event: %v", err)
	}

	if verbose {
		log.Printf("Nostr: Sent packet %s as event %s", packet.ID, event.ID)
	}

	return nil
}
