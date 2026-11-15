package i18n

import (
	"fmt"
	"sync/atomic"
)

// Locale identifies a supported interface language.
type Locale string

const (
	// English is the default language for non-Chinese systems.
	English Locale = "en-US"
	// SimplifiedChinese is used for Chinese systems and manual selection.
	SimplifiedChinese Locale = "zh-CN"
)

var supportedLocales = []Locale{English, SimplifiedChinese}

var languageNames = map[Locale]string{
	English:           "English",
	SimplifiedChinese: "简体中文",
}

var activeLocale atomic.Value

func init() {
	activeLocale.Store(DetectSystemLocale())
}

// Current returns the locale currently used by the UI.
func Current() Locale {
	value := activeLocale.Load()
	if value == nil {
		return English
	}
	return value.(Locale)
}

// SetLocale changes the active locale. Unknown locales fall back to English.
func SetLocale(locale Locale) {
	activeLocale.Store(Normalize(locale))
}

// Normalize maps unknown or empty locales to English.
func Normalize(locale Locale) Locale {
	switch locale {
	case SimplifiedChinese, "zh", "zh-Hans", "zh-Hans-CN":
		return SimplifiedChinese
	case English, "en", "en-GB":
		return English
	default:
		return English
	}
}

// Supported returns the supported locales in selector order.
func Supported() []Locale {
	out := make([]Locale, len(supportedLocales))
	copy(out, supportedLocales)
	return out
}

// Index returns the selector index for a locale.
func Index(locale Locale) int {
	normalized := Normalize(locale)
	for i, supported := range supportedLocales {
		if supported == normalized {
			return i
		}
	}
	return 0
}

// ByIndex returns the locale at a selector index.
func ByIndex(index int) Locale {
	if index < 0 || index >= len(supportedLocales) {
		return English
	}
	return supportedLocales[index]
}

// LanguageName returns the native display name for a locale.
func LanguageName(locale Locale) string {
	if name, ok := languageNames[Normalize(locale)]; ok {
		return name
	}
	return string(locale)
}

// HasKey reports whether a translation exists for the given locale.
func HasKey(locale Locale, key string) bool {
	_, ok := translations[Normalize(locale)][key]
	return ok
}

// T returns a translated string and applies fmt.Sprintf when args are present.
func T(key string, args ...any) string {
	template := translations[Current()][key]
	if template == "" {
		template = translations[English][key]
	}
	if template == "" {
		template = key
	}
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// RiskTitle returns the localized title for a controller safety category.
func RiskTitle(category string) string {
	return T("risk." + category + ".title")
}

// RiskWarning returns the localized warning for a controller safety category.
func RiskWarning(category string) string {
	return T("risk." + category + ".warning")
}

func allKeys(locale Locale) map[string]string {
	return translations[Normalize(locale)]
}
