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
	defer svc.Close()
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
	err = svc.ConfirmNativeDSDRisk("yes")
	if err != ErrInvalidConfirmationPhrase {
		t.Fatalf("expected lowercase phrase rejection, got %v", err)
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
	if status.NativeDSDBufferFrames == 0 {
		t.Errorf("expected negotiated Native DSD buffer frames")
	}
	supported := map[string]bool{}
	for _, capability := range status.NativeCapabilities {
		supported[capability.Format] = capability.Supported
	}
	if !supported["DSD256 (11.2896MHz)"] || !supported["DSD512 (22.5792MHz)"] {
		t.Errorf("expected DSD256/DSD512 capability detection, got %+v", status.NativeCapabilities)
	}
}

func TestAsioServiceDefaultsToPCMUntilExplicitNativeEntry(t *testing.T) {
	backend := asio.NewMockBackend(bridge.ProfileThesycon)
	if err := backend.Initialize(); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	defer backend.Close()

	svc := NewAsioService(backend)
	defer svc.Close()

	initial, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("initial GetStatus failed: %v", err)
	}
	if initial.NativeDSDMode || initial.NativeDSDActive || initial.NativeDSDConfirmed || initial.SelectedDSDFormat != "" {
		t.Fatalf("new ASIO service restored stale Native DSD state: %+v", initial)
	}

	if err := svc.SelectDriver(bridge.ProfileThesycon.Name); err != nil {
		t.Fatalf("SelectDriver failed: %v", err)
	}
	selected, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus after select failed: %v", err)
	}
	if selected.OperatingMode != "PCM" || selected.NativeDSDMode || selected.NativeDSDActive || selected.NativeDSDConfirmed {
		t.Fatalf("ASIO selection must remain in PCM, got %+v", selected)
	}
	if selected.SelectedDSDFormat != "" {
		t.Fatalf("ASIO selection exposed a pre-selected DSD format: %q", selected.SelectedDSDFormat)
	}
	if len(selected.NativeCapabilities) != 0 {
		t.Fatalf("ASIO selection must not probe Native DSD capabilities: %+v", selected.NativeCapabilities)
	}
	if rate, err := svc.driver.GetSampleRate(); err != nil || rate != 48000 {
		t.Fatalf("ASIO selection changed the PCM sample rate: rate=%v err=%v", rate, err)
	}

	if err := svc.ConfirmNativeDSDRisk("YES"); err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if err := svc.ConfigureNativeDSD("DSD256"); err != nil {
		t.Fatalf("explicit Native DSD entry failed: %v", err)
	}
	active, _ := svc.GetStatus()
	if !active.NativeDSDMode || !active.NativeDSDActive || active.OperatingMode != "Native DSD" {
		t.Fatalf("explicit Native DSD entry did not activate: %+v", active)
	}
	if active.SelectedDSDFormat != "DSD256 (11.2896MHz)" {
		t.Fatalf("unexpected selected Native DSD format: %q", active.SelectedDSDFormat)
	}

	if err := svc.DisableNativeDSD(); err != nil {
		t.Fatalf("DisableNativeDSD failed: %v", err)
	}
	restored, _ := svc.GetStatus()
	if restored.OperatingMode != "PCM" || restored.NativeDSDMode || restored.NativeDSDActive {
		t.Fatalf("Native DSD exit did not restore PCM: %+v", restored)
	}
	if restored.NativeDSDConfirmed || restored.SelectedDSDFormat != "" {
		t.Fatalf("Native DSD exit retained session authorization/format: %+v", restored)
	}
	if rate, err := svc.driver.GetSampleRate(); err != nil || rate != 48000 {
		t.Fatalf("Native DSD exit did not restore PCM sample rate: rate=%v err=%v", rate, err)
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

func TestAsioServiceUnsupportedNativeFormatStaysInPCM(t *testing.T) {
	backend := asio.NewMockBackend(bridge.ProfileYamaha)
	if err := backend.Initialize(); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	defer backend.Close()

	svc := NewAsioService(backend)
	defer svc.Close()
	if err := svc.SelectDriver(bridge.ProfileYamaha.Name); err != nil {
		t.Fatalf("SelectDriver failed: %v", err)
	}
	if err := svc.ConfirmNativeDSDRisk("YES"); err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if err := svc.ConfigureNativeDSD("DSD256"); err == nil {
		t.Fatalf("expected unsupported format error")
	}
	status, _ := svc.GetStatus()
	if status.NativeDSDMode || status.NativeDSDActive || status.OperatingMode != "PCM" {
		t.Fatalf("unsupported format must remain PCM/inactive: %+v", status)
	}
	if status.NativeDSDConfirmed || status.SelectedDSDFormat != "" {
		t.Fatalf("failed Native DSD entry retained authorization/format: %+v", status)
	}
	if svc.NativeStream() != nil {
		t.Fatalf("no NativeDSDStream may remain after unsupported selection")
	}
}

func TestParseNativeDSDFormatAcceptsUIAndProtocolLabels(t *testing.T) {
	tests := map[string]string{
		"DSD64":                  "DSD64",
		"dsd128":                 "DSD128",
		"256":                    "DSD256",
		"DSD256 (11.2896MHz)":    "DSD256",
		"DSD512 · 22.5792 MHz":   "DSD512",
		" DSD512\t(22.5792 MHz)": "DSD512",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			format, err := parseNativeDSDFormat(input)
			if err != nil {
				t.Fatalf("parseNativeDSDFormat(%q) failed: %v", input, err)
			}
			if got := format.ShortName(); got != want {
				t.Fatalf("parseNativeDSDFormat(%q) = %s, want %s", input, got, want)
			}
		})
	}
}
