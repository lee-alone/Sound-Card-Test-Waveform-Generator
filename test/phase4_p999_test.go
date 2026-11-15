package test

import (
	"os"
	"runtime"
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/format"
	"sound-signal-generator/internal/platform"
)

type phase4Case struct {
	name       string
	sampleRate int
	frames     int
	waveform   controller.WaveformType
}

var phase4Matrix = []phase4Case{
	{name: "48kHz_480frames_Sine", sampleRate: 48000, frames: 480, waveform: controller.WaveformSine},
	{name: "96kHz_960frames_Square", sampleRate: 96000, frames: 960, waveform: controller.WaveformPolyBlepSquare},
	{name: "192kHz_1920frames_Multitone", sampleRate: 192000, frames: 1920, waveform: controller.WaveformMultitoneCCIF},
}

type phase4Fixture struct {
	channels  [][]float64
	rawBytes  []byte
	targetFmt audio.AudioFormat
	generator *controller.ChannelScaleGenerator
}

func newPhase4Fixture(tc phase4Case) (*phase4Fixture, error) {
	req := controller.DefaultExtendedSignalRequest()
	req.SampleRate = tc.sampleRate
	req.Channels = 2
	req.SampleFormat = audio.SampleFormat24In32
	req.Waveform = tc.waveform
	req.Frequency = 1000.0
	req.Amplitude = 0.8
	req.LeftVolumeDBFS = 0.0
	req.RightVolumeDBFS = 0.0

	gen, err := controller.NewGeneratorFactory().CreateGenerator(req)
	if err != nil {
		return nil, err
	}

	const numChannels = 2
	channels := make([][]float64, numChannels)
	for ch := range channels {
		channels[ch] = make([]float64, tc.frames)
	}

	return &phase4Fixture{
		channels: channels,
		rawBytes: make([]byte, tc.frames*numChannels*4),
		targetFmt: audio.AudioFormat{
			SampleRate:   tc.sampleRate,
			Channels:     numChannels,
			SampleFormat: audio.SampleFormat24In32,
		},
		generator: controller.NewChannelScaleGenerator(gen, 0.0, 0.0),
	}, nil
}

func (f *phase4Fixture) renderOnce() error {
	for ch := range f.channels {
		f.generator.Render(f.channels[ch])
	}
	return format.Interleave(f.rawBytes, f.channels, f.targetFmt, false)
}

// TestPhase4_RenderThroughputMatrix is the always-on performance smoke test.
// Each sample is the mean of a 1000-callback batch, which filters ordinary OS
// preemption while still catching material DSP or quantization regressions.
func TestPhase4_RenderThroughputMatrix(t *testing.T) {
	const (
		bufferPeriodNs     = 10_000_000.0
		throughputLimitNs  = 0.30 * bufferPeriodNs
		iterationsPerBatch = 1000
		batches            = 9
	)

	for _, tc := range phase4Matrix {
		t.Run(tc.name, func(t *testing.T) {
			fixture, err := newPhase4Fixture(tc)
			if err != nil {
				t.Fatalf("fixture creation failed: %v", err)
			}

			for i := 0; i < 100; i++ {
				if err := fixture.renderOnce(); err != nil {
					t.Fatalf("warmup render failed: %v", err)
				}
			}

			tracker := controller.NewPercentileTracker(batches)
			for batch := 0; batch < batches; batch++ {
				start := time.Now()
				for i := 0; i < iterationsPerBatch; i++ {
					if err := fixture.renderOnce(); err != nil {
						t.Fatalf("render failed: %v", err)
					}
				}
				meanNs := time.Since(start).Nanoseconds() / iterationsPerBatch
				tracker.Record(meanNs)
			}

			p50Ns := tracker.P50()
			p99Ns := tracker.P99()
			t.Logf("[%s] batch mean P50: %d ns (%.3f%% of 10ms), P99: %d ns",
				tc.name, p50Ns, (float64(p50Ns)/bufferPeriodNs)*100.0, p99Ns)

			if float64(p50Ns) >= throughputLimitNs {
				t.Errorf("[%s] median batch mean %d ns exceeds 30%% buffer budget (%d ns)",
					tc.name, p50Ns, int64(throughputLimitNs))
			}
		})
	}
}

// TestPhase4_P999_BenchmarkMatrix runs the strict tail-latency acceptance test.
// Wall-clock max latency is only meaningful on an idle reference host (see
// DEVELOPMENT_SCHEDULE.md), so keep it opt-in rather than making normal unit
// test runs flaky due to unrelated OS scheduling and DPC activity.
func TestPhase4_P999_BenchmarkMatrix(t *testing.T) {
	if os.Getenv("AUDIO_PERF_STRESS") != "1" {
		t.Skip("strict tail-latency test is opt-in; set AUDIO_PERF_STRESS=1 on an idle reference host")
	}

	const (
		bufferPeriodNs = 10_000_000.0
		p99LimitNs     = 0.20 * bufferPeriodNs
		p999LimitNs    = 0.30 * bufferPeriodNs
		maxLimitNs     = 0.70 * bufferPeriodNs
		iterations     = 20_000
	)

	for _, tc := range phase4Matrix {
		t.Run(tc.name, func(t *testing.T) {
			fixture, err := newPhase4Fixture(tc)
			if err != nil {
				t.Fatalf("fixture creation failed: %v", err)
			}

			for i := 0; i < 100; i++ {
				if err := fixture.renderOnce(); err != nil {
					t.Fatalf("warmup render failed: %v", err)
				}
			}

			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			if revert, prioErr := platform.EnableRealtimeAudioPriority(); prioErr != nil {
				t.Logf("MMCSS priority unavailable, continuing with normal scheduling: %v", prioErr)
			} else {
				defer revert()
			}

			tracker := controller.NewPercentileTracker(iterations)
			var maxRenderNs int64
			for i := 0; i < iterations; i++ {
				start := time.Now()
				if err := fixture.renderOnce(); err != nil {
					t.Fatalf("render failed: %v", err)
				}
				durNs := time.Since(start).Nanoseconds()
				tracker.Record(durNs)
				if durNs > maxRenderNs {
					maxRenderNs = durNs
				}
			}

			p50Ns := tracker.P50()
			p99Ns := tracker.P99()
			p999Ns := tracker.P999()
			t.Logf("[%s] P50: %d ns, P99: %d ns (%.2f%%), P99.9: %d ns (%.2f%%), Max: %d ns (%.2f%%), N=%d",
				tc.name,
				p50Ns,
				p99Ns, (float64(p99Ns)/bufferPeriodNs)*100.0,
				p999Ns, (float64(p999Ns)/bufferPeriodNs)*100.0,
				maxRenderNs, (float64(maxRenderNs)/bufferPeriodNs)*100.0,
				iterations)

			if float64(p99Ns) >= p99LimitNs {
				t.Errorf("[%s] P99 render time %d ns exceeds 20%% limit (%d ns)",
					tc.name, p99Ns, int64(p99LimitNs))
			}
			if float64(p999Ns) >= p999LimitNs {
				t.Errorf("[%s] P99.9 render time %d ns exceeds 30%% limit (%d ns)",
					tc.name, p999Ns, int64(p999LimitNs))
			}
			if float64(maxRenderNs) >= maxLimitNs {
				t.Errorf("[%s] Max render time %d ns exceeds 70%% limit (%d ns)",
					tc.name, maxRenderNs, int64(maxLimitNs))
			}
		})
	}
}

// TestPhase4_ZeroAllocsDuringRender verifies zero allocations in the render path.
func TestPhase4_ZeroAllocsDuringRender(t *testing.T) {
	fixture, err := newPhase4Fixture(phase4Matrix[0])
	if err != nil {
		t.Fatalf("fixture creation failed: %v", err)
	}

	allocs := testing.AllocsPerRun(1000, func() {
		_ = fixture.renderOnce()
	})
	if allocs > 0 {
		t.Errorf("expected 0 allocations per render cycle, got %f", allocs)
	}
}

// BenchmarkPhase4_RenderMatrix measures steady-state DSP + interleave throughput
// without turning scheduler stalls into test failures.
func BenchmarkPhase4_RenderMatrix(b *testing.B) {
	for _, tc := range phase4Matrix {
		b.Run(tc.name, func(b *testing.B) {
			fixture, err := newPhase4Fixture(tc)
			if err != nil {
				b.Fatalf("fixture creation failed: %v", err)
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(fixture.rawBytes)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := fixture.renderOnce(); err != nil {
					b.Fatalf("render failed: %v", err)
				}
			}
		})
	}
}
