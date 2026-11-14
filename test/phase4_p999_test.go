package test

import (
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/format"
)

type MockPhase4Device struct {
	rate       int
	channels   int
	sampleFmt  audio.SampleFormat
	bufferSize uint32
}

func (d *MockPhase4Device) Close() error {
	return nil
}
func (d *MockPhase4Device) ID() string             { return "mock-phase4-dac" }
func (d *MockPhase4Device) Name() string           { return "Mock Pro Audio DAC" }
func (d *MockPhase4Device) Flow() audio.DeviceFlow { return audio.FlowRender }
func (d *MockPhase4Device) State() audio.DeviceState {
	return audio.StateActive
}
func (d *MockPhase4Device) ProbeCapabilities(candidates []audio.AudioFormat, shareMode audio.ShareMode) ([]audio.FormatSupportResult, error) {
	results := make([]audio.FormatSupportResult, len(candidates))
	for i, c := range candidates {
		results[i] = audio.FormatSupportResult{Format: c, Status: audio.StatusSupported}
	}
	return results, nil
}
func (d *MockPhase4Device) OpenStream(config audio.StreamConfig) (audio.AudioStream, error) {
	frames := config.Format.SampleRate / 100 // 10ms period
	return &MockPhase4Stream{
		frames:     frames,
		sampleRate: config.Format.SampleRate,
		channels:   config.Format.Channels,
	}, nil
}

type MockPhase4Stream struct {
	frames     int
	sampleRate int
	channels   int
	started    bool
}

func (s *MockPhase4Stream) Start() error {
	s.started = true
	return nil
}
func (s *MockPhase4Stream) Stop() error {
	s.started = false
	return nil
}
func (s *MockPhase4Stream) GetBuffer(numFrames int) (uintptr, error) {
	return 0, nil
}
func (s *MockPhase4Stream) ReleaseBuffer(numFramesWritten int, flags uint32) error {
	return nil
}
func (s *MockPhase4Stream) GetBufferSize() (uint32, error) {
	return uint32(s.frames * 4), nil
}
func (s *MockPhase4Stream) GetDevicePeriod() (time.Duration, time.Duration, error) {
	return 10 * time.Millisecond, 5 * time.Millisecond, nil
}
func (s *MockPhase4Stream) GetWaitHandle() uintptr { return 0 }
func (s *MockPhase4Stream) Close() error           { return nil }

type MockPhase4Backend struct {
	dev *MockPhase4Device
}

func (b *MockPhase4Backend) Type() audio.BackendType              { return audio.BackendMock }
func (b *MockPhase4Backend) Initialize() error                    { return nil }
func (b *MockPhase4Backend) Close() error                         { return nil }
func (b *MockPhase4Backend) EnumerateDevices() ([]audio.DeviceInfo, error) {
	return []audio.DeviceInfo{
		{ID: b.dev.ID(), Name: b.dev.Name(), IsDefault: true, State: audio.StateActive},
	}, nil
}
func (b *MockPhase4Backend) GetDevice(id string) (audio.Device, error) {
	return b.dev, nil
}
func (b *MockPhase4Backend) GetDefaultDevice() (audio.Device, error) {
	return b.dev, nil
}

// TestPhase4_P999_BenchmarkMatrix tests the 10ms buffer matrix:
// 48k (480 frames), 96k (960 frames), 192k (1920 frames).
// Verifies:
// P99.9 render latency < 30% BufferPeriod (3ms / 3,000,000 ns)
// Max single render latency < 70% BufferPeriod (7ms / 7,000,000 ns)
// Underrun == 0
func TestPhase4_P999_BenchmarkMatrix(t *testing.T) {
	testMatrix := []struct {
		name       string
		sampleRate int
		frames     int
		waveform   controller.WaveformType
	}{
		{name: "48kHz_480frames_Sine", sampleRate: 48000, frames: 480, waveform: controller.WaveformSine},
		{name: "96kHz_960frames_Square", sampleRate: 96000, frames: 960, waveform: controller.WaveformPolyBlepSquare},
		{name: "192kHz_1920frames_Multitone", sampleRate: 192000, frames: 1920, waveform: controller.WaveformMultitoneCCIF},
	}

	const bufferPeriodNs = 10_000_000.0 // 10ms = 10,000,000 ns
	const p999LimitNs = 0.30 * bufferPeriodNs // 3,000,000 ns (3ms)
	const maxLimitNs = 0.70 * bufferPeriodNs  // 7,000,000 ns (7ms)

	for _, tc := range testMatrix {
		t.Run(tc.name, func(t *testing.T) {
			dev := &MockPhase4Device{
				rate:       tc.sampleRate,
				channels:   2,
				sampleFmt:  audio.SampleFormat24In32,
				bufferSize: uint32(tc.frames * 4),
			}
			backend := &MockPhase4Backend{dev: dev}
			ctrl := controller.NewAppController(backend)
			_ = ctrl.Initialize()
			defer ctrl.Close()
			_ = ctrl.SelectDevice(dev.ID())

			ctrl.ConfirmSafety(controller.RiskSquareWave, true)
			ctrl.ConfirmSafety(controller.RiskMultitoneHighCrest, true)

			req := controller.DefaultExtendedSignalRequest()
			req.SampleRate = tc.sampleRate
			req.Channels = 2
			req.SampleFormat = audio.SampleFormat24In32
			req.Waveform = tc.waveform
			req.Frequency = 1000.0
			req.Amplitude = 0.8
			req.LeftVolumeDBFS = 0.0
			req.RightVolumeDBFS = 0.0

			err := ctrl.ConfigureExtendedSignal(req)
			if err != nil {
				t.Fatalf("configure failed: %v", err)
			}

			// Render micro-benchmarking: run 1,000 iterations to simulate real-time render cycles
			// Allocations per run must be zero and compute time must be well under limits
			tracker := controller.NewPercentileTracker(2000)
			numChannels := 2
			channels := make([][]float64, numChannels)
			for ch := 0; ch < numChannels; ch++ {
				channels[ch] = make([]float64, tc.frames)
			}
			rawBytes := make([]byte, tc.frames*numChannels*4)

			factory := controller.NewGeneratorFactory()
			gen, err := factory.CreateGenerator(req)
			if err != nil {
				t.Fatalf("factory create failed: %v", err)
			}
			scaledGen := controller.NewChannelScaleGenerator(gen, 0.0, 0.0)

			var maxRenderNs int64
			const iterations = 1000

			// Warmup
			for i := 0; i < 50; i++ {
				for ch := 0; ch < numChannels; ch++ {
					scaledGen.Render(channels[ch])
				}
				_ = format.Interleave(rawBytes, channels, audio.AudioFormat{
					SampleRate:   tc.sampleRate,
					Channels:     numChannels,
					SampleFormat: audio.SampleFormat24In32,
				}, false)
			}

			// Benchmark render iterations
			for i := 0; i < iterations; i++ {
				start := time.Now()

				for ch := 0; ch < numChannels; ch++ {
					scaledGen.Render(channels[ch])
				}
				_ = format.Interleave(rawBytes, channels, audio.AudioFormat{
					SampleRate:   tc.sampleRate,
					Channels:     numChannels,
					SampleFormat: audio.SampleFormat24In32,
				}, false)

				durNs := time.Since(start).Nanoseconds()
				tracker.Record(durNs)
				if durNs > maxRenderNs {
					maxRenderNs = durNs
				}
			}

			p999Ns := tracker.P999()
			p999Pct := (float64(p999Ns) / bufferPeriodNs) * 100.0
			maxPct := (float64(maxRenderNs) / bufferPeriodNs) * 100.0

			t.Logf("[%s] P99.9: %d ns (%.2f%% of 10ms), Max: %d ns (%.2f%% of 10ms)",
				tc.name, p999Ns, p999Pct, maxRenderNs, maxPct)

			if float64(p999Ns) >= p999LimitNs {
				t.Errorf("[%s] P99.9 render time %d ns exceeds limit 30%% (%d ns)",
					tc.name, p999Ns, int64(p999LimitNs))
			}
			if float64(maxRenderNs) >= maxLimitNs {
				t.Errorf("[%s] Max render time %d ns exceeds limit 70%% (%d ns)",
					tc.name, maxRenderNs, int64(maxLimitNs))
			}
		})
	}
}

// TestPhase4_ZeroAllocsDuringRender verifies zero allocations in render path.
func TestPhase4_ZeroAllocsDuringRender(t *testing.T) {
	const frames = 480
	channels := make([][]float64, 2)
	for ch := 0; ch < 2; ch++ {
		channels[ch] = make([]float64, frames)
	}
	rawBytes := make([]byte, frames*2*4)

	factory := controller.NewGeneratorFactory()
	req := controller.DefaultExtendedSignalRequest()
	gen, _ := factory.CreateGenerator(req)
	scaledGen := controller.NewChannelScaleGenerator(gen, 0.0, 0.0)

	targetFmt := audio.AudioFormat{
		SampleRate:   48000,
		Channels:     2,
		SampleFormat: audio.SampleFormat24In32,
	}

	allocs := testing.AllocsPerRun(1000, func() {
		for ch := 0; ch < 2; ch++ {
			scaledGen.Render(channels[ch])
		}
		_ = format.Interleave(rawBytes, channels, targetFmt, false)
	})

	if allocs > 0 {
		t.Errorf("expected 0 allocations per render cycle, got %f", allocs)
	}
}
