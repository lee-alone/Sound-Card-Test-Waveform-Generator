package i18n

import "testing"

func TestCatalogKeyParity(t *testing.T) {
	english := allKeys(English)
	chinese := allKeys(SimplifiedChinese)
	for key := range english {
		if _, ok := chinese[key]; !ok {
			t.Errorf("Chinese catalog missing key %q", key)
		}
	}
	for key := range chinese {
		if _, ok := english[key]; !ok {
			t.Errorf("English catalog missing key %q", key)
		}
	}
}

func TestCatalogValuesNonEmpty(t *testing.T) {
	for _, locale := range Supported() {
		for key, value := range allKeys(locale) {
			if value == "" && key != "risk.none.title" && key != "risk.none.warning" {
				t.Errorf("%s translation for %q is empty", locale, key)
			}
		}
	}
}

func TestTranslationSwitch(t *testing.T) {
	previous := Current()
	t.Cleanup(func() { SetLocale(previous) })

	SetLocale(English)
	if got := T("transport.start"); got != "Start output" {
		t.Fatalf("English translation = %q", got)
	}
	SetLocale(SimplifiedChinese)
	if got := T("transport.start"); got != "开始输出" {
		t.Fatalf("Chinese translation = %q", got)
	}
}

func TestUnknownLocaleFallsBackToEnglish(t *testing.T) {
	previous := Current()
	t.Cleanup(func() { SetLocale(previous) })
	SetLocale(Locale("fr-FR"))
	if Current() != English {
		t.Fatalf("unknown locale normalized to %q", Current())
	}
}
