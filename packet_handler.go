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
	baseDir   string
	sessionID string
	verbose   bool
}

// NewPacketHandler creates a new packet handler
func NewPacketHandler(baseDir, sessionID string, verbose bool) *PacketHandler {
	return &PacketHandler{
		baseDir:   baseDir,
		sessionID: sessionID,
		verbose:   verbose,
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

// ReadPacket reads a packet from a file
func (ph *PacketHandler) ReadPacket(filename string) (*Packet, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read packet file %s: %v", filename, err)
	}

	packet, err := FromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse packet from %s: %v", filename, err)
	}

	if ph.verbose {
		log.Printf("PacketHandler: Read packet %s from %s", packet.ID, filename)
	}

	return packet, nil
}

// GetPacketFiles returns all packet files for a specific direction, sorted by sequence
func (ph *PacketHandler) GetPacketFiles(direction string) ([]string, error) {
	pattern := fmt.Sprintf("%s_%s_*.json", ph.sessionID, direction)
	matches, err := filepath.Glob(filepath.Join(ph.baseDir, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to glob packet files: %v", err)
	}

	// Sort by sequence number
	sort.Slice(matches, func(i, j int) bool {
		return extractSequence(matches[i]) < extractSequence(matches[j])
	})

	return matches, nil
}

// GetAllPacketFiles returns all packet files for the session, sorted by timestamp
func (ph *PacketHandler) GetAllPacketFiles() ([]string, error) {
	pattern := fmt.Sprintf("%s_*.json", ph.sessionID)
	matches, err := filepath.Glob(filepath.Join(ph.baseDir, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to glob packet files: %v", err)
	}

	// Sort by modification time
	sort.Slice(matches, func(i, j int) bool {
		info1, err1 := os.Stat(matches[i])
		info2, err2 := os.Stat(matches[j])
		if err1 != nil || err2 != nil {
			return false
		}
		return info1.ModTime().Before(info2.ModTime())
	})

	return matches, nil
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

// CleanupSession removes all packet files for the session
func (ph *PacketHandler) CleanupSession() error {
	pattern := fmt.Sprintf("%s_*.json", ph.sessionID)
	matches, err := filepath.Glob(filepath.Join(ph.baseDir, pattern))
	if err != nil {
		return fmt.Errorf("failed to glob packet files for cleanup: %v", err)
	}

	for _, filename := range matches {
		if err := os.Remove(filename); err != nil {
			if ph.verbose {
				log.Printf("PacketHandler: Warning: failed to remove %s: %v", filename, err)
			}
		}
	}

	if ph.verbose && len(matches) > 0 {
		log.Printf("PacketHandler: Cleaned up %d packet files for session %s", len(matches), ph.sessionID)
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

// extractSequence extracts sequence number from filename
func extractSequence(filename string) uint64 {
	base := filepath.Base(filename)
	parts := strings.Split(base, "_")
	if len(parts) < 3 {
		return 0
	}

	var seq uint64
	fmt.Sscanf(parts[2], "%06d", &seq)
	return seq
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
