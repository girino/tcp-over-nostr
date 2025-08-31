package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

func runClientPackets(clientPort int, packetDir string, verbose bool) {
	// Validate inputs
	if clientPort < 1 || clientPort > 65535 {
		log.Fatal("Client port must be between 1 and 65535")
	}

	listenAddr := fmt.Sprintf(":%d", clientPort)

	fmt.Printf("Starting TCP proxy client (packet mode):\n")
	fmt.Printf("  Listening on: %s\n", listenAddr)
	fmt.Printf("  Packet directory: %s\n", packetDir)
	fmt.Printf("  Verbose logging: %v\n", verbose)

	// Ensure packet directory exists
	if err := os.MkdirAll(packetDir, 0755); err != nil {
		log.Fatalf("Failed to create packet directory %s: %v", packetDir, err)
	}

	// Process old packet files on startup - mark packets older than startup time as processed
	startupTime := time.Now()
	if verbose {
		log.Printf("Client: Processing old packet files from before startup at %v", startupTime)
	}
	
	// This will be used by packet handlers to ignore old packets
	_ = startupTime

	// Start listening
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", listenAddr, err)
	}
	defer listener.Close()

	fmt.Printf("TCP proxy client started successfully. Press Ctrl+C to stop.\n\n")

	// Accept connections
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			if verbose {
				log.Printf("Failed to accept connection: %v", err)
			}
			continue
		}

		// Handle each connection in a goroutine
		go handleClientConnectionPackets(clientConn, packetDir, verbose)
	}
}

func handleClientConnectionPackets(clientConn net.Conn, packetDir string, verbose bool) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	if verbose {
		log.Printf("Client: New connection from %s", clientAddr)
	}

	// Create session ID based on connection time and address
	sessionID := fmt.Sprintf("session_%d_%s", time.Now().UnixNano(), clientAddr)
	sessionID = sanitizeSessionID(sessionID)

	if verbose {
		log.Printf("Client: Session %s started", sessionID)
	}

	// Create packet handler
	packetHandler := NewPacketHandler(packetDir, sessionID, verbose)

	// Send open packet
	openPacket := CreateOpenPacket(sessionID, "client_to_server", "", 0, clientAddr)
	if err := packetHandler.WritePacket(openPacket); err != nil {
		if verbose {
			log.Printf("Client: Failed to write open packet for session %s: %v", sessionID, err)
		}
		return
	}

	done := make(chan bool, 2)
	var sequence uint64 = 1

	// Start goroutine to read server responses and send to client
	go func() {
		defer func() { done <- true }()

		processedSequences := make(map[uint64]bool)
		var nextExpectedSequence uint64 = 1 // Server responses start from 1

		for {
			// Get all server response packets for this session
			files, err := packetHandler.GetPacketFiles("server_to_client")
			if err != nil {
				if verbose {
					log.Printf("Client: Session %s - Error getting response packet files: %v", sessionID, err)
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}

			processedAny := false
			sessionClosed := false

			// Process packets in sequence order
			for _, filename := range files {
				packet, err := packetHandler.ReadPacket(filename)
				if err != nil {
					if verbose {
						log.Printf("Client: Session %s - Error reading response packet %s: %v", sessionID, filename, err)
					}
					continue
				}

				// Skip already processed packets
				if processedSequences[packet.Sequence] {
					continue
				}

				// Process packets in sequence order (except heartbeats)
				if packet.Type == PacketTypeHeartbeat || packet.Sequence == nextExpectedSequence {
					switch packet.Type {
					case PacketTypeData:
						data, err := packet.GetData()
						if err != nil {
							if verbose {
								log.Printf("Client: Error decoding data from packet %s: %v", packet.ID, err)
							}
							continue
						}

						if len(data) > 0 {
							_, writeErr := clientConn.Write(data)
							if verbose && writeErr == nil {
								log.Printf("Client: Session %s - Sent %d bytes to client (seq %d)", sessionID, len(data), packet.Sequence)
							}
							if writeErr != nil {
								if verbose {
									log.Printf("Client: Session %s - Error writing to client: %v", sessionID, writeErr)
								}
								return
							}
						}

					case PacketTypeClose:
						if verbose {
							log.Printf("Client: Session %s - Received close packet: %s", sessionID, packet.ErrorMsg)
						}
						sessionClosed = true

					case PacketTypeHeartbeat:
						// Respond to heartbeat if needed
						if verbose {
							log.Printf("Client: Session %s - Received heartbeat", sessionID)
						}
					}

					processedSequences[packet.Sequence] = true
					if packet.Type != PacketTypeHeartbeat {
						nextExpectedSequence++
					}
					processedAny = true

					// Packet processed successfully (keeping file for history)

					if sessionClosed {
						return
					}
				}
			}

			if !processedAny {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	// Read from client and write data packets
	go func() {
		defer func() { done <- true }()

		buffer := make([]byte, 4096)
		for {
			n, err := clientConn.Read(buffer)
			if n > 0 {
				dataPacket := CreateDataPacket(sessionID, "client_to_server", sequence, buffer[:n])
				sequence++

				writeErr := packetHandler.WritePacket(dataPacket)
				if verbose && writeErr == nil {
					log.Printf("Client: Session %s - Wrote %d bytes to packet %s", sessionID, n, dataPacket.ID)
				}
				if writeErr != nil {
					if verbose {
						log.Printf("Client: Session %s - Error writing data packet: %v", sessionID, writeErr)
					}
					return
				}
			}
			if err == io.EOF {
				// Send close packet
				closePacket := CreateClosePacket(sessionID, "client_to_server", sequence, "client disconnected")
				if writeErr := packetHandler.WritePacket(closePacket); writeErr != nil && verbose {
					log.Printf("Client: Session %s - Error writing close packet: %v", sessionID, writeErr)
				}
				if verbose {
					log.Printf("Client: Session %s - Client disconnected", sessionID)
				}
				return
			}
			if err != nil {
				// Send close packet with error
				closePacket := CreateClosePacket(sessionID, "client_to_server", sequence, fmt.Sprintf("read error: %v", err))
				if writeErr := packetHandler.WritePacket(closePacket); writeErr != nil && verbose {
					log.Printf("Client: Session %s - Error writing error close packet: %v", sessionID, writeErr)
				}
				if verbose {
					log.Printf("Client: Session %s - Error reading from client: %v", sessionID, err)
				}
				return
			}
		}
	}()

	// Wait for either direction to finish
	<-done

	if verbose {
		log.Printf("Client: Session %s - Connection closed", sessionID)
	}
}

// sanitizeSessionID removes characters that might cause issues in filenames
func sanitizeSessionID(sessionID string) string {
	// Replace problematic characters with underscores
	result := ""
	for _, char := range sessionID {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' || char == '-' {
			result += string(char)
		} else {
			result += "_"
		}
	}
	return result
}
