package audio

import "encoding/json"

// DeviceFlow specifies data flow direction (Render = Playback, Capture = Recording).
type DeviceFlow int

const (
	FlowRender DeviceFlow = iota
	FlowCapture
)

func (f DeviceFlow) String() string {
	switch f {
	case FlowRender:
		return "Render"
	case FlowCapture:
		return "Capture"
	default:
		return "Unknown"
	}
}

func (f DeviceFlow) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.String())
}

func (f *DeviceFlow) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		switch str {
		case "Render":
			*f = FlowRender
		case "Capture":
			*f = FlowCapture
		default:
			*f = FlowRender
		}
		return nil
	}
	var val int
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	*f = DeviceFlow(val)
	return nil
}

// DeviceState represents the operational state of a Windows audio endpoint.
type DeviceState int

const (
	StateActive DeviceState = iota
	StateDisabled
	StateNotPresent
	StateUnplugged
)

func (s DeviceState) String() string {
	switch s {
	case StateActive:
		return "Active"
	case StateDisabled:
		return "Disabled"
	case StateNotPresent:
		return "NotPresent"
	case StateUnplugged:
		return "Unplugged"
	default:
		return "Unknown"
	}
}

func (s DeviceState) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *DeviceState) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		switch str {
		case "Active":
			*s = StateActive
		case "Disabled":
			*s = StateDisabled
		case "NotPresent":
			*s = StateNotPresent
		case "Unplugged":
			*s = StateUnplugged
		default:
			*s = StateActive
		}
		return nil
	}
	var val int
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	*s = DeviceState(val)
	return nil
}

// DeviceInfo provides metadata describing a hardware audio endpoint.
type DeviceInfo struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	IsDefault bool        `json:"is_default"`
	Flow      DeviceFlow  `json:"flow"`
	State     DeviceState `json:"state"`
}

// Device abstracts an individual audio hardware device.
type Device interface {
	ID() string
	Name() string
	Flow() DeviceFlow
	State() DeviceState
	ProbeCapabilities(candidates []AudioFormat, shareMode ShareMode) ([]FormatSupportResult, error)
	OpenStream(config StreamConfig) (AudioStream, error)
	Close() error
}
