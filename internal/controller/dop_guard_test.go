package controller

import (
	"errors"
	"testing"
)

func TestDoPSafetyGuardDefaultLocked(t *testing.T) {
	guard := NewDoPSafetyGuard()
	if guard.IsConfirmed() {
		t.Fatal("DoPSafetyGuard must be locked by default")
	}

	err := guard.Validate(true)
	if !errors.Is(err, ErrDoPRiskNotConfirmed) {
		t.Fatalf("expected ErrDoPRiskNotConfirmed, got %v", err)
	}

	// Non-DoP should pass regardless
	if err := guard.Validate(false); err != nil {
		t.Fatalf("non-DoP should pass validation, got %v", err)
	}
}

func TestDoPConfirmationExactToken(t *testing.T) {
	guard := NewDoPSafetyGuard()

	// Rejections
	invalidTokens := []string{"yes", "Y", "y", "true", "1", "OK", "confirm", " YES "}
	for _, tok := range invalidTokens {
		// Note: TrimSpace is tested, but let's test lowercase vs uppercase
		if tok == " YES " {
			if !guard.ConfirmWithToken(tok) {
				t.Errorf("TrimSpace should allow ' YES '")
			}
			guard.Revoke()
			continue
		}
		if guard.ConfirmWithToken(tok) {
			t.Errorf("token %q should not confirm DoP", tok)
		}
		if guard.IsConfirmed() {
			t.Errorf("guard should remain locked after token %q", tok)
		}
	}

	// Successful confirmation
	if !guard.ConfirmWithToken("YES") {
		t.Fatal("exact token 'YES' failed to confirm")
	}
	if !guard.IsConfirmed() {
		t.Fatal("guard should be confirmed after 'YES'")
	}
	if err := guard.Validate(true); err != nil {
		t.Fatalf("validation should succeed after confirmation, got %v", err)
	}

	// Revocation
	guard.Revoke()
	if guard.IsConfirmed() {
		t.Fatal("guard should be locked after Revoke()")
	}
}

func TestDoPConfirmationFlag(t *testing.T) {
	guard := NewDoPSafetyGuard()

	if guard.ConfirmViaFlag(false) {
		t.Fatal("flag=false should not confirm")
	}
	if guard.IsConfirmed() {
		t.Fatal("guard should remain locked")
	}

	if !guard.ConfirmViaFlag(true) {
		t.Fatal("flag=true should confirm")
	}
	if !guard.IsConfirmed() {
		t.Fatal("guard should be confirmed")
	}
}

func TestDoPWarningPromptContent(t *testing.T) {
	guard := NewDoPSafetyGuard()
	expected := "若 DAC 不支持 DoP，输出内容、频谱与电平不可预设，存在损伤听力与设备风险；测试前必须断开扬声器/耳机，或将后级放大器增益调至最低"
	if guard.WarningPrompt() != expected {
		t.Errorf("warning prompt text mismatch:\ngot:  %q\nwant: %q", guard.WarningPrompt(), expected)
	}
}
