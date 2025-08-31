package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// PacketType defines the type of packet
type PacketType string

const (
	PacketTypeOpen      PacketType = "open"      // Session open/handshake
	PacketTypeData      PacketType = "data"      // Data transfer
	PacketTypeClose     PacketType = "close"     // Session close
	PacketTypeAck       PacketType = "ack"       // Acknowledgment
	PacketTypeHeartbeat PacketType = "heartbeat" // Keep-alive
)

// Packet represents a communication packet between client and server
type Packet struct {
	// Packet identification
	ID        string    `json:"id"`         // Unique packet ID
	SessionID string    `json:"session_id"` // Session identifier
	Timestamp time.Time `json:"timestamp"`  // Packet creation time

	// Packet type and sequence
	Type     PacketType `json:"type"`     // Type of packet
	Sequence uint64     `json:"sequence"` // Sequence number for ordering

	// Data payload
	Data     string `json:"data,omitempty"` // Base64 encoded data
	DataSize int    `json:"data_size"`      // Original data size before encoding

	// Control flow metadata
	TargetHost string `json:"target_host,omitempty"` // Target host (for open packets)
	TargetPort int    `json:"target_port,omitempty"` // Target port (for open packets)
	ErrorMsg   string `json:"error,omitempty"`       // Error message (for close/error packets)

	// Flow control
	WindowSize int    `json:"window_size,omitempty"` // Flow control window size
	AckID      string `json:"ack_id,omitempty"`      // ID of packet being acknowledged

	// Metadata
	ClientAddr string `json:"client_addr,omitempty"` // Original client address
	Direction  string `json:"direction"`             // "client_to_server" or "server_to_client"
}

// NewPacket creates a new packet with basic fields populated
func NewPacket(sessionID string, packetType PacketType, direction string) *Packet {
	return &Packet{
		ID:        generatePacketID(),
		SessionID: sessionID,
		Timestamp: time.Now(),
		Type:      packetType,
		Direction: direction,
	}
}

// SetData encodes data as base64 and sets it in the packet
func (p *Packet) SetData(data []byte) {
	if len(data) > 0 {
		p.Data = base64.StdEncoding.EncodeToString(data)
		p.DataSize = len(data)
	}
}

// GetData decodes the base64 data and returns the original bytes
func (p *Packet) GetData() ([]byte, error) {
	if p.Data == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(p.Data)
}

// ToJSON converts the packet to JSON bytes
func (p *Packet) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// FromJSON creates a packet from JSON bytes
func FromJSON(data []byte) (*Packet, error) {
	var packet Packet
	err := json.Unmarshal(data, &packet)
	return &packet, err
}

// generatePacketID generates a unique packet ID
func generatePacketID() string {
	return fmt.Sprintf("pkt_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond()%1000000)
}

// GetPacketFilename generates a SHA256-based filename for the packet
func (p *Packet) GetPacketFilename(baseDir string) string {
	// Generate JSON content first
	jsonData, err := p.ToJSON()
	if err != nil {
		// Fallback to a deterministic name if JSON fails
		return fmt.Sprintf("%s/%s_%s_%06d_%s.json", 
			baseDir, p.SessionID, p.Direction, p.Sequence, p.ID)
	}
	
	// Generate SHA256 hash of the JSON content
	hash := sha256.Sum256(jsonData)
	hashString := hex.EncodeToString(hash[:])
	
	return fmt.Sprintf("%s/%s.json", baseDir, hashString)
}

// CreateOpenPacket creates a session open packet
func CreateOpenPacket(sessionID, direction, targetHost string, targetPort int, clientAddr string) *Packet {
	packet := NewPacket(sessionID, PacketTypeOpen, direction)
	packet.TargetHost = targetHost
	packet.TargetPort = targetPort
	packet.ClientAddr = clientAddr
	packet.Sequence = 0
	return packet
}

// CreateDataPacket creates a data packet
func CreateDataPacket(sessionID, direction string, sequence uint64, data []byte) *Packet {
	packet := NewPacket(sessionID, PacketTypeData, direction)
	packet.Sequence = sequence
	packet.SetData(data)
	return packet
}

// CreateClosePacket creates a session close packet
func CreateClosePacket(sessionID, direction string, sequence uint64, errorMsg string) *Packet {
	packet := NewPacket(sessionID, PacketTypeClose, direction)
	packet.Sequence = sequence
	packet.ErrorMsg = errorMsg
	return packet
}

// CreateAckPacket creates an acknowledgment packet
func CreateAckPacket(sessionID, direction string, ackID string) *Packet {
	packet := NewPacket(sessionID, PacketTypeAck, direction)
	packet.AckID = ackID
	return packet
}

// CreateHeartbeatPacket creates a heartbeat packet
func CreateHeartbeatPacket(sessionID, direction string) *Packet {
	return NewPacket(sessionID, PacketTypeHeartbeat, direction)
}
