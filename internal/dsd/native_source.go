package dsd

import "errors"

var (
	ErrNativeSourceEmpty    = errors.New("native DSD source bitstream is empty")
	ErrNativeSourceTooSmall = errors.New("destination buffer is too small for native DSD frames")
	ErrNativeSourceChannels = errors.New("native DSD channel count must be positive")
)

// NativeDSDSource supplies raw MSB-first DSD bytes for the ASIO Native DSD
// path. The staging layout is planar-concatenated (channel 0 then channel 1),
// matching asio.NativeDSDStream.
type NativeDSDSource struct {
	bitstream []byte
	channels  int
	position  int
}

// NewNativeDSDSource creates a deterministic looping native DSD data source.
func NewNativeDSDSource(bitstream []byte, channels int) (*NativeDSDSource, error) {
	if len(bitstream) == 0 {
		return nil, ErrNativeSourceEmpty
	}
	if channels <= 0 {
		return nil, ErrNativeSourceChannels
	}
	owned := make([]byte, len(bitstream))
	copy(owned, bitstream)
	return &NativeDSDSource{bitstream: owned, channels: channels}, nil
}

// ProvideFrames fills dst with a planar-concatenated raw DSD bitstream.
func (s *NativeDSDSource) ProvideFrames(dst []byte, numFrames int) error {
	if s == nil || len(s.bitstream) == 0 {
		return ErrNativeSourceEmpty
	}
	if s.channels <= 0 {
		return ErrNativeSourceChannels
	}
	if numFrames < 0 {
		return ErrNativeSourceTooSmall
	}
	bytesPerChannel := (numFrames + 7) / 8
	required := bytesPerChannel * s.channels
	if required > len(dst) {
		return ErrNativeSourceTooSmall
	}
	if required == 0 {
		return nil
	}

	for ch := 0; ch < s.channels; ch++ {
		dstStart := ch * bytesPerChannel
		copyNativeCycle(dst[dstStart:dstStart+bytesPerChannel], s.bitstream, s.position)
	}
	s.position = (s.position + bytesPerChannel) % len(s.bitstream)
	return nil
}

// Reset rewinds the source to the deterministic first byte.
func (s *NativeDSDSource) Reset() {
	if s != nil {
		s.position = 0
	}
}

func copyNativeCycle(dst, src []byte, start int) {
	if len(dst) == 0 || len(src) == 0 {
		return
	}
	pos := start % len(src)
	written := 0
	for written < len(dst) {
		n := copy(dst[written:], src[pos:])
		written += n
		pos = 0
	}
}
