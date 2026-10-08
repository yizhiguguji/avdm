package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScrcpyArgsAvoidTimeoutAndKeepDeviceActive(t *testing.T) {
	args := scrcpyArgs("emulator-5554", "安卓设备矩阵 test", false)
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "no-window-aspect-ratio-lock") {
		t.Fatal("forcing a common width introduces black side panels")
	}
	if strings.Contains(joined, "time-limit") {
		t.Fatalf("scrcpy production args must not use --time-limit: %v", args)
	}
	for _, want := range []string{
		"--serial=emulator-5554",
		"--max-size=1920",
		"--max-fps=60",
		"--video-codec=h264",
		"--no-audio",
		"--render-fit=letterbox",
		"--keyboard=uhid",
		"--window-title=安卓设备矩阵 test",
		"--window-width=0",
		"--window-height=594",
		"--no-clipboard-autosync",
		"--keep-active",
	} {
		if !scrcpyArgsContain(args, want) {
			t.Fatalf("missing scrcpy arg %q in %v", want, args)
		}
	}
}

func TestLooksLikeScrcpyCommand(t *testing.T) {
	for _, line := range []string{
		"123 /opt/homebrew/bin/scrcpy --serial=PHONE-TEST-001",
		"456 scrcpy --serial=PHONE-TEST-001",
	} {
		if !looksLikeScrcpyCommand(line) {
			t.Fatalf("expected scrcpy command to match: %s", line)
		}
	}
	for _, line := range []string{
		"123 adb -s PHONE-TEST-001 shell CLASSPATH=/data/local/tmp/scrcpy-server.jar app_process / com.genymobile.scrcpy.Server",
		"456 /opt/homebrew/bin/adb --serial=PHONE-TEST-001",
	} {
		if looksLikeScrcpyCommand(line) {
			t.Fatalf("expected non-scrcpy command to be ignored: %s", line)
		}
	}
}

func TestScrcpyCommandUsesSerial(t *testing.T) {
	for _, command := range []string{
		"/opt/homebrew/bin/scrcpy --serial=PHONE-TEST-001",
		"/opt/homebrew/bin/scrcpy --serial PHONE-TEST-001",
		"/opt/homebrew/bin/scrcpy -s PHONE-TEST-001",
	} {
		if !scrcpyCommandUsesSerial(command, "PHONE-TEST-001") {
			t.Fatalf("expected command to use serial: %s", command)
		}
	}
	if scrcpyCommandUsesSerial("/opt/homebrew/bin/scrcpy --serial=OTHER", "PHONE-TEST-001") {
		t.Fatal("unexpected serial match")
	}
	if scrcpyCommandUsesSerial("scrcpy --serial=PHONE-TEST-0012", "PHONE-TEST-001") {
		t.Fatal("serial prefix must not match another device")
	}
}

func TestScrcpyAlwaysOnTopIsOptionalAndDetectable(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		args := scrcpyArgs("phone", "phone mirror", enabled)
		if got := scrcpyCommandAlwaysOnTop("scrcpy " + strings.Join(args, " ")); got != enabled {
			t.Fatalf("enabled=%v args=%v", enabled, args)
		}
	}
	if scrcpyCommandAlwaysOnTop("scrcpy --window-title=--always-on-top") {
		t.Fatal("window title must not count as an option")
	}
}

func TestChangingWindowLevelStopsOldMirrorBeforeLaunchingReplacement(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, enabled := range []bool{true, false} {
		old := exec.Command("sleep", "30")
		if err := old.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() { _ = old.Wait(); close(done) }()
		t.Cleanup(func() { _ = old.Process.Kill(); <-done })
		serial := "adm-test-window-level"
		path := filepath.Join(t.TempDir(), "scrcpy")
		argsPath := path + ".args"
		// Exit immediately so this test never opens/focuses any actual window.
		script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + argsPath + "'\nexit 1\n"
		if err := os.WriteFile(path, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		a := &App{tools: &ToolResolver{statuses: map[string]ToolStatus{
			toolScrcpy: {Name: toolScrcpy, Path: path, Available: true},
		}}, scrcpySessions: map[string]*scrcpySession{
			serial: {Cmd: old, AlwaysOnTop: !enabled, Done: done},
		}}
		if err := a.startScrcpySession(serial, "test", enabled); err == nil {
			t.Fatal("expected replacement startup failure")
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("old mirror was not stopped")
		}
		args, err := os.ReadFile(argsPath)
		if err != nil {
			t.Fatal(err)
		}
		if got := scrcpyCommandAlwaysOnTop(string(args)); got != enabled {
			t.Fatalf("enabled=%v args=%s", enabled, args)
		}
		a.remoteMu.Lock()
		remaining := len(a.scrcpySessions)
		a.remoteMu.Unlock()
		if remaining != 0 {
			t.Fatal("failed replacement left a stale session")
		}
	}
}

func scrcpyArgsContain(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestScrcpyReuseRequiresUniformGeometry(t *testing.T) {
	for _, command := range []string{"scrcpy --serial=phone", "scrcpy --render-fit=letterbox", "scrcpy --no-window-aspect-ratio-lock"} {
		if scrcpyCommandUniformGeometry(command) {
			t.Fatalf("legacy mirror incorrectly reused: %s", command)
		}
	}
	if !scrcpyCommandUniformGeometry("scrcpy " + strings.Join(scrcpyArgs("phone", "mirror", false), " ")) {
		t.Fatal("current mirror should be reusable")
	}
}

func TestMirrorWithoutWindowAccessKeepsOpenedWindowUsable(t *testing.T) {
	for _, canResize := range []bool{false, true} {
		resizes, focuses := 0, 0
		err := prepareScrcpyWindowWithOps(42, canResize, func(pid, width, height int) error {
			resizes++
			if pid != 42 || width != 0 || height != scrcpyWindowHeight {
				t.Fatal("wrong window geometry")
			}
			return nil
		}, func(pid int) error {
			focuses++
			return nil
		})
		wantResizes := 0
		if canResize {
			wantResizes = 1
		}
		if err != nil || resizes != wantResizes || focuses != 1 {
			t.Fatalf("canResize=%v resizes=%d focuses=%d err=%v", canResize, resizes, focuses, err)
		}
	}
}

func TestMirrorFallbackDoesNotHideRealActivationFailure(t *testing.T) {
	failure := errors.New("process exited")
	err := prepareScrcpyWindowWithOps(42, false, func(int, int, int) error {
		t.Fatal("untrusted process tried to resize")
		return nil
	}, func(int) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("lost activation failure: %v", err)
	}
}

func TestMirrorArrangementRecreatesEvenPreviouslyPositionedWindow(t *testing.T) {
	command := "scrcpy " + strings.Join(scrcpyArgs("phone", "mirror", true), " ") + " --window-x=16 --window-y=60"
	if !canReuseScrcpyWindow(command, true, nil) {
		t.Fatal("ordinary focus must reuse a compatible mirror")
	}
	if canReuseScrcpyWindow(command, true, &scrcpyWindowPlacement{X: 16, Y: 100, Width: 266, Height: 624}) {
		t.Fatal("launch position does not prove the window has not been moved")
	}
	if canReuseScrcpyWindow(command, false, nil) {
		t.Fatal("different top setting must recreate mirror")
	}
}
