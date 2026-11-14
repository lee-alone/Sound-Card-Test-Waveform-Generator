//go:build !windows

package platform

import (
	"time"
)

// WaitForWaitHandle fallback for non-Windows platforms.
func WaitForWaitHandle(handle uintptr, timeout time.Duration) (bool, error) {
	if timeout > 0 {
		time.Sleep(timeout)
	}
	return true, nil
}

// EnableRealtimeAudioPriority fallback for non-Windows platforms.
func EnableRealtimeAudioPriority() (revert func(), err error) {
	return func() {}, nil
}
