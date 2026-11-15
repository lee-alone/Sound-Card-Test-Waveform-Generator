package giuui

import (
	"os"
	"path/filepath"
	"testing"

	"sound-signal-generator/internal/ui/i18n"
)

func TestUIPreferencesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ui.json")
	if err := saveUILocaleTo(path, i18n.SimplifiedChinese); err != nil {
		t.Fatalf("saveUILocaleTo failed: %v", err)
	}
	locale, ok := loadUILocaleFrom(path)
	if !ok {
		t.Fatal("expected saved UI locale to load")
	}
	if locale != i18n.SimplifiedChinese {
		t.Fatalf("locale = %q, want %q", locale, i18n.SimplifiedChinese)
	}
}

func TestUIPreferencesRejectInvalidVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ui.json")
	payload := []byte(`{"version":999,"language":"zh-CN"}`)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, ok := loadUILocaleFrom(path); ok {
		t.Fatal("invalid preferences version must be rejected")
	}
}
