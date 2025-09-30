// Copyright 2023 LiveKit, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sdp

import (
	"net/netip"
	"testing"

	"github.com/livekit/media-sdk"
	"github.com/livekit/media-sdk/h264"
)

func TestOfferVideoMedia(t *testing.T) {
	// Register H.264 codec for testing
	codec := h264.NewCodec()
	media.RegisterCodec(codec)

	port := 5004
	encrypted := EncryptionNone

	desc, mediaDesc, err := OfferVideoMedia(port, encrypted)
	if err != nil {
		t.Fatalf("OfferVideoMedia failed: %v", err)
	}

	// desc is a value, not a pointer, so it can't be nil

	if mediaDesc == nil {
		t.Error("Expected non-nil MediaDescription")
	}

	if mediaDesc.MediaName.Media != "video" {
		t.Errorf("Expected media type 'video', got '%s'", mediaDesc.MediaName.Media)
	}

	if mediaDesc.MediaName.Port.Value != port {
		t.Errorf("Expected port %d, got %d", port, mediaDesc.MediaName.Port.Value)
	}

	// Check for H.264 codec in the description
	foundH264 := false
	for _, codecInfo := range desc.Codecs {
		if codecInfo.Codec.Info().SDPName == "H264/90000" {
			foundH264 = true
			break
		}
	}

	if !foundH264 {
		t.Error("Expected to find H.264 codec in video media description")
	}
}

func TestAnswerVideoMedia(t *testing.T) {
	// Register H.264 codec for testing
	codec := h264.NewCodec()
	media.RegisterCodec(codec)

	port := 5004
	videoConfig := &VideoConfig{
		Codec: codec,
		Type:  96, // Dynamic payload type for H.264
	}

	mediaDesc := AnswerVideoMedia(port, videoConfig, nil)
	if mediaDesc == nil {
		t.Error("Expected non-nil MediaDescription")
	}

	if mediaDesc.MediaName.Media != "video" {
		t.Errorf("Expected media type 'video', got '%s'", mediaDesc.MediaName.Media)
	}

	if mediaDesc.MediaName.Port.Value != port {
		t.Errorf("Expected port %d, got %d", port, mediaDesc.MediaName.Port.Value)
	}
}

func TestSelectVideo(t *testing.T) {
	// Register H.264 codec for testing
	codec := h264.NewCodec()
	media.RegisterCodec(codec)

	desc := VideoMediaDesc{
		Codecs: []CodecInfo{
			{
				Type:  96,
				Codec: codec,
			},
		},
	}

	// Test answer mode
	videoConfig, err := SelectVideo(desc, true)
	if err != nil {
		t.Fatalf("SelectVideo failed: %v", err)
	}

	if videoConfig == nil {
		t.Error("Expected non-nil VideoConfig")
	}

	if videoConfig.Type != 96 {
		t.Errorf("Expected type 96, got %d", videoConfig.Type)
	}

	if videoConfig.Codec.Info().SDPName != "H264/90000" {
		t.Errorf("Expected codec 'H264/90000', got '%s'", videoConfig.Codec.Info().SDPName)
	}
}

func TestSelectVideoNoCodec(t *testing.T) {
	desc := VideoMediaDesc{
		Codecs: []CodecInfo{},
	}

	_, err := SelectVideo(desc, true)
	if err != ErrNoCommonVideo {
		t.Errorf("Expected ErrNoCommonVideo, got %v", err)
	}
}

func TestNewOfferWithVideo(t *testing.T) {
	// Register H.264 codec for testing
	codec := h264.NewCodec()
	media.RegisterCodec(codec)

	publicIP := netip.MustParseAddr("192.168.1.100")
	audioPort := 5004
	videoPort := 5006
	encrypted := EncryptionNone

	offer, videoDesc, err := NewOfferWithVideo(publicIP, audioPort, videoPort, encrypted)
	if err != nil {
		t.Fatalf("NewOfferWithVideo failed: %v", err)
	}

	if offer == nil {
		t.Error("Expected non-nil Offer")
	}

	if videoDesc == nil {
		t.Error("Expected non-nil VideoMediaDesc")
	}

	if offer.Addr.Port() != uint16(audioPort) {
		t.Errorf("Expected audio port %d, got %d", audioPort, offer.Addr.Port())
	}

	// Check that we have both audio and video media descriptions
	if len(offer.SDP.MediaDescriptions) < 2 {
		t.Errorf("Expected at least 2 media descriptions, got %d", len(offer.SDP.MediaDescriptions))
	}

	// Check for video media description
	foundVideo := false
	for _, mediaDesc := range offer.SDP.MediaDescriptions {
		if mediaDesc.MediaName.Media == "video" {
			foundVideo = true
			break
		}
	}

	if !foundVideo {
		t.Error("Expected to find video media description in offer")
	}
}
