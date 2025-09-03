package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// getFlagOrEnv gets a value from flag first, or falls back to environment variable with TON_ prefix
func getFlagOrEnv(flagValue, envName, flagName string) string {
	// Check if the flag was actually set by the user
	if isFlagSet(flagName) {
		return flagValue
	}
	// Fall back to environment variable
	if envValue := os.Getenv("TON_" + envName); envValue != "" {
		return envValue
	}
	return flagValue
}

// getFlagOrEnvBool gets a boolean value from flag first, or falls back to environment variable with TON_ prefix
func getFlagOrEnvBool(flagValue bool, envName, flagName string) bool {
	// Check if the flag was actually set by the user
	if isFlagSet(flagName) {
		return flagValue
	}
	// Fall back to environment variable
	if envValue := os.Getenv("TON_" + envName); envValue != "" {
		if parsed, err := strconv.ParseBool(envValue); err == nil {
			return parsed
		}
	}
	return flagValue
}

// getFlagOrEnvInt gets an integer value from flag first, or falls back to environment variable with TON_ prefix
func getFlagOrEnvInt(flagValue int, envName, flagName string) int {
	// Check if the flag was actually set by the user
	if isFlagSet(flagName) {
		return flagValue
	}
	// Fall back to environment variable
	if envValue := os.Getenv("TON_" + envName); envValue != "" {
		if parsed, err := strconv.Atoi(envValue); err == nil {
			return parsed
		}
	}
	return flagValue
}

// isFlagSet checks if a flag was actually set by the user
func isFlagSet(flagName string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == flagName {
			set = true
		}
	})
	return set
}

func main() {
	// Mode selection
	var mode = flag.String("mode", "", "Mode to run: 'client' or 'server' (required)")

	// Client flags
	var clientPort = flag.Int("client-port", 8080, "Port for client to listen on")

	// Server flags
	var targetHost = flag.String("target-host", "localhost", "Target host to proxy to")
	var targetPort = flag.Int("target-port", 80, "Target port to proxy to")

	// Nostr flags
	var relay = flag.String("relay", "ws://localhost:10547", "Nostr relay URL for event communication (can specify multiple with -relay flag)")
	var serverKey = flag.String("server-key", "", "Server's Nostr public key (required for client)")
	var privateKey = flag.String("private-key", "", "Private key in hex or nsec format (if not provided, keys will be generated)")

	var verbose = flag.Bool("verbose", false, "Enable verbose logging")
	var version = flag.Bool("version", false, "Show version information")

	flag.Parse()

	// Use flag values first, fall back to environment variables if flags are not set
	*mode = getFlagOrEnv(*mode, "MODE", "mode")
	*clientPort = getFlagOrEnvInt(*clientPort, "CLIENT_PORT", "client-port")
	*targetHost = getFlagOrEnv(*targetHost, "TARGET_HOST", "target-host")
	*targetPort = getFlagOrEnvInt(*targetPort, "TARGET_PORT", "target-port")
	*relay = getFlagOrEnv(*relay, "RELAY", "relay")
	*serverKey = getFlagOrEnv(*serverKey, "SERVER_KEY", "server-key")
	*privateKey = getFlagOrEnv(*privateKey, "PRIVATE_KEY", "private-key")
	*verbose = getFlagOrEnvBool(*verbose, "VERBOSE", "verbose")
	*version = getFlagOrEnvBool(*version, "VERSION", "version")

	// Collect all relay URLs (can be specified multiple times with -relay flag or comma-separated)
	var relayURLs []string

	// First, collect relays from command line -relay flags
	for i, arg := range os.Args {
		if arg == "-relay" && i+1 < len(os.Args) {
			relayValue := os.Args[i+1]
			// Check if the value contains commas (comma-separated format)
			if strings.Contains(relayValue, ",") {
				// Split by comma and add each relay
				relays := strings.Split(relayValue, ",")
				for _, r := range relays {
					r = strings.TrimSpace(r)
					if r != "" {
						relayURLs = append(relayURLs, r)
					}
				}
			} else {
				// Single relay value
				relayURLs = append(relayURLs, relayValue)
			}
		}
	}

	// If no relays specified via -relay flags, process the relay value (from environment or default)
	if len(relayURLs) == 0 {
		relayValue := *relay
		// Check if the value contains commas (comma-separated format)
		if strings.Contains(relayValue, ",") {
			// Split by comma and add each relay
			relays := strings.Split(relayValue, ",")
			for _, r := range relays {
				r = strings.TrimSpace(r)
				if r != "" {
					relayURLs = append(relayURLs, r)
				}
			}
		} else {
			// Single relay value
			relayURLs = []string{relayValue}
		}
	}

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
		fmt.Fprintf(os.Stderr, "Environment Variables:\n")
		fmt.Fprintf(os.Stderr, "  All command line parameters can also be provided as environment variables\n")
		fmt.Fprintf(os.Stderr, "  with TON_ prefix (e.g., TON_MODE, TON_CLIENT_PORT, TON_SERVER_KEY, etc.)\n")
		fmt.Fprintf(os.Stderr, "  Command line flags take precedence over environment variables.\n\n")
		fmt.Fprintf(os.Stderr, "Client mode options:\n")
		fmt.Fprintf(os.Stderr, "  -client-port int     Port for client to listen on (default 8080)\n")
		fmt.Fprintf(os.Stderr, "  -server-key string   Server's Nostr public key in hex or npub format (required)\n")
		fmt.Fprintf(os.Stderr, "  -private-key string  Private key in hex or nsec format (if not provided, keys will be generated)\n")
		fmt.Fprintf(os.Stderr, "  -relay string        Nostr relay URL (can specify multiple times or comma-separated, default \"ws://localhost:10547\")\n")
		fmt.Fprintf(os.Stderr, "  -verbose            Enable verbose logging\n")
		fmt.Fprintf(os.Stderr, "  -version            Show version information\n\n")
		fmt.Fprintf(os.Stderr, "Server mode options:\n")
		fmt.Fprintf(os.Stderr, "  -target-host string  Target host to proxy to (default \"localhost\") or host:port format\n")
		fmt.Fprintf(os.Stderr, "  -target-port int     Target port to proxy to (default 80, ignored if host:port format used)\n")
		fmt.Fprintf(os.Stderr, "  -private-key string  Private key in hex or nsec format (if not provided, keys will be generated)\n")
		fmt.Fprintf(os.Stderr, "  -relay string        Nostr relay URL (can specify multiple times or comma-separated, default \"ws://localhost:10547\")\n")
		fmt.Fprintf(os.Stderr, "  -verbose            Enable verbose logging\n")
		fmt.Fprintf(os.Stderr, "  -version            Show version information\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  # Start server (shows pubkey for client) - separate host and port\n")
		fmt.Fprintf(os.Stderr, "  %s -mode server -target-host httpbin.org -target-port 80 -relay ws://relay.damus.io\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Start server - combined host:port format\n")
		fmt.Fprintf(os.Stderr, "  %s -mode server -target-host 192.168.31.131:22 -relay ws://relay.damus.io\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Start client with server's pubkey - multiple relay flags\n")
		fmt.Fprintf(os.Stderr, "  %s -mode client -server-key <server_pubkey> -relay ws://relay.damus.io -relay ws://relay.primal.net\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Start client with server's pubkey - comma-separated relays\n")
		fmt.Fprintf(os.Stderr, "  %s -mode client -server-key <server_pubkey> -relay ws://relay.damus.io,ws://relay.primal.net,ws://nostr.girino.org\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # SSH proxy example\n")
		fmt.Fprintf(os.Stderr, "  %s -mode server -target-host 192.168.1.100:22\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode client -server-key <pubkey> -client-port 2222\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  ssh -p 2222 user@localhost\n\n")
		fmt.Fprintf(os.Stderr, "For more information:\n")
		fmt.Fprintf(os.Stderr, "  Version: %s --version\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  License: %s\n\n", License)
		os.Exit(1)
	}

	// Parse target-host for combined host:port format
	if *mode == "server" {
		if strings.Contains(*targetHost, ":") {
			// Split host:port format
			parts := strings.Split(*targetHost, ":")
			if len(parts) != 2 {
				log.Fatal("Invalid target-host format. Use 'host:port' or separate -target-host and -target-port")
			}
			*targetHost = parts[0]
			if port, err := strconv.Atoi(parts[1]); err != nil {
				log.Fatalf("Invalid port in target-host: %v", err)
			} else {
				*targetPort = port
			}
		}
	}

	// Validate client requirements
	if *mode == "client" && *serverKey == "" {
		log.Fatal("Client mode requires -server-key parameter")
	}

	switch *mode {
	case "client":
		runClientNostr(*clientPort, relayURLs, *serverKey, *privateKey, *verbose)
	case "server":
		runServerNostr(*targetHost, *targetPort, relayURLs, *privateKey, *verbose)
	default:
		log.Fatalf("Invalid mode '%s'. Must be 'client' or 'server'", *mode)
	}
}
