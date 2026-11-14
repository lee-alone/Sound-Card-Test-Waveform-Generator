package audio

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SampleFormat defines audio sample representation and container size.
type SampleFormat int

const (
	// SampleFormat16Int: 16-bit signed integer (2 bytes per sample, 16-bit container)
	SampleFormat16Int SampleFormat = iota
	// SampleFormat24Packed: 24-bit signed integer packed (3 bytes per sample, 24-bit container)
	SampleFormat24Packed
	// SampleFormat24In32: 24-bit valid audio in 32-bit container (4 bytes per sample, 24 valid bits)
	SampleFormat24In32
	// SampleFormat32Float: 32-bit IEEE float (4 bytes per sample, 32-bit container)
	SampleFormat32Float
)

func (sf SampleFormat) String() string {
	switch sf {
	case SampleFormat16Int:
		return "16-bit Integer"
	case SampleFormat24Packed:
		return "24-bit Packed Integer"
	case SampleFormat24In32:
		return "24-in-32-bit Integer"
	case SampleFormat32Float:
		return "32-bit Float"
	default:
		return "Unknown"
	}
}

func (sf SampleFormat) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(sf))
}

func (sf *SampleFormat) UnmarshalJSON(data []byte) error {
	var val int
	if err := json.Unmarshal(data, &val); err == nil {
		*sf = SampleFormat(val)
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		clean := strings.ToLower(strings.TrimSpace(str))
		switch clean {
		case "0", "16", "16-bit", "16-bit integer", "sampleformat16int":
			*sf = SampleFormat16Int
		case "1", "24p", "24-bit packed", "24-bit packed integer", "sampleformat24packed":
			*sf = SampleFormat24Packed
		case "2", "24in32", "24-in-32", "24-in-32-bit integer", "sampleformat24in32":
			*sf = SampleFormat24In32
		case "3", "32f", "32-bit float", "sampleformat32float":
			*sf = SampleFormat32Float
		default:
			*sf = SampleFormat16Int
		}
		return nil
	}
	return fmt.Errorf("invalid sample format: %s", string(data))
}

// BytesPerSample returns container byte size for a single sample.
func (sf SampleFormat) BytesPerSample() int {
	switch sf {
	case SampleFormat16Int:
		return 2
	case SampleFormat24Packed:
		return 3
	case SampleFormat24In32:
		return 4
	case SampleFormat32Float:
		return 4
	default:
		return 0
	}
}

// ContainerBits returns the bit width of the sample container.
func (sf SampleFormat) ContainerBits() int {
	return sf.BytesPerSample() * 8
}

// ValidBits returns the number of valid audio data bits.
func (sf SampleFormat) ValidBits() int {
	switch sf {
	case SampleFormat16Int:
		return 16
	case SampleFormat24Packed:
		return 24
	case SampleFormat24In32:
		return 24
	case SampleFormat32Float:
		return 32
	default:
		return 0
	}
}

// AudioFormat fully describes an audio stream format configuration.
type AudioFormat struct {
	SampleRate   int          `json:"sample_rate"`
	Channels     int          `json:"channels"`
	SampleFormat SampleFormat `json:"sample_format"`
	ChannelMask  uint32       `json:"channel_mask"`
}

// BytesPerFrame returns the byte size for all interleaved channels in one sample frame.
func (af AudioFormat) BytesPerFrame() int {
	return af.SampleFormat.BytesPerSample() * af.Channels
}

func (af AudioFormat) String() string {
	return fmt.Sprintf("%dHz %s %dch", af.SampleRate, af.SampleFormat, af.Channels)
}

// SupportStatus indicates the hardware capability status for a specific audio format.
type SupportStatus int

const (
	StatusSupported SupportStatus = iota
	StatusUnsupported
	StatusDeviceBusy
	StatusDriverError
)

func (s SupportStatus) String() string {
	switch s {
	case StatusSupported:
		return "Supported"
	case StatusUnsupported:
		return "Unsupported"
	case StatusDeviceBusy:
		return "DeviceBusy"
	case StatusDriverError:
		return "DriverError"
	default:
		return "Unknown"
	}
}

// FormatSupportResult records the outcome of probing a specific format on a device.
type FormatSupportResult struct {
	Format       AudioFormat   `json:"format"`
	Status       SupportStatus `json:"status"`
	HResult      int32         `json:"hresult"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// Standard candidate sample rates defined in Phase 1 and Phase 3:
// {44.1k, 48k, 88.2k, 96k, 176.4k, 192k, 352.8k, 384k}
var CandidateSampleRates = []int{
	44100,
	48000,
	88200,
	96000,
	176400,
	192000,
	352800,
	384000,
}

// DoPCandidateSampleRates defines carrier sample rates for DoP v1.1 transmission:
// 176.4kHz (for DSD64) and 352.8kHz (for DSD128).
var DoPCandidateSampleRates = []int{
	176400,
	352800,
}

// Standard candidate sample formats: {16I, 24P, 24in32, 32F}
var CandidateSampleFormats = []SampleFormat{
	SampleFormat16Int,
	SampleFormat24Packed,
	SampleFormat24In32,
	SampleFormat32Float,
}

// StandardSpeakerChannelMaskStereo is the standard stereo speaker channel mask (SPEAKER_FRONT_LEFT | SPEAKER_FRONT_RIGHT)
const StandardSpeakerChannelMaskStereo uint32 = 0x00000001 | 0x00000002

// GenerateStandardCandidateMatrix returns the complete candidate matrix for probing.
func GenerateStandardCandidateMatrix(channels int) []AudioFormat {
	if channels <= 0 {
		channels = 2
	}
	mask := uint32(0)
	if channels == 2 {
		mask = StandardSpeakerChannelMaskStereo
	}

	matrix := make([]AudioFormat, 0, len(CandidateSampleRates)*len(CandidateSampleFormats))
	for _, rate := range CandidateSampleRates {
		for _, fmt := range CandidateSampleFormats {
			matrix = append(matrix, AudioFormat{
				SampleRate:   rate,
				Channels:     channels,
				SampleFormat: fmt,
				ChannelMask:  mask,
			})
		}
	}
	return matrix
}
