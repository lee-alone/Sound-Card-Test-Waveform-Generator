package audio

import "time"

// BackendType represents the audio driver backend implementation.
type BackendType string

const (
	BackendWASAPI BackendType = "WASAPI"
	BackendASIO   BackendType = "ASIO"
	BackendMock   BackendType = "Mock"
)

// ShareMode defines whether the audio device is opened in exclusive or shared mode.
type ShareMode int

const (
	ShareModeShared ShareMode = iota
	ShareModeExclusive
)

func (s ShareMode) String() string {
	switch s {
	case ShareModeShared:
		return "Shared"
	case ShareModeExclusive:
		return "Exclusive"
	default:
		return "Unknown"
	}
}

// StreamConfig specifies parameters for opening an audio stream.
type StreamConfig struct {
	Format         AudioFormat
	BufferFrames   int
	ShareMode      ShareMode
	BufferDuration time.Duration
	EventDriven    bool
}

// AudioStream represents an active audio playback/render stream.
type AudioStream interface {
	Start() error
	Stop() error
	GetBuffer(numFrames int) (uintptr, error)
	ReleaseBuffer(numFramesWritten int, flags uint32) error
	GetBufferSize() (uint32, error)
	GetDevicePeriod() (defaultPeriod, minPeriod time.Duration, err error)
	GetWaitHandle() uintptr
	Close() error
}

// AudioBackend defines the abstraction for platform-specific audio subsystems.
type AudioBackend interface {
	Type() BackendType
	Initialize() error
	Close() error
	EnumerateDevices() ([]DeviceInfo, error)
	GetDevice(id string) (Device, error)
	GetDefaultDevice() (Device, error)
}
