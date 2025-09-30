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
	"testing"
)

func TestSample(t *testing.T) {
	// Test VideoFrame interface implementation
	frame := Sample([]byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x0a})

	// Test Size
	if frame.Size() != 8 {
		t.Errorf("Expected size 8, got %d", frame.Size())
	}

	// Test CopyTo
	dst := make([]byte, 10)
	n, err := frame.CopyTo(dst)
	if err != nil {
		t.Errorf("CopyTo failed: %v", err)
	}
	if n != 8 {
		t.Errorf("Expected copied bytes 8, got %d", n)
	}

	// Test CopyTo with insufficient buffer
	dst = make([]byte, 4)
	n, err = frame.CopyTo(dst)
	if err == nil {
		t.Error("Expected error for insufficient buffer")
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes copied, got %d", n)
	}
}

func TestCodecInfo(t *testing.T) {
	info := CodecInfo()

	if info.SDPName != "H264/90000" {
		t.Errorf("Expected SDPName 'H264/90000', got '%s'", info.SDPName)
	}

	if info.SampleRate != 90000 {
		t.Errorf("Expected SampleRate 90000, got %d", info.SampleRate)
	}

	if info.RTPClockRate != 90000 {
		t.Errorf("Expected RTPClockRate 90000, got %d", info.RTPClockRate)
	}

	if info.RTPDefType != 96 {
		t.Errorf("Expected RTPDefType 96, got %d", info.RTPDefType)
	}

	if info.RTPIsStatic {
		t.Error("Expected RTPIsStatic false")
	}

	if info.Priority != 100 {
		t.Errorf("Expected Priority 100, got %d", info.Priority)
	}

	if info.Disabled {
		t.Error("Expected Disabled false")
	}

	if info.FileExt != ".h264" {
		t.Errorf("Expected FileExt '.h264', got '%s'", info.FileExt)
	}
}

func TestEncoder(t *testing.T) {
	// Create a mock writer
	var receivedSamples []Sample
	mockWriter := &mockWriter[Sample]{
		writeSample: func(sample Sample) error {
			receivedSamples = append(receivedSamples, sample)
			return nil
		},
	}

	encoder := &Encoder{w: mockWriter}

	// Test WriteSample
	sample := Sample([]byte{0x00, 0x00, 0x00, 0x01, 0x67})
	err := encoder.WriteSample(sample)
	if err != nil {
		t.Errorf("WriteSample failed: %v", err)
	}

	if len(receivedSamples) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(receivedSamples))
	}

	// Test SampleRate
	if encoder.SampleRate() != 90000 {
		t.Errorf("Expected SampleRate 90000, got %d", encoder.SampleRate())
	}

	// Test String
	if encoder.String() != "H264Encoder" {
		t.Errorf("Expected String 'H264Encoder', got '%s'", encoder.String())
	}
}

func TestEncodeDecode(t *testing.T) {
	// Create a mock writer
	var receivedSamples []Sample
	mockWriter := &mockWriter[Sample]{
		writeSample: func(sample Sample) error {
			receivedSamples = append(receivedSamples, sample)
			return nil
		},
	}

	// Test Encode
	encodedWriter := Encode(mockWriter)
	if encodedWriter == nil {
		t.Error("Encode returned nil")
	}

	// Test Decode
	decodedWriter := Decode(mockWriter)
	if decodedWriter == nil {
		t.Error("Decode returned nil")
	}
}

func TestNewCodec(t *testing.T) {
	codec := NewCodec()
	if codec == nil {
		t.Error("NewCodec returned nil")
	}

	info := codec.Info()
	if info.SDPName != "H264/90000" {
		t.Errorf("Expected SDPName 'H264/90000', got '%s'", info.SDPName)
	}
}

// mockWriter is a mock implementation of media.Writer for testing
type mockWriter[T any] struct {
	writeSample func(T) error
}

func (m *mockWriter[T]) WriteSample(sample T) error {
	if m.writeSample != nil {
		return m.writeSample(sample)
	}
	return nil
}

func (m *mockWriter[T]) SampleRate() int {
	return 90000
}

func (m *mockWriter[T]) String() string {
	return "mockWriter"
}
