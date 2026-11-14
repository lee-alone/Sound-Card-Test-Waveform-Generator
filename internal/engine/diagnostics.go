package engine

import (
	"sync/atomic"
	"time"
)

// DiagnosticsSnapshot holds an immutable point-in-time copy of performance counters.
type DiagnosticsSnapshot struct {
	RenderCallbackCount uint64        `json:"render_callback_count"`
	UnderrunCount       uint64        `json:"underrun_count"`
	LateCallbackCount   uint64        `json:"late_callback_count"`
	MaxRenderDuration   time.Duration `json:"max_render_duration"`
	LastRenderDuration  time.Duration `json:"last_render_duration"`
	LastErrorCode       uint32        `json:"last_error_code"`
	CurrentState        State         `json:"current_state"`
}

// Diagnostics maintains lock-free atomic counters updated by the real-time audio thread.
type Diagnostics struct {
	renderCallbackCount atomic.Uint64
	underrunCount       atomic.Uint64
	lateCallbackCount   atomic.Uint64
	maxRenderTimeNs     atomic.Int64
	lastRenderTimeNs    atomic.Int64
	lastErrorCode       atomic.Uint32
	currentState        atomic.Int32
}

// NewDiagnostics creates and initializes atomic diagnostic counters.
func NewDiagnostics() *Diagnostics {
	return &Diagnostics{}
}

// IncrementRenderCallbacks atomically increases render count by 1.
func (d *Diagnostics) IncrementRenderCallbacks() {
	d.renderCallbackCount.Add(1)
}

// IncrementUnderruns atomically increases underrun count by 1.
func (d *Diagnostics) IncrementUnderruns() {
	d.underrunCount.Add(1)
}

// IncrementLateCallbacks atomically increases late wakeups count by 1.
func (d *Diagnostics) IncrementLateCallbacks() {
	d.lateCallbackCount.Add(1)
}

// RecordRenderDuration atomically records duration of a render cycle and updates peak render time.
func (d *Diagnostics) RecordRenderDuration(durationNs int64) {
	d.lastRenderTimeNs.Store(durationNs)
	for {
		currentMax := d.maxRenderTimeNs.Load()
		if durationNs <= currentMax {
			break
		}
		if d.maxRenderTimeNs.CompareAndSwap(currentMax, durationNs) {
			break
		}
	}
}

// SetLastErrorCode atomically stores the most recent driver or backend error code.
func (d *Diagnostics) SetLastErrorCode(code uint32) {
	d.lastErrorCode.Store(code)
}

// SetCurrentState atomically stores the current state machine state ID.
func (d *Diagnostics) SetCurrentState(s State) {
	d.currentState.Store(int32(s))
}

// Snapshot returns a thread-safe snapshot of all performance counters.
func (d *Diagnostics) Snapshot() DiagnosticsSnapshot {
	return DiagnosticsSnapshot{
		RenderCallbackCount: d.renderCallbackCount.Load(),
		UnderrunCount:       d.underrunCount.Load(),
		LateCallbackCount:   d.lateCallbackCount.Load(),
		MaxRenderDuration:   time.Duration(d.maxRenderTimeNs.Load()) * time.Nanosecond,
		LastRenderDuration:  time.Duration(d.lastRenderTimeNs.Load()) * time.Nanosecond,
		LastErrorCode:       d.lastErrorCode.Load(),
		CurrentState:        State(d.currentState.Load()),
	}
}

// Reset clears all counters back to zero.
func (d *Diagnostics) Reset() {
	d.renderCallbackCount.Store(0)
	d.underrunCount.Store(0)
	d.lateCallbackCount.Store(0)
	d.maxRenderTimeNs.Store(0)
	d.lastRenderTimeNs.Store(0)
	d.lastErrorCode.Store(0)
}
