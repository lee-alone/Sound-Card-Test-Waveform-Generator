package format

import (
	"encoding/binary"
	"math"
)

const (
	floatTolerance = 1e-12

	maxInt16 = 32767.0
	minInt16 = -32768.0

	maxInt24 = 8388607.0
	minInt24 = -8388608.0

	maxInt32From24 = 8388607.0 * 256.0
	minInt32From24 = -8388608.0 * 256.0
)

// Quantize16Int converts float64 samples to little-endian 16-bit signed integers.
func Quantize16Int(dst []byte, src []float64, allowSaturation bool) error {
	needed := len(src) * 2
	if len(dst) < needed {
		return ErrBufferTooSmall
	}

	for i, s := range src {
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

		idx := i * 2
		dst[idx] = byte(v)
		dst[idx+1] = byte(v >> 8)
	}

	return nil
}

// Quantize24Packed converts float64 samples to little-endian packed 24-bit (3 bytes) signed integers.
func Quantize24Packed(dst []byte, src []float64, allowSaturation bool) error {
	needed := len(src) * 3
	if len(dst) < needed {
		return ErrBufferTooSmall
	}

	for i, s := range src {
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

		idx := i * 3
		dst[idx] = byte(v)
		dst[idx+1] = byte(v >> 8)
		dst[idx+2] = byte(v >> 16)
	}

	return nil
}

// Quantize24In32 converts float64 samples to 24-bit valid audio in little-endian 32-bit container (left-aligned).
// Byte layout: [Padding0, Low, Mid, High]
func Quantize24In32(dst []byte, src []float64, allowSaturation bool) error {
	needed := len(src) * 4
	if len(dst) < needed {
		return ErrBufferTooSmall
	}

	for i, s := range src {
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

		// Shift left by 8 bits for 24-valid-in-32 left alignment
		v32 := uint32(v) << 8

		idx := i * 4
		dst[idx] = byte(v32)         // 0x00 padding
		dst[idx+1] = byte(v32 >> 8)  // low
		dst[idx+2] = byte(v32 >> 16) // mid
		dst[idx+3] = byte(v32 >> 24) // high (sign)
	}

	return nil
}

// Quantize32Float converts float64 samples to IEEE 754 32-bit float in little-endian format.
func Quantize32Float(dst []byte, src []float64, allowSaturation bool) error {
	needed := len(src) * 4
	if len(dst) < needed {
		return ErrBufferTooSmall
	}

	for i, s := range src {
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

		f32 := float32(s)
		u32 := math.Float32bits(f32)
		binary.LittleEndian.PutUint32(dst[i*4:(i+1)*4], u32)
	}

	return nil
}
