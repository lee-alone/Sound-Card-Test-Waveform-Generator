package engine

import (
	"encoding/json"
	"fmt"
	"sync"
)

// State represents the 8 core lifecycle states defined in the architecture whitepaper.
type State int

const (
	StateClosed State = iota
	StateProbing
	StateOpenMuted
	StatePrimed
	StateRunning
	StateFadingOut
	StateRecovering
	StateFaulted
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "Closed"
	case StateProbing:
		return "Probing"
	case StateOpenMuted:
		return "OpenMuted"
	case StatePrimed:
		return "Primed"
	case StateRunning:
		return "Running"
	case StateFadingOut:
		return "FadingOut"
	case StateRecovering:
		return "Recovering"
	case StateFaulted:
		return "Faulted"
	default:
		return "Unknown"
	}
}

// MarshalJSON serializes State to its string name for JSON clients.
func (s State) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON parses State from string or integer.
func (s *State) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		switch str {
		case "Closed":
			*s = StateClosed
		case "Probing":
			*s = StateProbing
		case "OpenMuted":
			*s = StateOpenMuted
		case "Primed":
			*s = StatePrimed
		case "Running":
			*s = StateRunning
		case "FadingOut":
			*s = StateFadingOut
		case "Recovering":
			*s = StateRecovering
		case "Faulted":
			*s = StateFaulted
		default:
			*s = StateClosed
		}
		return nil
	}

	var num int
	if err := json.Unmarshal(b, &num); err == nil {
		*s = State(num)
		return nil
	}

	return fmt.Errorf("invalid State value: %s", string(b))
}

// Event represents lifecycle triggers causing state transitions.
type Event int

const (
	EventProbeStart Event = iota
	EventProbeSuccess
	EventProbeFailed
	EventOpenMutedComplete
	EventPrimeBuffer
	EventStartRunning
	EventRequestStop
	EventFadeOutComplete
	EventCloseDevice
	EventFaultDetected
	EventDeviceInvalidated
	EventBeginRecovery
	EventRecoverySuccess
	EventRecoveryFailed
	EventResetFault
)

func (e Event) String() string {
	switch e {
	case EventProbeStart:
		return "ProbeStart"
	case EventProbeSuccess:
		return "ProbeSuccess"
	case EventProbeFailed:
		return "ProbeFailed"
	case EventOpenMutedComplete:
		return "OpenMutedComplete"
	case EventPrimeBuffer:
		return "PrimeBuffer"
	case EventStartRunning:
		return "StartRunning"
	case EventRequestStop:
		return "RequestStop"
	case EventFadeOutComplete:
		return "FadeOutComplete"
	case EventCloseDevice:
		return "CloseDevice"
	case EventFaultDetected:
		return "FaultDetected"
	case EventDeviceInvalidated:
		return "DeviceInvalidated"
	case EventBeginRecovery:
		return "BeginRecovery"
	case EventRecoverySuccess:
		return "RecoverySuccess"
	case EventRecoveryFailed:
		return "RecoveryFailed"
	case EventResetFault:
		return "ResetFault"
	default:
		return "UnknownEvent"
	}
}

// ErrInvalidTransition is returned when an event is not allowed in current state.
type ErrInvalidTransition struct {
	From  State
	Event Event
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("invalid state transition: state %s cannot accept event %s", e.From, e.Event)
}

// StateMachine manages valid lifecycle transitions thread-safely.
type StateMachine struct {
	mu      sync.RWMutex
	current State
}

// NewStateMachine creates a new state machine initialized in StateClosed.
func NewStateMachine() *StateMachine {
	return &StateMachine{
		current: StateClosed,
	}
}

// Current returns the current state.
func (sm *StateMachine) Current() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// CanTransition checks if the given event can be applied from state `from`.
func CanTransition(from State, event Event) (State, bool) {
	switch from {
	case StateClosed:
		switch event {
		case EventProbeStart:
			return StateProbing, true
		case EventOpenMutedComplete:
			return StateOpenMuted, true
		}

	case StateProbing:
		switch event {
		case EventProbeSuccess:
			return StateOpenMuted, true
		case EventProbeFailed, EventFaultDetected:
			return StateFaulted, true
		case EventCloseDevice:
			return StateClosed, true
		}

	case StateOpenMuted:
		switch event {
		case EventPrimeBuffer:
			return StatePrimed, true
		case EventProbeStart:
			return StateProbing, true
		case EventCloseDevice:
			return StateClosed, true
		case EventFaultDetected:
			return StateFaulted, true
		case EventBeginRecovery:
			return StateRecovering, true
		}

	case StatePrimed:
		switch event {
		case EventStartRunning:
			return StateRunning, true
		case EventRequestStop:
			return StateOpenMuted, true
		case EventCloseDevice:
			return StateClosed, true
		case EventFaultDetected, EventDeviceInvalidated:
			return StateFaulted, true
		case EventBeginRecovery:
			return StateRecovering, true
		}

	case StateRunning:
		switch event {
		case EventRequestStop:
			return StateFadingOut, true
		case EventCloseDevice:
			return StateClosed, true
		case EventFaultDetected, EventDeviceInvalidated:
			return StateFaulted, true
		case EventBeginRecovery:
			return StateRecovering, true
		}

	case StateFadingOut:
		switch event {
		case EventFadeOutComplete:
			return StateOpenMuted, true
		case EventCloseDevice:
			return StateClosed, true
		case EventFaultDetected, EventDeviceInvalidated:
			return StateFaulted, true
		case EventBeginRecovery:
			return StateRecovering, true
		}

	case StateRecovering:
		switch event {
		case EventRecoverySuccess:
			// Disconnection never auto-recovers to Running, always to OpenMuted
			return StateOpenMuted, true
		case EventRecoveryFailed, EventFaultDetected, EventDeviceInvalidated:
			return StateFaulted, true
		case EventCloseDevice:
			return StateClosed, true
		}

	case StateFaulted:
		switch event {
		case EventResetFault, EventCloseDevice:
			return StateClosed, true
		}
	}

	return from, false
}

// Transition attempts to advance the state machine with an event.
func (sm *StateMachine) Transition(event Event) (State, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	next, ok := CanTransition(sm.current, event)
	if !ok {
		return sm.current, &ErrInvalidTransition{From: sm.current, Event: event}
	}
	sm.current = next
	return next, nil
}

// ForceState overrides current state (used only for emergency fault shutdown).
func (sm *StateMachine) ForceState(s State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.current = s
}
