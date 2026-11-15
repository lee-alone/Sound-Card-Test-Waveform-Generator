package giuui

import (
	"math"
	"math/rand"

	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
)

// ScopeWidget renders a deterministic preview of the synthesized signal. The
// plot is intentionally larger than every other control: it is the fastest way
// to confirm waveform, routing and level before engaging hardware output.
type ScopeWidget struct {
	state *UIState
}

func NewScopeWidget(state *UIState) *ScopeWidget {
	return &ScopeWidget{state: state}
}

func (s *ScopeWidget) GenerateTraces(numPoints int) ([]float64, []float64) {
	if numPoints <= 0 {
		numPoints = 320
	}

	lTrace := make([]float64, numPoints)
	rTrace := make([]float64, numPoints)

	lGain := math.Pow(10, float64(s.state.LeftVolumeDBFS)/20.0)
	rGain := math.Pow(10, float64(s.state.RightVolumeDBFS)/20.0)

	switch ChannelMode(s.state.SelectedChannelMode) {
	case ChannelModeSoloL:
		rGain = 0
	case ChannelModeSoloR:
		lGain = 0
	case ChannelModeInvertR:
		rGain = -rGain
	case ChannelModeMono:
		monoGain := (lGain + rGain) * 0.5
		lGain = monoGain
		rGain = monoGain
	}

	waveID := controller.WaveformSine
	if int(s.state.SelectedWaveformIndex) >= 0 && int(s.state.SelectedWaveformIndex) < len(WaveformOptions) {
		waveID = WaveformOptions[s.state.SelectedWaveformIndex].ID
	}

	cycles := 2.0
	rng := rand.New(rand.NewSource(int64(s.state.NoiseSeed)))
	for i := 0; i < numPoints; i++ {
		tNorm := float64(i) / float64(numPoints)
		theta := 2.0 * math.Pi * cycles * tNorm

		var sample float64
		switch waveID {
		case controller.WaveformSine:
			sample = math.Sin(theta)
		case controller.WaveformPolyBlepSquare:
			sample = math.Copysign(1.0, math.Sin(theta))
		case controller.WaveformPolyBlepTriangle:
			sample = (2.0 / math.Pi) * math.Asin(math.Sin(theta))
		case controller.WaveformLinearSweep, controller.WaveformLogarithmicSweep:
			chirpTheta := 2.0 * math.Pi * (1.0 + 4.0*tNorm) * tNorm * cycles
			sample = math.Sin(chirpTheta)
		case controller.WaveformWhiteNoise:
			sample = (rng.Float64()*2.0 - 1.0) * 0.9
		case controller.WaveformPinkNoise:
			sample = math.Sin(theta*0.5)*0.5 + math.Sin(theta*2.0)*0.3 + (rng.Float64()*2.0-1.0)*0.2
		case controller.WaveformMultitoneSMPTE:
			sample = 0.8*math.Sin(theta) + 0.2*math.Sin(theta*11.67)
		case controller.WaveformMultitoneCCIF:
			sample = 0.5*math.Sin(theta*9.5) + 0.5*math.Sin(theta*10.0)
		case controller.WaveformDoP:
			marker := 0.8
			if int(tNorm*16)%2 == 0 {
				marker = -0.8
			}
			sample = marker * math.Sin(theta*4.0)
		default:
			sample = math.Sin(theta)
		}

		lTrace[i] = sample * lGain
		rTrace[i] = sample * rGain
	}

	return lTrace, rTrace
}

func (s *ScopeWidget) Build(height int) g.Widget {
	if height < 120 {
		height = 120
	}
	lTrace, rTrace := s.GenerateTraces(480)

	return g.Plot("##oscilloscope").
		Size(-1, int(px(float32(height)))).
		SetXAxisLabel(g.AxisX1, "时间 / 采样点").
		SetYAxisLabel(g.AxisY1, "幅度 / FS").
		AxisLimits(0, float64(len(lTrace)-1), -1.08, 1.08, g.ConditionAlways).
		Plots(
			g.Line("L", lTrace),
			g.Line("R", rTrace),
		)
}
