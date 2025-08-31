package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runServerPackets(targetHost string, targetPort int, packetDir string, verbose bool) {
	// Validate inputs
	if targetPort < 1 || targetPort > 65535 {
		log.Fatal("Target port must be between 1 and 65535")
	}
	if targetHost == "" {
		log.Fatal("Target host cannot be empty")
	}

	targetAddr := fmt.Sprintf("%s:%d", targetHost, targetPort)

	fmt.Printf("Starting TCP proxy server (packet mode):\n")
	fmt.Printf("  Target: %s\n", targetAddr)
	fmt.Printf("  Packet directory: %s\n", packetDir)
	fmt.Printf("  Verbose logging: %v\n", verbose)

	// Ensure packet directory exists
	if err := os.MkdirAll(packetDir, 0755); err != nil {
		log.Fatalf("Failed to create packet directory %s: %v", packetDir, err)
	}

	fmt.Printf("TCP proxy server started successfully. Monitoring for packets...\n\n")

	// Monitor for new session open packets
	monitorSessionPackets(packetDir, targetAddr, verbose)
}

func monitorSessionPackets(packetDir, targetAddr string, verbose bool) {
	processedSessions := make(map[string]bool)

	for {
		// Find open packets for new sessions
		pattern := filepath.Join(packetDir, "*_client_to_server_*.json")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			if verbose {
				log.Printf("Server: Error globbing packet files: %v", err)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

				for _, filename := range matches {
			// Extract session ID from filename
			// Format: sessionID_direction_sequence_packetID.json
			base := filepath.Base(filename)
			// Remove .json extension
			base = strings.TrimSuffix(base, ".json")
			parts := strings.Split(base, "_")
			
			// Find where client_to_server starts to split properly
			var sessionParts []string
			for i, part := range parts {
				if part == "client" && i+2 < len(parts) && parts[i+1] == "to" && parts[i+2] == "server" {
					sessionParts = parts[:i]
					break
				}
			}
			
			if len(sessionParts) == 0 {
				if verbose {
					log.Printf("Server: Could not extract session ID from %s", filename)
				}
				continue
			}
			
			sessionID := strings.Join(sessionParts, "_")
			
			if !processedSessions[sessionID] {
				processedSessions[sessionID] = true
				
				if verbose {
					log.Printf("Server: Found new session %s", sessionID)
				}
				
				// Handle this session in a goroutine
				go handleServerSessionPackets(sessionID, packetDir, targetAddr, verbose)
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func handleServerSessionPackets(sessionID, packetDir, targetAddr string, verbose bool) {
	if verbose {
		log.Printf("Server: Session %s - Starting packet processing", sessionID)
	}

	// Create packet handler
	packetHandler := NewPacketHandler(packetDir, sessionID, verbose)

	var targetConn net.Conn
	var err error
	var sequence uint64 = 1
	sessionActive := true

	defer func() {
		if targetConn != nil {
			targetConn.Close()
		}
		if err := packetHandler.CleanupSession(); err != nil && verbose {
			log.Printf("Server: Warning: cleanup failed for session %s: %v", sessionID, err)
		}
	}()

	done := make(chan bool, 2)

	// Start goroutine to read from target and write response packets
	go func() {
		defer func() { done <- true }()

		for sessionActive {
			if targetConn == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			buffer := make([]byte, 4096)
			n, err := targetConn.Read(buffer)
			if n > 0 {
				dataPacket := CreateDataPacket(sessionID, "server_to_client", sequence, buffer[:n])
				sequence++

				writeErr := packetHandler.WritePacket(dataPacket)
				if verbose && writeErr == nil {
					log.Printf("Server: Session %s - Wrote %d bytes to packet %s", sessionID, n, dataPacket.ID)
				}
				if writeErr != nil {
					if verbose {
						log.Printf("Server: Session %s - Error writing response packet: %v", sessionID, writeErr)
					}
					return
				}
			}
			if err == io.EOF {
				// Send close packet
				closePacket := CreateClosePacket(sessionID, "server_to_client", sequence, "target disconnected")
				if writeErr := packetHandler.WritePacket(closePacket); writeErr != nil && verbose {
					log.Printf("Server: Session %s - Error writing close packet: %v", sessionID, writeErr)
				}
				if verbose {
					log.Printf("Server: Session %s - Target disconnected", sessionID)
				}
				return
			}
			if err != nil {
				// Send close packet with error
				closePacket := CreateClosePacket(sessionID, "server_to_client", sequence, fmt.Sprintf("target error: %v", err))
				if writeErr := packetHandler.WritePacket(closePacket); writeErr != nil && verbose {
					log.Printf("Server: Session %s - Error writing error close packet: %v", sessionID, writeErr)
				}
				if verbose {
					log.Printf("Server: Session %s - Error reading from target: %v", sessionID, err)
				}
				return
			}
		}
	}()

		// Process client packets in order
	go func() {
		defer func() { done <- true }()
		
		processedSequences := make(map[uint64]bool)
		var nextExpectedSequence uint64 = 0
		
		for sessionActive {
			// Get all client packets for this session
			files, err := packetHandler.GetPacketFiles("client_to_server")
			if err != nil {
				if verbose {
					log.Printf("Server: Session %s - Error getting packet files: %v", sessionID, err)
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}
			
			processedAny := false
			
			// Process packets in sequence order
			for _, filename := range files {
				packet, err := packetHandler.ReadPacket(filename)
				if err != nil {
					if verbose {
						log.Printf("Server: Session %s - Error reading packet %s: %v", sessionID, filename, err)
					}
					continue
				}
				
				// Skip already processed packets
				if processedSequences[packet.Sequence] {
					continue
				}
				
				// For open packets, process immediately
				if packet.Type == PacketTypeOpen {
					// Connect to target server using packet metadata or default target
					connectAddr := targetAddr
					if packet.TargetHost != "" && packet.TargetPort != 0 {
						connectAddr = fmt.Sprintf("%s:%d", packet.TargetHost, packet.TargetPort)
					}
					
					targetConn, err = net.Dial("tcp", connectAddr)
					if err != nil {
						if verbose {
							log.Printf("Server: Session %s - Failed to connect to target %s: %v", sessionID, connectAddr, err)
						}
						// Send error close packet
						closePacket := CreateClosePacket(sessionID, "server_to_client", sequence, fmt.Sprintf("connection failed: %v", err))
						if writeErr := packetHandler.WritePacket(closePacket); writeErr != nil && verbose {
							log.Printf("Server: Session %s - Error writing connection error packet: %v", sessionID, writeErr)
						}
						return
					}
					
					if verbose {
						log.Printf("Server: Session %s - Connected to target %s", sessionID, connectAddr)
					}
					
					processedSequences[packet.Sequence] = true
					processedAny = true
					
					// Clean up processed packet file
					if err := os.Remove(filename); err != nil && verbose {
						log.Printf("Server: Warning: failed to remove processed packet file %s: %v", filename, err)
					}
					continue
				}
				
				// For other packets, process in sequence order
				if packet.Sequence == nextExpectedSequence {
					switch packet.Type {
					case PacketTypeData:
						if targetConn == nil {
							if verbose {
								log.Printf("Server: Session %s - Received data packet but no target connection", sessionID)
							}
							continue
						}
						
						data, err := packet.GetData()
						if err != nil {
							if verbose {
								log.Printf("Server: Error decoding data from packet %s: %v", packet.ID, err)
							}
							continue
						}
						
						if len(data) > 0 {
							_, writeErr := targetConn.Write(data)
							if verbose && writeErr == nil {
								log.Printf("Server: Session %s - Sent %d bytes to target (seq %d)", sessionID, len(data), packet.Sequence)
							}
							if writeErr != nil {
								if verbose {
									log.Printf("Server: Session %s - Error writing to target: %v", sessionID, writeErr)
								}
								return
							}
						}
						
					case PacketTypeClose:
						if verbose {
							log.Printf("Server: Session %s - Received close packet: %s", sessionID, packet.ErrorMsg)
						}
						sessionActive = false
						
						// Clean up processed packet file
						if err := os.Remove(filename); err != nil && verbose {
							log.Printf("Server: Warning: failed to remove processed packet file %s: %v", filename, err)
						}
						return
						
					case PacketTypeHeartbeat:
						// Respond to heartbeat
						heartbeatPacket := CreateHeartbeatPacket(sessionID, "server_to_client")
						if writeErr := packetHandler.WritePacket(heartbeatPacket); writeErr != nil && verbose {
							log.Printf("Server: Session %s - Error writing heartbeat response: %v", sessionID, writeErr)
						}
						if verbose {
							log.Printf("Server: Session %s - Responded to heartbeat", sessionID)
						}
					}
					
					processedSequences[packet.Sequence] = true
					nextExpectedSequence++
					processedAny = true
					
					// Clean up processed packet file
					if err := os.Remove(filename); err != nil && verbose {
						log.Printf("Server: Warning: failed to remove processed packet file %s: %v", filename, err)
					}
				}
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
