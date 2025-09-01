package main

import (
	"encoding/json"
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

// Packet represents raw TCP data for Nostr events
// All metadata is now stored in Nostr event tags, not in the packet
type Packet struct {
	// Only raw data - all metadata moved to Nostr event tags
	Data []byte `json:"data"` // Raw TCP data (will be base64 encoded in event content)
}

// NewPacket creates a new packet with raw data
func NewPacket(data []byte) *Packet {
	return &Packet{
		Data: data,
	}
}

// SetData sets raw data in the packet
func (p *Packet) SetData(data []byte) {
	p.Data = data
}

// GetData returns the raw data
func (p *Packet) GetData() []byte {
	return p.Data
}

// ToJSON converts the packet to JSON bytes (for debugging only)
func (p *Packet) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// FromJSON creates a packet from JSON bytes (for debugging only)
func FromJSON(data []byte) (*Packet, error) {
	var packet Packet
	err := json.Unmarshal(data, &packet)
	return &packet, err
}

// CreateDataPacket creates a data packet with raw TCP data
func CreateDataPacket(data []byte) *Packet {
	return NewPacket(data)
}

// CreateEmptyPacket creates an empty packet (for control messages)
func CreateEmptyPacket() *Packet {
	return NewPacket(nil)
}
