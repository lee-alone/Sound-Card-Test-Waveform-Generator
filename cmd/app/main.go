package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"sound-signal-generator/internal/audio/wasapi"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/platform"
)

//go:embed dist/*
var distFS embed.FS

func main() {
	checkEnvFlag := flag.Bool("check-env", false, "Perform environment pre-check for WebView2 Evergreen Runtime and exit")
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println("Windows Professional Audio Signal Generator v2.0 (Phase 4 Wails v3)")
		return
	}

	// 1. Mandatory Pre-Flight Environment Self-Check (Task T4.1)
	found, version := platform.CheckWebView2Runtime()
	if *checkEnvFlag {
		if found {
			fmt.Printf("WebView2 Evergreen Runtime verified: %s\n", version)
			os.Exit(0)
		} else {
			fmt.Fprintln(os.Stderr, "WebView2 Evergreen Runtime NOT found!")
			os.Exit(1)
		}
	}

	if !found {
		// Native Windows modal error dialog blocking startup
		_ = platform.ShowWebView2MissingDialog()
		os.Exit(1)
	}

	// Lock OS thread for COM
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 2. Initialize Audio Engine & Controller
	backend := wasapi.NewBackend()
	ctrl := controller.NewAppController(backend)
	if err := ctrl.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize WASAPI audio backend: %v\n", err)
	}
	defer ctrl.Close()

	// Automatically select default device on startup
	if err := ctrl.SelectDevice(""); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to select default audio device on startup: %v\n", err)
	}

	// Start background hardware hotplug watcher (250ms interval)
	ctrl.DeviceService().StartWatcher()
	defer ctrl.DeviceService().StopWatcher()

	// 3. UI RPC Service
	uiService := controller.NewUIService(ctrl)

	// Sub-FS for web assets
	assets, err := fs.Sub(distFS, "dist")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to locate embedded assets: %v\n", err)
		os.Exit(1)
	}

	// 4. Wails v3 Application
	app := application.New(application.Options{
		Name:        "Windows Professional Audio Signal Generator",
		Description: "Professional Audio Test Waveform Generator & Analyzer",
		Services: []application.Service{
			application.NewService(uiService),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
	})

	// 5. Create Dark Industrial Instrument Window
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "Windows 专业音频测试信号发生器 v2.0",
		URL:            "/",
		Width:          1280,
		Height:         880,
		MinWidth:       1024,
		MinHeight:      720,
		DevToolsEnabled: true,
		BackgroundType: application.BackgroundTypeSolid,
		Windows: application.WindowsWindow{
			BackdropType: application.Auto,
		},
	})

	// 6. Run Event Loop
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Application terminated with error: %v\n", err)
		os.Exit(1)
	}
}
