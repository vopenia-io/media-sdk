package internal

import (
	"fmt"
	"net/netip"

	v2 "github.com/livekit/media-sdk/sdp/v2"
)

const (
	PolyDefaultPort = 9
	PolySetupActive = "active"
	PolySetupPassive = "passive"
	PolyFloorCtrl = "c-s"
	PolyContentSlides = "slides"
	PolyConnectionNew = "new"
)

type PolyConfig struct {
	Port         uint16
	Addr         netip.Addr
	ConferenceID uint32
	UserID       uint16
	FloorID      uint16
	MediaStream  uint16
	SetupRole    string
	FloorControl string
	Content      string
	Label        string
}

func ProcessPolyOffer(bfcp *v2.BFCPMedia, sdp *v2.SDP) (*PolyConfig, error) {
	if bfcp == nil {
		return nil, fmt.Errorf("bfcp media is nil")
	}

	config := &PolyConfig{
		Port:         bfcp.Port,
		Addr:         bfcp.ConnectionIP,
		ConferenceID: bfcp.ConferenceID,
		UserID:       bfcp.UserID,
		SetupRole:    bfcp.Setup,
		FloorControl: bfcp.FloorCtrl,
	}

	if len(bfcp.Floors) > 0 {
		config.FloorID = bfcp.Floors[0].FloorID
		config.MediaStream = bfcp.Floors[0].MediaStream
	} else {
		config.FloorID = bfcp.FloorID
		config.MediaStream = bfcp.MediaStream
	}

	if config.MediaStream > 0 {
		label := fmt.Sprintf("%d", config.MediaStream)

		if sdp.Video != nil && sdp.Video.Label == label {
			config.Label = label
			config.Content = sdp.Video.Content
		}

		if sdp.ScreenShareVideo != nil && sdp.ScreenShareVideo.Label == label {
			config.Label = label
			config.Content = sdp.ScreenShareVideo.Content
		}
	}

	return config, nil
}

func CreatePolyAnswer(offer *PolyConfig, localAddr netip.Addr, localPort uint16) *PolyConfig {
	answer := &PolyConfig{
		Port:         localPort,
		Addr:         localAddr,
		ConferenceID: offer.ConferenceID,
		UserID:       offer.UserID,
		FloorID:      offer.FloorID,
		MediaStream:  offer.MediaStream,
		FloorControl: offer.FloorControl,
		Label:        offer.Label,
		Content:      PolyContentSlides,
	}

	answer.SetupRole = reversePolySetup(offer.SetupRole)

	return answer
}

func reversePolySetup(setup string) string {
	switch setup {
	case PolySetupActive:
		return PolySetupPassive
	case "actpass":
		return PolySetupPassive
	case PolySetupPassive:
		return PolySetupActive
	default:
		return PolySetupPassive
	}
}

func ApplyPolyDefaults(config *PolyConfig) {
	if config.Port == 0 {
		config.Port = PolyDefaultPort
	}

	if config.FloorControl == "" {
		config.FloorControl = PolyFloorCtrl
	}

	if config.Content == "" {
		config.Content = PolyContentSlides
	}
}

func IsPolyPattern(bfcp *v2.BFCPMedia) bool {
	if bfcp == nil {
		return false
	}

	hasActiveSetup := bfcp.Setup == PolySetupActive || bfcp.Setup == "actpass"
	hasClientServerMode := bfcp.FloorCtrl == PolyFloorCtrl

	return hasActiveSetup && hasClientServerMode
}

func ToBFCPMedia(config *PolyConfig) *v2.BFCPMedia {
	if config == nil {
		return nil
	}

	return &v2.BFCPMedia{
		Port:         config.Port,
		ConnectionIP: config.Addr,
		FloorCtrl:    config.FloorControl,
		ConferenceID: config.ConferenceID,
		UserID:       config.UserID,
		FloorID:      config.FloorID,
		MediaStream:  config.MediaStream,
		Setup:        config.SetupRole,
		Connection:   PolyConnectionNew,
		Attributes:   make(map[string]string),
		Floors: []v2.BFCPFloor{
			{
				FloorID:     config.FloorID,
				MediaStream: config.MediaStream,
			},
		},
	}
}
