package controller

import (
	"testing"

	"sound-signal-generator/internal/audio/asio"
	"sound-signal-generator/internal/audio/asio/bridge"
)

func TestAsioServiceSafetyGuardEnforcement(t *testing.T) {
	backend := asio.NewMockBackend(bridge.ProfileThesycon)
	_ = backend.Initialize()
	defer backend.Close()

	svc := NewAsioService(backend)
	if err := svc.SelectDriver(bridge.ProfileThesycon.Name); err != nil {
		t.Fatalf("SelectDriver failed: %v", err)
	}

	// 1. Direct attempt to enable Native DSD without confirmation MUST FAIL
	err := svc.ConfigureNativeDSD("DSD256")
	if err != ErrNativeDSDNotConfirmed {
		t.Fatalf("expected ErrNativeDSDNotConfirmed, got %v", err)
	}

	// 2. Incorrect confirmation phrase MUST FAIL
	err = svc.ConfirmNativeDSDRisk("maybe")
	if err != ErrInvalidConfirmationPhrase {
		t.Fatalf("expected ErrInvalidConfirmationPhrase, got %v", err)
	}

	// Still unconfirmed
	if err := svc.ConfigureNativeDSD("DSD256"); err != ErrNativeDSDNotConfirmed {
		t.Fatalf("expected still ErrNativeDSDNotConfirmed, got %v", err)
	}

	// 3. Exact confirmation phrase unlocks Native DSD
	if err := svc.ConfirmNativeDSDRisk("YES"); err != nil {
		t.Fatalf("ConfirmNativeDSDRisk failed: %v", err)
	}

	if err := svc.ConfigureNativeDSD("DSD256"); err != nil {
		t.Fatalf("ConfigureNativeDSD after confirmation failed: %v", err)
	}

	// 4. Query status
	status, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if !status.NativeDSDConfirmed || !status.NativeDSDMode {
		t.Errorf("status reports unconfirmed or inactive Native DSD: %+v", status)
	}
	if status.LatencyReport == nil || status.LatencyReport.ReportedTotalLatency == 0 {
		t.Errorf("expected latency report, got %+v", status.LatencyReport)
	}
}

func TestAsioServiceClockAndBufferControl(t *testing.T) {
	backend := asio.NewMockBackend(bridge.ProfileThesycon)
	_ = backend.Initialize()
	defer backend.Close()

	svc := NewAsioService(backend)
	_ = svc.SelectDriver(bridge.ProfileThesycon.Name)

	status, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if len(status.ClockSources) == 0 {
		t.Fatalf("expected clock sources, got none")
	}

	// Switch clock source
	if err := svc.SetClockSource(1); err != nil {
		t.Fatalf("SetClockSource failed: %v", err)
	}

	// Buffer sizing
	if err := svc.SetBufferSize(128); err != nil {
		t.Fatalf("SetBufferSize failed: %v", err)
	}

	status2, _ := svc.GetStatus()
	if status2.ActiveBufferSize != 128 {
		t.Errorf("expected buffer size 128, got %d", status2.ActiveBufferSize)
	}
}
