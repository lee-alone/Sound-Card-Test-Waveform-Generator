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
	// ErrNativeDSDRequiresASIO is returned when Native DSD is requested without an active ASIO device.
	ErrNativeDSDRequiresASIO = errors.New("native dsd requires an active ASIO output device")
	// ErrNativeDSDFormatUnsupported is returned when the driver cannot expose the requested native DSD rate/sample type.
	ErrNativeDSDFormatUnsupported = errors.New("native dsd format is not supported by the active ASIO driver")
)

// NativeDSDCapability describes one native DSD rate probed on the active driver.
type NativeDSDCapability struct {
	Format     string `json:"format"`
	BitRateHz  int    `json:"bit_rate_hz"`
	Supported  bool   `json:"supported"`
	SampleType string `json:"sample_type,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// AsioStatus reports the active ASIO driver status, configuration and diagnostics.
type AsioStatus struct {
	ActiveDriver          string                         `json:"active_driver"`
	DriverVersion         string                         `json:"driver_version"`
	SampleType            string                         `json:"sample_type"`
	OperatingMode         string                         `json:"operating_mode"`
	ClockSources          []bridge.ClockSource           `json:"clock_sources"`
	ActiveClockIndex      int                            `json:"active_clock_index"`
	BufferSizes           bridge.BufferSizeInfo          `json:"buffer_sizes"`
	ActiveBufferSize      int                            `json:"active_buffer_size"`
	LatencyReport         *asio.LatencyMeasurementResult `json:"latency_report"`
	NativeDSDMode         bool                           `json:"native_dsd_mode"`
	NativeDSDActive       bool                           `json:"native_dsd_active"`
	NativeDSDConfirmed    bool                           `json:"native_dsd_confirmed"`
	SelectedDSDFormat     string                         `json:"selected_dsd_format"`
	NativeDSDBufferFrames int                            `json:"native_dsd_buffer_frames"`
	NativeCapabilities    []NativeDSDCapability          `json:"native_dsd_capabilities"`
}

// AsioService manages ASIO operations, Native DSD stream creation and controller-level security guards.
type AsioService struct {
	mu                 sync.Mutex
	backend            *asio.Backend
	activeDevice       *asio.Device
	driver             bridge.Driver
	activeDriverName   string
	activeBufferSize   int
	pcmSampleRate      float64
	nativeDSDConfirmed bool
	nativeDSDMode      bool
	nativeDSDFormatSet bool
	selectedDSDFormat  dsd.DSDFormat
	nativeStream       *asio.NativeDSDStream
	nativeCapabilities []NativeDSDCapability
}

// NewAsioService creates an AsioService with the provided ASIO backend. A nil
// backend selects the production ASIO backend; tests must inject a mock.
func NewAsioService(backend *asio.Backend) *AsioService {
	if backend == nil {
		backend = asio.NewBackend()
	}
	return &AsioService{backend: backend}
}

// SelectDriver activates the requested ASIO driver by driver ID.
func (s *AsioService) SelectDriver(driverID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backend == nil {
		return errors.New("ASIO backend is not configured")
	}
	dev, err := s.backend.GetDevice(driverID)
	if err != nil {
		return fmt.Errorf("failed to get asio driver: %w", err)
	}
	asioDev, ok := dev.(*asio.Device)
	if !ok {
		return errors.New("device is not an ASIO device")
	}
	return s.selectDeviceLocked(asioDev)
}

// SelectDevice activates an already-resolved ASIO device from the unified
// output-device selector. The controller uses this path so driver selection is
// not performed twice and cannot silently fall back to a mock.
func (s *AsioService) SelectDevice(dev *asio.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if dev == nil {
		return errors.New("ASIO device is nil")
	}
	return s.selectDeviceLocked(dev)
}

func (s *AsioService) selectDeviceLocked(dev *asio.Device) error {
	s.releaseNativeLocked(true)
	if s.activeDevice != nil && s.activeDevice != dev {
		_ = s.activeDevice.Close()
	}

	s.activeDevice = dev
	s.driver = dev.Driver()
	s.activeDriverName = dev.Name()
	s.nativeDSDMode = false
	s.nativeDSDConfirmed = false
	s.nativeDSDFormatSet = false
	s.selectedDSDFormat = 0
	s.nativeStream = nil
	s.activeBufferSize = 0
	s.pcmSampleRate = 48000
	s.nativeCapabilities = nil

	if s.driver == nil {
		s.activeDevice = nil
		return errors.New("ASIO device has no driver")
	}
	if err := s.driver.Init(0); err != nil {
		s.activeDevice = nil
		s.driver = nil
		return fmt.Errorf("ASIOInit failed: %w", err)
	}
	if rate, err := s.driver.GetSampleRate(); err == nil && rate > 0 {
		// A driver may have been left at a Native DSD rate by another process.
		// Never adopt that rate as the PCM baseline when selecting it.
		if rate >= float64(dsd.DSD64.BitRate()) && s.driver.CanSampleRate(48000) {
			if setErr := s.driver.SetSampleRate(48000); setErr == nil {
				rate = 48000
			}
		}
		s.pcmSampleRate = rate
	}
	if bInfo, err := s.driver.GetBufferSize(); err == nil {
		s.activeBufferSize = bInfo.PreferredSize
	}
	return nil
}

// GetStatus returns the operational status and latency report for the active ASIO driver.
func (s *AsioService) GetStatus() (AsioStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	selectedFormat := ""
	if s.nativeDSDFormatSet {
		selectedFormat = s.selectedDSDFormat.String()
	}
	status := AsioStatus{
		ActiveDriver:       s.activeDriverName,
		OperatingMode:      "PCM",
		NativeDSDMode:      s.nativeDSDMode,
		NativeDSDActive:    s.nativeDSDMode,
		NativeDSDConfirmed: s.nativeDSDConfirmed,
		SelectedDSDFormat:  selectedFormat,
		ActiveBufferSize:   s.activeBufferSize,
		NativeCapabilities: append([]NativeDSDCapability(nil), s.nativeCapabilities...),
	}
	if s.nativeStream != nil {
		status.NativeDSDBufferFrames = int(mustBufferSize(s.nativeStream))
		status.SampleType = s.nativeStream.SampleType().String()
	}
	if s.driver == nil {
		status.OperatingMode = "Unavailable"
		return status, nil
	}
	if s.nativeDSDMode {
		status.OperatingMode = "Native DSD"
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
	if status.SampleType == "" {
		if chInfo, err := s.driver.GetChannelInfo(0, false); err == nil {
			status.SampleType = chInfo.SampleType.String()
		}
	}

	sRate, _ := s.driver.GetSampleRate()
	if sRate == 0 {
		sRate = s.pcmSampleRate
	}
	if s.activeBufferSize > 0 {
		lat, _ := asio.MeasureLoopbackLatency(s.driver, s.activeBufferSize, sRate, 50)
		status.LatencyReport = lat
	}

	return status, nil
}

func mustBufferSize(stream *asio.NativeDSDStream) uint32 {
	if stream == nil {
		return 0
	}
	frames, err := stream.GetBufferSize()
	if err != nil {
		return 0
	}
	return frames
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

// SetBufferSize configures ASIO buffer size in frames for the next stream open.
func (s *AsioService) SetBufferSize(frames int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.driver == nil {
		return errors.New("no active ASIO driver")
	}
	if s.nativeDSDMode {
		return errors.New("disable Native DSD before changing the ASIO buffer size")
	}
	bInfo, err := s.driver.GetBufferSize()
	if err != nil {
		return err
	}
	if frames < bInfo.MinSize || (bInfo.MaxSize > 0 && frames > bInfo.MaxSize) {
		return fmt.Errorf("buffer size %d out of bounds [%d, %d]", frames, bInfo.MinSize, bInfo.MaxSize)
	}
	if !validBufferSize(frames, bInfo) {
		return fmt.Errorf("buffer size %d violates driver granularity %d", frames, bInfo.Granularity)
	}
	s.activeBufferSize = frames
	return nil
}

func validBufferSize(frames int, info bridge.BufferSizeInfo) bool {
	if info.Granularity == 0 {
		return true
	}
	if info.Granularity == -1 {
		return frames > 0 && frames&(frames-1) == 0
	}
	if info.Granularity > 0 {
		return (frames-info.MinSize)%info.Granularity == 0
	}
	return true
}

// BufferSize returns the buffer size that will be used by the next ASIO stream.
func (s *AsioService) BufferSize() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeBufferSize
}

// ConfirmNativeDSDRisk satisfies the mandatory Native DSD safety guard.
func (s *AsioService) ConfirmNativeDSDRisk(confirmationPhrase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if confirmationPhrase != "YES" {
		return ErrInvalidConfirmationPhrase
	}
	s.nativeDSDConfirmed = true
	return nil
}

// ConfigureNativeDSD validates driver capability, creates the real
// NativeDSDStream and only then reports Native DSD mode as enabled.
func (s *AsioService) ConfigureNativeDSD(formatStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.nativeDSDConfirmed {
		return ErrNativeDSDNotConfirmed
	}
	if s.driver == nil || s.activeDevice == nil {
		s.nativeDSDMode = false
		return ErrNativeDSDRequiresASIO
	}

	// Any new selection invalidates the previous native mode. If the new rate
	// cannot be validated, the service remains in PCM mode instead of leaving a
	// stale "enabled" state visible to the operator.
	_ = s.disableNativeLocked(true)
	s.nativeDSDFormatSet = false
	s.selectedDSDFormat = 0

	targetFmt, err := parseNativeDSDFormat(formatStr)
	if err != nil {
		s.failNativeEntryLocked()
		return err
	}

	if err := s.probeNativeCapabilitiesLocked(); err != nil {
		s.failNativeEntryLocked()
		return fmt.Errorf("native DSD capability probe failed: %w", err)
	}
	capability, ok := s.capabilityForLocked(targetFmt)
	if !ok || !capability.Supported {
		s.failNativeEntryLocked()
		reason := "rate not advertised by driver"
		if ok && capability.Reason != "" {
			reason = capability.Reason
		}
		return fmt.Errorf("%w: %s (%s)", ErrNativeDSDFormatUnsupported, targetFmt.String(), reason)
	}

	stream, err := asio.NewNativeDSDStreamWithBufferSize(s.driver, targetFmt, 2, s.activeBufferSize)
	if err != nil {
		s.failNativeEntryLocked()
		s.nativeStream = nil
		_ = s.restorePCMRateLocked()
		return fmt.Errorf("failed to create Native DSD stream for %s: %w", targetFmt.String(), err)
	}

	s.nativeStream = stream
	s.selectedDSDFormat = targetFmt
	s.nativeDSDFormatSet = true
	s.nativeDSDMode = true
	return nil
}

// NativeDSDConfirmed reports whether the exact YES safety phrase was accepted.
func (s *AsioService) NativeDSDConfirmed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nativeDSDConfirmed
}

// NativeDSDMode reports whether a native stream has been successfully created.
func (s *AsioService) NativeDSDMode() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nativeDSDMode
}

// NativeStream returns the configured native stream. Ownership remains with
// AsioService; AudioEngine must be configured with it as an external stream.
func (s *AsioService) NativeStream() *asio.NativeDSDStream {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nativeStream
}

// DisableNativeDSD closes the native buffers and restores the prior PCM clock.
func (s *AsioService) DisableNativeDSD() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.disableNativeLocked(true)
	s.resetNativeSessionLocked()
	return err
}

func (s *AsioService) disableNativeLocked(restoreRate bool) error {
	var errs []error
	if s.nativeStream != nil {
		if err := s.nativeStream.Close(); err != nil {
			errs = append(errs, err)
		}
		s.nativeStream = nil
	}
	s.nativeDSDMode = false
	if restoreRate {
		if err := s.restorePCMRateLocked(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *AsioService) releaseNativeLocked(restoreRate bool) {
	_ = s.disableNativeLocked(restoreRate)
	s.resetNativeSessionLocked()
}

func (s *AsioService) resetNativeSessionLocked() {
	s.nativeDSDConfirmed = false
	s.nativeDSDFormatSet = false
	s.selectedDSDFormat = 0
}

func (s *AsioService) failNativeEntryLocked() {
	s.nativeDSDMode = false
	s.nativeStream = nil
	s.resetNativeSessionLocked()
}

// CancelNativeDSDEntry revokes an unused safety confirmation. It is used when
// the operator closes the Native DSD entry flow without activating the stream.
func (s *AsioService) CancelNativeDSDEntry() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nativeDSDMode {
		return errors.New("cannot cancel Native DSD entry while Native DSD is active")
	}
	s.resetNativeSessionLocked()
	return nil
}

func (s *AsioService) restorePCMRateLocked() error {
	if s.driver == nil || s.pcmSampleRate <= 0 {
		return nil
	}
	return s.driver.SetSampleRate(s.pcmSampleRate)
}

// ClearDevice detaches from an ASIO device without closing the driver. The
// controller closes the owning asio.Device after this method.
func (s *AsioService) ClearDevice() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.disableNativeLocked(false)
	s.resetNativeSessionLocked()
	s.activeDevice = nil
	s.driver = nil
	s.activeDriverName = ""
	s.activeBufferSize = 0
	s.nativeCapabilities = nil
}

// Close relinquishes native buffers and the active driver handle.
func (s *AsioService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []error
	if err := s.disableNativeLocked(false); err != nil {
		errs = append(errs, err)
	}
	s.resetNativeSessionLocked()
	if s.activeDevice != nil {
		if err := s.activeDevice.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	s.activeDevice = nil
	s.driver = nil
	s.activeDriverName = ""
	s.activeBufferSize = 0
	s.nativeCapabilities = nil
	return errors.Join(errs...)
}

// ActiveDevice returns the active ASIO device if selected.
func (s *AsioService) ActiveDevice() audio.Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeDevice
}

func parseNativeDSDFormat(formatStr string) (dsd.DSDFormat, error) {
	switch strings.ToUpper(strings.TrimSpace(formatStr)) {
	case "DSD64", "64":
		return dsd.DSD64, nil
	case "DSD128", "128":
		return dsd.DSD128, nil
	case "DSD256", "256":
		return dsd.DSD256, nil
	case "DSD512", "512":
		return dsd.DSD512, nil
	default:
		return 0, fmt.Errorf("unrecognized DSD format: %s", formatStr)
	}
}

func (s *AsioService) probeNativeCapabilitiesLocked() error {
	if s.driver == nil {
		return errors.New("no active ASIO driver")
	}
	_, outputs, err := s.driver.GetChannels()
	if err != nil {
		return err
	}
	currentRate, rateErr := s.driver.GetSampleRate()
	if rateErr != nil || currentRate <= 0 {
		currentRate = s.pcmSampleRate
	}
	if currentRate <= 0 {
		currentRate = 48000
	}
	formats := []dsd.DSDFormat{dsd.DSD64, dsd.DSD128, dsd.DSD256, dsd.DSD512}
	caps := make([]NativeDSDCapability, 0, len(formats))
	for _, format := range formats {
		capability := NativeDSDCapability{
			Format:    format.String(),
			BitRateHz: format.BitRate(),
		}
		rate := float64(format.BitRate())
		if outputs <= 0 {
			capability.Reason = "driver exposes no output channels"
		} else if !s.driver.CanSampleRate(rate) {
			capability.Reason = "driver does not advertise this ASIO sample rate"
		} else if err := s.driver.SetSampleRate(rate); err != nil {
			capability.Reason = fmt.Sprintf("SetSampleRate failed: %v", err)
		} else {
			info, infoErr := s.driver.GetChannelInfo(0, false)
			switch {
			case infoErr != nil:
				capability.Reason = fmt.Sprintf("GetChannelInfo failed: %v", infoErr)
			case !info.SampleType.IsDSD():
				capability.Reason = fmt.Sprintf("channel reports %s instead of an ASIO DSD sample type", info.SampleType.String())
			default:
				capability.Supported = true
				capability.SampleType = info.SampleType.String()
			}
		}
		caps = append(caps, capability)
	}
	s.nativeCapabilities = caps
	if err := s.driver.SetSampleRate(currentRate); err != nil {
		return fmt.Errorf("restore PCM sample rate %.0f: %w", currentRate, err)
	}
	return nil
}

func (s *AsioService) capabilityForLocked(format dsd.DSDFormat) (NativeDSDCapability, bool) {
	for _, capability := range s.nativeCapabilities {
		if capability.BitRateHz == format.BitRate() {
			return capability, true
		}
	}
	return NativeDSDCapability{}, false
}
