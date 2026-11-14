package dsd

import (
	"testing"
)

func TestGoldenVectorsCryptoIntegrity(t *testing.T) {
	vectors := GetAllGoldenVectors()
	if len(vectors) != 4 {
		t.Fatalf("expected 4 golden vectors, got %d", len(vectors))
	}

	for _, v := range vectors {
		t.Run(v.Metadata.Name, func(t *testing.T) {
			// 1. Verify SHA-256 matches bitstream
			if !v.VerifyChecksum() {
				t.Fatalf("SHA-256 integrity check failed for %s", v.Metadata.Name)
			}

			// 2. Verify ByteLength and BitLength
			if len(v.Data) != v.Metadata.ByteLength {
				t.Errorf("byte length mismatch: data has %d, metadata says %d", len(v.Data), v.Metadata.ByteLength)
			}
			if len(v.Data)*8 != v.Metadata.BitLength {
				t.Errorf("bit length mismatch: expected %d, got %d", len(v.Data)*8, v.Metadata.BitLength)
			}

			// 3. Verify bit order
			if v.Metadata.BitOrder != BitOrderMSBFirst {
				t.Errorf("expected BitOrderMSBFirst, got %v", v.Metadata.BitOrder)
			}

			// 4. Verify Phase 3 support declarations
			switch v.Metadata.Format {
			case DSD64:
				if !v.Metadata.SupportedInPhase3 {
					t.Errorf("DSD64 must be supported in Phase 3")
				}
				if v.Metadata.Format.DoPCarrierSampleRate() != 176400 {
					t.Errorf("expected 176400 carrier for DSD64, got %d", v.Metadata.Format.DoPCarrierSampleRate())
				}
			case DSD128:
				if !v.Metadata.SupportedInPhase3 {
					t.Errorf("DSD128 must be supported in Phase 3")
				}
				if v.Metadata.Format.DoPCarrierSampleRate() != 352800 {
					t.Errorf("expected 352800 carrier for DSD128, got %d", v.Metadata.Format.DoPCarrierSampleRate())
				}
			case DSD256, DSD512:
				// Must NOT claim Phase 3 DoP support
				if v.Metadata.SupportedInPhase3 {
					t.Errorf("%s must NOT claim SupportedInPhase3", v.Metadata.Name)
				}
				if v.Metadata.Format.DoPCarrierSampleRate() != 0 {
					t.Errorf("%s must return 0 for DoPCarrierSampleRate", v.Metadata.Name)
				}
			}
		})
	}
}

func TestGetGoldenVectorNotFound(t *testing.T) {
	_, err := GetGoldenVector(DSDFormat(999))
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
}

func TestGoldenVectorClone(t *testing.T) {
	v, err := GetGoldenVector(DSD64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clone := v.Clone()
	if clone == v {
		t.Fatal("clone must return a new pointer")
	}
	if len(clone.Data) != len(v.Data) {
		t.Fatalf("clone data length mismatch")
	}

	// Modify clone data and verify original is unchanged
	clone.Data[0] ^= 0xFF
	if clone.Data[0] == v.Data[0] {
		t.Fatal("mutating clone modified original data")
	}
	if clone.VerifyChecksum() {
		t.Fatal("mutated clone should fail checksum verification")
	}
	if !v.VerifyChecksum() {
		t.Fatal("original vector checksum should remain valid")
	}
}
