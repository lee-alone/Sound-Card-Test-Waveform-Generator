//go:build windows

package i18n

import "golang.org/x/sys/windows"

var procGetUserDefaultUILanguage = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

func systemLanguageID() uint16 {
	result, _, _ := procGetUserDefaultUILanguage.Call()
	return uint16(result)
}

func isChineseLanguageID(languageID uint16) bool {
	return languageID&0x03ff == 0x0004
}
