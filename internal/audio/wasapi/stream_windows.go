//go:build windows

package wasapi

import (
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"sound-signal-generator/internal/audio"
)

// ConvertAudioFormatToWaveFormat creates a WAVEFORMATEXTENSIBLE from an AudioFormat.
func ConvertAudioFormatToWaveFormat(f audio.AudioFormat) WAVEFORMATEXTENSIBLE {
	channels := uint16(f.Channels)
	sampleRate := uint32(f.SampleRate)
	containerBits := uint16(f.SampleFormat.ContainerBits())
	validBits := uint16(f.SampleFormat.ValidBits())
	bytesPerSample := uint16(f.SampleFormat.BytesPerSample())
	blockAlign := channels * bytesPerSample
	avgBytesPerSec := sampleRate * uint32(blockAlign)

	subFormat := KSDATAFORMAT_SUBTYPE_PCM
	if f.SampleFormat == audio.SampleFormat32Float {
		subFormat = KSDATAFORMAT_SUBTYPE_IEEE_FLOAT
	}

	channelMask := f.ChannelMask
	if channelMask == 0 {
		if channels == 1 {
			channelMask = 0x00000004 // SPEAKER_FRONT_CENTER
		} else if channels == 2 {
			channelMask = audio.StandardSpeakerChannelMaskStereo
		}
	}

	return WAVEFORMATEXTENSIBLE{
		WFormatTag:         WAVE_FORMAT_EXTENSIBLE,
		NChannels:          channels,
		NSamplesPerSec:     sampleRate,
		NAvgBytesPerSec:    avgBytesPerSec,
		NBlockAlign:        blockAlign,
		WBitsPerSample:     containerBits,
		CBSize:             22,
		ValidBitsPerSample: validBits,
		DwChannelMask:      channelMask,
		SubFormat:          subFormat,
	}
}

// WASAPIDevice wraps an IMMDevice pointer and implements audio.Device.
type WASAPIDevice struct {
	pDevice uintptr
	info    audio.DeviceInfo
	mu      sync.Mutex
}

func newWASAPIDevice(pDevice uintptr) (*WASAPIDevice, error) {
	info, err := getDeviceInfo(pDevice)
	if err != nil {
		return nil, err
	}
	return &WASAPIDevice{
		pDevice: pDevice,
		info:    info,
	}, nil
}

func (d *WASAPIDevice) ID() string               { return d.info.ID }
func (d *WASAPIDevice) Name() string             { return d.info.Name }
func (d *WASAPIDevice) Flow() audio.DeviceFlow   { return d.info.Flow }
func (d *WASAPIDevice) State() audio.DeviceState { return d.info.State }

func (d *WASAPIDevice) activateAudioClient() (uintptr, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pDevice == 0 {
		return 0, fmt.Errorf("device is closed")
	}

	var pAudioClient uintptr
	hr := callCOM(
		d.pDevice,
		3, // IMMDevice::Activate
		uintptr(unsafe.Pointer(&IID_IAudioClient)),
		uintptr(CLSCTX_ALL),
		0,
		uintptr(unsafe.Pointer(&pAudioClient)),
	)
	if FAILED(hr) || pAudioClient == 0 {
		return 0, fmt.Errorf("IMMDevice::Activate(IAudioClient) failed with HRESULT 0x%08X", uint32(hr))
	}
	return pAudioClient, nil
}

// ProbeCapabilities evaluates candidate audio formats for exclusive or shared support.
func (d *WASAPIDevice) ProbeCapabilities(candidates []audio.AudioFormat, shareMode audio.ShareMode) ([]audio.FormatSupportResult, error) {
	client, err := d.activateAudioClient()
	if err != nil {
		return nil, err
	}
	defer releaseCOM(client)

	wasapiShareMode := AUDCLNT_SHAREMODE_EXCLUSIVE
	if shareMode == audio.ShareModeShared {
		wasapiShareMode = AUDCLNT_SHAREMODE_SHARED
	}

	results := make([]audio.FormatSupportResult, len(candidates))
	for i, candidate := range candidates {
		wf := ConvertAudioFormatToWaveFormat(candidate)
		var pClosestMatch uintptr

		// In exclusive mode, pClosestMatch must be NULL
		var pClosestArg uintptr
		if wasapiShareMode == AUDCLNT_SHAREMODE_SHARED {
			pClosestArg = uintptr(unsafe.Pointer(&pClosestMatch))
		}

		hr := callCOM(
			client,
			7, // IAudioClient::IsFormatSupported
			uintptr(wasapiShareMode),
			uintptr(unsafe.Pointer(&wf)),
			pClosestArg,
		)
		if pClosestMatch != 0 {
			procCoTaskMemFree.Call(pClosestMatch)
		}

		res := audio.FormatSupportResult{
			Format:  candidate,
			HResult: hr,
		}

		switch hr {
		case S_OK:
			res.Status = audio.StatusSupported
		case AUDCLNT_E_UNSUPPORTED_FORMAT:
			res.Status = audio.StatusUnsupported
			res.ErrorMessage = "Format not supported by hardware"
		case AUDCLNT_E_EXCLUSIVE_MODE_NOT_ALLOWED:
			res.Status = audio.StatusUnsupported
			res.ErrorMessage = "Exclusive mode not allowed for this format or device"
		case AUDCLNT_E_DEVICE_IN_USE:
			res.Status = audio.StatusDeviceBusy
			res.ErrorMessage = "Audio device currently in use exclusively by another application"
		default:
			if FAILED(hr) {
				res.Status = audio.StatusDriverError
				res.ErrorMessage = fmt.Sprintf("Driver error 0x%08X", uint32(hr))
			} else {
				// S_FALSE in shared mode means supported with closest match
				res.Status = audio.StatusSupported
			}
		}
		results[i] = res
	}

	return results, nil
}

// OpenStream opens an event-driven WASAPI stream.
func (d *WASAPIDevice) OpenStream(config audio.StreamConfig) (audio.AudioStream, error) {
	client, err := d.activateAudioClient()
	if err != nil {
		return nil, err
	}

	wasapiShareMode := AUDCLNT_SHAREMODE_EXCLUSIVE
	if config.ShareMode == audio.ShareModeShared {
		wasapiShareMode = AUDCLNT_SHAREMODE_SHARED
	}

	bufferDuration := config.BufferDuration
	if bufferDuration <= 0 {
		bufferDuration = 10 * time.Millisecond
	}
	// 100ns units (REFERENCE_TIME)
	hnsBufferDuration := int64(bufferDuration / (100 * time.Nanosecond))
	hnsPeriodicity := hnsBufferDuration
	if wasapiShareMode == AUDCLNT_SHAREMODE_SHARED {
		hnsPeriodicity = 0
	}

	streamFlags := streamFlagsForShareMode(config.ShareMode)
	wf := ConvertAudioFormatToWaveFormat(config.Format)

	hr := callCOM(
		client,
		3, // IAudioClient::Initialize
		uintptr(wasapiShareMode),
		uintptr(streamFlags),
		uintptr(hnsBufferDuration),
		uintptr(hnsPeriodicity),
		uintptr(unsafe.Pointer(&wf)),
		0,
	)

	// Handle AUDCLNT_E_BUFFER_SIZE_NOT_ALIGNED in exclusive mode
	if hr == AUDCLNT_E_BUFFER_SIZE_NOT_ALIGNED {
		var alignedFrames uint32
		if SUCCEEDED(callCOM(client, 4, uintptr(unsafe.Pointer(&alignedFrames)))) && alignedFrames > 0 {
			releaseCOM(client)
			client, err = d.activateAudioClient()
			if err != nil {
				return nil, err
			}
			hnsBufferDuration = int64(float64(alignedFrames) * 10000000.0 / float64(config.Format.SampleRate))
			hnsPeriodicity = hnsBufferDuration
			hr = callCOM(
				client,
				3,
				uintptr(wasapiShareMode),
				uintptr(streamFlags),
				uintptr(hnsBufferDuration),
				uintptr(hnsPeriodicity),
				uintptr(unsafe.Pointer(&wf)),
				0,
			)
		}
	}

	if FAILED(hr) {
		releaseCOM(client)
		return nil, fmt.Errorf("IAudioClient::Initialize failed with HRESULT 0x%08X", uint32(hr))
	}

	// Create Auto-Reset Event
	eventHandle, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		releaseCOM(client)
		return nil, fmt.Errorf("CreateEvent failed: %w", err)
	}

	hr = callCOM(client, 13, uintptr(eventHandle)) // IAudioClient::SetEventHandle
	if FAILED(hr) {
		windows.CloseHandle(eventHandle)
		releaseCOM(client)
		return nil, fmt.Errorf("IAudioClient::SetEventHandle failed with HRESULT 0x%08X", uint32(hr))
	}

	var pRenderClient uintptr
	hr = callCOM(client, 14, uintptr(unsafe.Pointer(&IID_IAudioRenderClient)), uintptr(unsafe.Pointer(&pRenderClient))) // GetService
	if FAILED(hr) || pRenderClient == 0 {
		windows.CloseHandle(eventHandle)
		releaseCOM(client)
		return nil, fmt.Errorf("IAudioClient::GetService(IAudioRenderClient) failed with HRESULT 0x%08X", uint32(hr))
	}

	return &WASAPIStream{
		client:       client,
		renderClient: pRenderClient,
		eventHandle:  eventHandle,
		format:       config.Format,
		shareMode:    config.ShareMode,
	}, nil
}

func streamFlagsForShareMode(shareMode audio.ShareMode) uint32 {
	flags := uint32(AUDCLNT_STREAMFLAGS_EVENTCALLBACK)
	if shareMode == audio.ShareModeShared {
		// Shared mode must accept the application format even when it differs
		// from the endpoint mix format. Windows performs the conversion and
		// resampling in the audio engine.
		flags |= AUDCLNT_STREAMFLAGS_NOPERSIST |
			AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM |
			AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY
	}
	return flags
}

// Close releases the IMMDevice reference.
func (d *WASAPIDevice) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pDevice != 0 {
		releaseCOM(d.pDevice)
		d.pDevice = 0
	}
	return nil
}

// WASAPIStream wraps an active IAudioClient and IAudioRenderClient session.
type WASAPIStream struct {
	client       uintptr
	renderClient uintptr
	eventHandle  windows.Handle
	format       audio.AudioFormat
	shareMode    audio.ShareMode
	closed       bool
	mu           sync.Mutex
}

func (s *WASAPIStream) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("stream is closed")
	}
	hr := callCOM(s.client, 10) // IAudioClient::Start
	if FAILED(hr) {
		return fmt.Errorf("IAudioClient::Start failed with HRESULT 0x%08X", uint32(hr))
	}
	return nil
}

func (s *WASAPIStream) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	hr := callCOM(s.client, 11) // IAudioClient::Stop
	if FAILED(hr) {
		return fmt.Errorf("IAudioClient::Stop failed with HRESULT 0x%08X", uint32(hr))
	}
	return nil
}

func (s *WASAPIStream) GetBuffer(numFrames int) (uintptr, error) {
	var pData uintptr
	hr := callCOM(s.renderClient, 3, uintptr(numFrames), uintptr(unsafe.Pointer(&pData)))
	if FAILED(hr) {
		return 0, fmt.Errorf("IAudioRenderClient::GetBuffer failed with HRESULT 0x%08X", uint32(hr))
	}
	return pData, nil
}

func (s *WASAPIStream) ReleaseBuffer(numFramesWritten int, flags uint32) error {
	hr := callCOM(s.renderClient, 4, uintptr(numFramesWritten), uintptr(flags))
	if FAILED(hr) {
		return fmt.Errorf("IAudioRenderClient::ReleaseBuffer failed with HRESULT 0x%08X", uint32(hr))
	}
	return nil
}

func (s *WASAPIStream) GetBufferSize() (uint32, error) {
	var numFrames uint32
	hr := callCOM(s.client, 4, uintptr(unsafe.Pointer(&numFrames)))
	if FAILED(hr) {
		return 0, fmt.Errorf("IAudioClient::GetBufferSize failed with HRESULT 0x%08X", uint32(hr))
	}
	return numFrames, nil
}

func (s *WASAPIStream) GetDevicePeriod() (time.Duration, time.Duration, error) {
	var defPeriod, minPeriod int64
	hr := callCOM(s.client, 9, uintptr(unsafe.Pointer(&defPeriod)), uintptr(unsafe.Pointer(&minPeriod)))
	if FAILED(hr) {
		return 0, 0, fmt.Errorf("IAudioClient::GetDevicePeriod failed with HRESULT 0x%08X", uint32(hr))
	}
	// Convert 100ns units to time.Duration
	return time.Duration(defPeriod * 100), time.Duration(minPeriod * 100), nil
}

func (s *WASAPIStream) GetWaitHandle() uintptr {
	return uintptr(s.eventHandle)
}

func (s *WASAPIStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	if s.renderClient != 0 {
		releaseCOM(s.renderClient)
		s.renderClient = 0
	}
	if s.client != 0 {
		releaseCOM(s.client)
		s.client = 0
	}
	if s.eventHandle != 0 {
		windows.CloseHandle(s.eventHandle)
		s.eventHandle = 0
	}
	return nil
}

// Suppress unused warning for math
var _ = math.Abs
