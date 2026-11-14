package engine

import (
	"encoding/json"
	"testing"
)

func TestStateTransitions(t *testing.T) {
	sm := NewStateMachine()

	if sm.Current() != StateClosed {
		t.Fatalf("initial state = %v, want Closed", sm.Current())
	}

	// Closed -> Probing
	if s, err := sm.Transition(EventProbeStart); err != nil || s != StateProbing {
		t.Errorf("Closed -> EventProbeStart failed: %v", err)
	}

	// Probing -> OpenMuted
	if s, err := sm.Transition(EventProbeSuccess); err != nil || s != StateOpenMuted {
		t.Errorf("Probing -> EventProbeSuccess failed: %v", err)
	}

	// OpenMuted -> Primed
	if s, err := sm.Transition(EventPrimeBuffer); err != nil || s != StatePrimed {
		t.Errorf("OpenMuted -> EventPrimeBuffer failed: %v", err)
	}

	// Primed -> Running
	if s, err := sm.Transition(EventStartRunning); err != nil || s != StateRunning {
		t.Errorf("Primed -> EventStartRunning failed: %v", err)
	}

	// Running -> FadingOut
	if s, err := sm.Transition(EventRequestStop); err != nil || s != StateFadingOut {
		t.Errorf("Running -> EventRequestStop failed: %v", err)
	}

	// FadingOut -> OpenMuted
	if s, err := sm.Transition(EventFadeOutComplete); err != nil || s != StateOpenMuted {
		t.Errorf("FadingOut -> EventFadeOutComplete failed: %v", err)
	}

	// OpenMuted -> Closed
	if s, err := sm.Transition(EventCloseDevice); err != nil || s != StateClosed {
		t.Errorf("OpenMuted -> EventCloseDevice failed: %v", err)
	}
}

func TestFaultAndRecoveryRules(t *testing.T) {
	// 1. Recovery must NEVER auto-recover to Running! Must transition to OpenMuted
	sm := NewStateMachine()
	sm.ForceState(StateRunning)

	// Running -> Faulted
	if s, err := sm.Transition(EventDeviceInvalidated); err != nil || s != StateFaulted {
		t.Errorf("Running -> EventDeviceInvalidated failed: %v", err)
	}

	// Faulted cannot directly go to Running
	if _, err := sm.Transition(EventStartRunning); err == nil {
		t.Errorf("Faulted -> EventStartRunning should be rejected!")
	}

	// Reset Fault -> Closed
	if s, err := sm.Transition(EventResetFault); err != nil || s != StateClosed {
		t.Errorf("Faulted -> EventResetFault failed: %v", err)
	}

	// 2. Recovering state test
	sm.ForceState(StateOpenMuted)
	if s, err := sm.Transition(EventBeginRecovery); err != nil || s != StateRecovering {
		t.Errorf("OpenMuted -> EventBeginRecovery failed: %v", err)
	}

	// RecoverySuccess MUST go to OpenMuted, never Running!
	if s, err := sm.Transition(EventRecoverySuccess); err != nil || s != StateOpenMuted {
		t.Errorf("Recovering -> EventRecoverySuccess must result in OpenMuted, got %v", s)
	}
}

func TestFormatChangeRuleRequiresFadeOut(t *testing.T) {
	sm := NewStateMachine()
	sm.ForceState(StateRunning)

	// In Running state, probe or reopen directly must be rejected
	if _, err := sm.Transition(EventProbeStart); err == nil {
		t.Errorf("Running -> EventProbeStart should be rejected! Must fade out first.")
	}

	// Fade out first
	_, _ = sm.Transition(EventRequestStop)
	_, _ = sm.Transition(EventFadeOutComplete)

	// Now in OpenMuted, can probe for format change!
	if s, err := sm.Transition(EventProbeStart); err != nil || s != StateProbing {
		t.Errorf("OpenMuted -> EventProbeStart failed: %v", err)
	}
}

func TestStateAndEventStrings(t *testing.T) {
	states := []State{
		StateClosed, StateProbing, StateOpenMuted, StatePrimed,
		StateRunning, StateFadingOut, StateRecovering, StateFaulted, State(999),
	}
	for _, s := range states {
		_ = s.String()
	}

	events := []Event{
		EventProbeStart, EventProbeSuccess, EventProbeFailed,
		EventOpenMutedComplete, EventPrimeBuffer, EventStartRunning,
		EventRequestStop, EventFadeOutComplete, EventCloseDevice,
		EventFaultDetected, EventDeviceInvalidated, EventBeginRecovery,
		EventRecoverySuccess, EventRecoveryFailed, EventResetFault, Event(999),
	}
	for _, e := range events {
		_ = e.String()
	}
}

func TestStateJSON(t *testing.T) {
	s := StateRunning
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if string(b) != `"Running"` {
		t.Fatalf("expected \"Running\", got %s", string(b))
	}

	var parsed State
	if err := json.Unmarshal([]byte(`"Running"`), &parsed); err != nil {
		t.Fatalf("Unmarshal string failed: %v", err)
	}
	if parsed != StateRunning {
		t.Fatalf("expected StateRunning, got %v", parsed)
	}

	if err := json.Unmarshal([]byte(`4`), &parsed); err != nil {
		t.Fatalf("Unmarshal int failed: %v", err)
	}
	if parsed != StateRunning {
		t.Fatalf("expected StateRunning from int, got %v", parsed)
	}
}
