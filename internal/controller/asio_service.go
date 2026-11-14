package controller

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio"
	"sound-signal-generator/internal/audio/asio/bridge"
	"sound-signal-generator/internal/dsd"
)

var (
	// ErrNativeDSDNotConfirmed is returned when Native DSD is requested without explicit safety confirmation.
	ErrNativeDSDNotConfirmed = errors.New("native dsd risk not confirmed: playback blocked by safety guard")
	// ErrInvalidConfirmationPhrase is returned when the user confirmation phrase does not match.
	ErrInvalidConfirmationPhrase = errors.New("invalid confirmation phrase: must enter 'YES'")
)

// AsioStatus reports the active ASIO driver status, configuration and diagnostics.
type AsioStatus struct {
	ActiveDriver        string                         `json:"active_driver"`
	DriverVersion       string                         `json:"driver_version"`
	SampleType          string                         `json:"sample_type"`
	ClockSources        []bridge.ClockSource           `json:"clock_sources"`
	ActiveClockIndex    int                            `json:"active_clock_index"`
	BufferSizes         bridge.BufferSizeInfo          `json:"buffer_sizes"`
	ActiveBufferSize    int                            `json:"active_buffer_size"`
	LatencyReport       *asio.LatencyMeasurementResult `json:"latency_report"`
	NativeDSDMode       bool                           `json:"native_dsd_mode"`
	NativeDSDConfirmed  bool                           `json:"native_dsd_confirmed"`
	SelectedDSDFormat   string                         `json:"selected_dsd_format"`
}

// AsioService manages ASIO operations and controller-level security guards.
type AsioService struct {
	mu                 sync.Mutex
	backend            *asio.Backend
	activeDevice       *asio.Device
	driver             bridge.Driver
	activeDriverName   string
	activeBufferSize   int
	nativeDSDConfirmed bool
	nativeDSDMode      bool
	selectedDSDFormat  dsd.DSDFormat
}

// NewAsioService creates an AsioService with the provided ASIO backend.
func NewAsioService(backend *asio.Backend) *AsioService {
	if backend == nil {
		backend = asio.NewMockBackend()
	}
	return &AsioService{
		backend:           backend,
		selectedDSDFormat: dsd.DSD256,
	}
}

// SelectDriver activates the requested ASIO driver.
func (s *AsioService) SelectDriver(driverID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeDevice != nil {
		_ = s.activeDevice.Close()
		s.activeDevice = nil
		s.driver = nil
	}

	dev, err := s.backend.GetDevice(driverID)
	if err != nil {
		return fmt.Errorf("failed to get asio driver: %w", err)
	}

	asioDev, ok := dev.(*asio.Device)
	if !ok {
		return errors.New("device is not an ASIO device")
	}

	s.activeDevice = asioDev
	s.driver = asioDev.Driver()
	s.activeDriverName = driverID
	if s.driver != nil {
		_ = s.driver.Init(0)
		bInfo, err := s.driver.GetBufferSize()
		if err == nil {
			s.activeBufferSize = bInfo.PreferredSize
		}
	}

	return nil
}

// GetStatus returns the operational status and latency report for the active ASIO driver.
func (s *AsioService) GetStatus() (AsioStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := AsioStatus{
		ActiveDriver:       s.activeDriverName,
		NativeDSDMode:      s.nativeDSDMode,
		NativeDSDConfirmed: s.nativeDSDConfirmed,
		SelectedDSDFormat:  s.selectedDSDFormat.String(),
		ActiveBufferSize:   s.activeBufferSize,
	}

	if s.driver == nil {
		return status, nil
	}

	status.DriverVersion = fmt.Sprintf("0x%04X", s.driver.GetDriverVersion())
	clocks, _ := s.driver.GetClockSources()
	status.ClockSources = clocks
	for i, c := range clocks {
		if c.IsCurrentSource {
			status.ActiveClockIndex = i
			break
		}
	}

	bInfo, _ := s.driver.GetBufferSize()
	status.BufferSizes = bInfo

	chInfo, _ := s.driver.GetChannelInfo(0, false)
	status.SampleType = chInfo.SampleType.String()

	sRate, _ := s.driver.GetSampleRate()
	if sRate == 0 {
		sRate = 48000
	}
	lat, _ := asio.MeasureLoopbackLatency(s.driver, s.activeBufferSize, sRate, 50)
	status.LatencyReport = lat

	return status, nil
}

// SetClockSource switches hardware clock source.
func (s *AsioService) SetClockSource(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.driver == nil {
		return errors.New("no active ASIO driver")
	}
	return s.driver.SetClockSource(index)
}

// SetBufferSize configures ASIO buffer size in frames.
func (s *AsioService) SetBufferSize(frames int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.driver == nil {
		return errors.New("no active ASIO driver")
	}
	bInfo, err := s.driver.GetBufferSize()
	if err != nil {
		return err
	}
	if frames < bInfo.MinSize || (bInfo.MaxSize > 0 && frames > bInfo.MaxSize) {
		return fmt.Errorf("buffer size %d out of bounds [%d, %d]", frames, bInfo.MinSize, bInfo.MaxSize)
	}
	s.activeBufferSize = frames
	return nil
}

// ConfirmNativeDSDRisk satisfies the mandatory safety guard.
func (s *AsioService) ConfirmNativeDSDRisk(confirmationPhrase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	trimmed := strings.TrimSpace(strings.ToUpper(confirmationPhrase))
	if trimmed != "YES" {
		return ErrInvalidConfirmationPhrase
	}

	s.nativeDSDConfirmed = true
	return nil
}

// ConfigureNativeDSD configures Native DSD mode with strict controller-level security check.
func (s *AsioService) ConfigureNativeDSD(formatStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.nativeDSDConfirmed {
		return ErrNativeDSDNotConfirmed
	}

	var targetFmt dsd.DSDFormat
	switch strings.ToUpper(formatStr) {
	case "DSD256":
		targetFmt = dsd.DSD256
	case "DSD512":
		targetFmt = dsd.DSD512
	case "DSD64":
		targetFmt = dsd.DSD64
	case "DSD128":
		targetFmt = dsd.DSD128
	default:
		return fmt.Errorf("unrecognized DSD format: %s", formatStr)
	}

	s.selectedDSDFormat = targetFmt
	s.nativeDSDMode = true
	return nil
}

// DisableNativeDSD reverts to standard PCM mode.
func (s *AsioService) DisableNativeDSD() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nativeDSDMode = false
}

// ActiveDevice returns the active ASIO device if selected.
func (s *AsioService) ActiveDevice() audio.Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeDevice
}
