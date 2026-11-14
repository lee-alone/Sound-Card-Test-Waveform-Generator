package format

import "errors"

var (
	// ErrPeakExceedsFullScale is returned when a signal exceeds 0 dBFS (amplitude > 1.0) and saturation is not allowed.
	ErrPeakExceedsFullScale = errors.New("signal peak exceeds 0 dBFS full scale limit")

	// ErrBufferTooSmall is returned when destination buffer has insufficient capacity.
	ErrBufferTooSmall = errors.New("destination buffer capacity is too small for output frames")

	// ErrUnsupportedFormat is returned when an unknown or unhandled SampleFormat is requested.
	ErrUnsupportedFormat = errors.New("unsupported audio sample format")

	// ErrChannelMismatch is returned when provided channel slice count does not match format channel count.
	ErrChannelMismatch = errors.New("channel slice count does not match target format channel count")
)

// Options defines configuration flags for format encoding and quantization.
type Options struct {
	// AllowSaturation when true permits hard clipping to max integer range instead of rejecting overloaded buffers.
	// Strictly false by default in production; only enabled in explicit test mode.
	AllowSaturation bool
}
