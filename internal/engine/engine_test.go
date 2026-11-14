package engine

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/dsp"
)

type MockStream struct {
	bufferSize  uint32
	getBufferFn func(numFrames int) (uintptr, error)
	started     atomic.Bool
	closed      atomic.Bool
	dummyBuf    []byte
}

func NewMockStream(bufferSize uint32, bytesPerFrame int) *MockStream {
	return &MockStream{
		bufferSize: bufferSize,
		dummyBuf:   make([]byte, int(bufferSize)*bytesPerFrame),
	}
}

func (m *MockStream) Start() error {
	m.started.Store(true)
	return nil
}
func (m *MockStream) Stop() error {
	m.started.Store(false)
	return nil
}
func (m *MockStream) GetBuffer(numFrames int) (uintptr, error) {
	if m.getBufferFn != nil {
		return m.getBufferFn(numFrames)
	}
	return uintptr(0), nil
}
func (m *MockStream) ReleaseBuffer(numFramesWritten int, flags uint32) error {
	return nil
}
func (m *MockStream) GetBufferSize() (uint32, error) {
	return m.bufferSize, nil
}
func (m *MockStream) GetDevicePeriod() (time.Duration, time.Duration, error) {
	return 10 * time.Millisecond, 5 * time.Millisecond, nil
}
func (m *MockStream) GetWaitHandle() uintptr {
	return 0
}
func (m *MockStream) Close() error {
	m.closed.Store(true)
	return nil
}

type MockEngineDevice struct {
	stream *MockStream
}

func (d *MockEngineDevice) ID() string         { return "mock-engine-dev" }
func (d *MockEngineDevice) Name() string       { return "Mock Engine Device" }
func (d *MockEngineDevice) Flow() audio.DeviceFlow { return audio.FlowRender }
func (d *MockEngineDevice) State() audio.DeviceState { return audio.StateActive }
func (d *MockEngineDevice) ProbeCapabilities(candidates []audio.AudioFormat, shareMode audio.ShareMode) ([]audio.FormatSupportResult, error) {
	results := make([]audio.FormatSupportResult, len(candidates))
	for i, c := range candidates {
		results[i] = audio.FormatSupportResult{Format: c, Status: audio.StatusSupported}
	}
	return results, nil
}
func (d *MockEngineDevice) OpenStream(config audio.StreamConfig) (audio.AudioStream, error) {
	return d.stream, nil
}
func (d *MockEngineDevice) Close() error {
	return nil
}

func TestEngineLifecycleAndTransitions(t *testing.T) {
	eng := NewAudioEngine()

	mockStream := NewMockStream(480, 4) // 480 frames, 16-bit stereo = 4 bytes per frame
	dev := &MockEngineDevice{stream: mockStream}

	cfg := EngineConfig{
		Device: dev,
		Format: audio.AudioFormat{
			SampleRate:   48000,
			Channels:     2,
			SampleFormat: audio.SampleFormat16Int,
		},
		ShareMode:      audio.ShareModeExclusive,
		BufferDuration: 10 * time.Millisecond,
		FadeDurationMs: 10.0,
	}

	if err := eng.Open(cfg); err != nil {
		t.Fatalf("eng.Open failed: %v", err)
	}
	if eng.State() != StateOpenMuted {
		t.Fatalf("state want OpenMuted, got %v", eng.State())
	}

	gen := dsp.NewSineGenerator()
	_ = gen.Prepare(dsp.Config{SampleRate: 48000, Frequency: 1000.0, Amplitude: 0.5})

	if err := eng.Prime(gen); err != nil {
		t.Fatalf("eng.Prime failed: %v", err)
	}
	if eng.State() != StatePrimed {
		t.Fatalf("state want Primed, got %v", eng.State())
	}

	if err := eng.Start(); err != nil {
		t.Fatalf("eng.Start failed: %v", err)
	}
	if eng.State() != StateRunning {
		t.Fatalf("state want Running, got %v", eng.State())
	}

	time.Sleep(30 * time.Millisecond)

	diag := eng.Diagnostics()
	if diag.RenderCallbackCount == 0 {
		t.Errorf("expected render callbacks > 0, got %d", diag.RenderCallbackCount)
	}

	if err := eng.Stop(); err != nil {
		t.Fatalf("eng.Stop failed: %v", err)
	}
	if eng.State() != StateOpenMuted {
		t.Fatalf("state after Stop want OpenMuted, got %v", eng.State())
	}

	if err := eng.Close(); err != nil {
		t.Fatalf("eng.Close failed: %v", err)
	}
	if eng.State() != StateClosed {
		t.Fatalf("state after Close want Closed, got %v", eng.State())
	}
}

func TestFaultInjectionSafeMuteInOnePeriod(t *testing.T) {
	eng := NewAudioEngine()
	mockStream := NewMockStream(480, 4)
	dev := &MockEngineDevice{stream: mockStream}

	cfg := EngineConfig{
		Device: dev,
		Format: audio.AudioFormat{
			SampleRate:   48000,
			Channels:     2,
			SampleFormat: audio.SampleFormat16Int,
		},
		BufferDuration: 10 * time.Millisecond,
	}

	_ = eng.Open(cfg)
	gen := dsp.NewSineGenerator()
	_ = gen.Prepare(dsp.Config{SampleRate: 48000, Frequency: 1000, Amplitude: 0.5})
	_ = eng.Prime(gen)
	_ = eng.Start()

	// Inject fault
	injectTime := time.Now()
	eng.InjectFault(errors.New("injected hardware glitch"))

	// Must transition to Faulted/Muted within 1 device period (~10ms + small scheduling buffer < 25ms)
	deadline := time.Now().Add(25 * time.Millisecond)
	for time.Now().Before(deadline) {
		if eng.State() == StateFaulted {
			break
		}
		time.Sleep(1 * time.Millisecond)
	}

	elapsed := time.Since(injectTime)
	t.Logf("Fault muted within %v (State=%v)", elapsed, eng.State())

	if eng.State() != StateFaulted {
		t.Errorf("expected StateFaulted within 1 device period, got %v", eng.State())
	}
	if eng.fader.CurrentGain() != 0.0 {
		t.Errorf("fader gain should be 0.0 (muted), got %f", eng.fader.CurrentGain())
	}

	eng.Close()
}

func TestDeviceInvalidationUnplugResponseTime(t *testing.T) {
	eng := NewAudioEngine()
	mockStream := NewMockStream(480, 4)
	dev := &MockEngineDevice{stream: mockStream}

	cfg := EngineConfig{
		Device: dev,
		Format: audio.AudioFormat{
			SampleRate:   48000,
			Channels:     2,
			SampleFormat: audio.SampleFormat16Int,
		},
		BufferDuration: 10 * time.Millisecond,
	}

	_ = eng.Open(cfg)
	gen := dsp.NewSineGenerator()
	_ = gen.Prepare(dsp.Config{SampleRate: 48000, Frequency: 1000, Amplitude: 0.5})
	_ = eng.Prime(gen)
	_ = eng.Start()

	start := time.Now()
	eng.HandleDeviceInvalidated()
	elapsed := time.Since(start)

	t.Logf("HandleDeviceInvalidated returned in %v (Target < 50ms)", elapsed)
	if elapsed > 50*time.Millisecond {
		t.Errorf("HandleDeviceInvalidated took %v, exceeds 50ms target!", elapsed)
	}

	if eng.State() != StateFaulted {
		t.Errorf("state = %v, want Faulted", eng.State())
	}

	eng.Close()
}
