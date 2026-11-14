package controller

import (
	"math"
	"sound-signal-generator/internal/dsp"
)

// ChannelScaleGenerator decorates any dsp.Generator to apply independent channel gain.
type ChannelScaleGenerator struct {
	base      dsp.Generator
	scales    []float64
	numCh     int
	currentCh int
	cache     []float64
}

// NewChannelScaleGenerator creates a gain decorator for multi-channel output.
func NewChannelScaleGenerator(base dsp.Generator, leftDBFS, rightDBFS float64) *ChannelScaleGenerator {
	scaleL := dbfsToLinear(leftDBFS)
	scaleR := dbfsToLinear(rightDBFS)
	return &ChannelScaleGenerator{
		base:      base,
		scales:    []float64{scaleL, scaleR},
		numCh:     2,
		currentCh: 0,
		cache:     make([]float64, 0, 4096),
	}
}

func (g *ChannelScaleGenerator) Prepare(cfg dsp.Config) error {
	g.currentCh = 0
	return g.base.Prepare(cfg)
}

func (g *ChannelScaleGenerator) Reset(policy dsp.ResetPolicy) {
	g.currentCh = 0
	g.base.Reset(policy)
}

// RenderChannels renders the underlying monophonic generator once and distributes
// the in-phase samples to all channels according to their individual gains.
func (g *ChannelScaleGenerator) RenderChannels(dst [][]float64) {
	if len(dst) == 0 || len(dst[0]) == 0 {
		return
	}
	numFrames := len(dst[0])
	if len(g.cache) < numFrames {
		g.cache = make([]float64, numFrames)
	}
	buf := g.cache[:numFrames]
	g.base.Render(buf)

	for ch := 0; ch < len(dst); ch++ {
		scale := 1.0
		if ch < len(g.scales) {
			scale = g.scales[ch]
		}
		chDst := dst[ch]
		if scale == 1.0 {
			copy(chDst, buf)
		} else if scale == 0.0 {
			for i := 0; i < numFrames; i++ {
				chDst[i] = 0.0
			}
		} else {
			for i := 0; i < numFrames; i++ {
				chDst[i] = buf[i] * scale
			}
		}
	}
	g.currentCh = 0
}

// Render executes when channels are rendered one by one. It renders the underlying
// generator only on channel 0 and reuses the cached samples for subsequent channels.
func (g *ChannelScaleGenerator) Render(dst []float64) {
	numFrames := len(dst)
	if g.currentCh == 0 {
		if len(g.cache) < numFrames {
			g.cache = make([]float64, numFrames)
		}
		g.base.Render(g.cache[:numFrames])
	}

	scale := 1.0
	if g.currentCh < len(g.scales) {
		scale = g.scales[g.currentCh]
	}

	buf := g.cache[:numFrames]
	if scale == 1.0 {
		copy(dst, buf)
	} else if scale == 0.0 {
		for i := 0; i < numFrames; i++ {
			dst[i] = 0.0
		}
	} else {
		for i := 0; i < numFrames; i++ {
			dst[i] = buf[i] * scale
		}
	}

	g.currentCh = (g.currentCh + 1) % g.numCh
}

func (g *ChannelScaleGenerator) MaxPeak() float64 {
	basePeak := g.base.MaxPeak()
	maxScale := 0.0
	for _, s := range g.scales {
		if s > maxScale {
			maxScale = s
		}
	}
	return basePeak * maxScale
}

func dbfsToLinear(dbfs float64) float64 {
	if dbfs >= 0.0 {
		return 1.0
	}
	if dbfs <= -144.0 {
		return 0.0
	}
	return math.Pow(10.0, dbfs/20.0)
}
