package audio

import (
	"errors"
	"testing"
)

type MockDevice struct {
	id          string
	name        string
	supportFunc func(format AudioFormat) FormatSupportResult
}

func (m *MockDevice) ID() string         { return m.id }
func (m *MockDevice) Name() string       { return m.name }
func (m *MockDevice) Flow() DeviceFlow   { return FlowRender }
func (m *MockDevice) State() DeviceState { return StateActive }
func (m *MockDevice) ProbeCapabilities(candidates []AudioFormat, shareMode ShareMode) ([]FormatSupportResult, error) {
	if m.supportFunc == nil {
		return nil, errors.New("supportFunc not set")
	}
	results := make([]FormatSupportResult, len(candidates))
	for i, c := range candidates {
		results[i] = m.supportFunc(c)
	}
	return results, nil
}
func (m *MockDevice) OpenStream(config StreamConfig) (AudioStream, error) {
	return nil, nil
}
func (m *MockDevice) Close() error {
	return nil
}

func TestMockDeviceCapabilityProbing(t *testing.T) {
	mock := &MockDevice{
		id:   "mock-1",
		name: "Mock DAC",
		supportFunc: func(f AudioFormat) FormatSupportResult {
			if f.SampleRate == 192000 && f.SampleFormat == SampleFormat32Float {
				return FormatSupportResult{Format: f, Status: StatusSupported, HResult: 0}
			}
			if f.SampleRate == 176400 {
				return FormatSupportResult{Format: f, Status: StatusUnsupported, HResult: -2004287480, ErrorMessage: "AUDCLNT_E_UNSUPPORTED_FORMAT"}
			}
			if f.SampleRate == 96000 {
				return FormatSupportResult{Format: f, Status: StatusDeviceBusy, HResult: -2004287478, ErrorMessage: "AUDCLNT_E_DEVICE_IN_USE"}
			}
			return FormatSupportResult{Format: f, Status: StatusDriverError, HResult: -2147467259, ErrorMessage: "E_FAIL"}
		},
	}

	candidates := []AudioFormat{
		{SampleRate: 192000, Channels: 2, SampleFormat: SampleFormat32Float},
		{SampleRate: 176400, Channels: 2, SampleFormat: SampleFormat24Packed},
		{SampleRate: 96000, Channels: 2, SampleFormat: SampleFormat16Int},
		{SampleRate: 44100, Channels: 2, SampleFormat: SampleFormat24In32},
	}

	res, err := mock.ProbeCapabilities(candidates, ShareModeExclusive)
	if err != nil {
		t.Fatalf("ProbeCapabilities failed: %v", err)
	}
	if len(res) != 4 {
		t.Fatalf("expected 4 results, got %d", len(res))
	}
	if res[0].Status != StatusSupported {
		t.Errorf("result 0 status = %v, want Supported", res[0].Status)
	}
	if res[1].Status != StatusUnsupported {
		t.Errorf("result 1 status = %v, want Unsupported", res[1].Status)
	}
	if res[2].Status != StatusDeviceBusy {
		t.Errorf("result 2 status = %v, want DeviceBusy", res[2].Status)
	}
	if res[3].Status != StatusDriverError {
		t.Errorf("result 3 status = %v, want DriverError", res[3].Status)
	}
}

func TestDeviceFlowAndStateString(t *testing.T) {
	if FlowRender.String() != "Render" || FlowCapture.String() != "Capture" || DeviceFlow(99).String() != "Unknown" {
		t.Errorf("Flow string unexpected")
	}
	if StateActive.String() != "Active" || StateDisabled.String() != "Disabled" ||
		StateNotPresent.String() != "NotPresent" || StateUnplugged.String() != "Unplugged" ||
		DeviceState(99).String() != "Unknown" {
		t.Errorf("State string unexpected")
	}
	if ShareModeShared.String() != "Shared" || ShareModeExclusive.String() != "Exclusive" || ShareMode(99).String() != "Unknown" {
		t.Errorf("ShareMode string unexpected")
	}
}
