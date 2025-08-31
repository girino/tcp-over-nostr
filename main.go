package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
)

func main() {
	// Command line flags
	var listenPort = flag.Int("listen", 8080, "Port to listen on")
	var targetHost = flag.String("target-host", "localhost", "Target host to proxy to")
	var targetPort = flag.Int("target-port", 80, "Target port to proxy to")
	var verbose = flag.Bool("verbose", false, "Enable verbose logging")
	
	flag.Parse()

	// Validate inputs
	if *listenPort < 1 || *listenPort > 65535 {
		log.Fatal("Listen port must be between 1 and 65535")
	}
	if *targetPort < 1 || *targetPort > 65535 {
		log.Fatal("Target port must be between 1 and 65535")
	}
	if *targetHost == "" {
		log.Fatal("Target host cannot be empty")
	}

	listenAddr := fmt.Sprintf(":%d", *listenPort)
	targetAddr := fmt.Sprintf("%s:%d", *targetHost, *targetPort)

	fmt.Printf("Starting TCP proxy:\n")
	fmt.Printf("  Listening on: %s\n", listenAddr)
	fmt.Printf("  Proxying to: %s\n", targetAddr)
	fmt.Printf("  Verbose logging: %v\n", *verbose)

	// Start listening
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", listenAddr, err)
	}
	defer listener.Close()

	fmt.Printf("TCP proxy started successfully. Press Ctrl+C to stop.\n\n")

	// Accept connections
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			if *verbose {
				log.Printf("Failed to accept connection: %v", err)
			}
			continue
		}

		// Handle each connection in a goroutine
		go handleConnection(clientConn, targetAddr, *verbose)
	}
}

func handleConnection(clientConn net.Conn, targetAddr string, verbose bool) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	if verbose {
		log.Printf("New connection from %s", clientAddr)
	}

	// Connect to target server
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		if verbose {
			log.Printf("Failed to connect to target %s: %v", targetAddr, err)
		}
		return
	}
	defer targetConn.Close()

	if verbose {
		log.Printf("Connected to target %s for client %s", targetAddr, clientAddr)
	}

	// Start proxying data in both directions
	done := make(chan bool, 2)

	// Client -> Target
	go func() {
		defer func() { done <- true }()
		bytesWritten, err := io.Copy(targetConn, clientConn)
		if verbose && err != nil {
			log.Printf("Error copying from client %s to target: %v", clientAddr, err)
		}
		if verbose {
			log.Printf("Client %s -> Target: %d bytes", clientAddr, bytesWritten)
		}
	}()

	// Target -> Client
	go func() {
		defer func() { done <- true }()
		bytesWritten, err := io.Copy(clientConn, targetConn)
		if verbose && err != nil {
			log.Printf("Error copying from target to client %s: %v", clientAddr, err)
		}
		if verbose {
			log.Printf("Target -> Client %s: %d bytes", clientAddr, bytesWritten)
		}
	}()

	// Wait for either direction to finish
	<-done

	if verbose {
		log.Printf("Connection with client %s closed", clientAddr)
	}
}
