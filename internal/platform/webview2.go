package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Known WebView2 Client GUIDs in EdgeUpdate
var webView2ClientGUIDs = []string{
	"{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}",
	"{F3017226-234A-4265-9617-56F608CEB4E1}",
}

// RegistryHive represents a root registry hive.
type RegistryHive struct {
	Key  registry.Key
	Path string
}

// SystemEnvironment abstracts registry and file system checks for unit testability.
type SystemEnvironment interface {
	GetRegistryString(root registry.Key, path string, valueName string) (string, error)
	FileExists(path string) bool
	ShowMessageBox(title, message string) error
}

// DefaultSystemEnvironment implements SystemEnvironment using real Windows APIs.
type DefaultSystemEnvironment struct{}

func (e *DefaultSystemEnvironment) GetRegistryString(root registry.Key, path string, valueName string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("registry only available on windows")
	}
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()

	val, _, err := k.GetStringValue(valueName)
	return val, err
}

func (e *DefaultSystemEnvironment) FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func (e *DefaultSystemEnvironment) ShowMessageBox(title, message string) error {
	if runtime.GOOS != "windows" {
		fmt.Fprintf(os.Stderr, "FATAL ERROR: %s\n%s\n", title, message)
		return nil
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW := user32.NewProc("MessageBoxW")

	pTitle, _ := syscall.UTF16PtrFromString(title)
	pMsg, _ := syscall.UTF16PtrFromString(message)

	const (
		mbOK        = 0x00000000
		mbIconError = 0x00000010
		mbSystemModal = 0x00001000
	)

	_, _, err := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(pMsg)),
		uintptr(unsafe.Pointer(pTitle)),
		uintptr(mbOK|mbIconError|mbSystemModal),
	)
	if err != nil && err != windows.ERROR_SUCCESS {
		return err
	}
	return nil
}

// WebView2Checker checks for Microsoft Edge WebView2 Evergreen Runtime.
type WebView2Checker struct {
	env SystemEnvironment
}

// NewWebView2Checker creates a checker with the specified system environment.
func NewWebView2Checker(env SystemEnvironment) *WebView2Checker {
	if env == nil {
		env = &DefaultSystemEnvironment{}
	}
	return &WebView2Checker{env: env}
}

// Check inspects registry keys and standard disk paths for WebView2 Evergreen Runtime.
func (c *WebView2Checker) Check() (bool, string) {
	// 1. Check registry hives
	hives := []struct {
		Root registry.Key
		Base string
	}{
		{Root: registry.LOCAL_MACHINE, Base: `SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients`},
		{Root: registry.LOCAL_MACHINE, Base: `SOFTWARE\Microsoft\EdgeUpdate\Clients`},
		{Root: registry.CURRENT_USER, Base: `Software\Microsoft\EdgeUpdate\Clients`},
	}

	for _, h := range hives {
		for _, guid := range webView2ClientGUIDs {
			regPath := h.Base + `\` + guid
			ver, err := c.env.GetRegistryString(h.Root, regPath, "pv")
			if err == nil && ver != "" && ver != "0.0.0.0" {
				return true, ver
			}
		}
	}

	// 2. Check standard directory installations if registry query was restricted
	standardBases := []string{
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("ProgramFiles"),
		os.Getenv("LocalAppData"),
	}

	for _, base := range standardBases {
		if base == "" {
			continue
		}
		appDir := filepath.Join(base, "Microsoft", "EdgeWebView", "Application")
		entries, err := os.ReadDir(appDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() && strings.Count(entry.Name(), ".") >= 2 {
					exePath := filepath.Join(appDir, entry.Name(), "msedgewebview2.exe")
					if c.env.FileExists(exePath) {
						return true, entry.Name()
					}
				}
			}
		}
	}

	return false, ""
}

// ShowMissingDialog alerts the user via a native modal dialog when WebView2 is not detected.
func (c *WebView2Checker) ShowMissingDialog() error {
	title := "缺失必要组件 (Missing Required Component)"
	msg := "系统未检测到 Microsoft Edge WebView2 Evergreen Runtime。\n\n" +
		"Windows 专业音频测试信号发生器需要 WebView2 运行时来渲染高精度仪器界面。\n" +
		"请前往微软官方网站下载并安装 Evergreen WebView2 运行时后再启动程序：\n" +
		"https://developer.microsoft.com/en-us/microsoft-edge/webview2/\n\n" +
		"程序即将退出。"

	return c.env.ShowMessageBox(title, msg)
}

// CheckWebView2Runtime provides a convenient default runtime check.
func CheckWebView2Runtime() (bool, string) {
	checker := NewWebView2Checker(nil)
	return checker.Check()
}

// ShowWebView2MissingDialog shows the default missing dialog.
func ShowWebView2MissingDialog() error {
	checker := NewWebView2Checker(nil)
	return checker.ShowMissingDialog()
}
