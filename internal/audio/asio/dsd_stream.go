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

var (
	ErrNativeDSDUnsupported = errors.New("driver does not expose a Native DSD ASIO sample type")
	ErrInvalidDSDBufferSize = errors.New("invalid Native DSD buffer size")
)

// NativeDSDStream manages hardware native DSD direct streaming via ASIO.
// ASIO receives raw 1-bit DSD bytes only; no PCM/DoP packing is performed.
type NativeDSDStream struct {
	mu           sync.Mutex
	driver       bridge.Driver
	format       dsd.DSDFormat
	channels     int
	sampleTypes  []bridge.SampleType
	bufferFrames int // DSD bit frames per buffer, always a multiple of 8
	bufferBytes  int // whole DSD bytes per channel per buffer
	planar       [][]byte
	stagingBuf   []byte
	running      bool
	closed       bool
}

// NewNativeDSDStream initializes and negotiates a Native DSD stream using the
// driver's preferred buffer size.
func NewNativeDSDStream(driver bridge.Driver, format dsd.DSDFormat, channels int) (*NativeDSDStream, error) {
	return NewNativeDSDStreamWithBufferSize(driver, format, channels, 0)
}

// NewNativeDSDStreamWithBufferSize negotiates Native DSD using an explicitly
// requested buffer size when it falls within the driver's reported limits.
func NewNativeDSDStreamWithBufferSize(driver bridge.Driver, format dsd.DSDFormat, channels int, requestedBufferFrames int) (*NativeDSDStream, error) {
	if driver == nil {
		return nil, errors.New("driver is nil")
	}
	if format.BitRate() <= 0 {
		return nil, fmt.Errorf("invalid Native DSD format: %s", format.String())
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

	_, numOutputs, err := driver.GetChannels()
	if err != nil {
		return nil, fmt.Errorf("failed to query ASIO output channels: %w", err)
	}
	if channels > numOutputs {
		return nil, fmt.Errorf("requested %d Native DSD channels, driver exposes %d outputs", channels, numOutputs)
	}

	sampleTypes := make([]bridge.SampleType, channels)
	for ch := 0; ch < channels; ch++ {
		chInfo, err := driver.GetChannelInfo(ch, false)
		if err != nil {
			return nil, fmt.Errorf("failed to query Native DSD channel %d: %w", ch, err)
		}
		if !chInfo.SampleType.IsDSD() {
			return nil, fmt.Errorf("%w: channel %d reports %s at %s", ErrNativeDSDUnsupported, ch, chInfo.SampleType.String(), format.String())
		}
		sampleTypes[ch] = chInfo.SampleType
	}

	bInfo, err := driver.GetBufferSize()
	if err != nil {
		return nil, fmt.Errorf("failed to query buffer size: %w", err)
	}

	bufFrames, err := selectDSDBufferFrames(bInfo, requestedBufferFrames)
	if err != nil {
		return nil, err
	}

	if err := driver.CreateBuffers(channels, bufFrames); err != nil {
		return nil, fmt.Errorf("failed to create ASIO DSD buffers: %w", err)
	}

	bufBytes := bufFrames / 8
	stream := &NativeDSDStream{
		driver:       driver,
		format:       format,
		channels:     channels,
		sampleTypes:  sampleTypes,
		bufferFrames: bufFrames,
		bufferBytes:  bufBytes,
		planar:       make([][]byte, channels),
		stagingBuf:   make([]byte, bufBytes*channels),
	}
	for ch := range stream.planar {
		stream.planar[ch] = make([]byte, bufBytes)
	}
	return stream, nil
}

func selectDSDBufferFrames(info bridge.BufferSizeInfo, requested int) (int, error) {
	frames := requested
	if frames <= 0 {
		frames = info.PreferredSize
	}
	if frames <= 0 {
		frames = 1024
	}
	if info.MinSize > 0 && frames < info.MinSize {
		frames = info.MinSize
	}
	if info.MaxSize > 0 && frames > info.MaxSize {
		frames = info.MaxSize
	}
	frames = frames / 8 * 8 // ASIO DSD uses one byte for eight 1-bit frames.
	if frames < 64 {
		return 0, fmt.Errorf("%w: %d DSD frames is below the 64-frame byte-aligned minimum", ErrInvalidDSDBufferSize, frames)
	}
	return frames, nil
}

func (s *NativeDSDStream) Format() dsd.DSDFormat {
	return s.format
}

func (s *NativeDSDStream) SampleType() bridge.SampleType {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sampleTypes) == 0 {
		return 0
	}
	return s.sampleTypes[0]
}

func (s *NativeDSDStream) SampleTypes() []bridge.SampleType {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]bridge.SampleType, len(s.sampleTypes))
	copy(out, s.sampleTypes)
	return out
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
	if err := s.driver.Stop(); err != nil {
		return err
	}
	s.running = false
	return nil
}

// GetBuffer returns a pointer to the interleaved staging area. Native DSD
// staging layout is planar-concatenated: channel 0 bytes, then channel 1 bytes.
func (s *NativeDSDStream) GetBuffer(numFrames int) (uintptr, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, errors.New("stream closed")
	}
	if numFrames <= 0 || numFrames > s.bufferFrames {
		return 0, fmt.Errorf("%w: requested %d, negotiated %d", ErrInvalidDSDBufferSize, numFrames, s.bufferFrames)
	}
	requiredBytes := ((numFrames + 7) / 8) * s.channels
	if len(s.stagingBuf) < requiredBytes {
		s.stagingBuf = make([]byte, requiredBytes)
	}
	return uintptr(unsafe.Pointer(&s.stagingBuf[0])), nil
}

// ReleaseBuffer converts the staging bitstream to each driver channel's
// reported ASIO DSD byte order and commits it directly to planar ASIO buffers.
func (s *NativeDSDStream) ReleaseBuffer(numFramesWritten int, flags uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("stream closed")
	}
	if numFramesWritten < 0 || numFramesWritten > s.bufferFrames {
		return fmt.Errorf("%w: released %d, negotiated %d", ErrInvalidDSDBufferSize, numFramesWritten, s.bufferFrames)
	}

	bytesPerChannel := (numFramesWritten + 7) / 8
	if bytesPerChannel == 0 {
		return nil
	}
	if bytesPerChannel*s.channels > len(s.stagingBuf) {
		return fmt.Errorf("%w: staging buffer has %d bytes, need %d", ErrInvalidDSDBufferSize, len(s.stagingBuf), bytesPerChannel*s.channels)
	}

	bufIdx := s.driver.GetActiveBufferIndex()
	for ch := 0; ch < s.channels; ch++ {
		src := s.stagingBuf[ch*bytesPerChannel : (ch+1)*bytesPerChannel]
		dst := s.planar[ch][:bytesPerChannel]
		if flags&0x01 != 0 {
			for i := range dst {
				dst[i] = 0x69 // deterministic DSD idle/silence byte
			}
		} else if err := ConvertPlanarDSD(dst, src, s.sampleTypes[ch], dsd.BitOrderMSBFirst); err != nil {
			return err
		}
		s.driver.WritePlanarDSD(bufIdx, ch, dst)
	}
	return nil
}

func (s *NativeDSDStream) GetBufferSize() (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return uint32(s.bufferFrames), nil
}

// BufferSizeBytes returns the whole-byte DSD buffer size reported by GetBuffer.
func (s *NativeDSDStream) BufferSizeBytes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bufferBytes
}

func (s *NativeDSDStream) GetDevicePeriod() (defaultPeriod, minPeriod time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sec := float64(s.bufferFrames) / float64(s.format.BitRate())
	d := time.Duration(sec * float64(time.Second))
	return d, d, nil
}

func (s *NativeDSDStream) GetWaitHandle() uintptr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.driver.GetWaitHandle()
}

func (s *NativeDSDStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	var errs []error
	if s.running {
		if err := s.driver.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := s.driver.DisposeBuffers(); err != nil {
		errs = append(errs, err)
	}
	s.closed = true
	s.running = false
	return errors.Join(errs...)
}
