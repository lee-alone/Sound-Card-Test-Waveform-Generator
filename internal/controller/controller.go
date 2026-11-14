package controller

import (
	"fmt"
	"sync"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/dsp"
	"sound-signal-generator/internal/engine"
)

// ControllerStatus reports the aggregated operational status.
type ControllerStatus struct {
	State        engine.State              `json:"state"`
	DeviceName   string                    `json:"device_name"`
	ActiveFormat audio.AudioFormat         `json:"active_format"`
	Frequency    float64                   `json:"frequency"`
	Amplitude    float64                   `json:"amplitude"`
	Diagnostics  engine.DiagnosticsSnapshot `json:"diagnostics"`
}

// AppController serves as the central control coordinator between UI/CLI and audio engine.
type AppController struct {
	mu                 sync.Mutex
	backend            audio.AudioBackend
	selectedDevice     audio.Device
	currentRequest     SignalRequest
	extendedRequest    ExtendedSignalRequest
	engine             *engine.AudioEngine
	generator          *dsp.SineGenerator
	activeGenerator    dsp.Generator
	dopGuard           *DoPSafetyGuard
	safetyManager      *SafetyManager
	generatorFactory   *GeneratorFactory
	bitPerfect         *BitPerfectEvaluator
	deviceService      *DeviceService
	diagnosticsService *DiagnosticsService
	asioService        *AsioService
	isExclusive        bool
	bufferDuration     time.Duration
}

// NewAppController creates a controller instance with the given backend.
func NewAppController(backend audio.AudioBackend) *AppController {
	c := &AppController{
		backend:            backend,
		engine:             engine.NewAudioEngine(),
		generator:          dsp.NewSineGenerator(),
		dopGuard:           NewDoPSafetyGuard(),
		safetyManager:      NewSafetyManager(),
		generatorFactory:   NewGeneratorFactory(),
		bitPerfect:         NewBitPerfectEvaluator(),
		deviceService:      NewDeviceService(backend),
		diagnosticsService: NewDiagnosticsService(),
		asioService:        NewAsioService(nil),
		isExclusive:        true,
		bufferDuration:     10 * time.Millisecond,
		extendedRequest:    DefaultExtendedSignalRequest(),
	}
	return c
}

// Initialize initializes the backend.
func (c *AppController) Initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.backend.Initialize()
}

// EnumerateDevices lists all available render endpoints.
func (c *AppController) EnumerateDevices() ([]audio.DeviceInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.backend.EnumerateDevices()
}

// SelectDevice sets active audio device by ID or default if empty.
func (c *AppController) SelectDevice(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var dev audio.Device
	var err error
	if id == "" {
		dev, err = c.backend.GetDefaultDevice()
	} else {
		dev, err = c.backend.GetDevice(id)
	}
	if err != nil {
		return err
	}

	if c.selectedDevice != nil {
		_ = c.selectedDevice.Close()
	}
	c.selectedDevice = dev
	return nil
}

// ConfigureSignal validates parameters and configures the engine and generator.
func (c *AppController) ConfigureSignal(req SignalRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ValidateSignalRequest(req); err != nil {
		return err
	}

	if c.selectedDevice == nil {
		var dev audio.Device
		var err error
		if req.DeviceID != "" {
			dev, err = c.backend.GetDevice(req.DeviceID)
		}
		if dev == nil {
			dev, err = c.backend.GetDefaultDevice()
		}
		if err != nil || dev == nil {
			return ErrDeviceNotSelected
		}
		c.selectedDevice = dev
	} else if req.DeviceID != "" && c.selectedDevice.ID() != req.DeviceID {
		dev, err := c.backend.GetDevice(req.DeviceID)
		if err == nil && dev != nil {
			_ = c.selectedDevice.Close()
			c.selectedDevice = dev
		}
	}

	// Prepare generator
	genCfg := dsp.Config{
		SampleRate:   req.SampleRate,
		Frequency:    req.Frequency,
		Amplitude:    req.Amplitude,
		DCOffset:     req.DCOffset,
		InitialPhase: 0.0,
	}
	if err := c.generator.Prepare(genCfg); err != nil {
		return fmt.Errorf("generator prepare failed: %w", err)
	}

	// Close engine if already open
	if c.engine.State() != engine.StateClosed {
		_ = c.engine.Close()
	}

	targetFormat := audio.AudioFormat{
		SampleRate:   req.SampleRate,
		Channels:     req.Channels,
		SampleFormat: req.SampleFormat,
	}

	engCfg := engine.EngineConfig{
		Device:         c.selectedDevice,
		Format:         targetFormat,
		ShareMode:      audio.ShareModeExclusive,
		BufferDuration: 10 * time.Millisecond,
		FadeDurationMs: 10.0,
	}

	if err := c.engine.Open(engCfg); err != nil {
		return fmt.Errorf("engine open failed: %w", err)
	}

	if err := c.engine.Prime(c.generator); err != nil {
		return fmt.Errorf("engine prime failed: %w", err)
	}

	c.currentRequest = req
	return nil
}

// StartPlayback starts the audio stream.
func (c *AppController) StartPlayback() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.engine.Start()
}

// StopPlayback stops playback with a smooth fade-out.
func (c *AppController) StopPlayback() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.engine.Stop()
}

// Status returns current controller status and diagnostic snapshot.
func (c *AppController) Status() ControllerStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	devName := "None"
	if c.selectedDevice != nil {
		devName = c.selectedDevice.Name()
	}

	return ControllerStatus{
		State:        c.engine.State(),
		DeviceName:   devName,
		ActiveFormat: audio.AudioFormat{
			SampleRate:   c.currentRequest.SampleRate,
			Channels:     c.currentRequest.Channels,
			SampleFormat: c.currentRequest.SampleFormat,
		},
		Frequency:   c.currentRequest.Frequency,
		Amplitude:   c.currentRequest.Amplitude,
		Diagnostics: c.engine.Diagnostics(),
	}
}

// AsioService returns the ASIO subsystem controller service.
func (c *AppController) AsioService() *AsioService {
	return c.asioService
}

// Close gracefully releases all resources.
func (c *AppController) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_ = c.engine.Close()
	if c.selectedDevice != nil {
		_ = c.selectedDevice.Close()
		c.selectedDevice = nil
	}
	return c.backend.Close()
}
