package audio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var (
	// ErrLengthMismatch is returned when source and captured buffers have different lengths.
	ErrLengthMismatch = errors.New("source and captured data lengths do not match")
)

// BitPerfectStatus represents the UI verification tier for bit-perfect transmission.
type BitPerfectStatus string

const (
	StatusVerified     BitPerfectStatus = "Verified"
	StatusMismatch     BitPerfectStatus = "Mismatch"
	StatusEligibleOnly BitPerfectStatus = "Eligible"
	StatusUnknown      BitPerfectStatus = "Unknown"
)

// BitPerfectEvidenceRecord archives traceable hardware bit-perfect proof as specified in T3.6.
type BitPerfectEvidenceRecord struct {
	DeviceID              string           `json:"device_id"`
	DeviceName            string           `json:"device_name"`
	Driver                string           `json:"driver"`
	RequestedFormat       AudioFormat      `json:"requested_format"`
	ActualFormat          AudioFormat      `json:"actual_format"`
	SourceDataSHA256      string           `json:"source_data_sha256"`
	CapturedDataSHA256    string           `json:"captured_data_sha256"`
	TotalFramesCompared   int              `json:"total_frames_compared"`
	TotalBytesCompared    int              `json:"total_bytes_compared"`
	DiffOffsets           []int            `json:"diff_offsets"` // Byte offsets where difference occurred; empty if 100% bit-perfect
	DiffSampleIndices     []int            `json:"diff_sample_indices,omitempty"`
	BitPerfectEligible    bool             `json:"bit_perfect_eligible"`
	BitPerfectVerified    bool             `json:"bit_perfect_verified"`
	Status                BitPerfectStatus `json:"status"`
	VerificationTimestamp string           `json:"verification_timestamp"`
	Notes                 string           `json:"notes,omitempty"`
}

// GeneratePRBSSequence generates a deterministic pseudo-random PCM sequence using LFSR-32.
func GeneratePRBSSequence(numFrames int, format AudioFormat, seed uint32) []byte {
	if seed == 0 {
		seed = 0x12345678
	}

	bytesPerSample := format.SampleFormat.BytesPerSample()
	totalBytes := numFrames * format.Channels * bytesPerSample
	buf := make([]byte, totalBytes)

	lfsr := seed
	nextRand := func() uint32 {
		// Standard maximal-period 32-bit Galois LFSR polynomial (taps at 32, 31, 29, 1)
		lfsr = (lfsr >> 1) ^ (-(lfsr & 1) & 0x80200003)
		return lfsr
	}

	idx := 0
	for f := 0; f < numFrames; f++ {
		for ch := 0; ch < format.Channels; ch++ {
			r := nextRand()
			switch format.SampleFormat {
			case SampleFormat16Int:
				buf[idx] = byte(r)
				buf[idx+1] = byte(r >> 8)
				idx += 2

			case SampleFormat24Packed:
				buf[idx] = byte(r)
				buf[idx+1] = byte(r >> 8)
				buf[idx+2] = byte(r >> 16)
				idx += 3

			case SampleFormat24In32:
				// Little-endian 24-in-32: [0, B0, B1, B2]
				buf[idx] = 0x00
				buf[idx+1] = byte(r)
				buf[idx+2] = byte(r >> 8)
				buf[idx+3] = byte(r >> 16)
				idx += 4

			case SampleFormat32Float:
				// Float32 raw bits
				buf[idx] = byte(r)
				buf[idx+1] = byte(r >> 8)
				buf[idx+2] = byte(r >> 16)
				buf[idx+3] = byte(r >> 24)
				idx += 4
			}
		}
	}

	return buf
}

// CompareBitPerfect compares source and captured byte slices sample-by-sample and outputs an evidence record.
func CompareBitPerfect(source, captured []byte, reqFmt, actFmt AudioFormat, devName, devID, driverName string) (BitPerfectEvidenceRecord, error) {
	srcSum := sha256.Sum256(source)
	srcHex := hex.EncodeToString(srcSum[:])

	capSum := sha256.Sum256(captured)
	capHex := hex.EncodeToString(capSum[:])

	bytesPerFrame := actFmt.BytesPerFrame()
	if bytesPerFrame <= 0 {
		bytesPerFrame = 1
	}
	totalFrames := len(source) / bytesPerFrame

	rec := BitPerfectEvidenceRecord{
		DeviceID:              devID,
		DeviceName:            devName,
		Driver:                driverName,
		RequestedFormat:       reqFmt,
		ActualFormat:          actFmt,
		SourceDataSHA256:      srcHex,
		CapturedDataSHA256:    capHex,
		TotalFramesCompared:   totalFrames,
		TotalBytesCompared:    len(source),
		DiffOffsets:           make([]int, 0),
		DiffSampleIndices:     make([]int, 0),
		BitPerfectEligible:    reqFmt == actFmt,
		VerificationTimestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if len(source) != len(captured) {
		rec.Status = StatusMismatch
		rec.BitPerfectVerified = false
		rec.Notes = fmt.Sprintf("Length mismatch: source %d bytes, captured %d bytes", len(source), len(captured))
		return rec, ErrLengthMismatch
	}

	// Byte-by-byte comparison and recording of discrepancy locations
	maxRecordedDiffs := 100 // cap diff list size to prevent huge JSONs
	bytesPerSample := actFmt.SampleFormat.BytesPerSample()
	if bytesPerSample <= 0 {
		bytesPerSample = 1
	}

	for i := 0; i < len(source); i++ {
		if source[i] != captured[i] {
			if len(rec.DiffOffsets) < maxRecordedDiffs {
				rec.DiffOffsets = append(rec.DiffOffsets, i)
				rec.DiffSampleIndices = append(rec.DiffSampleIndices, i/bytesPerSample)
			}
		}
	}

	if len(rec.DiffOffsets) == 0 && srcHex == capHex {
		rec.BitPerfectVerified = true
		rec.Status = StatusVerified
		rec.Notes = "100% bit-perfect match across all samples and bytes"
	} else {
		rec.BitPerfectVerified = false
		rec.Status = StatusMismatch
		rec.Notes = fmt.Sprintf("Discrepancies found at %d byte offsets", len(rec.DiffOffsets))
	}

	return rec, nil
}

// SaveEvidence archives the record to a JSON file on disk.
func (r *BitPerfectEvidenceRecord) SaveEvidence(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}
