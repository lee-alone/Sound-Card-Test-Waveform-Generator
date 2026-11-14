package dsd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// DSDFormat defines DSD rate specifications.
type DSDFormat int

const (
	// DSD64: 2.8224 MHz (64 * 44.1 kHz), DoP carrier: 176.4 kHz
	DSD64 DSDFormat = iota
	// DSD128: 5.6448 MHz (128 * 44.1 kHz), DoP carrier: 352.8 kHz
	DSD128
	// DSD256: 11.2896 MHz (256 * 44.1 kHz), Native DSD offline vector
	DSD256
	// DSD512: 22.5792 MHz (512 * 44.1 kHz), Native DSD offline vector
	DSD512
)

func (f DSDFormat) String() string {
	switch f {
	case DSD64:
		return "DSD64 (2.8224MHz)"
	case DSD128:
		return "DSD128 (5.6448MHz)"
	case DSD256:
		return "DSD256 (11.2896MHz)"
	case DSD512:
		return "DSD512 (22.5792MHz)"
	default:
		return "Unknown DSD"
	}
}

func (f DSDFormat) MarshalJSON() ([]byte, error) {
	switch f {
	case DSD64:
		return json.Marshal("DSD64")
	case DSD128:
		return json.Marshal("DSD128")
	case DSD256:
		return json.Marshal("DSD256")
	case DSD512:
		return json.Marshal("DSD512")
	default:
		return json.Marshal("DSD64")
	}
}

func (f *DSDFormat) UnmarshalJSON(data []byte) error {
	var val int
	if err := json.Unmarshal(data, &val); err == nil {
		*f = DSDFormat(val)
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		clean := strings.ToUpper(strings.TrimSpace(str))
		switch clean {
		case "DSD64", "64":
			*f = DSD64
		case "DSD128", "128":
			*f = DSD128
		case "DSD256", "256":
			*f = DSD256
		case "DSD512", "512":
			*f = DSD512
		default:
			if strings.Contains(clean, "64") {
				*f = DSD64
			} else if strings.Contains(clean, "128") {
				*f = DSD128
			} else if strings.Contains(clean, "256") {
				*f = DSD256
			} else if strings.Contains(clean, "512") {
				*f = DSD512
			} else {
				*f = DSD64
			}
		}
		return nil
	}
	return fmt.Errorf("invalid DSDFormat: %s", string(data))
}

// BitRate returns bit clock frequency in Hz.
func (f DSDFormat) BitRate() int {
	switch f {
	case DSD64:
		return 2822400
	case DSD128:
		return 5644800
	case DSD256:
		return 11289600
	case DSD512:
		return 22579200
	default:
		return 0
	}
}

// DoPCarrierSampleRate returns the PCM sample rate required to carry DoP.
// Returns 0 for DSD256 and DSD512 because Phase 3 does not support DoP carriers for them.
func (f DSDFormat) DoPCarrierSampleRate() int {
	switch f {
	case DSD64:
		return 176400
	case DSD128:
		return 352800
	default:
		return 0
	}
}

// BitOrder defines the serialization bit order within each byte.
type BitOrder int

const (
	// BitOrderMSBFirst: standard DSD bitstream transmission (bit 7 first down to bit 0)
	BitOrderMSBFirst BitOrder = iota
	// BitOrderLSBFirst: bit 0 first up to bit 7 (used by certain ASIO hardware layouts)
	BitOrderLSBFirst
)

func (b BitOrder) String() string {
	switch b {
	case BitOrderMSBFirst:
		return "MSBFirst"
	case BitOrderLSBFirst:
		return "LSBFirst"
	default:
		return "Unknown"
	}
}

func (b BitOrder) MarshalJSON() ([]byte, error) {
	switch b {
	case BitOrderMSBFirst:
		return json.Marshal("MSBFirst")
	case BitOrderLSBFirst:
		return json.Marshal("LSBFirst")
	default:
		return json.Marshal("MSBFirst")
	}
}

func (b *BitOrder) UnmarshalJSON(data []byte) error {
	var val int
	if err := json.Unmarshal(data, &val); err == nil {
		*b = BitOrder(val)
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		clean := strings.ToLower(strings.TrimSpace(str))
		switch clean {
		case "msbfirst", "msb", "0":
			*b = BitOrderMSBFirst
		case "lsbfirst", "lsb", "1":
			*b = BitOrderLSBFirst
		default:
			*b = BitOrderMSBFirst
		}
		return nil
	}
	return fmt.Errorf("invalid BitOrder: %s", string(data))
}

// ModulationParameters records parameters used to generate the golden DSD bitstream.
type ModulationParameters struct {
	Order               int     `json:"order"`
	OverSamplingRatio   int     `json:"oversampling_ratio"`
	DitherType          string  `json:"dither_type"`
	SignalFrequencyHz   float64 `json:"signal_frequency_hz"`
	AmplitudeDBFS       float64 `json:"amplitude_dbfs"`
	TargetBandwidthHz   float64 `json:"target_bandwidth_hz"`
}

// DSDVectorMetadata records traceable provenance and crypto integrity verification data.
type DSDVectorMetadata struct {
	Format              DSDFormat            `json:"format"`
	Name                string               `json:"name"`
	ReferenceTool       string               `json:"reference_tool"`
	Version             string               `json:"version"`
	ModulationParams    ModulationParameters `json:"modulation_params"`
	BitOrder            BitOrder             `json:"bit_order"`
	ByteLength          int                  `json:"byte_length"`
	BitLength           int                  `json:"bit_length"`
	SHA256Checksum      string               `json:"sha256_checksum"`
	SupportedInPhase3   bool                 `json:"supported_in_phase_3"`
	SupportedCarrier    string               `json:"supported_carrier"`
}

// GoldenVector bundles the raw 1-bit stream with its immutable verification metadata.
type GoldenVector struct {
	Metadata DSDVectorMetadata
	Data     []byte
}

// VerifyChecksum validates that Data matches the registered SHA-256 checksum.
func (g *GoldenVector) VerifyChecksum() bool {
	sum := sha256.Sum256(g.Data)
	computedHex := hex.EncodeToString(sum[:])
	return computedHex == g.Metadata.SHA256Checksum
}

// Clone creates a detached copy of the golden vector.
func (g *GoldenVector) Clone() *GoldenVector {
	dataCopy := make([]byte, len(g.Data))
	copy(dataCopy, g.Data)
	return &GoldenVector{
		Metadata: g.Metadata,
		Data:     dataCopy,
	}
}

// String returns formatted summary.
func (g *GoldenVector) String() string {
	return fmt.Sprintf("%s: %d bytes, SHA256:%s", g.Metadata.Name, len(g.Data), g.Metadata.SHA256Checksum)
}
