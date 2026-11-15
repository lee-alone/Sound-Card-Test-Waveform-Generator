//go:build windows

package wasapi

import (
	"runtime"
	"testing"
	"unsafe"

	"sound-signal-generator/internal/audio"
)

func TestConvertAudioFormatToWaveFormat(t *testing.T) {
	tests := []struct {
		name          string
		format        audio.AudioFormat
		wantTag       uint16
		wantChannels  uint16
		wantBits      uint16
		wantValidBits uint16
		wantBlock     uint16
		wantSubFormat [16]byte
	}{
		{
			name: "16-bit Integer 48kHz Stereo",
			format: audio.AudioFormat{
				SampleRate:   48000,
				Channels:     2,
				SampleFormat: audio.SampleFormat16Int,
				ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
			},
			wantTag:       WAVE_FORMAT_EXTENSIBLE,
			wantChannels:  2,
			wantBits:      16,
			wantValidBits: 16,
			wantBlock:     4,
			wantSubFormat: *(*[16]byte)(unsafe.Pointer(&KSDATAFORMAT_SUBTYPE_PCM)),
		},
		{
			name: "24-bit Packed 96kHz Stereo",
			format: audio.AudioFormat{
				SampleRate:   96000,
				Channels:     2,
				SampleFormat: audio.SampleFormat24Packed,
				ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
			},
			wantTag:       WAVE_FORMAT_EXTENSIBLE,
			wantChannels:  2,
			wantBits:      24,
			wantValidBits: 24,
			wantBlock:     6,
			wantSubFormat: *(*[16]byte)(unsafe.Pointer(&KSDATAFORMAT_SUBTYPE_PCM)),
		},
		{
			name: "24-in-32-bit 192kHz Stereo",
			format: audio.AudioFormat{
				SampleRate:   192000,
				Channels:     2,
				SampleFormat: audio.SampleFormat24In32,
				ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
			},
			wantTag:       WAVE_FORMAT_EXTENSIBLE,
			wantChannels:  2,
			wantBits:      32,
			wantValidBits: 24,
			wantBlock:     8,
			wantSubFormat: *(*[16]byte)(unsafe.Pointer(&KSDATAFORMAT_SUBTYPE_PCM)),
		},
		{
			name: "32-bit Float 192kHz Stereo",
			format: audio.AudioFormat{
				SampleRate:   192000,
				Channels:     2,
				SampleFormat: audio.SampleFormat32Float,
				ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
			},
			wantTag:       WAVE_FORMAT_EXTENSIBLE,
			wantChannels:  2,
			wantBits:      32,
			wantValidBits: 32,
			wantBlock:     8,
			wantSubFormat: *(*[16]byte)(unsafe.Pointer(&KSDATAFORMAT_SUBTYPE_IEEE_FLOAT)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := ConvertAudioFormatToWaveFormat(tt.format)
			if wf.WFormatTag != tt.wantTag {
				t.Errorf("WFormatTag = %d, want %d", wf.WFormatTag, tt.wantTag)
			}
			if wf.NChannels != tt.wantChannels {
				t.Errorf("NChannels = %d, want %d", wf.NChannels, tt.wantChannels)
			}
			if wf.WBitsPerSample != tt.wantBits {
				t.Errorf("WBitsPerSample = %d, want %d", wf.WBitsPerSample, tt.wantBits)
			}
			if wf.ValidBitsPerSample != tt.wantValidBits {
				t.Errorf("ValidBitsPerSample = %d, want %d", wf.ValidBitsPerSample, tt.wantValidBits)
			}
			if wf.NBlockAlign != tt.wantBlock {
				t.Errorf("NBlockAlign = %d, want %d", wf.NBlockAlign, tt.wantBlock)
			}
			subBytes := *(*[16]byte)(unsafe.Pointer(&wf.SubFormat))
			if subBytes != tt.wantSubFormat {
				t.Errorf("SubFormat GUID mismatch")
			}
		})
	}
}

func TestStreamFlagsEnableSharedModeAutomaticConversion(t *testing.T) {
	shared := streamFlagsForShareMode(audio.ShareModeShared)
	wantShared := uint32(AUDCLNT_STREAMFLAGS_EVENTCALLBACK |
		AUDCLNT_STREAMFLAGS_NOPERSIST |
		AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM |
		AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY)
	if shared != wantShared {
		t.Fatalf("shared flags = 0x%08X, want 0x%08X", shared, wantShared)
	}

	exclusive := streamFlagsForShareMode(audio.ShareModeExclusive)
	if exclusive != uint32(AUDCLNT_STREAMFLAGS_EVENTCALLBACK) {
		t.Fatalf("exclusive flags = 0x%08X, want event callback only", exclusive)
	}
}

func TestCOMAndMMCSSInitialization(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := CoInitializeExWrapper(COINIT_MULTITHREADED)
	if err != nil {
		t.Fatalf("CoInitializeExWrapper failed: %v", err)
	}
	defer CoUninitializeWrapper()

	// Test MMCSS
	hMMCSS, taskIdx, err := SetMmThreadCharacteristics("Pro Audio")
	if err != nil {
		t.Logf("MMCSS 'Pro Audio' not available on this host: %v", err)
	} else {
		t.Logf("MMCSS active: taskIndex=%d", taskIdx)
		err = RevertMmThreadCharacteristics(hMMCSS)
		if err != nil {
			t.Errorf("RevertMmThreadCharacteristics failed: %v", err)
		}
	}
}

func TestWASAPIBackendEnumeration(t *testing.T) {
	backend := NewBackend()
	if err := backend.Initialize(); err != nil {
		t.Fatalf("backend.Initialize failed: %v", err)
	}
	defer backend.Close()

	devices, err := backend.EnumerateDevices()
	if err != nil {
		t.Fatalf("EnumerateDevices failed: %v", err)
	}

	t.Logf("Discovered %d audio render devices", len(devices))
	for _, d := range devices {
		t.Logf("  Device: %s (ID: %s, Default: %v, State: %v)", d.Name, d.ID, d.IsDefault, d.State)
	}

	if len(devices) > 0 {
		defDev, err := backend.GetDefaultDevice()
		if err != nil {
			t.Logf("GetDefaultDevice error: %v", err)
		} else {
			defer defDev.Close()
			t.Logf("Default Device Name: %s", defDev.Name())

			// Probe candidates
			candidates := audio.GenerateStandardCandidateMatrix(2)
			results, err := defDev.ProbeCapabilities(candidates, audio.ShareModeExclusive)
			if err != nil {
				t.Fatalf("ProbeCapabilities failed: %v", err)
			}
			t.Logf("Probed %d format candidates on default device:", len(results))
			for _, r := range results {
				t.Logf("    %s -> %s (HR: 0x%08X %s)", r.Format, r.Status, uint32(r.HResult), r.ErrorMessage)
			}
		}
	}
}
