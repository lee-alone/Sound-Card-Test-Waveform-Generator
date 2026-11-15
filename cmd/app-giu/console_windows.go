//go:build windows

package main

import "golang.org/x/sys/windows"

var procFreeConsole = windows.NewLazySystemDLL("kernel32.dll").NewProc("FreeConsole")

// detachConsoleForGUI removes the console window allocated because this is a
// console-subsystem executable. CLI arguments run before this function and
// therefore keep their console attached.
func detachConsoleForGUI() {
	_, _, _ = procFreeConsole.Call()
}
