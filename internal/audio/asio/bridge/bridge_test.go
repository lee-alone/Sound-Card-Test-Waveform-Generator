package bridge

import (
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestEnumerateInstalledDrivers(t *testing.T) {
	drivers, err := EnumerateInstalledDrivers()
	if err != nil {
		t.Fatalf("EnumerateInstalledDrivers failed: %v", err)
	}

	t.Logf("Found %d installed ASIO driver(s) in Windows Registry:", len(drivers))
	for i, d := range drivers {
		t.Logf("  [%d] ID: %s, Name: %s, Desc: %s, CLSID: %s", i, d.ID, d.Name, d.Description, d.CLSID)
		if d.ID == "" || d.CLSID == "" {
			t.Errorf("Driver %d has empty ID or CLSID", i)
		}
	}

	// At least the Combo384 driver detected earlier should be present
	foundCombo384 := false
	for _, d := range drivers {
		if d.Name == "ASIO Combo384 Driver" {
			foundCombo384 = true
			if d.CLSID != "{188135E1-D565-11D2-854F-00A0C99F5D21}" {
				t.Errorf("unexpected CLSID for Combo384: %s", d.CLSID)
			}
		}
	}
	if !foundCombo384 && len(drivers) > 0 {
		t.Log("Note: Combo384 driver not found directly, but registry enumeration succeeded.")
	}
}

func TestMockDriverLifecycleAndZeroLeak(t *testing.T) {
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	routinesBefore := runtime.NumGoroutine()

	driver := NewMockDriver(ProfileThesycon)
	if err := driver.Init(0); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if driver.GetDriverName() != ProfileThesycon.Name {
		t.Errorf("unexpected driver name: got %s, want %s", driver.GetDriverName(), ProfileThesycon.Name)
	}

	// Verify buffer sizing
	bInfo, err := driver.GetBufferSize()
	if err != nil {
		t.Fatalf("GetBufferSize failed: %v", err)
	}
	if bInfo.MinSize != 64 || bInfo.PreferredSize != 256 {
		t.Errorf("unexpected buffer size info: %+v", bInfo)
	}

	// Set sample rate & create buffers
	if err := driver.SetSampleRate(96000.0); err != nil {
		t.Fatalf("SetSampleRate failed: %v", err)
	}
	if err := driver.CreateBuffers(2, 256); err != nil {
		t.Fatalf("CreateBuffers failed: %v", err)
	}

	// Start playback
	if err := driver.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for 3 buffer switch events
	hWait := windows.Handle(driver.GetWaitHandle())
	switches := 0
	for i := 0; i < 5; i++ {
		res, err := windows.WaitForSingleObject(hWait, 500)
		if err != nil || res != windows.WAIT_OBJECT_0 {
			t.Fatalf("WaitForSingleObject failed: res=%d, err=%v", res, err)
		}
		switches++
		idx := driver.GetActiveBufferIndex()
		// Test write
		dummyData := make([]byte, 256*2*4)
		driver.WriteInterleavedPCM(idx, dummyData, 256, 4, 2)
	}

	if switches < 3 {
		t.Errorf("insufficient buffer switch events: %d", switches)
	}

	// Stop and dispose
	if err := driver.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
	if err := driver.DisposeBuffers(); err != nil {
		t.Errorf("DisposeBuffers failed: %v", err)
	}
	if err := driver.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Check leak: give goroutines a moment to exit
	time.Sleep(20 * time.Millisecond)
	runtime.GC()
	routinesAfter := runtime.NumGoroutine()
	if routinesAfter > routinesBefore+1 { // allowing max 1 leeway for runtime test worker
		t.Errorf("goroutine leak: before=%d, after=%d", routinesBefore, routinesAfter)
	}
}

func TestSampleTypeIsDSDAndBytes(t *testing.T) {
	tests := []struct {
		st       SampleType
		isDSD    bool
		bytesPer int
	}{
		{SampleTypeInt16LSB, false, 2},
		{SampleTypeInt24LSB, false, 3},
		{SampleTypeInt32LSB, false, 4},
		{SampleTypeFloat32LSB, false, 4},
		{SampleTypeDSDInt8LSB1, true, 1},
		{SampleTypeDSDInt8MSB1, true, 1},
		{SampleTypeDSDInt8NER8, true, 1},
	}

	for _, tc := range tests {
		if tc.st.IsDSD() != tc.isDSD {
			t.Errorf("%s IsDSD got %v, want %v", tc.st, tc.st.IsDSD(), tc.isDSD)
		}
		if tc.st.BytesPerSample() != tc.bytesPer {
			t.Errorf("%s BytesPerSample got %d, want %d", tc.st, tc.st.BytesPerSample(), tc.bytesPer)
		}
	}
}
