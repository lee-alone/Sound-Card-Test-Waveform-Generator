package audio

import (
	"testing"
	"time"
)

type compositeTestBackend struct {
	kind    BackendType
	id      string
	name    string
	device  Device
	initCnt int
}

func (b *compositeTestBackend) Type() BackendType { return b.kind }
func (b *compositeTestBackend) Initialize() error {
	b.initCnt++
	return nil
}
func (b *compositeTestBackend) Close() error { return nil }
func (b *compositeTestBackend) EnumerateDevices() ([]DeviceInfo, error) {
	return []DeviceInfo{{ID: b.id, Name: b.name, Backend: b.kind, IsDefault: true, Flow: FlowRender, State: StateActive}}, nil
}
func (b *compositeTestBackend) GetDevice(id string) (Device, error) {
	if id != b.id {
		return nil, ErrDeviceNotSelectedForTest
	}
	return b.device, nil
}
func (b *compositeTestBackend) GetDefaultDevice() (Device, error) { return b.device, nil }

type compositeTestDevice struct {
	id   string
	name string
}

func (d *compositeTestDevice) ID() string         { return d.id }
func (d *compositeTestDevice) Name() string       { return d.name }
func (d *compositeTestDevice) Flow() DeviceFlow   { return FlowRender }
func (d *compositeTestDevice) State() DeviceState { return StateActive }
func (d *compositeTestDevice) Close() error       { return nil }
func (d *compositeTestDevice) ProbeCapabilities([]AudioFormat, ShareMode) ([]FormatSupportResult, error) {
	return nil, nil
}
func (d *compositeTestDevice) OpenStream(StreamConfig) (AudioStream, error) {
	return nil, nil
}

type compositeTestError string

func (e compositeTestError) Error() string { return string(e) }

const ErrDeviceNotSelectedForTest = compositeTestError("not found")

func TestCompositeBackendEnumeratesAndRoutesBothBackends(t *testing.T) {
	wasapiDevice := &compositeTestDevice{id: "wasapi-dev", name: "Speakers"}
	asioDevice := &compositeTestDevice{id: "asio-dev", name: "ASIO DAC"}
	wasapiBackend := &compositeTestBackend{kind: BackendWASAPI, id: "wasapi-dev", name: "Speakers", device: wasapiDevice}
	asioBackend := &compositeTestBackend{kind: BackendASIO, id: "asio-dev", name: "ASIO DAC", device: asioDevice}
	composite := NewCompositeBackend(wasapiBackend, asioBackend)

	if err := composite.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	devices, err := composite.EnumerateDevices()
	if err != nil {
		t.Fatalf("EnumerateDevices failed: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 unified devices, got %d", len(devices))
	}
	if devices[0].Backend != BackendWASAPI || devices[1].Backend != BackendASIO {
		t.Fatalf("unexpected backend metadata: %+v", devices)
	}

	dev, key, err := composite.ResolveDevice("ASIO::asio-dev")
	if err != nil {
		t.Fatalf("ResolveDevice failed: %v", err)
	}
	if dev != asioDevice || key != "ASIO::asio-dev" {
		t.Fatalf("ASIO route mismatch: dev=%p key=%s", dev, key)
	}
	if _, _, err := composite.ResolveDevice("Speakers"); err == nil {
		t.Fatalf("composite IDs must be used for selection, raw names are not IDs")
	}
	dev, key, err = composite.ResolveDevice("wasapi-dev")
	if err != nil || dev != wasapiDevice || key != "WASAPI::wasapi-dev" {
		t.Fatalf("native ID compatibility route failed: dev=%p key=%s err=%v", dev, key, err)
	}
}

func TestCompositeBackendDefaultIsPrimary(t *testing.T) {
	primary := &compositeTestBackend{kind: BackendWASAPI, id: "main", device: &compositeTestDevice{id: "main"}}
	secondary := &compositeTestBackend{kind: BackendASIO, id: "asio", device: &compositeTestDevice{id: "asio"}}
	composite := NewCompositeBackend(primary, secondary)
	_ = composite.Initialize()
	dev, key, err := composite.ResolveDevice("")
	if err != nil || dev.ID() != "main" || key != "WASAPI::main" {
		t.Fatalf("default route mismatch: dev=%v key=%s err=%v", dev, key, err)
	}
}

var _ = time.Second
