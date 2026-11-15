package engine

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/dsp"
	"sound-signal-generator/internal/format"
	"sound-signal-generator/internal/platform"
)

var (
	ErrEngineNotOpen     = errors.New("engine is not in an open state")
	ErrEngineNotRunning  = errors.New("engine is not currently running")
	ErrEngineAlreadyOpen = errors.New("engine is already open")
)

// EngineConfig holds configuration parameters for initializing AudioEngine.
type EngineConfig struct {
	Device         audio.Device
	ExternalStream audio.AudioStream
	Format         audio.AudioFormat
	ShareMode      audio.ShareMode
	BufferDuration time.Duration
	BufferFrames   int
	RawBufferBytes int
	FadeDurationMs float64
	AllowClipping  bool
}

// AudioEngine coordinates audio streaming, DSP generation, quantization, and safety state machines.
type AudioEngine struct {
	mu           sync.Mutex
	cfg          EngineConfig
	device       audio.Device
	stream       audio.AudioStream
	stateMachine *StateMachine
	fader        *Fader
	diagnostics  *Diagnostics
	bufferPool   *BufferPool
	generator    atomic.Pointer[generatorRef]
	rawProvider  RawFrameProvider
	peakGuard    *format.PeakGuard

	// Control channels
	stopSignal    atomic.Bool
	stopDone      chan struct{}
	faultInjected atomic.Pointer[error]
	invalidated   atomic.Bool

	activeGoroutine atomic.Bool
}

// generatorRef keeps the interface value behind an atomic pointer so the
// control thread can hot-swap a prepared generator without taking a lock on
// the real-time render thread.
type generatorRef struct {
	generator dsp.Generator
}

// RawFrameProvider defines direct zero-copy byte streaming (used for bit-perfect verification and DoP).
type RawFrameProvider interface {
	ProvideFrames(dst []byte, numFrames int) error
	Reset()
}

// NewAudioEngine creates an AudioEngine instance.
func NewAudioEngine() *AudioEngine {
	sm := NewStateMachine()
	diag := NewDiagnostics()
	diag.SetCurrentState(sm.Current())

	return &AudioEngine{
		stateMachine: sm,
		fader:        NewFader(),
		diagnostics:  diag,
		peakGuard:    format.NewPeakGuard(false),
	}
}

// State returns the current lifecycle state.
func (e *AudioEngine) State() State {
	return e.stateMachine.Current()
}

// Diagnostics returns the diagnostics counter snapshot.
func (e *AudioEngine) Diagnostics() DiagnosticsSnapshot {
	s := e.diagnostics.Snapshot()
	s.CurrentState = e.stateMachine.Current()
	return s
}

// Open initializes the device, verifies format, and sets the engine to StateOpenMuted.
func (e *AudioEngine) Open(cfg EngineConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stateMachine.Current() != StateClosed {
		return ErrEngineAlreadyOpen
	}

	if cfg.Device == nil && cfg.ExternalStream == nil {
		return errors.New("device or external stream cannot be nil")
	}

	// 1. Transition Closed -> Probing
	if _, err := e.stateMachine.Transition(EventProbeStart); err != nil {
		return err
	}
	e.diagnostics.SetCurrentState(StateProbing)

	// 2. Open Stream
	streamCfg := audio.StreamConfig{
		Format:         cfg.Format,
		BufferFrames:   cfg.BufferFrames,
		ShareMode:      cfg.ShareMode,
		BufferDuration: cfg.BufferDuration,
		EventDriven:    true,
	}

	stream := cfg.ExternalStream
	if stream == nil {
		var err error
		stream, err = cfg.Device.OpenStream(streamCfg)
		if err != nil {
			e.stateMachine.Transition(EventProbeFailed)
			e.diagnostics.SetCurrentState(StateFaulted)
			return fmt.Errorf("device OpenStream failed: %w", err)
		}
	}

	bufferFrames, err := stream.GetBufferSize()
	if err != nil || bufferFrames == 0 {
		if cfg.ExternalStream != nil {
			_ = stream.Close()
			e.stateMachine.Transition(EventProbeFailed)
			e.diagnostics.SetCurrentState(StateFaulted)
			if err != nil {
				return fmt.Errorf("external stream reported invalid buffer size %d: %w", bufferFrames, err)
			}
			return fmt.Errorf("external stream reported invalid buffer size: %d", bufferFrames)
		}
		bufferFrames = uint32(float64(cfg.Format.SampleRate) * 0.01) // 10ms fallback
	}

	// 3. Pre-allocate BufferPool
	if cfg.RawBufferBytes > 0 {
		e.bufferPool = NewBufferPoolWithRawBytes(int(bufferFrames), cfg.Format.Channels, cfg.RawBufferBytes)
	} else {
		bytesPerSample := cfg.Format.SampleFormat.BytesPerSample()
		e.bufferPool = NewBufferPool(int(bufferFrames), cfg.Format.Channels, bytesPerSample)
	}

	e.cfg = cfg
	e.device = cfg.Device
	e.stream = stream
	e.generator.Store(nil)
	e.peakGuard = format.NewPeakGuard(cfg.AllowClipping)

	// 4. Transition Probing -> OpenMuted
	if _, err := e.stateMachine.Transition(EventProbeSuccess); err != nil {
		stream.Close()
		return err
	}
	e.diagnostics.SetCurrentState(StateOpenMuted)
	e.fader.Mute()

	return nil
}

// Prime pre-fills the initial buffer with the generator output and transitions to StatePrimed.
func (e *AudioEngine) Prime(gen dsp.Generator) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stateMachine.Current() != StateOpenMuted {
		return fmt.Errorf("engine must be in StateOpenMuted to prime, current is %s", e.stateMachine.Current())
	}

	if gen == nil {
		return errors.New("generator cannot be nil")
	}

	// Verify peak
	if err := e.peakGuard.Check(gen.MaxPeak()); err != nil {
		return err
	}

	e.generator.Store(&generatorRef{generator: gen})
	e.rawProvider = nil
	e.bufferPool.Zero()

	// Transition OpenMuted -> Primed
	if _, err := e.stateMachine.Transition(EventPrimeBuffer); err != nil {
		return err
	}
	e.diagnostics.SetCurrentState(StatePrimed)

	return nil
}

// PrimeRaw pre-fills the engine with a direct raw frame provider (DoP or bit-perfect PRBS).
func (e *AudioEngine) PrimeRaw(provider RawFrameProvider) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stateMachine.Current() != StateOpenMuted {
		return fmt.Errorf("engine must be in StateOpenMuted to prime, current is %s", e.stateMachine.Current())
	}

	if provider == nil {
		return errors.New("raw provider cannot be nil")
	}

	e.rawProvider = provider
	e.generator.Store(nil)
	e.bufferPool.Zero()

	// Transition OpenMuted -> Primed
	if _, err := e.stateMachine.Transition(EventPrimeBuffer); err != nil {
		return err
	}
	e.diagnostics.SetCurrentState(StatePrimed)

	return nil
}

// Start begins playback on a dedicated locked real-time thread.
func (e *AudioEngine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stateMachine.Current() != StatePrimed && e.stateMachine.Current() != StateOpenMuted {
		return fmt.Errorf("engine cannot start from state %s", e.stateMachine.Current())
	}

	if e.generator.Load() == nil && e.rawProvider == nil {
		return errors.New("no generator or raw provider configured")
	}

	// If starting directly from OpenMuted without Prime, do Prime first
	if e.stateMachine.Current() == StateOpenMuted {
		if _, err := e.stateMachine.Transition(EventPrimeBuffer); err != nil {
			return err
		}
	}

	// Transition Primed -> Running
	if _, err := e.stateMachine.Transition(EventStartRunning); err != nil {
		return err
	}
	e.diagnostics.SetCurrentState(StateRunning)

	fadeMs := e.cfg.FadeDurationMs
	if fadeMs <= 0 {
		fadeMs = 10.0
	}
	e.fader.StartFadeIn(e.cfg.Format.SampleRate, fadeMs)

	// Pre-roll initial buffer before Start if supported (recommended for event-driven exclusive WASAPI)
	if e.bufferPool != nil && e.stream != nil {
		numFrames := e.bufferPool.NumFrames
		if pBuf, err := e.stream.GetBuffer(numFrames); err == nil && pBuf != 0 {
			dst := unsafe.Slice((*byte)(unsafe.Pointer(pBuf)), len(e.bufferPool.RawBytes))
			for i := range dst {
				dst[i] = 0
			}
			// Raw 1-bit paths need an explicit idle pattern. PCM keeps the
			// legacy zero-filled release path because some exclusive WASAPI
			// endpoints reject the silent flag during pre-roll.
			silentFlags := uint32(0)
			if e.rawProvider != nil {
				silentFlags = audio.StreamFlagSilent
			}
			if err := e.stream.ReleaseBuffer(numFrames, silentFlags); err != nil && e.rawProvider != nil {
				e.fader.Mute()
				e.stateMachine.ForceState(StateFaulted)
				e.diagnostics.SetCurrentState(StateFaulted)
				_ = e.stream.Stop()
				return fmt.Errorf("initial silent buffer release failed: %w", err)
			}
		}
	}

	if err := e.stream.Start(); err != nil {
		e.stateMachine.Transition(EventFaultDetected)
		e.diagnostics.SetCurrentState(StateFaulted)
		return fmt.Errorf("stream.Start failed: %w", err)
	}

	e.stopSignal.Store(false)
	e.stopDone = make(chan struct{})
	e.faultInjected.Store(nil)
	e.invalidated.Store(false)

	go e.renderLoop()

	return nil
}

// ReplaceGenerator atomically installs a prepared PCM generator while the
// stream is running. The active format and audio stream remain untouched, so
// waveform, frequency and sweep changes do not require a stop/start cycle.
func (e *AudioEngine) ReplaceGenerator(gen dsp.Generator) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	state := e.stateMachine.Current()
	if state != StateRunning && state != StatePrimed {
		return fmt.Errorf("engine cannot replace generator from state %s", state)
	}
	if gen == nil {
		return errors.New("generator cannot be nil")
	}
	if e.rawProvider != nil {
		return errors.New("cannot replace a raw frame provider with a PCM generator")
	}
	if err := e.peakGuard.Check(gen.MaxPeak()); err != nil {
		return err
	}

	e.generator.Store(&generatorRef{generator: gen})
	return nil
}

// Stop initiates a smooth fade-out and transitions through StateFadingOut to StateOpenMuted.
func (e *AudioEngine) Stop() error {
	e.mu.Lock()
	current := e.stateMachine.Current()
	if current == StatePrimed {
		_, _ = e.stateMachine.Transition(EventRequestStop)
		e.diagnostics.SetCurrentState(StateOpenMuted)
		e.mu.Unlock()
		return nil
	}
	if current != StateRunning {
		e.mu.Unlock()
		return nil
	}

	// Transition Running -> FadingOut
	if _, err := e.stateMachine.Transition(EventRequestStop); err != nil {
		e.mu.Unlock()
		return err
	}
	e.diagnostics.SetCurrentState(StateFadingOut)

	fadeMs := e.cfg.FadeDurationMs
	if fadeMs <= 0 {
		fadeMs = 10.0
	}
	e.fader.StartFadeOut(e.cfg.Format.SampleRate, fadeMs)
	e.stopSignal.Store(true)
	stopDone := e.stopDone
	e.mu.Unlock()

	// Wait for render loop to finish fade out
	select {
	case <-stopDone:
	case <-time.After(1 * time.Second):
		// Timeout fallback: force stop stream and reset state
		e.mu.Lock()
		if e.stream != nil {
			_ = e.stream.Stop()
		}
		e.stateMachine.ForceState(StateOpenMuted)
		e.diagnostics.SetCurrentState(StateOpenMuted)
		e.mu.Unlock()
	}

	return nil
}

// Close gracefully closes the audio stream and resets to StateClosed.
func (e *AudioEngine) Close() error {
	_ = e.Stop()
	e.waitForRenderLoop()

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stream != nil {
		_ = e.stream.Stop()
		_ = e.stream.Close()
		e.stream = nil
	}
	e.device = nil
	e.generator.Store(nil)
	e.rawProvider = nil

	if _, err := e.stateMachine.Transition(EventCloseDevice); err != nil {
		e.stateMachine.ForceState(StateClosed)
	}
	e.diagnostics.SetCurrentState(StateClosed)
	return nil
}

// waitForRenderLoop ensures the streaming goroutine has fully exited before the
// engine releases or clears the active stream.
func (e *AudioEngine) waitForRenderLoop() {
	e.mu.Lock()
	done := e.stopDone
	active := e.activeGoroutine.Load()
	e.mu.Unlock()
	if !active || done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(time.Second):
	}
}

// InjectFault injects a simulated error into the real-time path for verification.
// The engine must immediately transition to mute within 1 device period.
func (e *AudioEngine) InjectFault(err error) {
	e.faultInjected.Store(&err)
}

// HandleDeviceInvalidated handles USB device detachment within 50ms.
func (e *AudioEngine) HandleDeviceInvalidated() {
	e.invalidated.Store(true)
	e.stateMachine.ForceState(StateFaulted)
	e.diagnostics.SetCurrentState(StateFaulted)
	e.fader.Mute()

	// Stop driver submissions immediately; COM release proceeds asynchronously
	go func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.stream != nil {
			_ = e.stream.Stop()
			_ = e.stream.Close()
			e.stream = nil
		}
	}()
}

// renderLoop is the high-priority real-time audio thread.
func (e *AudioEngine) renderLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	revertPriority, _ := platform.EnableRealtimeAudioPriority()
	defer revertPriority()

	e.activeGoroutine.Store(true)
	defer e.activeGoroutine.Store(false)
	defer close(e.stopDone)

	numFrames := e.bufferPool.NumFrames
	channels := e.bufferPool.Channels
	rawBytes := e.bufferPool.RawBytes
	numChannels := e.cfg.Format.Channels
	allowClipping := e.cfg.AllowClipping
	audioFormat := e.cfg.Format

	for {
		// 1. Check for device invalidation (e.g. USB unplug)
		if e.invalidated.Load() {
			e.fader.Mute()
			return
		}

		// 2. Check for software injected fault (must mute in 1 device period)
		if fErr := e.faultInjected.Load(); fErr != nil {
			e.fader.Mute()
			e.diagnostics.SetLastErrorCode(0xDEADBEEF)
			e.stateMachine.ForceState(StateFaulted)
			e.diagnostics.SetCurrentState(StateFaulted)
			_ = e.stream.Stop()
			return
		}

		startTime := time.Now()
		e.diagnostics.IncrementRenderCallbacks()

		var fadeOutFinished bool
		if e.rawProvider != nil {
			// Direct zero-copy raw frame rendering (DoP / bit-perfect PRBS)
			if err := e.rawProvider.ProvideFrames(rawBytes, numFrames); err != nil {
				e.fader.Mute()
				e.diagnostics.SetLastErrorCode(0xE0000002)
				e.stateMachine.ForceState(StateFaulted)
				e.diagnostics.SetCurrentState(StateFaulted)
				_ = e.stream.Stop()
				return
			}
		} else {
			genRef := e.generator.Load()
			if genRef == nil || genRef.generator == nil {
				e.fader.Mute()
				e.diagnostics.SetLastErrorCode(0xE0000004)
				e.stateMachine.ForceState(StateFaulted)
				e.diagnostics.SetCurrentState(StateFaulted)
				_ = e.stream.Stop()
				return
			}
			gen := genRef.generator

			// 3. Render waveform samples for all channels
			if mg, ok := gen.(dsp.MultiChannelGenerator); ok {
				mg.RenderChannels(channels[:numChannels])
			} else {
				for ch := 0; ch < numChannels; ch++ {
					gen.Render(channels[ch])
				}
			}

			// 4. Apply smooth fader envelope
			fadeOutFinished = e.fader.ApplyChannels(channels)

			// 5. Interleave and quantize into raw bytes (Zero allocations)
			interleaveErr := format.Interleave(rawBytes, channels, audioFormat, allowClipping)
			if interleaveErr != nil {
				// Overload or format error: immediate safe mute
				e.fader.Mute()
				e.diagnostics.SetLastErrorCode(0xE0000001)
				e.stateMachine.ForceState(StateFaulted)
				e.diagnostics.SetCurrentState(StateFaulted)
				_ = e.stream.Stop()
				return
			}
		}

		// 6. Submit buffer to audio driver
		pBuf, err := e.stream.GetBuffer(numFrames)
		if err != nil {
			e.diagnostics.IncrementUnderruns()
			if e.rawProvider != nil {
				// On underrun in DoP mode, immediately reset and return to OpenMuted
				e.rawProvider.Reset()
				e.stateMachine.ForceState(StateOpenMuted)
				e.diagnostics.SetCurrentState(StateOpenMuted)
				_ = e.stream.Stop()
				return
			}
		} else if pBuf != 0 {
			// Copy pre-rendered bytes to driver buffer without allocation
			dstSlice := unsafe.Slice((*byte)(unsafe.Pointer(pBuf)), len(rawBytes))
			copy(dstSlice, rawBytes)
			if err := e.stream.ReleaseBuffer(numFrames, 0); err != nil {
				e.fader.Mute()
				e.diagnostics.SetLastErrorCode(0xE0000003)
				e.stateMachine.ForceState(StateFaulted)
				e.diagnostics.SetCurrentState(StateFaulted)
				_ = e.stream.Stop()
				return
			}
		}

		renderDuration := time.Since(startTime).Nanoseconds()
		e.diagnostics.RecordRenderDuration(renderDuration)

		// 7. Check if stopped or fade-out completed
		if e.rawProvider != nil && e.stopSignal.Load() {
			e.rawProvider.Reset()
			e.stateMachine.Transition(EventFadeOutComplete)
			e.diagnostics.SetCurrentState(StateOpenMuted)
			_ = e.stream.Stop()
			return
		} else if (fadeOutFinished || e.fader.State() == FaderStateMuted) && e.stopSignal.Load() {
			e.stateMachine.Transition(EventFadeOutComplete)
			e.diagnostics.SetCurrentState(StateOpenMuted)
			_ = e.stream.Stop()
			return
		}

		// 8. Event-driven synchronization with driver audio hardware
		// In production WASAPI event callback, wait on stream wait handle for DAC buffer request.
		// If handle is 0 (mock stream in unit tests), fallback to time.Sleep.
		period := time.Duration(float64(numFrames) / float64(audioFormat.SampleRate) * float64(time.Second))
		hWait := e.stream.GetWaitHandle()
		if hWait != 0 {
			timeout := period * 4
			if timeout < 50*time.Millisecond {
				timeout = 50 * time.Millisecond
			}
			signaled, err := platform.WaitForWaitHandle(hWait, timeout)
			if err != nil || !signaled {
				e.diagnostics.IncrementLateCallbacks()
			}
		} else if period > 0 {
			time.Sleep(period)
		}
	}
}
