package main

import (
	"fmt"
	"runtime"
)

// Version information - updated during releases
var (
	Version   = "v1.0.0-rc3"
	BuildDate = "unknown"
	GitCommit = "unknown"
	Author    = "Girino Vey"
	Copyright = "2025"
	License   = "https://license.girino.org"
)

// GetVersionInfo returns formatted version information
func GetVersionInfo() string {
	return fmt.Sprintf("TCP-over-Nostr %s", Version)
}

// GetFullVersionInfo returns detailed version information
func GetFullVersionInfo() string {
	return fmt.Sprintf(`TCP-over-Nostr %s
Built: %s
Commit: %s
Go: %s %s/%s`,
		Version,
		BuildDate,
		GitCommit,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
}

// GetBanner returns the startup banner
func GetBanner() string {
	return fmt.Sprintf(`
┌─────────────────────────────────────────────────────────────┐
│                     TCP-over-Nostr %s                     │
│                                                             │
│  Decentralized TCP Proxy over Nostr Protocol               │
│  Author: %s                                        │
│  Copyright © %s                                            │
│  License: %s                              │
└─────────────────────────────────────────────────────────────┘
`, Version, Author, Copyright, License)
}

// GetCopyrightInfo returns copyright and license information
func GetCopyrightInfo() string {
	return fmt.Sprintf("Copyright © %s %s. Licensed under %s", Copyright, Author, License)
}
