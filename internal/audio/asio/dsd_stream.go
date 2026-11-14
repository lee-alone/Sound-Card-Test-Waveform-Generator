package asio

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"sound-signal-generator/internal/audio/asio/bridge"
	"sound-signal-generator/internal/dsd"
)

// NativeDSDStream manages hardware native DSD direct streaming via ASIO.
type NativeDSDStream struct {
	mu           sync.Mutex
	driver       bridge.Driver
	format       dsd.DSDFormat
	channels     int
	sampleType   bridge.SampleType
	bufferFrames int // DSD sample frames per buffer (must be multiple of 8)
	bufferBytes  int // bufferFrames / 8 per channel
	planarLeft   []byte
	planarRight  []byte
	stagingBuf   []byte
	running      bool
	closed       bool
}

// NewNativeDSDStream initializes and negotiates a Native DSD stream over ASIO.
func NewNativeDSDStream(driver bridge.Driver, format dsd.DSDFormat, channels int) (*NativeDSDStream, error) {
	if driver == nil {
		return nil, errors.New("driver is nil")
	}
	if channels <= 0 {
		channels = 2
	}

	if err := driver.Init(0); err != nil {
		return nil, fmt.Errorf("failed to initialize ASIO driver: %w", err)
	}

	bitRate := float64(format.BitRate())
	if !driver.CanSampleRate(bitRate) {
		return nil, fmt.Errorf("driver does not support Native DSD rate: %.0f Hz (%s)", bitRate, format.String())
	}

	if err := driver.SetSampleRate(bitRate); err != nil {
		return nil, fmt.Errorf("failed to set Native DSD sample rate: %w", err)
	}

	// Query driver channel 0 sample type
	chInfo, err := driver.GetChannelInfo(0, false)
	sampleType := chInfo.SampleType
	if err != nil || !sampleType.IsDSD() {
		// Default to MSB1 if not explicitly reported by driver mock
		sampleType = bridge.SampleTypeDSDInt8MSB1
	}

	// Negotiate buffer size in DSD frames
	bInfo, err := driver.GetBufferSize()
	if err != nil {
		return nil, fmt.Errorf("failed to query buffer size: %w", err)
	}

	bufFrames := bInfo.PreferredSize
	// Ensure bufferFrames is a multiple of 8 for whole-byte DSD streaming
	if bufFrames%8 != 0 {
		bufFrames = (bufFrames / 8) * 8
	}
	if bufFrames < 64 {
		bufFrames = 64
	}

	if err := driver.CreateBuffers(channels, bufFrames); err != nil {
		return nil, fmt.Errorf("failed to create ASIO DSD buffers: %w", err)
	}

	bufBytes := bufFrames / 8
	return &NativeDSDStream{
		driver:       driver,
		format:       format,
		channels:     channels,
		sampleType:   sampleType,
		bufferFrames: bufFrames,
		bufferBytes:  bufBytes,
		planarLeft:   make([]byte, bufBytes),
		planarRight:  make([]byte, bufBytes),
		stagingBuf:   make([]byte, bufBytes*channels),
	}, nil
}

func (s *NativeDSDStream) Format() dsd.DSDFormat {
	return s.format
}

func (s *NativeDSDStream) SampleType() bridge.SampleType {
	return s.sampleType
}

func (s *NativeDSDStream) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("stream closed")
	}
	if s.running {
		return nil
	}
	if err := s.driver.Start(); err != nil {
		return err
	}
	s.running = true
	return nil
}

func (s *NativeDSDStream) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return nil
	}
	_ = s.driver.Stop()
	s.running = false
	return nil
}

// GetBuffer returns a pointer to staging buffer for writing DSD bitstream frames.
func (s *NativeDSDStream) GetBuffer(numFrames int) (uintptr, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, errors.New("stream closed")
	}
	requiredBytes := (numFrames / 8) * s.channels
	if len(s.stagingBuf) < requiredBytes {
		s.stagingBuf = make([]byte, requiredBytes)
	}
	return uintptr(unsafe.Pointer(&s.stagingBuf[0])), nil
}

// ReleaseBuffer converts staging planar DSD bytes and commits them directly to ASIO driver.
func (s *NativeDSDStream) ReleaseBuffer(numFramesWritten int, flags uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("stream closed")
	}

	bytesPerChannel := numFramesWritten / 8
	if bytesPerChannel > s.bufferBytes {
		bytesPerChannel = s.bufferBytes
	}

	// De-interleave or split channels if staging is interleaved
	// Staging layout: channel 0 [bytesPerChannel], channel 1 [bytesPerChannel]
	ch0 := s.stagingBuf[:bytesPerChannel]
	ch1 := s.stagingBuf[bytesPerChannel : bytesPerChannel*2]

	if flags&0x01 != 0 { // Mute / Silent: 0x69 or alternating 0xAA/0x55 (standard DSD silence pattern)
		for i := 0; i < bytesPerChannel; i++ {
			s.planarLeft[i] = 0x69
			s.planarRight[i] = 0x69
		}
	} else {
		_ = ConvertPlanarDSD(s.planarLeft[:bytesPerChannel], ch0, s.sampleType, dsd.BitOrderMSBFirst)
		_ = ConvertPlanarDSD(s.planarRight[:bytesPerChannel], ch1, s.sampleType, dsd.BitOrderMSBFirst)
	}

	bufIdx := s.driver.GetActiveBufferIndex()
	s.driver.WritePlanarDSD(bufIdx, 0, s.planarLeft[:bytesPerChannel])
	if s.channels > 1 {
		s.driver.WritePlanarDSD(bufIdx, 1, s.planarRight[:bytesPerChannel])
	}
	return nil
}

func (s *NativeDSDStream) GetBufferSize() (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return uint32(s.bufferFrames), nil
}

func (s *NativeDSDStream) GetDevicePeriod() (defaultPeriod, minPeriod time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sec := float64(s.bufferFrames) / float64(s.format.BitRate())
	d := time.Duration(sec * float64(time.Second))
	return d, d, nil
}

func (s *NativeDSDStream) GetWaitHandle() uintptr {
	return s.driver.GetWaitHandle()
}

func (s *NativeDSDStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	_ = s.driver.Stop()
	_ = s.driver.DisposeBuffers()
	s.closed = true
	s.running = false
	return nil
}
