package format

import (
	"encoding/binary"
	"math"

	"sound-signal-generator/internal/audio"
)

// Interleave converts separate channel float64 buffers into an interleaved byte slice according to format.
// It is designed for real-time paths and executes with zero memory allocations.
func Interleave(dst []byte, channels [][]float64, fmt audio.AudioFormat, allowSaturation bool) error {
	numChannels := fmt.Channels
	if len(channels) != numChannels {
		return ErrChannelMismatch
	}
	if numChannels == 0 {
		return nil
	}

	numFrames := len(channels[0])
	for ch := 1; ch < numChannels; ch++ {
		if len(channels[ch]) != numFrames {
			return ErrChannelMismatch
		}
	}

	bytesPerSample := fmt.SampleFormat.BytesPerSample()
	bytesPerFrame := bytesPerSample * numChannels
	totalBytesNeeded := numFrames * bytesPerFrame

	if len(dst) < totalBytesNeeded {
		return ErrBufferTooSmall
	}

	switch fmt.SampleFormat {
	case audio.SampleFormat16Int:
		offset := 0
		for f := 0; f < numFrames; f++ {
			for ch := 0; ch < numChannels; ch++ {
				s := channels[ch][f]
				if math.Abs(s) > 1.0+floatTolerance {
					if !allowSaturation {
						return ErrPeakExceedsFullScale
					}
					if s > 1.0 {
						s = 1.0
					} else if s < -1.0 {
						s = -1.0
					}
				}
				var v int32
				if s >= 0 {
					v = int32(math.Round(s * maxInt16))
					if v > 32767 {
						v = 32767
					}
				} else {
					v = int32(math.Round(s * (-minInt16)))
					if v < -32768 {
						v = -32768
					}
				}
				dst[offset] = byte(v)
				dst[offset+1] = byte(v >> 8)
				offset += 2
			}
		}

	case audio.SampleFormat24Packed:
		offset := 0
		for f := 0; f < numFrames; f++ {
			for ch := 0; ch < numChannels; ch++ {
				s := channels[ch][f]
				if math.Abs(s) > 1.0+floatTolerance {
					if !allowSaturation {
						return ErrPeakExceedsFullScale
					}
					if s > 1.0 {
						s = 1.0
					} else if s < -1.0 {
						s = -1.0
					}
				}
				var v int32
				if s >= 0 {
					v = int32(math.Round(s * maxInt24))
					if v > 8388607 {
						v = 8388607
					}
				} else {
					v = int32(math.Round(s * (-minInt24)))
					if v < -8388608 {
						v = -8388608
					}
				}
				dst[offset] = byte(v)
				dst[offset+1] = byte(v >> 8)
				dst[offset+2] = byte(v >> 16)
				offset += 3
			}
		}

	case audio.SampleFormat24In32:
		offset := 0
		for f := 0; f < numFrames; f++ {
			for ch := 0; ch < numChannels; ch++ {
				s := channels[ch][f]
				if math.Abs(s) > 1.0+floatTolerance {
					if !allowSaturation {
						return ErrPeakExceedsFullScale
					}
					if s > 1.0 {
						s = 1.0
					} else if s < -1.0 {
						s = -1.0
					}
				}
				var v int32
				if s >= 0 {
					v = int32(math.Round(s * maxInt24))
					if v > 8388607 {
						v = 8388607
					}
				} else {
					v = int32(math.Round(s * (-minInt24)))
					if v < -8388608 {
						v = -8388608
					}
				}
				v32 := uint32(v) << 8
				dst[offset] = byte(v32)
				dst[offset+1] = byte(v32 >> 8)
				dst[offset+2] = byte(v32 >> 16)
				dst[offset+3] = byte(v32 >> 24)
				offset += 4
			}
		}

	case audio.SampleFormat32Float:
		offset := 0
		for f := 0; f < numFrames; f++ {
			for ch := 0; ch < numChannels; ch++ {
				s := channels[ch][f]
				if math.Abs(s) > 1.0+floatTolerance {
					if !allowSaturation {
						return ErrPeakExceedsFullScale
					}
					if s > 1.0 {
						s = 1.0
					} else if s < -1.0 {
						s = -1.0
					}
				}
				u32 := math.Float32bits(float32(s))
				binary.LittleEndian.PutUint32(dst[offset:offset+4], u32)
				offset += 4
			}
		}

	default:
		return ErrUnsupportedFormat
	}

	return nil
}
