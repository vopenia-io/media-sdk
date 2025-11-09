package v2

import (
	"strconv"

	v1 "github.com/livekit/media-sdk/sdp"
)

// AddBandwidth adds bandwidth constraints to a media section
// line: 0 for AS (Application-Specific), 1 for TIAS (Transport Independent Application Specific)
// kbps: bandwidth in kilobits per second
func (m *SDPMedia) AddBandwidth(line int, kbps uint32) {
	switch line {
	case 0: // AS bandwidth
		m.BandwidthAS = kbps
	case 1: // TIAS bandwidth (convert kbps to bps)
		m.BandwidthTIAS = kbps * 1000
	}
}

// H264Profile represents an H.264 profile configuration
type H264Profile struct {
	ProfileLevelID   string // e.g., "428020" for Constrained Baseline Level 3.2
	PacketizationMode int    // 0 or 1
	MaxFS            int    // Maximum frame size in macroblocks
	MaxMBPS          int    // Maximum macroblock processing rate
}

// Common H.264 profiles
var (
	H264ProfileBaseline32 = H264Profile{
		ProfileLevelID:   "42801f", // Baseline Level 3.1
		PacketizationMode: 1,
		MaxFS:            3600,
		MaxMBPS:          108000,
	}

	H264ProfileMain32 = H264Profile{
		ProfileLevelID:   "4d001f", // Main Level 3.1
		PacketizationMode: 1,
		MaxFS:            3600,
		MaxMBPS:          108000,
	}

	H264ProfileHigh32 = H264Profile{
		ProfileLevelID:   "64001f", // High Level 3.1
		PacketizationMode: 1,
		MaxFS:            3600,
		MaxMBPS:          108000,
	}

	H264ProfileConstrainedBaseline32 = H264Profile{
		ProfileLevelID:   "428020", // Constrained Baseline Level 3.2
		PacketizationMode: 1,
		MaxFS:            5120,
		MaxMBPS:          216000,
	}
)

// AddH264Attributes adds H.264 codec with specified profile to the media section
// profile: profile string like "428020" or use predefined H264Profile
func (m *SDPMedia) AddH264Attributes(profile H264Profile) error {
	// Get H.264 codec from registry
	h264Codec := v1.CodecByName("H264/90000")
	if h264Codec == nil {
		return v1.ErrNoCommonMedia
	}

	// Build codec with FMTP parameters
	fmtp := make(map[string]string)
	if profile.ProfileLevelID != "" {
		fmtp["profile-level-id"] = profile.ProfileLevelID
	}
	if profile.PacketizationMode >= 0 {
		fmtp["packetization-mode"] = strconv.Itoa(profile.PacketizationMode)
	}
	if profile.MaxFS > 0 {
		fmtp["max-fs"] = strconv.Itoa(profile.MaxFS)
	}
	if profile.MaxMBPS > 0 {
		fmtp["max-mbps"] = strconv.Itoa(profile.MaxMBPS)
	}

	codec, err := (&Codec{}).Builder().
		SetPayloadType(96). // Dynamic payload type for H.264
		SetCodec(h264Codec).
		SetFMTP(fmtp).
		Build()

	if err != nil {
		return err
	}

	m.Codecs = append(m.Codecs, codec)
	if m.Codec == nil {
		m.Codec = codec
	}

	return nil
}

// AddBFCPFloors adds multiple BFCP floor IDs to the BFCP media section
func AddBFCPFloors(bfcp *BFCPMedia, floors []BFCPFloor) {
	if bfcp == nil {
		return
	}

	bfcp.Floors = append(bfcp.Floors, floors...)

	// For backward compatibility, set the first floor as the deprecated single floor
	if len(floors) > 0 && bfcp.FloorID == 0 {
		bfcp.FloorID = floors[0].FloorID
		bfcp.MediaStream = floors[0].MediaStream
	}
}
