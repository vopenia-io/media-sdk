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
)

const SDPName = "H264/90000"

// Register the H.264 codec on package initialization.
func init() {
	print("Registering H264 codec\n")
	media.RegisterCodec(media.NewCodec(
		media.CodecInfo{
			SDPName:      SDPName,
			SampleRate:   90000,
			RTPClockRate: 90000,
			RTPDefType:   97,
			RTPIsStatic:  false,
			Priority:     100,
			FileExt:      "h264",
		},
	))
	// media.RegisterCodec(
	// 	rtp.NewVideoCodec[Sample](media.CodecInfo{
	// 		SDPName:      SDPName,
	// 		SampleRate:   90000,
	// 		RTPClockRate: 90000,
	// 		RTPDefType:   97,
	// 		RTPIsStatic:  false,
	// 		Priority:     100,
	// 		FileExt:      "h264",
	// 	}, Encode, Decode))
}
