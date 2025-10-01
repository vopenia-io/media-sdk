package media

import (
	"io"
)

type FrameSample []byte

var _ Frame = FrameSample{}

func (s FrameSample) Size() int {
	return len(s) * 2
}

func (s FrameSample) CopyTo(dst []byte) (int, error) {
	if len(dst) < len(s) {
		return 0, io.ErrShortBuffer
	}
	return copy(dst, s), nil
}

func (s FrameSample) Clear() {
	for i := range s {
		s[i] = 0
	}
}

func (s *FrameSample) WriteSample(data FrameSample) error {
	*s = append(*s, data...)
	return nil
}

type FrameWriter = WriteCloser[FrameSample]

