package main

import (
	"fmt"
	"runtime"
	"unicode/utf8"
)

// Version information - updated during releases
var (
	Version   = "v2.0.2"
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
	// Box width is 63 characters (including corners)
	boxWidth := 63
	contentWidth := boxWidth - 2 // Subtract the │ characters on each side

	// Calculate padding for each line using UTF-8 rune count for proper Unicode handling
	titleText := fmt.Sprintf("TCP-over-Nostr %s", Version)
	titleTextLength := utf8.RuneCountInString(titleText)
	totalPadding := contentWidth - titleTextLength
	leftPadding := totalPadding / 2
	rightPadding := totalPadding - leftPadding
	titleLine := fmt.Sprintf("%*s%s%*s", leftPadding, "", titleText, rightPadding, "")

	// Calculate padding for the description line
	descLine := "  Decentralized TCP Proxy over Nostr Protocol"
	descPadding := contentWidth - utf8.RuneCountInString(descLine)
	descSpaces := ""
	if descPadding > 0 {
		descSpaces = fmt.Sprintf("%*s", descPadding, "")
	}

	authorLine := fmt.Sprintf("  Author: %s", Author)
	authorPadding := contentWidth - utf8.RuneCountInString(authorLine)
	authorSpaces := ""
	if authorPadding > 0 {
		authorSpaces = fmt.Sprintf("%*s", authorPadding, "")
	}

	copyrightLine := fmt.Sprintf("  Copyright © %s", Copyright)
	copyrightPadding := contentWidth - utf8.RuneCountInString(copyrightLine)
	copyrightSpaces := ""
	if copyrightPadding > 0 {
		copyrightSpaces = fmt.Sprintf("%*s", copyrightPadding, "")
	}

	licenseLine := fmt.Sprintf("  License: %s", License)
	licensePadding := contentWidth - utf8.RuneCountInString(licenseLine)
	licenseSpaces := ""
	if licensePadding > 0 {
		licenseSpaces = fmt.Sprintf("%*s", licensePadding, "")
	}

	return fmt.Sprintf(`
┌─────────────────────────────────────────────────────────────┐
│%s│
│                                                             │
│%s%s│
│%s%s│
│%s%s│
│%s%s│
└─────────────────────────────────────────────────────────────┘
`, titleLine, descLine, descSpaces, authorLine, authorSpaces, copyrightLine, copyrightSpaces, licenseLine, licenseSpaces)
}

// GetCopyrightInfo returns copyright and license information
func GetCopyrightInfo() string {
	return fmt.Sprintf("Copyright © %s %s. Licensed under %s", Copyright, Author, License)
}
