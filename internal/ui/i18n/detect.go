package i18n

import (
	"os"
	"strings"
)

// DetectSystemLocale reports the preferred UI locale. Windows uses the OS UI
// language; other platforms use the usual LANG/LC_ALL environment variables.
func DetectSystemLocale() Locale {
	if isChineseLanguageID(systemLanguageID()) {
		return SimplifiedChinese
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if strings.HasPrefix(value, "zh") {
			return SimplifiedChinese
		}
	}
	return English
}

var translations = map[Locale]map[string]string{
	English:           englishTranslations,
	SimplifiedChinese: chineseTranslations,
}
