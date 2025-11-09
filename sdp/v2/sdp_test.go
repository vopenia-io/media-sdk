package v2

import (
	"net/netip"
	"strings"
	"testing"

	v1 "github.com/livekit/media-sdk/sdp"
	_ "github.com/livekit/media-sdk/h264" // Import to register H.264 codec
)

// TestSDPSerialization tests SDP generation with two video m-lines and BFCP
func TestSDPSerialization(t *testing.T) {
	// Create SDP with main video, slides video, and BFCP
	addr := netip.MustParseAddr("192.168.1.100")

	s := &SDP{
		Addr: addr,
	}

	// Build main video media
	mainVideo := &SDPMedia{
		Kind:      MediaKindVideo,
		Port:      5004,
		RTCPPort:  5005,
		Direction: DirectionSendRecv,
		Content:   "main",
	}
	mainVideo.BandwidthAS = 2000    // 2000 kbps
	mainVideo.BandwidthTIAS = 2000000 // 2000000 bps (2 Mbps)

	// Add H.264 codec to main video
	h264Codec := v1.CodecByName("H264/90000")
	if h264Codec == nil {
		t.Fatal("H.264 codec not found")
	}

	mainH264, err := (&Codec{}).Builder().
		SetPayloadType(96).
		SetCodec(h264Codec).
		SetFMTP(map[string]string{
			"profile-level-id":   "428020",
			"packetization-mode": "1",
			"max-fs":             "5120",
			"max-mbps":           "216000",
		}).
		Build()
	if err != nil {
		t.Fatalf("Failed to build main video H.264 codec: %v", err)
	}
	mainVideo.Codecs = append(mainVideo.Codecs, mainH264)
	mainVideo.Codec = mainH264
	s.Video = mainVideo

	// Build slides video media (screen share)
	slidesVideo := &SDPMedia{
		Kind:      MediaKindVideo,
		Port:      5006,
		RTCPPort:  5007,
		Direction: DirectionSendOnly,
		Content:   "slides",
	}
	slidesVideo.BandwidthAS = 4000    // 4000 kbps
	slidesVideo.BandwidthTIAS = 4000000 // 4000000 bps (4 Mbps)

	// Add H.264 codec to slides video
	slidesH264, err := (&Codec{}).Builder().
		SetPayloadType(97).
		SetCodec(h264Codec).
		SetFMTP(map[string]string{
			"profile-level-id":   "428020",
			"packetization-mode": "1",
			"max-fs":             "8192",
			"max-mbps":           "245760",
		}).
		Build()
	if err != nil {
		t.Fatalf("Failed to build slides video H.264 codec: %v", err)
	}
	slidesVideo.Codecs = append(slidesVideo.Codecs, slidesH264)
	slidesVideo.Codec = slidesH264
	s.ScreenShareVideo = slidesVideo

	// Build BFCP media with multiple floor IDs
	bfcp := &BFCPMedia{
		Port:         5008,
		ConnectionIP: addr,
		FloorCtrl:    "c-s",
		ConferenceID: 1,
		UserID:       2,
		Setup:        "passive",
		Connection:   "new",
		Floors: []BFCPFloor{
			{FloorID: 1, MediaStream: 10},
			{FloorID: 2, MediaStream: 11},
		},
	}
	s.BFCP = bfcp

	// Serialize to SDP
	sdpBytes, err := s.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal SDP: %v", err)
	}

	sdpString := string(sdpBytes)
	t.Logf("Generated SDP:\n%s", sdpString)

	// Verify SDP contains expected elements
	if !strings.Contains(sdpString, "m=video 5004") {
		t.Error("SDP missing main video m-line")
	}

	if !strings.Contains(sdpString, "m=video 5006") {
		t.Error("SDP missing slides video m-line")
	}

	if !strings.Contains(sdpString, "m=application 5008") {
		t.Error("SDP missing BFCP m-line")
	}

	// Verify bandwidth lines
	hasAS := strings.Contains(sdpString, "b=AS:")
	hasTIAS := strings.Contains(sdpString, "b=TIAS:")
	if !hasAS || !hasTIAS {
		t.Error("Bandwidth lines: AS/TIAS not found")
	} else {
		t.Log("Bandwidth lines: AS/TIAS found")
	}

	// Verify content attributes
	if !strings.Contains(sdpString, "a=content:main") {
		t.Error("Content attribute 'main' not found")
	}

	if !strings.Contains(sdpString, "a=content:slides") {
		t.Error("Content attribute 'slides' not found")
	}

	// Verify direction attributes
	if !strings.Contains(sdpString, "a=sendrecv") {
		t.Error("Direction attribute 'sendrecv' not found for main video")
	}

	if !strings.Contains(sdpString, "a=sendonly") {
		t.Error("Direction attribute 'sendonly' not found for slides video")
	}

	// Verify H.264 attributes
	hasProfileLevelID := strings.Contains(sdpString, "profile-level-id=428020")
	if !hasProfileLevelID {
		t.Error("H264 fmtp: profile-level-id=428020 not found")
	} else {
		t.Log("H264 fmtp: profile-level-id=428020")
	}

	// Verify BFCP floor IDs
	hasFloor1 := strings.Contains(sdpString, "a=floorid:1")
	hasFloor2 := strings.Contains(sdpString, "a=floorid:2")
	if !hasFloor1 || !hasFloor2 {
		t.Error("Includes floorid:1 and floorid:2: FAILED")
	} else {
		t.Log("Includes floorid:1 and floorid:2")
	}

	// Final test result
	t.Log("SDP serialization: PASS")
}

// TestAddBandwidth tests the AddBandwidth helper function
func TestAddBandwidth(t *testing.T) {
	m := &SDPMedia{
		Kind: MediaKindVideo,
	}

	// Add AS bandwidth
	m.AddBandwidth(0, 1500)
	if m.BandwidthAS != 1500 {
		t.Errorf("Expected BandwidthAS=1500, got %d", m.BandwidthAS)
	}

	// Add TIAS bandwidth
	m.AddBandwidth(1, 2000)
	if m.BandwidthTIAS != 2000000 { // Should be converted to bps
		t.Errorf("Expected BandwidthTIAS=2000000, got %d", m.BandwidthTIAS)
	}
}

// TestAddH264Attributes tests the AddH264Attributes helper function
func TestAddH264Attributes(t *testing.T) {
	m := &SDPMedia{
		Kind: MediaKindVideo,
	}

	err := m.AddH264Attributes(H264ProfileConstrainedBaseline32)
	if err != nil {
		t.Fatalf("Failed to add H.264 attributes: %v", err)
	}

	if len(m.Codecs) == 0 {
		t.Fatal("No codecs added")
	}

	codec := m.Codecs[0]
	if codec.FMTP["profile-level-id"] != "428020" {
		t.Errorf("Expected profile-level-id=428020, got %s", codec.FMTP["profile-level-id"])
	}
}

// TestAddBFCPFloors tests the AddBFCPFloors helper function
func TestAddBFCPFloors(t *testing.T) {
	bfcp := &BFCPMedia{
		Port:      5008,
		FloorCtrl: "c-s",
	}

	floors := []BFCPFloor{
		{FloorID: 1, MediaStream: 10},
		{FloorID: 2, MediaStream: 11},
		{FloorID: 3, MediaStream: 12},
	}

	AddBFCPFloors(bfcp, floors)

	if len(bfcp.Floors) != 3 {
		t.Errorf("Expected 3 floors, got %d", len(bfcp.Floors))
	}

	// Check backward compatibility
	if bfcp.FloorID != 1 || bfcp.MediaStream != 10 {
		t.Errorf("Backward compatibility failed: FloorID=%d, MediaStream=%d", bfcp.FloorID, bfcp.MediaStream)
	}
}

// TestSDPRoundTrip tests parsing and serialization
func TestSDPRoundTrip(t *testing.T) {
	// Create original SDP
	addr := netip.MustParseAddr("10.0.0.1")

	original := &SDP{
		Addr: addr,
	}

	// Add video with all attributes
	video := &SDPMedia{
		Kind:          MediaKindVideo,
		Port:          6000,
		RTCPPort:      6001,
		Direction:     DirectionRecvOnly,
		BandwidthAS:   3000,
		BandwidthTIAS: 3000000,
		Content:       "slides",
	}

	h264Codec := v1.CodecByName("H264/90000")
	codec, _ := (&Codec{}).Builder().
		SetPayloadType(98).
		SetCodec(h264Codec).
		Build()
	video.Codecs = append(video.Codecs, codec)
	video.Codec = codec

	original.Video = video

	// Serialize
	sdpBytes, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Parse back
	parsed, err := NewSDP(sdpBytes)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	t.Logf("Parsed SDP:\n  Video: %v\n  ScreenShareVideo: %v", parsed.Video != nil, parsed.ScreenShareVideo != nil)

	// Verify - check which video field was populated
	var parsedVideo *SDPMedia
	if parsed.ScreenShareVideo != nil {
		parsedVideo = parsed.ScreenShareVideo
	} else if parsed.Video != nil {
		parsedVideo = parsed.Video
	}

	if parsedVideo == nil {
		t.Fatal("Parsed SDP has no video")
	}

	if parsedVideo.BandwidthAS != 3000 {
		t.Errorf("BandwidthAS mismatch: expected 3000, got %d", parsedVideo.BandwidthAS)
	}

	if parsedVideo.BandwidthTIAS != 3000000 {
		t.Errorf("BandwidthTIAS mismatch: expected 3000000, got %d", parsedVideo.BandwidthTIAS)
	}

	if parsedVideo.Content != "slides" {
		t.Errorf("Content mismatch: expected 'slides', got '%s'", parsedVideo.Content)
	}

	if parsedVideo.Direction != DirectionRecvOnly {
		t.Errorf("Direction mismatch: expected 'recvonly', got '%s'", parsedVideo.Direction)
	}
}
