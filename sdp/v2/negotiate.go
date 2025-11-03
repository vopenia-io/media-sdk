package v2

import (
	"encoding/base64"
	"fmt"
	"net/netip"

	"github.com/pion/sdp/v3"

	"github.com/livekit/media-sdk/rtp"
	sdpv1 "github.com/livekit/media-sdk/sdp"
	"github.com/livekit/media-sdk/srtp"
)

// GenerateAnswer creates an answer SDP from an offer, applying our capabilities and preferences.
// This negotiates codecs, crypto, and other parameters.
func (offer *Session) GenerateAnswer(localAddr netip.Addr, localPort int, encryption sdpv1.Encryption) ([]byte, sdpv1.MediaConfig, error) {
	answer := &Session{
		Addr: localAddr,
	}

	// Copy session-level fields from offer
	answer.Description = sdp.SessionDescription{
		Version: 0,
		Origin: sdp.Origin{
			Username:       "-",
			SessionID:      offer.Description.Origin.SessionID,
			SessionVersion: offer.Description.Origin.SessionID + 2,
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: localAddr.String(),
		},
		SessionName: "LiveKit",
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &sdp.Address{Address: localAddr.String()},
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

	// Negotiate audio
	if offer.Audio != nil {
		audioAnswer, err := negotiateMediaSection(offer.Audio, localPort, encryption)
		if err != nil {
			return nil, sdpv1.MediaConfig{}, err
		}
		answer.Audio = audioAnswer
		answer.Description.MediaDescriptions = append(answer.Description.MediaDescriptions, audioAnswer.Description)
	}

	// Negotiate video
	if offer.Video != nil {
		videoAnswer, err := negotiateMediaSection(offer.Video, localPort+2, encryption)
		if err != nil {
			// Video is optional, just skip it
			videoAnswer = &MediaSection{
				Description: &sdp.MediaDescription{
					MediaName: sdp.MediaName{
						Media: "video",
						Port:  sdp.RangedPort{Value: 0},
					},
				},
				Disabled: true,
			}
		}
		answer.Video = videoAnswer
		answer.Description.MediaDescriptions = append(answer.Description.MediaDescriptions, videoAnswer.Description)
	}

	// Select codecs
	if err := answer.SelectCodecs(); err != nil {
		return nil, sdpv1.MediaConfig{}, err
	}

	// Sync back to pion structures
	if err := answer.ToSDP(); err != nil {
		return nil, sdpv1.MediaConfig{}, err
	}

	// Marshal answer
	answerData, err := answer.Description.Marshal()
	if err != nil {
		return nil, sdpv1.MediaConfig{}, err
	}

	// Build v1 MediaConfig
	var mc sdpv1.MediaConfig
	if answer.Audio != nil && !answer.Audio.Disabled {
		// Get remote address from offer
		var remoteAddr netip.AddrPort
		if offer.Audio != nil {
			remotePort := offer.Audio.Port
			if remotePort == 0 {
				remotePort = uint16(offer.Description.MediaDescriptions[0].MediaName.Port.Value)
			}

			// Get remote IP from offer
			offerAddr := offer.Addr
			if !offerAddr.IsValid() {
				// Try to extract from connection info
				if addr, err := parseConnectionAddress(&offer.Description, offer.Description.MediaDescriptions[0]); err == nil {
					offerAddr = addr
				}
			}

			if offerAddr.IsValid() && remotePort > 0 {
				remoteAddr = netip.AddrPortFrom(offerAddr, remotePort)
			}
		}

		audioCodec, ok := answer.Audio.Codec.Codec.(rtp.AudioCodec)
		if !ok {
			return nil, sdpv1.MediaConfig{}, sdpv1.ErrNoCommonMedia
		}

		mc.Audio = sdpv1.AudioConfig{
			Codec:    audioCodec,
			Type:     answer.Audio.Codec.PayloadType,
			DTMFType: 0,
		}

		// Find DTMF
		for _, codec := range answer.Audio.Codecs {
			if codec.Name == "telephone-event" {
				mc.Audio.DTMFType = codec.PayloadType
				break
			}
		}

		mc.Local = netip.AddrPortFrom(localAddr, uint16(localPort))
		mc.Remote = remoteAddr

		// Negotiate crypto
		if len(answer.Audio.Security.Profiles) > 0 && len(offer.Audio.Security.Profiles) > 0 {
			// Find common profile
			for _, ansProf := range answer.Audio.Security.Profiles {
				for _, offProf := range offer.Audio.Security.Profiles {
					if ansProf.Profile == offProf.Profile {
						sp, err := ansProf.Profile.Parse()
						if err != nil {
							continue
						}

						mc.Crypto = &srtp.Config{
							Profile: sp,
							Keys: srtp.SessionKeys{
								LocalMasterKey:   ansProf.Key,
								LocalMasterSalt:  ansProf.Salt,
								RemoteMasterKey:  offProf.Key,
								RemoteMasterSalt: offProf.Salt,
							},
						}
						goto cryptoDone
					}
				}
			}
		cryptoDone:
		}

		// Check encryption requirements
		if mc.Crypto == nil && encryption == sdpv1.EncryptionRequire {
			return nil, sdpv1.MediaConfig{}, sdpv1.ErrNoCommonCrypto
		}
	}

	return answerData, mc, nil
}

// negotiateMediaSection creates an answer media section from an offer.
func negotiateMediaSection(offerSection *MediaSection, localPort int, encryption sdpv1.Encryption) (*MediaSection, error) {
	if offerSection.Disabled {
		return &MediaSection{
			Description: &sdp.MediaDescription{
				MediaName: sdp.MediaName{
					Media: string(offerSection.Kind),
					Port:  sdp.RangedPort{Value: 0},
				},
			},
			Disabled: true,
		}, nil
	}

	// Select our codec (highest priority from supported)
	var selectedCodec *Codec
	for _, codec := range offerSection.Codecs {
		if codec.Codec == nil {
			continue
		}
		selectedCodec = codec
		break // First one is already the best after filtering
	}

	if selectedCodec == nil {
		return nil, fmt.Errorf("no common codec for %s", offerSection.Kind)
	}

	// Build answer formats (selected codec + DTMF if present)
	formats := []string{fmt.Sprint(selectedCodec.PayloadType)}

	// Build rtpmap value - only include channels if > 1
	rtpmapValue := fmt.Sprintf("%d %s/%d", selectedCodec.PayloadType, selectedCodec.Name, selectedCodec.ClockRate)
	if selectedCodec.Channels > 1 {
		rtpmapValue = fmt.Sprintf("%d %s/%d/%d", selectedCodec.PayloadType, selectedCodec.Name, selectedCodec.ClockRate, selectedCodec.Channels)
	}

	attrs := []sdp.Attribute{
		{Key: "rtpmap", Value: rtpmapValue},
	}

	// Add FMTP if present
	if len(selectedCodec.FMTP) > 0 {
		fmtpParts := []string{}
		for k, v := range selectedCodec.FMTP {
			if v != "" {
				fmtpParts = append(fmtpParts, fmt.Sprintf("%s=%s", k, v))
			} else {
				fmtpParts = append(fmtpParts, k)
			}
		}
		if len(fmtpParts) > 0 {
			attrs = append(attrs, sdp.Attribute{
				Key:   "fmtp",
				Value: fmt.Sprintf("%d %s", selectedCodec.PayloadType, fmtpParts[0]),
			})
		}
	}

	// Add DTMF if present in offer
	var dtmfCodec *Codec
	for _, codec := range offerSection.Codecs {
		if codec.Name == "telephone-event" {
			dtmfCodec = codec
			formats = append(formats, fmt.Sprint(codec.PayloadType))
			attrs = append(attrs, sdp.Attribute{
				Key:   "rtpmap",
				Value: fmt.Sprintf("%d telephone-event/8000", codec.PayloadType),
			})
			attrs = append(attrs, sdp.Attribute{
				Key:   "fmtp",
				Value: fmt.Sprintf("%d 0-16", codec.PayloadType),
			})
			break
		}
	}

	// Negotiate crypto
	var cryptoProfiles []srtp.Profile
	if encryption != sdpv1.EncryptionNone && len(offerSection.Security.Profiles) > 0 {
		// Generate our crypto profiles
		ourProfiles, err := srtp.DefaultProfiles()
		if err != nil {
			return nil, err
		}

		// Find common profile
		for _, ourProf := range ourProfiles {
			for _, offProf := range offerSection.Security.Profiles {
				if ourProf.Profile == offProf.Profile {
					cryptoProfiles = []srtp.Profile{ourProf}
					goto cryptoFound
				}
			}
		}
	cryptoFound:

		// Add crypto attributes
		if len(cryptoProfiles) > 0 {
			attrs = appendCryptoAttributes(attrs, cryptoProfiles)
		}
	}

	if len(cryptoProfiles) == 0 && encryption == sdpv1.EncryptionRequire {
		return nil, sdpv1.ErrNoCommonCrypto
	}

	// Determine protocol
	proto := "AVP"
	if len(cryptoProfiles) > 0 {
		proto = "SAVP"
	}

	// Add ptime and direction
	attrs = append(attrs, sdp.Attribute{Key: "ptime", Value: "20"})
	attrs = append(attrs, sdp.Attribute{Key: string(offerSection.Direction)})

	answerSection := &MediaSection{
		Description: &sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:   string(offerSection.Kind),
				Port:    sdp.RangedPort{Value: localPort},
				Protos:  []string{"RTP", proto},
				Formats: formats,
			},
			Attributes: attrs,
		},
		Kind:      offerSection.Kind,
		Port:      uint16(localPort),
		Direction: offerSection.Direction,
		Codecs:    []*Codec{selectedCodec},
		Codec:     selectedCodec,
		Security: Security{
			Mode:     encryption,
			Profiles: cryptoProfiles,
		},
	}

	if dtmfCodec != nil {
		answerSection.Codecs = append(answerSection.Codecs, dtmfCodec)
	}

	return answerSection, nil
}

// appendCryptoAttributes appends crypto attributes to the attribute list.
func appendCryptoAttributes(attrs []sdp.Attribute, profiles []srtp.Profile) []sdp.Attribute {
	for _, p := range profiles {
		keyMaterial := append([]byte{}, p.Key...)
		keyMaterial = append(keyMaterial, p.Salt...)
		skey := base64EncodeKey(keyMaterial)

		attrs = append(attrs, sdp.Attribute{
			Key:   "crypto",
			Value: fmt.Sprintf("%d %s inline:%s", p.Index, p.Profile, skey),
		})
	}
	return attrs
}

// base64EncodeKey encodes key material in base64.
func base64EncodeKey(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
