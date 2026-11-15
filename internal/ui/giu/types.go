package giuui

import (
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/dsd"
	"sound-signal-generator/internal/engine"
	"sound-signal-generator/internal/ui/i18n"
)

// Supported sample rate candidates for manual selection
var SampleRateOptions = []int{
	44100,
	48000,
	88200,
	96000,
	176400,
	192000,
	352800,
	384000,
}

// Concise sample rate labels for compact display
var SampleRateLabels = []string{
	"44.1 kHz",
	"48.0 kHz",
	"88.2 kHz",
	"96.0 kHz",
	"176.4 kHz",
	"192.0 kHz",
	"352.8 kHz",
	"384.0 kHz",
}

// Supported sample format candidates for manual selection
var SampleFormatOptions = []audio.SampleFormat{
	audio.SampleFormat16Int,
	audio.SampleFormat24Packed,
	audio.SampleFormat24In32,
	audio.SampleFormat32Float,
}

// Concise sample format labels for compact display
var SampleFormatLabels = []string{
	"16-bit Int (2B)",
	"24-bit Packed (3B)",
	"24-in-32 LE (4B)",
	"32-bit Float (4B)",
}

// DoPFormatOption defines a DSD bitstream rate and the PCM carrier rate that
// transports it through the dCS DoP v1.1 container.
type DoPFormatOption struct {
	Format      dsd.DSDFormat
	LabelKey    string
	CarrierRate int
}

var DoPFormatOptions = []DoPFormatOption{
	{Format: dsd.DSD64, LabelKey: "dop.format.dsd64", CarrierRate: 176400},
	{Format: dsd.DSD128, LabelKey: "dop.format.dsd128", CarrierRate: 352800},
}

var DoPBitOrderOptions = []dsd.BitOrder{
	dsd.BitOrderMSBFirst,
	dsd.BitOrderLSBFirst,
}

var DoPBitOrderLabelKeys = []string{
	"dop.bit_order.msb",
	"dop.bit_order.lsb",
}

// NativeDSDFormatOptions are the direct ASIO 1-bit formats exposed by the
// high-rate Native DSD panel. DSD64/128 remain available to the controller but
// are not the primary hardware-validation target for this application.
var NativeDSDFormatOptions = []dsd.DSDFormat{dsd.DSD256, dsd.DSD512}

var NativeDSDFormatLabels = []string{
	"DSD256 · 11.2896 MHz",
	"DSD512 · 22.5792 MHz",
}

// WaveformOption defines waveform metadata for UI selection
type WaveformOption struct {
	ID       controller.WaveformType
	LabelKey string
}

// Concise waveform labels without oversized technical paragraphs
var WaveformOptions = []WaveformOption{
	{ID: controller.WaveformSine, LabelKey: "wave.sine"},
	{ID: controller.WaveformPolyBlepSquare, LabelKey: "wave.square"},
	{ID: controller.WaveformPolyBlepTriangle, LabelKey: "wave.triangle"},
	{ID: controller.WaveformLinearSweep, LabelKey: "wave.linear"},
	{ID: controller.WaveformLogarithmicSweep, LabelKey: "wave.log"},
	{ID: controller.WaveformWhiteNoise, LabelKey: "wave.white"},
	{ID: controller.WaveformPinkNoise, LabelKey: "wave.pink"},
	{ID: controller.WaveformMultitoneSMPTE, LabelKey: "wave.smpte"},
	{ID: controller.WaveformMultitoneCCIF, LabelKey: "wave.ccif"},
	{ID: controller.WaveformDoP, LabelKey: "wave.dop"},
}

func WaveformLabelAt(index int) string {
	idx := clampIndex(int32(index), len(WaveformOptions))
	if len(WaveformOptions) == 0 {
		return ""
	}
	return i18n.T(WaveformOptions[idx].LabelKey)
}

func WaveformLabels() []string {
	labels := make([]string, len(WaveformOptions))
	for i, option := range WaveformOptions {
		labels[i] = i18n.T(option.LabelKey)
	}
	return labels
}

func DoPFormatLabelAt(index int) string {
	idx := clampIndex(int32(index), len(DoPFormatOptions))
	if len(DoPFormatOptions) == 0 {
		return ""
	}
	return i18n.T(DoPFormatOptions[idx].LabelKey)
}

func DoPFormatLabels() []string {
	labels := make([]string, len(DoPFormatOptions))
	for i, option := range DoPFormatOptions {
		labels[i] = i18n.T(option.LabelKey)
	}
	return labels
}

func DoPBitOrderLabelAt(index int) string {
	idx := clampIndex(int32(index), len(DoPBitOrderLabelKeys))
	if len(DoPBitOrderLabelKeys) == 0 {
		return ""
	}
	return i18n.T(DoPBitOrderLabelKeys[idx])
}

func DoPBitOrderLabels() []string {
	labels := make([]string, len(DoPBitOrderLabelKeys))
	for i, key := range DoPBitOrderLabelKeys {
		labels[i] = i18n.T(key)
	}
	return labels
}

// ChannelMode represents stereo routing and differential phase mode
type ChannelMode int

const (
	ChannelModeStereo  ChannelMode = iota // Normal L/R stereo
	ChannelModeInvertR                    // Invert Right Channel 180° for CMRR differential test
	ChannelModeMono                       // Mono dual-channel sum
	ChannelModeSoloL                      // Solo Left (Mute Right)
	ChannelModeSoloR                      // Solo Right (Mute Left)
)

var ChannelModeLabelKeys = []string{
	"channel.stereo",
	"channel.invert_r",
	"channel.mono",
	"channel.solo_l",
	"channel.solo_r",
}

func ChannelModeLabelAt(index int) string {
	idx := clampIndex(int32(index), len(ChannelModeLabelKeys))
	if len(ChannelModeLabelKeys) == 0 {
		return ""
	}
	return i18n.T(ChannelModeLabelKeys[idx])
}

func ChannelModeLabels() []string {
	labels := make([]string, len(ChannelModeLabelKeys))
	for i, key := range ChannelModeLabelKeys {
		labels[i] = i18n.T(key)
	}
	return labels
}

// UIState encapsulates the GUI mutable state across loop renders.
type UIState struct {
	// Device & Format Selection
	DeviceNames         []string
	DeviceIDs           []string
	DeviceBackends      []audio.BackendType
	SelectedDeviceIndex int32
	IsExclusive         bool
	MasterTrimDB        float32 // DAC calibration trim [-6.0, +6.0 dB]

	// User Manual Output Format Selection
	SelectedSampleRateIndex   int32
	SelectedSampleFormatIndex int32

	// Generator Parameters
	SelectedWaveformIndex int32
	SelectedPresetIndex   int32
	Frequency             float32 // Signal frequency in Hz
	LeftVolumeDBFS        float32 // Left dBFS [-144, 0]
	RightVolumeDBFS       float32 // Right dBFS [-144, 0]
	LinkChannels          bool
	SelectedChannelMode   int32 // ChannelMode

	// Sweep & Noise Extra Params
	SweepStartFreq   float32
	SweepEndFreq     float32
	SweepDurationSec float32
	SweepLoop        bool
	NoiseSeed        int32

	// DoP (DSD over PCM) parameters and mandatory safety state.
	DoPFormatIndex   int32
	DoPBitOrderIndex int32
	DoPConfirmed     bool
	ShowDoPConfirm   bool

	// ASIO / Native DSD direct path state.
	AsioStatus            controller.AsioStatus
	NativeDSDFormatIndex  int32
	NativeDSDConfirmed    bool
	ShowNativeDSDConfirm  bool
	NativeDSDConfirmation string
	NativeDSDError        string
	SelectedClockIndex    int32
	AsioBufferSize        int32

	// Capabilities Probe
	ProbeResults     []audio.FormatSupportResult
	ShowProbeResults bool
	IsProbing        bool

	// Interface language selector index (0 = English, 1 = Simplified Chinese).
	SelectedLanguageIndex int32

	// Generic risk confirmation state used by square, multitone and ultrasound
	// signals. DoP and Native DSD retain their dedicated higher-friction flows.
	ShowSafetyConfirm    bool
	SafetyAssessment     controller.SafetyAssessment
	SuppressSafetyPrompt bool
	PendingSafetyStart   bool

	// Diagnostics & Stream Telemetry
	Diagnostics     controller.DiagnosticsReport
	StatusMessage   string
	ErrorMessage    string
	StreamStartTime time.Time
	// SignalStartTime marks sample zero of the currently installed DSP source.
	// It is deliberately separate from StreamStartTime: live waveform and sweep
	// edits replace the generator without restarting the audio stream.
	SignalStartTime time.Time
	IsActiveStream  bool
}

// NewUIState initializes a default UIState matching instrument standards.
func NewUIState() *UIState {
	return &UIState{
		DeviceNames:               []string{i18n.T("device.scanning")},
		DeviceIDs:                 []string{""},
		DeviceBackends:            []audio.BackendType{""},
		SelectedDeviceIndex:       0,
		IsExclusive:               true,
		MasterTrimDB:              0.0,
		SelectedSampleRateIndex:   1, // 48000 Hz default
		SelectedSampleFormatIndex: 0, // 16-bit Int default
		SelectedWaveformIndex:     0, // Sine default
		SelectedPresetIndex:       -1,
		Frequency:                 1000.0,
		LeftVolumeDBFS:            -6.0,
		RightVolumeDBFS:           -6.0,
		LinkChannels:              true,
		SelectedChannelMode:       0, // Stereo
		SweepStartFreq:            20.0,
		SweepEndFreq:              20000.0,
		SweepDurationSec:          5.0,
		SweepLoop:                 true,
		NoiseSeed:                 12345,
		DoPFormatIndex:            0, // DSD64
		DoPBitOrderIndex:          0, // MSB First
		NativeDSDFormatIndex:      0, // DSD256
		AsioBufferSize:            512,
		ShowProbeResults:          false,
		StatusMessage:             i18n.T("status.ready"),
		SelectedLanguageIndex:     int32(i18n.Index(i18n.Current())),
	}
}

// SelectedBackend returns the backend of the selected unified output device.
func (s *UIState) SelectedBackend() audio.BackendType {
	index := clampIndex(s.SelectedDeviceIndex, len(s.DeviceBackends))
	if len(s.DeviceBackends) == 0 {
		return ""
	}
	return s.DeviceBackends[index]
}

// IsAsioSelected reports whether the unified selector points at ASIO.
func (s *UIState) IsAsioSelected() bool {
	return s.SelectedBackend() == audio.BackendASIO
}

// NativeDSDTestVisible is true only while a successfully confirmed Native DSD
// stream is active. Selecting ASIO alone must never reveal the DSD test surface.
func (s *UIState) NativeDSDTestVisible() bool {
	return s.IsAsioSelected() && s.AsioStatus.NativeDSDMode
}

// ResetNativeDSDSession clears transient Native DSD UI state when entering the
// PCM path, switching devices, cancelling entry or leaving the test mode.
func (s *UIState) ResetNativeDSDSession() {
	s.AsioStatus.NativeDSDMode = false
	s.AsioStatus.NativeDSDActive = false
	s.AsioStatus.NativeDSDConfirmed = false
	s.AsioStatus.SelectedDSDFormat = ""
	s.NativeDSDConfirmed = false
	s.ShowNativeDSDConfirm = false
	s.NativeDSDConfirmation = ""
}

// SelectedNativeDSDFormat returns the validated high-rate Native DSD format.
func (s *UIState) SelectedNativeDSDFormat() dsd.DSDFormat {
	index := clampIndex(s.NativeDSDFormatIndex, len(NativeDSDFormatOptions))
	return NativeDSDFormatOptions[index]
}

// IsDoPSelected reports whether the generator is currently routed through the
// DoP carrier path.
func (s *UIState) IsDoPSelected() bool {
	index := clampIndex(s.SelectedWaveformIndex, len(WaveformOptions))
	return WaveformOptions[index].ID == controller.WaveformDoP
}

// IsOutputRunning reports whether the physical output stream is active. The
// flag is maintained by the transport buttons and refreshed from diagnostics,
// making it safe to use for locking format controls in either code path.
func (s *UIState) IsOutputRunning() bool {
	return s.IsActiveStream || s.Diagnostics.State == engine.StateRunning
}

// SelectedDoPFormat returns the validated DSD format currently selected.
func (s *UIState) SelectedDoPFormat() dsd.DSDFormat {
	index := clampIndex(s.DoPFormatIndex, len(DoPFormatOptions))
	return DoPFormatOptions[index].Format
}

// SelectedDoPBitOrder returns the validated bit order currently selected.
func (s *UIState) SelectedDoPBitOrder() dsd.BitOrder {
	index := clampIndex(s.DoPBitOrderIndex, len(DoPBitOrderLabelKeys))
	return DoPBitOrderOptions[index]
}

// ApplyDoPConstraints mirrors the visible DoP transport invariants in the UI.
// Unity gain and exclusive mode remain enforced by BuildExtendedRequest and the
// controller, but the operator's normal PCM level/format preferences are left
// untouched so returning to the PCM path does not unexpectedly jump to 0 dBFS.
func (s *UIState) ApplyDoPConstraints() {
	format := s.SelectedDoPFormat()
	carrierRate := format.DoPCarrierSampleRate()

	for i, rate := range SampleRateOptions {
		if rate == carrierRate {
			s.SelectedSampleRateIndex = int32(i)
			break
		}
	}
	for i, formatOption := range SampleFormatOptions {
		if formatOption == audio.SampleFormat24In32 {
			s.SelectedSampleFormatIndex = int32(i)
			break
		}
	}

}
