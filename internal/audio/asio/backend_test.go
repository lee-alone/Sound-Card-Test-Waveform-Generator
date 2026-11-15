package asio

import (
	"errors"
	"testing"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio/bridge"
)

func TestNewBackendIsNotMock(t *testing.T) {
	backend := NewBackend()
	if backend.IsMock() {
		t.Fatalf("NewBackend must not select mock mode")
	}
}

func TestLiveDriverCreationFailureDoesNotFallbackToMock(t *testing.T) {
	backend := NewBackend()
	backend.SetCustomDriverFactory(func(bridge.DriverDesc) (bridge.Driver, error) {
		return nil, errors.New("COM instantiation failed")
	})
	dev, err := backend.createDevice(audio.DeviceInfo{ID: "ASIO Combo384 Driver", Name: "ASIO Combo384 Driver", Backend: audio.BackendASIO})
	if err == nil {
		t.Fatalf("expected live driver failure")
	}
	if dev != nil {
		t.Fatalf("driver failure must not return a mock device: %T", dev)
	}
}
