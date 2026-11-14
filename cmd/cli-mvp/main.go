package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/wasapi"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/dsd"
	"sound-signal-generator/internal/format"
)

func main() {
	listFlag := flag.Bool("list", false, "List available audio devices and exit")
	probeFlag := flag.Bool("probe", false, "Probe candidate formats on selected device and exit")
	deviceFlag := flag.String("device", "", "Device ID or substring of device name (empty for default)")
	rateFlag := flag.Int("rate", 48000, "Sample rate in Hz (44100, 48000, 88200, 96000, 176400, 192000)")
	freqFlag := flag.Float64("freq", 1000.0, "Sine signal frequency in Hz")
	levelFlag := flag.Float64("level", -6.0, "Signal peak level in dBFS (<= 0.0)")
	formatFlag := flag.String("format", "16", "Sample format: 16 (int16), 24p (24-bit packed), 24in32 (24-in-32), 32f (32-bit float)")
	durationFlag := flag.Duration("duration", 10*time.Second, "Run duration (e.g. 10s, 5m, 1h). 0 for infinite")
	soakFlag := flag.Bool("soak", false, "Execute automated Soak Test Matrix (1kHz, 30kHz, 60kHz)")
	dopFlag := flag.Bool("dop", false, "Enable dCS DoP v1.1 DSD carrier transmission mode (DSD64/DSD128)")
	dsdFlag := flag.String("dsd", "64", "DSD rate specification for DoP: 64 (DSD64 @ 176.4kHz) or 128 (DSD128 @ 352.8kHz)")
	confirmDoPFlag := flag.Bool("confirm-dop-risk", false, "Explicitly confirm DoP hearing/equipment safety risk (required to unlock DoP)")

	flag.Parse()

	// Lock OS thread for COM
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	backend := wasapi.NewBackend()
	ctrl := controller.NewAppController(backend)
	if err := ctrl.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing audio backend: %v\n", err)
		os.Exit(1)
	}
	defer ctrl.Close()

	// 1. List devices
	devices, err := ctrl.EnumerateDevices()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error enumerating devices: %v\n", err)
		os.Exit(1)
	}

	if *listFlag {
		fmt.Printf("Discovered %d audio endpoints:\n", len(devices))
		for i, d := range devices {
			defTag := ""
			if d.IsDefault {
				defTag = " [DEFAULT]"
			}
			fmt.Printf(" [%d] %s (State: %s)%s\n     ID: %s\n", i, d.Name, d.State, defTag, d.ID)
		}
		return
	}

	// 2. Resolve selected device
	targetDeviceID := *deviceFlag
	if targetDeviceID != "" {
		found := false
		for _, d := range devices {
			if d.ID == targetDeviceID || strings.Contains(strings.ToLower(d.Name), strings.ToLower(targetDeviceID)) {
				targetDeviceID = d.ID
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Specified device '%s' not found.\n", *deviceFlag)
			os.Exit(1)
		}
	}

	if err := ctrl.SelectDevice(targetDeviceID); err != nil {
		fmt.Fprintf(os.Stderr, "Error selecting device: %v\n", err)
		os.Exit(1)
	}

	// 3. Probe capabilities
	if *probeFlag {
		candidates := audio.GenerateStandardCandidateMatrix(2)
		dev, err := backend.GetDevice(targetDeviceID)
		if targetDeviceID == "" {
			dev, err = backend.GetDefaultDevice()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get device: %v\n", err)
			os.Exit(1)
		}
		defer dev.Close()

		fmt.Printf("Probing capabilities for: %s\n", dev.Name())
		results, err := dev.ProbeCapabilities(candidates, audio.ShareModeExclusive)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Probe failed: %v\n", err)
			os.Exit(1)
		}

		for _, r := range results {
			statusTag := fmt.Sprintf("[%s]", r.Status)
			if r.Status == audio.StatusSupported {
				fmt.Printf("  %-15s %s\n", statusTag, r.Format)
			} else {
				fmt.Printf("  %-15s %s (HR: 0x%08X %s)\n", statusTag, r.Format, uint32(r.HResult), r.ErrorMessage)
			}
		}
		return
	}

	// 4. Soak Test Matrix
	if *soakFlag {
		runSoakTestMatrix(ctrl, targetDeviceID, *durationFlag)
		return
	}

	// 5. Playback configuration
	sampleFmt := parseSampleFormat(*formatFlag)

	if *dopFlag {
		// DoP safety risk confirmation
		if *confirmDoPFlag {
			ctrl.DoPGuard().ConfirmViaFlag(true)
		} else {
			fmt.Println("\n" + strings.Repeat("!", 70))
			fmt.Println("【高危操作安全警告】")
			fmt.Println(ctrl.DoPGuard().WarningPrompt())
			fmt.Println(strings.Repeat("!", 70))
			fmt.Print("若确认无误，请输入 'YES' 解锁 DoP 测试模式: ")
			var input string
			_, _ = fmt.Scanln(&input)
			if !ctrl.DoPGuard().ConfirmWithToken(input) {
				fmt.Fprintln(os.Stderr, "\n[安全阻断] 用户未确认风险，DoP 模式拒绝启动，输出强制保持 OpenMuted。")
				os.Exit(1)
			}
		}

		targetDSD := dsd.DSD64
		if *dsdFlag == "128" || strings.ToLower(*dsdFlag) == "dsd128" {
			targetDSD = dsd.DSD128
		}

		dopReq := controller.DoPRequest{
			DeviceID:     targetDeviceID,
			DSDFormat:    targetDSD,
			SampleFormat: sampleFmt,
			Channels:     2,
			BitOrder:     dsd.BitOrderMSBFirst,
		}

		fmt.Println("=================================================================")
		fmt.Println("   Windows Professional Audio Test Signal Generator (DoP Mode)   ")
		fmt.Println("=================================================================")
		fmt.Printf("DSD Specification: %s (Carrier %d Hz)\n", targetDSD, targetDSD.DoPCarrierSampleRate())
		fmt.Printf("Marker Protocol:   dCS DoP v1.1 (0x05 / 0xFA alternating)\n")
		fmt.Printf("Container Format:  %s, Stereo\n", sampleFmt)
		fmt.Printf("Gain / DSP:        Bit-Perfect Unity (No DSP / No Fader)\n")

		if err := ctrl.ConfigureDoP(dopReq); err != nil {
			fmt.Fprintf(os.Stderr, "DoP Configuration error: %v\n", err)
			os.Exit(1)
		}
	} else {
		linearAmp := format.DBFSToAmplitude(*levelFlag)
		req := controller.SignalRequest{
			DeviceID:     targetDeviceID,
			SampleRate:   *rateFlag,
			SampleFormat: sampleFmt,
			Channels:     2,
			Frequency:    *freqFlag,
			Amplitude:    linearAmp,
		}

		fmt.Println("=================================================================")
		fmt.Println("   Windows Professional Audio Test Signal Generator (Phase 1-3)  ")
		fmt.Println("=================================================================")
		fmt.Printf("Frequency:     %.2f Hz\n", req.Frequency)
		fmt.Printf("Level:         %.2f dBFS (Linear: %.4f)\n", *levelFlag, linearAmp)
		fmt.Printf("Format:        %d Hz, %s, Stereo\n", req.SampleRate, req.SampleFormat)
		fmt.Printf("Mode:          WASAPI Exclusive Event-Driven\n")

		if err := ctrl.ConfigureSignal(req); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := ctrl.StartPlayback(); err != nil {
		fmt.Fprintf(os.Stderr, "Playback start failed: %v\n", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	var endTime time.Time
	if *durationFlag > 0 {
		endTime = startTime.Add(*durationFlag)
		fmt.Printf("Running for %v... Press Ctrl+C to abort.\n", *durationFlag)
	} else {
		fmt.Println("Running indefinitely... Press Ctrl+C to stop.")
	}

	running := true
	for running {
		select {
		case <-sigChan:
			fmt.Println("\nStop signal received. Gracefully fading out...")
			running = false
		case now := <-ticker.C:
			status := ctrl.Status()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			elapsed := now.Sub(startTime).Round(time.Second)
			diag := status.Diagnostics

			fmt.Printf("\r[%s] State: %-10s | Callbacks: %d | Underruns: %d | PeakRender: %v | HeapAlloc: %d KB",
				elapsed, status.State, diag.RenderCallbackCount, diag.UnderrunCount, diag.MaxRenderDuration, m.Alloc/1024)

			if !endTime.IsZero() && now.After(endTime) {
				fmt.Println("\nTarget duration reached.")
				running = false
			}
		}
	}

	_ = ctrl.StopPlayback()
	fmt.Println("\nPlayback safely stopped.")
}

func parseSampleFormat(s string) audio.SampleFormat {
	switch strings.ToLower(s) {
	case "16", "int16":
		return audio.SampleFormat16Int
	case "24p", "24packed":
		return audio.SampleFormat24Packed
	case "24in32", "24":
		return audio.SampleFormat24In32
	case "32f", "float", "32":
		return audio.SampleFormat32Float
	default:
		return audio.SampleFormat16Int
	}
}

func runSoakTestMatrix(ctrl *controller.AppController, deviceID string, perCaseDuration time.Duration) {
	if perCaseDuration <= 0 {
		perCaseDuration = 10 * time.Second
	}

	type testCase struct {
		Freq float64
		Rate int
		Name string
	}

	// Schedule T1.7 Test Matrix:
	// 1kHz (48k/96k/192k)
	// 30kHz (96k/192k)
	// 60kHz (176.4k/192k)
	matrix := []testCase{
		{Freq: 1000, Rate: 48000, Name: "1kHz @ 48kHz"},
		{Freq: 1000, Rate: 96000, Name: "1kHz @ 96kHz"},
		{Freq: 1000, Rate: 192000, Name: "1kHz @ 192kHz"},
		{Freq: 30000, Rate: 96000, Name: "30kHz @ 96kHz"},
		{Freq: 30000, Rate: 192000, Name: "30kHz @ 192kHz"},
		{Freq: 60000, Rate: 176400, Name: "60kHz @ 176.4kHz"},
		{Freq: 60000, Rate: 192000, Name: "60kHz @ 192kHz"},
	}

	fmt.Println("=================================================================")
	fmt.Println("            Phase 1 Automated Soak Test Suite                    ")
	fmt.Println("=================================================================")
	fmt.Printf("Running %d test cases for %v each...\n\n", len(matrix), perCaseDuration)

	allPassed := true
	for i, tc := range matrix {
		fmt.Printf("[%d/%d] Running %s ...\n", i+1, len(matrix), tc.Name)

		req := controller.SignalRequest{
			DeviceID:     deviceID,
			SampleRate:   tc.Rate,
			SampleFormat: audio.SampleFormat16Int,
			Channels:     2,
			Frequency:    tc.Freq,
			Amplitude:    format.DBFSToAmplitude(-6.0),
		}

		if err := ctrl.ConfigureSignal(req); err != nil {
			fmt.Printf("  FAILED Configuration: %v\n", err)
			allPassed = false
			continue
		}

		if err := ctrl.StartPlayback(); err != nil {
			fmt.Printf("  FAILED Playback Start: %v\n", err)
			allPassed = false
			continue
		}

		time.Sleep(perCaseDuration)

		status := ctrl.Status()
		_ = ctrl.StopPlayback()

		diag := status.Diagnostics
		if diag.UnderrunCount > 0 {
			fmt.Printf("  FAILED: Underrun count = %d\n", diag.UnderrunCount)
			allPassed = false
		} else {
			fmt.Printf("  PASSED: Underruns=0, Callbacks=%d, PeakRenderTime=%v\n",
				diag.RenderCallbackCount, diag.MaxRenderDuration)
		}
	}

	fmt.Println("\n=================================================================")
	if allPassed {
		fmt.Println(">> ALL SOAK TEST CASES PASSED SUCCESSFULLY (0 UNDERRUNS) <<")
	} else {
		fmt.Println(">> SOME SOAK TEST CASES FAILED <<")
	}
	fmt.Println("=================================================================")
}
