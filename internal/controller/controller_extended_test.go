package controller

import (
	"encoding/json"
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/dsd"
	"sound-signal-generator/internal/engine"
)

func setupTestController(t *testing.T) *AppController {
	mockDev := &MockControllerDevice{stream: &MockControllerStream{bufferSize: 1920}}
	backend := &MockBackend{device: mockDev}
	ctrl := newTestAppController(backend)
	if err := ctrl.Initialize(); err != nil {
		t.Fatalf("controller init failed: %v", err)
	}
	if err := ctrl.SelectDevice("mock-dev-1"); err != nil {
		t.Fatalf("select device failed: %v", err)
	}
	return ctrl
}

func TestController_ExtendedWaveforms(t *testing.T) {
	ctrl := setupTestController(t)
	defer ctrl.Close()

	waveforms := []struct {
		name     string
		waveform WaveformType
		setupReq func(r *ExtendedSignalRequest)
	}{
		{
			name:     "Sine",
			waveform: WaveformSine,
			setupReq: func(r *ExtendedSignalRequest) { r.Frequency = 1000.0 },
		},
		{
			name:     "PolyBLEP_Square",
			waveform: WaveformPolyBlepSquare,
			setupReq: func(r *ExtendedSignalRequest) {
				r.Frequency = 1000.0
				ctrl.ConfirmSafety(RiskSquareWave, true)
			},
		},
		{
			name:     "PolyBLEP_Triangle",
			waveform: WaveformPolyBlepTriangle,
			setupReq: func(r *ExtendedSignalRequest) { r.Frequency = 1000.0 },
		},
		{
			name:     "Linear_Sweep",
			waveform: WaveformLinearSweep,
			setupReq: func(r *ExtendedSignalRequest) {
				r.Sweep.StartFreq = 20.0
				r.Sweep.EndFreq = 18000.0
				r.Sweep.DurationSec = 1.0
			},
		},
		{
			name:     "Logarithmic_Sweep",
			waveform: WaveformLogarithmicSweep,
			setupReq: func(r *ExtendedSignalRequest) {
				r.Sweep.StartFreq = 20.0
				r.Sweep.EndFreq = 18000.0
				r.Sweep.DurationSec = 1.0
			},
		},
		{
			name:     "White_Noise",
			waveform: WaveformWhiteNoise,
			setupReq: func(r *ExtendedSignalRequest) { r.Noise.Seed = 12345 },
		},
		{
			name:     "Pink_Noise",
			waveform: WaveformPinkNoise,
			setupReq: func(r *ExtendedSignalRequest) { r.Noise.Seed = 54321 },
		},
		{
			name:     "SMPTE_Multitone",
			waveform: WaveformMultitoneSMPTE,
			setupReq: func(r *ExtendedSignalRequest) {
				ctrl.ConfirmSafety(RiskMultitoneHighCrest, true)
			},
		},
		{
			name:     "CCIF_Multitone",
			waveform: WaveformMultitoneCCIF,
			setupReq: func(r *ExtendedSignalRequest) {
				ctrl.ConfirmSafety(RiskMultitoneHighCrest, true)
			},
		},
	}

	for _, tc := range waveforms {
		t.Run(tc.name, func(t *testing.T) {
			req := DefaultExtendedSignalRequest()
			req.Waveform = tc.waveform
			tc.setupReq(&req)

			err := ctrl.ConfigureExtendedSignal(req)
			if err != nil {
				t.Fatalf("ConfigureExtendedSignal for %s failed: %v", tc.name, err)
			}

			if ctrl.engine.State() != engine.StatePrimed && ctrl.engine.State() != engine.StateOpenMuted {
				t.Errorf("expected engine StatePrimed/OpenMuted, got %s", ctrl.engine.State())
			}

			err = ctrl.StartPlayback()
			if err != nil {
				t.Fatalf("StartPlayback for %s failed: %v", tc.name, err)
			}

			time.Sleep(20 * time.Millisecond)

			err = ctrl.StopPlayback()
			if err != nil {
				t.Fatalf("StopPlayback for %s failed: %v", tc.name, err)
			}
		})
	}
}

func TestController_UltrasoundSafetyBlock(t *testing.T) {
	ctrl := setupTestController(t)
	defer ctrl.Close()

	req := DefaultExtendedSignalRequest()
	req.SampleRate = 96000
	req.Frequency = 25000.0 // > 20 kHz: Ultrasound!

	// Reset any confirmation
	ctrl.SafetyManager().Reset()

	// Should be refused because ultrasound risk was not confirmed
	err := ctrl.ConfigureExtendedSignal(req)
	if err == nil {
		t.Fatalf("expected ultrasound safety error, got nil")
	}

	// Confirm ultrasound safety
	ctrl.ConfirmSafety(RiskUltrasound, true)

	err = ctrl.ConfigureExtendedSignal(req)
	if err != nil {
		t.Fatalf("expected success after confirming ultrasound safety, got: %v", err)
	}
}

func TestController_DoPSafetyBlock(t *testing.T) {
	ctrl := setupTestController(t)
	defer ctrl.Close()

	req := DefaultExtendedSignalRequest()
	req.Waveform = WaveformDoP
	req.DoP.DSDFormat = dsd.DSD64
	req.SampleRate = 176400
	req.SampleFormat = audio.SampleFormat24In32

	ctrl.SafetyManager().Reset()

	// Refused without confirmation
	err := ctrl.ConfigureExtendedSignal(req)
	if err == nil {
		t.Fatalf("expected DoP safety error, got nil")
	}

	// Confirm DoP
	ctrl.ConfirmSafety(RiskDoP, true)

	err = ctrl.ConfigureExtendedSignal(req)
	if err != nil {
		t.Fatalf("expected success after confirming DoP risk, got: %v", err)
	}
}

func TestController_10HzDiagnosticsReport(t *testing.T) {
	ctrl := setupTestController(t)
	defer ctrl.Close()

	req := DefaultExtendedSignalRequest()
	req.Frequency = 1000.0
	req.Amplitude = 1.0
	req.LeftVolumeDBFS = 0.0
	req.RightVolumeDBFS = 0.0
	_ = ctrl.ConfigureExtendedSignal(req)

	reportBefore := ctrl.Get10HzDiagnostics()
	if reportBefore.BitPerfectStatus != TierConfigured {
		t.Errorf("expected TierConfigured before starting playback, got %s", reportBefore.BitPerfectStatus)
	}

	_ = ctrl.StartPlayback()
	time.Sleep(30 * time.Millisecond)

	reportDuring := ctrl.Get10HzDiagnostics()
	if reportDuring.State != engine.StateRunning {
		t.Errorf("expected StateRunning, got %s", reportDuring.State)
	}
	if reportDuring.BufferPeriodMs != 10.0 {
		t.Errorf("expected 10.0 ms buffer period, got %f", reportDuring.BufferPeriodMs)
	}
	if reportDuring.RenderCallbackCount == 0 {
		t.Errorf("expected render callbacks > 0, got %d", reportDuring.RenderCallbackCount)
	}

	_ = ctrl.StopPlayback()
}

func TestController_ReconfigureWhileRunning(t *testing.T) {
	ctrl := setupTestController(t)
	defer ctrl.Close()

	req := DefaultExtendedSignalRequest()
	req.Frequency = 1000.0

	if err := ctrl.ConfigureExtendedSignal(req); err != nil {
		t.Fatalf("ConfigureExtendedSignal failed: %v", err)
	}
	if err := ctrl.StartPlayback(); err != nil {
		t.Fatalf("StartPlayback failed: %v", err)
	}

	// Re-configure while running: should stop, close, reopen, and prime
	req.Frequency = 2000.0
	if err := ctrl.ConfigureExtendedSignal(req); err != nil {
		t.Fatalf("reconfigure while running failed: %v", err)
	}

	if ctrl.engine.State() != engine.StatePrimed {
		t.Fatalf("expected state Primed, got %s", ctrl.engine.State())
	}

	// Resume
	if err := ctrl.StartPlayback(); err != nil {
		t.Fatalf("StartPlayback after reconfigure failed: %v", err)
	}
	_ = ctrl.StopPlayback()
}

func TestController_JSONUnmarshalExtendedSignalRequest(t *testing.T) {
	jsonPayload := `{
		"device_id": "",
		"sample_rate": 48000,
		"sample_format": 0,
		"channels": 2,
		"waveform": "sine",
		"frequency": 1000.0,
		"amplitude": 0.501187,
		"dc_offset": 0.0,
		"left_volume_dbfs": -6.0,
		"right_volume_dbfs": -6.0,
		"sweep": {
			"start_freq": 20.0,
			"end_freq": 20000.0,
			"duration_sec": 5.0,
			"taper_fraction": 0.10,
			"loop": true
		},
		"noise": {
			"seed": 12345
		},
		"multitone": {
			"frequencies": [60, 7000],
			"amplitudes": [0.8, 0.2]
		},
		"dop": {
			"dsd_format": "DSD64",
			"bit_order": 0
		}
	}`

	var req ExtendedSignalRequest
	if err := json.Unmarshal([]byte(jsonPayload), &req); err != nil {
		t.Fatalf("failed to unmarshal ExtendedSignalRequest: %v", err)
	}

	if req.DoP.DSDFormat != dsd.DSD64 {
		t.Errorf("expected DSDFormat to be DSD64, got %v", req.DoP.DSDFormat)
	}
	if req.DoP.BitOrder != dsd.BitOrderMSBFirst {
		t.Errorf("expected BitOrder to be BitOrderMSBFirst, got %v", req.DoP.BitOrder)
	}
}

func TestController_ReconfigureWhilePrimed(t *testing.T) {
	ctrl := setupTestController(t)
	defer ctrl.Close()

	req := DefaultExtendedSignalRequest()
	req.Frequency = 1000.0

	// 1. Initial configuration (State becomes Primed)
	if err := ctrl.ConfigureExtendedSignal(req); err != nil {
		t.Fatalf("first ConfigureExtendedSignal failed: %v", err)
	}

	if ctrl.engine.State() != engine.StatePrimed {
		t.Fatalf("expected state Primed, got %s", ctrl.engine.State())
	}

	// 2. Re-configure while in StatePrimed (this previously threw 'engine is already open')
	if err := ctrl.ConfigureExtendedSignal(req); err != nil {
		t.Fatalf("second ConfigureExtendedSignal failed: %v", err)
	}

	if ctrl.engine.State() != engine.StatePrimed {
		t.Fatalf("expected state Primed after reconfigure, got %s", ctrl.engine.State())
	}

	// 3. Start playback
	if err := ctrl.StartPlayback(); err != nil {
		t.Fatalf("StartPlayback failed: %v", err)
	}

	if ctrl.engine.State() != engine.StateRunning {
		t.Fatalf("expected state Running, got %s", ctrl.engine.State())
	}

	_ = ctrl.StopPlayback()
}
