package controller

import (
	"path/filepath"
	"testing"
)

func TestUIService_FullWorkflow(t *testing.T) {
	mockDev := &MockControllerDevice{stream: &MockControllerStream{bufferSize: 1920}}
	backend := &MockBackend{device: mockDev}
	ctrl := newTestAppController(backend)
	_ = ctrl.Initialize()
	defer ctrl.Close()

	svc := NewUIService(ctrl)

	// 1. Devices
	devs, err := svc.GetDevices()
	if err != nil || len(devs) == 0 {
		t.Fatalf("failed to get devices: %v", err)
	}

	err = svc.SelectDevice("mock-dev-1")
	if err != nil {
		t.Fatalf("failed to select device: %v", err)
	}

	// 2. Capabilities
	caps, err := svc.ProbeDeviceCapabilities()
	if err != nil || len(caps) == 0 {
		t.Fatalf("probe failed: %v", err)
	}

	// 3. Configure and safety
	req := DefaultExtendedSignalRequest()
	req.Waveform = WaveformSine
	req.Frequency = 1000.0

	assessment := svc.CheckSafety(req)
	if assessment.HasRisk {
		t.Errorf("1kHz sine should not have risk")
	}

	err = svc.ConfigureSignal(req)
	if err != nil {
		t.Fatalf("configure failed: %v", err)
	}

	// 4. Playback and 10Hz Diagnostics
	err = svc.StartPlayback()
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	diag := svc.GetDiagnostics()
	if diag.BufferPeriodMs <= 0 {
		t.Errorf("invalid buffer period: %f", diag.BufferPeriodMs)
	}

	err = svc.StopPlayback()
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	// 5. Preset Save & Apply
	tmp := t.TempDir()
	pPath := filepath.Join(tmp, "p.json")
	err = svc.SavePreset(pPath, "Preset1", "Desc", req)
	if err != nil {
		t.Fatalf("save preset failed: %v", err)
	}

	loaded, err := svc.LoadPreset(pPath)
	if err != nil {
		t.Fatalf("load preset failed: %v", err)
	}

	err = svc.ApplyPreset(loaded)
	if err != nil {
		t.Fatalf("apply preset failed: %v", err)
	}

	// 6. Device change polling
	ev := svc.PollDeviceChange()
	_ = ev // nil when no change
}
