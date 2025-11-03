package v2

import (
	"strings"

	media "github.com/livekit/media-sdk"
)

// CodecCapability represents a supported codec configuration.
type CodecCapability struct {
	Name      string            // Codec name (case-insensitive match)
	ClockRate uint32            // Expected clock rate (0 = any)
	Channels  uint16            // Expected channels (0 = any)
	FMTP      map[string]string // Required FMTP parameters (nil = no requirements)
}

// Matches checks if a codec from SDP matches this capability.
func (cap *CodecCapability) Matches(name string, clockRate uint32, channels uint16, fmtp map[string]string) bool {
	if !strings.EqualFold(cap.Name, name) {
		return false
	}
	if cap.ClockRate != 0 && cap.ClockRate != clockRate {
		return false
	}
	if cap.Channels != 0 && cap.Channels != channels {
		return false
	}
	// FMTP matching: all required parameters must be present
	for k, v := range cap.FMTP {
		if fmtpVal, ok := fmtp[k]; !ok || fmtpVal != v {
			return false
		}
	}
	return true
}

// getAudioCapabilities returns the list of supported audio codecs.
// This is derived from media.EnabledCodecs() at runtime.
func getAudioCapabilities() []CodecCapability {
	codecs := media.EnabledCodecs()
	caps := make([]CodecCapability, 0, len(codecs))

	for _, c := range codecs {
		info := c.Info()
		// Only include audio codecs (non-zero RTPClockRate)
		if info.RTPClockRate > 0 {
			cap := CodecCapability{
				Name:      info.SDPName,
				ClockRate: uint32(info.RTPClockRate),
				// Most audio codecs have implicit channels = 1, except when explicitly in name
				Channels: 0, // Accept any
			}
			caps = append(caps, cap)
		}
	}
	return caps
}

// getVideoCapabilities returns the list of supported video codecs.
// For now this is a static list: H264, VP8
func getVideoCapabilities() []CodecCapability {
	return []CodecCapability{
		{
			Name:      "H264",
			ClockRate: 90000,
		},
		{
			Name:      "VP8",
			ClockRate: 90000,
		},
	}
}

// isCodecSupported checks if the given codec is supported for the media kind.
// For audio, we check if we can resolve the codec from the registry.
// For video, we use a static whitelist.
func isCodecSupported(kind MediaKind, name string, clockRate uint32, channels uint16, fmtp map[string]string) bool {
	switch kind {
	case MediaKindAudio:
		// For audio, accept any codec that can be resolved
		// This automatically respects media.EnabledCodecs()
		return true // We'll check resolvability later in parseMediaSection
	case MediaKindVideo:
		// For video, use explicit whitelist
		caps := getVideoCapabilities()
		for _, cap := range caps {
			if cap.Matches(name, clockRate, channels, fmtp) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
