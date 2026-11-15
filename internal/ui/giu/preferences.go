package giuui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sound-signal-generator/internal/ui/i18n"
)

const uiPreferencesVersion = 1

type uiPreferencesFile struct {
	Version  int    `json:"version"`
	Language string `json:"language"`
}

func loadUILocale() i18n.Locale {
	path, err := defaultUIPreferencesPath()
	if err == nil {
		if locale, ok := loadUILocaleFrom(path); ok {
			return locale
		}
	}
	return i18n.DetectSystemLocale()
}

func loadUILocaleFrom(path string) (i18n.Locale, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return i18n.English, false
	}
	var prefs uiPreferencesFile
	if err := json.Unmarshal(data, &prefs); err != nil || prefs.Version != uiPreferencesVersion {
		return i18n.English, false
	}
	locale := i18n.Locale(prefs.Language)
	normalized := i18n.Normalize(locale)
	if string(normalized) != prefs.Language {
		return i18n.English, false
	}
	return normalized, true
}

func saveUILocale(locale i18n.Locale) error {
	path, err := defaultUIPreferencesPath()
	if err != nil {
		return err
	}
	return saveUILocaleTo(path, locale)
}

func saveUILocaleTo(path string, locale i18n.Locale) error {
	data, err := json.MarshalIndent(uiPreferencesFile{
		Version:  uiPreferencesVersion,
		Language: string(i18n.Normalize(locale)),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal UI preferences: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create UI preferences directory: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write UI preferences: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace UI preferences: %w", err)
	}
	return nil
}

func defaultUIPreferencesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "SoundCardTestWaveformGenerator", "ui.json"), nil
}
