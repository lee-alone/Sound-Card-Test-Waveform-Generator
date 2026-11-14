package controller

import (
	"errors"
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/dsd"
	"sound-signal-generator/internal/engine"
)

type mockBackendWithDoP struct {
	dev *mockDoPDeviceForCtrl
}

func (m *mockBackendWithDoP) Type() audio.BackendType              { return audio.BackendWASAPI }
func (m *mockBackendWithDoP) Initialize() error                    { return nil }
func (m *mockBackendWithDoP) Close() error                         { return nil }
func (m *mockBackendWithDoP) EnumerateDevices() ([]audio.DeviceInfo, error) {
	return []audio.DeviceInfo{
		{ID: "dop-device-1", Name: "DoP Capable DAC", Flow: audio.FlowRender, State: audio.StateActive, IsDefault: true},
	}, nil
}
func (m *mockBackendWithDoP) GetDefaultDevice() (audio.Device, error) { return m.dev, nil }
func (m *mockBackendWithDoP) GetDevice(id string) (audio.Device, error) { return m.dev, nil }

type mockDoPDeviceForCtrl struct {
	lastOpened audio.AudioFormat
}

func (d *mockDoPDeviceForCtrl) ID() string             { return "dop-device-1" }
func (d *mockDoPDeviceForCtrl) Name() string           { return "DoP Capable DAC" }
func (d *mockDoPDeviceForCtrl) Flow() audio.DeviceFlow { return audio.FlowRender }
func (d *mockDoPDeviceForCtrl) State() audio.DeviceState {
	return audio.StateActive
}
func (d *mockDoPDeviceForCtrl) ProbeCapabilities(candidates []audio.AudioFormat, shareMode audio.ShareMode) ([]audio.FormatSupportResult, error) {
	res := make([]audio.FormatSupportResult, len(candidates))
	for i, c := range candidates {
		res[i] = audio.FormatSupportResult{Format: c, Status: audio.StatusSupported}
	}
	return res, nil
}
func (d *mockDoPDeviceForCtrl) OpenStream(config audio.StreamConfig) (audio.AudioStream, error) {
	d.lastOpened = config.Format
	return &mockCtrlDoPStream{format: config.Format}, nil
}
func (d *mockDoPDeviceForCtrl) Close() error { return nil }

type mockCtrlDoPStream struct {
	format audio.AudioFormat
}

func (s *mockCtrlDoPStream) Start() error                                        { return nil }
func (s *mockCtrlDoPStream) Stop() error                                         { return nil }
func (s *mockCtrlDoPStream) GetBuffer(numFrames int) (uintptr, error)            { return 0, nil }
func (s *mockCtrlDoPStream) ReleaseBuffer(numFramesWritten int, flags uint32) error { return nil }
func (s *mockCtrlDoPStream) GetBufferSize() (uint32, error)                      { return 1764, nil }
func (s *mockCtrlDoPStream) GetDevicePeriod() (time.Duration, time.Duration, error) {
	return 10 * time.Millisecond, 5 * time.Millisecond, nil
}
func (s *mockCtrlDoPStream) GetWaitHandle() uintptr { return 0 }
func (s *mockCtrlDoPStream) Close() error           { return nil }

func TestConfigureDoPEnforcesSafetyGuard(t *testing.T) {
	mockDev := &mockDoPDeviceForCtrl{}
	ctrl := NewAppController(&mockBackendWithDoP{dev: mockDev})
	_ = ctrl.Initialize()
	_ = ctrl.SelectDevice("dop-device-1")
	defer ctrl.Close()

	req := DoPRequest{
		DeviceID:     "dop-device-1",
		DSDFormat:    dsd.DSD64,
		SampleFormat: audio.SampleFormat24In32,
		Channels:     2,
		BitOrder:     dsd.BitOrderMSBFirst,
	}

	// 1. Without confirmation, must fail with ErrDoPRiskNotConfirmed
	err := ctrl.ConfigureDoP(req)
	if !errors.Is(err, ErrDoPRiskNotConfirmed) {
		t.Fatalf("expected ErrDoPRiskNotConfirmed, got %v", err)
	}
	if ctrl.engine.State() != engine.StateClosed {
		t.Errorf("engine must remain closed/muted on unconfirmed DoP, got %s", ctrl.engine.State())
	}

	// 2. Reject DSD256 in Phase 3
	ctrl.DoPGuard().ConfirmViaFlag(true)
	req256 := req
	req256.DSDFormat = dsd.DSD256
	err = ctrl.ConfigureDoP(req256)
	if !errors.Is(err, ErrUnsupportedDoPDSDFormat) {
		t.Fatalf("expected ErrUnsupportedDoPDSDFormat for DSD256, got %v", err)
	}

	// 3. Valid DSD64 configuration
	err = ctrl.ConfigureDoP(req)
	if err != nil {
		t.Fatalf("unexpected error configuring DoP: %v", err)
	}

	if ctrl.engine.State() != engine.StatePrimed {
		t.Errorf("engine state should be Primed, got %s", ctrl.engine.State())
	}

	if mockDev.lastOpened.SampleRate != 176400 {
		t.Errorf("expected 176400 carrier rate, got %d", mockDev.lastOpened.SampleRate)
	}
}
