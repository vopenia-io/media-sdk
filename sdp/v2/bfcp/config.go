package bfcp

import (
	"net/netip"

	v2 "github.com/livekit/media-sdk/sdp/v2"
)

// Config is a generic BFCP configuration that hides vendor-specific details.
// It represents the negotiated BFCP parameters in a clean, vendor-agnostic way.
type Config struct {
	// Connection details
	Port uint16
	Addr netip.Addr

	// BFCP protocol parameters
	ConferenceID uint32
	UserID       uint16
	FloorID      uint16
	MediaStream  uint16

	// TCP setup role - "active" means we connect, "passive" means we listen
	SetupRole string

	// Floor control mode - who can request/grant floors
	FloorControl string

	// Content type for the associated video stream (usually "slides")
	Content string

	// Label for correlating BFCP with video streams
	Label string

	// Raw BFCP media for advanced use cases
	raw *v2.BFCPMedia
}

// IsActive returns true if we should initiate the TCP connection (setup:active)
func (c *Config) IsActive() bool {
	return c.SetupRole == "active"
}

// IsPassive returns true if we should listen for TCP connections (setup:passive)
func (c *Config) IsPassive() bool {
	return c.SetupRole == "passive"
}

// IsClient returns true if we're acting as a BFCP client (can request floors)
func (c *Config) IsClient() bool {
	return c.FloorControl == "c-only" || c.FloorControl == "c-s"
}

// IsServer returns true if we're acting as a BFCP server (can grant floors)
func (c *Config) IsServer() bool {
	return c.FloorControl == "s-only" || c.FloorControl == "c-s"
}

// ToBFCPMedia converts the generic config back to the raw BFCPMedia struct
func (c *Config) ToBFCPMedia() *v2.BFCPMedia {
	if c == nil {
		return nil
	}

	return &v2.BFCPMedia{
		Port:         c.Port,
		ConnectionIP: c.Addr,
		FloorCtrl:    c.FloorControl,
		ConferenceID: c.ConferenceID,
		UserID:       c.UserID,
		FloorID:      c.FloorID,
		MediaStream:  c.MediaStream,
		Setup:        c.SetupRole,
		Connection:   "new",
		Attributes:   make(map[string]string),
		Floors: []v2.BFCPFloor{
			{
				FloorID:     c.FloorID,
				MediaStream: c.MediaStream,
			},
		},
	}
}
