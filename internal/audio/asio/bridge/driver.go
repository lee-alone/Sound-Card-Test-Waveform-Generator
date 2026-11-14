package bridge

// Driver defines the unified abstraction for interacting with an ASIO driver instance.
type Driver interface {
	Init(sysHandle uintptr) error
	GetDriverName() string
	GetDriverVersion() int
	GetErrorMessage() string
	Start() error
	Stop() error
	GetChannels() (numInputs, numOutputs int, err error)
	GetLatencies() (inputLatency, outputLatency int, err error)
	GetBufferSize() (BufferSizeInfo, error)
	CanSampleRate(sampleRate float64) bool
	GetSampleRate() (float64, error)
	SetSampleRate(sampleRate float64) error
	GetClockSources() ([]ClockSource, error)
	SetClockSource(reference int) error
	GetChannelInfo(channel int, isInput bool) (ChannelInfo, error)
	CreateBuffers(numChannels int, bufferSize int) error
	DisposeBuffers() error
	ControlPanel() error
	Future(selector int, opt uintptr) error
	GetWaitHandle() uintptr
	GetActiveBufferIndex() int
	WriteInterleavedPCM(bufferIndex int, src []byte, numFrames int, bytesPerSample int, numChannels int)
	WritePlanarDSD(bufferIndex int, channel int, src []byte)
	Close() error
}
