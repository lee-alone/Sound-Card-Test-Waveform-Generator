package controller

import (
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/engine"
)

type MockBackend struct {
	device audio.Device
}

func (m *MockBackend) Type() audio.BackendType              { return audio.BackendMock }
func (m *MockBackend) Initialize() error                    { return nil }
func (m *MockBackend) Close() error                         { return nil }
func (m *MockBackend) EnumerateDevices() ([]audio.DeviceInfo, error) {
	return []audio.DeviceInfo{
		{ID: "mock-dev-1", Name: "Mock DAC", IsDefault: true, State: audio.StateActive},
	}, nil
}
func (m *MockBackend) GetDevice(id string) (audio.Device, error) {
	return m.device, nil
}
func (m *MockBackend) GetDefaultDevice() (audio.Device, error) {
	return m.device, nil
}

type MockControllerStream struct {
	bufferSize uint32
}

func (m *MockControllerStream) Start() error                                        { return nil }
func (m *MockControllerStream) Stop() error                                         { return nil }
func (m *MockControllerStream) GetBuffer(numFrames int) (uintptr, error)            { return 0, nil }
func (m *MockControllerStream) ReleaseBuffer(numFramesWritten int, flags uint32) error { return nil }
func (m *MockControllerStream) GetBufferSize() (uint32, error)                     { return m.bufferSize, nil }
func (m *MockControllerStream) GetDevicePeriod() (time.Duration, time.Duration, error) {
	return 10 * time.Millisecond, 5 * time.Millisecond, nil
}
func (m *MockControllerStream) GetWaitHandle() uintptr { return 0 }
func (m *MockControllerStream) Close() error           { return nil }

type MockControllerDevice struct {
	stream audio.AudioStream
}

func (d *MockControllerDevice) ID() string         { return "mock-dev-1" }
func (d *MockControllerDevice) Name() string       { return "Mock DAC" }
func (d *MockControllerDevice) Flow() audio.DeviceFlow { return audio.FlowRender }
func (d *MockControllerDevice) State() audio.DeviceState { return audio.StateActive }
func (d *MockControllerDevice) ProbeCapabilities(candidates []audio.AudioFormat, shareMode audio.ShareMode) ([]audio.FormatSupportResult, error) {
	results := make([]audio.FormatSupportResult, len(candidates))
	for i, c := range candidates {
		results[i] = audio.FormatSupportResult{Format: c, Status: audio.StatusSupported}
	}
	return results, nil
}
func (d *MockControllerDevice) OpenStream(config audio.StreamConfig) (audio.AudioStream, error) {
	return d.stream, nil
}
func (d *MockControllerDevice) Close() error {
	return nil
}

func TestAppControllerOrchestration(t *testing.T) {
	stream := &MockControllerStream{bufferSize: 480}
	dev := &MockControllerDevice{stream: stream}
	backend := &MockBackend{device: dev}

	ctrl := NewAppController(backend)
	if err := ctrl.Initialize(); err != nil {
		t.Fatalf("ctrl.Initialize failed: %v", err)
	}
	defer ctrl.Close()

	// 1. Select device
	if err := ctrl.SelectDevice(""); err != nil {
		t.Fatalf("ctrl.SelectDevice failed: %v", err)
	}

	// 2. Configure signal: 1kHz at 48kHz stereo
	req := SignalRequest{
		DeviceID:     "mock-dev-1",
		SampleRate:   48000,
		SampleFormat: audio.SampleFormat16Int,
		Channels:     2,
		Frequency:    1000.0,
		Amplitude:    0.8,
	}

	if err := ctrl.ConfigureSignal(req); err != nil {
		t.Fatalf("ctrl.ConfigureSignal failed: %v", err)
	}

	status := ctrl.Status()
	if status.State != engine.StatePrimed {
		t.Errorf("status state want Primed, got %v", status.State)
	}

	// 3. Start
	if err := ctrl.StartPlayback(); err != nil {
		t.Fatalf("ctrl.StartPlayback failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	status = ctrl.Status()
	if status.State != engine.StateRunning {
		t.Errorf("status state want Running, got %v", status.State)
	}

	// 4. Stop
	if err := ctrl.StopPlayback(); err != nil {
		t.Fatalf("ctrl.StopPlayback failed: %v", err)
	}
	status = ctrl.Status()
	if status.State != engine.StateOpenMuted {
		t.Errorf("status state after stop want OpenMuted, got %v", status.State)
	}
}
