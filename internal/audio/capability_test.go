package audio

import (
	"testing"
)

func TestSampleFormatAttributes(t *testing.T) {
	tests := []struct {
		format         SampleFormat
		expectedBytes  int
		expectedValid  int
		expectedBits   int
		expectedString string
	}{
		{SampleFormat16Int, 2, 16, 16, "16-bit Integer"},
		{SampleFormat24Packed, 3, 24, 24, "24-bit Packed Integer"},
		{SampleFormat24In32, 4, 24, 32, "24-in-32-bit Integer"},
		{SampleFormat32Float, 4, 32, 32, "32-bit Float"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedString, func(t *testing.T) {
			if b := tt.format.BytesPerSample(); b != tt.expectedBytes {
				t.Errorf("BytesPerSample() = %d, want %d", b, tt.expectedBytes)
			}
			if v := tt.format.ValidBits(); v != tt.expectedValid {
				t.Errorf("ValidBits() = %d, want %d", v, tt.expectedValid)
			}
			if c := tt.format.ContainerBits(); c != tt.expectedBits {
				t.Errorf("ContainerBits() = %d, want %d", c, tt.expectedBits)
			}
			if s := tt.format.String(); s != tt.expectedString {
				t.Errorf("String() = %s, want %s", s, tt.expectedString)
			}
		})
	}
}

func TestCandidateMatrixGeneration(t *testing.T) {
	matrix := GenerateStandardCandidateMatrix(2)
	expectedCount := len(CandidateSampleRates) * len(CandidateSampleFormats)
	if len(matrix) != expectedCount {
		t.Fatalf("GenerateStandardCandidateMatrix(2) length = %d, want %d", len(matrix), expectedCount)
	}

	// Verify stereo mask and frame byte sizes
	for _, fmt := range matrix {
		if fmt.Channels != 2 {
			t.Errorf("expected 2 channels, got %d", fmt.Channels)
		}
		if fmt.ChannelMask != StandardSpeakerChannelMaskStereo {
			t.Errorf("expected stereo mask, got %d", fmt.ChannelMask)
		}
		expectedFrameBytes := fmt.SampleFormat.BytesPerSample() * 2
		if fmt.BytesPerFrame() != expectedFrameBytes {
			t.Errorf("BytesPerFrame() = %d, want %d", fmt.BytesPerFrame(), expectedFrameBytes)
		}
	}
}

func TestSupportStatusString(t *testing.T) {
	statuses := []struct {
		status SupportStatus
		str    string
	}{
		{StatusSupported, "Supported"},
		{StatusUnsupported, "Unsupported"},
		{StatusDeviceBusy, "DeviceBusy"},
		{StatusDriverError, "DriverError"},
		{SupportStatus(999), "Unknown"},
	}
	for _, s := range statuses {
		if s.status.String() != s.str {
			t.Errorf("status %v string = %q, want %q", s.status, s.status.String(), s.str)
		}
	}
}
