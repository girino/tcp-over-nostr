package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

func runClient(clientPort int, inputFile, outputFile string, verbose bool) {
	// Validate inputs
	if clientPort < 1 || clientPort > 65535 {
		log.Fatal("Client port must be between 1 and 65535")
	}

	listenAddr := fmt.Sprintf(":%d", clientPort)

	fmt.Printf("Starting TCP proxy client:\n")
	fmt.Printf("  Listening on: %s\n", listenAddr)
	fmt.Printf("  Input file: %s\n", inputFile)
	fmt.Printf("  Output file: %s\n", outputFile)
	fmt.Printf("  Verbose logging: %v\n", verbose)

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
		go handleClientConnection(clientConn, inputFile, outputFile, verbose)
	}
}

func handleClientConnection(clientConn net.Conn, inputFile, outputFile string, verbose bool) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	if verbose {
		log.Printf("Client: New connection from %s", clientAddr)
	}

	// Create session ID based on connection time and address
	sessionID := fmt.Sprintf("%d_%s", time.Now().UnixNano(), clientAddr)
	sessionInputFile := fmt.Sprintf("%s_%s", inputFile, sessionID)
	sessionOutputFile := fmt.Sprintf("%s_%s", outputFile, sessionID)

	if verbose {
		log.Printf("Client: Session %s - Input: %s, Output: %s", sessionID, sessionInputFile, sessionOutputFile)
	}

	// Create input file for writing client data
	inFile, err := os.Create(sessionInputFile)
	if err != nil {
		if verbose {
			log.Printf("Client: Failed to create input file %s: %v", sessionInputFile, err)
		}
		return
	}
	defer inFile.Close()
	defer os.Remove(sessionInputFile) // Clean up when done

	// Start goroutine to read from output file and send to client
	done := make(chan bool, 2)

	go func() {
		defer func() { done <- true }()

		// Wait for output file to be created by server
		for {
			if _, err := os.Stat(sessionOutputFile); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		outFile, err := os.Open(sessionOutputFile)
		if err != nil {
			if verbose {
				log.Printf("Client: Failed to open output file %s: %v", sessionOutputFile, err)
			}
			return
		}
		defer outFile.Close()
		defer os.Remove(sessionOutputFile) // Clean up when done

		reader := bufio.NewReader(outFile)
		buffer := make([]byte, 4096)

		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				bytesWritten, writeErr := clientConn.Write(buffer[:n])
				if verbose && writeErr == nil {
					log.Printf("Client: Session %s - Sent %d bytes to client", sessionID, bytesWritten)
				}
				if writeErr != nil {
					if verbose {
						log.Printf("Client: Session %s - Error writing to client: %v", sessionID, writeErr)
					}
					return
				}
			}
			if err == io.EOF {
				// Keep reading, server might write more
				time.Sleep(10 * time.Millisecond)
				continue
			}
			if err != nil {
				if verbose {
					log.Printf("Client: Session %s - Error reading output file: %v", sessionID, err)
				}
				return
			}
		}
	}()

	// Read from client and write to input file
	go func() {
		defer func() { done <- true }()

		buffer := make([]byte, 4096)
		for {
			n, err := clientConn.Read(buffer)
			if n > 0 {
				bytesWritten, writeErr := inFile.Write(buffer[:n])
				if writeErr == nil {
					inFile.Sync() // Ensure data is flushed
				}
				if verbose && writeErr == nil {
					log.Printf("Client: Session %s - Wrote %d bytes to input file", sessionID, bytesWritten)
				}
				if writeErr != nil {
					if verbose {
						log.Printf("Client: Session %s - Error writing to input file: %v", sessionID, writeErr)
					}
					return
				}
			}
			if err == io.EOF {
				if verbose {
					log.Printf("Client: Session %s - Client disconnected", sessionID)
				}
				return
			}
			if err != nil {
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
