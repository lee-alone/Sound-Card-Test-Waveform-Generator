package dsp

import (
	"math"
	"math/bits"
)

// fastPRNG implements a fast, deterministic 64-bit XorShift/SplitMix pseudo-random generator
// that allows fully reproducible sequences from an explicit 64-bit seed with zero allocations.
type fastPRNG struct {
	state uint64
}

func newFastPRNG(seed uint64) fastPRNG {
	if seed == 0 {
		seed = 0x853c49e6748fea9b // Default non-zero seed
	}
	return fastPRNG{state: seed}
}

// NextUint64 returns a 64-bit pseudo-random unsigned integer (SplitMix64).
func (r *fastPRNG) NextUint64() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// NextFloat64 returns a uniform float in (0.0, 1.0].
func (r *fastPRNG) NextFloat64() float64 {
	// Generate 53-bit mantissa in (0, 1]
	val := r.NextUint64() >> 11
	return (float64(val) + 1.0) / 9007199254740992.0 // (val + 1) / 2^53
}

// NextGaussian generates a standard normal N(0, 1) variable using Box-Muller transform.
func (r *fastPRNG) NextGaussian() (float64, float64) {
	u1 := r.NextFloat64()
	u2 := r.NextFloat64()
	mag := math.Sqrt(-2.0 * math.Log(u1))
	z0 := mag * math.Cos(twoPi*u2)
	z1 := mag * math.Sin(twoPi*u2)
	return z0, z1
}

// NoiseType defines whether noise is White (flat PSD) or Pink (-3dB/octave PSD).
type NoiseType int

const (
	// NoiseTypeWhite represents Gaussian white noise.
	NoiseTypeWhite NoiseType = iota

	// NoiseTypePink represents 1/f pink noise generated via Voss-McCartney algorithm.
	NoiseTypePink
)

// NoiseConfig defines configuration parameters for noise generators.
type NoiseConfig struct {
	SampleRate int       `json:"sample_rate"`
	Amplitude  float64   `json:"amplitude"` // Peak/RMS reference scaling
	DCOffset   float64   `json:"dc_offset"`
	Seed       uint64    `json:"seed"`
	Type       NoiseType `json:"type"`
}

// GaussianWhiteNoiseGenerator generates reproducible Gaussian white noise.
// Operates with zero memory allocations during Render.
type GaussianWhiteNoiseGenerator struct {
	sampleRate float64
	amplitude  float64
	dcOffset   float64
	seed       uint64
	prng       fastPRNG

	hasCached bool
	cachedVal float64
}

// NewGaussianWhiteNoiseGenerator creates an uninitialized GaussianWhiteNoiseGenerator instance.
func NewGaussianWhiteNoiseGenerator() *GaussianWhiteNoiseGenerator {
	return &GaussianWhiteNoiseGenerator{}
}

// Prepare implements general Generator interface.
func (g *GaussianWhiteNoiseGenerator) Prepare(cfg Config) error {
	return g.PrepareNoise(NoiseConfig{
		SampleRate: cfg.SampleRate,
		Amplitude:  cfg.Amplitude,
		DCOffset:   cfg.DCOffset,
		Seed:       12345,
		Type:       NoiseTypeWhite,
	})
}

// PrepareNoise configures the white noise generator.
func (g *GaussianWhiteNoiseGenerator) PrepareNoise(cfg NoiseConfig) error {
	if cfg.SampleRate <= 0 {
		return ErrInvalidSampleRate
	}
	if cfg.Amplitude < 0 {
		return ErrNegativeAmplitude
	}

	g.sampleRate = float64(cfg.SampleRate)
	g.amplitude = cfg.Amplitude
	g.dcOffset = cfg.DCOffset
	g.seed = cfg.Seed
	g.prng = newFastPRNG(cfg.Seed)
	g.hasCached = false

	return nil
}

// Reset resets the PRNG state according to policy.
func (g *GaussianWhiteNoiseGenerator) Reset(policy ResetPolicy) {
	if policy == ResetToZero {
		g.prng = newFastPRNG(g.seed)
		g.hasCached = false
	}
}

// Render populates dst with Gaussian white noise samples.
// Scales by amplitude / 4.0 so that 4-sigma peaks remain within nominal range.
func (g *GaussianWhiteNoiseGenerator) Render(dst []float64) {
	scale := (g.amplitude / 4.0)
	dc := g.dcOffset

	for i := 0; i < len(dst); i++ {
		var z float64
		if g.hasCached {
			z = g.cachedVal
			g.hasCached = false
		} else {
			z0, z1 := g.prng.NextGaussian()
			z = z0
			g.cachedVal = z1
			g.hasCached = true
		}

		dst[i] = z*scale + dc
	}
}

// MaxPeak returns theoretical 4-sigma peak limit.
func (g *GaussianWhiteNoiseGenerator) MaxPeak() float64 {
	return g.amplitude + math.Abs(g.dcOffset)
}

// VossPinkNoiseGenerator generates 1/f pink noise (-3 dB/octave) using the Voss-McCartney algorithm.
// Employs 20 octave rows with optimized white noise injection and zero memory allocations in Render.
type VossPinkNoiseGenerator struct {
	sampleRate float64
	amplitude  float64
	dcOffset   float64
	seed       uint64
	prng       fastPRNG

	numRows    int
	rows       [20]float64
	runningSum float64
	counter    uint64

	hasCached bool
	cachedVal float64
}

const (
	vossNumRows     = 20
	vossWhiteWeight = 1.20
)

// NewVossPinkNoiseGenerator creates an uninitialized VossPinkNoiseGenerator instance.
func NewVossPinkNoiseGenerator() *VossPinkNoiseGenerator {
	return &VossPinkNoiseGenerator{
		numRows: vossNumRows,
	}
}

// Prepare implements general Generator interface.
func (g *VossPinkNoiseGenerator) Prepare(cfg Config) error {
	return g.PrepareNoise(NoiseConfig{
		SampleRate: cfg.SampleRate,
		Amplitude:  cfg.Amplitude,
		DCOffset:   cfg.DCOffset,
		Seed:       12345,
		Type:       NoiseTypePink,
	})
}

// PrepareNoise configures the pink noise generator.
func (g *VossPinkNoiseGenerator) PrepareNoise(cfg NoiseConfig) error {
	if cfg.SampleRate <= 0 {
		return ErrInvalidSampleRate
	}
	if cfg.Amplitude < 0 {
		return ErrNegativeAmplitude
	}

	g.sampleRate = float64(cfg.SampleRate)
	g.amplitude = cfg.Amplitude
	g.dcOffset = cfg.DCOffset
	g.seed = cfg.Seed
	g.numRows = vossNumRows

	g.initVossState()
	return nil
}

func (g *VossPinkNoiseGenerator) nextGaussian() float64 {
	if g.hasCached {
		g.hasCached = false
		return g.cachedVal
	}
	z0, z1 := g.prng.NextGaussian()
	g.cachedVal = z1
	g.hasCached = true
	return z0
}

func (g *VossPinkNoiseGenerator) initVossState() {
	g.prng = newFastPRNG(g.seed)
	g.hasCached = false
	g.runningSum = 0.0
	g.counter = 0

	for i := 0; i < g.numRows; i++ {
		val := g.nextGaussian()
		g.rows[i] = val
		g.runningSum += val
	}
}

// Reset resets PRNG and Voss rows according to policy.
func (g *VossPinkNoiseGenerator) Reset(policy ResetPolicy) {
	if policy == ResetToZero {
		g.initVossState()
	}
}

// Render populates dst with 1/f pink noise samples.
// Executes in the real-time thread hot path and performs zero memory allocations.
func (g *VossPinkNoiseGenerator) Render(dst []float64) {
	// Normalization factor: variance of runningSum + white is approx (numRows + whiteWeight^2)
	norm := math.Sqrt(float64(g.numRows) + vossWhiteWeight*vossWhiteWeight)
	scale := (g.amplitude / 4.0) / norm
	dc := g.dcOffset
	numRows := g.numRows

	for i := 0; i < len(dst); i++ {
		g.counter++
		tz := bits.TrailingZeros64(g.counter)
		if tz < numRows {
			oldVal := g.rows[tz]
			newVal := g.nextGaussian()
			g.runningSum += (newVal - oldVal)
			g.rows[tz] = newVal
		}

		white := g.nextGaussian() * vossWhiteWeight
		sample := (g.runningSum + white) * scale

		dst[i] = sample + dc
	}
}

// MaxPeak returns theoretical peak limit.
func (g *VossPinkNoiseGenerator) MaxPeak() float64 {
	return g.amplitude + math.Abs(g.dcOffset)
}
