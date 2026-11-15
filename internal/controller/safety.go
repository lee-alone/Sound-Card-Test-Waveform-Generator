package controller

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SafetyRiskCategory classifies the hazard source.
type SafetyRiskCategory string

const (
	RiskNone               SafetyRiskCategory = "none"
	RiskUltrasound         SafetyRiskCategory = "ultrasound"
	RiskSquareWave         SafetyRiskCategory = "square_wave"
	RiskMultitoneHighCrest SafetyRiskCategory = "multitone_crest"
	RiskDoP                SafetyRiskCategory = "dop_carrier"
)

// SafetyAssessment details the hazards identified for a given signal configuration.
type SafetyAssessment struct {
	HasRisk              bool               `json:"has_risk"`
	Category             SafetyRiskCategory `json:"category"`
	Title                string             `json:"title"`
	WarningMessage       string             `json:"warning_message"`
	RequiresConfirmation bool               `json:"requires_confirmation"`
	IsConfirmed          bool               `json:"is_confirmed"`
}

// SafetyManager tracks risk evaluations and explicit user confirmations.
type SafetyManager struct {
	mu              sync.Mutex
	confirmedRisks  map[SafetyRiskCategory]bool
	suppressedRisks map[SafetyRiskCategory]bool
}

const safetyPreferencesVersion = 1

type safetyPreferencesFile struct {
	Version    int                         `json:"version"`
	Suppressed map[SafetyRiskCategory]bool `json:"suppressed"`
}

// NewSafetyManager creates a safety manager with empty confirmation cache.
func NewSafetyManager() *SafetyManager {
	return &SafetyManager{
		confirmedRisks:  make(map[SafetyRiskCategory]bool),
		suppressedRisks: make(map[SafetyRiskCategory]bool),
	}
}

// Assess inspects the signal parameters and determines if high-risk warnings apply.
func (sm *SafetyManager) Assess(req ExtendedSignalRequest) SafetyAssessment {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 1. DoP Risk (Highest Priority)
	if req.Waveform == WaveformDoP {
		confirmed := sm.isConfirmedLocked(RiskDoP)
		return SafetyAssessment{
			HasRisk:              true,
			Category:             RiskDoP,
			Title:                "高危操作警示：DoP DSD 载波直通",
			WarningMessage:       "若连接的 DAC 不支持 dCS DoP v1.1 协议，解码器会将高频标记误认为 PCM 音频并输出全幅近 0 dBFS 强粉红/白噪声音，存在永久性损伤听力与扬声器高音单元的严重风险！测试前请断开扬声器/耳机，或将后级放大器增益调至最低。",
			RequiresConfirmation: true,
			IsConfirmed:          confirmed,
		}
	}

	// 2. Ultrasound Risk (> 20 kHz)
	isUltrasound := false
	if req.Frequency > 20000.0 {
		isUltrasound = true
	}
	if (req.Waveform == WaveformLinearSweep || req.Waveform == WaveformLogarithmicSweep) && req.Sweep.EndFreq > 20000.0 {
		isUltrasound = true
	}
	if req.Waveform == WaveformMultitoneCustom {
		for _, f := range req.Multitone.Frequencies {
			if f > 20000.0 {
				isUltrasound = true
				break
			}
		}
	}

	if isUltrasound {
		confirmed := sm.isConfirmedLocked(RiskUltrasound)
		return SafetyAssessment{
			HasRisk:              true,
			Category:             RiskUltrasound,
			Title:                "安全警示：超声频段测试信号 (>20kHz)",
			WarningMessage:       "当前信号包含高于 20 kHz 的超声频段分量。人耳虽不可听，但持续大功率超声信号可能导致放大器高频自激过热或烧毁扬声器高音单元音圈。建议降低后级增益并在专业示波器监控下进行测试。",
			RequiresConfirmation: true,
			IsConfirmed:          confirmed,
		}
	}

	// 3. PolyBLEP Square Wave Risk
	if req.Waveform == WaveformPolyBlepSquare {
		confirmed := sm.isConfirmedLocked(RiskSquareWave)
		return SafetyAssessment{
			HasRisk:              true,
			Category:             RiskSquareWave,
			Title:                "安全提示：带限方波信号",
			WarningMessage:       "方波信号能量密度较高且富含奇次谐波。在大振幅（> -6 dBFS）下长时间输出可能对后级功放和扬声器带来持续高负荷热应力。请确认输出电平并在安全音量下运行。",
			RequiresConfirmation: true,
			IsConfirmed:          confirmed,
		}
	}

	// 4. Multitone / High Crest Factor Risk
	if req.Waveform == WaveformMultitoneSMPTE || req.Waveform == WaveformMultitoneCCIF || req.Waveform == WaveformMultitoneCustom {
		confirmed := sm.isConfirmedLocked(RiskMultitoneHighCrest)
		return SafetyAssessment{
			HasRisk:              true,
			Category:             RiskMultitoneHighCrest,
			Title:                "提示：多音互调与波峰因数注意",
			WarningMessage:       "多音频号具备较高的波峰因数 (PAPR)。尽管发生器已执行自动归一化防削波保护，但在接入高灵敏度耳放或功放时仍需防止瞬态过冲。",
			RequiresConfirmation: true,
			IsConfirmed:          confirmed,
		}
	}

	return SafetyAssessment{
		HasRisk:              false,
		Category:             RiskNone,
		RequiresConfirmation: false,
		IsConfirmed:          true,
	}
}

// Confirm explicitly records user agreement for the given risk category.
func (sm *SafetyManager) Confirm(category SafetyRiskCategory, confirmed bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.confirmedRisks[category] = confirmed
}

// IsConfirmed returns whether the risk category is currently acknowledged.
func (sm *SafetyManager) IsConfirmed(category SafetyRiskCategory) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.isConfirmedLocked(category)
}

func (sm *SafetyManager) isConfirmedLocked(category SafetyRiskCategory) bool {
	return sm.confirmedRisks[category] || sm.suppressedRisks[category]
}

// IsSuppressed reports whether the user permanently disabled prompts for a
// risk category on this machine.
func (sm *SafetyManager) IsSuppressed(category SafetyRiskCategory) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.suppressedRisks[category]
}

// Suppress records a permanent acknowledgement and persists it to the default
// user configuration directory.
func (sm *SafetyManager) Suppress(category SafetyRiskCategory) error {
	path, err := defaultSafetyPreferencesPath()
	if err != nil {
		return err
	}
	return sm.suppressToFile(category, path)
}

func (sm *SafetyManager) suppressToFile(category SafetyRiskCategory, path string) error {
	sm.mu.Lock()
	sm.suppressedRisks[category] = true
	sm.confirmedRisks[category] = true
	snapshot := make(map[SafetyRiskCategory]bool, len(sm.suppressedRisks))
	for risk, suppressed := range sm.suppressedRisks {
		if suppressed {
			snapshot[risk] = true
		}
	}
	sm.mu.Unlock()

	return saveSafetyPreferences(path, snapshot)
}

// LoadSafetyPreferences loads non-blocking prompt suppressions from the default
// user configuration directory. A missing file is not an error.
func (sm *SafetyManager) LoadSafetyPreferences() error {
	path, err := defaultSafetyPreferencesPath()
	if err != nil {
		return err
	}
	return sm.loadSafetyPreferences(path)
}

func (sm *SafetyManager) loadSafetyPreferences(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read safety preferences: %w", err)
	}

	var prefs safetyPreferencesFile
	if err := json.Unmarshal(data, &prefs); err != nil {
		return fmt.Errorf("parse safety preferences: %w", err)
	}
	if prefs.Version != safetyPreferencesVersion {
		return fmt.Errorf("unsupported safety preferences version %d", prefs.Version)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()
	for risk, suppressed := range prefs.Suppressed {
		if suppressed {
			sm.suppressedRisks[risk] = true
			sm.confirmedRisks[risk] = true
		}
	}
	return nil
}

func saveSafetyPreferences(path string, suppressed map[SafetyRiskCategory]bool) error {
	data, err := json.MarshalIndent(safetyPreferencesFile{
		Version:    safetyPreferencesVersion,
		Suppressed: suppressed,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal safety preferences: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create safety preferences directory: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write safety preferences: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace safety preferences: %w", err)
	}
	return nil
}

func defaultSafetyPreferencesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "SoundCardTestWaveformGenerator", "safety.json"), nil
}

// Reset clears all session confirmations and permanent suppressions.
func (sm *SafetyManager) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.confirmedRisks = make(map[SafetyRiskCategory]bool)
	sm.suppressedRisks = make(map[SafetyRiskCategory]bool)
}

// ValidateConfirmation checks whether the required risk confirmation has been satisfied.
func (sm *SafetyManager) ValidateConfirmation(req ExtendedSignalRequest) error {
	assessment := sm.Assess(req)
	if assessment.HasRisk && assessment.RequiresConfirmation && !assessment.IsConfirmed {
		return fmt.Errorf("safety confirmation required for %s: %s", assessment.Category, strings.ReplaceAll(assessment.Title, "\n", " "))
	}
	return nil
}
