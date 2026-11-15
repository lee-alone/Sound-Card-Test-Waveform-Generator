package controller

import (
	"errors"
	"fmt"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio"
	"sound-signal-generator/internal/engine"
)

// ErrOutputFormatLocked prevents the physical stream format from changing
// underneath an active render loop. DSP-only parameters remain live-editable.
var ErrOutputFormatLocked = errors.New("output sample rate and sample format are locked while playback is running; stop output before changing them")

// ConfigureExtendedSignal validates parameters, enforces safety checks, and
// configures the engine and generator. DSP-only changes are applied to an
// active stream in place; format-changing paths resume automatically only when
// playback was already running, preserving the operator's intent.
func (c *AppController) ConfigureExtendedSignal(req ExtendedSignalRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	wasRunning := c.engine.State() == engine.StateRunning

	// 1. Check safety confirmation
	if err := c.safetyManager.ValidateConfirmation(req); err != nil {
		return err
	}

	if c.selectedDevice == nil {
		if err := c.selectDeviceLocked(req.DeviceID); err != nil {
			return ErrDeviceNotSelected
		}
	} else if req.DeviceID != "" && req.DeviceID != c.selectedDeviceKey && req.DeviceID != c.selectedDevice.ID() {
		if err := c.selectDeviceLocked(req.DeviceID); err != nil {
			return err
		}
	}

	// Leaving Native DSD always closes the direct 1-bit stream and restores the
	// prior PCM clock before any PCM/DoP stream is opened.
	if c.nativeDSDActive || c.nativeDSDFormat != 0 || c.asioService.NativeDSDMode() {
		if c.engine.State() != engine.StateClosed {
			_ = c.engine.Close()
		}
		if err := c.deactivateNativeDSDLocked(); err != nil {
			return fmt.Errorf("failed to restore PCM mode: %w", err)
		}
	}

	// 2. Handle DoP signal path
	if req.Waveform == WaveformDoP {
		req = normalizeDoPRequest(req)
		c.mu.Unlock()
		dopReq := DoPRequest{
			DeviceID:     req.DeviceID,
			DSDFormat:    req.DoP.DSDFormat,
			SampleFormat: req.SampleFormat,
			Channels:     req.Channels,
			BitOrder:     req.DoP.BitOrder,
		}
		// Confirm DoP guard if safety manager has confirmed RiskDoP
		if c.safetyManager.IsConfirmed(RiskDoP) {
			c.dopGuard.ConfirmViaFlag(true)
		}
		err := c.ConfigureDoP(dopReq)
		c.mu.Lock()
		if err == nil {
			c.extendedRequest = req
			if wasRunning {
				if resumeErr := c.engine.Start(); resumeErr != nil {
					err = fmt.Errorf("resume DoP playback after reconfiguration: %w", resumeErr)
				}
			}
		}
		return err
	}

	// A PCM waveform or frequency change does not require touching the audio
	// device. Swap the prepared DSP generator atomically so the active stream
	// continues without a silent Primed state or a manual stop/start.
	if c.engine.State() == engine.StateRunning && c.extendedRequest.Waveform != WaveformDoP {
		if c.currentRequest.SampleRate != req.SampleRate ||
			c.currentRequest.SampleFormat != req.SampleFormat ||
			c.currentRequest.Channels != req.Channels {
			return ErrOutputFormatLocked
		}
		return c.replaceRunningPCMGeneratorLocked(req)
	}

	// 3. Create base DSP generator
	baseGen, err := c.generatorFactory.CreateGenerator(req)
	if err != nil {
		return fmt.Errorf("failed to create waveform generator: %w", err)
	}

	// 4. Wrap with stereo channel gain scaler
	scaledGen := NewChannelScaleGenerator(baseGen, req.LeftVolumeDBFS, req.RightVolumeDBFS)
	c.activeGenerator = scaledGen

	// 5. Close engine if already open
	if c.engine.State() != engine.StateClosed {
		_ = c.engine.Close()
	}

	targetFormat := audio.AudioFormat{
		SampleRate:   req.SampleRate,
		Channels:     req.Channels,
		SampleFormat: req.SampleFormat,
	}

	shareMode := audio.ShareModeExclusive
	if !c.isExclusive {
		shareMode = audio.ShareModeShared
	}

	engCfg := engine.EngineConfig{
		Device:         c.selectedDevice,
		Format:         targetFormat,
		ShareMode:      shareMode,
		BufferDuration: c.bufferDuration,
		FadeDurationMs: 10.0,
	}
	if _, ok := c.selectedDevice.(*asio.Device); ok {
		engCfg.BufferFrames = c.asioService.BufferSize()
	}

	if err := c.engine.Open(engCfg); err != nil {
		return fmt.Errorf("engine open failed: %w", err)
	}

	if err := c.engine.Prime(scaledGen); err != nil {
		return fmt.Errorf("engine prime failed: %w", err)
	}

	normalizedPCM := req
	c.lastPCMRequest = &normalizedPCM
	c.extendedRequest = req
	c.currentRequest = SignalRequest{
		DeviceID:     req.DeviceID,
		SampleRate:   req.SampleRate,
		SampleFormat: req.SampleFormat,
		Channels:     req.Channels,
		Frequency:    req.Frequency,
		Amplitude:    req.Amplitude,
		DCOffset:     req.DCOffset,
	}

	// Format-changing transitions (DoP <-> PCM, device changes, native DSD
	// exit) necessarily rebuild the stream. Preserve the operator's intent by
	// resuming automatically instead of leaving the new stream silently primed.
	if wasRunning {
		if err := c.engine.Start(); err != nil {
			return fmt.Errorf("resume playback after reconfiguration: %w", err)
		}
	}

	return nil
}

// replaceRunningPCMGeneratorLocked rebuilds only the DSP source and channel
// trim wrapper. The device stream, clock, buffer and negotiated format remain
// untouched, so waveform, frequency and sweep edits are audible immediately.
func (c *AppController) replaceRunningPCMGeneratorLocked(req ExtendedSignalRequest) error {
	baseGen, err := c.generatorFactory.CreateGenerator(req)
	if err != nil {
		return fmt.Errorf("failed to create waveform generator: %w", err)
	}
	scaledGen := NewChannelScaleGenerator(baseGen, req.LeftVolumeDBFS, req.RightVolumeDBFS)
	if err := c.engine.ReplaceGenerator(scaledGen); err != nil {
		return fmt.Errorf("replace running waveform generator: %w", err)
	}

	c.activeGenerator = scaledGen
	normalizedPCM := req
	c.lastPCMRequest = &normalizedPCM
	c.extendedRequest = req
	c.currentRequest = SignalRequest{
		DeviceID:     req.DeviceID,
		SampleRate:   req.SampleRate,
		SampleFormat: req.SampleFormat,
		Channels:     req.Channels,
		Frequency:    req.Frequency,
		Amplitude:    req.Amplitude,
		DCOffset:     req.DCOffset,
	}
	return nil
}

// Get10HzDiagnostics builds and returns a complete 10Hz diagnostics report.
func (c *AppController) Get10HzDiagnostics() DiagnosticsReport {
	c.mu.Lock()
	defer c.mu.Unlock()

	status := c.statusLocked()
	isDoP := (c.extendedRequest.Waveform == WaveformDoP)
	isDirectDSD := isDoP || c.nativeDSDActive

	reqFormat := audio.AudioFormat{
		SampleRate:   c.extendedRequest.SampleRate,
		Channels:     c.extendedRequest.Channels,
		SampleFormat: c.extendedRequest.SampleFormat,
	}
	leftVolume := c.extendedRequest.LeftVolumeDBFS
	rightVolume := c.extendedRequest.RightVolumeDBFS
	if c.nativeDSDActive {
		reqFormat = status.ActiveFormat
		leftVolume = 0
		rightVolume = 0
	}
	actualFormat := status.ActiveFormat

	bitTier := c.bitPerfect.Evaluate(
		status.State,
		c.isExclusive,
		reqFormat,
		actualFormat,
		leftVolume,
		rightVolume,
		isDirectDSD,
	)

	return c.diagnosticsService.BuildReport(status, bitTier, c.bufferDuration, c.isExclusive)
}

// statusLocked returns ControllerStatus while mutex is already held.
func (c *AppController) statusLocked() ControllerStatus {
	devName := "None"
	if c.selectedDevice != nil {
		devName = c.selectedDevice.Name()
	}

	return ControllerStatus{
		State:      c.engine.State(),
		DeviceName: devName,
		ActiveFormat: audio.AudioFormat{
			SampleRate:   c.currentRequest.SampleRate,
			Channels:     c.currentRequest.Channels,
			SampleFormat: c.currentRequest.SampleFormat,
		},
		Frequency:   c.currentRequest.Frequency,
		Amplitude:   c.currentRequest.Amplitude,
		Diagnostics: c.engine.Diagnostics(),
	}
}

// SetExclusiveMode toggles Exclusive mode vs Shared mode.
func (c *AppController) SetExclusiveMode(exclusive bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.isExclusive = exclusive
}

// IsExclusiveMode returns whether Exclusive mode is currently configured.
func (c *AppController) IsExclusiveMode() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isExclusive
}

// CheckSafety evaluates whether the given signal request presents hearing or hardware risks.
func (c *AppController) CheckSafety(req ExtendedSignalRequest) SafetyAssessment {
	return c.safetyManager.Assess(req)
}

// ConfirmSafety acknowledges a specific safety hazard category.
func (c *AppController) ConfirmSafety(category SafetyRiskCategory, confirmed bool) {
	c.safetyManager.Confirm(category, confirmed)
	if category == RiskDoP && confirmed {
		c.dopGuard.ConfirmViaFlag(true)
	}
}

// LoadSafetyPreferences restores permanent "do not ask again" choices.
func (c *AppController) LoadSafetyPreferences() error {
	return c.safetyManager.LoadSafetyPreferences()
}

// SuppressSafety permanently disables prompts for one risk category.
func (c *AppController) SuppressSafety(category SafetyRiskCategory) error {
	return c.safetyManager.Suppress(category)
}

// SafetyManager returns the safety manager.
func (c *AppController) SafetyManager() *SafetyManager {
	return c.safetyManager
}

// DeviceService returns the device service.
func (c *AppController) DeviceService() *DeviceService {
	return c.deviceService
}

// BitPerfectEvaluator returns the bit-perfect evaluator.
func (c *AppController) BitPerfectEvaluator() *BitPerfectEvaluator {
	return c.bitPerfect
}

// ExtendedRequest returns the currently configured extended signal request.
func (c *AppController) ExtendedRequest() ExtendedSignalRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.extendedRequest
}

// SetBufferDuration configures the engine buffer period duration (e.g. 10ms).
func (c *AppController) SetBufferDuration(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bufferDuration = d
}
