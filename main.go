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

	// Client flags
	var clientPort = flag.Int("client-port", 8080, "Port for client to listen on")

	// Server flags
	var targetHost = flag.String("target-host", "localhost", "Target host to proxy to")
	var targetPort = flag.Int("target-port", 80, "Target port to proxy to")

	// Shared flags
	var inputFile = flag.String("input-file", "input", "File for communication (input for client, pattern for server)")
	var outputFile = flag.String("output-file", "output", "File for communication (output for client, pattern for server)")
	var packetDir = flag.String("packet-dir", "packets", "Directory for packet-based communication")
	var verbose = flag.Bool("verbose", false, "Enable verbose logging")

	flag.Parse()

	if *mode == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -mode <client|server> [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Modes:\n")
		fmt.Fprintf(os.Stderr, "  client: Accept TCP connections and forward data\n")
		fmt.Fprintf(os.Stderr, "  server: Receive forwarded data and connect to target host\n\n")
		fmt.Fprintf(os.Stderr, "Communication options:\n")
		fmt.Fprintf(os.Stderr, "  -packet-mode        Use JSON packet files (default: stream files)\n\n")
		fmt.Fprintf(os.Stderr, "Client mode options:\n")
		fmt.Fprintf(os.Stderr, "  -client-port int     Port for client to listen on (default 8080)\n")
		if *packetMode {
			fmt.Fprintf(os.Stderr, "  -packet-dir string   Directory for packet files (default \"packets\")\n")
		} else {
			fmt.Fprintf(os.Stderr, "  -input-file string   File to write client data to (default \"input\")\n")
			fmt.Fprintf(os.Stderr, "  -output-file string  File to read server responses from (default \"output\")\n")
		}
		fmt.Fprintf(os.Stderr, "  -verbose            Enable verbose logging\n\n")
		fmt.Fprintf(os.Stderr, "Server mode options:\n")
		fmt.Fprintf(os.Stderr, "  -target-host string  Target host to proxy to (default \"localhost\")\n")
		fmt.Fprintf(os.Stderr, "  -target-port int     Target port to proxy to (default 80)\n")
		if *packetMode {
			fmt.Fprintf(os.Stderr, "  -packet-dir string   Directory for packet files (default \"packets\")\n")
		} else {
			fmt.Fprintf(os.Stderr, "  -input-file string   File pattern to read client data from (default \"input\")\n")
			fmt.Fprintf(os.Stderr, "  -output-file string  File pattern to write server responses to (default \"output\")\n")
		}
		fmt.Fprintf(os.Stderr, "  -verbose            Enable verbose logging\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s -mode client -client-port 8080 -verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode client -packet-mode -client-port 8080 -verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode server -target-host google.com -target-port 80 -verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode server -packet-mode -target-host google.com -target-port 80 -verbose\n", os.Args[0])
		os.Exit(1)
	}

	switch *mode {
	case "client":
		if *packetMode {
			runClientPackets(*clientPort, *packetDir, *verbose)
		} else {
			runClient(*clientPort, *inputFile, *outputFile, *verbose)
		}
	case "server":
		if *packetMode {
			runServerPackets(*targetHost, *targetPort, *packetDir, *verbose)
		} else {
			runServer(*targetHost, *targetPort, *inputFile, *outputFile, *verbose)
		}
	default:
		log.Fatalf("Invalid mode '%s'. Must be 'client' or 'server'", *mode)
	}
}
