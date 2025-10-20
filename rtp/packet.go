package rtp

import (
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/pion/rtp"
)

type PacketWriter interface {
	WriteRTP(p *rtp.Packet) error
}

type packetWriteStream struct {
	WriteStream
	pw PacketWriter
}

func NewPacketWriteStream(pw PacketWriter) WriteStream {
	return &packetWriteStream{pw: pw}
}

func (pws *packetWriteStream) WriteRTP(header *rtp.Header, payload []byte) (int, error) {
	packet := &rtp.Packet{
		Header:  *header,
		Payload: payload,
	}
	err := pws.pw.WriteRTP(packet)
	if err != nil {
		return 0, err
	}
	return len(payload), nil
}

func (pws *packetWriteStream) String() string {
	return "PacketWriteStream"
}

type WriteStreamCloser interface {
	WriteStream
	Close() error
}

func NewStreamNopCloser(pw WriteStream) WriteStreamCloser {
	return &PacketNopCloser{WriteStream: pw}
}

type PacketNopCloser struct {
	WriteStream
}

func (p *PacketNopCloser) Close() error {
	return nil
}

func NewWriteStreamSwitcher() *WriteStreamSwitcher {
	return &WriteStreamSwitcher{}
}

type WriteStreamSwitcher struct {
	w atomic.Pointer[WriteStreamCloser]
}

func (pws *WriteStreamSwitcher) Swap(w WriteStreamCloser) WriteStreamCloser {
	var old *WriteStreamCloser
	if w == nil {
		old = pws.w.Swap(nil)
	} else {
		old = pws.w.Swap(&w)
	}
	if old == nil {
		return nil
	}
	return *old
}

func (pws *WriteStreamSwitcher) WriteRTP(header *rtp.Header, payload []byte) (int, error) {
	w := pws.w.Load()
	if w == nil {
		return 0, nil
	}
	return (*w).WriteRTP(header, payload)
}

func (pws *WriteStreamSwitcher) Close() error {
	w := pws.Swap(nil)
	if w == nil {
		return nil
	}
	return w.Close()
}

func (pws *WriteStreamSwitcher) String() string {
	w := pws.w.Load()
	if w == nil {
		return "WriteStreamSwitcher(nil)"
	}
	return fmt.Sprintf("WriteStreamSwitcher(%s)", (*w).String())
}
