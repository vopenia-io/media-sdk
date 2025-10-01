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

package rtp

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/livekit/media-sdk"
)

var (
	mediaID         atomic.Uint32
	mediaDumpToFile = os.Getenv("LK_DUMP_MEDIA") == "true"
)

var (
	codecByType [0xff]media.Codec
)

func init() {
	media.OnRegister(func(c media.Codec) {
		info := c.Info()
		if info.RTPIsStatic {
			codecByType[info.RTPDefType] = c
		}
	})
}

func CodecByPayloadType(typ byte) media.Codec {
	return codecByType[typ]
}

type TrackCodec[T media.Frame] interface {
	media.Codec
	EncodeRTP(w *Stream) media.WriteCloser[T]
	DecodeRTP(w media.Writer[T], typ byte) Handler
}

// AUDIO CODECS

type AudioCodec interface {
	media.Codec
	TrackCodec[media.PCM16Sample]
}

type AudioEncoder[S BytesFrame] interface {
	AudioCodec
	Decode(writer media.PCM16Writer) media.WriteCloser[S]
	Encode(writer media.WriteCloser[S]) media.PCM16Writer
}

func NewAudioCodec[S BytesFrame](
	info media.CodecInfo,
	decode func(writer media.PCM16Writer) media.WriteCloser[S],
	encode func(writer media.WriteCloser[S]) media.PCM16Writer,
) AudioCodec {
	if info.SampleRate <= 0 {
		panic("invalid sample rate")
	}
	if info.RTPClockRate == 0 {
		info.RTPClockRate = info.SampleRate
	}
	return &audioCodec[S]{
		info:   info,
		encode: encode,
		decode: decode,
	}
}

type audioCodec[S BytesFrame] struct {
	info   media.CodecInfo
	decode func(writer media.PCM16Writer) media.WriteCloser[S]
	encode func(writer media.WriteCloser[S]) media.PCM16Writer
}

func (c *audioCodec[S]) Info() media.CodecInfo {
	return c.info
}

func (c *audioCodec[S]) Decode(w media.PCM16Writer) media.WriteCloser[S] {
	return c.decode(w)
}

func (c *audioCodec[S]) Encode(w media.WriteCloser[S]) media.PCM16Writer {
	return c.encode(w)
}

func (c *audioCodec[S]) EncodeRTP(w *Stream) media.PCM16Writer {
	var s media.WriteCloser[S] = NewMediaStreamOut[S](w, c.info.SampleRate)
	if mediaDumpToFile {
		id := mediaID.Add(1)
		name := fmt.Sprintf("sip_rtp_out_%d", id)
		ext := c.info.FileExt
		if ext == "" {
			ext = "raw"
		}
		s = media.DumpWriter[S](ext, name, media.NopCloser(s))
	}
	return c.encode(s)
}

func (c *audioCodec[S]) DecodeRTP(w media.Writer[media.PCM16Sample], typ byte) Handler {
	s := c.decode(media.NopCloser(w))
	if mediaDumpToFile {
		id := mediaID.Add(1)
		name := fmt.Sprintf("sip_rtp_in_%d", id)
		ext := c.info.FileExt
		if ext == "" {
			ext = "raw"
		}
		s = media.DumpWriter(ext, name, media.NopCloser(s))
	}
	return NewMediaStreamIn(s)
}

// VIDEO CODECS

type VideoCodec interface {
	media.Codec
	TrackCodec[media.FrameSample]
}

type VideoEncoder[S BytesFrame] interface {
	VideoCodec
	Decode(writer media.FrameWriter) media.WriteCloser[S]
	Encode(writer media.WriteCloser[S]) media.FrameWriter
}

type videoCodec[S BytesFrame] struct {
	info   media.CodecInfo
	encode func(writer media.WriteCloser[S]) media.FrameWriter
	decode func(writer media.FrameWriter) media.WriteCloser[S] // currently not used
}

func NewVideoCodec[S BytesFrame](info media.CodecInfo, encode func(writer media.WriteCloser[S]) media.FrameWriter, decode func(writer media.FrameWriter) media.WriteCloser[S]) VideoCodec {
	return &videoCodec[S]{
		info:   info,
		encode: encode,
		decode: decode, // currently not used
	}
}

func (c *videoCodec[S]) Info() media.CodecInfo {
	return c.info
}

func (c *videoCodec[S]) EncodeRTP(w *Stream) media.FrameWriter {
	var s media.WriteCloser[S] = NewMediaStreamOut[S](w, c.info.SampleRate)
	if mediaDumpToFile {
		id := mediaID.Add(1)
		name := fmt.Sprintf("sip_rtp_out_%d", id)
		ext := c.info.FileExt
		if ext == "" {
			ext = "raw"
		}
		s = media.DumpWriter(ext, name, media.NopCloser(s))
	}
	return c.encode(s)
}

func (c *videoCodec[S]) DecodeRTP(w media.Writer[media.FrameSample], typ byte) Handler {
	s := media.NopCloser(w)
	if mediaDumpToFile {
		id := mediaID.Add(1)
		name := fmt.Sprintf("sip_rtp_in_%d", id)
		ext := c.info.FileExt
		if ext == "" {
			ext = "raw"
		}
		s = media.DumpWriter(ext, name, media.NopCloser(s))
	}
	return NewMediaStreamIn(s)
}
