package bridge

import (
	"errors"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// VendorProfile defines specific driver quirks and capabilities for multi-vendor simulation.
type VendorProfile struct {
	Name               string
	Version            int
	MaxInputChannels   int
	MaxOutputChannels  int
	SupportedRates     []float64
	SupportedDSDTypes  []SampleType
	DefaultPCMType     SampleType
	MinBufferSize      int
	MaxBufferSize      int
	PreferredSize      int
	Granularity        int // -1 = power of two, >= 0 = step
	InputOffsetFrames  int
	OutputOffsetFrames int
	SupportsDSD        bool
}

// Pre-defined vendor profiles matching real hardware drivers
var (
	ProfileThesycon = VendorProfile{
		Name:              "Thesycon USB Audio 2.0 Class Driver",
		Version:           0x0512,
		MaxInputChannels:  2,
		MaxOutputChannels: 2,
		SupportedRates: []float64{
			44100, 48000, 88200, 96000, 176400, 192000, 352800, 384000, 768000,
			2822400, 5644800, 11289600, 22579200, // DSD64..DSD512
		},
		SupportedDSDTypes:  []SampleType{SampleTypeDSDInt8MSB1, SampleTypeDSDInt8LSB1},
		DefaultPCMType:     SampleTypeInt32LSB,
		MinBufferSize:      64,
		MaxBufferSize:      4096,
		PreferredSize:      256,
		Granularity:        -1, // Power of two
		InputOffsetFrames:  32,
		OutputOffsetFrames: 32,
		SupportsDSD:        true,
	}

	ProfileYamaha = VendorProfile{
		Name:              "Yamaha Steinberg USB ASIO",
		Version:           0x0211,
		MaxInputChannels:  2,
		MaxOutputChannels: 2,
		SupportedRates: []float64{
			44100, 48000, 88200, 96000, 176400, 192000,
		},
		SupportedDSDTypes:  nil,
		DefaultPCMType:     SampleTypeInt24LSB,
		MinBufferSize:      64,
		MaxBufferSize:      2048,
		PreferredSize:      128,
		Granularity:        64,
		InputOffsetFrames:  24,
		OutputOffsetFrames: 24,
		SupportsDSD:        false,
	}

	ProfileCombo384 = VendorProfile{
		Name:              "ASIO Combo384 Driver",
		Version:           0x0103,
		MaxInputChannels:  0,
		MaxOutputChannels: 2,
		SupportedRates: []float64{
			44100, 48000, 88200, 96000, 176400, 192000, 352800, 384000,
			2822400, 5644800, 11289600, 22579200, // DSD64..DSD512
		},
		SupportedDSDTypes:  []SampleType{SampleTypeDSDInt8LSB1, SampleTypeDSDInt8MSB1},
		DefaultPCMType:     SampleTypeInt32LSB,
		MinBufferSize:      64,
		MaxBufferSize:      4096,
		PreferredSize:      512,
		Granularity:        -1,
		InputOffsetFrames:  0,
		OutputOffsetFrames: 48,
		SupportsDSD:        true,
	}

	ProfileASIO4ALL = VendorProfile{
		Name:              "ASIO4ALL v2",
		Version:           0x020E,
		MaxInputChannels:  2,
		MaxOutputChannels: 2,
		SupportedRates: []float64{
			44100, 48000, 88200, 96000, 176400, 192000,
		},
		SupportedDSDTypes:  nil,
		DefaultPCMType:     SampleTypeInt16LSB,
		MinBufferSize:      64,
		MaxBufferSize:      2048,
		PreferredSize:      512,
		Granularity:        32,
		InputOffsetFrames:  16,
		OutputOffsetFrames: 16,
		SupportsDSD:        false,
	}
)

// MockDriver implements the Driver interface for deterministic testing without live hardware.
type MockDriver struct {
	mu            sync.Mutex
	profile       VendorProfile
	initialized   bool
	running       bool
	sampleRate    float64
	bufferSize    int
	numChannels   int
	activeBufIdx  int
	waitHandle    windows.Handle
	tickerStop    chan struct{}
	writtenPCM    [][]byte
	writtenDSD    [][][]byte // [channel][bufIndex][]byte
	clockSources  []ClockSource
	selectedClock int
}

// NewMockDriver creates a mock ASIO driver with the given vendor profile.
func NewMockDriver(profile VendorProfile) *MockDriver {
	h, _ := windows.CreateEvent(nil, 0, 0, nil)
	return &MockDriver{
		profile:     profile,
		sampleRate:  48000.0,
		bufferSize:  profile.PreferredSize,
		numChannels: profile.MaxOutputChannels,
		waitHandle:  h,
		clockSources: []ClockSource{
			{Index: 0, Name: "Internal Master Clock", IsCurrentSource: true},
			{Index: 1, Name: "S/PDIF Word Clock", IsCurrentSource: false},
		},
	}
}

func (m *MockDriver) Init(sysHandle uintptr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initialized = true
	return nil
}

func (m *MockDriver) GetDriverName() string {
	return m.profile.Name
}

func (m *MockDriver) GetDriverVersion() int {
	return m.profile.Version
}

func (m *MockDriver) GetErrorMessage() string {
	return "No error"
}

func (m *MockDriver) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.initialized {
		return errors.New("driver not initialized")
	}
	if m.running {
		return nil
	}
	m.running = true
	m.tickerStop = make(chan struct{})

	// Calculate period
	periodMs := float64(m.bufferSize) / m.sampleRate * 1000.0
	if periodMs < 1.0 {
		periodMs = 1.0
	}

	go func(stopCh chan struct{}, period time.Duration, h windows.Handle) {
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				m.mu.Lock()
				m.activeBufIdx = 1 - m.activeBufIdx
				_ = windows.SetEvent(h)
				m.mu.Unlock()
			}
		}
	}(m.tickerStop, time.Duration(periodMs*float64(time.Millisecond)), m.waitHandle)

	return nil
}

func (m *MockDriver) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil
	}
	m.running = false
	if m.tickerStop != nil {
		close(m.tickerStop)
		m.tickerStop = nil
	}
	return nil
}

func (m *MockDriver) GetChannels() (numInputs, numOutputs int, err error) {
	return m.profile.MaxInputChannels, m.profile.MaxOutputChannels, nil
}

func (m *MockDriver) GetLatencies() (inputLatency, outputLatency int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bufferSize + m.profile.InputOffsetFrames, m.bufferSize + m.profile.OutputOffsetFrames, nil
}

func (m *MockDriver) GetBufferSize() (BufferSizeInfo, error) {
	return BufferSizeInfo{
		MinSize:       m.profile.MinBufferSize,
		MaxSize:       m.profile.MaxBufferSize,
		PreferredSize: m.profile.PreferredSize,
		Granularity:   m.profile.Granularity,
	}, nil
}

func (m *MockDriver) CanSampleRate(sampleRate float64) bool {
	for _, r := range m.profile.SupportedRates {
		if r == sampleRate {
			return true
		}
	}
	return false
}

func (m *MockDriver) GetSampleRate() (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sampleRate, nil
}

func (m *MockDriver) SetSampleRate(sampleRate float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.CanSampleRate(sampleRate) {
		return errors.New("unsupported sample rate")
	}
	m.sampleRate = sampleRate
	return nil
}

func (m *MockDriver) GetClockSources() ([]ClockSource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ClockSource, len(m.clockSources))
	copy(out, m.clockSources)
	return out, nil
}

func (m *MockDriver) SetClockSource(reference int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if reference < 0 || reference >= len(m.clockSources) {
		return errors.New("invalid clock source index")
	}
	for i := range m.clockSources {
		m.clockSources[i].IsCurrentSource = (i == reference)
	}
	m.selectedClock = reference
	return nil
}

func (m *MockDriver) GetChannelInfo(channel int, isInput bool) (ChannelInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sampleType := m.profile.DefaultPCMType
	// If sample rate is DSD rate and driver supports DSD, report DSD sample type
	if m.profile.SupportsDSD && m.sampleRate >= 2822400 {
		if len(m.profile.SupportedDSDTypes) > 0 {
			sampleType = m.profile.SupportedDSDTypes[0]
		}
	}

	return ChannelInfo{
		Channel:      channel,
		IsInput:      isInput,
		IsActive:     true,
		ChannelGroup: 0,
		SampleType:   sampleType,
		Name:         m.profile.Name,
	}, nil
}

func (m *MockDriver) CreateBuffers(numChannels int, bufferSize int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.numChannels = numChannels
	m.bufferSize = bufferSize
	m.writtenPCM = make([][]byte, 2)
	m.writtenDSD = make([][][]byte, numChannels)
	for ch := 0; ch < numChannels; ch++ {
		m.writtenDSD[ch] = make([][]byte, 2)
	}
	return nil
}

func (m *MockDriver) DisposeBuffers() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writtenPCM = nil
	m.writtenDSD = nil
	return nil
}

func (m *MockDriver) ControlPanel() error {
	return nil
}

func (m *MockDriver) Future(selector int, opt uintptr) error {
	return nil
}

func (m *MockDriver) GetWaitHandle() uintptr {
	return uintptr(m.waitHandle)
}

func (m *MockDriver) GetActiveBufferIndex() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeBufIdx
}

func (m *MockDriver) WriteInterleavedPCM(bufferIndex int, src []byte, numFrames int, bytesPerSample int, numChannels int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if bufferIndex >= 0 && bufferIndex < len(m.writtenPCM) {
		buf := make([]byte, len(src))
		copy(buf, src)
		m.writtenPCM[bufferIndex] = buf
	}
}

func (m *MockDriver) WritePlanarDSD(bufferIndex int, channel int, src []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if channel >= 0 && channel < len(m.writtenDSD) && bufferIndex >= 0 && bufferIndex < 2 {
		buf := make([]byte, len(src))
		copy(buf, src)
		m.writtenDSD[channel][bufferIndex] = buf
	}
}

// WrittenDSDBuffer returns a detached copy of a simulated ASIO planar DSD buffer.
func (m *MockDriver) WrittenDSDBuffer(bufferIndex int, channel int) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if channel < 0 || channel >= len(m.writtenDSD) || bufferIndex < 0 || bufferIndex >= len(m.writtenDSD[channel]) {
		return nil
	}
	out := make([]byte, len(m.writtenDSD[channel][bufferIndex]))
	copy(out, m.writtenDSD[channel][bufferIndex])
	return out
}

// WrittenPCMBuffer returns a detached copy of a simulated ASIO PCM buffer.
func (m *MockDriver) WrittenPCMBuffer(bufferIndex int) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if bufferIndex < 0 || bufferIndex >= len(m.writtenPCM) {
		return nil
	}
	out := make([]byte, len(m.writtenPCM[bufferIndex]))
	copy(out, m.writtenPCM[bufferIndex])
	return out
}

func (m *MockDriver) Close() error {
	_ = m.Stop()
	if m.waitHandle != 0 {
		_ = windows.CloseHandle(m.waitHandle)
		m.waitHandle = 0
	}
	return nil
}
