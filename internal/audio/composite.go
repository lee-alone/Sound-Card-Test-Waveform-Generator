package audio

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

const compositeIDSeparator = "::"

// CompositeBackend exposes several audio backends behind one AudioBackend so
// UI and controller code can select WASAPI and ASIO endpoints uniformly.
//
// Composite device IDs have the form "<backend-type>::<native-id>". Raw native
// IDs are also accepted for backwards compatibility, provided they are unique.
type CompositeBackend struct {
	mu          sync.Mutex
	backends    []AudioBackend
	initialized bool
}

// NewCompositeBackend creates a routed backend. Nil entries are ignored.
func NewCompositeBackend(backends ...AudioBackend) *CompositeBackend {
	clean := make([]AudioBackend, 0, len(backends))
	for _, backend := range backends {
		if backend != nil {
			clean = append(clean, backend)
		}
	}
	return &CompositeBackend{backends: clean}
}

func (b *CompositeBackend) Type() BackendType {
	return "Composite"
}

func (b *CompositeBackend) Initialize() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.initialized {
		return nil
	}

	var errs []error
	for _, backend := range b.backends {
		if err := backend.Initialize(); err != nil {
			errs = append(errs, fmt.Errorf("%s initialize: %w", backend.Type(), err))
		}
	}
	b.initialized = true
	return errors.Join(errs...)
}

func (b *CompositeBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.initialized {
		return nil
	}

	var errs []error
	for _, backend := range b.backends {
		if err := backend.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s close: %w", backend.Type(), err))
		}
	}
	b.initialized = false
	return errors.Join(errs...)
}

func (b *CompositeBackend) EnumerateDevices() ([]DeviceInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	devices := make([]DeviceInfo, 0)
	var errs []error
	for _, backend := range b.backends {
		items, err := backend.EnumerateDevices()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s enumerate: %w", backend.Type(), err))
			continue
		}
		for _, item := range items {
			item.ID = makeCompositeID(backend.Type(), item.ID)
			item.Backend = backend.Type()
			devices = append(devices, item)
		}
	}

	// A failed secondary backend must not hide usable primary endpoints. The
	// returned error is reserved for the case where no backend could enumerate.
	if len(devices) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return devices, nil
}

// ResolveDevice resolves a composite or native ID and returns the device plus
// the canonical composite ID. An empty ID selects the first available default.
func (b *CompositeBackend) ResolveDevice(id string) (Device, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if id == "" {
		for _, backend := range b.backends {
			dev, err := backend.GetDefaultDevice()
			if err == nil && dev != nil {
				return dev, makeCompositeID(backend.Type(), dev.ID()), nil
			}
		}
		return nil, "", errors.New("no default audio device available")
	}

	if backendType, nativeID, ok := splitCompositeID(id); ok {
		for _, backend := range b.backends {
			if backend.Type() != backendType {
				continue
			}
			dev, err := backend.GetDevice(nativeID)
			if err != nil {
				return nil, "", err
			}
			return dev, makeCompositeID(backendType, nativeID), nil
		}
		return nil, "", fmt.Errorf("audio backend not found: %s", backendType)
	}

	var matched Device
	var matchedKey string
	for _, backend := range b.backends {
		dev, err := backend.GetDevice(id)
		if err != nil || dev == nil {
			continue
		}
		if matched != nil {
			return nil, "", fmt.Errorf("ambiguous native device ID %q; use a composite ID", id)
		}
		matched = dev
		matchedKey = makeCompositeID(backend.Type(), dev.ID())
	}
	if matched == nil {
		return nil, "", fmt.Errorf("audio device not found: %s", id)
	}
	return matched, matchedKey, nil
}

func (b *CompositeBackend) GetDevice(id string) (Device, error) {
	dev, _, err := b.ResolveDevice(id)
	return dev, err
}

func (b *CompositeBackend) GetDefaultDevice() (Device, error) {
	dev, _, err := b.ResolveDevice("")
	return dev, err
}

func makeCompositeID(backendType BackendType, nativeID string) string {
	return string(backendType) + compositeIDSeparator + nativeID
}

func splitCompositeID(id string) (BackendType, string, bool) {
	parts := strings.SplitN(id, compositeIDSeparator, 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", false
	}
	return BackendType(parts[0]), parts[1], true
}
