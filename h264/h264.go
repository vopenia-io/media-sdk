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

package h264

import (
	"io"

	"github.com/livekit/media-sdk"
	"github.com/livekit/media-sdk/rtp"
)

// Sample represents an H.264 video frame.
type Sample []byte

// CopyTo implements the Frame interface.
func (s Sample) CopyTo(dst []byte) (int, error) {
	if len(dst) < len(s) {
		return 0, io.ErrShortBuffer
	}
	return copy(dst, s), nil
}

// Size returns the size of the frame in bytes.
func (s Sample) Size() int {
	return len(s)
}

// Encoder wraps a writer to provide H.264 encoding functionality.
type Encoder struct {
	w   media.Writer[Sample]
	buf Sample
}

// WriteSample writes an H.264 sample to the underlying writer.
func (e *Encoder) WriteSample(in Sample) error {
	return e.w.WriteSample(in)
}

// SampleRate returns the sample rate for H.264 (RTP clock rate).
func (e *Encoder) SampleRate() int {
	return 90000
}

// String returns a string representation of the encoder.
func (e *Encoder) String() string {
	return "H264Encoder"
}

// Encode creates a new H.264 encoder that wraps the provided writer.
func Encode(w media.Writer[Sample]) media.Writer[Sample] {
	return &Encoder{w: w}
}

// Decode creates a new H.264 decoder that wraps the provided writer.
func Decode(w media.Writer[Sample]) media.Writer[Sample] {
	return &Encoder{w: w}
}

// CodecInfo returns the H.264 codec information.
func CodecInfo() media.CodecInfo {
	return media.CodecInfo{
		SDPName:      "H264/90000",
		SampleRate:   90000, // RTP clock rate for H.264
		RTPClockRate: 90000,
		RTPDefType:   96, // Dynamic payload type for H.264
		RTPIsStatic:  false,
		Priority:     100, // High priority for video
		Disabled:     false,
		FileExt:      ".h264",
	}
}

// NewCodec creates a new H.264 codec.
func NewCodec() media.Codec {
	return &codec{}
}

type codec struct{}

func (c *codec) Info() media.CodecInfo {
	return CodecInfo()
}

// EncodeRTP creates an RTP encoder for H.264.
func (c *codec) EncodeRTP(w *rtp.Stream) media.Writer[Sample] {
	return rtp.NewMediaStreamOut[Sample](w, int(rtp.DefFrameDur))
}

// DecodeRTP creates an RTP decoder for H.264.
func (c *codec) DecodeRTP(w media.Writer[Sample], typ byte) rtp.Handler {
	return rtp.NewMediaStreamIn[Sample](w)
}

// Register the H.264 codec on package initialization.
func init() {
	media.RegisterCodec(NewCodec())
}
