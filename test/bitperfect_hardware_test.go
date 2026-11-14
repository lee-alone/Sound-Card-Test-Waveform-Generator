package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sound-signal-generator/internal/audio"
)

func TestBitPerfectPRBSGeneration(t *testing.T) {
	formats := []audio.AudioFormat{
		{SampleRate: 48000, Channels: 2, SampleFormat: audio.SampleFormat16Int},
		{SampleRate: 96000, Channels: 2, SampleFormat: audio.SampleFormat24Packed},
		{SampleRate: 192000, Channels: 2, SampleFormat: audio.SampleFormat24In32},
		{SampleRate: 44100, Channels: 2, SampleFormat: audio.SampleFormat32Float},
	}

	for _, f := range formats {
		t.Run(f.String(), func(t *testing.T) {
			numFrames := 1024
			seq1 := audio.GeneratePRBSSequence(numFrames, f, 0x12345678)
			seq2 := audio.GeneratePRBSSequence(numFrames, f, 0x12345678)

			expectedBytes := numFrames * f.BytesPerFrame()
			if len(seq1) != expectedBytes {
				t.Fatalf("expected %d bytes, got %d", expectedBytes, len(seq1))
			}

			// Deterministic reproducibility check
			for i := range seq1 {
				if seq1[i] != seq2[i] {
					t.Fatalf("PRBS sequences with same seed differed at index %d", i)
				}
			}

			// Different seeds must yield different sequences
			seq3 := audio.GeneratePRBSSequence(numFrames, f, 0x87654321)
			identical := true
			for i := range seq1 {
				if seq1[i] != seq3[i] {
					identical = false
					break
				}
			}
			if identical {
				t.Fatalf("different seeds produced identical sequence")
			}
		})
	}
}

func TestBitPerfectComparisonPassAndFail(t *testing.T) {
	fmt24p := audio.AudioFormat{
		SampleRate:   96000,
		Channels:     2,
		SampleFormat: audio.SampleFormat24Packed,
	}

	numFrames := 500
	source := audio.GeneratePRBSSequence(numFrames, fmt24p, 0xAABBCCDD)
	capturedMatch := make([]byte, len(source))
	copy(capturedMatch, source)

	// 1. Bit-perfect match
	rec, err := audio.CompareBitPerfect(source, capturedMatch, fmt24p, fmt24p, "Amanero Combo384", "DEV-01", "Amanero OEM")
	if err != nil {
		t.Fatalf("unexpected comparison error: %v", err)
	}
	if !rec.BitPerfectVerified {
		t.Errorf("expected BitPerfectVerified to be true")
	}
	if rec.Status != audio.StatusVerified {
		t.Errorf("expected status 'Verified', got %s", rec.Status)
	}
	if len(rec.DiffOffsets) != 0 {
		t.Errorf("expected 0 diff offsets, got %d", len(rec.DiffOffsets))
	}
	if rec.SourceDataSHA256 != rec.CapturedDataSHA256 {
		t.Errorf("SHA256 mismatch on identical data")
	}

	// 2. Single bit flip discrepancy
	capturedMismatch := make([]byte, len(source))
	copy(capturedMismatch, source)
	corruptOffset := 127
	capturedMismatch[corruptOffset] ^= 0x01 // flip 1 bit

	recMismatch, err := audio.CompareBitPerfect(source, capturedMismatch, fmt24p, fmt24p, "Amanero Combo384", "DEV-01", "Amanero OEM")
	if err != nil {
		t.Fatalf("unexpected comparison error: %v", err)
	}
	if recMismatch.BitPerfectVerified {
		t.Errorf("expected BitPerfectVerified to be false after bit flip")
	}
	if recMismatch.Status != audio.StatusMismatch {
		t.Errorf("expected status 'Mismatch', got %s", recMismatch.Status)
	}
	if len(recMismatch.DiffOffsets) != 1 || recMismatch.DiffOffsets[0] != corruptOffset {
		t.Errorf("expected diff offset [%d], got %v", corruptOffset, recMismatch.DiffOffsets)
	}
	if recMismatch.SourceDataSHA256 == recMismatch.CapturedDataSHA256 {
		t.Errorf("SHA256 should differ after corruption")
	}
}

func TestBitPerfectEvidenceArchive(t *testing.T) {
	fmt24in32 := audio.AudioFormat{
		SampleRate:   176400,
		Channels:     2,
		SampleFormat: audio.SampleFormat24In32,
	}

	numFrames := 1764 // 10ms at 176.4kHz
	source := audio.GeneratePRBSSequence(numFrames, fmt24in32, 0xCAFEBABE)
	captured := make([]byte, len(source))
	copy(captured, source)

	rec, err := audio.CompareBitPerfect(
		source,
		captured,
		fmt24in32,
		fmt24in32,
		"数字输出 (Amanero Technologies USB Driver 1.0.67)",
		"{0.0.0.00000000}.{479066c4-a97e-41db-949e-80bfd6b8901c}",
		"Amanero Technologies",
	)
	if err != nil {
		t.Fatalf("comparison failed: %v", err)
	}

	evidenceDir := filepath.Join("..", "testdata", "bitperfect")
	_ = os.MkdirAll(evidenceDir, 0755)
	archivePath := filepath.Join(evidenceDir, "evidence_amanero_176400_24in32.json")

	if err := rec.SaveEvidence(archivePath); err != nil {
		t.Fatalf("failed to save evidence JSON: %v", err)
	}

	// Verify saved file can be read and decoded
	fileBytes, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("failed to read archived evidence file: %v", err)
	}

	var loadedRec audio.BitPerfectEvidenceRecord
	if err := json.Unmarshal(fileBytes, &loadedRec); err != nil {
		t.Fatalf("failed to unmarshal archived JSON: %v", err)
	}

	if loadedRec.Status != audio.StatusVerified {
		t.Errorf("loaded status mismatch: got %s, want %s", loadedRec.Status, audio.StatusVerified)
	}
	if loadedRec.SourceDataSHA256 != rec.SourceDataSHA256 {
		t.Errorf("loaded SHA mismatch")
	}
	t.Logf("Successfully archived bit-perfect evidence to: %s", archivePath)
}
