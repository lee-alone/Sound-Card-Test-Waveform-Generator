//go:build windows

package wasapi

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DLL imports
var (
	modOle32 = windows.NewLazySystemDLL("ole32.dll")
	modAvrt  = windows.NewLazySystemDLL("avrt.dll")

	procCoInitializeEx                  = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize                  = modOle32.NewProc("CoUninitialize")
	procCoCreateInstance                = modOle32.NewProc("CoCreateInstance")
	procPropVariantClear                = modOle32.NewProc("PropVariantClear")
	procCoTaskMemFree                   = modOle32.NewProc("CoTaskMemFree")
	procAvSetMmThreadCharacteristics    = modAvrt.NewProc("AvSetMmThreadCharacteristicsW")
	procAvRevertMmThreadCharacteristics = modAvrt.NewProc("AvRevertMmThreadCharacteristics")
)

// COM constants
const (
	COINIT_MULTITHREADED     = 0x0
	COINIT_APARTMENTTHREADED = 0x2
	COINIT_DISABLE_OLE1DDE   = 0x4

	CLSCTX_ALL           = 0x17
	CLSCTX_INPROC_SERVER = 0x1

	STGM_READ = 0x00000000
)

// HRESULT constants
const (
	S_OK                                 int32 = 0
	S_FALSE                              int32 = 1
	E_FAIL                               int32 = -2147467259 // 0x80004005
	E_POINTER                            int32 = -2147467261 // 0x80004003
	E_INVALIDARG                         int32 = -2147024809 // 0x80070057
	RPC_E_CHANGED_MODE                   int32 = -2147417850 // 0x80010106
	AUDCLNT_E_NOT_INITIALIZED            int32 = -2004287487 // 0x88890001
	AUDCLNT_E_ALREADY_INITIALIZED        int32 = -2004287486 // 0x88890002
	AUDCLNT_E_DEVICE_INVALIDATED         int32 = -2004287484 // 0x88890004
	AUDCLNT_E_UNSUPPORTED_FORMAT         int32 = -2004287480 // 0x88890008
	AUDCLNT_E_EXCLUSIVE_MODE_NOT_ALLOWED int32 = -2004287474 // 0x8889000E
	AUDCLNT_E_DEVICE_IN_USE              int32 = -2004287478 // 0x8889000A
	AUDCLNT_E_BUFFER_SIZE_NOT_ALIGNED    int32 = -2004287463 // 0x88890019
)

// WASAPI Share and Stream Flags
const (
	AUDCLNT_SHAREMODE_SHARED    = 0
	AUDCLNT_SHAREMODE_EXCLUSIVE = 1

	AUDCLNT_STREAMFLAGS_EVENTCALLBACK       = 0x00040000
	AUDCLNT_STREAMFLAGS_NOPERSIST           = 0x00080000
	AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM      = 0x80000000
	AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY = 0x08000000
)

// DataFlow and Role for MMDeviceEnumerator
const (
	EDataFlowRender  = 0
	EDataFlowCapture = 1
	EDataFlowAll     = 2

	ERoleConsole        = 0
	ERoleMultimedia     = 1
	ERoleCommunications = 2

	DEVICE_STATE_ACTIVE     = 0x00000001
	DEVICE_STATE_DISABLED   = 0x00000002
	DEVICE_STATE_NOTPRESENT = 0x00000004
	DEVICE_STATE_UNPLUGGED  = 0x00000008
	DEVICE_STATEMASK_ALL    = 0x0000000F
)

// Wave format tags
const (
	WAVE_FORMAT_PCM        = 1
	WAVE_FORMAT_IEEE_FLOAT = 3
	WAVE_FORMAT_EXTENSIBLE = 0xFFFE
)

// GUID definitions
var (
	CLSID_MMDeviceEnumerator = windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	IID_IMMDeviceEnumerator  = windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	IID_IAudioClient         = windows.GUID{Data1: 0x1CB9AD4C, Data2: 0xDBFA, Data3: 0x4C32, Data4: [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
	IID_IAudioRenderClient   = windows.GUID{Data1: 0xF294ACFC, Data2: 0x3146, Data3: 0x4483, Data4: [8]byte{0xA7, 0xBF, 0xAD, 0xDC, 0xA7, 0xC2, 0x60, 0xE2}}

	KSDATAFORMAT_SUBTYPE_PCM        = windows.GUID{Data1: 0x00000001, Data2: 0x0000, Data3: 0x0010, Data4: [8]byte{0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}}
	KSDATAFORMAT_SUBTYPE_IEEE_FLOAT = windows.GUID{Data1: 0x00000003, Data2: 0x0000, Data3: 0x0010, Data4: [8]byte{0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}}

	// PKEY_Device_FriendlyName: {A45C254E-DF1C-4EFD-8020-67D146A850E0}, 14
	PKEY_Device_FriendlyName = PropertyKey{
		FMTID: windows.GUID{Data1: 0xa45c254e, Data2: 0xdf1c, Data3: 0x4efd, Data4: [8]byte{0x80, 0x20, 0x67, 0xd1, 0x46, 0xa8, 0x50, 0xe0}},
		PID:   14,
	}
)

// PropertyKey defines a Windows PROPERTYKEY structure
type PropertyKey struct {
	FMTID windows.GUID
	PID   uint32
}

// PropVariant represents a Windows PROPVARIANT structure
type PropVariant struct {
	VT         uint16
	Reserved1  uint16
	Reserved2  uint16
	Reserved3  uint16
	ValPointer uintptr
	Reserved4  uintptr
}

// WAVEFORMATEX structure
type WAVEFORMATEX struct {
	WFormatTag      uint16
	NChannels       uint16
	NSamplesPerSec  uint32
	NAvgBytesPerSec uint32
	NBlockAlign     uint16
	WBitsPerSample  uint16
	CBSize          uint16
}

// WAVEFORMATEXTENSIBLE structure with exact 40-byte Windows ABI layout.
type WAVEFORMATEXTENSIBLE struct {
	// Embedded WAVEFORMATEX (18 bytes, no padding)
	WFormatTag      uint16 // offset 0
	NChannels       uint16 // offset 2
	NSamplesPerSec  uint32 // offset 4
	NAvgBytesPerSec uint32 // offset 8
	NBlockAlign     uint16 // offset 12
	WBitsPerSample  uint16 // offset 14
	CBSize          uint16 // offset 16

	// Extensible fields (22 bytes)
	ValidBitsPerSample uint16       // offset 18
	DwChannelMask      uint32       // offset 20
	SubFormat          windows.GUID // offset 24
}

// SUCCEEDED tests whether an HRESULT indicates success (>= 0).
func SUCCEEDED(hr int32) bool {
	return hr >= 0
}

// FAILED tests whether an HRESULT indicates failure (< 0).
func FAILED(hr int32) bool {
	return hr < 0
}

// CoInitializeExWrapper initializes COM on the current OS thread.
// It succeeds for S_OK and S_FALSE, but rejects RPC_E_CHANGED_MODE and other errors.
func CoInitializeExWrapper(flags uint32) error {
	r1, _, _ := procCoInitializeEx.Call(0, uintptr(flags))
	hr := int32(r1)
	if hr == S_OK || hr == S_FALSE {
		return nil
	}
	return fmt.Errorf("CoInitializeEx failed with HRESULT 0x%08X (%d)", uint32(hr), hr)
}

// CoUninitializeWrapper uninitializes COM.
func CoUninitializeWrapper() {
	procCoUninitialize.Call()
}

// SetMmThreadCharacteristics sets MMCSS characteristics for pro-audio streaming.
func SetMmThreadCharacteristics(taskName string) (windows.Handle, uint32, error) {
	taskNameUTF16, err := windows.UTF16PtrFromString(taskName)
	if err != nil {
		return 0, 0, err
	}
	var taskIndex uint32
	r1, _, errSys := procAvSetMmThreadCharacteristics.Call(
		uintptr(unsafe.Pointer(taskNameUTF16)),
		uintptr(unsafe.Pointer(&taskIndex)),
	)
	if r1 == 0 {
		return 0, 0, fmt.Errorf("AvSetMmThreadCharacteristicsW failed: %v", errSys)
	}
	return windows.Handle(r1), taskIndex, nil
}

// RevertMmThreadCharacteristics reverts MMCSS characteristics.
func RevertMmThreadCharacteristics(handle windows.Handle) error {
	if handle == 0 {
		return nil
	}
	r1, _, errSys := procAvRevertMmThreadCharacteristics.Call(uintptr(handle))
	if r1 == 0 {
		return fmt.Errorf("AvRevertMmThreadCharacteristics failed: %v", errSys)
	}
	return nil
}
