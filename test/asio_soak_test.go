package test

import (
	"flag"
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio"
	"sound-signal-generator/internal/audio/asio/bridge"
	"sound-signal-generator/internal/dsp"
	"sound-signal-generator/internal/engine"
)

var run8hSoak = flag.Bool("soak8h", false, "Run full 8-hour ASIO soak test")

var (
	modKernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procGetProcessHandleCount = modKernel32.NewProc("GetProcessHandleCount")
)

// SoakSample records system health metrics at a specific test timestamp.
type SoakSample struct {
	Elapsed     time.Duration
	HeapAllocKB uint64
	Goroutines  int
	HandleCount uint32
	Underruns   uint64
}

func getOSHandleCount() uint32 {
	var count uint32
	proc := windows.CurrentProcess()
	_, _, _ = procGetProcessHandleCount.Call(uintptr(proc), uintptr(unsafe.Pointer(&count)))
	return count
}

func TestAsio8HourFullStackSoak(t *testing.T) {
	duration := 3 * time.Second // Fast test mode for CI
	sampleInterval := 500 * time.Millisecond

	if *run8hSoak || os.Getenv("SOAK_8H") == "true" {
		duration = 8 * time.Hour
		sampleInterval = 30 * time.Minute
		t.Log("=== Starting Full 8-Hour ASIO Soak Test ===")
	} else {
		t.Log("=== Running Accelerated ASIO Soak Test (pass -soak8h for full 8h run) ===")
	}

	// 1. Initialize full ASIO stack with realistic driver profile
	backend := asio.NewMockBackend(bridge.ProfileThesycon)
	if err := backend.Initialize(); err != nil {
		t.Fatalf("backend.Initialize failed: %v", err)
	}
	defer backend.Close()

	dev, err := backend.GetDefaultDevice()
	if err != nil {
		t.Fatalf("GetDefaultDevice failed: %v", err)
	}
	defer dev.Close()

	audioEng := engine.NewAudioEngine()
	defer audioEng.Close()

	streamFmt := audio.AudioFormat{
		SampleRate:   48000,
		Channels:     2,
		SampleFormat: audio.SampleFormat24In32,
	}

	engCfg := engine.EngineConfig{
		Device:         dev,
		Format:         streamFmt,
		ShareMode:      audio.ShareModeExclusive,
		BufferDuration: 10 * time.Millisecond,
		FadeDurationMs: 10.0,
	}

	if err := audioEng.Open(engCfg); err != nil {
		t.Fatalf("engine.Open failed: %v", err)
	}

	sineGen := dsp.NewSineGenerator()
	sineCfg := dsp.Config{
		SampleRate: 48000,
		Frequency:  1000.0,
		Amplitude:  0.5,
	}
	_ = sineGen.Prepare(sineCfg)

	if err := audioEng.Prime(sineGen); err != nil {
		t.Fatalf("engine.Prime failed: %v", err)
	}

	if err := audioEng.Start(); err != nil {
		t.Fatalf("engine.Start failed: %v", err)
	}

	// Warm up
	time.Sleep(100 * time.Millisecond)

	// Baseline snapshot
	runtime.GC()
	var baseMem runtime.MemStats
	runtime.ReadMemStats(&baseMem)
	baseGoroutines := runtime.NumGoroutine()
	baseHandles := getOSHandleCount()

	t.Logf("Baseline metrics: Heap=%d KB, Goroutines=%d, OS Handles=%d",
		baseMem.HeapAlloc/1024, baseGoroutines, baseHandles)

	startTime := time.Now()
	var samples []SoakSample

	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	timeout := time.After(duration)
loop:
	for {
		select {
		case <-timeout:
			break loop
		case <-ticker.C:
			runtime.GC()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			routines := runtime.NumGoroutine()
			handles := getOSHandleCount()
			diag := audioEng.Diagnostics()

			sample := SoakSample{
				Elapsed:     time.Since(startTime),
				HeapAllocKB: m.HeapAlloc / 1024,
				Goroutines:  routines,
				HandleCount: handles,
				Underruns:   diag.UnderrunCount,
			}
			samples = append(samples, sample)

			t.Logf("  [%v] Heap=%d KB, Goroutines=%d, Handles=%d, Underruns=%d",
				sample.Elapsed.Truncate(time.Millisecond), sample.HeapAllocKB, sample.Goroutines, sample.HandleCount, sample.Underruns)
		}
	}

	// Stop audio stream gracefully and close resources
	if err := audioEng.Stop(); err != nil {
		t.Errorf("engine.Stop failed: %v", err)
	}
	_ = audioEng.Close()
	_ = dev.Close()

	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	finalDiag := audioEng.Diagnostics()
	finalHandles := getOSHandleCount()
	finalGoroutines := runtime.NumGoroutine()

	t.Logf("\n=== Soak Test Final Verification ===")
	t.Logf("  Total Run Duration: %v", time.Since(startTime))
	t.Logf("  Final Underruns:    %d (Acceptance target: 0)", finalDiag.UnderrunCount)
	t.Logf("  Net Handle Growth:  %d (Acceptance target: <= 20 across runtime/threadpools)", int(finalHandles)-int(baseHandles))
	t.Logf("  Goroutine Count:    %d (Baseline: %d)", finalGoroutines, baseGoroutines)

	// 1. Underruns strictly 0
	if finalDiag.UnderrunCount != 0 {
		t.Errorf("underrun count was not zero: got %d", finalDiag.UnderrunCount)
	}

	// 2. Net OS handle growth must be within normal Windows NT threadpool tolerance (<= 20)
	handleGrowth := int(finalHandles) - int(baseHandles)
	if handleGrowth > 20 {
		t.Errorf("Windows process handle leak detected: net growth %d", handleGrowth)
	}

	// 3. Goroutines must not continuously grow monotonically
	if finalGoroutines > baseGoroutines+2 {
		t.Errorf("goroutine leak detected: baseline %d, final %d", baseGoroutines, finalGoroutines)
	}
}
