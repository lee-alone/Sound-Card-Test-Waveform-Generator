//go:build windows

package wasapi

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"sound-signal-generator/internal/audio"
)

func callCOM(obj uintptr, vtableIndex int, args ...uintptr) int32 {
	vtable := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtable + uintptr(vtableIndex)*unsafe.Sizeof(uintptr(0))))
	allArgs := make([]uintptr, 0, len(args)+1)
	allArgs = append(allArgs, obj)
	allArgs = append(allArgs, args...)
	r1, _, _ := syscall.SyscallN(fn, allArgs...)
	return int32(r1)
}

func releaseCOM(obj uintptr) uint32 {
	if obj == 0 {
		return 0
	}
	vtable := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtable + 2*unsafe.Sizeof(uintptr(0)))) // Release is at index 2
	r1, _, _ := syscall.SyscallN(fn, obj)
	return uint32(r1)
}

// Backend implements audio.AudioBackend for Windows WASAPI.
type Backend struct {
	initialized bool
}

// NewBackend creates a new WASAPI backend instance.
func NewBackend() *Backend {
	return &Backend{}
}

// Type returns the backend type identifier.
func (b *Backend) Type() audio.BackendType {
	return audio.BackendWASAPI
}

// Initialize initializes COM for the calling thread if needed. WASAPI and
// common ASIO drivers share the hosting thread, so use STA consistently.
// Initializing this thread as MTA first makes later Apartment-threaded ASIO
// driver creation fail with E_NOINTERFACE.
func (b *Backend) Initialize() error {
	runtime.LockOSThread()
	err := CoInitializeExWrapper(COINIT_APARTMENTTHREADED | COINIT_DISABLE_OLE1DDE)
	if err != nil {
		runtime.UnlockOSThread()
		return err
	}
	b.initialized = true
	return nil
}

// Close uninitializes the backend and COM on the current OS thread.
func (b *Backend) Close() error {
	if b.initialized {
		CoUninitializeWrapper()
		runtime.UnlockOSThread()
		b.initialized = false
	}
	return nil
}

func createDeviceEnumerator() (uintptr, error) {
	var enumerator uintptr
	r1, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&CLSID_MMDeviceEnumerator)),
		0,
		CLSCTX_ALL,
		uintptr(unsafe.Pointer(&IID_IMMDeviceEnumerator)),
		uintptr(unsafe.Pointer(&enumerator)),
	)
	hr := int32(r1)
	if FAILED(hr) {
		return 0, fmt.Errorf("CoCreateInstance(MMDeviceEnumerator) failed with HRESULT 0x%08X", uint32(hr))
	}
	return enumerator, nil
}

// EnumerateDevices lists all active and available render endpoints.
func (b *Backend) EnumerateDevices() ([]audio.DeviceInfo, error) {
	enumerator, err := createDeviceEnumerator()
	if err != nil {
		return nil, err
	}
	defer releaseCOM(enumerator)

	var pCollection uintptr
	hr := callCOM(enumerator, 3, uintptr(EDataFlowRender), uintptr(DEVICE_STATEMASK_ALL), uintptr(unsafe.Pointer(&pCollection)))
	if FAILED(hr) {
		return nil, fmt.Errorf("EnumAudioEndpoints failed with HRESULT 0x%08X", uint32(hr))
	}
	defer releaseCOM(pCollection)

	var count uint32
	hr = callCOM(pCollection, 3, uintptr(unsafe.Pointer(&count)))
	if FAILED(hr) {
		return nil, fmt.Errorf("IMMDeviceCollection::GetCount failed with HRESULT 0x%08X", uint32(hr))
	}

	// Also find default render endpoint to mark IsDefault
	var defaultID string
	var pDefaultDevice uintptr
	hrDef := callCOM(enumerator, 4, uintptr(EDataFlowRender), uintptr(ERoleConsole), uintptr(unsafe.Pointer(&pDefaultDevice)))
	if SUCCEEDED(hrDef) && pDefaultDevice != 0 {
		var pstrDefId *uint16
		if SUCCEEDED(callCOM(pDefaultDevice, 5, uintptr(unsafe.Pointer(&pstrDefId)))) && pstrDefId != nil {
			defaultID = windows.UTF16PtrToString(pstrDefId)
			procCoTaskMemFree.Call(uintptr(unsafe.Pointer(pstrDefId)))
		}
		releaseCOM(pDefaultDevice)
	}

	devices := make([]audio.DeviceInfo, 0, count)
	for i := uint32(0); i < count; i++ {
		var pDevice uintptr
		hr = callCOM(pCollection, 4, uintptr(i), uintptr(unsafe.Pointer(&pDevice)))
		if FAILED(hr) || pDevice == 0 {
			continue
		}

		info, err := getDeviceInfo(pDevice)
		releaseCOM(pDevice)
		if err != nil {
			continue
		}
		if info.ID == defaultID {
			info.IsDefault = true
		}
		devices = append(devices, info)
	}

	return devices, nil
}

// GetDevice returns a Device instance by endpoint ID string.
func (b *Backend) GetDevice(id string) (audio.Device, error) {
	enumerator, err := createDeviceEnumerator()
	if err != nil {
		return nil, err
	}
	defer releaseCOM(enumerator)

	idUTF16, err := windows.UTF16PtrFromString(id)
	if err != nil {
		return nil, err
	}

	var pDevice uintptr
	hr := callCOM(enumerator, 5, uintptr(unsafe.Pointer(idUTF16)), uintptr(unsafe.Pointer(&pDevice)))
	if FAILED(hr) || pDevice == 0 {
		return nil, fmt.Errorf("IMMDeviceEnumerator::GetDevice failed with HRESULT 0x%08X", uint32(hr))
	}

	return newWASAPIDevice(pDevice)
}

// GetDefaultDevice retrieves the default audio render device.
func (b *Backend) GetDefaultDevice() (audio.Device, error) {
	enumerator, err := createDeviceEnumerator()
	if err != nil {
		return nil, err
	}
	defer releaseCOM(enumerator)

	var pDevice uintptr
	hr := callCOM(enumerator, 4, uintptr(EDataFlowRender), uintptr(ERoleConsole), uintptr(unsafe.Pointer(&pDevice)))
	if FAILED(hr) || pDevice == 0 {
		return nil, fmt.Errorf("GetDefaultAudioEndpoint failed with HRESULT 0x%08X", uint32(hr))
	}

	return newWASAPIDevice(pDevice)
}

func getDeviceInfo(pDevice uintptr) (audio.DeviceInfo, error) {
	var info audio.DeviceInfo
	info.Backend = audio.BackendWASAPI
	info.Flow = audio.FlowRender

	var pstrId *uint16
	hr := callCOM(pDevice, 5, uintptr(unsafe.Pointer(&pstrId)))
	if FAILED(hr) || pstrId == nil {
		return info, fmt.Errorf("IMMDevice::GetId failed with HRESULT 0x%08X", uint32(hr))
	}
	info.ID = windows.UTF16PtrToString(pstrId)
	procCoTaskMemFree.Call(uintptr(unsafe.Pointer(pstrId)))

	var state uint32
	hr = callCOM(pDevice, 6, uintptr(unsafe.Pointer(&state)))
	if SUCCEEDED(hr) {
		switch state {
		case DEVICE_STATE_ACTIVE:
			info.State = audio.StateActive
		case DEVICE_STATE_DISABLED:
			info.State = audio.StateDisabled
		case DEVICE_STATE_NOTPRESENT:
			info.State = audio.StateNotPresent
		case DEVICE_STATE_UNPLUGGED:
			info.State = audio.StateUnplugged
		}
	}

	var pPropStore uintptr
	hr = callCOM(pDevice, 4, uintptr(STGM_READ), uintptr(unsafe.Pointer(&pPropStore)))
	if SUCCEEDED(hr) && pPropStore != 0 {
		defer releaseCOM(pPropStore)
		var pv PropVariant
		hr = callCOM(pPropStore, 5, uintptr(unsafe.Pointer(&PKEY_Device_FriendlyName)), uintptr(unsafe.Pointer(&pv)))
		if SUCCEEDED(hr) {
			if pv.VT == 31 && pv.ValPointer != 0 { // VT_LPWSTR = 31
				info.Name = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(pv.ValPointer)))
			}
			procPropVariantClear.Call(uintptr(unsafe.Pointer(&pv)))
		}
	}
	if info.Name == "" {
		info.Name = "Unknown Audio Device"
	}

	return info, nil
}
