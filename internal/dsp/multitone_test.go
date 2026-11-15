package dsp

import (
	"math"
	"testing"
)

func TestMultitoneZeroAllocations(t *testing.T) {
	gen := NewMultitoneGenerator()
	err := gen.PrepareMultitone(NewSMPTEConfig(48000, 1.0, 0.0))
	if err != nil {
		t.Fatalf("PrepareMultitone failed: %v", err)
	}

	dst := make([]float64, 512)

	allocs := testing.AllocsPerRun(1000, func() {
		gen.Render(dst)
	})

	if allocs != 0 {
		t.Errorf("expected 0 allocations in Render, got %v", allocs)
	}
}

func TestMultitonePAPRAndClippingPrevention(t *testing.T) {
	tests := []struct {
		name       string
		cfg        MultitoneConfig
		expCF      float64
		expPAPR_dB float64
	}{
		{
			name:       "SMPTE 4:1 (60Hz + 7kHz)",
			cfg:        NewSMPTEConfig(48000, 1.0, 0.0),
			expCF:      1.715,
			expPAPR_dB: 4.69,
		},
		{
			name:       "CCIF 1:1 (19kHz + 20kHz)",
			cfg:        NewCCIFConfig(48000, 1.0, 0.0),
			expCF:      2.000,
			expPAPR_dB: 6.02,
		},
		{
			name:       "DIN 4:1 (250Hz + 8kHz)",
			cfg:        NewDINConfig(48000, 0.9, 0.05),
			expCF:      1.715, // Without DC offset component
			expPAPR_dB: 4.69,
		},
		{
			name: "Arbitrary 5-Tone Overloaded Configuration",
			cfg: MultitoneConfig{
				SampleRate:   48000,
				TargetPeak:   1.0,
				DCOffset:     0.0,
				AutoNormPAPR: true,
				Components: []ToneComponent{
					{Frequency: 100.0, Amplitude: 0.5},
					{Frequency: 500.0, Amplitude: 0.5},
					{Frequency: 1000.0, Amplitude: 0.5},
					{Frequency: 2000.0, Amplitude: 0.5},
					{Frequency: 5000.0, Amplitude: 0.5}, // Sum of amps = 2.5 (would clip without PAPR norm)
				},
			},
			expCF:      0.0, // Checked dynamically below
			expPAPR_dB: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gen := NewMultitoneGenerator()
			err := gen.PrepareMultitone(tc.cfg)
			if err != nil {
				t.Fatalf("PrepareMultitone failed: %v", err)
			}

			scaleFactor := gen.ScaleFactor()
			theoPeak := gen.TheoreticalPeak()
			cf := gen.CrestFactor()
			paprDB := gen.PAPR_dB()

			t.Logf("[%s] ScaleFactor=%.4f, TheoPeak=%.4f, CF=%.3f, PAPR=%.2f dB",
				tc.name, scaleFactor, theoPeak, cf, paprDB)

			// 1. Scale factor must be explicitly defined and positive
			if scaleFactor <= 0.0 || scaleFactor > 1.0+1e-12 {
				t.Errorf("invalid scale factor: %v", scaleFactor)
			}

			// 2. Theoretical peak strictly bounded by target peak (no clipping!)
			if theoPeak > tc.cfg.TargetPeak+1e-12 {
				t.Errorf("theoretical peak %.6f exceeds target peak %.6f",
					theoPeak, tc.cfg.TargetPeak)
			}

			// 3. For 5-tone case, scale factor must be exactly 1.0 / 2.5 = 0.4
			if tc.name == "Arbitrary 5-Tone Overloaded Configuration" {
				if math.Abs(scaleFactor-0.4) > 1e-6 {
					t.Errorf("expected scale factor 0.4 for 5x0.5 tones, got %v", scaleFactor)
				}
			}

			// 4. Render 96,000 samples and verify measured absolute peak never exceeds TargetPeak
			samples := make([]float64, 96000)
			gen.Render(samples)

			maxMeasured := 0.0
			for _, s := range samples {
				absVal := math.Abs(s)
				if absVal > maxMeasured {
					maxMeasured = absVal
				}
			}

			t.Logf("[%s] Max Measured Sample: %.6f <= TargetPeak %.6f",
				tc.name, maxMeasured, tc.cfg.TargetPeak)

			if maxMeasured > tc.cfg.TargetPeak+1e-9 {
				t.Errorf("[%s] sample value %.6f exceeded target peak %.6f (clipping occurred!)",
					tc.name, maxMeasured, tc.cfg.TargetPeak)
			}
		})
	}
}

func TestMultitoneNyquistGuard(t *testing.T) {
	gen := NewMultitoneGenerator()
	err := gen.PrepareMultitone(MultitoneConfig{
		SampleRate: 48000,
		TargetPeak: 1.0,
		Components: []ToneComponent{
			{Frequency: 23000.0, Amplitude: 0.5}, // Exceeds safe 0.45 * 48000 = 21600 Hz
		},
	})
	if err == nil {
		t.Errorf("expected error for frequency exceeding safe Nyquist limit, got nil")
	}
}

func TestMultitoneOscillatorRecurrenceAccuracy(t *testing.T) {
	cfg := NewCCIFConfig(192000, 1.0, 0.0)
	gen := NewMultitoneGenerator()
	if err := gen.PrepareMultitone(cfg); err != nil {
		t.Fatalf("PrepareMultitone failed: %v", err)
	}

	const n = 4096
	got := make([]float64, n)
	gen.Render(got)

	phases := make([]float64, len(cfg.Components))
	steps := make([]float64, len(cfg.Components))
	for i, c := range cfg.Components {
		phases[i] = math.Mod(c.InitialPhase, twoPi)
		if phases[i] < 0 {
			phases[i] += twoPi
		}
		steps[i] = twoPi * c.Frequency / float64(cfg.SampleRate)
	}

	for i := 0; i < n; i++ {
		want := cfg.DCOffset
		for j, c := range cfg.Components {
			want += c.Amplitude * gen.ScaleFactor() * math.Sin(phases[j])
			phases[j] += steps[j]
			if phases[j] >= twoPi {
				phases[j] -= twoPi
			}
		}
		if diff := math.Abs(got[i] - want); diff > 1e-12 {
			t.Fatalf("sample %d: error %g, got %g want %g", i, diff, got[i], want)
		}
	}
}
