//go:build windows

package wasapi

import (
	"errors"
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
)

type mockDoPDevice struct {
	name          string
	supportedFmts map[audio.AudioFormat]bool
	openErr       error
}

func (m *mockDoPDevice) ID() string             { return "mock-dop-id" }
func (m *mockDoPDevice) Name() string           { return m.name }
func (m *mockDoPDevice) Flow() audio.DeviceFlow { return audio.FlowRender }
func (m *mockDoPDevice) State() audio.DeviceState {
	return audio.StateActive
}

func (m *mockDoPDevice) ProbeCapabilities(candidates []audio.AudioFormat, shareMode audio.ShareMode) ([]audio.FormatSupportResult, error) {
	results := make([]audio.FormatSupportResult, len(candidates))
	for i, c := range candidates {
		status := audio.StatusUnsupported
		if m.supportedFmts != nil && m.supportedFmts[c] {
			status = audio.StatusSupported
		}
		results[i] = audio.FormatSupportResult{
			Format:  c,
			Status:  status,
			HResult: 0,
		}
	}
	return results, nil
}

func (m *mockDoPDevice) OpenStream(config audio.StreamConfig) (audio.AudioStream, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	return &mockDoPStream{format: config.Format}, nil
}

func (m *mockDoPDevice) Close() error {
	return nil
}

type mockDoPStream struct {
	format audio.AudioFormat
}

func (s *mockDoPStream) Start() error                                        { return nil }
func (s *mockDoPStream) Stop() error                                         { return nil }
func (s *mockDoPStream) GetBuffer(numFrames int) (uintptr, error)            { return 0, nil }
func (s *mockDoPStream) ReleaseBuffer(numFramesWritten int, flags uint32) error { return nil }
func (s *mockDoPStream) GetBufferSize() (uint32, error)                      { return 1764, nil }
func (s *mockDoPStream) GetDevicePeriod() (time.Duration, time.Duration, error) {
	return 10 * time.Millisecond, 5 * time.Millisecond, nil
}
func (s *mockDoPStream) GetWaitHandle() uintptr { return 0 }
func (s *mockDoPStream) Close() error           { return nil }

func TestIsDoPCarrierRate(t *testing.T) {
	validRates := []int{176400, 352800}
	for _, r := range validRates {
		if !IsDoPCarrierRate(r) {
			t.Errorf("expected %d to be recognized as valid DoP carrier rate", r)
		}
	}

	invalidRates := []int{44100, 48000, 88200, 96000, 192000, 705600}
	for _, r := range invalidRates {
		if IsDoPCarrierRate(r) {
			t.Errorf("expected %d NOT to be recognized as valid DoP carrier rate", r)
		}
	}
}

func TestCalculateDoPBufferParams(t *testing.T) {
	// 176.4kHz, 10ms
	frames, hns := CalculateDoPBufferParams(176400, 10*time.Millisecond, 10*time.Millisecond, 3*time.Millisecond)
	if frames != 1764 {
		t.Errorf("expected 1764 frames, got %d", frames)
	}
	if hns != 100000 { // 10ms in 100ns units = 100,000
		t.Errorf("expected 100000 hns, got %d", hns)
	}

	// 352.8kHz, 10ms
	frames128, hns128 := CalculateDoPBufferParams(352800, 10*time.Millisecond, 10*time.Millisecond, 5*time.Millisecond)
	if frames128 != 3528 {
		t.Errorf("expected 3528 frames, got %d", frames128)
	}
	if hns128 != 100000 {
		t.Errorf("expected 100000 hns, got %d", hns128)
	}

	// Minimum period constraint enforcement
	framesMin, _ := CalculateDoPBufferParams(176400, 2*time.Millisecond, 10*time.Millisecond, 5*time.Millisecond)
	if framesMin != 882 { // 5ms @ 176.4k = 882 frames
		t.Errorf("expected 882 frames due to minPeriod, got %d", framesMin)
	}
}

func TestNegotiateDoPFormat(t *testing.T) {
	fmt24in32 := audio.AudioFormat{
		SampleRate:   176400,
		Channels:     2,
		SampleFormat: audio.SampleFormat24In32,
		ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
	}
	fmt24packed := audio.AudioFormat{
		SampleRate:   176400,
		Channels:     2,
		SampleFormat: audio.SampleFormat24Packed,
		ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
	}

	// 1. Invalid rate
	_, err := NegotiateDoPFormat(&mockDoPDevice{}, 48000)
	if !errors.Is(err, ErrUnsupportedDoPRate) {
		t.Errorf("expected ErrUnsupportedDoPRate, got %v", err)
	}

	// 2. Device supports 24in32
	dev24in32 := &mockDoPDevice{
		name:          "DAC-24in32",
		supportedFmts: map[audio.AudioFormat]bool{fmt24in32: true},
	}
	negotiated, err := NegotiateDoPFormat(dev24in32, 176400)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if negotiated.SampleFormat != audio.SampleFormat24In32 {
		t.Errorf("expected 24in32, got %s", negotiated.SampleFormat)
	}

	// 3. Device only supports 24packed
	dev24p := &mockDoPDevice{
		name:          "DAC-24p",
		supportedFmts: map[audio.AudioFormat]bool{fmt24packed: true},
	}
	negotiatedP, err := NegotiateDoPFormat(dev24p, 176400)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if negotiatedP.SampleFormat != audio.SampleFormat24Packed {
		t.Errorf("expected 24packed, got %s", negotiatedP.SampleFormat)
	}

	// 4. Device supports neither
	devUnsupported := &mockDoPDevice{
		name:          "Non-DSD-DAC",
		supportedFmts: map[audio.AudioFormat]bool{},
	}
	_, err = NegotiateDoPFormat(devUnsupported, 176400)
	if err == nil {
		t.Fatal("expected error when format is unsupported, got nil")
	}
}

func TestOpenDoPExclusiveStream(t *testing.T) {
	fmt352 := audio.AudioFormat{
		SampleRate:   352800,
		Channels:     2,
		SampleFormat: audio.SampleFormat24In32,
		ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
	}
	dev := &mockDoPDevice{
		name:          "HighRes-DAC",
		supportedFmts: map[audio.AudioFormat]bool{fmt352: true},
	}

	stream, negotiatedFmt, err := OpenDoPExclusiveStream(dev, 352800, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("OpenDoPExclusiveStream failed: %v", err)
	}
	defer stream.Close()

	if negotiatedFmt.SampleRate != 352800 {
		t.Errorf("expected sample rate 352800, got %d", negotiatedFmt.SampleRate)
	}
}
