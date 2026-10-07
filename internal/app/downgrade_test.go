package app

import (
	"errors"
	"fmt"
	"testing"
)

func TestAnalyzeInstallFailureDetectsRealDowngradeOutput(t *testing.T) {
	// The exact adb output reproduced on device.
	out := "Performing Streamed Install\nadb: failed to install low.apk: Failure [INSTALL_FAILED_VERSION_DOWNGRADE: Downgrade detected: Update version code 1784086171 is older than current 1784097066]"
	hint := analyzeInstallFailure(out)
	if !hint.NeedsDowngrade {
		t.Fatalf("expected NeedsDowngrade for real downgrade output")
	}
}

func TestAsReinstallRequiredUnwrapsWrappedError(t *testing.T) {
	base := &ReinstallRequiredError{Package: "com.example.test", Serials: []string{"emulator-5554"}}
	wrapped := fmt.Errorf("安装失败：%w", base)
	got, ok := AsReinstallRequired(wrapped)
	if !ok {
		t.Fatalf("AsReinstallRequired should unwrap wrapped error")
	}
	if got.Package != "com.example.test" || len(got.Serials) != 1 {
		t.Fatalf("unexpected unwrapped value: %+v", got)
	}
}

func TestAsReinstallRequiredIgnoresOtherErrors(t *testing.T) {
	if _, ok := AsReinstallRequired(errors.New("some other failure")); ok {
		t.Fatalf("AsReinstallRequired must not match unrelated errors")
	}
}

func TestReinstallRequiredErrorMessageMentionsDataLoss(t *testing.T) {
	e := &ReinstallRequiredError{Package: "com.example.test", Serials: []string{"emulator-5554"}}
	msg := e.Error()
	if msg == "" {
		t.Fatal("empty error message")
	}
	// Should name the package and warn about data.
	if !contains(msg, "com.example.test") || !contains(msg, "数据") {
		t.Fatalf("message should mention package and data loss: %q", msg)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
