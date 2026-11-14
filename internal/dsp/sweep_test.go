package dsp

import (
	"math"
	"testing"
)

func TestSweepZeroAllocations(t *testing.T) {
	gen := NewSweepGenerator()
	err := gen.PrepareSweep(SweepConfig{
		SampleRate:    48000,
		StartFreq:     20.0,
		EndFreq:       20000.0,
		DurationSec:   1.0,
		Amplitude:     0.9,
		TaperFraction: 0.10,
		Mode:          SweepModeLogarithmic,
		Loop:          true,
	})
	if err != nil {
		t.Fatalf("PrepareSweep failed: %v", err)
	}

	dst := make([]float64, 512)

	allocs := testing.AllocsPerRun(1000, func() {
		gen.Render(dst)
	})

	if allocs != 0 {
		t.Errorf("expected 0 allocations in Render, got %v", allocs)
	}
}

func TestSweepPeakOvershootAndTapering(t *testing.T) {
	modes := []struct {
		mode SweepMode
		desc string
	}{
		{mode: SweepModeLinear, desc: "Linear Sweep"},
		{mode: SweepModeLogarithmic, desc: "Logarithmic Sweep"},
	}

	const (
		rate        = 48000
		startF      = 20.0
		endF        = 20000.0
		duration    = 1.0
		targetAmp   = 0.8
		dcOffset    = 0.05
		taperFrac   = 0.10
		totalPoints = int(duration * rate)
	)

	for _, m := range modes {
		t.Run(m.desc, func(t *testing.T) {
			gen := NewSweepGenerator()
			err := gen.PrepareSweep(SweepConfig{
				SampleRate:    rate,
				StartFreq:     startF,
				EndFreq:       endF,
				DurationSec:   duration,
				Amplitude:     targetAmp,
				DCOffset:      dcOffset,
				InitialPhase:  0.0,
				TaperFraction: taperFrac,
				Mode:          m.mode,
				Loop:          false,
			})
			if err != nil {
				t.Fatalf("PrepareSweep failed: %v", err)
			}

			samples := make([]float64, totalPoints)
			gen.Render(samples)

			// 1. Verify start and end are smoothly tapered (samples[0] close to DC offset)
			if math.Abs(samples[0]-dcOffset) > 1e-4 {
				t.Errorf("expected smooth start close to DC offset %.4f, got %.4f", dcOffset, samples[0])
			}
			lastSample := samples[len(samples)-1]
			if math.Abs(lastSample-dcOffset) > 0.05 {
				t.Errorf("expected smooth end close to DC offset %.4f, got %.4f", dcOffset, lastSample)
			}

			// 2. Acceptance: Time-domain peak overshoot < 1.05 * target amplitude
			maxObserved := 0.0
			for _, s := range samples {
				absVal := math.Abs(s - dcOffset)
				if absVal > maxObserved {
					maxObserved = absVal
				}
			}

			maxAllowed := 1.05 * targetAmp
			t.Logf("[%s] Target Amp: %.4f, Max Observed AC: %.4f (Allowed: %.4f)",
				m.desc, targetAmp, maxObserved, maxAllowed)

			if maxObserved > maxAllowed {
				t.Errorf("[%s] peak overshoot exceeded: observed %.4f > allowed %.4f",
					m.desc, maxObserved, maxAllowed)
			}
		})
	}
}

func TestSweepInstantaneousFrequencyAccuracy(t *testing.T) {
	modes := []struct {
		mode SweepMode
		desc string
	}{
		{mode: SweepModeLinear, desc: "Linear Sweep"},
		{mode: SweepModeLogarithmic, desc: "Logarithmic Sweep"},
	}

	const (
		rate      = 48000
		startF    = 50.0
		endF      = 10000.0
		duration  = 2.0
		totalN    = int(duration * rate)
		taperFrac = 0.10
		taperN    = int(taperFrac * float64(totalN))
	)

	for _, m := range modes {
		t.Run(m.desc, func(t *testing.T) {
			gen := NewSweepGenerator()
			err := gen.PrepareSweep(SweepConfig{
				SampleRate:    rate,
				StartFreq:     startF,
				EndFreq:       endF,
				DurationSec:   duration,
				Amplitude:     1.0,
				DCOffset:      0.0,
				InitialPhase:  0.0,
				TaperFraction: taperFrac,
				Mode:          m.mode,
				Loop:          false,
			})
			if err != nil {
				t.Fatalf("PrepareSweep failed: %v", err)
			}

			// Verify instantaneous frequency accuracy in steady-state (10% to 90% duration)
			// Phase difference between sample n and n+1: deltaPhi = phi(t + dt) - phi(t)
			// Estimated inst freq = deltaPhi / (2*pi * dt)
			dt := 1.0 / float64(rate)
			steadyStart := taperN
			steadyEnd := totalN - taperN

			maxRelErr := 0.0
			worstT := 0.0
			worstExpected := 0.0
			worstMeasured := 0.0

			for n := steadyStart; n < steadyEnd; n++ {
				tMid := (float64(n) + 0.5) * dt
				expectedF := gen.InstantaneousFrequency(tMid)

				t1 := float64(n) * dt
				t2 := float64(n+1) * dt
				phi1 := gen.calculatePhase(t1)
				phi2 := gen.calculatePhase(t2)

				deltaPhi := phi2 - phi1
				measuredF := deltaPhi / (twoPi * dt)

				relErr := math.Abs(measuredF-expectedF) / expectedF
				if relErr > maxRelErr {
					maxRelErr = relErr
					worstT = tMid
					worstExpected = expectedF
					worstMeasured = measuredF
				}
			}

			relErrPercent := maxRelErr * 100.0
			t.Logf("[%s] Max Inst Freq Error in Steady-State: %.5f%% (at t=%.3fs, expected=%.2f Hz, measured=%.2f Hz)",
				m.desc, relErrPercent, worstT, worstExpected, worstMeasured)

			// Acceptance criteria: instantaneous frequency error < 0.1%
			if relErrPercent >= 0.1 {
				t.Errorf("[%s] expected inst freq error < 0.1%%, got %.5f%%", m.desc, relErrPercent)
			}
		})
	}
}

func TestSweepLoopingAndSingleShot(t *testing.T) {
	gen := NewSweepGenerator()
	err := gen.PrepareSweep(SweepConfig{
		SampleRate:    48000,
		StartFreq:     100.0,
		EndFreq:       1000.0,
		DurationSec:   0.01, // 480 samples
		Amplitude:     1.0,
		DCOffset:      0.0,
		TaperFraction: 0.10,
		Mode:          SweepModeLinear,
		Loop:          false, // Single shot
	})
	if err != nil {
		t.Fatalf("PrepareSweep failed: %v", err)
	}

	firstPass := make([]float64, 480)
	gen.Render(firstPass)

	// Second pass: should output silence / DC offset because Loop=false
	secondPass := make([]float64, 480)
	gen.Render(secondPass)

	for i, s := range secondPass {
		if s != 0.0 {
			t.Fatalf("sample %d in single-shot second pass expected 0.0, got %v", i, s)
		}
	}

	// Now reset with Loop = true
	_ = gen.PrepareSweep(SweepConfig{
		SampleRate:    48000,
		StartFreq:     100.0,
		EndFreq:       1000.0,
		DurationSec:   0.01,
		Amplitude:     1.0,
		TaperFraction: 0.10,
		Mode:          SweepModeLinear,
		Loop:          true,
	})

	loopBuf1 := make([]float64, 480)
	gen.Render(loopBuf1)
	loopBuf2 := make([]float64, 480)
	gen.Render(loopBuf2)

	// In looping mode, loopBuf2 should be identical to loopBuf1
	for i := range loopBuf1 {
		if math.Abs(loopBuf1[i]-loopBuf2[i]) > 1e-12 {
			t.Errorf("sample %d differs between loop cycles: %v vs %v", i, loopBuf1[i], loopBuf2[i])
			break
		}
	}
}
