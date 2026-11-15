//go:build windows && cgo

package bridge

/*
#cgo windows CFLAGS: -I.
#cgo windows LDFLAGS: -lole32
#include "asio_bridge.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

// CGODriver wraps the native C/C++ ASIO bridge instance.
type CGODriver struct {
	handle C.AsioBridgeHandle
}

// NewCGODriver loads and instantiates the ASIO COM driver given its CLSID string.
func NewCGODriver(clsid string) (*CGODriver, error) {
	if clsid == "" {
		return nil, errors.New("empty CLSID")
	}

	cClsid := C.CString(clsid)
	defer C.free(unsafe.Pointer(cClsid))

	h := C.asio_bridge_create(cClsid)
	if h == nil {
		hr := uint32(C.asio_bridge_get_last_create_hresult())
		return nil, fmt.Errorf("failed to load ASIO COM server for CLSID %s (HRESULT 0x%08X%s)", clsid, hr, asioCreateHint(hr))
	}

	return &CGODriver{handle: h}, nil
}

func (d *CGODriver) Init(sysHandle uintptr) error {
	if d.handle == nil {
		return errors.New("driver not instantiated")
	}
	res := C.asio_bridge_init(d.handle, unsafe.Pointer(sysHandle))
	if res == 0 {
		return errors.New("ASIOInit returned ASIOFalse")
	}
	return nil
}

func (d *CGODriver) GetDriverName() string {
	if d.handle == nil {
		return ""
	}
	var nameBuf [128]C.char
	C.asio_bridge_get_driver_name(d.handle, &nameBuf[0])
	return C.GoString(&nameBuf[0])
}

func (d *CGODriver) GetDriverVersion() int {
	if d.handle == nil {
		return 0
	}
	return int(C.asio_bridge_get_driver_version(d.handle))
}

func (d *CGODriver) GetErrorMessage() string {
	if d.handle == nil {
		return ""
	}
	var msgBuf [256]C.char
	C.asio_bridge_get_error_message(d.handle, &msgBuf[0])
	return C.GoString(&msgBuf[0])
}

func (d *CGODriver) Start() error {
	if d.handle == nil {
		return errors.New("driver not instantiated")
	}
	errCode := C.asio_bridge_start(d.handle)
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return fmt.Errorf("ASIOStart error: %d", int(errCode))
	}
	return nil
}

func (d *CGODriver) Stop() error {
	if d.handle == nil {
		return errors.New("driver not instantiated")
	}
	errCode := C.asio_bridge_stop(d.handle)
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return fmt.Errorf("ASIOStop error: %d", int(errCode))
	}
	return nil
}

func (d *CGODriver) GetChannels() (numInputs, numOutputs int, err error) {
	if d.handle == nil {
		return 0, 0, errors.New("driver not instantiated")
	}
	var inCh, outCh C.long
	errCode := C.asio_bridge_get_channels(d.handle, &inCh, &outCh)
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return 0, 0, fmt.Errorf("ASIOGetChannels error: %d", int(errCode))
	}
	return int(inCh), int(outCh), nil
}

func (d *CGODriver) GetLatencies() (inputLatency, outputLatency int, err error) {
	if d.handle == nil {
		return 0, 0, errors.New("driver not instantiated")
	}
	var inLat, outLat C.long
	errCode := C.asio_bridge_get_latencies(d.handle, &inLat, &outLat)
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return 0, 0, fmt.Errorf("ASIOGetLatencies error: %d", int(errCode))
	}
	return int(inLat), int(outLat), nil
}

func (d *CGODriver) GetBufferSize() (BufferSizeInfo, error) {
	if d.handle == nil {
		return BufferSizeInfo{}, errors.New("driver not instantiated")
	}
	var minS, maxS, prefS, gran C.long
	errCode := C.asio_bridge_get_buffer_size(d.handle, &minS, &maxS, &prefS, &gran)
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return BufferSizeInfo{}, fmt.Errorf("ASIOGetBufferSize error: %d", int(errCode))
	}
	return BufferSizeInfo{
		MinSize:       int(minS),
		MaxSize:       int(maxS),
		PreferredSize: int(prefS),
		Granularity:   int(gran),
	}, nil
}

func (d *CGODriver) CanSampleRate(sampleRate float64) bool {
	if d.handle == nil {
		return false
	}
	res := C.asio_bridge_can_sample_rate(d.handle, C.double(sampleRate))
	return res == C.ASE_OK || res == C.ASE_SUCCESS
}

func (d *CGODriver) GetSampleRate() (float64, error) {
	if d.handle == nil {
		return 0, errors.New("driver not instantiated")
	}
	var sRate C.double
	errCode := C.asio_bridge_get_sample_rate(d.handle, &sRate)
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return 0, fmt.Errorf("ASIOGetSampleRate error: %d", int(errCode))
	}
	return float64(sRate), nil
}

func (d *CGODriver) SetSampleRate(sampleRate float64) error {
	if d.handle == nil {
		return errors.New("driver not instantiated")
	}
	errCode := C.asio_bridge_set_sample_rate(d.handle, C.double(sampleRate))
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return fmt.Errorf("ASIOSetSampleRate error: %d", int(errCode))
	}
	return nil
}

func (d *CGODriver) GetClockSources() ([]ClockSource, error) {
	if d.handle == nil {
		return nil, errors.New("driver not instantiated")
	}
	var clocks [32]C.ASIOClockSource
	var num C.long = 32
	errCode := C.asio_bridge_get_clock_sources(d.handle, &clocks[0], &num)
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return nil, fmt.Errorf("ASIOGetClockSources error: %d", int(errCode))
	}
	count := int(num)
	if count > 32 {
		count = 32
	}
	res := make([]ClockSource, count)
	for i := 0; i < count; i++ {
		res[i] = ClockSource{
			Index:           int(clocks[i].index),
			Name:            C.GoString(&clocks[i].name[0]),
			IsCurrentSource: clocks[i].isCurrentSource != 0,
		}
	}
	return res, nil
}

func (d *CGODriver) SetClockSource(reference int) error {
	if d.handle == nil {
		return errors.New("driver not instantiated")
	}
	errCode := C.asio_bridge_set_clock_source(d.handle, C.long(reference))
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return fmt.Errorf("ASIOSetClockSource error: %d", int(errCode))
	}
	return nil
}

func (d *CGODriver) GetChannelInfo(channel int, isInput bool) (ChannelInfo, error) {
	if d.handle == nil {
		return ChannelInfo{}, errors.New("driver not instantiated")
	}
	var info C.ASIOChannelInfo
	var inC C.int
	if isInput {
		inC = 1
	}
	errCode := C.asio_bridge_get_channel_info(d.handle, C.long(channel), inC, &info)
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return ChannelInfo{}, fmt.Errorf("ASIOGetChannelInfo error: %d", int(errCode))
	}
	return ChannelInfo{
		Channel:      int(info.channel),
		IsInput:      info.isInput != 0,
		IsActive:     info.isActive != 0,
		ChannelGroup: int(info.channelGroup),
		SampleType:   SampleType(info._type),
		Name:         C.GoString(&info.name[0]),
	}, nil
}

func (d *CGODriver) CreateBuffers(numChannels int, bufferSize int) error {
	if d.handle == nil {
		return errors.New("driver not instantiated")
	}
	errCode := C.asio_bridge_create_buffers(d.handle, C.long(numChannels), C.long(bufferSize))
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return fmt.Errorf("ASIOCreateBuffers error: %d", int(errCode))
	}
	return nil
}

func (d *CGODriver) DisposeBuffers() error {
	if d.handle == nil {
		return nil
	}
	_ = C.asio_bridge_dispose_buffers(d.handle)
	return nil
}

func (d *CGODriver) ControlPanel() error {
	if d.handle == nil {
		return errors.New("driver not instantiated")
	}
	_ = C.asio_bridge_control_panel(d.handle)
	return nil
}

func (d *CGODriver) Future(selector int, opt uintptr) error {
	if d.handle == nil {
		return errors.New("driver not instantiated")
	}
	errCode := C.asio_bridge_future(d.handle, C.long(selector), unsafe.Pointer(opt))
	if errCode != C.ASE_OK && errCode != C.ASE_SUCCESS {
		return fmt.Errorf("ASIOFuture error: %d", int(errCode))
	}
	return nil
}

func (d *CGODriver) GetWaitHandle() uintptr {
	if d.handle == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(C.asio_bridge_get_wait_handle(d.handle)))
}

func (d *CGODriver) GetActiveBufferIndex() int {
	if d.handle == nil {
		return 0
	}
	return int(C.asio_bridge_get_active_buffer_index(d.handle))
}

func (d *CGODriver) WriteInterleavedPCM(bufferIndex int, src []byte, numFrames int, bytesPerSample int, numChannels int) {
	if d.handle == nil || len(src) == 0 {
		return
	}
	C.asio_bridge_write_interleaved_pcm(d.handle, C.long(bufferIndex), unsafe.Pointer(&src[0]), C.long(numFrames), C.int(bytesPerSample), C.long(numChannels))
}

func (d *CGODriver) WritePlanarDSD(bufferIndex int, channel int, src []byte) {
	if d.handle == nil || len(src) == 0 {
		return
	}
	C.asio_bridge_write_planar_dsd(d.handle, C.long(bufferIndex), C.long(channel), unsafe.Pointer(&src[0]), C.long(len(src)))
}

func (d *CGODriver) Close() error {
	if d.handle != nil {
		C.asio_bridge_destroy(d.handle)
		d.handle = nil
	}
	return nil
}

func asioCreateHint(hr uint32) string {
	switch hr {
	case 0x80040154:
		return ": class not registered for this process architecture"
	case 0x80004002:
		return ": COM apartment/interface mismatch (ASIO drivers normally require STA)"
	case 0x8007007E:
		return ": ASIO driver DLL or one of its dependencies could not be loaded"
	case 0x80070005:
		return ": access denied while loading the ASIO driver"
	case 0x800401F0:
		return ": COM was not initialized"
	default:
		return ""
	}
}
