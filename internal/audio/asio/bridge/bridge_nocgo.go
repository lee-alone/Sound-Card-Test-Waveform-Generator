//go:build !cgo || !windows

package bridge

import "errors"

// CGODriver fallback when CGO is disabled or on non-Windows platforms.
type CGODriver struct{}

func NewCGODriver(clsid string) (*CGODriver, error) {
	return nil, errors.New("CGO and Windows environment required for live ASIO hardware COM bridge")
}

func (d *CGODriver) Init(sysHandle uintptr) error { return nil }
func (d *CGODriver) GetDriverName() string        { return "" }
func (d *CGODriver) GetDriverVersion() int        { return 0 }
func (d *CGODriver) GetErrorMessage() string      { return "" }
func (d *CGODriver) Start() error                 { return nil }
func (d *CGODriver) Stop() error                  { return nil }
func (d *CGODriver) GetChannels() (int, int, error) {
	return 0, 0, errors.New("CGO disabled")
}
func (d *CGODriver) GetLatencies() (int, int, error) {
	return 0, 0, errors.New("CGO disabled")
}
func (d *CGODriver) GetBufferSize() (BufferSizeInfo, error) {
	return BufferSizeInfo{}, errors.New("CGO disabled")
}
func (d *CGODriver) CanSampleRate(sampleRate float64) bool { return false }
func (d *CGODriver) GetSampleRate() (float64, error) {
	return 0, errors.New("CGO disabled")
}
func (d *CGODriver) SetSampleRate(sampleRate float64) error {
	return errors.New("CGO disabled")
}
func (d *CGODriver) GetClockSources() ([]ClockSource, error) {
	return nil, errors.New("CGO disabled")
}
func (d *CGODriver) SetClockSource(reference int) error {
	return errors.New("CGO disabled")
}
func (d *CGODriver) GetChannelInfo(channel int, isInput bool) (ChannelInfo, error) {
	return ChannelInfo{}, errors.New("CGO disabled")
}
func (d *CGODriver) CreateBuffers(numChannels int, bufferSize int) error {
	return errors.New("CGO disabled")
}
func (d *CGODriver) DisposeBuffers() error  { return nil }
func (d *CGODriver) ControlPanel() error    { return nil }
func (d *CGODriver) Future(selector int, opt uintptr) error {
	return errors.New("CGO disabled")
}
func (d *CGODriver) GetWaitHandle() uintptr      { return 0 }
func (d *CGODriver) GetActiveBufferIndex() int   { return 0 }
func (d *CGODriver) WriteInterleavedPCM(bufferIndex int, src []byte, numFrames int, bytesPerSample int, numChannels int) {
}
func (d *CGODriver) WritePlanarDSD(bufferIndex int, channel int, src []byte) {}
func (d *CGODriver) Close() error                                            { return nil }
