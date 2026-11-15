package controller

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio"
	"sound-signal-generator/internal/dsd"
	"sound-signal-generator/internal/dsp"
	"sound-signal-generator/internal/engine"
)

// ControllerStatus reports the aggregated operational status.
type ControllerStatus struct {
	State        engine.State               `json:"state"`
	DeviceName   string                     `json:"device_name"`
	ActiveFormat audio.AudioFormat          `json:"active_format"`
	Frequency    float64                    `json:"frequency"`
	Amplitude    float64                    `json:"amplitude"`
	Diagnostics  engine.DiagnosticsSnapshot `json:"diagnostics"`
}

// AppController serves as the central control coordinator between UI/CLI and audio engine.
type AppController struct {
	mu                 sync.Mutex
	backend            audio.AudioBackend
	outputBackend      *audio.CompositeBackend
	selectedDevice     audio.Device
	selectedDeviceKey  string
	lastPCMRequest     *ExtendedSignalRequest
	nativeDSDActive    bool
	nativeDSDFormat    dsd.DSDFormat
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

// NewAppController creates a controller using the supplied primary backend and
// the production ASIO backend. Tests that exercise ASIO must explicitly inject
// a mock through NewAppControllerWithAsioBackend.
func NewAppController(backend audio.AudioBackend) *AppController {
	return newAppController(backend, asio.NewBackend())
}

// NewAppControllerWithAsioBackend creates a controller with an explicitly
// supplied ASIO backend for integration tests or custom driver factories.
func NewAppControllerWithAsioBackend(backend audio.AudioBackend, asioBackend *asio.Backend) *AppController {
	return newAppController(backend, asioBackend)
}

func newAppController(backend audio.AudioBackend, asioBackend *asio.Backend) *AppController {
	if asioBackend == nil {
		asioBackend = asio.NewBackend()
	}
	asioService := NewAsioService(asioBackend)
	outputBackend := audio.NewCompositeBackend(backend, asioBackend)
	return &AppController{
		backend:            backend,
		outputBackend:      outputBackend,
		engine:             engine.NewAudioEngine(),
		generator:          dsp.NewSineGenerator(),
		dopGuard:           NewDoPSafetyGuard(),
		safetyManager:      NewSafetyManager(),
		generatorFactory:   NewGeneratorFactory(),
		bitPerfect:         NewBitPerfectEvaluator(),
		deviceService:      NewDeviceService(outputBackend),
		diagnosticsService: NewDiagnosticsService(),
		asioService:        asioService,
		isExclusive:        true,
		bufferDuration:     10 * time.Millisecond,
		extendedRequest:    DefaultExtendedSignalRequest(),
	}
}

// Initialize initializes WASAPI and ASIO. A backend failure is reported and is
// never replaced by a mock implementation.
func (c *AppController) Initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.outputBackend.Initialize()
}

// EnumerateDevices lists WASAPI and ASIO render endpoints with canonical IDs.
func (c *AppController) EnumerateDevices() ([]audio.DeviceInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.outputBackend.EnumerateDevices()
}

// SelectDevice sets active audio device by composite ID, native ID or default if empty.
func (c *AppController) SelectDevice(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.selectDeviceLocked(id)
}

func (c *AppController) selectDeviceLocked(id string) error {
	if id != "" && id == c.selectedDeviceKey {
		return nil
	}
	dev, key, err := c.outputBackend.ResolveDevice(id)
	if err != nil {
		return err
	}

	if c.engine.State() != engine.StateClosed {
		_ = c.engine.Close()
	}
	if c.nativeDSDActive || c.nativeDSDFormat != 0 || c.asioService.NativeDSDMode() {
		if err := c.deactivateNativeDSDLocked(); err != nil {
			_ = dev.Close()
			return fmt.Errorf("failed to restore PCM mode before switching devices: %w", err)
		}
	}
	c.asioService.ClearDevice()
	if c.selectedDevice != nil {
		_ = c.selectedDevice.Close()
	}

	if asioDev, ok := dev.(*asio.Device); ok {
		if err := c.asioService.SelectDevice(asioDev); err != nil {
			_ = dev.Close()
			c.selectedDevice = nil
			c.selectedDeviceKey = ""
			return fmt.Errorf("select ASIO device failed: %w", err)
		}
	}

	c.selectedDevice = dev
	c.selectedDeviceKey = key
	c.deviceService.SetActiveDeviceID(key)
	return nil
}

// ProbeSelectedDeviceCapabilities probes the currently selected unified output device.
func (c *AppController) ProbeSelectedDeviceCapabilities(candidates []audio.AudioFormat, mode audio.ShareMode) ([]audio.FormatSupportResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.selectedDevice == nil {
		return nil, ErrDeviceNotSelected
	}
	return c.selectedDevice.ProbeCapabilities(candidates, mode)
}

// ConfigureSignal validates parameters and configures the engine and generator.
func (c *AppController) ConfigureSignal(req SignalRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ValidateSignalRequest(req); err != nil {
		return err
	}

	if c.selectedDevice == nil {
		if err := c.selectDeviceLocked(req.DeviceID); err != nil {
			return ErrDeviceNotSelected
		}
	} else if req.DeviceID != "" && req.DeviceID != c.selectedDeviceKey && req.DeviceID != c.selectedDevice.ID() {
		if err := c.selectDeviceLocked(req.DeviceID); err != nil {
			return err
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

	// Close Native DSD before the legacy PCM configuration path.
	if c.nativeDSDActive || c.nativeDSDFormat != 0 || c.asioService.NativeDSDMode() {
		if c.engine.State() != engine.StateClosed {
			_ = c.engine.Close()
		}
		if err := c.deactivateNativeDSDLocked(); err != nil {
			return fmt.Errorf("failed to restore PCM mode: %w", err)
		}
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
	if _, ok := c.selectedDevice.(*asio.Device); ok {
		engCfg.BufferFrames = c.asioService.BufferSize()
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
		State:      c.engine.State(),
		DeviceName: devName,
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

// SelectAsioDriver selects an ASIO endpoint through the unified device router.
func (c *AppController) SelectAsioDriver(driverID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if driverID == "" {
		return errors.New("ASIO driver ID cannot be empty")
	}
	if !strings.HasPrefix(driverID, string(audio.BackendASIO)+"::") {
		driverID = string(audio.BackendASIO) + "::" + driverID
	}

	// Re-selecting the active driver is also an explicit PCM selection. End a
	// Native DSD session rather than allowing the same-ID fast path to retain it.
	if driverID == c.selectedDeviceKey {
		if c.nativeDSDActive || c.nativeDSDFormat != 0 || c.asioService.NativeDSDMode() {
			if c.engine.State() != engine.StateClosed {
				_ = c.engine.Close()
			}
			if err := c.deactivateNativeDSDLocked(); err != nil {
				return fmt.Errorf("failed to restore PCM mode before selecting ASIO driver: %w", err)
			}
			return nil
		}
		return c.asioService.CancelNativeDSDEntry()
	}
	return c.selectDeviceLocked(driverID)
}

// CancelNativeDSDEntry revokes a pending Native DSD safety confirmation when
// the operator closes the entry flow without activating a stream.
func (c *AppController) CancelNativeDSDEntry() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nativeDSDActive || c.asioService.NativeDSDMode() {
		return errors.New("Native DSD is active; exit Native DSD instead of cancelling entry")
	}
	return c.asioService.CancelNativeDSDEntry()
}

// SetAsioClockSource applies an ASIO hardware clock source while safely
// closing any active stream first.
func (c *AppController) SetAsioClockSource(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nativeDSDActive || c.asioService.NativeDSDMode() {
		return errors.New("disable Native DSD before changing the ASIO clock source")
	}
	if c.engine.State() != engine.StateClosed {
		_ = c.engine.Close()
	}
	return c.asioService.SetClockSource(index)
}

// SetAsioBufferSize stores the next ASIO buffer size while safely closing any
// active stream so the setting cannot appear to apply to the current buffers.
func (c *AppController) SetAsioBufferSize(frames int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nativeDSDActive || c.asioService.NativeDSDMode() {
		return errors.New("disable Native DSD before changing the ASIO buffer size")
	}
	if c.engine.State() != engine.StateClosed {
		_ = c.engine.Close()
	}
	return c.asioService.SetBufferSize(frames)
}

// AsioService returns the ASIO subsystem controller service.
func (c *AppController) AsioService() *AsioService {
	return c.asioService
}

// Close gracefully releases all resources.
func (c *AppController) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error
	if err := c.engine.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := c.asioService.Close(); err != nil {
		errs = append(errs, err)
	}
	c.nativeDSDActive = false
	c.nativeDSDFormat = 0
	if c.selectedDevice != nil {
		if err := c.selectedDevice.Close(); err != nil {
			errs = append(errs, err)
		}
		c.selectedDevice = nil
		c.selectedDeviceKey = ""
	}
	if err := c.outputBackend.Close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
