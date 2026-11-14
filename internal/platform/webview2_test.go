package platform

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

type mockSystemEnvironment struct {
	regValues   map[string]string
	files       map[string]bool
	boxTitle    string
	boxMessage  string
	boxTriggered bool
}

func (m *mockSystemEnvironment) GetRegistryString(root registry.Key, path string, valueName string) (string, error) {
	key := path + "\\" + valueName
	if val, ok := m.regValues[key]; ok {
		return val, nil
	}
	return "", errors.New("key not found")
}

func (m *mockSystemEnvironment) FileExists(path string) bool {
	return m.files[path]
}

func (m *mockSystemEnvironment) ShowMessageBox(title, message string) error {
	m.boxTitle = title
	m.boxMessage = message
	m.boxTriggered = true
	return nil
}

func TestWebView2Checker_FoundInRegistry(t *testing.T) {
	mockEnv := &mockSystemEnvironment{
		regValues: map[string]string{
			`SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}\pv`: "157.0.4341.46",
		},
		files: make(map[string]bool),
	}

	checker := NewWebView2Checker(mockEnv)
	found, version := checker.Check()

	if !found {
		t.Fatalf("expected WebView2 to be found, got not found")
	}
	if version != "157.0.4341.46" {
		t.Errorf("expected version 157.0.4341.46, got %s", version)
	}
}

func TestWebView2Checker_NotFound(t *testing.T) {
	mockEnv := &mockSystemEnvironment{
		regValues: make(map[string]string),
		files:     make(map[string]bool),
	}

	checker := NewWebView2Checker(mockEnv)
	found, version := checker.Check()

	if found {
		t.Fatalf("expected WebView2 to be not found, got version %s", version)
	}

	err := checker.ShowMissingDialog()
	if err != nil {
		t.Fatalf("unexpected error showing dialog: %v", err)
	}
	if !mockEnv.boxTriggered {
		t.Errorf("expected message box to be triggered")
	}
	if !strings.Contains(mockEnv.boxMessage, "WebView2") {
		t.Errorf("expected dialog message to mention WebView2, got: %s", mockEnv.boxMessage)
	}
}

func TestWebView2Checker_RealSystem(t *testing.T) {
	// Verifies real machine environment
	found, version := CheckWebView2Runtime()
	t.Logf("Real System WebView2 Detection: Found=%v, Version=%s", found, version)
	if !found {
		t.Logf("Notice: WebView2 runtime not found on this machine, test verified missing branch")
	} else {
		if version == "" {
			t.Errorf("found is true but version string is empty")
		}
	}
}
