package audio

import "testing"

func TestStreamFlagSilentMatchesWASAPI(t *testing.T) {
	// AUDCLNT_BUFFERFLAGS_SILENT is 0x2 on Windows. 0x1 is a different
	// buffer-status flag and is rejected by IAudioRenderClient::ReleaseBuffer.
	if StreamFlagSilent != 0x00000002 {
		t.Fatalf("StreamFlagSilent = 0x%X, want WASAPI AUDCLNT_BUFFERFLAGS_SILENT (0x2)", StreamFlagSilent)
	}
}
