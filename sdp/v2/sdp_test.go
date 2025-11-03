package v2

import (
	"net/netip"
	"testing"

	_ "github.com/livekit/media-sdk/dtmf"  // Register DTMF codec
	_ "github.com/livekit/media-sdk/g711"  // Register PCMU/PCMA
	_ "github.com/livekit/media-sdk/g722"  // Register G722
	_ "github.com/livekit/media-sdk/opus" // Register Opus
	"github.com/stretchr/testify/require"
)

func TestCapabilityFiltering(t *testing.T) {
	// Offer with multiple codecs, including unsupported VP9
	offerSDP := `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Test
c=IN IP4 192.168.1.1
t=0 0
m=video 9000 RTP/AVP 96 97 98
a=rtpmap:96 VP8/90000
a=rtpmap:97 VP9/90000
a=rtpmap:98 H264/90000
a=sendrecv
`

	session, err := NewSDP([]byte(offerSDP))
	require.NoError(t, err)
	require.NotNil(t, session.Video)

	// VP9 should be filtered out
	codecNames := []string{}
	for _, codec := range session.Video.Codecs {
		codecNames = append(codecNames, codec.Name)
	}

	require.Contains(t, codecNames, "VP8")
	require.Contains(t, codecNames, "H264")
	require.NotContains(t, codecNames, "VP9", "VP9 should be filtered out")

	// Marshal back and verify VP9 is gone
	answerData, err := session.Marshal()
	require.NoError(t, err)

	answerStr := string(answerData)
	require.Contains(t, answerStr, "VP8")
	require.Contains(t, answerStr, "H264")
	require.NotContains(t, answerStr, "VP9", "VP9 should not appear in marshaled SDP")
}

func TestAudioCodecNegotiation(t *testing.T) {
	offerSDP := `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Test
c=IN IP4 192.168.1.1
t=0 0
m=audio 9000 RTP/AVP 0 8 101
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:101 telephone-event/8000
a=fmtp:101 0-16
a=sendrecv
`

	session, err := NewSDP([]byte(offerSDP))
	require.NoError(t, err)
	require.NotNil(t, session.Audio)

	// Should have PCMU, PCMA, and telephone-event
	require.Len(t, session.Audio.Codecs, 3)

	// Best codec should be selected
	require.NotNil(t, session.Audio.Codec)
}

func TestDisabledMediaSection(t *testing.T) {
	offerSDP := `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Test
c=IN IP4 192.168.1.1
t=0 0
m=video 0 RTP/AVP 96
a=inactive
`

	session, err := NewSDP([]byte(offerSDP))
	require.NoError(t, err)
	require.NotNil(t, session.Video)
	require.True(t, session.Video.Disabled)
	require.Equal(t, DirectionInactive, session.Video.Direction)
}

func TestUnsupportedCodecsRejected(t *testing.T) {
	// Offer with only unsupported codecs
	offerSDP := `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Test
c=IN IP4 192.168.1.1
t=0 0
m=video 9000 RTP/AVP 99
a=rtpmap:99 VP9/90000
a=sendrecv
`

	session, err := NewSDP([]byte(offerSDP))
	require.NoError(t, err)
	require.NotNil(t, session.Video)

	// No supported codecs, should be marked as disabled
	require.True(t, session.Video.Disabled)
	require.Equal(t, DirectionInactive, session.Video.Direction)

	// Marshal back and verify port is 0
	answerData, err := session.Marshal()
	require.NoError(t, err)

	answerStr := string(answerData)
	require.Contains(t, answerStr, "m=video 0", "Media should be rejected with port 0")
	require.Contains(t, answerStr, "a=inactive")
}

func TestCryptoNegotiation(t *testing.T) {
	offerSDP := `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Test
c=IN IP4 192.168.1.1
t=0 0
m=audio 9000 RTP/SAVP 0
a=rtpmap:0 PCMU/8000
a=crypto:1 AES_CM_128_HMAC_SHA1_80 inline:d0RmdmcmVCspeEc3QGZiNWpVLFJhQX1cfHAwJSoj
a=sendrecv
`

	session, err := NewSDP([]byte(offerSDP))
	require.NoError(t, err)
	require.NotNil(t, session.Audio)

	// Should have crypto profile
	require.NotEmpty(t, session.Audio.Security.Profiles)
	require.Equal(t, "AES_CM_128_HMAC_SHA1_80", string(session.Audio.Security.Profiles[0].Profile))
}

func TestGenerateAnswer(t *testing.T) {
	offerSDP := `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Test
c=IN IP4 192.168.1.1
t=0 0
m=audio 9000 RTP/AVP 0 8
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=sendrecv
`

	offer, err := NewSDP([]byte(offerSDP))
	require.NoError(t, err)

	// Generate answer
	answerData, mc, err := offer.GenerateAnswer(mustParseAddr("192.168.1.2"), 10000, 0)
	require.NoError(t, err)
	require.NotEmpty(t, answerData)

	// Check MediaConfig
	require.NotNil(t, mc.Audio.Codec)
	require.Equal(t, "192.168.1.2", mc.Local.Addr().String())
	require.Equal(t, uint16(10000), mc.Local.Port())
	require.Equal(t, "192.168.1.1", mc.Remote.Addr().String())
	require.Equal(t, uint16(9000), mc.Remote.Port())

	// Verify answer SDP
	answerStr := string(answerData)
	require.Contains(t, answerStr, "192.168.1.2")
	require.Contains(t, answerStr, "m=audio 10000")
	require.Contains(t, answerStr, "a=rtpmap:")

	// CRITICAL: rtpmap must not include channels for mono audio
	require.Contains(t, answerStr, "a=rtpmap:0 PCMU/8000\r\n", "rtpmap should be PCMU/8000, not PCMU/8000/8000")
}

func TestGenerateAnswerStaticPayload(t *testing.T) {
	// Test with static payload types (no rtpmap in offer)
	offerSDP := `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Test
c=IN IP4 192.168.1.1
t=0 0
m=audio 9000 RTP/AVP 0 8 101
a=rtpmap:101 telephone-event/8000
a=sendrecv
`

	offer, err := NewSDP([]byte(offerSDP))
	require.NoError(t, err)

	// Generate answer
	answerData, mc, err := offer.GenerateAnswer(mustParseAddr("192.168.1.2"), 10000, 0)
	require.NoError(t, err)
	require.NotEmpty(t, answerData)

	// Check MediaConfig
	require.NotNil(t, mc.Audio.Codec)

	// Verify answer SDP
	answerStr := string(answerData)
	t.Logf("Answer SDP:\n%s", answerStr)

	// CRITICAL: rtpmap for static types must not include channels
	require.Contains(t, answerStr, "a=rtpmap:0 PCMU/8000\r\n", "rtpmap should be PCMU/8000, not PCMU/8000/8000")
	require.NotContains(t, answerStr, "PCMU/8000/8000", "rtpmap must not have /8000/8000")
}

func TestMarshalFlow(t *testing.T) {
	// Test the livekit-sip flow: parse offer, set params, marshal answer
	offerSDP := `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Test
c=IN IP4 192.168.1.1
t=0 0
m=audio 9000 RTP/AVP 0 8 101
a=rtpmap:101 telephone-event/8000
a=sendrecv
`

	// Parse offer
	offer, err := NewSDP([]byte(offerSDP))
	require.NoError(t, err)
	require.NotNil(t, offer.Audio)

	// Set answer parameters (like livekit-sip does)
	offer.Addr = mustParseAddr("192.168.1.2")
	offer.Audio.Port = 10000
	offer.Audio.Security.Mode = 0 // No encryption

	// Get v1 config
	mc, err := offer.V1MediaConfig()
	require.NoError(t, err)
	require.NotNil(t, mc.Audio.Codec)

	// CRITICAL: Check that Remote address is extracted from original offer
	require.True(t, mc.Remote.IsValid(), "Remote address should be set")
	require.Equal(t, "192.168.1.1", mc.Remote.Addr().String(), "Remote IP should match offer")
	require.Equal(t, uint16(9000), mc.Remote.Port(), "Remote port should match offer")

	// Check Local address matches what we set
	require.Equal(t, "192.168.1.2", mc.Local.Addr().String(), "Local IP should match answer")
	require.Equal(t, uint16(10000), mc.Local.Port(), "Local port should match answer")

	// Marshal answer
	answerData, err := offer.Marshal()
	require.NoError(t, err)
	require.NotEmpty(t, answerData)

	// Verify answer SDP
	answerStr := string(answerData)
	t.Logf("Answer SDP:\n%s", answerStr)

	// CRITICAL: rtpmap for static types must not include channels
	require.Contains(t, answerStr, "a=rtpmap:0 PCMU/8000\r\n", "rtpmap should be PCMU/8000, not PCMU/8000/8000")
	require.NotContains(t, answerStr, "PCMU/8000/8000", "rtpmap must not have /8000/8000")
	require.Contains(t, answerStr, "m=audio 10000", "Port should be 10000")
	require.Contains(t, answerStr, "c=IN IP4 192.168.1.2", "Address should be 192.168.1.2")
}

func TestMultipleMediaSections(t *testing.T) {
	offerSDP := `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Test
c=IN IP4 192.168.1.1
t=0 0
m=audio 9000 RTP/AVP 0
a=rtpmap:0 PCMU/8000
a=sendrecv
m=video 9002 RTP/AVP 96
a=rtpmap:96 VP8/90000
a=sendrecv
`

	session, err := NewSDP([]byte(offerSDP))
	require.NoError(t, err)
	require.NotNil(t, session.Audio)
	require.NotNil(t, session.Video)

	require.Equal(t, uint16(9000), session.Audio.Port)
	require.Equal(t, uint16(9002), session.Video.Port)
}

func mustParseAddr(s string) netip.Addr {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		panic(err)
	}
	return addr
}
