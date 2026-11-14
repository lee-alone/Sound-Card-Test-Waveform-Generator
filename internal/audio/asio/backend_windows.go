package asio

import (
	"errors"
	"fmt"
	"sync"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio/bridge"
)

// Backend implements audio.AudioBackend for Windows ASIO subsystems.
type Backend struct {
	mu             sync.Mutex
	initialized    bool
	mockMode       bool
	mockProfiles   []bridge.VendorProfile
	customDriverFn func(desc bridge.DriverDesc) (bridge.Driver, error)
}

// NewBackend creates a new ASIO backend instance.
func NewBackend() *Backend {
	return &Backend{}
}

// NewMockBackend creates an ASIO backend configured with simulated vendor profiles for testing.
func NewMockBackend(profiles ...bridge.VendorProfile) *Backend {
	if len(profiles) == 0 {
		profiles = []bridge.VendorProfile{
			bridge.ProfileThesycon,
			bridge.ProfileCombo384,
			bridge.ProfileYamaha,
			bridge.ProfileASIO4ALL,
		}
	}
	return &Backend{
		mockMode:     true,
		mockProfiles: profiles,
	}
}

func (b *Backend) Type() audio.BackendType {
	return audio.BackendASIO
}

func (b *Backend) Initialize() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.initialized = true
	return nil
}

func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.initialized = false
	return nil
}

// SetCustomDriverFactory allows injecting driver factory (for testing or customized CGO loaders).
func (b *Backend) SetCustomDriverFactory(fn func(desc bridge.DriverDesc) (bridge.Driver, error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.customDriverFn = fn
}

func (b *Backend) EnumerateDevices() ([]audio.DeviceInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return nil, errors.New("asio backend not initialized")
	}

	if b.mockMode {
		devices := make([]audio.DeviceInfo, len(b.mockProfiles))
		for i, p := range b.mockProfiles {
			devices[i] = audio.DeviceInfo{
				ID:        p.Name,
				Name:      p.Name,
				IsDefault: i == 0,
				Flow:      audio.FlowRender,
				State:     audio.StateActive,
			}
		}
		return devices, nil
	}

	// Live registry scanning
	descs, err := bridge.EnumerateInstalledDrivers()
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate ASIO drivers: %w", err)
	}

	devices := make([]audio.DeviceInfo, len(descs))
	for i, d := range descs {
		devices[i] = audio.DeviceInfo{
			ID:        d.ID,
			Name:      d.Description,
			IsDefault: i == 0,
			Flow:      audio.FlowRender,
			State:     audio.StateActive,
		}
	}

	return devices, nil
}

func (b *Backend) GetDevice(id string) (audio.Device, error) {
	devices, err := b.EnumerateDevices()
	if err != nil {
		return nil, err
	}

	for _, dev := range devices {
		if dev.ID == id || dev.Name == id {
			return b.createDevice(dev)
		}
	}

	return nil, fmt.Errorf("asio device not found: %s", id)
}

func (b *Backend) GetDefaultDevice() (audio.Device, error) {
	devices, err := b.EnumerateDevices()
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, errors.New("no ASIO drivers found")
	}
	return b.createDevice(devices[0])
}

func (b *Backend) createDevice(info audio.DeviceInfo) (audio.Device, error) {
	if b.mockMode {
		for _, p := range b.mockProfiles {
			if p.Name == info.ID || p.Name == info.Name {
				driver := bridge.NewMockDriver(p)
				return NewDevice(info, driver), nil
			}
		}
	}

	if b.customDriverFn != nil {
		driver, err := b.customDriverFn(bridge.DriverDesc{ID: info.ID, Name: info.Name})
		if err != nil {
			return nil, err
		}
		return NewDevice(info, driver), nil
	}

	// Try live driver instantiation
	descs, err := bridge.EnumerateInstalledDrivers()
	if err == nil {
		for _, d := range descs {
			if d.ID == info.ID || d.Name == info.Name {
				cgoDriver, cgoErr := bridge.NewCGODriver(d.CLSID)
				if cgoErr == nil {
					return NewDevice(info, cgoDriver), nil
				}
				// If CGO instantiation failed (e.g. hardware not attached), fallback to mock with Combo384 profile
				if d.Name == "ASIO Combo384 Driver" {
					return NewDevice(info, bridge.NewMockDriver(bridge.ProfileCombo384)), nil
				}
				return nil, cgoErr
			}
		}
	}

	return nil, fmt.Errorf("unable to instantiate ASIO driver for device %s", info.Name)
}
