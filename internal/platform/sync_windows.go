//go:build windows

package platform

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modAvrt                             = windows.NewLazySystemDLL("avrt.dll")
	procAvSetMmThreadCharacteristicsW   = modAvrt.NewProc("AvSetMmThreadCharacteristicsW")
	procAvSetMmThreadPriority           = modAvrt.NewProc("AvSetMmThreadPriority")
	procAvRevertMmThreadCharacteristics = modAvrt.NewProc("AvRevertMmThreadCharacteristics")
)

const avrtPriorityCritical = 2

// WaitForWaitHandle waits for a Windows event handle up to timeout.
// Returns (true, nil) if signaled, (false, nil) if timed out.
func WaitForWaitHandle(handle uintptr, timeout time.Duration) (bool, error) {
	if handle == 0 {
		if timeout > 0 {
			time.Sleep(timeout)
		}
		return true, nil
	}

	timeoutMs := uint32(timeout.Milliseconds())
	if timeoutMs == 0 && timeout > 0 {
		timeoutMs = 1
	}

	res, err := windows.WaitForSingleObject(windows.Handle(handle), timeoutMs)
	if err != nil && err != windows.Errno(0) {
		return false, err
	}

	const waitTimeout = 0x00000102 // WAIT_TIMEOUT

	switch res {
	case windows.WAIT_OBJECT_0:
		return true, nil
	case waitTimeout:
		return false, nil
	default:
		return false, fmt.Errorf("WaitForSingleObject returned 0x%08X", res)
	}
}

// EnableRealtimeAudioPriority elevates the calling thread to MMCSS Pro Audio priority.
// It returns a revert closure to be called on thread exit.
func EnableRealtimeAudioPriority() (revert func(), err error) {
	if err := modAvrt.Load(); err != nil {
		return func() {}, nil
	}

	taskName, err := windows.UTF16PtrFromString("Pro Audio")
	if err != nil {
		return func() {}, err
	}

	var taskIndex uint32 = 0
	r0, _, err := procAvSetMmThreadCharacteristicsW.Call(
		uintptr(unsafe.Pointer(taskName)),
		uintptr(unsafe.Pointer(&taskIndex)),
	)
	if r0 == 0 {
		return func() {}, fmt.Errorf("AvSetMmThreadCharacteristicsW failed: %v", err)
	}

	handle := r0
	if result, _, callErr := procAvSetMmThreadPriority.Call(handle, avrtPriorityCritical); result == 0 {
		_, _, _ = procAvRevertMmThreadCharacteristics.Call(handle)
		return func() {}, fmt.Errorf("AvSetMmThreadPriority failed: %v", callErr)
	}

	return func() {
		_, _, _ = procAvRevertMmThreadCharacteristics.Call(handle)
	}, nil
}
