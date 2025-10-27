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

import "github.com/pion/rtcp"

// WriteStreamCloser combines WriteStream with Close
type WriteStreamCloser interface {
	WriteStream
	Close() error
}

// WriteStreamSwitcher allows swapping the underlying WriteStream atomically
type WriteStreamSwitcher struct {
	impl WriteStreamCloser
}

// NewWriteStreamSwitcher creates a new WriteStreamSwitcher
func NewWriteStreamSwitcher() *WriteStreamSwitcher {
	return &WriteStreamSwitcher{
		impl: &nopWriteStream{},
	}
}

// Swap replaces the current WriteStream with a new one
func (s *WriteStreamSwitcher) Swap(w WriteStreamCloser) WriteStreamCloser {
	old := s.impl
	s.impl = w
	return old
}

// WriteRTCP writes to the current WriteStream
func (s *WriteStreamSwitcher) WriteRTCP(pkt rtcp.Packet) (int, error) {
	return s.impl.WriteRTCP(pkt)
}

// String returns the string representation
func (s *WriteStreamSwitcher) String() string {
	return s.impl.String()
}

// Close closes the current WriteStream
func (s *WriteStreamSwitcher) Close() error {
	return s.impl.Close()
}

// nopWriteStream is a no-op WriteStream
type nopWriteStream struct{}

func (n *nopWriteStream) WriteRTCP(pkt rtcp.Packet) (int, error) {
	buf, err := pkt.Marshal()
	if err != nil {
		return 0, err
	}
	return len(buf), nil
}

func (n *nopWriteStream) String() string {
	return "NopRTCPWriteStream"
}

func (n *nopWriteStream) Close() error {
	return nil
}

// streamNopCloser wraps a WriteStream to make it a WriteStreamCloser
type streamNopCloser struct {
	WriteStream
}

func (s streamNopCloser) Close() error {
	return nil
}

// NewStreamNopCloser wraps a WriteStream with a no-op Close
func NewStreamNopCloser(w WriteStream) WriteStreamCloser {
	return streamNopCloser{WriteStream: w}
}
