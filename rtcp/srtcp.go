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

package rtcp

import (
	"fmt"
	"io"
	"net"

	"github.com/pion/rtcp"
	"github.com/pion/srtp/v3"

	"github.com/livekit/protocol/logger"
)

// NewSessionSRTCP creates a new encrypted RTCP session using SRTCP
func NewSessionSRTCP(log logger.Logger, conn net.Conn, config *srtp.Config) (Session, error) {
	s, err := srtp.NewSessionSRTCP(conn, config)
	if err != nil {
		return nil, err
	}
	return &srtcpSession{log: log, s: s}, nil
}

type srtcpSession struct {
	log logger.Logger
	s   *srtp.SessionSRTCP
}

func (s *srtcpSession) OpenWriteStream() (WriteStream, error) {
	w, err := s.s.OpenWriteStream()
	if err != nil {
		return nil, err
	}
	return &srtcpWriteStream{w: w}, nil
}

func (s *srtcpSession) AcceptStream() (ReadStream, uint32, error) {
	r, ssrc, err := s.s.AcceptStream()
	if err != nil {
		return nil, 0, err
	}
	return &srtcpReadStream{r: r}, ssrc, nil
}

func (s *srtcpSession) Close() error {
	return s.s.Close()
}

type srtcpWriteStream struct {
	w *srtp.WriteStreamSRTCP
}

func (w *srtcpWriteStream) String() string {
	return "SRTCPWriteStream"
}

func (w *srtcpWriteStream) WriteRTCP(pkt rtcp.Packet) (int, error) {
	buf, err := pkt.Marshal()
	if err != nil {
		return 0, err
	}

	n, err := w.w.Write(buf)
	if err != nil {
		return 0, fmt.Errorf("SRTCP write failed: %w", err)
	}

	return n, nil
}

type srtcpReadStream struct {
	r *srtp.ReadStreamSRTCP
}

func (r *srtcpReadStream) ReadRTCP() ([]rtcp.Packet, error) {
	buf := make([]byte, MTUSize)
	n, err := r.r.Read(buf)
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("SRTCP read failed: %w", err)
	}

	pkts, err := rtcp.Unmarshal(buf[:n])
	if err != nil {
		return nil, fmt.Errorf("unmarshal RTCP packet failed: %w", err)
	}

	return pkts, nil
}
