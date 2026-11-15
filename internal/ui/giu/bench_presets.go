package giuui

import (
	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/ui/i18n"
)

type BenchPreset struct {
	NameKey        string
	DescriptionKey string
	Waveform       controller.WaveformType
	Frequency      float32
	VolumeDBFS     float32
	SweepStart     float32
	SweepEnd       float32
	SweepDur       float32
	ChannelMode    ChannelMode
}

var StandardBenchPresets = []BenchPreset{
	{
		NameKey: "preset.full_scale.name", DescriptionKey: "preset.full_scale.desc",
		Waveform: controller.WaveformSine, Frequency: 1000, VolumeDBFS: 0, ChannelMode: ChannelModeStereo,
	},
	{
		NameKey: "preset.minus1.name", DescriptionKey: "preset.minus1.desc",
		Waveform: controller.WaveformSine, Frequency: 1000, VolumeDBFS: -1, ChannelMode: ChannelModeStereo,
	},
	{
		NameKey: "preset.minus60.name", DescriptionKey: "preset.minus60.desc",
		Waveform: controller.WaveformSine, Frequency: 1000, VolumeDBFS: -60, ChannelMode: ChannelModeStereo,
	},
	{
		NameKey: "preset.sweep.name", DescriptionKey: "preset.sweep.desc",
		Waveform: controller.WaveformLogarithmicSweep, SweepStart: 20, SweepEnd: 20000, SweepDur: 5,
		VolumeDBFS: -6, ChannelMode: ChannelModeStereo,
	},
	{
		NameKey: "preset.smpte.name", DescriptionKey: "preset.smpte.desc",
		Waveform: controller.WaveformMultitoneSMPTE, VolumeDBFS: -6, ChannelMode: ChannelModeStereo,
	},
	{
		NameKey: "preset.ccif.name", DescriptionKey: "preset.ccif.desc",
		Waveform: controller.WaveformMultitoneCCIF, VolumeDBFS: -6, ChannelMode: ChannelModeStereo,
	},
	{
		NameKey: "preset.cmrr.name", DescriptionKey: "preset.cmrr.desc",
		Waveform: controller.WaveformSine, Frequency: 1000, VolumeDBFS: -6, ChannelMode: ChannelModeInvertR,
	},
}

type BenchPresetsBar struct {
	state   *UIState
	onApply func()
}

func NewBenchPresetsBar(state *UIState, onApply func()) *BenchPresetsBar {
	return &BenchPresetsBar{state: state, onApply: onApply}
}

func (b *BenchPresetsBar) ApplyPreset(p BenchPreset) {
	for i, w := range WaveformOptions {
		if w.ID == p.Waveform {
			b.state.SelectedWaveformIndex = int32(i)
			break
		}
	}

	if p.Frequency > 0 {
		b.state.Frequency = p.Frequency
	}
	b.state.LeftVolumeDBFS = p.VolumeDBFS
	b.state.RightVolumeDBFS = p.VolumeDBFS
	b.state.LinkChannels = true
	b.state.SelectedChannelMode = int32(p.ChannelMode)

	if p.SweepEnd > 0 {
		b.state.SweepStartFreq = p.SweepStart
		b.state.SweepEndFreq = p.SweepEnd
		b.state.SweepDurationSec = p.SweepDur
		b.state.SweepLoop = true
	}

	b.onApply()
}

func (b *BenchPresetsBar) Build() g.Widget {
	labels := make([]string, len(StandardBenchPresets))
	for i, preset := range StandardBenchPresets {
		labels[i] = i18n.T(preset.NameKey)
	}

	preview := i18n.T("preset.select")
	description := i18n.T("preset.default_desc")
	if int(b.state.SelectedPresetIndex) >= 0 && int(b.state.SelectedPresetIndex) < len(StandardBenchPresets) {
		preset := StandardBenchPresets[b.state.SelectedPresetIndex]
		preview = i18n.T(preset.NameKey)
		description = i18n.T(preset.DescriptionKey)
	}

	return g.Row(
		captionLabel(i18n.T("preset.measurement")),
		g.Combo("##benchPreset", preview, labels, &b.state.SelectedPresetIndex).
			Size(px(270)).
			OnChange(func() {
				if int(b.state.SelectedPresetIndex) >= 0 && int(b.state.SelectedPresetIndex) < len(StandardBenchPresets) {
					b.ApplyPreset(StandardBenchPresets[b.state.SelectedPresetIndex])
				}
			}),
		captionLabel(description),
	)
}
