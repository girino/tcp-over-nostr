package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	// Mode selection
	var mode = flag.String("mode", "", "Mode to run: 'client' or 'server' (required)")

	// Client flags
	var clientPort = flag.Int("client-port", 8080, "Port for client to listen on")

	// Server flags
	var targetHost = flag.String("target-host", "localhost", "Target host to proxy to")
	var targetPort = flag.Int("target-port", 80, "Target port to proxy to")

	// Nostr flags
	var relay = flag.String("relay", "ws://localhost:10547", "Nostr relay URL for event communication")
	var serverKey = flag.String("server-key", "", "Server's Nostr public key (required for client)")
	var keysFile = flag.String("keys-file", "", "File to store Nostr key pair (default: client-keys.json or server-keys.json)")

	var verbose = flag.Bool("verbose", false, "Enable verbose logging")
	var version = flag.Bool("version", false, "Show version information")

	flag.Parse()

	if *version {
		fmt.Printf("%s\n", GetFullVersionInfo())
		fmt.Printf("%s\n", GetCopyrightInfo())
		os.Exit(0)
	}

	if *mode == "" {
		fmt.Fprintf(os.Stderr, "%s\n", GetVersionInfo())
		fmt.Fprintf(os.Stderr, "Decentralized TCP Proxy over Nostr Protocol\n")
		fmt.Fprintf(os.Stderr, "%s\n\n", GetCopyrightInfo())
		fmt.Fprintf(os.Stderr, "Usage: %s -mode <client|server> [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Modes:\n")
		fmt.Fprintf(os.Stderr, "  client: Accept TCP connections and forward data via Nostr events\n")
		fmt.Fprintf(os.Stderr, "  server: Receive Nostr events and connect to target host\n\n")
		fmt.Fprintf(os.Stderr, "Client mode options:\n")
		fmt.Fprintf(os.Stderr, "  -client-port int     Port for client to listen on (default 8080)\n")
		fmt.Fprintf(os.Stderr, "  -server-key string   Server's Nostr public key (required)\n")
		fmt.Fprintf(os.Stderr, "  -keys-file string    File to store Nostr key pair (default \"client-keys.json\")\n")
		fmt.Fprintf(os.Stderr, "  -relay string        Nostr relay URL (default \"ws://localhost:10547\")\n")
		fmt.Fprintf(os.Stderr, "  -verbose            Enable verbose logging\n")
		fmt.Fprintf(os.Stderr, "  -version            Show version information\n\n")
		fmt.Fprintf(os.Stderr, "Server mode options:\n")
		fmt.Fprintf(os.Stderr, "  -target-host string  Target host to proxy to (default \"localhost\")\n")
		fmt.Fprintf(os.Stderr, "  -target-port int     Target port to proxy to (default 80)\n")
		fmt.Fprintf(os.Stderr, "  -keys-file string    File to store Nostr key pair (default \"server-keys.json\")\n")
		fmt.Fprintf(os.Stderr, "  -relay string        Nostr relay URL (default \"ws://localhost:10547\")\n")
		fmt.Fprintf(os.Stderr, "  -verbose            Enable verbose logging\n")
		fmt.Fprintf(os.Stderr, "  -version            Show version information\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  # Start server (shows pubkey for client)\n")
		fmt.Fprintf(os.Stderr, "  %s -mode server -target-host httpbin.org -target-port 80 -relay ws://relay.damus.io\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Start client with server's pubkey\n")
		fmt.Fprintf(os.Stderr, "  %s -mode client -server-key <server_pubkey> -client-port 8080 -relay ws://relay.damus.io\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Local relay (nak) example\n")
		fmt.Fprintf(os.Stderr, "  %s -mode server -target-host 192.168.1.100 -target-port 22\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode client -server-key <pubkey> -client-port 2222\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  ssh -p 2222 user@localhost\n\n")
		fmt.Fprintf(os.Stderr, "For more information:\n")
		fmt.Fprintf(os.Stderr, "  Version: %s --version\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  License: %s\n\n", License)
		os.Exit(1)
	}

	// Validate client requirements
	if *mode == "client" && *serverKey == "" {
		log.Fatal("Client mode requires -server-key parameter")
	}

	// Set default key file names if not specified
	if *keysFile == "" {
		if *mode == "client" {
			*keysFile = "client-keys.json"
		} else if *mode == "server" {
			*keysFile = "server-keys.json"
		}
	}

	switch *mode {
	case "client":
		runClientNostr(*clientPort, *relay, *serverKey, *keysFile, *verbose)
	case "server":
		runServerNostr(*targetHost, *targetPort, *relay, *keysFile, *verbose)
	default:
		log.Fatalf("Invalid mode '%s'. Must be 'client' or 'server'", *mode)
	}
}
