package controller

import (
	"errors"
	"fmt"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio"
	"sound-signal-generator/internal/dsd"
	"sound-signal-generator/internal/engine"
)

var (
	// ErrUnsupportedDoPDSDFormat is returned when attempting DoP with unsupported DSD rates (e.g. DSD256/512 in Phase 3).
	ErrUnsupportedDoPDSDFormat = errors.New("DoP transmission is only supported for DSD64 (176.4kHz carrier) and DSD128 (352.8kHz carrier) in Phase 3")
)

// DoPRequest specifies high-level DoP test signal configuration parameters.
type DoPRequest struct {
	DeviceID     string             `json:"device_id"`
	DSDFormat    dsd.DSDFormat      `json:"dsd_format"`
	SampleFormat audio.SampleFormat `json:"sample_format"` // Always normalized to SampleFormat24In32
	Channels     int                `json:"channels"`      // default 2
	BitOrder     dsd.BitOrder       `json:"bit_order"`
}

// normalizeDoPRequest applies the transport invariants that must not be
// controlled by callers: carrier rate, 24-in-32 container, stereo unity gain
// and no DC offset.
func normalizeDoPRequest(req ExtendedSignalRequest) ExtendedSignalRequest {
	req.SampleRate = req.DoP.DSDFormat.DoPCarrierSampleRate()
	req.SampleFormat = audio.SampleFormat24In32
	req.Channels = 2
	req.Amplitude = 1.0
	req.DCOffset = 0.0
	req.LeftVolumeDBFS = 0.0
	req.RightVolumeDBFS = 0.0
	return req
}

// DoPGuard returns the reference to the controller's safety guard.
func (c *AppController) DoPGuard() *DoPSafetyGuard {
	return c.dopGuard
}

// ConfigureDoP sets up the engine and DoP packer for the selected audio device.
// Strictly enforces safety guard: if unconfirmed, playback is refused and engine remains OpenMuted.
func (c *AppController) ConfigureDoP(req DoPRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. Mandatory safety confirmation check
	if err := c.dopGuard.Validate(true); err != nil {
		return err
	}

	if c.selectedDevice == nil {
		return ErrDeviceNotSelected
	}

	// 2. Validate DSD format for Phase 3 DoP
	carrierRate := req.DSDFormat.DoPCarrierSampleRate()
	if carrierRate == 0 {
		return fmt.Errorf("%w: requested %s", ErrUnsupportedDoPDSDFormat, req.DSDFormat)
	}

	if req.Channels <= 0 {
		req.Channels = 2
	}
	// DoP v1.1 is locked to the 24-in-32 container in this application so a
	// caller cannot accidentally select a lossy or incompatible PCM layout.
	req.SampleFormat = audio.SampleFormat24In32

	// 3. Load Golden Vector for requested DSD format
	golden, err := dsd.GetGoldenVector(req.DSDFormat)
	if err != nil {
		return fmt.Errorf("failed to retrieve golden vector: %w", err)
	}

	// 4. Create DoPPacker
	packer, err := dsd.NewDoPPacker(golden.Data, req.Channels, req.SampleFormat, req.BitOrder)
	if err != nil {
		return fmt.Errorf("failed to create DoP packer: %w", err)
	}

	// 5. Close PCM/Native streams before negotiating the DoP carrier.
	if c.engine.State() != engine.StateClosed {
		_ = c.engine.Close()
	}
	if c.nativeDSDActive || c.nativeDSDFormat != 0 || c.asioService.NativeDSDMode() {
		if err := c.deactivateNativeDSDLocked(); err != nil {
			return fmt.Errorf("failed to restore PCM mode before DoP: %w", err)
		}
	}

	// 6. Configure engine at carrier rate in Exclusive mode
	targetFormat := audio.AudioFormat{
		SampleRate:   carrierRate,
		Channels:     req.Channels,
		SampleFormat: req.SampleFormat,
		ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
	}

	engCfg := engine.EngineConfig{
		Device:         c.selectedDevice,
		Format:         targetFormat,
		ShareMode:      audio.ShareModeExclusive,
		BufferDuration: 10 * time.Millisecond,
		FadeDurationMs: 0.0, // DoP stream must not be altered by volume fades
	}
	if _, ok := c.selectedDevice.(*asio.Device); ok {
		engCfg.BufferFrames = c.asioService.BufferSize()
	}

	if err := c.engine.Open(engCfg); err != nil {
		return fmt.Errorf("engine open for DoP failed: %w", err)
	}

	// 7. Prime raw DoP stream (enters StatePrimed)
	if err := c.engine.PrimeRaw(packer); err != nil {
		return fmt.Errorf("engine prime DoP failed: %w", err)
	}

	c.currentRequest = SignalRequest{
		DeviceID:     req.DeviceID,
		SampleRate:   carrierRate,
		SampleFormat: req.SampleFormat,
		Channels:     req.Channels,
		Frequency:    1000.0,
		Amplitude:    1.0, // Unity gain for bit-perfect DoP
	}

	return nil
}
