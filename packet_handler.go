package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PacketHandler manages packet file operations
type PacketHandler struct {
	baseDir        string
	sessionID      string
	verbose        bool
	processedFiles map[string]*Packet // Cache of processed packets by filename
}

// NewPacketHandler creates a new packet handler
func NewPacketHandler(baseDir, sessionID string, verbose bool) *PacketHandler {
	return &PacketHandler{
		baseDir:        baseDir,
		sessionID:      sessionID,
		verbose:        verbose,
		processedFiles: make(map[string]*Packet),
	}
}

// WritePacket writes a packet to a file
func (ph *PacketHandler) WritePacket(packet *Packet) error {
	// Ensure directory exists
	if err := os.MkdirAll(ph.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", ph.baseDir, err)
	}

	// Generate filename
	filename := packet.GetPacketFilename(ph.baseDir)

	// Convert packet to JSON
	jsonData, err := packet.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to convert packet to JSON: %v", err)
	}

	// Write to file
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write packet file %s: %v", filename, err)
	}

	if ph.verbose {
		log.Printf("PacketHandler: Wrote packet %s to %s", packet.ID, filename)
	}

	return nil
}

// ReadPacket reads a packet from a file, using cache when possible
func (ph *PacketHandler) ReadPacket(filename string) (*Packet, error) {
	// Check cache first
	if packet, exists := ph.processedFiles[filename]; exists {
		return packet, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read packet file %s: %v", filename, err)
	}

	packet, err := FromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse packet from %s: %v", filename, err)
	}

	// Cache the parsed packet
	ph.processedFiles[filename] = packet

	if ph.verbose {
		log.Printf("PacketHandler: Read packet %s from %s", packet.ID, filename)
	}

	return packet, nil
}

// GetPacketFiles returns all packet files for a specific session and direction, sorted by sequence
func (ph *PacketHandler) GetPacketFiles(direction string) ([]string, error) {
	// Get all JSON files in the directory
	pattern := "*.json"
	allFiles, err := filepath.Glob(filepath.Join(ph.baseDir, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to glob packet files: %v", err)
	}

	var matchingFiles []string

	// Process files, using cache when available
	for _, filename := range allFiles {
		var packet *Packet
		var err error

		// Try cache first
		if cachedPacket, exists := ph.processedFiles[filename]; exists {
			packet = cachedPacket
		} else {
			// Only read and parse if not in cache
			packet, err = ph.ReadPacket(filename)
			if err != nil {
				// Skip files that can't be parsed as packets
				continue
			}
		}

		// Check if this packet matches our session and direction
		if packet.SessionID == ph.sessionID && packet.Direction == direction {
			matchingFiles = append(matchingFiles, filename)
		}
	}

	// Sort by sequence number (using cached packets for efficiency)
	sort.Slice(matchingFiles, func(i, j int) bool {
		packet1 := ph.processedFiles[matchingFiles[i]]
		packet2 := ph.processedFiles[matchingFiles[j]]
		return packet1.Sequence < packet2.Sequence
	})

	return matchingFiles, nil
}

// GetAllPacketFiles returns all packet files for the session, sorted by timestamp
func (ph *PacketHandler) GetAllPacketFiles() ([]string, error) {
	// Get all JSON files in the directory
	pattern := "*.json"
	allFiles, err := filepath.Glob(filepath.Join(ph.baseDir, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to glob packet files: %v", err)
	}

	var matchingFiles []string

	// Parse each file to check if it matches our session
	for _, filename := range allFiles {
		packet, err := ph.ReadPacket(filename)
		if err != nil {
			// Skip files that can't be parsed as packets
			continue
		}

		// Check if this packet matches our session
		if packet.SessionID == ph.sessionID {
			matchingFiles = append(matchingFiles, filename)
		}
	}

	// Sort by packet timestamp
	sort.Slice(matchingFiles, func(i, j int) bool {
		packet1, err1 := ph.ReadPacket(matchingFiles[i])
		packet2, err2 := ph.ReadPacket(matchingFiles[j])
		if err1 != nil || err2 != nil {
			return false
		}
		return packet1.Timestamp.Before(packet2.Timestamp)
	})

	return matchingFiles, nil
}

// WatchForPackets monitors the directory for new packet files of a specific direction
func (ph *PacketHandler) WatchForPackets(direction string, callback func(*Packet, string)) {
	processedFiles := make(map[string]bool)

	for {
		files, err := ph.GetPacketFiles(direction)
		if err != nil {
			if ph.verbose {
				log.Printf("PacketHandler: Error getting packet files: %v", err)
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}

		for _, filename := range files {
			if !processedFiles[filename] {
				processedFiles[filename] = true

				packet, err := ph.ReadPacket(filename)
				if err != nil {
					if ph.verbose {
						log.Printf("PacketHandler: Error reading packet %s: %v", filename, err)
					}
					continue
				}

				// Call the callback with the packet
				go callback(packet, filename)
			}
		}

		time.Sleep(50 * time.Millisecond)
	}
}

// CleanupSession is deprecated - packets are now persistent and never deleted
func (ph *PacketHandler) CleanupSession() error {
	if ph.verbose {
		log.Printf("PacketHandler: Session %s complete - packets preserved for future reference", ph.sessionID)
	}
	return nil
}

// GetNewPacketFiles returns packet files that haven't been processed yet
func (ph *PacketHandler) GetNewPacketFiles(direction string, processedFiles map[string]bool) ([]string, error) {
	allFiles, err := ph.GetPacketFiles(direction)
	if err != nil {
		return nil, err
	}

	var newFiles []string
	for _, filename := range allFiles {
		if !processedFiles[filename] {
			newFiles = append(newFiles, filename)
		}
	}

	return newFiles, nil
}

// WalkPacketDir walks through packet directory and processes files
func (ph *PacketHandler) WalkPacketDir(callback func(packet *Packet, filename string) error) error {
	return filepath.WalkDir(ph.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		// Check if file belongs to this session
		base := filepath.Base(path)
		if !strings.HasPrefix(base, ph.sessionID+"_") {
			return nil
		}

		packet, err := ph.ReadPacket(path)
		if err != nil {
			if ph.verbose {
				log.Printf("PacketHandler: Error reading packet %s: %v", path, err)
			}
			return nil // Continue processing other files
		}

		return callback(packet, path)
	})
}
