package dsd

import (
	"errors"
	"fmt"
)

var (
	// ErrUnknownDSDFormat is returned when an unrecognized DSDFormat is requested.
	ErrUnknownDSDFormat = errors.New("unknown or unsupported DSD format")
)

const (
	// ReferenceModulatorTool records generation tool identity.
	ReferenceModulatorTool = "Standard 2nd-Order Delta-Sigma Golden Reference Model"
	// ReferenceModulatorVersion records generation tool version.
	ReferenceModulatorVersion = "1.0.0"

	// GoldenChecksumDSD64 is the SHA-256 hash for DSD64 1kHz -6dBFS golden bitstream.
	GoldenChecksumDSD64 = "6a26a7dfc91684e0ecc53f8ece42c6035a1f32536532d2c5816526f6a38e6d69"
	// GoldenChecksumDSD128 is the SHA-256 hash for DSD128 1kHz -6dBFS golden bitstream.
	GoldenChecksumDSD128 = "67bccaf1badd436a7dedf57b0db6a6ae186a17eac96a80b3a76a3660ed8e1e24"
	// GoldenChecksumDSD256 is the SHA-256 hash for DSD256 1kHz -6dBFS golden bitstream.
	GoldenChecksumDSD256 = "24a69438cc0f94bfd34dcac3e66731f3fed9c49d74d13c6bf8256d7101b351e0"
	// GoldenChecksumDSD512 is the SHA-256 hash for DSD512 1kHz -6dBFS golden bitstream.
	GoldenChecksumDSD512 = "cac55ec3f9b48a7fd7dbf411496af34ce43f72aadd716ade7838ec721897ee8d"
)

// GetGoldenVector retrieves the immutable reference bitstream and metadata for the requested DSD format.
func GetGoldenVector(format DSDFormat) (*GoldenVector, error) {
	switch format {
	case DSD64:
		return &GoldenVector{
			Metadata: DSDVectorMetadata{
				Format:        DSD64,
				Name:          "DSD64-1kHz-Minus6dBFS-Golden",
				ReferenceTool: ReferenceModulatorTool,
				Version:       ReferenceModulatorVersion,
				ModulationParams: ModulationParameters{
					Order:             2,
					OverSamplingRatio: 64,
					DitherType:        "None (Deterministic Phase-Locked)",
					SignalFrequencyHz: 1000.0,
					AmplitudeDBFS:     -6.0,
					TargetBandwidthHz: 20000.0,
				},
				BitOrder:          BitOrderMSBFirst,
				ByteLength:        len(rawGoldenVectorDSD64),
				BitLength:         len(rawGoldenVectorDSD64) * 8,
				SHA256Checksum:    GoldenChecksumDSD64,
				SupportedInPhase3: true,
				SupportedCarrier:  "176.4kHz DoP v1.1",
			},
			Data: rawGoldenVectorDSD64,
		}, nil

	case DSD128:
		return &GoldenVector{
			Metadata: DSDVectorMetadata{
				Format:        DSD128,
				Name:          "DSD128-1kHz-Minus6dBFS-Golden",
				ReferenceTool: ReferenceModulatorTool,
				Version:       ReferenceModulatorVersion,
				ModulationParams: ModulationParameters{
					Order:             2,
					OverSamplingRatio: 128,
					DitherType:        "None (Deterministic Phase-Locked)",
					SignalFrequencyHz: 1000.0,
					AmplitudeDBFS:     -6.0,
					TargetBandwidthHz: 20000.0,
				},
				BitOrder:          BitOrderMSBFirst,
				ByteLength:        len(rawGoldenVectorDSD128),
				BitLength:         len(rawGoldenVectorDSD128) * 8,
				SHA256Checksum:    GoldenChecksumDSD128,
				SupportedInPhase3: true,
				SupportedCarrier:  "352.8kHz DoP v1.1",
			},
			Data: rawGoldenVectorDSD128,
		}, nil

	case DSD256:
		return &GoldenVector{
			Metadata: DSDVectorMetadata{
				Format:        DSD256,
				Name:          "DSD256-1kHz-Minus6dBFS-Golden",
				ReferenceTool: ReferenceModulatorTool,
				Version:       ReferenceModulatorVersion,
				ModulationParams: ModulationParameters{
					Order:             2,
					OverSamplingRatio: 256,
					DitherType:        "None (Deterministic Phase-Locked)",
					SignalFrequencyHz: 1000.0,
					AmplitudeDBFS:     -6.0,
					TargetBandwidthHz: 20000.0,
				},
				BitOrder:          BitOrderMSBFirst,
				ByteLength:        len(rawGoldenVectorDSD256),
				BitLength:         len(rawGoldenVectorDSD256) * 8,
				SHA256Checksum:    GoldenChecksumDSD256,
				SupportedInPhase3: false, // Phase 3 offline vector only, not claiming DoP carrier
				SupportedCarrier:  "Native DSD Only (Phase 5)",
			},
			Data: rawGoldenVectorDSD256,
		}, nil

	case DSD512:
		return &GoldenVector{
			Metadata: DSDVectorMetadata{
				Format:        DSD512,
				Name:          "DSD512-1kHz-Minus6dBFS-Golden",
				ReferenceTool: ReferenceModulatorTool,
				Version:       ReferenceModulatorVersion,
				ModulationParams: ModulationParameters{
					Order:             2,
					OverSamplingRatio: 512,
					DitherType:        "None (Deterministic Phase-Locked)",
					SignalFrequencyHz: 1000.0,
					AmplitudeDBFS:     -6.0,
					TargetBandwidthHz: 20000.0,
				},
				BitOrder:          BitOrderMSBFirst,
				ByteLength:        len(rawGoldenVectorDSD512),
				BitLength:         len(rawGoldenVectorDSD512) * 8,
				SHA256Checksum:    GoldenChecksumDSD512,
				SupportedInPhase3: false, // Phase 3 offline vector only, not claiming DoP carrier
				SupportedCarrier:  "Native DSD Only (Phase 5)",
			},
			Data: rawGoldenVectorDSD512,
		}, nil

	default:
		return nil, fmt.Errorf("%w: %v", ErrUnknownDSDFormat, format)
	}
}

// GetAllGoldenVectors returns the complete suite of golden test vectors.
func GetAllGoldenVectors() []*GoldenVector {
	formats := []DSDFormat{DSD64, DSD128, DSD256, DSD512}
	vectors := make([]*GoldenVector, 0, len(formats))
	for _, f := range formats {
		v, err := GetGoldenVector(f)
		if err == nil {
			vectors = append(vectors, v)
		}
	}
	return vectors
}
