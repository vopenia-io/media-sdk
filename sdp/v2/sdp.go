package v2

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net/netip"

	"github.com/pion/sdp/v3"
)

func NewSDP(sdpData []byte) (*SDP, error) {
	s := &SDP{}
	if err := s.Unmarshal(sdpData); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SDP) Unmarshal(sdpData []byte) error {
	var psdp sdp.SessionDescription
	if err := psdp.Unmarshal(sdpData); err != nil {
		return err
	}
	if err := s.FromPion(psdp); err != nil {
		return err
	}
	return nil
}

func (s *SDP) Marshal() ([]byte, error) {
	psdp, err := s.ToPion()
	if err != nil {
		return nil, err
	}
	return psdp.Marshal()
}

func (s *SDP) FromPion(sd sdp.SessionDescription) error {
	addr, err := netip.ParseAddr(sd.Origin.UnicastAddress)
	if err != nil {
		return err
	}
	s.Addr = addr

	for _, md := range sd.MediaDescriptions {
		// Phase 4.2: Check if this is a BFCP application media
		if md.MediaName.Media == "application" {
			// Try to parse as BFCP
			if bfcp, err := parseBFCP(*md, addr); err == nil {
				s.BFCP = bfcp
			}
			// Skip adding to regular media sections
			continue
		}

		sm := &SDPMedia{}
		if err := sm.FromPion(*md); err != nil {
			// Skip unsupported media kinds
			continue
		}
		switch sm.Kind {
		case MediaKindAudio:
			s.Audio = sm
		case MediaKindVideo:
			// Phase 5.3: Check if this is screen share video (content:slides)
			isScreenShare := false
			for _, attr := range md.Attributes {
				if attr.Key == "content" && attr.Value == "slides" {
					isScreenShare = true
					break
				}
			}
			if isScreenShare {
				s.ScreenShareVideo = sm
			} else {
				s.Video = sm
			}
		default:
			// Skip unsupported media kinds
			continue
		}
	}

	return nil
}

func (s *SDP) ToPion() (sdp.SessionDescription, error) {
	sessId := rand.Uint64() // TODO: do we need to track these?

	sd := sdp.SessionDescription{
		Version: 0,
		Origin: sdp.Origin{
			Username:       "-",
			SessionID:      sessId,
			SessionVersion: sessId,
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: s.Addr.String(),
		},
		SessionName: "LiveKit",
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &sdp.Address{Address: s.Addr.String()},
		},
		TimeDescriptions: []sdp.TimeDescription{
			{
				Timing: sdp.Timing{
					StartTime: 0,
					StopTime:  0,
				},
			},
		},
	}
	if s.Audio != nil {
		audioMD, err := s.Audio.ToPion()
		if err != nil {
			return sd, fmt.Errorf("failed to convert audio media: %w", err)
		}
		sd.MediaDescriptions = append(sd.MediaDescriptions, &audioMD)
	}
	if s.Video != nil {
		videoMD, err := s.Video.ToPion()
		if err != nil {
			return sd, fmt.Errorf("failed to convert video media: %w", err)
		}
		sd.MediaDescriptions = append(sd.MediaDescriptions, &videoMD)
	}
	// Phase 5.3: Add screen share video with content:slides attribute
	if s.ScreenShareVideo != nil {
		screenShareMD, err := s.ScreenShareVideo.ToPion()
		if err != nil {
			return sd, fmt.Errorf("failed to convert screen share video media: %w", err)
		}
		// Content attribute is already added in ToPion() if Content field is set
		sd.MediaDescriptions = append(sd.MediaDescriptions, &screenShareMD)
	}
	// Phase 4.3: Add BFCP application media to SDP answer
	if s.BFCP != nil {
		bfcpMD := bfcpToPion(s.BFCP)
		sd.MediaDescriptions = append(sd.MediaDescriptions, &bfcpMD)
	}

	return sd, nil
}

func (s *SDP) Clone() *SDP {
	if s == nil {
		return nil
	}
	clone := &SDP{
		Addr: s.Addr,
	}
	if s.Audio != nil {
		clone.Audio = s.Audio.Clone()
	}
	if s.Video != nil {
		clone.Video = s.Video.Clone()
	}
	// Phase 5.3: Clone screen share video if present
	if s.ScreenShareVideo != nil {
		clone.ScreenShareVideo = s.ScreenShareVideo.Clone()
	}
	// Phase 4.2: Clone BFCP if present
	if s.BFCP != nil {
		bfcpClone := *s.BFCP
		if s.BFCP.Attributes != nil {
			bfcpClone.Attributes = make(map[string]string)
			for k, v := range s.BFCP.Attributes {
				bfcpClone.Attributes[k] = v
			}
		}
		clone.BFCP = &bfcpClone
	}
	return clone
}

func (s *SDP) Builder() *SDPBuilder {
	return &SDPBuilder{s: s.Clone()}
}

type SDPBuilder struct {
	errs []error
	s    *SDP
}

var _ interface {
	Builder[*SDP]
	SetAddress(netip.Addr) *SDPBuilder
	SetVideo(func(b *SDPMediaBuilder) (*SDPMedia, error)) *SDPBuilder
	SetAudio(func(b *SDPMediaBuilder) (*SDPMedia, error)) *SDPBuilder
	SetScreenShareVideo(func(b *SDPMediaBuilder) (*SDPMedia, error)) *SDPBuilder
} = (*SDPBuilder)(nil)

func (b *SDPBuilder) Build() (*SDP, error) {
	if len(b.errs) > 0 {
		return nil, fmt.Errorf("failed to build SDP with %d errors: %w", len(b.errs), errors.Join(b.errs...))
	}
	return b.s, nil
}

func (b *SDPBuilder) SetAddress(addr netip.Addr) *SDPBuilder {
	b.s.Addr = addr
	return b
}

func (b *SDPBuilder) SetVideo(fn func(b *SDPMediaBuilder) (*SDPMedia, error)) *SDPBuilder {
	mb := &SDPMediaBuilder{m: &SDPMedia{}}
	mb.SetKind(MediaKindVideo)
	m, err := fn(mb)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.s.Video = m
	return b
}

func (b *SDPBuilder) SetAudio(fn func(b *SDPMediaBuilder) (*SDPMedia, error)) *SDPBuilder {
	mb := &SDPMediaBuilder{m: &SDPMedia{}}
	mb.SetKind(MediaKindAudio)
	m, err := fn(mb)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.s.Audio = m
	return b
}

// SetScreenShareVideo sets screen share video media in the SDP
// Phase 5.3: Build screen share video with content:slides
func (b *SDPBuilder) SetScreenShareVideo(fn func(b *SDPMediaBuilder) (*SDPMedia, error)) *SDPBuilder {
	mb := &SDPMediaBuilder{m: &SDPMedia{}}
	mb.SetKind(MediaKindVideo)
	m, err := fn(mb)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.s.ScreenShareVideo = m
	return b
}

// SetBFCP sets BFCP application media in the SDP answer
// Phase 4.3: Build BFCP SDP answer as server
func (b *SDPBuilder) SetBFCP(bfcp *BFCPMedia) *SDPBuilder {
	// Allow clearing BFCP by setting it to nil
	b.s.BFCP = bfcp
	return b
}

// parseBFCP extracts BFCP (Binary Floor Control Protocol) parameters from an application media description.
// Phase 4.2: Parse BFCP from SIP device SDP
func parseBFCP(md sdp.MediaDescription, defaultAddr netip.Addr) (*BFCPMedia, error) {
	// Check if this is a BFCP media type
	if md.MediaName.Media != "application" {
		return nil, fmt.Errorf("not an application media")
	}

	// Check for TCP/BFCP or TCP/TLS/BFCP protocol
	proto := ""
	for _, p := range md.MediaName.Protos {
		proto += p + "/"
	}
	if !contains(proto, "BFCP") {
		return nil, fmt.Errorf("not a BFCP protocol: %s", proto)
	}

	bfcp := &BFCPMedia{
		Port:       uint16(md.MediaName.Port.Value),
		Attributes: make(map[string]string),
		Floors:     []BFCPFloor{},
	}

	// Get connection address (prefer media-level, fallback to session-level)
	if md.ConnectionInformation != nil && md.ConnectionInformation.Address != nil {
		if addr, err := netip.ParseAddr(md.ConnectionInformation.Address.Address); err == nil {
			bfcp.ConnectionIP = addr
		}
	}
	if !bfcp.ConnectionIP.IsValid() {
		bfcp.ConnectionIP = defaultAddr
	}

	// Parse BFCP-specific attributes
	for _, attr := range md.Attributes {
		switch attr.Key {
		case "floorctrl":
			bfcp.FloorCtrl = attr.Value
		case "confid":
			if val, err := parseUint32(attr.Value); err == nil {
				bfcp.ConferenceID = val
			}
		case "userid":
			if val, err := parseUint16(attr.Value); err == nil {
				bfcp.UserID = val
			}
		case "floorid":
			// Format: "floorid:<id> mstrm:<stream>"
			floorID, mediaStream := parseFloorID(attr.Value)
			// Store in deprecated fields for backward compatibility
			if bfcp.FloorID == 0 {
				bfcp.FloorID = floorID
				bfcp.MediaStream = mediaStream
			}
			// Also store in new Floors slice
			bfcp.Floors = append(bfcp.Floors, BFCPFloor{
				FloorID:     floorID,
				MediaStream: mediaStream,
			})
		case "setup":
			bfcp.Setup = attr.Value
		case "connection":
			bfcp.Connection = attr.Value
		default:
			// Store other attributes for potential future use
			bfcp.Attributes[attr.Key] = attr.Value
		}
	}

	return bfcp, nil
}

// Helper functions for parsing
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func parseUint32(s string) (uint32, error) {
	var val uint32
	_, err := fmt.Sscanf(s, "%d", &val)
	return val, err
}

func parseUint16(s string) (uint16, error) {
	var val uint16
	_, err := fmt.Sscanf(s, "%d", &val)
	return val, err
}

// parseFloorID parses "floorid:<id> mstrm:<stream>" format
func parseFloorID(value string) (floorID uint16, mediaStream uint16) {
	// Example: "1 mstrm:3" or just "1"
	var fid, mstr uint16
	if n, _ := fmt.Sscanf(value, "%d mstrm:%d", &fid, &mstr); n >= 1 {
		floorID = fid
		mediaStream = mstr
	}
	return
}

// bfcpToPion converts BFCPMedia to pion MediaDescription
// Phase 4.3: Build BFCP SDP answer as server
func bfcpToPion(bfcp *BFCPMedia) sdp.MediaDescription {
	md := sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media: "application",
			Port: sdp.RangedPort{
				Value: int(bfcp.Port),
			},
			Protos:  []string{"TCP", "BFCP"},
			Formats: []string{"*"},
		},
	}

	// Add connection information if specified
	if bfcp.ConnectionIP.IsValid() {
		md.ConnectionInformation = &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &sdp.Address{Address: bfcp.ConnectionIP.String()},
		}
	}

	// Add BFCP attributes
	if bfcp.FloorCtrl != "" {
		md.Attributes = append(md.Attributes, sdp.Attribute{Key: "floorctrl", Value: bfcp.FloorCtrl})
	}
	if bfcp.ConferenceID > 0 {
		md.Attributes = append(md.Attributes, sdp.Attribute{Key: "confid", Value: fmt.Sprintf("%d", bfcp.ConferenceID)})
	}
	if bfcp.UserID > 0 {
		md.Attributes = append(md.Attributes, sdp.Attribute{Key: "userid", Value: fmt.Sprintf("%d", bfcp.UserID)})
	}

	// Add multiple floor IDs if available in Floors slice
	if len(bfcp.Floors) > 0 {
		for _, floor := range bfcp.Floors {
			floorIDValue := fmt.Sprintf("%d", floor.FloorID)
			if floor.MediaStream > 0 {
				floorIDValue += fmt.Sprintf(" mstrm:%d", floor.MediaStream)
			}
			md.Attributes = append(md.Attributes, sdp.Attribute{Key: "floorid", Value: floorIDValue})
		}
	} else if bfcp.FloorID > 0 {
		// Fallback to deprecated single floor ID for backward compatibility
		floorIDValue := fmt.Sprintf("%d", bfcp.FloorID)
		if bfcp.MediaStream > 0 {
			floorIDValue += fmt.Sprintf(" mstrm:%d", bfcp.MediaStream)
		}
		md.Attributes = append(md.Attributes, sdp.Attribute{Key: "floorid", Value: floorIDValue})
	}

	if bfcp.Setup != "" {
		md.Attributes = append(md.Attributes, sdp.Attribute{Key: "setup", Value: bfcp.Setup})
	}
	if bfcp.Connection != "" {
		md.Attributes = append(md.Attributes, sdp.Attribute{Key: "connection", Value: bfcp.Connection})
	}

	// Add any additional attributes
	for key, value := range bfcp.Attributes {
		md.Attributes = append(md.Attributes, sdp.Attribute{Key: key, Value: value})
	}

	return md
}
