package dsd

import (
	"errors"

	"sound-signal-generator/internal/audio"
)

const (
	// DoPMarker05 is the primary dCS DoP synchronization marker.
	DoPMarker05 byte = 0x05
	// DoPMarkerFA is the secondary dCS DoP synchronization marker.
	DoPMarkerFA byte = 0xFA
)

var (
	// ErrUnsupportedDoPFormat is returned when a non-24-bit PCM container is passed to DoP packer.
	ErrUnsupportedDoPFormat = errors.New("DoP requires 24-bit container (SampleFormat24Packed or SampleFormat24In32)")
	// ErrDestinationTooSmall is returned when destination buffer cannot hold the packed frames.
	ErrDestinationTooSmall = errors.New("destination buffer too small for DoP frame packing")
	// ErrNilBitstreamSource is returned when no bitstream data is provided to the packer.
	ErrNilBitstreamSource = errors.New("bitstream source cannot be nil")
)

// DoPPacker encapsulates zero-allocation dCS DoP v1.1 frame serialization and marker state management.
type DoPPacker struct {
	marker        byte
	bitOrder      BitOrder
	bitstream     []byte
	bitstreamPos  int
	channels      int
	sampleFormat  audio.SampleFormat
	framesPacked  uint64
	resetOnNext   bool
}

// NewDoPPacker creates a new DoP v1.1 packer instance initialized to deterministic starting marker 0x05.
func NewDoPPacker(bitstream []byte, channels int, sampleFormat audio.SampleFormat, bitOrder BitOrder) (*DoPPacker, error) {
	if len(bitstream) == 0 {
		return nil, ErrNilBitstreamSource
	}
	if channels <= 0 {
		channels = 2
	}
	if sampleFormat != audio.SampleFormat24Packed && sampleFormat != audio.SampleFormat24In32 {
		return nil, ErrUnsupportedDoPFormat
	}

	return &DoPPacker{
		marker:       DoPMarker05, // Deterministic starting marker
		bitOrder:     bitOrder,
		bitstream:    bitstream,
		bitstreamPos: 0,
		channels:     channels,
		sampleFormat: sampleFormat,
	}, nil
}

// Reset resets the marker sequence back to deterministic starting marker 0x05 and rewinds bitstream position.
// Must be called upon pause, underrun recovery, or device reopen.
func (p *DoPPacker) Reset() {
	p.marker = DoPMarker05
	p.bitstreamPos = 0
	p.framesPacked = 0
	p.resetOnNext = false
}

// CurrentMarker returns the next marker byte to be written.
func (p *DoPPacker) CurrentMarker() byte {
	return p.marker
}

// FramesPacked returns total count of frames packaged.
func (p *DoPPacker) FramesPacked() uint64 {
	return p.framesPacked
}

// PackFrames writes numFrames of interleaved DoP data into dst buffer.
// Guarantees zero heap allocation in hot path.
// Layouts strictly follow schedule:
// - 24-bit packed:   [DSD0, DSD1, Marker]
// - 24-valid-in-32:  [Padding0, DSD0, DSD1, Marker]
func (p *DoPPacker) PackFrames(dst []byte, numFrames int) (int, error) {
	bytesPerSample := p.sampleFormat.BytesPerSample()
	totalBytesRequired := numFrames * p.channels * bytesPerSample
	if len(dst) < totalBytesRequired {
		return 0, ErrDestinationTooSmall
	}

	bitstreamLen := len(p.bitstream)
	dstIdx := 0

	marker := p.marker
	pos := p.bitstreamPos
	channels := p.channels
	bitOrder := p.bitOrder

	for frame := 0; frame < numFrames; frame++ {
		// Read 2 bytes (16 bits) of DSD audio for this frame
		// If stereo or multi-channel, all channels can share the same golden test bitstream
		b0 := p.bitstream[pos]
		b1 := p.bitstream[(pos+1)%bitstreamLen]
		pos = (pos + 2) % bitstreamLen

		if bitOrder == BitOrderLSBFirst {
			b0 = reverseBits(b0)
			b1 = reverseBits(b1)
		}

		if p.sampleFormat == audio.SampleFormat24Packed {
			// [DSD0, DSD1, Marker] per channel
			for ch := 0; ch < channels; ch++ {
				dst[dstIdx] = b0
				dst[dstIdx+1] = b1
				dst[dstIdx+2] = marker
				dstIdx += 3
			}
		} else {
			// SampleFormat24In32: [Padding0, DSD0, DSD1, Marker] per channel
			for ch := 0; ch < channels; ch++ {
				dst[dstIdx] = 0x00
				dst[dstIdx+1] = b0
				dst[dstIdx+2] = b1
				dst[dstIdx+3] = marker
				dstIdx += 4
			}
		}

		// Strictly alternate marker per frame: 0x05 -> 0xFA -> 0x05 -> 0xFA
		if marker == DoPMarker05 {
			marker = DoPMarkerFA
		} else {
			marker = DoPMarker05
		}
	}

	p.marker = marker
	p.bitstreamPos = pos
	p.framesPacked += uint64(numFrames)

	return totalBytesRequired, nil
}

// ProvideFrames implements engine.RawFrameProvider for direct zero-copy DoP stream rendering.
func (p *DoPPacker) ProvideFrames(dst []byte, numFrames int) error {
	_, err := p.PackFrames(dst, numFrames)
	return err
}

// reverseBits reverses bit order in a byte (for LSBFirst).
func reverseBits(b byte) byte {
	b = (b&0xF0)>>4 | (b&0x0F)<<4
	b = (b&0xCC)>>2 | (b&0x33)<<2
	b = (b&0xAA)>>1 | (b&0x55)<<1
	return b
}
