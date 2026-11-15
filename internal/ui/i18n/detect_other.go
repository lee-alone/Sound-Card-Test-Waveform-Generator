//go:build !windows

package i18n

func systemLanguageID() uint16 {
	return 0
}

func isChineseLanguageID(uint16) bool {
	return false
}
