//go:build windows

package wasapi

import (
	"testing"
	"unsafe"
)

func TestStructOffsets(t *testing.T) {
	var wf WAVEFORMATEXTENSIBLE
	t.Logf("sizeof(WAVEFORMATEXTENSIBLE) = %d", unsafe.Sizeof(wf))
	t.Logf("offset ValidBitsPerSample = %d", unsafe.Offsetof(wf.ValidBitsPerSample))
	t.Logf("offset DwChannelMask = %d", unsafe.Offsetof(wf.DwChannelMask))
	t.Logf("offset SubFormat = %d", unsafe.Offsetof(wf.SubFormat))
	if unsafe.Sizeof(wf) != 40 {
		t.Fatalf("sizeof(WAVEFORMATEXTENSIBLE) = %d, want 40", unsafe.Sizeof(wf))
	}
	if unsafe.Offsetof(wf.ValidBitsPerSample) != 18 {
		t.Fatalf("offset ValidBitsPerSample = %d, want 18", unsafe.Offsetof(wf.ValidBitsPerSample))
	}
	if unsafe.Offsetof(wf.DwChannelMask) != 20 {
		t.Fatalf("offset DwChannelMask = %d, want 20", unsafe.Offsetof(wf.DwChannelMask))
	}
	if unsafe.Offsetof(wf.SubFormat) != 24 {
		t.Fatalf("offset SubFormat = %d, want 24", unsafe.Offsetof(wf.SubFormat))
	}
}
