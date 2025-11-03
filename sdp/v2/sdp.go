package v2

import (
	v1 "github.com/livekit/media-sdk/sdp"
	"github.com/pion/sdp/v3"
)

func NewSDP(sdpData []byte) (*Session, error) {
	s := &Session{}
	if err := s.Unmarshal(sdpData); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) Unmarshal(sdpData []byte) error {
	if err := s.Description.Unmarshal(sdpData); err != nil {
		return err
	}
	if err := s.FromSDP(s.Description); err != nil {
		return err
	}
	return nil
}

// FromSDP populates the structure from a pion SessionDescription.
func (s *Session) FromSDP(sd sdp.SessionDescription) error {
	panic("not implemented")
}

func (s *Session) Marshal() ([]byte, error) {
	if err := s.ToSDP(); err != nil {
		return nil, err
	}
	return s.Description.Marshal()
}

// ToSDP takes the data in the structure and syncs it to the pion SessionDescription and MediaDescription.
// it preserves any raw attributes that are not explicitly modeled.
func (s *Session) ToSDP() error {
	panic("not implemented")
}

func (s *Session) SelectCodecs() error {
	if s.Audio != nil {
		if err := s.Audio.SelectCodec(); err != nil {
			return err
		}
	}
	if s.Video != nil {
		if err := s.Video.SelectCodec(); err != nil {
			return err
		}
	}
	return nil
}

// Find the best codec according to priority rules and set by Codec.Info().Priority.
func (m *MediaSection) SelectCodec() error {
	panic("not implemented")
}

// V1MediaConfig converts the SDP to a v1 MediaConfig structure.
// It only contain audio as video support was added in v2.
func (s *Session) V1MediaConfig() (v1.MediaConfig, error) {
	panic("not implemented")
}
