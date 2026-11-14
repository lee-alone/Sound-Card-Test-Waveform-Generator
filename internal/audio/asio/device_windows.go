package asio

import (
	"errors"
	"sync"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio/bridge"
)

// Device wraps an ASIO hardware driver as an audio.Device implementation.
type Device struct {
	mu           sync.Mutex
	info         audio.DeviceInfo
	driver       bridge.Driver
	activeStream *Stream
}

// NewDevice wraps an ASIO driver instance as an audio.Device.
func NewDevice(info audio.DeviceInfo, driver bridge.Driver) *Device {
	return &Device{
		info:   info,
		driver: driver,
	}
}

func (d *Device) ID() string {
	return d.info.ID
}

func (d *Device) Name() string {
	return d.info.Name
}

func (d *Device) Flow() audio.DeviceFlow {
	return d.info.Flow
}

func (d *Device) State() audio.DeviceState {
	return d.info.State
}

// Driver returns the underlying ASIO driver interface.
func (d *Device) Driver() bridge.Driver {
	return d.driver
}

func (d *Device) ProbeCapabilities(candidates []audio.AudioFormat, shareMode audio.ShareMode) ([]audio.FormatSupportResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.driver == nil {
		return nil, errors.New("driver not initialized")
	}

	_ = d.driver.Init(0)
	_, numOutputs, err := d.driver.GetChannels()
	if err != nil {
		return nil, err
	}

	results := make([]audio.FormatSupportResult, len(candidates))
	for i, cand := range candidates {
		res := audio.FormatSupportResult{
			Format: cand,
			Status: audio.StatusUnsupported,
		}

		if cand.Channels > numOutputs {
			res.Status = audio.StatusUnsupported
			results[i] = res
			continue
		}

		if d.driver.CanSampleRate(float64(cand.SampleRate)) {
			res.Status = audio.StatusSupported
		} else {
			res.Status = audio.StatusUnsupported
		}

		results[i] = res
	}

	return results, nil
}

func (d *Device) OpenStream(config audio.StreamConfig) (audio.AudioStream, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.activeStream != nil {
		_ = d.activeStream.Close()
		d.activeStream = nil
	}

	stream, err := NewStream(d.driver, config)
	if err != nil {
		return nil, err
	}

	d.activeStream = stream
	return stream, nil
}

func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.activeStream != nil {
		_ = d.activeStream.Close()
		d.activeStream = nil
	}

	if d.driver != nil {
		_ = d.driver.Close()
	}
	return nil
}
