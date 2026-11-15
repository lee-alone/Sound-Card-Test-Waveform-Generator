package main

import (
	"fmt"
	"os"
	"runtime"

	"sound-signal-generator/internal/audio/asio"
	"sound-signal-generator/internal/audio/wasapi"
	"sound-signal-generator/internal/cli"
	"sound-signal-generator/internal/controller"
	giuui "sound-signal-generator/internal/ui/giu"
)

func main() {
	// No arguments launches the GUI. Any CLI flag runs through the same
	// executable, so one binary supports both graphical and command-line use.
	if len(os.Args) == 1 {
		// Close the automatically allocated console before showing the GUI.
		detachConsoleForGUI()
		runGUI()
		return
	}

	os.Exit(cli.Run(os.Args[1:]))
}

func runGUI() {
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
	if err := uiService.LoadSafetyPreferences(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load safety preferences: %v\n", err)
	}

	// 3. Launch Native Dear ImGui Industrial Console
	app := giuui.NewGiuApp(uiService)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Application terminated with error: %v\n", err)
		os.Exit(1)
	}
}
