package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runServer(targetHost string, targetPort int, inputFile, outputFile string, verbose bool) {
	// Validate inputs
	if targetPort < 1 || targetPort > 65535 {
		log.Fatal("Target port must be between 1 and 65535")
	}
	if targetHost == "" {
		log.Fatal("Target host cannot be empty")
	}

	targetAddr := fmt.Sprintf("%s:%d", targetHost, targetPort)

	fmt.Printf("Starting TCP proxy server:\n")
	fmt.Printf("  Target: %s\n", targetAddr)
	fmt.Printf("  Input file pattern: %s_*\n", inputFile)
	fmt.Printf("  Output file pattern: %s_*\n", outputFile)
	fmt.Printf("  Verbose logging: %v\n", verbose)

	fmt.Printf("TCP proxy server started successfully. Monitoring for input files...\n\n")

	// Monitor for new input files
	monitorInputFiles(inputFile, outputFile, targetAddr, verbose)
}

func monitorInputFiles(inputFilePattern, outputFilePattern, targetAddr string, verbose bool) {
	processedFiles := make(map[string]bool)

	for {
		// Find input files matching pattern
		pattern := inputFilePattern + "_*"
		matches, err := filepath.Glob(pattern)
		if err != nil {
			if verbose {
				log.Printf("Server: Error globbing files: %v", err)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Process new input files
		for _, inputFile := range matches {
			if !processedFiles[inputFile] {
				processedFiles[inputFile] = true

				// Extract session ID from filename
				sessionID := strings.TrimPrefix(inputFile, inputFilePattern+"_")
				outputFile := outputFilePattern + "_" + sessionID

				if verbose {
					log.Printf("Server: Found new session %s", sessionID)
				}

				// Handle this session in a goroutine
				go handleServerSession(inputFile, outputFile, targetAddr, sessionID, verbose)
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func handleServerSession(inputFile, outputFile, targetAddr, sessionID string, verbose bool) {
	if verbose {
		log.Printf("Server: Session %s - Starting, input: %s, output: %s", sessionID, inputFile, outputFile)
	}

	// Connect to target server
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		if verbose {
			log.Printf("Server: Session %s - Failed to connect to target %s: %v", sessionID, targetAddr, err)
		}
		return
	}
	defer targetConn.Close()

	if verbose {
		log.Printf("Server: Session %s - Connected to target %s", sessionID, targetAddr)
	}

	// Create output file for writing server responses
	outFile, err := os.Create(outputFile)
	if err != nil {
		if verbose {
			log.Printf("Server: Session %s - Failed to create output file %s: %v", sessionID, outputFile, err)
		}
		return
	}
	defer outFile.Close()

	done := make(chan bool, 2)

	// Read from target and write to output file
	go func() {
		defer func() { done <- true }()

		buffer := make([]byte, 4096)
		for {
			n, err := targetConn.Read(buffer)
			if n > 0 {
				bytesWritten, writeErr := outFile.Write(buffer[:n])
				if writeErr == nil {
					outFile.Sync() // Ensure data is flushed
				}
				if verbose && writeErr == nil {
					log.Printf("Server: Session %s - Wrote %d bytes to output file", sessionID, bytesWritten)
				}
				if writeErr != nil {
					if verbose {
						log.Printf("Server: Session %s - Error writing to output file: %v", sessionID, writeErr)
					}
					return
				}
			}
			if err == io.EOF {
				if verbose {
					log.Printf("Server: Session %s - Target disconnected", sessionID)
				}
				return
			}
			if err != nil {
				if verbose {
					log.Printf("Server: Session %s - Error reading from target: %v", sessionID, err)
				}
				return
			}
		}
	}()

	// Read from input file and write to target
	go func() {
		defer func() { done <- true }()

		// Open input file for reading
		inFile, err := os.Open(inputFile)
		if err != nil {
			if verbose {
				log.Printf("Server: Session %s - Failed to open input file %s: %v", sessionID, inputFile, err)
			}
			return
		}
		defer inFile.Close()

		reader := bufio.NewReader(inFile)
		buffer := make([]byte, 4096)

		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				bytesWritten, writeErr := targetConn.Write(buffer[:n])
				if verbose && writeErr == nil {
					log.Printf("Server: Session %s - Sent %d bytes to target", sessionID, bytesWritten)
				}
				if writeErr != nil {
					if verbose {
						log.Printf("Server: Session %s - Error writing to target: %v", sessionID, writeErr)
					}
					return
				}
			}
			if err == io.EOF {
				// Keep reading, client might send more
				time.Sleep(10 * time.Millisecond)
				continue
			}
			if err != nil {
				if verbose {
					log.Printf("Server: Session %s - Error reading input file: %v", sessionID, err)
				}
				return
			}
		}
	}()

	// Wait for either direction to finish
	<-done

	if verbose {
		log.Printf("Server: Session %s - Connection closed", sessionID)
	}
}
