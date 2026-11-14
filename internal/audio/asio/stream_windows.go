package asio

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio/bridge"
)

// Stream implements audio.AudioStream for Windows ASIO low-latency streaming.
type Stream struct {
	mu            sync.Mutex
	driver        bridge.Driver
	config        audio.StreamConfig
	bufferSize    int
	stagingBuffer []byte
	running       bool
	closed        bool
}

// NewStream creates, negotiates and initializes an ASIO audio stream.
func NewStream(driver bridge.Driver, config audio.StreamConfig) (*Stream, error) {
	if driver == nil {
		return nil, errors.New("driver is nil")
	}

	if err := driver.Init(0); err != nil {
		return nil, fmt.Errorf("failed to initialize ASIO driver: %w", err)
	}

	// Set Sample Rate
	targetRate := float64(config.Format.SampleRate)
	if !driver.CanSampleRate(targetRate) {
		return nil, fmt.Errorf("driver does not support sample rate: %d", config.Format.SampleRate)
	}
	if err := driver.SetSampleRate(targetRate); err != nil {
		return nil, fmt.Errorf("failed to set sample rate %d: %w", config.Format.SampleRate, err)
	}

	// Negotiate buffer size
	bInfo, err := driver.GetBufferSize()
	if err != nil {
		return nil, fmt.Errorf("failed to query buffer size: %w", err)
	}

	bufFrames := bInfo.PreferredSize
	if config.BufferDuration > 0 {
		requestedFrames := int(targetRate * config.BufferDuration.Seconds())
		if requestedFrames >= bInfo.MinSize && requestedFrames <= bInfo.MaxSize {
			bufFrames = requestedFrames
		}
	}
	if bufFrames < bInfo.MinSize {
		bufFrames = bInfo.MinSize
	}
	if bufFrames > bInfo.MaxSize && bInfo.MaxSize > 0 {
		bufFrames = bInfo.MaxSize
	}

	// Create Buffers
	if err := driver.CreateBuffers(config.Format.Channels, bufFrames); err != nil {
		return nil, fmt.Errorf("failed to create ASIO buffers: %w", err)
	}

	stagingLen := bufFrames * config.Format.BytesPerFrame()
	if stagingLen == 0 {
		stagingLen = bufFrames * 2 * 4 // default stereo 32-bit
	}

	return &Stream{
		driver:        driver,
		config:        config,
		bufferSize:    bufFrames,
		stagingBuffer: make([]byte, stagingLen),
	}, nil
}

func (s *Stream) Start() error {
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

func (s *Stream) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return nil
	}
	_ = s.driver.Stop()
	s.running = false
	return nil
}

func (s *Stream) GetBuffer(numFrames int) (uintptr, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, errors.New("stream closed")
	}
	requiredBytes := numFrames * s.config.Format.BytesPerFrame()
	if len(s.stagingBuffer) < requiredBytes {
		s.stagingBuffer = make([]byte, requiredBytes)
	}
	return uintptr(unsafe.Pointer(&s.stagingBuffer[0])), nil
}

func (s *Stream) ReleaseBuffer(numFramesWritten int, flags uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("stream closed")
	}

	// Handle silent flag (0x01 = AUDCLNT_BUFFERFLAGS_SILENT)
	if flags&0x01 != 0 {
		for i := range s.stagingBuffer {
			s.stagingBuffer[i] = 0
		}
	}

	bufIdx := s.driver.GetActiveBufferIndex()
	bytesPerSample := s.config.Format.SampleFormat.BytesPerSample()
	s.driver.WriteInterleavedPCM(bufIdx, s.stagingBuffer, numFramesWritten, bytesPerSample, s.config.Format.Channels)
	return nil
}

func (s *Stream) GetBufferSize() (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return uint32(s.bufferSize), nil
}

func (s *Stream) GetDevicePeriod() (defaultPeriod, minPeriod time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	periodSec := float64(s.bufferSize) / float64(s.config.Format.SampleRate)
	d := time.Duration(periodSec * float64(time.Second))
	return d, d, nil
}

func (s *Stream) GetWaitHandle() uintptr {
	return s.driver.GetWaitHandle()
}

func (s *Stream) Close() error {
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
