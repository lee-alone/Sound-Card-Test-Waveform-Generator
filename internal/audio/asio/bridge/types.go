package bridge

import "fmt"

// SampleType defines the sample format reported by an ASIO channel.
type SampleType int32

const (
	SampleTypeInt16MSB    SampleType = 0
	SampleTypeInt24MSB    SampleType = 1
	SampleTypeInt32MSB    SampleType = 2
	SampleTypeFloat32MSB  SampleType = 3
	SampleTypeFloat64MSB  SampleType = 4
	SampleTypeInt16LSB    SampleType = 16
	SampleTypeInt24LSB    SampleType = 17
	SampleTypeInt32LSB    SampleType = 18
	SampleTypeFloat32LSB  SampleType = 19
	SampleTypeFloat64LSB  SampleType = 20
	SampleTypeDSDInt8LSB1 SampleType = 32
	SampleTypeDSDInt8MSB1 SampleType = 33
	SampleTypeDSDInt8NER8 SampleType = 34
)

func (s SampleType) String() string {
	switch s {
	case SampleTypeInt16LSB:
		return "ASIOSTInt16LSB"
	case SampleTypeInt24LSB:
		return "ASIOSTInt24LSB"
	case SampleTypeInt32LSB:
		return "ASIOSTInt32LSB"
	case SampleTypeFloat32LSB:
		return "ASIOSTFloat32LSB"
	case SampleTypeFloat64LSB:
		return "ASIOSTFloat64LSB"
	case SampleTypeDSDInt8LSB1:
		return "ASIOSTDSDInt8LSB1"
	case SampleTypeDSDInt8MSB1:
		return "ASIOSTDSDInt8MSB1"
	case SampleTypeDSDInt8NER8:
		return "ASIOSTDSDInt8NER8"
	default:
		return fmt.Sprintf("SampleType(%d)", s)
	}
}

// IsDSD returns true if the sample type is a native DSD bitstream type.
func (s SampleType) IsDSD() bool {
	return s == SampleTypeDSDInt8LSB1 || s == SampleTypeDSDInt8MSB1 || s == SampleTypeDSDInt8NER8
}

// BytesPerSample returns bytes per sample for PCM types or 1 for DSD (1 byte holds 8 DSD bits).
func (s SampleType) BytesPerSample() int {
	switch s {
	case SampleTypeInt16LSB, SampleTypeInt16MSB:
		return 2
	case SampleTypeInt24LSB, SampleTypeInt24MSB:
		return 3
	case SampleTypeInt32LSB, SampleTypeInt32MSB, SampleTypeFloat32LSB, SampleTypeFloat32MSB:
		return 4
	case SampleTypeFloat64LSB, SampleTypeFloat64MSB:
		return 8
	case SampleTypeDSDInt8LSB1, SampleTypeDSDInt8MSB1, SampleTypeDSDInt8NER8:
		return 1
	default:
		return 4
	}
}

// BufferSizeInfo specifies minimum, maximum, preferred buffer frame counts and granularity.
type BufferSizeInfo struct {
	MinSize       int `json:"min_size"`
	MaxSize       int `json:"max_size"`
	PreferredSize int `json:"preferred_size"`
	Granularity   int `json:"granularity"`
}

// LatencyInfo records driver-reported input and output hardware latencies in sample frames.
type LatencyInfo struct {
	InputLatency  int `json:"input_latency"`
	OutputLatency int `json:"output_latency"`
}

// ChannelInfo describes an individual ASIO input or output channel.
type ChannelInfo struct {
	Channel      int        `json:"channel"`
	IsInput      bool       `json:"is_input"`
	IsActive     bool       `json:"is_active"`
	ChannelGroup int        `json:"channel_group"`
	SampleType   SampleType `json:"sample_type"`
	Name         string     `json:"name"`
}

// ClockSource describes an ASIO hardware clock selection.
type ClockSource struct {
	Index           int    `json:"index"`
	Name            string `json:"name"`
	IsCurrentSource bool   `json:"is_current_source"`
}

// DriverDesc represents registry metadata for an installed ASIO driver.
type DriverDesc struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CLSID       string `json:"clsid"`
}
