package engine

// FaderState represents the gain progression state of the fader.
type FaderState int

const (
	FaderStateMuted FaderState = iota
	FaderStateFadingIn
	FaderStateUnity
	FaderStateFadingOut
)

func (s FaderState) String() string {
	switch s {
	case FaderStateMuted:
		return "Muted"
	case FaderStateFadingIn:
		return "FadingIn"
	case FaderStateUnity:
		return "Unity"
	case FaderStateFadingOut:
		return "FadingOut"
	default:
		return "Unknown"
	}
}

// Fader provides click-free sample-accurate gain ramps between 0.0 and 1.0.
type Fader struct {
	state       FaderState
	currentGain float64
	step        float64
}

// NewFader creates a fader initialized in the Muted state.
func NewFader() *Fader {
	return &Fader{
		state:       FaderStateMuted,
		currentGain: 0.0,
	}
}

// State returns current fader operational state.
func (f *Fader) State() FaderState {
	return f.state
}

// CurrentGain returns the instantaneous gain factor [0.0, 1.0].
func (f *Fader) CurrentGain() float64 {
	return f.currentGain
}

// StartFadeIn initiates a smooth transition from current gain up to 1.0.
func (f *Fader) StartFadeIn(sampleRate int, durationMs float64) {
	if durationMs < 5.0 {
		durationMs = 5.0
	} else if durationMs > 20.0 {
		durationMs = 20.0
	}

	totalSamples := float64(sampleRate) * (durationMs / 1000.0)
	if totalSamples <= 1.0 {
		f.currentGain = 1.0
		f.state = FaderStateUnity
		return
	}

	f.step = (1.0 - f.currentGain) / totalSamples
	f.state = FaderStateFadingIn
}

// StartFadeOut initiates a smooth transition from current gain down to 0.0.
func (f *Fader) StartFadeOut(sampleRate int, durationMs float64) {
	if durationMs < 5.0 {
		durationMs = 5.0
	} else if durationMs > 20.0 {
		durationMs = 20.0
	}

	totalSamples := float64(sampleRate) * (durationMs / 1000.0)
	if totalSamples <= 1.0 {
		f.currentGain = 0.0
		f.state = FaderStateMuted
		return
	}

	f.step = f.currentGain / totalSamples
	if f.step <= 0.0 {
		f.step = 1.0 / totalSamples
	}
	f.state = FaderStateFadingOut
}

// Mute immediately zeroes gain and sets state to FaderStateMuted.
func (f *Fader) Mute() {
	f.currentGain = 0.0
	f.state = FaderStateMuted
}

// Apply applies the gain envelope to a slice of float64 samples.
// Returns true if a fade-out completed during this buffer.
func (f *Fader) Apply(samples []float64) bool {
	fadeOutFinished := false

	switch f.state {
	case FaderStateMuted:
		for i := 0; i < len(samples); i++ {
			samples[i] = 0.0
		}
		fadeOutFinished = true
	case FaderStateUnity:
		// No modification needed; unity gain
	case FaderStateFadingIn:
		gain := f.currentGain
		step := f.step
		for i := 0; i < len(samples); i++ {
			gain += step
			if gain >= 1.0-1e-9 {
				gain = 1.0
				f.state = FaderStateUnity
				// Remaining samples remain untouched at unity gain
				break
			}
			samples[i] *= gain
		}
		f.currentGain = gain

	case FaderStateFadingOut:
		gain := f.currentGain
		step := f.step
		for i := 0; i < len(samples); i++ {
			gain -= step
			if gain <= 1e-9 {
				gain = 0.0
				f.state = FaderStateMuted
				fadeOutFinished = true
				for ; i < len(samples); i++ {
					samples[i] = 0.0
				}
				break
			}
			samples[i] *= gain
		}
		f.currentGain = gain
	}

	return fadeOutFinished
}

// ApplyChannels applies the gain envelope identically across all channels.
func (f *Fader) ApplyChannels(channels [][]float64) bool {
	if len(channels) == 0 || len(channels[0]) == 0 {
		return false
	}

	numFrames := len(channels[0])
	fadeOutFinished := false

	switch f.state {
	case FaderStateMuted:
		for ch := 0; ch < len(channels); ch++ {
			for i := 0; i < numFrames; i++ {
				channels[ch][i] = 0.0
			}
		}
		fadeOutFinished = true
	case FaderStateUnity:
		// Unity gain: nothing to do
	case FaderStateFadingIn:
		gain := f.currentGain
		step := f.step
		for i := 0; i < numFrames; i++ {
			gain += step
			if gain >= 1.0-1e-9 {
				gain = 1.0
				f.state = FaderStateUnity
				break
			}
			for ch := 0; ch < len(channels); ch++ {
				channels[ch][i] *= gain
			}
		}
		f.currentGain = gain

	case FaderStateFadingOut:
		gain := f.currentGain
		step := f.step
		for i := 0; i < numFrames; i++ {
			gain -= step
			if gain <= 1e-9 {
				gain = 0.0
				f.state = FaderStateMuted
				fadeOutFinished = true
				for ; i < numFrames; i++ {
					for ch := 0; ch < len(channels); ch++ {
						channels[ch][i] = 0.0
					}
				}
				break
			}
			for ch := 0; ch < len(channels); ch++ {
				channels[ch][i] *= gain
			}
		}
		f.currentGain = gain
	}

	return fadeOutFinished
}
