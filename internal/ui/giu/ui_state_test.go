package giuui

import (
	"math"
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/dsd"
	"sound-signal-generator/internal/engine"
)

func TestUIState_Defaults(t *testing.T) {
	state := NewUIState()

	if state.Frequency != 1000.0 {
		t.Errorf("expected default frequency 1000Hz, got %f", state.Frequency)
	}
	if state.LeftVolumeDBFS != -6.0 || state.RightVolumeDBFS != -6.0 {
		t.Errorf("expected default dBFS -6.0, got L=%f, R=%f", state.LeftVolumeDBFS, state.RightVolumeDBFS)
	}
	if !state.LinkChannels {
		t.Errorf("expected default LinkChannels to be true")
	}
	if !state.IsExclusive {
		t.Errorf("expected default IsExclusive to be true")
	}
	if state.SelectedSampleRateIndex != 1 { // 48000 Hz
		t.Errorf("expected default sample rate index 1 (48000 Hz), got %d", state.SelectedSampleRateIndex)
	}
	if state.SelectedSampleFormatIndex != 0 { // 16-bit Int
		t.Errorf("expected default format index 0 (16-bit), got %d", state.SelectedSampleFormatIndex)
	}
	if state.DoPFormatIndex != 0 || state.SelectedDoPFormat() != dsd.DSD64 {
		t.Errorf("expected default DoP format DSD64, got index=%d format=%v", state.DoPFormatIndex, state.SelectedDoPFormat())
	}
	if state.DoPBitOrderIndex != 0 || state.SelectedDoPBitOrder() != dsd.BitOrderMSBFirst {
		t.Errorf("expected default DoP bit order MSB First, got index=%d order=%v", state.DoPBitOrderIndex, state.SelectedDoPBitOrder())
	}
}

func TestUIState_OutputRunningLocksFromEitherSignal(t *testing.T) {
	state := NewUIState()
	if state.IsOutputRunning() {
		t.Fatal("new UI state must not report active output")
	}

	state.IsActiveStream = true
	if !state.IsOutputRunning() {
		t.Fatal("transport flag should lock output format controls")
	}

	state.IsActiveStream = false
	state.Diagnostics.State = engine.StateRunning
	if !state.IsOutputRunning() {
		t.Fatal("diagnostics state should lock output format controls")
	}
}

func TestGiuApp_BuildExtendedRequest(t *testing.T) {
	app := &GiuApp{
		state: NewUIState(),
	}
	app.state.DeviceIDs = []string{"test-device-id-123"}
	app.state.SelectedDeviceIndex = 0
	app.state.SelectedSampleRateIndex = 3   // 96000 Hz
	app.state.SelectedSampleFormatIndex = 3 // 32-bit Float
	app.state.SelectedWaveformIndex = 0     // Pure Sine
	app.state.Frequency = 2400.0
	app.state.LeftVolumeDBFS = -12.0
	app.state.RightVolumeDBFS = -12.0

	req := app.BuildExtendedRequest()

	if req.DeviceID != "test-device-id-123" {
		t.Errorf("expected DeviceID 'test-device-id-123', got '%s'", req.DeviceID)
	}
	if req.SampleRate != 96000 {
		t.Errorf("expected SampleRate 96000, got %d", req.SampleRate)
	}
	if req.SampleFormat != audio.SampleFormat32Float {
		t.Errorf("expected SampleFormat 32-bit float, got %s", req.SampleFormat)
	}
	if req.Waveform != controller.WaveformSine {
		t.Errorf("expected Waveform Pure Sine, got %s", req.Waveform)
	}
	if req.Frequency != 2400.0 {
		t.Errorf("expected Frequency 2400.0, got %f", req.Frequency)
	}

	expectedAmp := math.Pow(10, -12.0/20.0)
	if math.Abs(req.Amplitude-expectedAmp) > 0.001 {
		t.Errorf("expected amplitude ~%f, got %f", expectedAmp, req.Amplitude)
	}
}

func TestGiuApp_BuildExtendedRequest_DoPHardConstraints(t *testing.T) {
	app := &GiuApp{state: NewUIState()}
	app.state.DeviceIDs = []string{"dop-device"}
	app.state.SelectedDeviceIndex = 0

	for i, option := range WaveformOptions {
		if option.ID == controller.WaveformDoP {
			app.state.SelectedWaveformIndex = int32(i)
			break
		}
	}
	app.state.DoPFormatIndex = 1   // DSD128
	app.state.DoPBitOrderIndex = 1 // LSB First
	app.state.SelectedSampleRateIndex = 1
	app.state.SelectedSampleFormatIndex = 0
	app.state.LeftVolumeDBFS = -12
	app.state.RightVolumeDBFS = -30
	app.state.MasterTrimDB = 4.5

	app.state.ApplyDoPConstraints()
	req := app.BuildExtendedRequest()

	if req.SampleRate != 352800 {
		t.Errorf("expected DSD128 carrier rate 352800, got %d", req.SampleRate)
	}
	if req.SampleFormat != audio.SampleFormat24In32 {
		t.Errorf("expected 24-in-32 DoP container, got %s", req.SampleFormat)
	}
	if req.Amplitude != 1.0 || req.LeftVolumeDBFS != 0.0 || req.RightVolumeDBFS != 0.0 {
		t.Errorf("DoP must be unity gain, got amp=%f L=%f R=%f", req.Amplitude, req.LeftVolumeDBFS, req.RightVolumeDBFS)
	}
	if req.DoP.DSDFormat != dsd.DSD128 {
		t.Errorf("expected DSD128 request, got %v", req.DoP.DSDFormat)
	}
	if req.DoP.BitOrder != dsd.BitOrderLSBFirst {
		t.Errorf("expected LSB First request, got %v", req.DoP.BitOrder)
	}
	if app.state.SelectedSampleRateIndex != 6 {
		t.Errorf("expected UI sample-rate index 6 (352.8 kHz), got %d", app.state.SelectedSampleRateIndex)
	}
	if app.state.SelectedSampleFormatIndex != 2 {
		t.Errorf("expected UI sample-format index 2 (24-in-32), got %d", app.state.SelectedSampleFormatIndex)
	}
	if app.state.LeftVolumeDBFS != -12 || app.state.RightVolumeDBFS != -30 {
		t.Errorf("DoP must not destroy PCM level preferences, got L=%f R=%f", app.state.LeftVolumeDBFS, app.state.RightVolumeDBFS)
	}
}

func TestGeneratorPanel_NyquistCheck(t *testing.T) {
	state := NewUIState()
	state.SelectedSampleRateIndex = 1 // 48000 Hz, Nyquist safe limit = 48000 * 0.45 = 21600 Hz

	panel := NewGeneratorPanel(nil, state, func() {}, nil)

	maxSafe := panel.GetMaxSafeFrequency()
	if maxSafe != 21600.0 {
		t.Fatalf("expected max safe frequency 21600.0, got %f", maxSafe)
	}

	// 1. Safe frequency
	state.SelectedWaveformIndex = 0 // Sine
	state.Frequency = 1000.0
	if panel.IsNyquistExceeded() {
		t.Errorf("1000 Hz should not violate Nyquist ceiling")
	}

	// 2. Frequency exceeding limit
	state.Frequency = 22000.0
	if !panel.IsNyquistExceeded() {
		t.Errorf("22000 Hz must violate 21600 Hz Nyquist ceiling")
	}

	// 3. Sweep frequency exceeding limit
	state.SelectedWaveformIndex = 3 // Linear sweep
	state.SweepStartFreq = 20.0
	state.SweepEndFreq = 22000.0
	if !panel.IsNyquistExceeded() {
		t.Errorf("Sweep end freq 22000 Hz must violate Nyquist ceiling")
	}

	state.SweepEndFreq = 20000.0
	if panel.IsNyquistExceeded() {
		t.Errorf("Sweep end freq 20000 Hz should not violate Nyquist ceiling")
	}
}

func TestGeneratorPanel_CorrectFrequencyInput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		current float64
		max     float64
		want    float64
	}{
		{name: "precise value", input: "438", current: 440, max: 21600, want: 438},
		{name: "decimal value", input: "438.5", current: 440, max: 21600, want: 438.5},
		{name: "hz suffix", input: " 438.25Hz ", current: 440, max: 21600, want: 438.25},
		{name: "below minimum", input: "0", current: 440, max: 21600, want: 1},
		{name: "negative", input: "-20", current: 440, max: 21600, want: 1},
		{name: "above maximum", input: "99999", current: 440, max: 21600, want: 21600},
		{name: "invalid restores current", input: "not-a-number", current: 440, max: 21600, want: 440},
		{name: "empty restores current", input: "   ", current: 440, max: 21600, want: 440},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := correctFrequencyInput(tc.input, tc.current, tc.max)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("correctFrequencyInput(%q) = %f, want %f", tc.input, got, tc.want)
			}
		})
	}
}

func TestGeneratorPanel_FormatEditableFrequency(t *testing.T) {
	tests := map[float64]string{
		440:    "440",
		438.5:  "438.5",
		1000:   "1000",
		20000:  "20000",
		438.25: "438.25",
	}
	for value, want := range tests {
		if got := formatEditableFrequency(value); got != want {
			t.Errorf("formatEditableFrequency(%f) = %q, want %q", value, got, want)
		}
	}
}

func TestScopeWidget_Synthesis(t *testing.T) {
	state := NewUIState()
	state.LeftVolumeDBFS = 0.0
	state.RightVolumeDBFS = 0.0
	state.SelectedWaveformIndex = 0 // Sine

	scope := NewScopeWidget(state)
	lTrace, rTrace := scope.GenerateTraces(200)

	if len(lTrace) != 200 || len(rTrace) != 200 {
		t.Fatalf("expected 200 points, got L=%d, R=%d", len(lTrace), len(rTrace))
	}

	// Normal stereo: L and R should be identical for sine
	if math.Abs(lTrace[50]-rTrace[50]) > 0.0001 {
		t.Errorf("normal stereo should have matching L and R, got %f vs %f", lTrace[50], rTrace[50])
	}

	// Differential test: Invert R 180°
	state.SelectedChannelMode = int32(ChannelModeInvertR)
	lInv, rInv := scope.GenerateTraces(200)
	if math.Abs(lInv[50]+rInv[50]) > 0.0001 {
		t.Errorf("differential mode should have inverted R trace, got L=%f, R=%f", lInv[50], rInv[50])
	}
}

func TestScopeWidget_IsGatedByOutputState(t *testing.T) {
	state := NewUIState()
	state.SelectedWaveformIndex = 5 // White noise exercises the animation path.
	scope := NewScopeWidget(state)

	if scope.showLiveTrace() {
		t.Fatal("new scope must not display a trace before output starts")
	}
	if scope.NeedsAnimation() {
		t.Fatal("standby scope must not request preview animation")
	}

	state.IsActiveStream = true
	if !scope.showLiveTrace() {
		t.Fatal("active transport should enable the live trace")
	}
	if !scope.NeedsAnimation() {
		t.Fatal("active noise trace should request preview animation")
	}

	state.IsActiveStream = false
	state.Diagnostics.State = engine.StateRunning
	if !scope.showLiveTrace() {
		t.Fatal("running diagnostics state should also enable the live trace")
	}
}

func TestScopeWidget_TimeAxisTracksFrequency(t *testing.T) {
	state := NewUIState()
	state.SelectedWaveformIndex = 0   // Sine
	state.SelectedSampleRateIndex = 1 // 48 kHz
	state.Frequency = 1000

	scope := NewScopeWidget(state)
	frame1k := scope.GenerateFrame(301)

	if math.Abs(frame1k.DurationSec-0.003) > 1e-12 {
		t.Fatalf("1 kHz should display 3 periods (3 ms), got %.12f s", frame1k.DurationSec)
	}
	if len(frame1k.X) != 301 || len(frame1k.Left) != 301 || len(frame1k.Right) != 301 {
		t.Fatalf("unexpected frame lengths: X=%d L=%d R=%d", len(frame1k.X), len(frame1k.Left), len(frame1k.Right))
	}
	if math.Abs(frame1k.X[0]) > 1e-15 || math.Abs(frame1k.X[len(frame1k.X)-1]-frame1k.DurationSec) > 1e-12 {
		t.Errorf("X axis must span [0, window duration]")
	}
	for i := 1; i < len(frame1k.X); i++ {
		if frame1k.X[i] <= frame1k.X[i-1] {
			t.Fatalf("X axis must be strictly increasing at index %d", i)
		}
	}

	state.Frequency = 10000
	frame10k := scope.GenerateFrame(301)
	if math.Abs(frame10k.DurationSec-0.0003) > 1e-12 {
		t.Fatalf("10 kHz should display 3 periods (300 us), got %.12f s", frame10k.DurationSec)
	}
}

func TestScopeWidget_WheelZoomAndReset(t *testing.T) {
	state := NewUIState()
	state.SelectedWaveformIndex = 0 // Sine
	state.Frequency = 1000

	scope := NewScopeWidget(state)
	base := scope.GenerateFrame(101).DurationSec
	if !scope.XZoomIsDefault() {
		t.Fatal("scope must start at the default X zoom")
	}

	scope.applyXZoomWheel(1)
	zoomedIn := scope.GenerateFrame(101).DurationSec
	if scope.XZoomIsDefault() {
		t.Fatal("wheel up should leave the default X zoom")
	}
	if math.Abs(zoomedIn-base*scopeWheelZoomStep) > 1e-12 {
		t.Fatalf("wheel up should shrink X window to %.2fx, got base %.9f -> %.9f", scopeWheelZoomStep, base, zoomedIn)
	}

	scope.ResetXZoom()
	if !scope.XZoomIsDefault() {
		t.Fatal("reset should restore the default X zoom")
	}

	scope.applyXZoomWheel(-1000)
	if math.Abs(scope.XZoom()-scopeMaxXZoom) > 1e-6 {
		t.Fatalf("zoom out must clamp at %.2f, got %.6f", scopeMaxXZoom, scope.XZoom())
	}
	scope.applyXZoomWheel(1000)
	if math.Abs(scope.XZoom()-scopeMinXZoom) > 1e-6 {
		t.Fatalf("zoom in must clamp at %.2f, got %.6f", scopeMinXZoom, scope.XZoom())
	}
}

func TestScopeWidget_NoiseAnimates(t *testing.T) {
	state := NewUIState()
	state.SelectedWaveformIndex = 5 // White noise
	scope := NewScopeWidget(state)

	first := scope.GenerateFrame(256)
	second := scope.GenerateFrame(256)

	changed := false
	for i := range first.Left {
		if math.Abs(first.Left[i]-second.Left[i]) > 1e-12 {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("noise preview should advance its PRNG state between frames")
	}
}

func TestScopeWidget_SweepAnimates(t *testing.T) {
	state := NewUIState()
	state.SelectedWaveformIndex = 3 // Linear sweep
	state.SweepStartFreq = 20
	state.SweepEndFreq = 20000
	state.SweepDurationSec = 5
	state.SweepLoop = true
	state.StreamStartTime = time.Time{}

	scope := NewScopeWidget(state)
	clock := time.Unix(0, 0)
	scope.now = func() time.Time { return clock }
	scope.previewStart = clock

	first := scope.GenerateFrame(256)
	clock = clock.Add(500 * time.Millisecond)
	second := scope.GenerateFrame(256)

	if math.Abs(first.DurationSec-second.DurationSec) > 1e-15 {
		t.Fatalf("sweep timebase must stay stable across frames, got %.9fs -> %.9fs", first.DurationSec, second.DurationSec)
	}
	if first.AxisLabel == second.AxisLabel {
		t.Fatal("sweep status label should update with sweep progress")
	}
	changed := false
	for i := range first.Left {
		if math.Abs(first.Left[i]-second.Left[i]) > 1e-12 {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("sweep preview should advance with elapsed playback time")
	}
	if crossings := countZeroCrossings(second.Left); crossings <= countZeroCrossings(first.Left) {
		t.Fatalf("higher sweep frequency should visibly increase waveform density, crossings %d -> %d", countZeroCrossings(first.Left), crossings)
	}
}

func TestScopeWidget_SweepUsesSignalClockNotStreamClock(t *testing.T) {
	state := NewUIState()
	state.SelectedWaveformIndex = 3 // Linear sweep
	state.SweepStartFreq = 20
	state.SweepEndFreq = 20000
	state.SweepDurationSec = 5
	state.SweepLoop = true

	clock := time.Unix(100, 0)
	state.StreamStartTime = clock
	state.SignalStartTime = clock
	state.Diagnostics.BufferPeriodMs = 10

	scope := NewScopeWidget(state)
	scope.now = func() time.Time { return clock }

	clock = clock.Add(2 * time.Second)
	if got := scope.previewElapsed(clock); math.Abs(got-1.99) > 1e-9 {
		t.Fatalf("preview should follow signal clock with buffer compensation, got %.9fs", got)
	}

	// Simulate a live sweep edit: DSP sample zero restarts while stream uptime
	// continues. The scope must restart too instead of jumping to stream age.
	state.SignalStartTime = clock
	if got := scope.previewElapsed(clock); got != 0 {
		t.Fatalf("preview should restart with the DSP source, got %.9fs", got)
	}
	clock = clock.Add(10 * time.Millisecond)
	if got := scope.previewElapsed(clock); got != 0 {
		t.Fatalf("buffer latency must hold the preview at sample zero until output starts, got %.9fs", got)
	}
}

func countZeroCrossings(samples []float64) int {
	crossings := 0
	for i := 1; i < len(samples); i++ {
		if samples[i-1] == 0 || samples[i] == 0 {
			continue
		}
		if (samples[i-1] < 0) != (samples[i] < 0) {
			crossings++
		}
	}
	return crossings
}

func TestBenchPresets_Application(t *testing.T) {
	state := NewUIState()
	applied := false
	bar := NewBenchPresetsBar(state, func() {
		applied = true
	})

	// Apply 1kHz -1dBFS
	preset := StandardBenchPresets[1]
	bar.ApplyPreset(preset)

	if !applied {
		t.Errorf("expected preset callback to trigger")
	}
	if state.LeftVolumeDBFS != -1.0 || state.RightVolumeDBFS != -1.0 {
		t.Errorf("expected volume -1.0 dBFS, got L=%f, R=%f", state.LeftVolumeDBFS, state.RightVolumeDBFS)
	}
}

func TestOptions_LabelsConsistency(t *testing.T) {
	if len(SampleRateOptions) != len(SampleRateLabels) {
		t.Errorf("mismatch in SampleRateOptions (%d) and SampleRateLabels (%d)", len(SampleRateOptions), len(SampleRateLabels))
	}
	if len(SampleFormatOptions) != len(SampleFormatLabels) {
		t.Errorf("mismatch in SampleFormatOptions (%d) and SampleFormatLabels (%d)", len(SampleFormatOptions), len(SampleFormatLabels))
	}
	if len(WaveformOptions) == 0 {
		t.Errorf("WaveformOptions cannot be empty")
	}
}

func TestPreferredDeviceIndexUsesActiveWASAPIDefault(t *testing.T) {
	devices := []audio.DeviceInfo{
		{ID: "asio-default", Backend: audio.BackendASIO, State: audio.StateActive, IsDefault: true},
		{ID: "wasapi-inactive", Backend: audio.BackendWASAPI, State: audio.StateUnplugged, IsDefault: true},
		{ID: "wasapi-active", Backend: audio.BackendWASAPI, State: audio.StateActive},
	}
	if got := preferredDeviceIndex(devices); got != 2 {
		t.Fatalf("expected active WASAPI fallback index 2, got %d", got)
	}

	devices[2].IsDefault = true
	if got := preferredDeviceIndex(devices); got != 2 {
		t.Fatalf("expected active WASAPI default index 2, got %d", got)
	}

	devices = devices[:1]
	if got := preferredDeviceIndex(devices); got != 0 {
		t.Fatalf("expected ASIO fallback when it is the only active device, got %d", got)
	}
}

func TestUIState_NativeDSDTestSurfaceRequiresActiveSession(t *testing.T) {
	state := NewUIState()
	if state.NativeDSDTestVisible() || state.NativeDSDConfirmed || state.ShowNativeDSDConfirm {
		t.Fatalf("new UI state must start in PCM with Native DSD entry closed")
	}

	state.DeviceNames = []string{"ASIO DAC"}
	state.DeviceIDs = []string{"ASIO::one"}
	state.DeviceBackends = []audio.BackendType{audio.BackendASIO}
	state.SelectedDeviceIndex = 0
	if state.NativeDSDTestVisible() {
		t.Fatalf("selecting an ASIO device must not expose the Native DSD test surface")
	}

	state.NativeDSDConfirmed = true
	state.ShowNativeDSDConfirm = true
	if state.NativeDSDTestVisible() {
		t.Fatalf("a pending safety confirmation alone must not expose the DSD surface")
	}

	state.AsioStatus.NativeDSDMode = true
	state.AsioStatus.NativeDSDActive = true
	state.AsioStatus.NativeDSDConfirmed = true
	state.AsioStatus.SelectedDSDFormat = "DSD256 (11.2896MHz)"
	if !state.NativeDSDTestVisible() {
		t.Fatalf("explicitly activated Native DSD should expose the DSD test surface")
	}

	state.ResetNativeDSDSession()
	if state.NativeDSDTestVisible() || state.NativeDSDConfirmed || state.ShowNativeDSDConfirm || state.AsioStatus.SelectedDSDFormat != "" {
		t.Fatalf("leaving Native DSD must clear all transient DSD UI state: %+v", state)
	}
}

func TestUIState_ASIOAndNativeDSDSelection(t *testing.T) {
	state := NewUIState()
	state.DeviceNames = []string{"WASAPI DAC", "ASIO DAC"}
	state.DeviceIDs = []string{"WASAPI::one", "ASIO::two"}
	state.DeviceBackends = []audio.BackendType{audio.BackendWASAPI, audio.BackendASIO}
	state.SelectedDeviceIndex = 1

	if !state.IsAsioSelected() {
		t.Fatalf("expected selected unified device to report ASIO")
	}
	if state.SelectedNativeDSDFormat() != dsd.DSD256 {
		t.Fatalf("expected default native DSD256, got %v", state.SelectedNativeDSDFormat())
	}
	state.NativeDSDFormatIndex = 1
	if state.SelectedNativeDSDFormat() != dsd.DSD512 {
		t.Fatalf("expected native DSD512, got %v", state.SelectedNativeDSDFormat())
	}
}

func TestAdjustFrequencyForWheel(t *testing.T) {
	tests := []struct {
		name    string
		current float64
		min     float64
		max     float64
		wheel   float64
		want    float64
	}{
		{name: "up one notch", current: 1000, min: 1, max: 20000, wheel: 1, want: 1100},
		{name: "down one notch", current: 1000, min: 1, max: 20000, wheel: -1, want: 909},
		{name: "low frequency precision", current: 5, min: 1, max: 20000, wheel: 1, want: 5.5},
		{name: "clamp upper", current: 20000, min: 1, max: 20000, wheel: 1, want: 20000},
		{name: "clamp lower", current: 1, min: 1, max: 20000, wheel: -1, want: 1},
		{name: "zero wheel", current: 440, min: 1, max: 20000, wheel: 0, want: 440},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := adjustFrequencyForWheel(tc.current, tc.min, tc.max, tc.wheel)
			if math.Abs(got-tc.want) > 1e-6 {
				t.Fatalf("adjustFrequencyForWheel() = %f, want %f", got, tc.want)
			}
		})
	}
}
