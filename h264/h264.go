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
	"github.com/livekit/media-sdk"
	"github.com/livekit/media-sdk/rtp"
)

const SDPName = "H264/90000"

// Encode creates a new H.264 encoder that wraps the provided writer.
func Encode(w media.WriteCloser[media.FrameSample]) media.FrameWriter {
	return w
}

// Decode creates a new H.264 decoder that wraps the provided writer.
func Decode(w media.FrameWriter) media.WriteCloser[media.FrameSample] {
	return w
}

// Register the H.264 codec on package initialization.
func init() {
	print("Registering H264 codec\n")
	media.RegisterCodec(rtp.NewVideoCodec(media.CodecInfo{
		SDPName:      SDPName,
		SampleRate:   90000,
		RTPClockRate: 90000,
		RTPDefType:   96,
		RTPIsStatic:  false,
		Priority:     100,
		FileExt:      "h264",
	}, Decode, Encode))
}
