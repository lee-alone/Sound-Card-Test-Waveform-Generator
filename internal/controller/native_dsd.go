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

func (c *AppController) nativeDSDDeviceLocked() (*asio.Device, error) {
	dev, ok := c.selectedDevice.(*asio.Device)
	if !ok || dev == nil {
		return nil, ErrNativeDSDRequiresASIO
	}
	return dev, nil
}

// deactivateNativeDSDLocked closes the service-owned native stream and clears
// the controller's mode flags. c.mu must be held by the caller.
func (c *AppController) deactivateNativeDSDLocked() error {
	restoreErr := c.asioService.DisableNativeDSD()
	c.nativeDSDActive = false
	c.nativeDSDFormat = 0
	return restoreErr
}

// EnableNativeDSD creates a real ASIO Native DSD stream, injects it into
// AudioEngine as an external stream and primes it with raw 1-bit DSD bytes.
// The engine is never routed through the WASAPI PCM stream for this mode.
func (c *AppController) EnableNativeDSD(format string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.nativeDSDDeviceLocked(); err != nil {
		return err
	}
	if !c.asioService.NativeDSDConfirmed() {
		return ErrNativeDSDNotConfirmed
	}

	// Stop and release any current stream before ASIO asks the driver for new
	// Native DSD buffers.
	if c.engine.State() != engine.StateClosed {
		if closeErr := c.engine.Close(); closeErr != nil {
			return fmt.Errorf("failed to close previous stream: %w", closeErr)
		}
	}

	if err := c.asioService.ConfigureNativeDSD(format); err != nil {
		c.nativeDSDActive = false
		c.nativeDSDFormat = 0
		return err
	}
	stream := c.asioService.NativeStream()
	if stream == nil {
		_ = c.deactivateNativeDSDLocked()
		return errors.New("Native DSD stream was not created")
	}

	formatValue := stream.Format()
	golden, err := dsd.GetGoldenVector(formatValue)
	if err != nil {
		_ = c.deactivateNativeDSDLocked()
		return fmt.Errorf("failed to load Native DSD data source: %w", err)
	}
	source, err := dsd.NewNativeDSDSource(golden.Data, 2)
	if err != nil {
		_ = c.deactivateNativeDSDLocked()
		return fmt.Errorf("failed to create Native DSD data source: %w", err)
	}

	bufferFrames, err := stream.GetBufferSize()
	if err != nil || bufferFrames == 0 {
		_ = stream.Close()
		_ = c.deactivateNativeDSDLocked()
		return fmt.Errorf("Native DSD stream returned invalid buffer size: %d, %w", bufferFrames, err)
	}

	cfg := engine.EngineConfig{
		Device:         c.selectedDevice,
		ExternalStream: stream,
		Format: audio.AudioFormat{
			SampleRate:   formatValue.BitRate(),
			Channels:     2,
			SampleFormat: audio.SampleFormat24In32,
			ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
		},
		ShareMode:      audio.ShareModeExclusive,
		BufferDuration: time.Duration(float64(bufferFrames) / float64(formatValue.BitRate()) * float64(time.Second)),
		RawBufferBytes: int(bufferFrames/8) * 2,
		FadeDurationMs: 0.0,
	}

	cleanup := func() {
		_ = c.engine.Close()
		_ = c.deactivateNativeDSDLocked()
		c.nativeDSDActive = false
		c.nativeDSDFormat = 0
	}
	if err := c.engine.Open(cfg); err != nil {
		cleanup()
		return fmt.Errorf("engine open for Native DSD failed: %w", err)
	}
	if err := c.engine.PrimeRaw(source); err != nil {
		cleanup()
		return fmt.Errorf("engine prime Native DSD failed: %w", err)
	}

	c.nativeDSDActive = true
	c.nativeDSDFormat = formatValue
	c.currentRequest = SignalRequest{
		DeviceID:     c.selectedDeviceKey,
		SampleRate:   formatValue.BitRate(),
		SampleFormat: audio.SampleFormat24In32,
		Channels:     2,
		Frequency:    1000.0,
		Amplitude:    1.0,
	}
	return nil
}

// DisableNativeDSD closes the 1-bit stream, restores the PCM ASIO clock and
// safely reopens/primes the last PCM request in muted state.
func (c *AppController) DisableNativeDSD() error {
	c.mu.Lock()
	if err := c.engine.Close(); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to close Native DSD engine: %w", err)
	}
	restoreErr := c.deactivateNativeDSDLocked()
	pcm := c.lastPCMRequest
	var pcmCopy *ExtendedSignalRequest
	if pcm != nil {
		copyReq := *pcm
		pcmCopy = &copyReq
	}
	c.mu.Unlock()

	if restoreErr != nil {
		return fmt.Errorf("failed to restore PCM clock: %w", restoreErr)
	}
	if pcmCopy == nil {
		return nil
	}
	if err := c.ConfigureExtendedSignal(*pcmCopy); err != nil {
		return fmt.Errorf("Native DSD disabled but PCM restoration failed: %w", err)
	}
	return nil
}

// IsNativeDSDActive reports whether the controller owns an active Native DSD stream.
func (c *AppController) IsNativeDSDActive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nativeDSDActive
}
