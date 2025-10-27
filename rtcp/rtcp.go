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
	"sync"

	"github.com/frostbyte73/core"
	"github.com/pion/rtcp"

	"github.com/livekit/protocol/logger"
)

const (
	MTUSize = 1500
)

// Session handles RTCP packet read/write operations
type Session interface {
	OpenWriteStream() (WriteStream, error)
	AcceptStream() (ReadStream, uint32, error)
	Close() error
}

// WriteStream writes RTCP packets
type WriteStream interface {
	String() string
	// WriteRTCP writes RTCP packet to the connection.
	WriteRTCP(pkt rtcp.Packet) (int, error)
}

// ReadStream reads RTCP packets
type ReadStream interface {
	// ReadRTCP reads RTCP packets from the connection.
	ReadRTCP() ([]rtcp.Packet, error)
}

// NewSession creates a new RTCP session
func NewSession(log logger.Logger, conn net.Conn) Session {
	return &session{
		log:    log,
		conn:   conn,
		w:      &writeStream{conn: conn},
		bySSRC: make(map[uint32]*readStream),
		rbuf:   make([]byte, MTUSize+1),
	}
}

type session struct {
	log    logger.Logger
	conn   net.Conn
	closed core.Fuse
	w      *writeStream

	rmu    sync.Mutex
	rbuf   []byte
	bySSRC map[uint32]*readStream
}

func (s *session) OpenWriteStream() (WriteStream, error) {
	return s.w, nil
}

func (s *session) AcceptStream() (ReadStream, uint32, error) {
	s.rmu.Lock()
	defer s.rmu.Unlock()

	for {
		n, err := s.conn.Read(s.rbuf[:])
		if err != nil {
			return nil, 0, err
		}
		if n > MTUSize {
			s.log.Errorw("RTCP packet is larger than MTU limit", nil)
			continue
		}

		buf := s.rbuf[:n]

		// Parse RTCP packets
		pkts, err := rtcp.Unmarshal(buf)
		if err != nil {
			s.log.Errorw("unmarshal RTCP packet failed", err)
			continue
		}

		if len(pkts) == 0 {
			continue
		}

		// Extract SSRC from first packet for stream identification
		var ssrc uint32
		switch pkt := pkts[0].(type) {
		case *rtcp.SenderReport:
			ssrc = pkt.SSRC
		case *rtcp.ReceiverReport:
			ssrc = pkt.SSRC
		case *rtcp.SourceDescription:
			if len(pkt.Chunks) > 0 {
				ssrc = pkt.Chunks[0].Source
			}
		default:
			// For other packet types, use 0 as a fallback
			ssrc = 0
		}

		isNew := false
		r := s.bySSRC[ssrc]
		if r == nil {
			r = &readStream{
				ssrc:   ssrc,
				closed: s.closed.Watch(),
				recv:   make(chan []rtcp.Packet, 10),
			}
			s.bySSRC[ssrc] = r
			isNew = true
		}

		r.write(pkts)
		if isNew {
			return r, r.ssrc, nil
		}
	}
}

func (s *session) Close() error {
	var err error
	s.closed.Once(func() {
		err = s.conn.Close()
		s.rmu.Lock()
		defer s.rmu.Unlock()
		s.bySSRC = nil
	})
	return err
}

type writeStream struct {
	mu   sync.Mutex
	conn net.Conn
}

func (w *writeStream) String() string {
	return fmt.Sprintf("RTCPWriteStream(%s)", w.conn.RemoteAddr())
}

func (w *writeStream) WriteRTCP(pkt rtcp.Packet) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	buf, err := pkt.Marshal()
	if err != nil {
		return 0, err
	}

	return w.conn.Write(buf)
}

type readStream struct {
	ssrc   uint32
	closed <-chan struct{}
	recv   chan []rtcp.Packet
}

func (r *readStream) write(pkts []rtcp.Packet) {
	select {
	case r.recv <- pkts:
	default:
		// Drop if channel is full
	}
}

func (r *readStream) ReadRTCP() ([]rtcp.Packet, error) {
	select {
	case pkts := <-r.recv:
		return pkts, nil
	case <-r.closed:
		return nil, io.EOF
	}
}
