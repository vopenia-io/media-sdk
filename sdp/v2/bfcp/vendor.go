package bfcp

import (
	v2 "github.com/livekit/media-sdk/sdp/v2"
)

// Vendor represents different SIP device vendors that may have
// different BFCP implementations or requirements.
type Vendor string

const (
	VendorUnknown Vendor = "unknown"
	VendorPoly    Vendor = "poly"
	VendorCisco   Vendor = "cisco"
	VendorGeneric Vendor = "generic"
)

// DetectVendor identifies the device vendor from BFCP SDP patterns.
// Different vendors have different quirks in their BFCP implementations.
func DetectVendor(bfcp *v2.BFCPMedia) Vendor {
	if bfcp == nil {
		return VendorUnknown
	}

	// Poly devices have a distinctive BFCP signature:
	// - TCP/BFCP protocol (not TLS)
	// - setup:active (they initiate the connection)
	// - floorctrl:c-s (client-server mode)
	// - Always use setup:active from client side
	if isPolyDevice(bfcp) {
		return VendorPoly
	}

	// Add other vendor detection logic here as needed
	// For now, treat everything else as generic
	return VendorGeneric
}

// isPolyDevice checks for Poly-specific BFCP patterns
func isPolyDevice(bfcp *v2.BFCPMedia) bool {
	// Poly characteristics:
	// 1. Uses TCP/BFCP (not TLS)
	// 2. Client side uses setup:active
	// 3. Uses floorctrl:c-s (client-server)

	// Check for setup:active - Poly clients always use this
	if bfcp.Setup == "active" {
		// Check for c-s floor control - typical Poly pattern
		if bfcp.FloorCtrl == "c-s" {
			return true
		}
	}

	// Additional Poly pattern: sometimes they send actpass but expect passive response
	// This is a common Poly quirk
	if bfcp.Setup == "actpass" && bfcp.FloorCtrl == "c-s" {
		return true
	}

	return false
}

// PolyDefaults returns the default BFCP configuration for Poly devices.
// Poly has specific requirements for BFCP that differ from the standard.
type PolyDefaults struct {
	// Poly always expects the server to use setup:passive
	ServerSetup string

	// Poly uses TCP without TLS
	UseTLS bool

	// Default content type for screen share
	ContentType string

	// Default floor control mode
	FloorControl string
}

// GetPolyDefaults returns Poly-specific defaults
func GetPolyDefaults() PolyDefaults {
	return PolyDefaults{
		ServerSetup:  "passive",
		UseTLS:       false,
		ContentType:  "slides",
		FloorControl: "c-s", // Client-server mode
	}
}
