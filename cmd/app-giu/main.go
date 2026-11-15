package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"sound-signal-generator/internal/audio/asio"
	"sound-signal-generator/internal/audio/wasapi"
	"sound-signal-generator/internal/controller"
	giuui "sound-signal-generator/internal/ui/giu"
)

func main() {
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println("Windows Professional Audio Signal Generator (Dear ImGui Native Console v2.0)")
		return
	}

	// Lock OS thread for COM & GLFW interactions
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 1. Initialize Audio Engine & Controller
	backend := wasapi.NewBackend()
	ctrl := controller.NewAppControllerWithAsioBackend(backend, asio.NewBackend())
	if err := ctrl.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize WASAPI backend: %v\n", err)
	}
	defer ctrl.Close()

	// Select default audio device on startup
	if err := ctrl.SelectDevice(""); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to select default audio device: %v\n", err)
	}

	// Start background hardware hotplug watcher
	ctrl.DeviceService().StartWatcher()
	defer ctrl.DeviceService().StopWatcher()

	// 2. UI Boundary Service
	uiService := controller.NewUIService(ctrl)

	// 3. Launch Native Dear ImGui Industrial Console
	app := giuui.NewGiuApp(uiService)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Application terminated with error: %v\n", err)
		os.Exit(1)
	}
}
