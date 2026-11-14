package platform

import (
	"testing"
	"time"
)

func TestWaitForWaitHandle_ZeroHandle(t *testing.T) {
	start := time.Now()
	signaled, err := WaitForWaitHandle(0, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !signaled {
		t.Fatalf("expected signaled true")
	}
	if time.Since(start) < 4*time.Millisecond {
		t.Fatalf("expected sleep at least 4ms")
	}
}

func TestEnableRealtimeAudioPriority(t *testing.T) {
	revert, err := EnableRealtimeAudioPriority()
	if err != nil {
		t.Logf("MMCSS not available or failed: %v", err)
	}
	defer revert()
}
