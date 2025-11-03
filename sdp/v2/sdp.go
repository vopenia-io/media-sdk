package v2

import (
	"fmt"
	"net/netip"
	"strings"

	v1 "github.com/livekit/media-sdk/sdp"
	"github.com/livekit/media-sdk/rtp"
	"github.com/livekit/media-sdk/srtp"
	"github.com/pion/sdp/v3"
)

func NewSDP(sdpData []byte) (*Session, error) {
	s := &Session{}
	if err := s.Unmarshal(sdpData); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) Unmarshal(sdpData []byte) error {
	if err := s.Description.Unmarshal(sdpData); err != nil {
		return err
	}
	if err := s.FromSDP(s.Description); err != nil {
		return err
	}
	return nil
}

// FromSDP populates the structure from a pion SessionDescription.
// it does not modify the raw pion SessionDescription and overwrites any existing data.
// It parses all media sections, filters unsupported codecs, and extracts structured data.
func (s *Session) FromSDP(sd sdp.SessionDescription) error {
	s.Description = sd
	s.Audio = nil
	s.Video = nil

	// Parse connection address
	if sd.ConnectionInformation != nil && sd.ConnectionInformation.Address != nil {
		if addr, err := parseConnectionAddress(&sd, nil); err == nil {
			s.Addr = addr
		}
	}

	// Parse each media section
	for _, md := range sd.MediaDescriptions {
		if md == nil {
			continue
		}

		kind := MediaKind(md.MediaName.Media)

		// Only process audio and video
		if kind != MediaKindAudio && kind != MediaKindVideo {
			continue
		}

		section, err := parseMediaSection(&sd, md, kind)
		if err != nil {
			// Skip invalid media sections
			continue
		}

		// Store in appropriate field
		switch kind {
		case MediaKindAudio:
			if s.Audio == nil {
				s.Audio = section
			}
		case MediaKindVideo:
			if s.Video == nil {
				s.Video = section
			}
		}
	}

	return s.SelectCodecs()
}

func (s *Session) Marshal() ([]byte, error) {
	if err := s.ToSDP(); err != nil {
		return nil, err
	}
	return s.Description.Marshal()
}

// ToSDP takes the data in the structure and syncs it to the pion SessionDescription and MediaDescription.
// it preserves any raw attributes that are not explicitly modeled.
// This applies codec filtering to the underlying pion structures.
func (s *Session) ToSDP() error {
	// Update session-level fields
	if s.Addr.IsValid() {
		addrStr := s.Addr.String()
		// Update origin
		s.Description.Origin.UnicastAddress = addrStr
		// Update connection
		if s.Description.ConnectionInformation != nil && s.Description.ConnectionInformation.Address != nil {
			s.Description.ConnectionInformation.Address.Address = addrStr
		}
	}

	// Update audio media section
	if s.Audio != nil {
		if err := s.Audio.syncToDescription(); err != nil {
			return err
		}
	}

	// Update video media section
	if s.Video != nil {
		if err := s.Video.syncToDescription(); err != nil {
			return err
		}
	}

	return nil
}

// syncToDescription syncs the MediaSection state back to the underlying pion MediaDescription.
// This is where we apply pruning: remove unsupported codecs from the SDP.
func (m *MediaSection) syncToDescription() error {
	if m.Description == nil {
		return nil
	}

	md := m.Description

	// If disabled, set port to 0 and direction to inactive
	if m.Disabled {
		md.MediaName.Port.Value = 0
		// Remove sendrecv/sendonly/recvonly and set inactive
		md.Attributes = filterAttributes(md.Attributes, func(attr sdp.Attribute) bool {
			return attr.Key != "sendrecv" && attr.Key != "sendonly" &&
				attr.Key != "recvonly" && attr.Key != "inactive"
		})
		md.Attributes = append(md.Attributes, sdp.Attribute{Key: "inactive"})
		return nil
	}

	// Update port
	if m.Port != 0 {
		md.MediaName.Port.Value = int(m.Port)
	}

	// Build set of supported payload types
	supportedPTs := make(map[string]bool)
	for _, codec := range m.Codecs {
		supportedPTs[fmt.Sprint(codec.PayloadType)] = true
	}

	// Filter formats to only include supported codecs
	filteredFormats := []string{}
	for _, format := range md.MediaName.Formats {
		if supportedPTs[format] {
			filteredFormats = append(filteredFormats, format)
		}
	}
	md.MediaName.Formats = filteredFormats

	// Rebuild codec attributes (rtpmap, fmtp, rtcp-fb) from our Codecs list
	// This ensures static payload types get proper rtpmap even if not in offer
	filteredAttrs := []sdp.Attribute{}

	// Keep non-codec attributes
	for _, attr := range md.Attributes {
		switch attr.Key {
		case "rtpmap", "fmtp", "rtcp-fb":
			// Skip - we'll regenerate these
			continue
		default:
			filteredAttrs = append(filteredAttrs, attr)
		}
	}

	// Generate rtpmap, fmtp, rtcp-fb for all our codecs
	for _, codec := range m.Codecs {
		// Generate rtpmap - only include channels if > 1
		rtpmapValue := fmt.Sprintf("%d %s/%d", codec.PayloadType, codec.Name, codec.ClockRate)
		if codec.Channels > 1 {
			rtpmapValue = fmt.Sprintf("%d %s/%d/%d", codec.PayloadType, codec.Name, codec.ClockRate, codec.Channels)
		}
		filteredAttrs = append(filteredAttrs, sdp.Attribute{Key: "rtpmap", Value: rtpmapValue})

		// Add FMTP if present
		if len(codec.FMTP) > 0 {
			fmtpParts := []string{}
			for k, v := range codec.FMTP {
				if v != "" {
					fmtpParts = append(fmtpParts, fmt.Sprintf("%s=%s", k, v))
				} else {
					fmtpParts = append(fmtpParts, k)
				}
			}
			if len(fmtpParts) > 0 {
				filteredAttrs = append(filteredAttrs, sdp.Attribute{
					Key:   "fmtp",
					Value: fmt.Sprintf("%d %s", codec.PayloadType, strings.Join(fmtpParts, ";")),
				})
			}
		}

		// Add rtcp-fb if present
		for _, fb := range codec.RTCPFB {
			filteredAttrs = append(filteredAttrs, fb)
		}
	}

	// Add ptime attribute for audio
	if m.Kind == MediaKindAudio {
		filteredAttrs = append(filteredAttrs, sdp.Attribute{Key: "ptime", Value: "20"})
	}

	md.Attributes = filteredAttrs

	// Update direction
	md.Attributes = filterAttributes(md.Attributes, func(attr sdp.Attribute) bool {
		return attr.Key != "sendrecv" && attr.Key != "sendonly" &&
			attr.Key != "recvonly" && attr.Key != "inactive"
	})
	md.Attributes = append(md.Attributes, sdp.Attribute{Key: string(m.Direction)})

	return nil
}

// filterAttributes filters a slice of attributes.
func filterAttributes(attrs []sdp.Attribute, keep func(sdp.Attribute) bool) []sdp.Attribute {
	result := []sdp.Attribute{}
	for _, attr := range attrs {
		if keep(attr) {
			result = append(result, attr)
		}
	}
	return result
}

func (s *Session) SelectCodecs() error {
	if s.Audio != nil {
		if err := s.Audio.SelectCodec(); err != nil {
			return err
		}
	}
	if s.Video != nil {
		if err := s.Video.SelectCodec(); err != nil {
			return err
		}
	}
	return nil
}

// SelectCodec finds the best codec according to priority rules set by Codec.Info().Priority.
func (m *MediaSection) SelectCodec() error {
	if m.Disabled || len(m.Codecs) == 0 {
		return nil
	}

	var bestCodec *Codec
	var bestPriority int

	for _, codec := range m.Codecs {
		if codec.Codec == nil {
			continue
		}

		info := codec.Codec.Info()
		if bestCodec == nil || info.Priority > bestPriority {
			bestCodec = codec
			bestPriority = info.Priority
		}
	}

	m.Codec = bestCodec
	return nil
}

// V1MediaConfig converts the SDP to a v1 MediaConfig structure.
// It only contain audio as video support was added in v2.
func (s *Session) V1MediaConfig() (v1.MediaConfig, error) {
	var cfg v1.MediaConfig

	if s.Audio == nil {
		return cfg, v1.ErrNoCommonMedia
	}

	audio := s.Audio
	if audio.Disabled {
		return cfg, v1.ErrNoCommonMedia
	}

	if audio.Codec == nil || audio.Codec.Codec == nil {
		return cfg, v1.ErrNoCommonMedia
	}

	// Convert audio codec
	audioCodec, ok := audio.Codec.Codec.(rtp.AudioCodec)
	if !ok {
		return cfg, v1.ErrNoCommonMedia
	}

	cfg.Audio = v1.AudioConfig{
		Codec:    audioCodec,
		Type:     audio.Codec.PayloadType,
		DTMFType: 0, // Will be set if DTMF codec is found
	}

	// Look for DTMF codec
	for _, codec := range audio.Codecs {
		if codec.Name == "telephone-event" {
			cfg.Audio.DTMFType = codec.PayloadType
			break
		}
	}

	// Set addresses
	// Local address comes from the Session's Addr field
	if s.Addr.IsValid() {
		cfg.Local = netip.AddrPortFrom(s.Addr, audio.Port)
	}

	// Remote address will be set externally (from the original offer)
	// We don't have remote info in the answer generation phase

	// Convert crypto
	if len(audio.Security.Profiles) > 0 {
		// Select the first profile for now
		// In a real negotiation, this would be matched against offer
		profile := audio.Security.Profiles[0]
		sp, err := profile.Profile.Parse()
		if err != nil {
			return cfg, err
		}

		cfg.Crypto = &srtp.Config{
			Profile: sp,
			Keys: srtp.SessionKeys{
				LocalMasterKey:   profile.Key,
				LocalMasterSalt:  profile.Salt,
				RemoteMasterKey:  nil, // Will be set during negotiation
				RemoteMasterSalt: nil,
			},
		}
	}

	return cfg, nil
}

// Apply takes a pion SessionDescription (typically an answer) and applies its data to the structure.
// This is used when we sent an offer and received an answer.
// It updates the local Session with the negotiated parameters from the answer.
func (s *Session) Apply(sd sdp.SessionDescription) error {
	// Parse the answer SDP
	answer := &Session{}
	if err := answer.FromSDP(sd); err != nil {
		return err
	}

	// Apply audio answer
	if s.Audio != nil && answer.Audio != nil {
		if err := s.Audio.applyAnswer(answer.Audio); err != nil {
			return err
		}
	}

	// Apply video answer
	if s.Video != nil && answer.Video != nil {
		if err := s.Video.applyAnswer(answer.Video); err != nil {
			return err
		}
	}

	// Update address if provided
	if answer.Addr.IsValid() {
		s.Addr = answer.Addr
	}

	return nil
}

// applyAnswer applies the answer media section to this offer media section.
// This negotiates the final codec and crypto parameters.
func (m *MediaSection) applyAnswer(answer *MediaSection) error {
	// If answer disabled this media, mark as disabled
	if answer.Disabled {
		m.Disabled = true
		m.Direction = DirectionInactive
		return nil
	}

	// Find the selected codec from answer in our offer
	if answer.Codec != nil {
		// Find matching codec in our list
		for _, codec := range m.Codecs {
			if codec.PayloadType == answer.Codec.PayloadType {
				m.Codec = codec
				break
			}
		}
	}

	// Update direction (intersection of offer and answer)
	m.Direction = negotiateDirection(m.Direction, answer.Direction)

	// Update port with answer port
	m.Port = answer.Port

	// Negotiate crypto
	if len(m.Security.Profiles) > 0 && len(answer.Security.Profiles) > 0 {
		// Find common crypto suite
		for _, answerProf := range answer.Security.Profiles {
			for i, offerProf := range m.Security.Profiles {
				if answerProf.Profile == offerProf.Profile {
					// Use the answer's crypto keys (they're responding to us)
					m.Security.Profiles = []srtp.Profile{
						{
							Index:   offerProf.Index,
							Profile: offerProf.Profile,
							Key:     offerProf.Key,
							Salt:    offerProf.Salt,
						},
					}
					// Store remote keys from answer
					m.Security.Profiles[0].Key = offerProf.Key
					m.Security.Profiles[0].Salt = offerProf.Salt
					goto cryptoDone
				}
				_ = i
			}
		}
		// No common crypto suite
		m.Security.Profiles = nil
	cryptoDone:
	}

	return nil
}

// negotiateDirection negotiates the final direction from offer and answer directions.
func negotiateDirection(offer, answer Direction) Direction {
	// Simplified negotiation logic
	if offer == DirectionInactive || answer == DirectionInactive {
		return DirectionInactive
	}
	if offer == DirectionSendRecv && answer == DirectionSendRecv {
		return DirectionSendRecv
	}
	if offer == DirectionSendOnly && (answer == DirectionRecvOnly || answer == DirectionSendRecv) {
		return DirectionSendOnly
	}
	if offer == DirectionRecvOnly && (answer == DirectionSendOnly || answer == DirectionSendRecv) {
		return DirectionRecvOnly
	}
	if (offer == DirectionSendRecv || offer == DirectionRecvOnly) && answer == DirectionSendOnly {
		return DirectionRecvOnly
	}
	if (offer == DirectionSendRecv || offer == DirectionSendOnly) && answer == DirectionRecvOnly {
		return DirectionSendOnly
	}
	return DirectionInactive
}

func (s *Session) ApplySDP(sdpData []byte) error {
	sd := sdp.SessionDescription{}
	if err := sd.Unmarshal(sdpData); err != nil {
		return err
	}
	return s.Apply(sd)
}
