package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sound-signal-generator/internal/engine"
)

const (
	// CurrentPresetSchemaVersion is the schema format version for saved test presets.
	CurrentPresetSchemaVersion = "1.0"
)

var (
	// ErrInvalidPresetData is returned when preset JSON is corrupt or missing required fields.
	ErrInvalidPresetData = errors.New("invalid or corrupt preset data")

	// ErrUnsupportedPresetVersion is returned when preset schema version is incompatible.
	ErrUnsupportedPresetVersion = errors.New("unsupported preset schema version")
)

// SignalPreset represents a complete serialized instrument test preset.
type SignalPreset struct {
	SchemaVersion string                `json:"schema_version"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	Request       ExtendedSignalRequest `json:"request"`
}

// NewSignalPreset creates a new preset wrapper around an extended signal request.
func NewSignalPreset(name, description string, req ExtendedSignalRequest) SignalPreset {
	return SignalPreset{
		SchemaVersion: CurrentPresetSchemaVersion,
		Name:          name,
		Description:   description,
		CreatedAt:     time.Now().UTC(),
		Request:       req,
	}
}

// SavePresetToFile serializes a preset into a formatted JSON file.
func SavePresetToFile(filePath string, preset SignalPreset) error {
	if preset.Name == "" {
		preset.Name = "Untitled Preset"
	}
	preset.SchemaVersion = CurrentPresetSchemaVersion
	if preset.CreatedAt.IsZero() {
		preset.CreatedAt = time.Now().UTC()
	}

	data, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal preset: %w", err)
	}

	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for preset: %w", err)
		}
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write preset file: %w", err)
	}

	return nil
}

// LoadPresetFromFile reads and parses a preset from a JSON file.
func LoadPresetFromFile(filePath string) (*SignalPreset, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read preset file: %w", err)
	}

	var preset SignalPreset
	if err := json.Unmarshal(data, &preset); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPresetData, err)
	}

	if preset.SchemaVersion != CurrentPresetSchemaVersion {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrUnsupportedPresetVersion, CurrentPresetSchemaVersion, preset.SchemaVersion)
	}

	// Validate request fields
	if preset.Request.SampleRate <= 0 {
		return nil, fmt.Errorf("%w: sample_rate must be positive", ErrInvalidPresetData)
	}
	if preset.Request.Channels <= 0 {
		preset.Request.Channels = 2
	}

	return &preset, nil
}

// ApplyPresetSafely applies preset configuration to the controller.
// Mandatory safety invariant: Engine state MUST be StateOpenMuted upon completion,
// never automatically starting active audio playback.
func ApplyPresetSafely(ctrl *AppController, preset *SignalPreset) error {
	if ctrl == nil {
		return errors.New("controller cannot be nil")
	}
	if preset == nil {
		return errors.New("preset cannot be nil")
	}

	// 1. If currently playing, stop immediately
	if ctrl.engine.State() == engine.StateRunning || ctrl.engine.State() == engine.StatePrimed {
		_ = ctrl.StopPlayback()
	}

	// 2. Configure signal on the controller
	if err := ctrl.ConfigureExtendedSignal(preset.Request); err != nil {
		return fmt.Errorf("failed to configure signal from preset: %w", err)
	}

	// 3. Strict safety invariant check: MUST be OpenMuted or Closed, NOT Running
	state := ctrl.engine.State()
	if state == engine.StateRunning {
		_ = ctrl.StopPlayback()
		return errors.New("safety violation: preset attempted automatic playback start")
	}

	// If primed, transition or ensure muted
	if state != engine.StateOpenMuted && state != engine.StateClosed && state != engine.StatePrimed {
		return fmt.Errorf("unexpected engine state after applying preset: %s", state)
	}

	return nil
}
