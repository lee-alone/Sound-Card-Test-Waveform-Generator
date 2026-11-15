package engine

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"sound-signal-generator/internal/audio"
)

type testExternalStream struct {
	buffer     []byte
	size       uint32
	sizeErr    error
	startErr   error
	releaseErr error
	started    atomic.Bool
	closed     atomic.Bool
	releaseCnt atomic.Int64
	silentSeen atomic.Bool
}

func (s *testExternalStream) Start() error {
	if s.startErr != nil {
		return s.startErr
	}
	s.started.Store(true)
	return nil
}
func (s *testExternalStream) Stop() error {
	s.started.Store(false)
	return nil
}
func (s *testExternalStream) GetBuffer(int) (uintptr, error) {
	if len(s.buffer) == 0 {
		return 0, nil
	}
	return uintptr(unsafe.Pointer(&s.buffer[0])), nil
}
func (s *testExternalStream) ReleaseBuffer(_ int, flags uint32) error {
	if s.releaseErr != nil {
		return s.releaseErr
	}
	s.releaseCnt.Add(1)
	if flags&0x01 != 0 {
		s.silentSeen.Store(true)
	}
	return nil
}
func (s *testExternalStream) GetBufferSize() (uint32, error) { return s.size, s.sizeErr }
func (s *testExternalStream) GetDevicePeriod() (time.Duration, time.Duration, error) {
	return time.Microsecond, time.Microsecond, nil
}
func (s *testExternalStream) GetWaitHandle() uintptr { return 0 }
func (s *testExternalStream) Close() error {
	s.closed.Store(true)
	return nil
}

type testRawProvider struct {
	value byte
	err   error
}

func (p *testRawProvider) ProvideFrames(dst []byte, _ int) error {
	if p.err != nil {
		return p.err
	}
	for i := range dst {
		dst[i] = p.value
	}
	return nil
}
func (p *testRawProvider) Reset() {}

func TestAudioEngineAcceptsExternalStream(t *testing.T) {
	stream := &testExternalStream{size: 128, buffer: make([]byte, 32)}
	eng := NewAudioEngine()
	defer eng.Close()

	err := eng.Open(EngineConfig{
		ExternalStream: stream,
		Format: audio.AudioFormat{
			SampleRate:   11289600,
			Channels:     2,
			SampleFormat: audio.SampleFormat24In32,
		},
		BufferDuration: time.Microsecond,
		RawBufferBytes: 32,
		FadeDurationMs: 0,
	})
	if err != nil {
		t.Fatalf("Open external stream failed: %v", err)
	}
	provider := &testRawProvider{value: 0xAB}
	if err := eng.PrimeRaw(provider); err != nil {
		t.Fatalf("PrimeRaw failed: %v", err)
	}
	if err := eng.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) && stream.releaseCnt.Load() < 2 {
		time.Sleep(time.Millisecond)
	}
	if stream.releaseCnt.Load() < 2 {
		t.Fatalf("external stream was not driven, releases=%d", stream.releaseCnt.Load())
	}
	if !stream.started.Load() {
		t.Fatalf("external stream Start was not called")
	}
	if err := eng.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if eng.State() != StateOpenMuted {
		t.Fatalf("expected OpenMuted after stop, got %s", eng.State())
	}
}

func TestExternalStreamInvalidSizeFaultsWithoutFallback(t *testing.T) {
	stream := &testExternalStream{sizeErr: errors.New("no callback event")}
	eng := NewAudioEngine()
	err := eng.Open(EngineConfig{
		ExternalStream: stream,
		Format: audio.AudioFormat{
			SampleRate:   48000,
			Channels:     2,
			SampleFormat: audio.SampleFormat16Int,
		},
		RawBufferBytes: 4,
	})
	if err == nil {
		t.Fatalf("expected invalid external stream error")
	}
	if eng.State() != StateFaulted {
		t.Fatalf("expected Faulted muted state, got %s", eng.State())
	}
	if !stream.closed.Load() {
		t.Fatalf("invalid external stream must be closed")
	}
}

func TestRawProviderFailureFaultsAndStopsStream(t *testing.T) {
	stream := &testExternalStream{size: 64, buffer: make([]byte, 16)}
	eng := NewAudioEngine()
	defer eng.Close()
	_ = eng.Open(EngineConfig{
		ExternalStream: stream,
		Format: audio.AudioFormat{
			SampleRate:   48000,
			Channels:     2,
			SampleFormat: audio.SampleFormat16Int,
		},
		RawBufferBytes: 16,
	})
	_ = eng.PrimeRaw(&testRawProvider{err: errors.New("source failed")})
	_ = eng.Start()
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) && eng.State() != StateFaulted {
		time.Sleep(time.Millisecond)
	}
	if eng.State() != StateFaulted {
		t.Fatalf("expected Faulted state, got %s", eng.State())
	}
	if stream.started.Load() {
		t.Fatalf("stream must be stopped on source failure")
	}
}

func TestExternalStreamSilentPreRollUsesSilentFlag(t *testing.T) {
	stream := &testExternalStream{size: 64, buffer: make([]byte, 16)}
	eng := NewAudioEngine()
	defer eng.Close()
	_ = eng.Open(EngineConfig{
		ExternalStream: stream,
		Format:         audio.AudioFormat{SampleRate: 48000, Channels: 2, SampleFormat: audio.SampleFormat16Int},
		RawBufferBytes: 16,
	})
	_ = eng.PrimeRaw(&testRawProvider{value: 0xAA})
	_ = eng.Start()
	time.Sleep(2 * time.Millisecond)
	if stream.releaseCnt.Load() == 0 {
		t.Fatalf("expected pre-roll/render release")
	}
	if !stream.silentSeen.Load() {
		t.Fatalf("external stream pre-roll must request deterministic silent flags")
	}
}
