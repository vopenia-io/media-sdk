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
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/livekit/media-sdk"
)

const SDPName = "H264/90000"

func parseProfileLevelID(s string) (profileIDC, profileIOP, levelIDC byte, err error) {
	if len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid length")
	}
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid hex string")
	}
	return b[0], b[1], b[2], nil
}

var scoremap = map[string]func(value string) (int, error){
	"profile-level-id": func(value string) (int, error) {
		profileIDC, _, levelIDC, err := parseProfileLevelID(value)
		if err != nil {
			return 0, fmt.Errorf("invalid profile-level-id: %w", err)
		}
		score := 0
		switch profileIDC {
		case 0x42: // Baseline
			score += 10
		case 0x4D: // Main
			score += 20
		case 0x64: // High
			score += 30
		default:
			return 0, fmt.Errorf("unknown profile IDC: %x", profileIDC)
		}

		if levelIDC < 0x1F { // Level 3.1
			return 0, fmt.Errorf("unsupported level IDC: %x", levelIDC)
		}
		score += int(levelIDC - 0x1F)
		return score, nil
	},
	"packetization-mode": func(value string) (int, error) {
		i, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("invalid packetization-mode: %w", err)
		}
		switch i {
		case 0:
			return 0, nil
		case 1:
			return 100, nil
		default:
			return 0, fmt.Errorf("unknown packetization-mode: %d", i)
		}
	},
}

func (c *h264Codec) ParseFMTP(fmtp string) error {
	attrsStr := strings.Split(fmtp, ";")
	for i := range attrsStr {
		attrsStr[i] = strings.TrimSpace(attrsStr[i])
		keyValue := strings.SplitN(attrsStr[i], ":", 2)
		if len(keyValue) != 2 {
			continue
		}
		key := keyValue[0]
		value := keyValue[1]
		if scoreFunc, ok := scoremap[key]; ok {
			score, err := scoreFunc(value)
			if err != nil {
				c.info.Priority = math.MinInt
				return fmt.Errorf("error parsing fmtp %s at %s=%s: %w", fmtp, key, value, err)
			}
			c.info.Priority += score
		}
	}
	return nil
}

func (c *h264Codec) Info() media.CodecInfo {
	return c.info
}

type h264Codec struct {
	info media.CodecInfo
}

// Register the H.264 codec on package initialization.
func init() {
	print("Registering H264 codec\n")

	h264 := &h264Codec{
		info: media.CodecInfo{
			SDPName:      SDPName,
			SampleRate:   90000,
			RTPClockRate: 90000,
			RTPDefType:   97,
			RTPIsStatic:  false,
			Priority:     100,
			FileExt:      "h264",
		},
	}

	media.RegisterCodec(h264)
}
