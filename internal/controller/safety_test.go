package controller

import (
	"path/filepath"
	"testing"
)

func TestSafetyManager_SuppressionPersistsAcrossManagers(t *testing.T) {
	req := DefaultExtendedSignalRequest()
	req.Waveform = WaveformPolyBlepSquare

	sm := NewSafetyManager()
	assessment := sm.Assess(req)
	if !assessment.RequiresConfirmation || assessment.IsConfirmed {
		t.Fatalf("square wave should initially require confirmation: %+v", assessment)
	}

	path := filepath.Join(t.TempDir(), "safety.json")
	if err := sm.suppressToFile(RiskSquareWave, path); err != nil {
		t.Fatalf("suppress safety prompt: %v", err)
	}
	if !sm.IsSuppressed(RiskSquareWave) {
		t.Fatal("suppression should be active in the current manager")
	}
	if err := sm.ValidateConfirmation(req); err != nil {
		t.Fatalf("suppressed risk should pass validation: %v", err)
	}

	reloaded := NewSafetyManager()
	if err := reloaded.loadSafetyPreferences(path); err != nil {
		t.Fatalf("reload safety preferences: %v", err)
	}
	reloadedAssessment := reloaded.Assess(req)
	if !reloadedAssessment.IsConfirmed {
		t.Fatal("persisted suppression should suppress prompts after reload")
	}
}

func TestSafetyManager_OneTimeConfirmationDoesNotPersist(t *testing.T) {
	req := DefaultExtendedSignalRequest()
	req.Waveform = WaveformMultitoneSMPTE

	sm := NewSafetyManager()
	sm.Confirm(RiskMultitoneHighCrest, true)
	if err := sm.ValidateConfirmation(req); err != nil {
		t.Fatalf("session confirmation should satisfy validation: %v", err)
	}

	reloaded := NewSafetyManager()
	assessment := reloaded.Assess(req)
	if assessment.IsConfirmed {
		t.Fatal("one-time confirmation must not become a permanent suppression")
	}
}
