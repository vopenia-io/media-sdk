package v2

import (
	"net/netip"

	"github.com/pion/sdp/v3"

	media "github.com/livekit/media-sdk"
	sdpv1 "github.com/livekit/media-sdk/sdp"
	"github.com/livekit/media-sdk/srtp"
)

// Session wraps a pion SessionDescription and the interpreted media sections.
// The raw SDP remains available for round-tripping while Media holds
// higher-level details extracted from each m= block.
type Session struct {
	Description sdp.SessionDescription
	Addr        netip.Addr
	Audio       *MediaSection
	Video       *MediaSection
}

// MediaSection describes a single m= section while reusing pion's representation
// for raw attributes and payloads.
type MediaSection struct {
	Description *sdp.MediaDescription // Description references the underlying pion media description.
	MID         string                // MID mirrors the a=mid attribute when present.
	Kind        MediaKind             // Kind is the media type (audio, video, application, ...).
	Direction   Direction             // Direction derives from a=sendrecv/sendonly/recvonly/inactive. Defaults to sendrecv.
	Disabled    bool                  // Disabled is true when the port is zero (rejected m=).
	Codecs      []*Codec              // Codecs lists payload formats mapped onto media.Codec entries.
	Codec       *Codec                // PreferredCodec is the selected codec for this track.
	Security    Security              // Security captures SRTP profiles signaled for the media section.
	Port        uint16                // Port is the media port from the m= line.
	RTCPPort    uint16                // RTCPPort is the RTCP port from the m= line. (0 mean not specified)
}

// MediaKind is a simple string alias for the SDP media name.
type MediaKind string

const (
	MediaKindAudio       MediaKind = "audio"
	MediaKindVideo       MediaKind = "video"
	MediaKindApplication MediaKind = "application"
	MediaKindData        MediaKind = "data"
)

// Direction captures the media direction attribute.
type Direction string

const (
	DirectionSendRecv Direction = "sendrecv"
	DirectionSendOnly Direction = "sendonly"
	DirectionRecvOnly Direction = "recvonly"
	DirectionInactive Direction = "inactive"
)

// Codec ties a payload type to a media.Codec while retaining SDP fmtp/rtcp-fb data.
type Codec struct {
	PayloadType uint8
	Name        string
	Codec       media.Codec
	ClockRate   uint32
	Channels    uint16
	FMTP        map[string]string
	RTCPFB      []sdp.Attribute
}

// MediaSection still exposes the raw pion attributes via Description.Attributes,
// so additional data such as ICE/DTLS parameters can be retrieved on demand.

// Security keeps track of SRTP negotiation details.
type Security struct {
	Mode     sdpv1.Encryption
	Profiles []srtp.Profile
}
