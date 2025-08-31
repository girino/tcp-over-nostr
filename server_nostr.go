package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

func runServerNostr(targetHost string, targetPort int, nostrDir, keysFile string, verbose bool) {
	// Validate inputs
	if targetPort < 1 || targetPort > 65535 {
		log.Fatal("Target port must be between 1 and 65535")
	}

	targetAddr := fmt.Sprintf("%s:%d", targetHost, targetPort)

	fmt.Printf("Starting TCP proxy server (Nostr mode):\n")
	fmt.Printf("  Target: %s\n", targetAddr)
	fmt.Printf("  Events directory: %s\n", nostrDir)
	fmt.Printf("  Keys file: %s\n", keysFile)
	fmt.Printf("  Verbose logging: %t\n\n", verbose)

	// Initialize key manager
	keyMgr := NewKeyManager(keysFile, verbose)
	if err := keyMgr.LoadOrGenerateKeys(); err != nil {
		log.Fatalf("Failed to initialize keys: %v", err)
	}

	serverKeys := keyMgr.GetKeys()
	fmt.Printf("Server Nostr pubkey: %s\n", serverKeys.PublicKey)
	fmt.Printf("Share this pubkey with clients using -server-key parameter\n\n")

	// Initialize Nostr event handler
	eventHandler := NewNostrEventHandler(nostrDir, keyMgr, verbose)

	// Process old events on startup - mark events older than startup time as processed
	startupTime := time.Now()
	if verbose {
		log.Printf("Server: Processing old events from before startup at %v", startupTime)
	}

	fmt.Printf("TCP proxy server started successfully. Monitoring for Nostr events...\n\n")

	// Monitor for new session open events
	monitorNostrSessionEvents(eventHandler, serverKeys.PublicKey, targetAddr, startupTime, verbose)
}

func monitorNostrSessionEvents(eventHandler *NostrEventHandler, serverPubkey, targetAddr string, startupTime time.Time, verbose bool) {
	processedSessions := make(map[string]bool)
	processedEventFiles := make(map[string]bool) // Track processed filenames

	// First, scan all existing events and mark old ones as processed
	allEvents, err := eventHandler.GetAllEventFiles(serverPubkey)
	if err == nil {
		for _, filename := range allEvents {
			event, err := eventHandler.ReadEvent(filename)
			if err != nil {
				continue
			}

			// If event is older than startup time, mark as processed
			eventTime := time.Unix(int64(event.CreatedAt), 0)
			if eventTime.Before(startupTime) {
				processedEventFiles[filename] = true
				if verbose {
					log.Printf("Server: Marking old event %s (created %v) as already processed", event.ID, eventTime)
				}
			}
		}
		if verbose {
			log.Printf("Server: Marked %d existing events as already processed", len(processedEventFiles))
		}
	}

	for {
		// Find all events in the directory
		eventFiles, err := eventHandler.GetEventFiles(serverPubkey, startupTime)
		if err != nil {
			if verbose {
				log.Printf("Server: Error getting event files: %v", err)
			}
			time.Sleep(1 * time.Second)
			continue
		}

		for _, filename := range eventFiles {
			// Skip if already processed
			if processedEventFiles[filename] {
				continue
			}

			event, err := eventHandler.ReadEvent(filename)
			if err != nil {
				if verbose {
					log.Printf("Server: Error reading event file %s: %v", filename, err)
				}
				processedEventFiles[filename] = true
				continue
			}

			// Parse packet from event
			packet, err := ParseNostrEvent(event)
			if err != nil {
				if verbose {
					log.Printf("Server: Error parsing packet from event: %v", err)
				}
				processedEventFiles[filename] = true
				continue
			}

			// Only process client_to_server open packets to start new sessions
			if packet.Type == PacketTypeOpen && packet.Direction == "client_to_server" {
				sessionID := packet.SessionID
				if !processedSessions[sessionID] {
					processedSessions[sessionID] = true
					processedEventFiles[filename] = true

					if verbose {
						log.Printf("Server: Found new session %s", sessionID)
					}

					// Handle this session in a goroutine
					go handleServerNostrSession(eventHandler, sessionID, event.PubKey, targetAddr, startupTime, verbose)
				} else {
					// Session already being handled, mark file as processed
					processedEventFiles[filename] = true
				}
			} else {
				// Mark non-session-starting events as processed
				processedEventFiles[filename] = true
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func handleServerNostrSession(eventHandler *NostrEventHandler, sessionID, clientPubkey, targetAddr string, startupTime time.Time, verbose bool) {
	if verbose {
		log.Printf("Server: Session %s - Starting event processing", sessionID)
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

	// Start goroutine to read from target and send responses
	done := make(chan bool, 2)
	go readTargetNostrResponses(eventHandler, sessionID, clientPubkey, targetConn, done, verbose)

	// Process client packets in sequence
	processedSequences := make(map[uint64]bool)
	processedFiles := make(map[string]bool) // Track processed filenames
	nextExpectedSequence := uint64(0)
	sessionActive := true

	go func() {
		defer func() { done <- true }()

		for sessionActive {
			// Get events for this session
			eventFiles, err := eventHandler.GetEventFiles(eventHandler.keyMgr.GetKeys().PublicKey, startupTime)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			processedAny := false

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
					continue
				}

				// Parse packet from event - must be from the correct client
				if event.PubKey != clientPubkey {
					continue
				}

				packet, err := ParseNostrEvent(event)
				if err != nil {
					continue
				}

				// Check if this packet belongs to our session and is client_to_server
				if packet.SessionID != sessionID || packet.Direction != "client_to_server" {
					continue
				}

				// Check sequence order
				if packet.Sequence != nextExpectedSequence {
					continue
				}

				// Process packet based on type
				switch packet.Type {
				case PacketTypeOpen:
					if verbose {
						log.Printf("Server: Session %s - Processing open packet", sessionID)
					}

				case PacketTypeData:
					if len(packet.Data) > 0 {
						data, err := packet.GetData()
						if err != nil {
							log.Printf("Server: Session %s - Error decoding packet data: %v", sessionID, err)
							continue
						}

						if _, writeErr := targetConn.Write(data); writeErr != nil {
							if verbose {
								log.Printf("Server: Session %s - Error writing to target: %v", sessionID, writeErr)
							}
							return
						}

						if verbose {
							log.Printf("Server: Session %s - Sent %d bytes to target (seq %d)", sessionID, len(data), packet.Sequence)
						}
					}

				case PacketTypeClose:
					if verbose {
						log.Printf("Server: Session %s - Received close packet: %s", sessionID, packet.ErrorMsg)
					}
					sessionActive = false

					// Event processed successfully (keeping file for history)
					return

				case PacketTypeHeartbeat:
					// Respond to heartbeat by sending one back to client
					heartbeatPacket := CreateHeartbeatPacket(sessionID, "server_to_client")
					if err := sendNostrPacket(eventHandler, heartbeatPacket, clientPubkey, verbose); err != nil && verbose {
						log.Printf("Server: Session %s - Error sending heartbeat response: %v", sessionID, err)
					}
					if verbose {
						log.Printf("Server: Session %s - Responded to heartbeat", sessionID)
					}
				}

				processedSequences[packet.Sequence] = true
				processedFiles[filename] = true // Mark file as processed
				nextExpectedSequence++
				processedAny = true

				// Event processed successfully (keeping file for history)
			}

			if !processedAny {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	// Wait for either direction to finish
	<-done
	sessionActive = false

	if verbose {
		log.Printf("Server: Session %s - Connection closed", sessionID)
	}
}

func readTargetNostrResponses(eventHandler *NostrEventHandler, sessionID, clientPubkey string, targetConn net.Conn, done chan bool, verbose bool) {
	defer func() { done <- true }()

	sequence := uint64(0)
	buffer := make([]byte, 4096)

	for {
		n, err := targetConn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				if verbose {
					log.Printf("Server: Session %s - Target read error: %v", sessionID, err)
				}
			} else {
				if verbose {
					log.Printf("Server: Session %s - Target disconnected", sessionID)
				}
			}

			// Send close packet to client
			closePacket := CreateClosePacket(sessionID, "server_to_client", sequence, "target disconnected")
			if err := sendNostrPacket(eventHandler, closePacket, clientPubkey, verbose); err != nil {
				log.Printf("Server: Session %s - Error sending close packet: %v", sessionID, err)
			}
			return
		}

		if n > 0 {
			// Create data packet and send to client
			dataPacket := CreateDataPacket(sessionID, "server_to_client", sequence, buffer[:n])
			if err := sendNostrPacket(eventHandler, dataPacket, clientPubkey, verbose); err != nil {
				log.Printf("Server: Session %s - Error sending data packet: %v", sessionID, err)
				return
			}

			if verbose {
				log.Printf("Server: Session %s - Sent %d bytes to client via event (seq %d)", sessionID, n, sequence)
			}
			sequence++
		}
	}
}
