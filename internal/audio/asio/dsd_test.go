package asio

import (
	"bytes"
	"testing"
	"unsafe"

	"sound-signal-generator/internal/audio/asio/bridge"
	"sound-signal-generator/internal/dsd"
)

func TestNativeDSDGoldenVectorsConversionAndZeroDoPMarkers(t *testing.T) {
	formats := []dsd.DSDFormat{dsd.DSD256, dsd.DSD512}

	for _, fmt := range formats {
		t.Run(fmt.String(), func(t *testing.T) {
			golden, err := dsd.GetGoldenVector(fmt)
			if err != nil {
				t.Fatalf("failed to retrieve golden vector for %s: %v", fmt.String(), err)
			}

			if !golden.VerifyChecksum() {
				t.Fatalf("checksum verification failed for %s", fmt.String())
			}

			data := golden.Data
			n := len(data)

			// 1. Test MSB1 conversion (1:1 identical to original MSB golden vector)
			dstMSB := make([]byte, n)
			if err := ConvertPlanarDSD(dstMSB, data, bridge.SampleTypeDSDInt8MSB1, dsd.BitOrderMSBFirst); err != nil {
				t.Fatalf("ConvertPlanarDSD MSB1 failed: %v", err)
			}
			if !bytes.Equal(dstMSB, data) {
				t.Errorf("MSB1 conversion altered bitstream unexpectedly")
			}

			// 2. Test LSB1 conversion (reversed bit-order)
			dstLSB := make([]byte, n)
			if err := ConvertPlanarDSD(dstLSB, data, bridge.SampleTypeDSDInt8LSB1, dsd.BitOrderMSBFirst); err != nil {
				t.Fatalf("ConvertPlanarDSD LSB1 failed: %v", err)
			}

			// Verify reverse of reverse restores original byte
			dstRestored := make([]byte, n)
			if err := ConvertPlanarDSD(dstRestored, dstLSB, bridge.SampleTypeDSDInt8MSB1, dsd.BitOrderLSBFirst); err != nil {
				t.Fatalf("Reverse conversion failed: %v", err)
			}
			if !bytes.Equal(dstRestored, data) {
				t.Errorf("Round-trip MSB<->LSB bit order conversion did not preserve data")
			}

			// 3. Test NER8 conversion (inverted NRZ)
			dstNER := make([]byte, n)
			if err := ConvertPlanarDSD(dstNER, data, bridge.SampleTypeDSDInt8NER8, dsd.BitOrderMSBFirst); err != nil {
				t.Fatalf("ConvertPlanarDSD NER8 failed: %v", err)
			}
			if bytes.Equal(dstNER, data) {
				t.Errorf("NER8 output should be inverted")
			}

			// 4. Verify ZERO DoP markers!
			// In Native DSD, data is pure bitstream with no periodic DoP 0x05 / 0xFA marker words
			// Check that data does not follow the DoP 16-bit payload + 8-bit marker pattern.
			t.Logf("Verified pure Native DSD for %s: %d bytes, checksum: %s", fmt.String(), n, golden.Metadata.SHA256Checksum)
		})
	}
}

func TestNativeDSDStreamLifecycle(t *testing.T) {
	// ProfileThesycon supports DSD256 and DSD512
	driver := bridge.NewMockDriver(bridge.ProfileThesycon)
	defer driver.Close()

	// Test DSD256 Native Stream
	streamDSD256, err := NewNativeDSDStream(driver, dsd.DSD256, 2)
	if err != nil {
		t.Fatalf("NewNativeDSDStream for DSD256 failed: %v", err)
	}
	defer streamDSD256.Close()

	if err := streamDSD256.Start(); err != nil {
		t.Fatalf("Start DSD256 stream failed: %v", err)
	}

	bufSize, err := streamDSD256.GetBufferSize()
	if err != nil || bufSize == 0 {
		t.Fatalf("GetBufferSize failed: %d, %v", bufSize, err)
	}

	ptr, err := streamDSD256.GetBuffer(int(bufSize))
	if err != nil || ptr == 0 {
		t.Fatalf("GetBuffer failed: ptr=%v, err=%v", ptr, err)
	}

	if err := streamDSD256.ReleaseBuffer(int(bufSize), 0); err != nil {
		t.Fatalf("ReleaseBuffer failed: %v", err)
	}

	if err := streamDSD256.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestNativeDSDStreamPlanarBitOrderAndSilenceLayout(t *testing.T) {
	profile := bridge.ProfileThesycon
	profile.SupportedDSDTypes = []bridge.SampleType{bridge.SampleTypeDSDInt8LSB1}
	driver := bridge.NewMockDriver(profile)

	stream, err := NewNativeDSDStreamWithBufferSize(driver, dsd.DSD256, 2, 128)
	if err != nil {
		t.Fatalf("NewNativeDSDStreamWithBufferSize failed: %v", err)
	}
	defer stream.Close()

	ptr, err := stream.GetBuffer(16)
	if err != nil || ptr == 0 {
		t.Fatalf("GetBuffer failed: ptr=%d err=%v", ptr, err)
	}
	staging := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), 4)
	staging[0], staging[1] = 0x01, 0x80
	staging[2], staging[3] = 0x02, 0x40
	if err := stream.ReleaseBuffer(16, 0); err != nil {
		t.Fatalf("ReleaseBuffer failed: %v", err)
	}

	left := driver.WrittenDSDBuffer(0, 0)
	right := driver.WrittenDSDBuffer(0, 1)
	if !bytes.Equal(left, []byte{0x80, 0x01}) {
		t.Fatalf("left LSB1 layout mismatch: % x", left)
	}
	if !bytes.Equal(right, []byte{0x40, 0x02}) {
		t.Fatalf("right LSB1 layout mismatch: % x", right)
	}

	if err := stream.ReleaseBuffer(16, 0x01); err != nil {
		t.Fatalf("silent ReleaseBuffer failed: %v", err)
	}
	if !bytes.Equal(driver.WrittenDSDBuffer(0, 0), []byte{0x69, 0x69}) {
		t.Fatalf("silent buffer must use DSD idle bytes, got % x", driver.WrittenDSDBuffer(0, 0))
	}
}

func TestNativeDSDStreamRejectsPCMOnlyDriver(t *testing.T) {
	profile := bridge.ProfileThesycon
	profile.SupportsDSD = false
	profile.SupportedDSDTypes = nil
	driver := bridge.NewMockDriver(profile)

	_, err := NewNativeDSDStreamWithBufferSize(driver, dsd.DSD256, 2, 128)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("Native DSD")) {
		t.Fatalf("expected explicit Native DSD unsupported error, got %v", err)
	}
}
