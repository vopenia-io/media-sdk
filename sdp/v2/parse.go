package v2

import (
	"encoding/base64"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/pion/sdp/v3"

	media "github.com/livekit/media-sdk"
	"github.com/livekit/media-sdk/rtp"
	sdpv1 "github.com/livekit/media-sdk/sdp"
	"github.com/livekit/media-sdk/srtp"
)

// parseDirection extracts the direction from SDP attributes.
func parseDirection(attrs []sdp.Attribute) Direction {
	for _, attr := range attrs {
		switch attr.Key {
		case "sendrecv":
			return DirectionSendRecv
		case "sendonly":
			return DirectionSendOnly
		case "recvonly":
			return DirectionRecvOnly
		case "inactive":
			return DirectionInactive
		}
	}
	return DirectionSendRecv // default
}

// parseMID extracts the MID attribute value.
func parseMID(attrs []sdp.Attribute) string {
	for _, attr := range attrs {
		if attr.Key == "mid" {
			return attr.Value
		}
	}
	return ""
}

// parseRTPMap parses an rtpmap attribute value.
// Format: "<payload> <encoding>/<clock>[/<channels>]"
// Returns: payloadType, name, clockRate, channels, error
func parseRTPMap(value string) (uint8, string, uint32, uint16, error) {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 {
		return 0, "", 0, 0, fmt.Errorf("invalid rtpmap: %q", value)
	}

	pt, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", 0, 0, fmt.Errorf("invalid payload type: %q", parts[0])
	}

	// Parse encoding/clock/channels
	encoding := parts[1]
	encParts := strings.Split(encoding, "/")
	if len(encParts) < 2 {
		return 0, "", 0, 0, fmt.Errorf("invalid encoding: %q", encoding)
	}

	name := encParts[0]
	clockRate, err := strconv.ParseUint(encParts[1], 10, 32)
	if err != nil {
		return 0, "", 0, 0, fmt.Errorf("invalid clock rate: %q", encParts[1])
	}

	channels := uint16(0)
	if len(encParts) >= 3 {
		ch, err := strconv.ParseUint(encParts[2], 10, 16)
		if err != nil {
			return 0, "", 0, 0, fmt.Errorf("invalid channels: %q", encParts[2])
		}
		channels = uint16(ch)
	}

	return uint8(pt), name, uint32(clockRate), channels, nil
}

// parseFMTP parses an fmtp attribute value.
// Format: "<payload> <params>"
// Returns: payloadType, params map
func parseFMTP(value string) (uint8, map[string]string, error) {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 {
		return 0, nil, fmt.Errorf("invalid fmtp: %q", value)
	}

	pt, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, nil, fmt.Errorf("invalid payload type: %q", parts[0])
	}

	params := make(map[string]string)
	for _, param := range strings.Split(parts[1], ";") {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}
		kv := strings.SplitN(param, "=", 2)
		if len(kv) == 2 {
			params[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		} else {
			params[kv[0]] = ""
		}
	}

	return uint8(pt), params, nil
}

// parseCrypto parses a crypto attribute value.
// Format: "<tag> <crypto-suite> inline:<key>||..."
func parseCrypto(value string) (*srtp.Profile, error) {
	value = strings.TrimSpace(value)
	parts := strings.SplitN(value, " ", 3)
	if len(parts) != 3 {
		return nil, nil // Ignore malformed crypto lines
	}

	tag, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid crypto tag: %q", parts[0])
	}

	suite := srtp.ProtectionProfile(parts[1])
	keyParam := parts[2]

	// Extract inline key
	keyParam, ok := strings.CutPrefix(keyParam, "inline:")
	if !ok {
		return nil, nil // Only support inline keys
	}

	// Decode base64 key material
	keyMaterial, err := base64.RawStdEncoding.DecodeString(keyParam)
	if err != nil {
		// Fallback to padded encoding
		keyMaterial, err = base64.StdEncoding.DecodeString(keyParam)
		if err != nil {
			return nil, fmt.Errorf("invalid crypto key: %q", keyParam)
		}
	}

	// Split key and salt based on suite
	var key, salt []byte
	if sp, err := suite.Parse(); err == nil {
		keyLen, err := sp.KeyLen()
		if err != nil {
			return nil, err
		}
		if len(keyMaterial) < keyLen {
			return nil, fmt.Errorf("key material too short: %d < %d", len(keyMaterial), keyLen)
		}
		key = keyMaterial[:keyLen]
		salt = keyMaterial[keyLen:]
	} else {
		key = keyMaterial
	}

	return &srtp.Profile{
		Index:   tag,
		Profile: suite,
		Key:     key,
		Salt:    salt,
	}, nil
}

// parseConnectionAddress extracts the IP address from session or media connection.
func parseConnectionAddress(sd *sdp.SessionDescription, md *sdp.MediaDescription) (netip.Addr, error) {
	var conn *sdp.ConnectionInformation
	if md != nil && md.ConnectionInformation != nil {
		conn = md.ConnectionInformation
	} else if sd != nil && sd.ConnectionInformation != nil {
		conn = sd.ConnectionInformation
	}

	if conn == nil || conn.Address == nil {
		return netip.Addr{}, fmt.Errorf("no connection information")
	}

	addr, err := netip.ParseAddr(conn.Address.Address)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid address: %q", conn.Address.Address)
	}

	return addr, nil
}

// resolveCodec tries to find a matching media.Codec for the given SDP codec info.
func resolveCodec(name string, clockRate uint32) media.Codec {
	// Try exact match by name
	if c := sdpv1.CodecByName(name); c != nil {
		return c
	}

	// For some codecs, the SDP name includes clock rate
	fullName := fmt.Sprintf("%s/%d", name, clockRate)
	if c := sdpv1.CodecByName(fullName); c != nil {
		return c
	}

	return nil
}

// parseMediaSection parses a single MediaDescription and filters unsupported codecs.
// This function applies capability-based pruning.
func parseMediaSection(sd *sdp.SessionDescription, md *sdp.MediaDescription, kind MediaKind) (*MediaSection, error) {
	section := &MediaSection{
		Description: md,
		Kind:        kind,
		Port:        uint16(md.MediaName.Port.Value),
		Direction:   parseDirection(md.Attributes),
		MID:         parseMID(md.Attributes),
		Disabled:    md.MediaName.Port.Value == 0,
	}

	// Parse RTCP port if present
	for _, attr := range md.Attributes {
		if attr.Key == "rtcp" {
			if port, err := strconv.Atoi(attr.Value); err == nil {
				section.RTCPPort = uint16(port)
			}
			break
		}
	}

	// Build codec map from rtpmap attributes
	rtpMap := make(map[uint8]*Codec)
	fmtpMap := make(map[uint8]map[string]string)
	rtcpFBMap := make(map[uint8][]sdp.Attribute)

	// Parse all rtpmap attributes
	for _, attr := range md.Attributes {
		if attr.Key == "rtpmap" {
			pt, name, clockRate, channels, err := parseRTPMap(attr.Value)
			if err != nil {
				continue
			}

			rtpMap[pt] = &Codec{
				PayloadType: pt,
				Name:        name,
				ClockRate:   clockRate,
				Channels:    channels,
				FMTP:        make(map[string]string),
				RTCPFB:      []sdp.Attribute{},
			}
		}
	}

	// Parse fmtp attributes
	for _, attr := range md.Attributes {
		if attr.Key == "fmtp" {
			pt, params, err := parseFMTP(attr.Value)
			if err != nil {
				continue
			}
			fmtpMap[pt] = params
			if codec, ok := rtpMap[pt]; ok {
				codec.FMTP = params
			}
		}
	}

	// Parse rtcp-fb attributes
	for _, attr := range md.Attributes {
		if attr.Key == "rtcp-fb" {
			parts := strings.SplitN(attr.Value, " ", 2)
			if len(parts) < 1 {
				continue
			}
			pt, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			rtcpFBMap[uint8(pt)] = append(rtcpFBMap[uint8(pt)], attr)
			if codec, ok := rtpMap[uint8(pt)]; ok {
				codec.RTCPFB = append(codec.RTCPFB, attr)
			}
		}
	}

	// Filter codecs based on capabilities and build the supported list
	supportedPayloads := []string{}
	for _, format := range md.MediaName.Formats {
		pt, err := strconv.Atoi(format)
		if err != nil {
			continue
		}

		codec, hasRTPMap := rtpMap[uint8(pt)]
		if !hasRTPMap {
			// Static payload type without rtpmap - try to resolve from rtp package
			mediaCodec, ok := rtp.CodecByPayloadType(byte(pt)).(rtp.AudioCodec)
			if !ok {
				continue
			}

			info := mediaCodec.Info()
			fmtp := fmtpMap[uint8(pt)]
			if fmtp == nil {
				fmtp = make(map[string]string)
			}
			rtcpfb := rtcpFBMap[uint8(pt)]
			if rtcpfb == nil {
				rtcpfb = []sdp.Attribute{}
			}

			// Parse SDPName which may be "PCMU/8000" format
			codecName := info.SDPName
			if slashIdx := strings.Index(codecName, "/"); slashIdx > 0 {
				codecName = codecName[:slashIdx]
			}

			codec = &Codec{
				PayloadType: uint8(pt),
				Name:        codecName,
				ClockRate:   uint32(info.RTPClockRate),
				Channels:    0, // Typically 1 for audio
				FMTP:        fmtp,
				RTCPFB:      rtcpfb,
				Codec:       mediaCodec,
			}
			rtpMap[uint8(pt)] = codec
		}

		// Check if codec is supported (for video, use whitelist)
		if kind == MediaKindVideo && !isCodecSupported(kind, codec.Name, codec.ClockRate, codec.Channels, codec.FMTP) {
			// Unsupported video codec - prune it
			continue
		}

		// Resolve to media.Codec if not already resolved
		if codec.Codec == nil {
			mediaCodec := resolveCodec(codec.Name, codec.ClockRate)
			if mediaCodec != nil {
				codec.Codec = mediaCodec
			} else if kind == MediaKindAudio {
				// For audio, if we can't resolve it, skip it
				continue
			}
		}

		section.Codecs = append(section.Codecs, codec)
		supportedPayloads = append(supportedPayloads, format)
	}

	// If no supported codecs remain, mark section as disabled
	if len(section.Codecs) == 0 {
		section.Disabled = true
		section.Direction = DirectionInactive
	}

	// Parse crypto attributes
	cryptoProfiles := []srtp.Profile{}
	for _, attr := range md.Attributes {
		if attr.Key == "crypto" {
			profile, err := parseCrypto(attr.Value)
			if err != nil {
				continue
			}
			if profile != nil {
				cryptoProfiles = append(cryptoProfiles, *profile)
			}
		}
	}

	// Determine encryption mode
	encMode := sdpv1.EncryptionNone
	if len(cryptoProfiles) > 0 {
		encMode = sdpv1.EncryptionAllow
	}

	section.Security = Security{
		Mode:     encMode,
		Profiles: cryptoProfiles,
	}

	return section, nil
}
