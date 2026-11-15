package giuui

import (
	"testing"

	"sound-signal-generator/internal/ui/i18n"
)

func TestLocalizedOptionKeysExist(t *testing.T) {
	for _, locale := range i18n.Supported() {
		for _, option := range WaveformOptions {
			if !i18n.HasKey(locale, option.LabelKey) {
				t.Errorf("%s missing waveform key %q", locale, option.LabelKey)
			}
		}
		for _, option := range DoPFormatOptions {
			if !i18n.HasKey(locale, option.LabelKey) {
				t.Errorf("%s missing DoP format key %q", locale, option.LabelKey)
			}
		}
		for _, key := range DoPBitOrderLabelKeys {
			if !i18n.HasKey(locale, key) {
				t.Errorf("%s missing DoP bit-order key %q", locale, key)
			}
		}
		for _, key := range ChannelModeLabelKeys {
			if !i18n.HasKey(locale, key) {
				t.Errorf("%s missing channel-mode key %q", locale, key)
			}
		}
		for _, preset := range StandardBenchPresets {
			if !i18n.HasKey(locale, preset.NameKey) {
				t.Errorf("%s missing preset name key %q", locale, preset.NameKey)
			}
			if !i18n.HasKey(locale, preset.DescriptionKey) {
				t.Errorf("%s missing preset description key %q", locale, preset.DescriptionKey)
			}
		}
	}
}

func TestLanguageSelectorIndexRoundTrip(t *testing.T) {
	for index := 0; index < len(i18n.Supported()); index++ {
		locale := i18n.ByIndex(index)
		if got := i18n.Index(locale); got != index {
			t.Errorf("Index(ByIndex(%d)) = %d", index, got)
		}
	}
}
