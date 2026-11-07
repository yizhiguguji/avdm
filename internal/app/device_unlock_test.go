package app

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDeviceUnlockSettingsValidation(t *testing.T) {
	for _, tc := range []struct {
		s     DeviceUnlockSettings
		valid bool
	}{
		{DeviceUnlockSettings{Kind: "PIN", Secret: "123456"}, true},
		{DeviceUnlockSettings{Kind: "PIN", Secret: "12a4"}, false},
		{DeviceUnlockSettings{Kind: "密码", Secret: `a'\%s $(test);&"`}, true},
		{DeviceUnlockSettings{Kind: "密码", Secret: "中文密码"}, false},
		{DeviceUnlockSettings{Kind: "图案", Secret: "12589", Left: 25, Top: 50, Right: 75, Bottom: 80}, true},
		{DeviceUnlockSettings{Kind: "图案", Secret: "1359", Left: 25, Top: 50, Right: 75, Bottom: 80}, false},
		{DeviceUnlockSettings{Kind: "图案", Secret: "1125", Left: 25, Top: 50, Right: 75, Bottom: 80}, false},
		{DeviceUnlockSettings{Kind: "图案", Secret: "12589", Left: 75, Top: 50, Right: 25, Bottom: 80}, false},
	} {
		if err := validateDeviceUnlockSettings(tc.s); (err == nil) != tc.valid {
			t.Fatalf("kind %s validation: %v", tc.s.Kind, err)
		}
	}
}
func TestDeviceLockStateRequiresExplicitConsistentEvidence(t *testing.T) {
	for _, tc := range []struct {
		output        string
		locked, valid bool
	}{
		{"KeyguardServiceDelegate\n showing=true\n KeyguardStateMonitor\n mIsShowing=true\n", true, true},
		{"KeyguardServiceDelegate\n showing=false\n mIsShowing=false\n", false, true},
		{"unrelated showing=true", false, false},
		{"KeyguardServiceDelegate\n showing=true\n mIsShowing=false\n", false, false},
		{"", false, false},
	} {
		locked, err := deviceLockState(tc.output)
		if (err == nil) != tc.valid || locked != tc.locked {
			t.Fatalf("lock evidence misread: %q %t %v", tc.output, locked, err)
		}
	}
}
func TestUnlockPasswordScriptPreservesSymbolsWithoutShellExecution(t *testing.T) {
	s := DeviceUnlockSettings{Kind: "密码", Secret: `a'"\%s $HOME;$(exit 9)&`}
	script := `input() { if [ "$1" = text ]; then printf '%s\n' "$2"; fi; }
` + unlockCredentialScript(s, 1080, 2400)
	cmd := exec.Command("/bin/sh")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var actual strings.Builder
	for _, part := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if part == "%" {
			actual.WriteString(part)
		} else {
			actual.WriteString(strings.ReplaceAll(part, "%s", " "))
		}
	}
	if actual.String() != s.Secret {
		t.Fatal("password punctuation or whitespace changed")
	}
}
func TestUnlockPatternUsesContinuousGesture(t *testing.T) {
	s := DeviceUnlockSettings{Kind: "图案", Secret: "12589", Left: 25, Top: 50, Right: 75, Bottom: 80}
	script := unlockCredentialScript(s, 1080, 2400)
	if strings.Count(script, "motionevent DOWN") != 1 || strings.Count(script, "motionevent UP") != 1 || strings.Count(script, "motionevent MOVE") != 4 {
		t.Fatal("pattern must be one continuous gesture")
	}
	if !strings.Contains(script, "DOWN 270 1200") || !strings.Contains(script, "UP 810 1920") {
		t.Fatal("pattern calibration ignored")
	}
}
func TestUnlockSettingsAreNotExposedToGUI(t *testing.T) {
	// Loading the settings for a known hardware identity must omit its credential.
	path := t.TempDir() + "/adb"
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho demo-identity\n"), 0755); err != nil {
		t.Fatal(err)
	}
	a := &App{tools: &ToolResolver{statuses: map[string]ToolStatus{toolADB: {Available: true, Path: path}}}}
	state := a.unlockState()
	state.profiles["demo-identity"] = unlockCacheEntry{settings: DeviceUnlockSettings{Kind: "PIN", Secret: "123456", Configured: true}, paused: true}
	s, err := a.GUIGetDeviceUnlockSettings("demo-transport")
	if err != nil || s.Secret != "" || !s.Configured || !s.Paused {
		t.Fatal("GUI received credential or lost settings metadata")
	}
	if _, err := a.GUIAutoUnlockDevice("demo-transport"); err != nil {
		t.Fatal(err)
	}
}

func TestAutoUnlockStopsAfterOneFailedAttemptAndDoesNotLeakSecret(t *testing.T) {
	path := t.TempDir() + "/adb"
	script := `#!/bin/sh
case "$4" in
 getprop) echo demo-identity;;
 dumpsys) printf 'KeyguardServiceDelegate\n showing=true\n';;
 wm) echo 'Physical size: 1080x2400';;
 input) exit 0;;
 sh) cat >/dev/null; printf x >> "$0.calls"; echo '123456' >&2; exit 1;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	a := &App{tools: &ToolResolver{statuses: map[string]ToolStatus{toolADB: {Available: true, Path: path}}}}
	state := a.unlockState()
	state.profiles["demo-identity"] = unlockCacheEntry{settings: DeviceUnlockSettings{Kind: "PIN", Secret: "123456", Configured: true, Automatic: true}}
	attempted, err := a.GUIAutoUnlockDevice("demo-transport")
	if !attempted || err == nil || strings.Contains(err.Error(), "123456") {
		t.Fatalf("failed attempt not safely reported: %t %v", attempted, err)
	}
	attempted, err = a.GUIAutoUnlockDevice("demo-transport")
	if attempted || err != nil {
		t.Fatalf("paused automatic unlock retried: %t %v", attempted, err)
	}
	calls, err := os.ReadFile(path + ".calls")
	if err != nil || string(calls) != "x" {
		t.Fatal("credential sent more than once")
	}
}

func TestAutoConfirmPINDoesNotPressEnterInUnlockedApp(t *testing.T) {
	path := t.TempDir() + "/adb"
	script := `#!/bin/sh
case "$4" in
 getprop) echo demo-identity;;
 dumpsys) if [ -f "$0.ready" ]; then printf 'KeyguardServiceDelegate\n showing=false\n'; else printf 'KeyguardServiceDelegate\n showing=true\n'; fi;;
 wm) echo 'Physical size: 1080x2400';;
 input) if [ "$5" = keyevent ] && [ "$6" = 66 ]; then touch "$0.enter"; fi;;
 sh) cat >/dev/null; touch "$0.ready";;
esac
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	a := &App{tools: &ToolResolver{statuses: map[string]ToolStatus{toolADB: {Available: true, Path: path}}}}
	a.unlockState().profiles["demo-identity"] = unlockCacheEntry{settings: DeviceUnlockSettings{Kind: "PIN", Secret: "123456", Configured: true, Automatic: true}}
	attempted, err := a.GUIAutoUnlockDevice("demo-transport")
	if !attempted || err != nil {
		t.Fatalf("auto-confirm failed: %t %v", attempted, err)
	}
	if _, err := os.Stat(path + ".enter"); !os.IsNotExist(err) {
		t.Fatal("Enter sent into unlocked app")
	}
}
