package controller

import (
	"errors"
	"strings"
	"sync"
)

const (
	// DoPRiskWarningPrompt is the mandatory safety prompt defined by the project specification.
	DoPRiskWarningPrompt = "若 DAC 不支持 DoP，输出内容、频谱与电平不可预设，存在损伤听力与设备风险；测试前必须断开扬声器/耳机，或将后级放大器增益调至最低"

	// DoPConfirmationToken is the exact string token required to unlock DoP playback in interactive mode.
	DoPConfirmationToken = "YES"
)

var (
	// ErrDoPRiskNotConfirmed is returned when DoP output is requested without explicit confirmation.
	ErrDoPRiskNotConfirmed = errors.New("DoP safety risk not confirmed: device forced to stay in OpenMuted")
)

// DoPSafetyGuard enforces mandatory safety warnings and confirmation before allowing DoP carrier transmission.
type DoPSafetyGuard struct {
	mu        sync.RWMutex
	confirmed bool
}

// NewDoPSafetyGuard creates a new safety guard initialized in the locked/unconfirmed state.
func NewDoPSafetyGuard() *DoPSafetyGuard {
	return &DoPSafetyGuard{
		confirmed: false,
	}
}

// WarningPrompt returns the mandatory safety warning message.
func (g *DoPSafetyGuard) WarningPrompt() string {
	return DoPRiskWarningPrompt
}

// IsConfirmed returns true if the user has explicitly confirmed DoP risks.
func (g *DoPSafetyGuard) IsConfirmed() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.confirmed
}

// ConfirmWithToken verifies if input string equals "YES" (exact match) and unlocks DoP.
func (g *DoPSafetyGuard) ConfirmWithToken(input string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	trimmed := strings.TrimSpace(input)
	if trimmed == DoPConfirmationToken {
		g.confirmed = true
		return true
	}
	return false
}

// ConfirmViaFlag unlocks DoP if the command-line flag --confirm-dop-risk was supplied.
func (g *DoPSafetyGuard) ConfirmViaFlag(flag bool) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if flag {
		g.confirmed = true
		return true
	}
	return false
}

// Revoke locks DoP access and resets confirmed status back to false.
func (g *DoPSafetyGuard) Revoke() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.confirmed = false
}

// Validate ensures DoP transmission is permitted. If unconfirmed, returns ErrDoPRiskNotConfirmed.
func (g *DoPSafetyGuard) Validate(isDoP bool) error {
	if !isDoP {
		return nil
	}
	if !g.IsConfirmed() {
		return ErrDoPRiskNotConfirmed
	}
	return nil
}
