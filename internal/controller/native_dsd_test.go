package controller

import (
	"strings"
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio"
	"sound-signal-generator/internal/audio/asio/bridge"
	"sound-signal-generator/internal/dsd"
	"sound-signal-generator/internal/engine"
)

func setupAsioController(t *testing.T, profile bridge.VendorProfile) (*AppController, *bridge.MockDriver) {
	t.Helper()
	primaryDevice := &MockControllerDevice{stream: &MockControllerStream{bufferSize: 1920}}
	primary := &MockBackend{device: primaryDevice}
	asioBackend := asio.NewMockBackend(profile)
	ctrl := NewAppControllerWithAsioBackend(primary, asioBackend)
	if err := ctrl.Initialize(); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	t.Cleanup(func() { _ = ctrl.Close() })

	devices, err := ctrl.EnumerateDevices()
	if err != nil {
		t.Fatalf("enumerate failed: %v", err)
	}
	asioID := ""
	for _, dev := range devices {
		if dev.Backend == audio.BackendASIO {
			asioID = dev.ID
			break
		}
	}
	if asioID == "" {
		t.Fatalf("mock ASIO device not present in unified list: %+v", devices)
	}
	if err := ctrl.SelectDevice(asioID); err != nil {
		t.Fatalf("select ASIO device failed: %v", err)
	}
	asioDevice, ok := ctrl.selectedDevice.(*asio.Device)
	if !ok {
		t.Fatalf("selected device is not ASIO: %T", ctrl.selectedDevice)
	}
	driver, ok := asioDevice.Driver().(*bridge.MockDriver)
	if !ok {
		t.Fatalf("unexpected mock driver type: %T", asioDevice.Driver())
	}
	return ctrl, driver
}

func TestControllerASIOSelectionStartsInPCM(t *testing.T) {
	ctrl, _ := setupAsioController(t, bridge.ProfileThesycon)
	status, err := ctrl.AsioService().GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.OperatingMode != "PCM" || status.NativeDSDMode || status.NativeDSDActive || status.NativeDSDConfirmed {
		t.Fatalf("controller selected ASIO in Native DSD state: %+v", status)
	}
	if status.SelectedDSDFormat != "" {
		t.Fatalf("controller exposed a Native DSD format before explicit entry: %q", status.SelectedDSDFormat)
	}
	if ctrl.IsNativeDSDActive() {
		t.Fatalf("controller reports Native DSD active after ASIO selection")
	}
}

func TestNativeDSDSafetyRequiresExactYES(t *testing.T) {
	ctrl, _ := setupAsioController(t, bridge.ProfileThesycon)
	if err := ctrl.AsioService().ConfirmNativeDSDRisk("yes"); err != ErrInvalidConfirmationPhrase {
		t.Fatalf("lowercase phrase must be rejected, got %v", err)
	}
	if err := ctrl.AsioService().ConfirmNativeDSDRisk("YES "); err != ErrInvalidConfirmationPhrase {
		t.Fatalf("trailing whitespace must be rejected, got %v", err)
	}
	if ctrl.AsioService().NativeDSDConfirmed() {
		t.Fatalf("service confirmed after a phrase with trailing whitespace")
	}
	if err := ctrl.AsioService().ConfirmNativeDSDRisk("YES"); err != nil {
		t.Fatalf("exact YES should be accepted: %v", err)
	}
	if !ctrl.AsioService().NativeDSDConfirmed() {
		t.Fatalf("service did not record exact confirmation")
	}
}

func TestNativeDSDRealStreamCreationBufferWritesAndPCMRestore(t *testing.T) {
	ctrl, driver := setupAsioController(t, bridge.ProfileThesycon)

	pcmReq := DefaultExtendedSignalRequest()
	pcmReq.Waveform = WaveformSine
	pcmReq.SampleRate = 48000
	pcmReq.SampleFormat = audio.SampleFormat24In32
	pcmReq.Frequency = 1000
	pcmReq.Amplitude = 0.5
	pcmReq.LeftVolumeDBFS = 0
	pcmReq.RightVolumeDBFS = 0
	if err := ctrl.ConfigureExtendedSignal(pcmReq); err != nil {
		t.Fatalf("PCM configuration on ASIO failed: %v", err)
	}
	if ctrl.engine.State() != engine.StatePrimed {
		t.Fatalf("expected PCM ASIO primed state, got %s", ctrl.engine.State())
	}

	if err := ctrl.AsioService().ConfirmNativeDSDRisk("YES"); err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if err := ctrl.EnableNativeDSD("DSD256"); err != nil {
		t.Fatalf("EnableNativeDSD failed: %v", err)
	}
	if !ctrl.IsNativeDSDActive() {
		t.Fatalf("controller does not report Native DSD active")
	}
	if ctrl.AsioService().NativeStream() == nil {
		t.Fatalf("Native DSD stream was not created in production service path")
	}
	status, err := ctrl.AsioService().GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if !status.NativeDSDMode || !status.NativeDSDActive || status.OperatingMode != "Native DSD" {
		t.Fatalf("status did not report active Native DSD: %+v", status)
	}
	if !strings.Contains(status.SampleType, "DSDInt8") {
		t.Fatalf("unexpected Native DSD sample type: %s", status.SampleType)
	}
	if len(status.NativeCapabilities) < 4 {
		t.Fatalf("expected DSD64/128/256/512 capability report, got %+v", status.NativeCapabilities)
	}

	if err := ctrl.StartPlayback(); err != nil {
		t.Fatalf("Native DSD Start failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := ctrl.StopPlayback(); err != nil {
		t.Fatalf("Native DSD Stop failed: %v", err)
	}
	wroteDSD := driver.WrittenDSDBuffer(0, 0) != nil || driver.WrittenDSDBuffer(1, 0) != nil
	if !wroteDSD {
		t.Fatalf("ASIO driver did not receive planar Native DSD buffers")
	}

	if err := ctrl.DisableNativeDSD(); err != nil {
		t.Fatalf("DisableNativeDSD failed: %v", err)
	}
	if ctrl.IsNativeDSDActive() {
		t.Fatalf("controller still reports Native DSD active")
	}
	status, _ = ctrl.AsioService().GetStatus()
	if status.NativeDSDMode || status.OperatingMode != "PCM" {
		t.Fatalf("ASIO status did not return to PCM: %+v", status)
	}
	if ctrl.engine.State() != engine.StatePrimed {
		t.Fatalf("expected safe muted PCM restore in Primed state, got %s", ctrl.engine.State())
	}
}

func TestNativeDSDUnsupportedFormatFailsMutedAndNotActive(t *testing.T) {
	ctrl, _ := setupAsioController(t, bridge.ProfileYamaha)
	if err := ctrl.AsioService().ConfirmNativeDSDRisk("YES"); err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	err := ctrl.EnableNativeDSD("DSD256")
	if err == nil {
		t.Fatalf("expected unsupported DSD error")
	}
	if ctrl.IsNativeDSDActive() {
		t.Fatalf("unsupported Native DSD must not be reported active")
	}
	status, _ := ctrl.AsioService().GetStatus()
	if status.NativeDSDMode || status.NativeDSDActive {
		t.Fatalf("unsupported format incorrectly enabled: %+v", status)
	}
	if ctrl.engine.State() != engine.StateClosed {
		t.Fatalf("unsupported format must leave engine closed/muted, got %s", ctrl.engine.State())
	}
}

func TestProductionControllerUsesRealASIOBackend(t *testing.T) {
	ctrl := NewAppController(&MockBackend{device: &MockControllerDevice{stream: &MockControllerStream{bufferSize: 480}}})
	if ctrl.asioService.backend == nil {
		t.Fatalf("ASIO backend missing")
	}
	if ctrl.asioService.backend.IsMock() {
		t.Fatalf("production controller silently selected a mock ASIO backend")
	}
}

func TestNativeDSDExitPathsClearControllerState(t *testing.T) {
	enableNative := func(t *testing.T, ctrl *AppController) {
		t.Helper()
		if err := ctrl.AsioService().ConfirmNativeDSDRisk("YES"); err != nil {
			t.Fatalf("confirm failed: %v", err)
		}
		if err := ctrl.EnableNativeDSD("DSD256"); err != nil {
			t.Fatalf("EnableNativeDSD failed: %v", err)
		}
		if !ctrl.IsNativeDSDActive() {
			t.Fatalf("Native DSD was not active before transition")
		}
	}
	assertCleared := func(t *testing.T, ctrl *AppController) {
		t.Helper()
		if ctrl.IsNativeDSDActive() || ctrl.nativeDSDFormat != 0 {
			t.Fatalf("controller retained stale Native DSD state: active=%t format=%v", ctrl.nativeDSDActive, ctrl.nativeDSDFormat)
		}
		status, err := ctrl.AsioService().GetStatus()
		if err != nil {
			t.Fatalf("GetStatus failed: %v", err)
		}
		if status.NativeDSDMode || status.NativeDSDActive || (status.OperatingMode != "PCM" && status.OperatingMode != "Unavailable") {
			t.Fatalf("ASIO service retained Native DSD mode: %+v", status)
		}
		if status.NativeDSDConfirmed || status.SelectedDSDFormat != "" {
			t.Fatalf("ASIO service retained stale Native DSD authorization/format: %+v", status)
		}
	}

	t.Run("ASIO reselection", func(t *testing.T) {
		ctrl, _ := setupAsioController(t, bridge.ProfileThesycon)
		enableNative(t, ctrl)
		if err := ctrl.SelectAsioDriver(ctrl.selectedDeviceKey); err != nil {
			t.Fatalf("SelectAsioDriver after Native DSD failed: %v", err)
		}
		assertCleared(t, ctrl)
	})

	t.Run("extended PCM", func(t *testing.T) {
		ctrl, _ := setupAsioController(t, bridge.ProfileThesycon)
		enableNative(t, ctrl)
		req := DefaultExtendedSignalRequest()
		req.DeviceID = ctrl.selectedDeviceKey
		req.Waveform = WaveformSine
		req.SampleRate = 48000
		req.SampleFormat = audio.SampleFormat24In32
		req.Channels = 2
		if err := ctrl.ConfigureExtendedSignal(req); err != nil {
			t.Fatalf("ConfigureExtendedSignal after Native DSD failed: %v", err)
		}
		assertCleared(t, ctrl)
	})

	t.Run("legacy PCM", func(t *testing.T) {
		ctrl, _ := setupAsioController(t, bridge.ProfileThesycon)
		enableNative(t, ctrl)
		req := SignalRequest{
			DeviceID:     ctrl.selectedDeviceKey,
			SampleRate:   48000,
			SampleFormat: audio.SampleFormat24In32,
			Channels:     2,
			Frequency:    1000,
			Amplitude:    0.25,
		}
		if err := ctrl.ConfigureSignal(req); err != nil {
			t.Fatalf("ConfigureSignal after Native DSD failed: %v", err)
		}
		assertCleared(t, ctrl)
	})

	t.Run("DoP", func(t *testing.T) {
		ctrl, _ := setupAsioController(t, bridge.ProfileThesycon)
		enableNative(t, ctrl)
		ctrl.DoPGuard().ConfirmWithToken(DoPConfirmationToken)
		req := DoPRequest{
			DeviceID:     ctrl.selectedDeviceKey,
			DSDFormat:    dsd.DSD64,
			SampleFormat: audio.SampleFormat24In32,
			Channels:     2,
			BitOrder:     dsd.BitOrderMSBFirst,
		}
		if err := ctrl.ConfigureDoP(req); err != nil {
			t.Fatalf("ConfigureDoP after Native DSD failed: %v", err)
		}
		assertCleared(t, ctrl)
	})

	t.Run("device switch", func(t *testing.T) {
		ctrl, _ := setupAsioController(t, bridge.ProfileThesycon)
		enableNative(t, ctrl)
		devices, err := ctrl.EnumerateDevices()
		if err != nil {
			t.Fatalf("EnumerateDevices failed: %v", err)
		}
		nextID := ""
		for _, dev := range devices {
			if dev.Backend != audio.BackendASIO {
				nextID = dev.ID
				break
			}
		}
		if nextID == "" {
			t.Fatalf("no non-ASIO device available for switch test: %+v", devices)
		}
		if err := ctrl.SelectDevice(nextID); err != nil {
			t.Fatalf("SelectDevice after Native DSD failed: %v", err)
		}
		assertCleared(t, ctrl)
	})
}

func TestCancelNativeDSDEntryRevokesConfirmation(t *testing.T) {
	ctrl, _ := setupAsioController(t, bridge.ProfileThesycon)
	if err := ctrl.AsioService().ConfirmNativeDSDRisk("YES"); err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if err := ctrl.CancelNativeDSDEntry(); err != nil {
		t.Fatalf("CancelNativeDSDEntry failed: %v", err)
	}
	if ctrl.AsioService().NativeDSDConfirmed() {
		t.Fatalf("cancelled Native DSD entry retained safety confirmation")
	}
	status, _ := ctrl.AsioService().GetStatus()
	if status.NativeDSDMode || status.OperatingMode != "PCM" || status.SelectedDSDFormat != "" {
		t.Fatalf("cancelled Native DSD entry did not remain PCM: %+v", status)
	}
}

func TestAsioControlChangesAreRejectedWithoutStoppingNativeDSD(t *testing.T) {
	ctrl, _ := setupAsioController(t, bridge.ProfileThesycon)
	if err := ctrl.AsioService().ConfirmNativeDSDRisk("YES"); err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if err := ctrl.EnableNativeDSD("DSD256"); err != nil {
		t.Fatalf("EnableNativeDSD failed: %v", err)
	}
	if err := ctrl.StartPlayback(); err != nil {
		t.Fatalf("StartPlayback failed: %v", err)
	}
	defer func() {
		if err := ctrl.StopPlayback(); err != nil {
			t.Errorf("StopPlayback failed: %v", err)
		}
	}()

	before := ctrl.AsioService().BufferSize()
	if err := ctrl.SetAsioBufferSize(128); err == nil {
		t.Fatalf("SetAsioBufferSize succeeded while Native DSD was active")
	}
	if ctrl.engine.State() != engine.StateRunning {
		t.Fatalf("buffer rejection closed the active stream: state=%s", ctrl.engine.State())
	}
	if after := ctrl.AsioService().BufferSize(); after != before {
		t.Fatalf("buffer size changed despite rejection: before=%d after=%d", before, after)
	}

	if err := ctrl.SetAsioClockSource(1); err == nil {
		t.Fatalf("SetAsioClockSource succeeded while Native DSD was active")
	}
	if ctrl.engine.State() != engine.StateRunning {
		t.Fatalf("clock rejection closed the active stream: state=%s", ctrl.engine.State())
	}
	if !ctrl.IsNativeDSDActive() {
		t.Fatalf("Native DSD state was lost after rejected control changes")
	}
}
