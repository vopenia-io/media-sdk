// Copyright 2024 LiveKit, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sdp

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pion/sdp/v3"

	"github.com/livekit/media-sdk"
	"github.com/livekit/media-sdk/dtmf"
	"github.com/livekit/media-sdk/rtp"
	"github.com/livekit/media-sdk/srtp"
)

var (
	ErrNoCommonMedia  = errors.New("common audio codec not found")
	ErrNoCommonCrypto = errors.New("no common encryption profiles")
	ErrNoCommonVideo  = errors.New("common video codec not found")
)

type Encryption int

const (
	EncryptionNone Encryption = iota
	EncryptionAllow
	EncryptionRequire
)

type CodecInfo struct {
	Type  byte
	Codec media.Codec
}

func OfferCodecs() []CodecInfo {
	const dynamicType = 101
	codecs := media.EnabledCodecs()
	slices.SortFunc(codecs, func(a, b media.Codec) int {
		ai, bi := a.Info(), b.Info()
		if ai.RTPIsStatic != bi.RTPIsStatic {
			if ai.RTPIsStatic {
				return -1
			} else if bi.RTPIsStatic {
				return 1
			}
		}
		return bi.Priority - ai.Priority
	})
	infos := make([]CodecInfo, 0, len(codecs))
	nextType := byte(dynamicType)
	for _, c := range codecs {
		cinfo := c.Info()
		info := CodecInfo{
			Codec: c,
		}
		if cinfo.RTPIsStatic {
			info.Type = cinfo.RTPDefType
		} else {
			typ := nextType
			nextType++
			info.Type = typ
		}
		infos = append(infos, info)
	}
	return infos
}

type RTCP struct {
	Port int
	FbC  map[int]map[string]string
}

type MediaDesc struct {
	Codecs         []CodecInfo
	DTMFType       byte // set to 0 if there's no DTMF
	CryptoProfiles []srtp.Profile
	RTCP           *RTCP
}

type VideoMediaDesc struct {
	Codecs         []CodecInfo
	CryptoProfiles []srtp.Profile
}

func appendCryptoProfiles(attrs []sdp.Attribute, profiles []srtp.Profile) []sdp.Attribute {
	var buf []byte
	for _, p := range profiles {
		buf = buf[:0]
		buf = append(buf, p.Key...)
		buf = append(buf, p.Salt...)
		skey := base64.StdEncoding.WithPadding(base64.StdPadding).EncodeToString(buf)
		attrs = append(attrs, sdp.Attribute{
			Key:   "crypto",
			Value: fmt.Sprintf("%d %s inline:%s", p.Index, p.Profile, skey),
		})
	}
	return attrs
}

func OfferAudioMedia(rtpListenerPort int, encrypted Encryption) (MediaDesc, *sdp.MediaDescription, error) {
	// Static compiler check for frame duration hardcoded below.
	var _ = [1]struct{}{}[20*time.Millisecond-rtp.DefFrameDur]

	codecs := OfferCodecs()
	attrs := make([]sdp.Attribute, 0, len(codecs)+4)
	formats := make([]string, 0, len(codecs))
	dtmfType := byte(0)
	for _, codec := range codecs {
		if codec.Codec.Info().SDPName == dtmf.SDPName {
			dtmfType = codec.Type
		}
		styp := strconv.Itoa(int(codec.Type))
		formats = append(formats, styp)
		attrs = append(attrs, sdp.Attribute{
			Key:   "rtpmap",
			Value: styp + " " + codec.Codec.Info().SDPName,
		})
	}
	if dtmfType > 0 {
		attrs = append(attrs, sdp.Attribute{
			Key: "fmtp", Value: fmt.Sprintf("%d 0-16", dtmfType),
		})
	}
	var cryptoProfiles []srtp.Profile
	if encrypted != EncryptionNone {
		var err error
		cryptoProfiles, err = srtp.DefaultProfiles()
		if err != nil {
			return MediaDesc{}, nil, err
		}
		attrs = appendCryptoProfiles(attrs, cryptoProfiles)
	}

	attrs = append(attrs, []sdp.Attribute{
		{Key: "ptime", Value: "20"},
		{Key: "sendrecv"},
	}...)

	proto := "AVP"
	if encrypted != EncryptionNone {
		proto = "SAVP"
	}

	return MediaDesc{
			Codecs:         codecs,
			DTMFType:       dtmfType,
			CryptoProfiles: cryptoProfiles,
		}, &sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:   "audio",
				Port:    sdp.RangedPort{Value: rtpListenerPort},
				Protos:  []string{"RTP", proto},
				Formats: formats,
			},
			Attributes: attrs,
		}, nil
}

func OfferVideoMedia(rtpListenerPort int, encrypted Encryption) (MediaDesc, *sdp.MediaDescription, error) {
	// Static compiler check for frame duration hardcoded below.
	var _ = [1]struct{}{}[20*time.Millisecond-rtp.DefFrameDur]

	codecs := OfferCodecs()
	attrs := make([]sdp.Attribute, 0, len(codecs)+4)
	formats := make([]string, 0, len(codecs))
	for _, codec := range codecs {
		styp := strconv.Itoa(int(codec.Type))
		formats = append(formats, styp)
		attrs = append(attrs, sdp.Attribute{
			Key:   "rtpmap",
			Value: styp + " " + codec.Codec.Info().SDPName,
		})
	}
	var cryptoProfiles []srtp.Profile
	if encrypted != EncryptionNone {
		var err error
		cryptoProfiles, err = srtp.DefaultProfiles()
		if err != nil {
			return MediaDesc{}, nil, err
		}
		attrs = appendCryptoProfiles(attrs, cryptoProfiles)
	}

	attrs = append(attrs, []sdp.Attribute{
		{Key: "ptime", Value: "20"},
		{Key: "sendrecv"},
	}...)

	proto := "AVP"
	if encrypted != EncryptionNone {
		proto = "SAVP"
	}

	return MediaDesc{
			Codecs:         codecs,
			DTMFType:       0,
			CryptoProfiles: cryptoProfiles,
		}, &sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:   "video",
				Port:    sdp.RangedPort{Value: rtpListenerPort},
				Protos:  []string{"RTP", proto},
				Formats: formats,
			},
			Attributes: attrs,
		}, nil
}

// func OfferVideoMedia(rtpListenerPort int, encrypted Encryption) (VideoMediaDesc, *sdp.MediaDescription, error) {
// 	codecs := OfferCodecs()
// 	attrs := make([]sdp.Attribute, 0, len(codecs)+2)
// 	formats := make([]string, 0, len(codecs))

// 	// Filter for video codecs only
// 	videoCodecs := make([]CodecInfo, 0)
// 	for _, codec := range codecs {
// 		// Check if this is a video codec (H.264, VP8, VP9, etc.)
// 		name := codec.Codec.Info().SDPName
// 		if strings.HasPrefix(name, "H264") || strings.HasPrefix(name, "VP8") || strings.HasPrefix(name, "VP9") {
// 			videoCodecs = append(videoCodecs, codec)
// 			styp := strconv.Itoa(int(codec.Type))
// 			formats = append(formats, styp)
// 			attrs = append(attrs, sdp.Attribute{
// 				Key:   "rtpmap",
// 				Value: styp + " " + codec.Codec.Info().SDPName,
// 			})
// 		}
// 	}

// 	if len(videoCodecs) == 0 {
// 		return VideoMediaDesc{}, nil, ErrNoCommonVideo
// 	}

// 	var cryptoProfiles []srtp.Profile
// 	if encrypted != EncryptionNone {
// 		var err error
// 		cryptoProfiles, err = srtp.DefaultProfiles()
// 		if err != nil {
// 			return VideoMediaDesc{}, nil, err
// 		}
// 		attrs = appendCryptoProfiles(attrs, cryptoProfiles)
// 	}

// 	attrs = append(attrs, []sdp.Attribute{
// 		{Key: "sendrecv"},
// 	}...)

// 	proto := "AVP"
// 	if encrypted != EncryptionNone {
// 		proto = "SAVP"
// 	}

// 	return VideoMediaDesc{
// 			Codecs:         videoCodecs,
// 			CryptoProfiles: cryptoProfiles,
// 		}, &sdp.MediaDescription{
// 			MediaName: sdp.MediaName{
// 				Media:   "video",
// 				Port:    sdp.RangedPort{Value: rtpListenerPort},
// 				Protos:  []string{"RTP", proto},
// 				Formats: formats,
// 			},
// 			Attributes: attrs,
// 		}, nil
// }

// func AnswerMedia(rtpListenerPort int, audio *TrackConfig, tracks []TrackConfig, crypt *srtp.Profile) []*sdp.MediaDescription {
// 	descs := make([]*sdp.MediaDescription, 0, 1+len(tracks))
// 	descs = append(descs, AnswerAudioMedia(rtpListenerPort, audio, crypt))
// 	// TODO: either use different ports for audio and video, or use a single media description with multiple formats (BUNDLE)
// 	for _, track := range tracks {
// 		var t *sdp.MediaDescription
// 		switch track.Kind {
// 		case TrackKindAudio:
// 			t = AnswerAudioMedia(rtpListenerPort, &track, nil)
// 		case TrackKindVideo:
// 			t = AnswerVideoMedia(rtpListenerPort, track, nil)
// 		}
// 		descs = append(descs, t)
// 	}
// 	return descs
// }

// func AnswerMedia(rtpListenerPort int, audio *TrackConfig, video *TrackConfig, crypt *srtp.Profile) (*sdp.MediaDescription, *sdp.MediaDescription) {
// 	// descs := make([]*sdp.MediaDescription, 0, 1+len(tracks))
// 	a := AnswerAudioMedia(rtpListenerPort, audio, crypt)
// 	v := AnswerVideoMedia(rtpListenerPort, video, crypt)
// 	return a, v
// }

func AnswerAudioMedia(rtpListenerPort int, audio *TrackConfig, crypt *srtp.Profile) *sdp.MediaDescription {
	// Static compiler check for frame duration hardcoded below.
	var _ = [1]struct{}{}[20*time.Millisecond-rtp.DefFrameDur]

	attrs := make([]sdp.Attribute, 0, 6)
	attrs = append(attrs, sdp.Attribute{
		Key: "rtpmap", Value: fmt.Sprintf("%d %s", audio.Type, audio.Codec.Info().SDPName),
	})
	formats := make([]string, 0, 2)
	formats = append(formats, strconv.Itoa(int(audio.Type)))
	if audio.DTMFType != 0 {
		formats = append(formats, strconv.Itoa(int(audio.DTMFType)))
		attrs = append(attrs, []sdp.Attribute{
			{Key: "rtpmap", Value: fmt.Sprintf("%d %s", audio.DTMFType, dtmf.SDPName)},
			{Key: "fmtp", Value: fmt.Sprintf("%d 0-16", audio.DTMFType)},
		}...)
	}
	proto := "AVP"
	if crypt != nil {
		proto = "SAVP"
		attrs = appendCryptoProfiles(attrs, []srtp.Profile{*crypt})
	}
	attrs = append(attrs, []sdp.Attribute{
		{Key: "ptime", Value: "20"},
		{Key: "sendrecv"},
	}...)
	return &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media:   "audio",
			Port:    sdp.RangedPort{Value: rtpListenerPort},
			Protos:  []string{"RTP", proto},
			Formats: formats,
		},
		Attributes: attrs,
	}
}

func AnswerVideoMedia(rtpListenerPort int, track *TrackConfig, crypt *srtp.Profile, rtcp *RTCP) *sdp.MediaDescription {
	attrs := make([]sdp.Attribute, 0, 2)
	attrs = append(attrs, []sdp.Attribute{
		{Key: "rtpmap", Value: fmt.Sprintf("%d %s", track.Type, track.Codec.Info().SDPName)},
		{Key: "fmtp", Value: fmt.Sprintf("%d profile-level-id=%s", track.Codec.Info().RTPDefType, "42801F")},
	}...)
	if rtcp != nil {
		attrs = append(attrs, sdp.Attribute{
			Key:   "rtcp",
			Value: fmt.Sprintf("%d", rtcp.Port),
		})
		for pt, fbc := range rtcp.FbC {
			var k string
			if pt == 0 {
				k = "*"
			} else {
				k = strconv.Itoa(pt)
			}

			values := make([]string, 0, len(fbc))
			for _, v := range fbc {
				values = append(values, v)
			}

			attrs = append(attrs, sdp.Attribute{
				Key:   fmt.Sprintf("rtcp-fb:%s", k),
				Value: strings.Join(values, " "),
			})
		}
	}
	formats := []string{strconv.Itoa(int(track.Type))}
	proto := "AVP"
	if crypt != nil {
		proto = "SAVP"
		attrs = appendCryptoProfiles(attrs, []srtp.Profile{*crypt})
	}
	attrs = append(attrs, []sdp.Attribute{
		{Key: "sendrecv"},
	}...)
	return &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media:   "video",
			Port:    sdp.RangedPort{Value: rtpListenerPort},
			Protos:  []string{"RTP", proto},
			Formats: formats,
		},
		Attributes: attrs,
	}
}

type MediaDescAddr struct {
	MediaDesc
	Addr netip.AddrPort
}

type Description struct {
	SDP   sdp.SessionDescription
	Audio MediaDescAddr
	Video *MediaDescAddr
}

type Offer Description

type Answer Description

// func NewOffer(publicIp netip.Addr, rtpListenerAudioPort int, rtpListenerVideoPort *int, encrypted Encryption) (*Offer, error) {
// 	sessId := rand.Uint64() // TODO: do we need to track these?

// 	m, mediaDesc, err := OfferAudioMedia(rtpListenerAudioPort, encrypted)
// 	if err != nil {
// 		return nil, err
// 	}
// 	offer := sdp.SessionDescription{
// 		Version: 0,
// 		Origin: sdp.Origin{
// 			Username:       "-",
// 			SessionID:      sessId,
// 			SessionVersion: sessId,
// 			NetworkType:    "IN",
// 			AddressType:    "IP4",
// 			UnicastAddress: publicIp.String(),
// 		},
// 		SessionName: "LiveKit",
// 		ConnectionInformation: &sdp.ConnectionInformation{
// 			NetworkType: "IN",
// 			AddressType: "IP4",
// 			Address:     &sdp.Address{Address: publicIp.String()},
// 		},
// 		TimeDescriptions: []sdp.TimeDescription{
// 			{
// 				Timing: sdp.Timing{
// 					StartTime: 0,
// 					StopTime:  0,
// 				},
// 			},
// 		},
// 		MediaDescriptions: []*sdp.MediaDescription{mediaDesc},
// 	}
// 	return &Offer{
// 		SDP: offer,
// 		Audio: MediaDescAddr{
// 			MediaDesc: m,
// 			Addr:      netip.AddrPortFrom(publicIp, uint16(rtpListenerPort)),
// 		},
// 	}, nil
// }

func NewOffer(publicIp netip.Addr, rtpListenerAudioPort int, rtpListenerVideoPort *int, encrypted Encryption) (*Offer, error) {
	sessId := rand.Uint64() // TODO: do we need to track these?

	offer := &Offer{
		SDP: sdp.SessionDescription{
			Version: 0,
			Origin: sdp.Origin{
				Username:       "-",
				SessionID:      sessId,
				SessionVersion: sessId,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: publicIp.String(),
			},
			SessionName: "LiveKit",
			ConnectionInformation: &sdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     &sdp.Address{Address: publicIp.String()},
			},
			TimeDescriptions: []sdp.TimeDescription{
				{
					Timing: sdp.Timing{
						StartTime: 0,
						StopTime:  0,
					},
				},
			},
			MediaDescriptions: []*sdp.MediaDescription{},
		},
	}

	audio, audioMediaDesc, err := OfferAudioMedia(rtpListenerAudioPort, encrypted)
	if err != nil {
		return nil, err
	}
	offer.SDP.MediaDescriptions = append(offer.SDP.MediaDescriptions, audioMediaDesc)
	offer.Audio = MediaDescAddr{
		MediaDesc: audio,
		Addr:      netip.AddrPortFrom(publicIp, uint16(rtpListenerAudioPort)),
	}

	if rtpListenerVideoPort != nil {
		video, videoMediaDesc, err := OfferVideoMedia(*rtpListenerVideoPort, encrypted)
		if err != nil {
			return nil, err
		}
		offer.SDP.MediaDescriptions = append(offer.SDP.MediaDescriptions, videoMediaDesc)
		offer.Video = &MediaDescAddr{
			MediaDesc: video,
			Addr:      netip.AddrPortFrom(publicIp, uint16(*rtpListenerVideoPort)),
		}
	}

	return offer, nil
}

// func NewOfferWithVideo(publicIp netip.Addr, audioPort, videoPort int, encrypted Encryption) (*Offer, *VideoMediaDesc, error) {
// 	sessId := rand.Uint64()

// 	// Generate audio media
// 	audioDesc, audioMedia, err := OfferMedia(audioPort, encrypted)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	// Generate video media
// 	videoDesc, videoMedia, err := OfferVideoMedia(videoPort, encrypted)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	offer := sdp.SessionDescription{
// 		Version: 0,
// 		Origin: sdp.Origin{
// 			Username:       "-",
// 			SessionID:      sessId,
// 			SessionVersion: sessId,
// 			NetworkType:    "IN",
// 			AddressType:    "IP4",
// 			UnicastAddress: publicIp.String(),
// 		},
// 		SessionName: "LiveKit",
// 		ConnectionInformation: &sdp.ConnectionInformation{
// 			NetworkType: "IN",
// 			AddressType: "IP4",
// 			Address:     &sdp.Address{Address: publicIp.String()},
// 		},
// 		TimeDescriptions: []sdp.TimeDescription{
// 			{
// 				Timing: sdp.Timing{
// 					StartTime: 0,
// 					StopTime:  0,
// 				},
// 			},
// 		},
// 		MediaDescriptions: []*sdp.MediaDescription{audioMedia, videoMedia},
// 	}
// 	return &Offer{
// 		SDP:       offer,
// 		Addr:      netip.AddrPortFrom(publicIp, uint16(audioPort)),
// 		MediaDesc: audioDesc,
// 	}, &videoDesc, nil
// }

func (d *Offer) configToSdpDesc(config *TrackConfig, desc MediaDesc, rtpListenerPort int, enc Encryption, isVideo bool) (*sdp.MediaDescription, *srtp.Config, error) {
	var (
		sconf *srtp.Config
		sprof *srtp.Profile
	)
	if len(desc.CryptoProfiles) != 0 && enc != EncryptionNone {
		answer, err := srtp.DefaultProfiles()
		if err != nil {
			return nil, nil, err
		}
		sconf, sprof, err = SelectCrypto(desc.CryptoProfiles, answer, true)
		if err != nil {
			return nil, nil, err
		}
	}
	if sprof == nil && enc == EncryptionRequire {
		return nil, nil, ErrNoCommonCrypto
	}

	if isVideo {
		return AnswerVideoMedia(rtpListenerPort, config, sprof, desc.RTCP), sconf, nil
	} else {
		return AnswerAudioMedia(rtpListenerPort, config, sprof), sconf, nil
	}
}

func (d *Offer) Answer(publicIp netip.Addr, rtpListenerAudioPort int, rtpListenerVideoPort *int, enc Encryption) (*Answer, *MediaConfig, error) {
	slog.Info("answering offer", "audioPort", rtpListenerAudioPort, "videoPort", rtpListenerVideoPort)

	answer := &Answer{
		SDP: sdp.SessionDescription{
			Version: 0,
			Origin: sdp.Origin{
				Username:       "-",
				SessionID:      d.SDP.Origin.SessionID,
				SessionVersion: d.SDP.Origin.SessionID + 2,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: publicIp.String(),
			},
			SessionName: "LiveKit",
			ConnectionInformation: &sdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     &sdp.Address{Address: publicIp.String()},
			},
			TimeDescriptions: []sdp.TimeDescription{
				{
					Timing: sdp.Timing{
						StartTime: 0,
						StopTime:  0,
					},
				},
			},
			MediaDescriptions: nil,
		},
	}

	config := &MediaConfig{}

	audio, err := SelectAudio(d.Audio.MediaDesc, false)
	if err != nil {
		return nil, nil, err
	}

	audioDesc, audioSconf, err := d.configToSdpDesc(audio, d.Audio.MediaDesc, rtpListenerAudioPort, enc, false)
	if err != nil {
		return nil, nil, err
	}
	answer.SDP.MediaDescriptions = append(answer.SDP.MediaDescriptions, audioDesc)
	audioSrc := netip.AddrPortFrom(publicIp, uint16(rtpListenerAudioPort))
	answer.Audio = MediaDescAddr{
		MediaDesc: d.Audio.MediaDesc,
		Addr:      audioSrc,
	}
	config.Audio = MediaTrackConfig{
		TrackConfig: *audio,
		Local:       audioSrc,
		Remote:      d.Audio.Addr,
		Crypto:      audioSconf,
	}

	if rtpListenerVideoPort != nil && d.Video != nil {
		slog.Info("including video in answer", "port", *rtpListenerVideoPort)
		video, err := SelectVideo(d.Video.MediaDesc, false)
		if err != nil {
			return nil, nil, err
		}
		videoDesc, videoSconf, err := d.configToSdpDesc(video, d.Video.MediaDesc, *rtpListenerVideoPort, enc, true)
		if err != nil {
			return nil, nil, err
		}
		answer.SDP.MediaDescriptions = append(answer.SDP.MediaDescriptions, videoDesc)
		videoSrc := netip.AddrPortFrom(publicIp, uint16(*rtpListenerVideoPort))
		answer.Video = &MediaDescAddr{
			MediaDesc: d.Video.MediaDesc,
			Addr:      videoSrc,
		}
		config.Video = &MediaTrackConfig{
			TrackConfig: *video,
			Local:       videoSrc,
			Remote:      d.Video.Addr,
			Crypto:      videoSconf,
		}
	}

	return answer, config, nil
}

func (d *Answer) Apply(offer *Offer, enc Encryption) (*MediaConfig, error) {
	audio, err := SelectAudio(d.Audio.MediaDesc, true)
	if err != nil {
		return nil, err
	}
	var audioSconf *srtp.Config
	if len(d.Audio.CryptoProfiles) != 0 && enc != EncryptionNone {
		audioSconf, _, err = SelectCrypto(offer.Audio.CryptoProfiles, d.Audio.CryptoProfiles, false)
		if err != nil {
			return nil, err
		}
	}
	if audioSconf == nil && enc == EncryptionRequire {
		return nil, ErrNoCommonCrypto
	}
	audioConf := MediaTrackConfig{
		TrackConfig: *audio,
		Local:       offer.Audio.Addr,
		Remote:      d.Audio.Addr,
		Crypto:      audioSconf,
	}

	videoConf := (*MediaTrackConfig)(nil)
	if offer.Video != nil {
		video, err := SelectVideo(d.Video.MediaDesc, true)
		if err != nil {
			return nil, err
		}
		var videoSconf *srtp.Config
		if len(d.Video.CryptoProfiles) != 0 && enc != EncryptionNone {
			videoSconf, _, err = SelectCrypto(offer.Video.CryptoProfiles, d.Video.CryptoProfiles, false)
			if err != nil {
				return nil, err
			}
		}
		if audioSconf == nil && enc == EncryptionRequire {
			return nil, ErrNoCommonCrypto
		}

		videoConf = &MediaTrackConfig{
			TrackConfig: *video,
			Local:       offer.Video.Addr,
			Remote:      d.Video.Addr,
			Crypto:      videoSconf,
		}
	}

	return &MediaConfig{
		audioConf,
		videoConf,
	}, nil
}

func Parse(data []byte) (*Description, error) {
	desc := new(Description)
	if err := desc.SDP.Unmarshal(data); err != nil {
		return nil, err
	}
	audios, videos := GetMedias(&desc.SDP)
	if len(audios) == 0 {
		return nil, errors.New("no audio in sdp")
	}
	audio := audios[0]
	video := (*sdp.MediaDescription)(nil)
	if len(videos) > 0 {
		video = videos[0]
	}

	var err error
	desc.Audio.Addr, err = GetMediaDest(&desc.SDP, audio)
	if err != nil {
		return nil, err
	} else if !desc.Audio.Addr.IsValid() || desc.Audio.Addr.Port() == 0 {
		return nil, fmt.Errorf("invalid audio address %q", desc.Audio.Addr)
	}
	m, err := ParseMedia(audio, false)
	if err != nil {
		return nil, err
	}
	desc.Audio.MediaDesc = *m

	if video != nil {
		desc.Video = &MediaDescAddr{}
		desc.Video.Addr, err = GetMediaDest(&desc.SDP, video)
		if err != nil {
			return nil, err
		} else if !desc.Video.Addr.IsValid() || desc.Video.Addr.Port() == 0 {
			return nil, fmt.Errorf("invalid video address %q", desc.Video.Addr)
		}
		m, err := ParseMedia(video, true)
		if err != nil {
			return nil, err
		}
		desc.Video.MediaDesc = *m
	}
	slog.Info("TEST parsed offer", "offer", desc)
	return desc, nil
}

func ParseOffer(data []byte) (*Offer, error) {
	d, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return (*Offer)(d), nil
}

func ParseAnswer(data []byte) (*Answer, error) {
	d, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return (*Answer)(d), nil
}

func parseSRTPProfile(val string) (*srtp.Profile, error) {
	val = strings.TrimSpace(val)
	sub := strings.SplitN(val, " ", 3)
	if len(sub) != 3 {
		return nil, nil // ignore
	}
	sind, prof, skey := sub[0], srtp.ProtectionProfile(sub[1]), sub[2]
	ind, err := strconv.Atoi(sind)
	if err != nil {
		return nil, err
	}
	var ok bool
	skey, ok = strings.CutPrefix(skey, "inline:")
	if !ok {
		return nil, nil // ignore
	}
	keys, err := base64.RawStdEncoding.DecodeString(skey)
	if err != nil {
		// Fallback to padded encoding if raw fails
		if keys, err = base64.StdEncoding.DecodeString(skey); err != nil {
			return nil, fmt.Errorf("cannot parse crypto key %q: %v", skey, err)
		}
	}
	var salt []byte
	if sp, err := prof.Parse(); err == nil {
		keyLen, err := sp.KeyLen()
		if err != nil {
			return nil, err
		}
		keys, salt = keys[:keyLen], keys[keyLen:]
	}
	return &srtp.Profile{
		Index:   ind,
		Profile: prof,
		Key:     keys,
		Salt:    salt,
	}, nil
}

func ParseMedia(d *sdp.MediaDescription, isVideo bool) (*MediaDesc, error) {
	var out MediaDesc
	for _, m := range d.Attributes {
		switch m.Key {
		case "rtcp":
			port, err := strconv.Atoi(m.Value)
			if err != nil {
				slog.Warn("cannot parse rtcp port", "port", m.Value, "error", err)
				continue
			}
			if out.RTCP == nil {
				out.RTCP = &RTCP{}
			}
			out.RTCP.Port = port
		case "rtcp-fb":
			styp, rest, ok := strings.Cut(m.Value, " ")
			if !ok {
				continue
			}
			if styp == "*" {
				styp = "0"
			}
			typ, err := strconv.Atoi(styp)
			if err != nil {
				continue
			}
			if out.RTCP == nil {
				out.RTCP = &RTCP{}
			}
			if out.RTCP.FbC == nil {
				out.RTCP.FbC = make(map[int]map[string]string)
			}
			fbc, ok := out.RTCP.FbC[typ]
			if !ok {
				fbc = make(map[string]string)
				out.RTCP.FbC[typ] = fbc
			}
			n, p, ok := strings.Cut(rest, " ")
			if !ok {
				fbc[rest] = ""
			} else {
				fbc[n] = p
			}
		case "rtpmap":
			sub := strings.SplitN(m.Value, " ", 2)
			if len(sub) != 2 {
				continue
			}
			typ, err := strconv.Atoi(sub[0])
			if err != nil {
				continue
			}
			name := sub[1]
			if name == dtmf.SDPName || name == dtmf.SDPName+"/1" {
				out.DTMFType = byte(typ)
				continue
			}
			var codec media.Codec
			var ok bool
			if isVideo {
				codec, ok = CodecByName(name).(rtp.VideoCodec)
			} else {
				codec, ok = CodecByName(name).(rtp.AudioCodec)
			}
			if !ok {
				slog.Warn("unknown codec", "name", name)
				continue
			} else {
				slog.Info("found codec", "name", name, "type", typ, "isVideo", isVideo)
			}
			out.Codecs = append(out.Codecs, CodecInfo{
				Type:  byte(typ),
				Codec: codec,
			})
		case "crypto":
			p, err := parseSRTPProfile(m.Value)
			if err != nil {
				return nil, fmt.Errorf("cannot parse srtp profile %q: %v", m.Value, err)
			} else if p == nil {
				continue
			}
			out.CryptoProfiles = append(out.CryptoProfiles, *p)
		}
	}
	for _, f := range d.MediaName.Formats {
		typ, err := strconv.Atoi(f)
		if err != nil {
			continue
		}
		var codec media.Codec
		var ok bool
		if isVideo {
			codec, ok = rtp.CodecByPayloadType(byte(typ)).(rtp.VideoCodec)
		} else {
			codec, ok = rtp.CodecByPayloadType(byte(typ)).(rtp.AudioCodec)
		}
		if !ok {
			slog.Warn("unknown codec type", "type", typ)
			continue
		}
		out.Codecs = append(out.Codecs, CodecInfo{
			Type:  byte(typ),
			Codec: codec,
		})
	}

	if out.RTCP.Port == 0 && out.RTCP.FbC != nil {
		out.RTCP.Port = d.MediaName.Port.Value + 1
	}

	return &out, nil
}

type MediaTrackConfig struct {
	TrackConfig
	Local  netip.AddrPort
	Remote netip.AddrPort
	Crypto *srtp.Config
}

type MediaConfig struct {
	Audio MediaTrackConfig
	Video *MediaTrackConfig
}

type TrackConfig struct {
	Codec    media.Codec
	Type     byte
	DTMFType byte
}

func SelectVideo(desc MediaDesc, answer bool) (*TrackConfig, error) {
	var (
		priority   int
		videoCodec media.Codec
		videoType  byte
	)
	for _, c := range desc.Codecs {
		// Check if this is a video codec
		if c.Codec == nil {
			continue
		}
		if videoCodec == nil || c.Codec.Info().Priority > priority {
			videoType = c.Type
			videoCodec = c.Codec
			priority = c.Codec.Info().Priority
		}
		if answer {
			break
		}
	}
	if videoCodec == nil {
		return nil, ErrNoCommonVideo
	}
	return &TrackConfig{
		Codec: videoCodec.(rtp.VideoCodec),
		Type:  videoType,
	}, nil
}

func SelectAudio(desc MediaDesc, answer bool) (*TrackConfig, error) {
	var (
		priority   int
		audioCodec rtp.AudioCodec
		audioType  byte
	)
	for _, c := range desc.Codecs {
		codec, ok := c.Codec.(rtp.AudioCodec)
		if !ok {
			continue
		}
		if audioCodec == nil || codec.Info().Priority > priority {
			audioType = c.Type
			audioCodec = codec
			priority = codec.Info().Priority
		}
		if answer {
			break
		}
	}
	if audioCodec == nil {
		return nil, ErrNoCommonMedia
	}
	return &TrackConfig{
		Codec:    audioCodec,
		Type:     audioType,
		DTMFType: desc.DTMFType,
	}, nil
}

func SelectCrypto(offer, answer []srtp.Profile, swap bool) (*srtp.Config, *srtp.Profile, error) {
	if len(offer) == 0 {
		return nil, nil, nil
	}
	for _, ans := range answer {
		sp, err := ans.Profile.Parse()
		if err != nil {
			continue
		}
		i := slices.IndexFunc(offer, func(off srtp.Profile) bool {
			return off.Profile == ans.Profile
		})
		if i >= 0 {
			off := offer[i]
			c := &srtp.Config{
				Keys: srtp.SessionKeys{
					LocalMasterKey:   off.Key,
					LocalMasterSalt:  off.Salt,
					RemoteMasterKey:  ans.Key,
					RemoteMasterSalt: ans.Salt,
				},
				Profile: sp,
			}
			if swap {
				c.Keys.LocalMasterKey, c.Keys.RemoteMasterKey = c.Keys.RemoteMasterKey, c.Keys.LocalMasterKey
				c.Keys.LocalMasterSalt, c.Keys.RemoteMasterSalt = c.Keys.RemoteMasterSalt, c.Keys.LocalMasterSalt
			}
			prof := &off
			if swap {
				prof = &ans
				// Echo the cipher suite tag of the offer, in the answer
				prof.Index = off.Index
			}
			return c, prof, nil
		}
	}
	return nil, nil, nil
}
