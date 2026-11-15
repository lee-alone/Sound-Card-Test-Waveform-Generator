package controller

import (
	"fmt"
	"sound-signal-generator/internal/audio"
)

// UIService provides the RPC boundary between Wails v3 frontend and AppController.
// Adheres strictly to the low-coupling principle: only UI-relevant methods are exposed.
type UIService struct {
	ctrl *AppController
}

// NewUIService creates a UI service wrapping the AppController.
func NewUIService(ctrl *AppController) *UIService {
	return &UIService{ctrl: ctrl}
}

// GetDevices returns all discovered audio render endpoints.
func (s *UIService) GetDevices() ([]audio.DeviceInfo, error) {
	return s.ctrl.DeviceService().Enumerate()
}

// SelectDevice sets active audio device by ID.
func (s *UIService) SelectDevice(id string) error {
	if err := s.ctrl.SelectDevice(id); err != nil {
		return err
	}
	s.ctrl.DeviceService().SetActiveDeviceID(id)
	return nil
}

// ProbeDeviceCapabilities tests candidate formats on current device in exclusive mode.
func (s *UIService) ProbeDeviceCapabilities() ([]audio.FormatSupportResult, error) {
	s.ctrl.mu.Lock()
	dev := s.ctrl.selectedDevice
	s.ctrl.mu.Unlock()

	if dev == nil {
		return nil, ErrDeviceNotSelected
	}

	candidates := audio.GenerateStandardCandidateMatrix(2)
	mode := audio.ShareModeExclusive
	if !s.ctrl.IsExclusiveMode() {
		mode = audio.ShareModeShared
	}
	return s.ctrl.ProbeSelectedDeviceCapabilities(candidates, mode)
}

// SetExclusiveMode toggles Exclusive mode.
func (s *UIService) SetExclusiveMode(exclusive bool) {
	s.ctrl.SetExclusiveMode(exclusive)
}

// IsExclusiveMode returns exclusive mode state.
func (s *UIService) IsExclusiveMode() bool {
	return s.ctrl.IsExclusiveMode()
}

// ConfigureSignal validates parameters, runs safety assessment, and configures the engine.
func (s *UIService) ConfigureSignal(req ExtendedSignalRequest) error {
	return s.ctrl.ConfigureExtendedSignal(req)
}

// StartPlayback begins audio streaming.
func (s *UIService) StartPlayback() error {
	return s.ctrl.StartPlayback()
}

// StopPlayback gracefully fades out and stops audio.
func (s *UIService) StopPlayback() error {
	return s.ctrl.StopPlayback()
}

// GetDiagnostics returns real-time 10Hz performance & bit-perfect diagnostics.
func (s *UIService) GetDiagnostics() DiagnosticsReport {
	return s.ctrl.Get10HzDiagnostics()
}

// CheckSafety returns hazard assessments for the provided request.
func (s *UIService) CheckSafety(req ExtendedSignalRequest) SafetyAssessment {
	return s.ctrl.CheckSafety(req)
}

// ConfirmSafety acknowledges a hazard category.
func (s *UIService) ConfirmSafety(category string, confirmed bool) {
	s.ctrl.ConfirmSafety(SafetyRiskCategory(category), confirmed)
}

// SavePreset persists current test configuration into a JSON preset file.
func (s *UIService) SavePreset(filePath string, name, desc string, req ExtendedSignalRequest) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	preset := NewSignalPreset(name, desc, req)
	return SavePresetToFile(filePath, preset)
}

// LoadPreset reads and parses a preset from file.
func (s *UIService) LoadPreset(filePath string) (*SignalPreset, error) {
	return LoadPresetFromFile(filePath)
}

// ApplyPreset loads and applies a preset to the controller, maintaining OpenMuted safe state.
func (s *UIService) ApplyPreset(preset *SignalPreset) error {
	return ApplyPresetSafely(s.ctrl, preset)
}

// PollDeviceChange checks if the active device was detached (< 500ms detection).
func (s *UIService) PollDeviceChange() *DeviceChangeEvent {
	return s.ctrl.DeviceService().CheckHotplug()
}

// GetAsioStatus returns the active ASIO driver status, clock sources, and latency stats.
func (s *UIService) GetAsioStatus() (AsioStatus, error) {
	return s.ctrl.AsioService().GetStatus()
}

// SelectAsioDriver activates the requested ASIO driver.
func (s *UIService) SelectAsioDriver(id string) error {
	return s.ctrl.SelectAsioDriver(id)
}

// SetAsioClockSource switches hardware clock source on the active ASIO driver.
func (s *UIService) SetAsioClockSource(index int) error {
	return s.ctrl.SetAsioClockSource(index)
}

// SetAsioBufferSize configures the ASIO buffer size in sample frames.
func (s *UIService) SetAsioBufferSize(frames int) error {
	return s.ctrl.SetAsioBufferSize(frames)
}

// ConfigureNativeDSD configures Native DSD mode with controller-level safety guard.
func (s *UIService) ConfigureNativeDSD(format string) error {
	return s.ctrl.EnableNativeDSD(format)
}

// ConfirmNativeDSDRisk satisfies the mandatory Native DSD safety guard.
func (s *UIService) ConfirmNativeDSDRisk(confirmation string) error {
	return s.ctrl.AsioService().ConfirmNativeDSDRisk(confirmation)
}

// CancelNativeDSDEntry revokes a pending Native DSD entry confirmation.
func (s *UIService) CancelNativeDSDEntry() error {
	return s.ctrl.CancelNativeDSDEntry()
}

// DisableNativeDSD reverts ASIO output to standard PCM mode.
func (s *UIService) DisableNativeDSD() error {
	return s.ctrl.DisableNativeDSD()
}
