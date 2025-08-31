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

	// Communication method
	var packetMode = flag.Bool("packet-mode", false, "Use packet-based JSON communication instead of stream files")
	var nostrMode = flag.Bool("nostr-mode", false, "Use Nostr events for communication")

	// Client flags
	var clientPort = flag.Int("client-port", 8080, "Port for client to listen on")

	// Server flags
	var targetHost = flag.String("target-host", "localhost", "Target host to proxy to")
	var targetPort = flag.Int("target-port", 80, "Target port to proxy to")

	// Shared flags
	var inputFile = flag.String("input-file", "input", "File for communication (input for client, pattern for server)")
	var outputFile = flag.String("output-file", "output", "File for communication (output for client, pattern for server)")
	var packetDir = flag.String("packet-dir", "packets", "Directory for packet-based communication")

	// Nostr flags
	var nostrDir = flag.String("nostr-dir", "events", "Directory for Nostr event files")
	var serverKey = flag.String("server-key", "", "Server's Nostr public key (required for client in nostr-mode)")
	var keysFile = flag.String("keys-file", "", "File to store Nostr key pair (default: client-keys.json or server-keys.json)")

	var verbose = flag.Bool("verbose", false, "Enable verbose logging")

	flag.Parse()

	if *mode == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -mode <client|server> [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Modes:\n")
		fmt.Fprintf(os.Stderr, "  client: Accept TCP connections and forward data\n")
		fmt.Fprintf(os.Stderr, "  server: Receive forwarded data and connect to target host\n\n")
		fmt.Fprintf(os.Stderr, "Communication options:\n")
		fmt.Fprintf(os.Stderr, "  -packet-mode        Use JSON packet files (default: stream files)\n")
		fmt.Fprintf(os.Stderr, "  -nostr-mode         Use Nostr events for communication\n\n")
		fmt.Fprintf(os.Stderr, "Client mode options:\n")
		fmt.Fprintf(os.Stderr, "  -client-port int     Port for client to listen on (default 8080)\n")
		if *nostrMode {
			fmt.Fprintf(os.Stderr, "  -nostr-dir string    Directory for Nostr event files (default \"events\")\n")
			fmt.Fprintf(os.Stderr, "  -server-key string   Server's Nostr public key (required)\n")
			fmt.Fprintf(os.Stderr, "  -keys-file string    File to store Nostr key pair (default \"client-keys.json\")\n")
		} else if *packetMode {
			fmt.Fprintf(os.Stderr, "  -packet-dir string   Directory for packet files (default \"packets\")\n")
		} else {
			fmt.Fprintf(os.Stderr, "  -input-file string   File to write client data to (default \"input\")\n")
			fmt.Fprintf(os.Stderr, "  -output-file string  File to read server responses from (default \"output\")\n")
		}
		fmt.Fprintf(os.Stderr, "  -verbose            Enable verbose logging\n\n")
		fmt.Fprintf(os.Stderr, "Server mode options:\n")
		fmt.Fprintf(os.Stderr, "  -target-host string  Target host to proxy to (default \"localhost\")\n")
		fmt.Fprintf(os.Stderr, "  -target-port int     Target port to proxy to (default 80)\n")
		if *nostrMode {
			fmt.Fprintf(os.Stderr, "  -nostr-dir string    Directory for Nostr event files (default \"events\")\n")
			fmt.Fprintf(os.Stderr, "  -keys-file string    File to store Nostr key pair (default \"server-keys.json\")\n")
		} else if *packetMode {
			fmt.Fprintf(os.Stderr, "  -packet-dir string   Directory for packet files (default \"packets\")\n")
		} else {
			fmt.Fprintf(os.Stderr, "  -input-file string   File pattern to read client data from (default \"input\")\n")
			fmt.Fprintf(os.Stderr, "  -output-file string  File pattern to write server responses to (default \"output\")\n")
		}
		fmt.Fprintf(os.Stderr, "  -verbose            Enable verbose logging\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s -mode client -client-port 8080 -verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode client -packet-mode -client-port 8080 -verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode client -nostr-mode -server-key <server_pubkey> -verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode server -target-host google.com -target-port 80 -verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode server -packet-mode -target-host google.com -target-port 80 -verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode server -nostr-mode -target-host google.com -target-port 80 -verbose\n", os.Args[0])
		os.Exit(1)
	}

	// Validate Nostr mode requirements
	if *nostrMode && *mode == "client" && *serverKey == "" {
		log.Fatal("Client in nostr-mode requires -server-key parameter")
	}

	// Check for conflicting communication modes
	if *packetMode && *nostrMode {
		log.Fatal("Cannot use both -packet-mode and -nostr-mode simultaneously")
	}

	// Set default key file names if not specified
	if *keysFile == "" {
		if *mode == "client" {
			*keysFile = "client-keys.json"
		} else if *mode == "server" {
			*keysFile = "server-keys.json"
		} else {
			*keysFile = "nostr-keys.json"
		}
	}

	switch *mode {
	case "client":
		if *nostrMode {
			runClientNostr(*clientPort, *nostrDir, *serverKey, *keysFile, *verbose)
		} else if *packetMode {
			runClientPackets(*clientPort, *packetDir, *verbose)
		} else {
			runClient(*clientPort, *inputFile, *outputFile, *verbose)
		}
	case "server":
		if *nostrMode {
			runServerNostr(*targetHost, *targetPort, *nostrDir, *keysFile, *verbose)
		} else if *packetMode {
			runServerPackets(*targetHost, *targetPort, *packetDir, *verbose)
		} else {
			runServer(*targetHost, *targetPort, *inputFile, *outputFile, *verbose)
		}
	default:
		log.Fatalf("Invalid mode '%s'. Must be 'client' or 'server'", *mode)
	}
}
