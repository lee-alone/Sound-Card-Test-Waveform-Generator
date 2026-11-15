package giuui

import (
	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
)

type BenchPreset struct {
	Name        string
	Description string
	Waveform    controller.WaveformType
	Frequency   float32
	VolumeDBFS  float32
	SweepStart  float32
	SweepEnd    float32
	SweepDur    float32
	ChannelMode ChannelMode
}

var StandardBenchPresets = []BenchPreset{
	{
		Name: "1 kHz / 0 dBFS", Description: "全幅 THD+N 基准",
		Waveform: controller.WaveformSine, Frequency: 1000, VolumeDBFS: 0, ChannelMode: ChannelModeStereo,
	},
	{
		Name: "1 kHz / -1 dBFS", Description: "防削顶与过载容限",
		Waveform: controller.WaveformSine, Frequency: 1000, VolumeDBFS: -1, ChannelMode: ChannelModeStereo,
	},
	{
		Name: "1 kHz / -60 dBFS", Description: "动态范围与底噪",
		Waveform: controller.WaveformSine, Frequency: 1000, VolumeDBFS: -60, ChannelMode: ChannelModeStereo,
	},
	{
		Name: "20 Hz – 20 kHz 对数扫频", Description: "全频带频率响应",
		Waveform: controller.WaveformLogarithmicSweep, SweepStart: 20, SweepEnd: 20000, SweepDur: 5,
		VolumeDBFS: -6, ChannelMode: ChannelModeStereo,
	},
	{
		Name: "SMPTE 互调", Description: "60 Hz + 7 kHz",
		Waveform: controller.WaveformMultitoneSMPTE, VolumeDBFS: -6, ChannelMode: ChannelModeStereo,
	},
	{
		Name: "CCIF 差频", Description: "19 kHz + 20 kHz",
		Waveform: controller.WaveformMultitoneCCIF, VolumeDBFS: -6, ChannelMode: ChannelModeStereo,
	},
	{
		Name: "CMRR 差分反相", Description: "右声道 180° 反相",
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
		labels[i] = preset.Name
	}

	preview := "选择标准测量预设"
	description := "载入后立即应用完整信号参数"
	if int(b.state.SelectedPresetIndex) >= 0 && int(b.state.SelectedPresetIndex) < len(StandardBenchPresets) {
		preset := StandardBenchPresets[b.state.SelectedPresetIndex]
		preview = preset.Name
		description = preset.Description
	}

	return g.Row(
		captionLabel("测量预设"),
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
