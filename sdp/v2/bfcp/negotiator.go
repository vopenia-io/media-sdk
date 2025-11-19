package bfcp

import (
	"fmt"
	"net/netip"

	v2 "github.com/livekit/media-sdk/sdp/v2"
)

// Negotiator handles BFCP offer/answer negotiation with vendor-specific logic.
// It detects the vendor from the SDP and applies appropriate defaults internally.
type Negotiator struct {
	// Local configuration
	localAddr netip.Addr
	localPort uint16

	// Detected vendor
	vendor Vendor

	// Vendor-specific state
	polyDefaults *PolyDefaults
}

// NewNegotiator creates a new BFCP negotiator with the local address and port.
func NewNegotiator(localAddr netip.Addr, localPort uint16) *Negotiator {
	return &Negotiator{
		localAddr: localAddr,
		localPort: localPort,
	}
}

// ProcessOffer processes a BFCP offer from a remote device and returns a generic config.
// It detects the vendor and extracts relevant BFCP parameters.
func (n *Negotiator) ProcessOffer(sdp *v2.SDP) (*Config, error) {
	if sdp == nil || sdp.BFCP == nil {
		return nil, fmt.Errorf("no BFCP media in offer")
	}

	bfcp := sdp.BFCP

	// Detect vendor from SDP patterns
	n.vendor = DetectVendor(bfcp)

	// Extract common BFCP parameters
	config := &Config{
		Port:         bfcp.Port,
		Addr:         bfcp.ConnectionIP,
		ConferenceID: bfcp.ConferenceID,
		UserID:       bfcp.UserID,
		FloorControl: bfcp.FloorCtrl,
		raw:          bfcp,
	}

	// Get floor ID and media stream
	if len(bfcp.Floors) > 0 {
		// Use first floor if multiple are present
		config.FloorID = bfcp.Floors[0].FloorID
		config.MediaStream = bfcp.Floors[0].MediaStream
	} else {
		// Fallback to deprecated fields
		config.FloorID = bfcp.FloorID
		config.MediaStream = bfcp.MediaStream
	}

	// Apply vendor-specific processing
	switch n.vendor {
	case VendorPoly:
		n.applyPolyOfferProcessing(config, bfcp)
	default:
		// Generic processing
		config.SetupRole = bfcp.Setup
	}

	// Look for associated video stream with matching label
	if config.MediaStream > 0 {
		label := fmt.Sprintf("%d", config.MediaStream)

		// Check main video
		if sdp.Video != nil && sdp.Video.Label == label {
			config.Label = label
			config.Content = sdp.Video.Content
		}

		// Check screen share video
		if sdp.ScreenShareVideo != nil && sdp.ScreenShareVideo.Label == label {
			config.Label = label
			config.Content = sdp.ScreenShareVideo.Content
		}
	}

	return config, nil
}

// CreateAnswer creates a BFCP answer based on the processed offer.
// It applies vendor-specific defaults and returns a generic config for the answer.
func (n *Negotiator) CreateAnswer(offerConfig *Config) (*Config, error) {
	if offerConfig == nil {
		return nil, fmt.Errorf("offer config is nil")
	}

	// Create answer config based on offer
	answer := &Config{
		Port:         n.localPort,
		Addr:         n.localAddr,
		ConferenceID: offerConfig.ConferenceID,
		UserID:       offerConfig.UserID,
		FloorID:      offerConfig.FloorID,
		MediaStream:  offerConfig.MediaStream,
		FloorControl: offerConfig.FloorControl,
		Label:        offerConfig.Label,
	}

	// Apply vendor-specific answer logic
	switch n.vendor {
	case VendorPoly:
		n.applyPolyAnswerDefaults(answer, offerConfig)
	default:
		// Generic: reverse the setup role
		answer.SetupRole = reverseSetup(offerConfig.SetupRole)
		answer.Content = "slides"
	}

	return answer, nil
}

// applyPolyOfferProcessing applies Poly-specific logic when processing an offer
func (n *Negotiator) applyPolyOfferProcessing(config *Config, bfcp *v2.BFCPMedia) {
	// Poly clients use setup:active, meaning they initiate the connection
	config.SetupRole = bfcp.Setup

	// Store Poly defaults for answer generation
	defaults := GetPolyDefaults()
	n.polyDefaults = &defaults
}

// applyPolyAnswerDefaults applies Poly-specific defaults when creating an answer
func (n *Negotiator) applyPolyAnswerDefaults(answer *Config, offer *Config) {
	// Poly requires the server to use setup:passive when client uses setup:active
	if offer.SetupRole == "active" {
		answer.SetupRole = "passive"
	} else if offer.SetupRole == "actpass" {
		// Poly sometimes sends actpass but expects passive response
		answer.SetupRole = "passive"
	} else {
		// Default to active if offer is passive
		answer.SetupRole = "active"
	}

	// Poly always uses "slides" for screen share content
	answer.Content = "slides"

	// Keep the same floor control mode
	answer.FloorControl = offer.FloorControl
}

// reverseSetup returns the opposite setup role for generic devices
func reverseSetup(setup string) string {
	switch setup {
	case "active":
		return "passive"
	case "passive":
		return "active"
	case "actpass":
		// When remote offers actpass, we can choose - prefer active
		return "active"
	default:
		return "passive"
	}
}

// GetVendor returns the detected vendor
func (n *Negotiator) GetVendor() Vendor {
	return n.vendor
}
