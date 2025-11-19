package internal

import (
	"net/netip"
	"testing"

	v2 "github.com/livekit/media-sdk/sdp/v2"
)

func TestProcessPolyOffer(t *testing.T) {
	remoteAddr := netip.MustParseAddr("192.168.1.100")

	bfcp := &v2.BFCPMedia{
		Port:         9,
		ConnectionIP: remoteAddr,
		FloorCtrl:    "c-s",
		ConferenceID: 12345,
		UserID:       1,
		Setup:        "active",
		Connection:   "new",
		Floors: []v2.BFCPFloor{
			{FloorID: 1, MediaStream: 10},
		},
	}

	sdp := &v2.SDP{
		Addr: remoteAddr,
		ScreenShareVideo: &v2.SDPMedia{
			Label:   "10",
			Content: "slides",
		},
	}

	config, err := ProcessPolyOffer(bfcp, sdp)
	if err != nil {
		t.Fatalf("Failed to process Poly offer: %v", err)
	}

	if config.Port != 9 {
		t.Errorf("Expected port 9, got %d", config.Port)
	}

	if config.SetupRole != "active" {
		t.Errorf("Expected setup active, got %s", config.SetupRole)
	}

	if config.FloorControl != "c-s" {
		t.Errorf("Expected floor control c-s, got %s", config.FloorControl)
	}

	if config.FloorID != 1 {
		t.Errorf("Expected floor ID 1, got %d", config.FloorID)
	}

	if config.MediaStream != 10 {
		t.Errorf("Expected media stream 10, got %d", config.MediaStream)
	}

	if config.Label != "10" {
		t.Errorf("Expected label 10, got %s", config.Label)
	}

	if config.Content != "slides" {
		t.Errorf("Expected content slides, got %s", config.Content)
	}
}

func TestCreatePolyAnswer(t *testing.T) {
	localAddr := netip.MustParseAddr("192.168.1.200")

	offer := &PolyConfig{
		Port:         9,
		ConferenceID: 12345,
		UserID:       1,
		FloorID:      1,
		MediaStream:  10,
		SetupRole:    "active",
		FloorControl: "c-s",
		Label:        "10",
	}

	answer := CreatePolyAnswer(offer, localAddr, 9)

	if answer.Port != 9 {
		t.Errorf("Expected port 9, got %d", answer.Port)
	}

	if answer.SetupRole != "passive" {
		t.Errorf("Expected setup passive, got %s", answer.SetupRole)
	}

	if answer.Content != "slides" {
		t.Errorf("Expected content slides, got %s", answer.Content)
	}

	if answer.ConferenceID != offer.ConferenceID {
		t.Error("Conference ID should match offer")
	}

	if answer.FloorID != offer.FloorID {
		t.Error("Floor ID should match offer")
	}
}

func TestCreatePolyAnswer_Actpass(t *testing.T) {
	localAddr := netip.MustParseAddr("192.168.1.200")

	offer := &PolyConfig{
		Port:         9,
		ConferenceID: 12345,
		UserID:       1,
		FloorID:      1,
		MediaStream:  10,
		SetupRole:    "actpass",
		FloorControl: "c-s",
	}

	answer := CreatePolyAnswer(offer, localAddr, 9)

	if answer.SetupRole != "passive" {
		t.Errorf("Expected passive for actpass offer, got %s", answer.SetupRole)
	}
}

func TestApplyPolyDefaults(t *testing.T) {
	config := &PolyConfig{}

	ApplyPolyDefaults(config)

	if config.Port != 9 {
		t.Errorf("Expected default port 9, got %d", config.Port)
	}

	if config.FloorControl != "c-s" {
		t.Errorf("Expected default floor control c-s, got %s", config.FloorControl)
	}

	if config.Content != "slides" {
		t.Errorf("Expected default content slides, got %s", config.Content)
	}
}

func TestIsPolyPattern(t *testing.T) {
	tests := []struct {
		name     string
		bfcp     *v2.BFCPMedia
		expected bool
	}{
		{
			name: "poly active + c-s",
			bfcp: &v2.BFCPMedia{
				Setup:     "active",
				FloorCtrl: "c-s",
			},
			expected: true,
		},
		{
			name: "poly actpass + c-s",
			bfcp: &v2.BFCPMedia{
				Setup:     "actpass",
				FloorCtrl: "c-s",
			},
			expected: true,
		},
		{
			name: "not poly - passive",
			bfcp: &v2.BFCPMedia{
				Setup:     "passive",
				FloorCtrl: "c-s",
			},
			expected: false,
		},
		{
			name: "not poly - wrong floor ctrl",
			bfcp: &v2.BFCPMedia{
				Setup:     "active",
				FloorCtrl: "s-only",
			},
			expected: false,
		},
		{
			name:     "nil bfcp",
			bfcp:     nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPolyPattern(tt.bfcp)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestToBFCPMedia(t *testing.T) {
	addr := netip.MustParseAddr("192.168.1.100")

	config := &PolyConfig{
		Port:         9,
		Addr:         addr,
		ConferenceID: 12345,
		UserID:       1,
		FloorID:      1,
		MediaStream:  10,
		SetupRole:    "passive",
		FloorControl: "c-s",
	}

	bfcp := ToBFCPMedia(config)

	if bfcp == nil {
		t.Fatal("Expected non-nil BFCPMedia")
	}

	if bfcp.Port != 9 {
		t.Errorf("Expected port 9, got %d", bfcp.Port)
	}

	if bfcp.Setup != "passive" {
		t.Errorf("Expected setup passive, got %s", bfcp.Setup)
	}

	if bfcp.Connection != "new" {
		t.Errorf("Expected connection new, got %s", bfcp.Connection)
	}

	if len(bfcp.Floors) != 1 {
		t.Fatalf("Expected 1 floor, got %d", len(bfcp.Floors))
	}

	if bfcp.Floors[0].FloorID != 1 {
		t.Errorf("Expected floor ID 1, got %d", bfcp.Floors[0].FloorID)
	}

	if bfcp.Floors[0].MediaStream != 10 {
		t.Errorf("Expected media stream 10, got %d", bfcp.Floors[0].MediaStream)
	}
}
