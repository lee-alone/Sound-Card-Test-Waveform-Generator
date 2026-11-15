package controller

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"sound-signal-generator/internal/engine"
)

func TestPreset_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	presetFile := filepath.Join(tempDir, "test_preset.json")

	req := DefaultExtendedSignalRequest()
	req.Waveform = WaveformPolyBlepSquare
	req.Frequency = 2500.0
	req.Amplitude = 0.5
	req.LeftVolumeDBFS = -3.0
	req.RightVolumeDBFS = -6.0

	origPreset := NewSignalPreset("Industrial Square 2.5k", "Square wave test for amplifier slew rate", req)

	err := SavePresetToFile(presetFile, origPreset)
	if err != nil {
		t.Fatalf("SavePresetToFile failed: %v", err)
	}

	loaded, err := LoadPresetFromFile(presetFile)
	if err != nil {
		t.Fatalf("LoadPresetFromFile failed: %v", err)
	}

	if loaded.Name != origPreset.Name {
		t.Errorf("expected name %s, got %s", origPreset.Name, loaded.Name)
	}
	if loaded.Request.Waveform != WaveformPolyBlepSquare {
		t.Errorf("expected waveform square, got %s", loaded.Request.Waveform)
	}
	if loaded.Request.Frequency != 2500.0 {
		t.Errorf("expected freq 2500, got %f", loaded.Request.Frequency)
	}
	if loaded.Request.LeftVolumeDBFS != -3.0 {
		t.Errorf("expected left volume -3.0, got %f", loaded.Request.LeftVolumeDBFS)
	}
}

func TestPreset_CorruptFile(t *testing.T) {
	tempDir := t.TempDir()
	badFile := filepath.Join(tempDir, "bad.json")
	_ = os.WriteFile(badFile, []byte("{ invalid json"), 0644)

	_, err := LoadPresetFromFile(badFile)
	if err == nil {
		t.Fatalf("expected error on corrupt JSON, got nil")
	}
}

func TestPreset_ApplySafely_MaintainsMutedState(t *testing.T) {
	mockDev := &MockControllerDevice{stream: &MockControllerStream{bufferSize: 1920}}
	mockBackend := &MockBackend{device: mockDev}
	ctrl := newTestAppController(mockBackend)
	if err := ctrl.Initialize(); err != nil {
		t.Fatalf("ctrl init failed: %v", err)
	}
	defer ctrl.Close()

	if err := ctrl.SelectDevice("mock-dev-1"); err != nil {
		t.Fatalf("select device failed: %v", err)
	}

	req := DefaultExtendedSignalRequest()
	req.Frequency = 1000.0
	req.SampleRate = 48000
	preset := NewSignalPreset("Safe Test", "Ensures open muted invariant", req)

	// Apply preset
	err := ApplyPresetSafely(ctrl, &preset)
	if err != nil {
		t.Fatalf("ApplyPresetSafely failed: %v", err)
	}

	// Gate 4 / T4.6 verification: Output state MUST NOT be Running
	finalState := ctrl.engine.State()
	if finalState == engine.StateRunning {
		t.Errorf("SAFETY VIOLATION: applying preset resulted in StateRunning! Must be muted")
	}
	if finalState != engine.StatePrimed && finalState != engine.StateOpenMuted {
		t.Errorf("expected StatePrimed or StateOpenMuted, got %s", finalState)
	}
}

func TestPreset_ApplyWhilePlaying_TransitionsToSafe(t *testing.T) {
	mockDev := &MockControllerDevice{stream: &MockControllerStream{bufferSize: 1920}}
	mockBackend := &MockBackend{device: mockDev}
	ctrl := newTestAppController(mockBackend)
	_ = ctrl.Initialize()
	defer ctrl.Close()
	_ = ctrl.SelectDevice("mock-dev-1")

	req := DefaultExtendedSignalRequest()
	_ = ctrl.ConfigureExtendedSignal(req)
	_ = ctrl.StartPlayback()

	// Wait briefly so playback is active
	time.Sleep(20 * time.Millisecond)

	presetReq := DefaultExtendedSignalRequest()
	presetReq.Frequency = 440.0
	preset := NewSignalPreset("Switch While Playing", "desc", presetReq)

	err := ApplyPresetSafely(ctrl, &preset)
	if err != nil {
		t.Fatalf("ApplyPresetSafely failed: %v", err)
	}

	// Must NOT remain in StateRunning
	if ctrl.engine.State() == engine.StateRunning {
		t.Errorf("SAFETY VIOLATION: engine remained in StateRunning after preset load")
	}
}
