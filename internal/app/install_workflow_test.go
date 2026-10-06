package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fakeInstallADB(t *testing.T, body string) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	path := filepath.Join(dir, "adb")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + log + "'\n" + body
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return &App{tools: &ToolResolver{statuses: map[string]ToolStatus{
		toolADB: {Name: toolADB, Path: path, Available: true},
	}}}, log
}

func TestSignatureConflictNeverUninstallsWithoutConfirmation(t *testing.T) {
	a, log := fakeInstallADB(t, `
if [ "$3" = shell ]; then
  echo 'Filesystem 1K-blocks Used Available Use% Mounted on'
  echo '/dev/data 99999999 0 99999999 0% /data'
  exit 0
fi
echo 'Performing Streamed Install'
echo 'adb: failed to install: Failure [INSTALL_FAILED_UPDATE_INCOMPATIBLE: signatures do not match]'
exit 1
`)
	path := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(path, []byte("invalid manifest"), 0600); err != nil {
		t.Fatal(err)
	}
	err := a.smartInstallAPKForGUI("test-device", path, installMode{AllowTestOnly: true})
	blocked, ok := AsReinstallRequired(err)
	if !ok || !strings.Contains(blocked.Reason, "签名") || !reflect.DeepEqual(blocked.Serials, []string{"test-device"}) {
		t.Fatalf("expected signature confirmation, got %v", err)
	}
	// Even after confirmation, a corrupt APK must never lead to uninstalling.
	err = a.smartInstallAPKForGUI("test-device", path, installMode{AllowReinstall: true})
	if err == nil || !strings.Contains(err.Error(), "无法解析") {
		t.Fatalf("got %v", err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "uninstall") {
		t.Fatalf("unexpected uninstall: %s", calls)
	}
}

func TestConfirmedReinstallClearsDataAndStopsOnUninstallFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		body := "echo Success\n"
		if fail {
			body = "echo 'Failure [DELETE_FAILED_INTERNAL_ERROR]'\nexit 1\n"
		}
		a, log := fakeInstallADB(t, body)
		err := a.reinstallAfterUninstall("device", "com.example.app", "/app.apk", installMode{AllowTestOnly: true})
		if (err != nil) != fail {
			t.Fatalf("fail=%v err=%v", fail, err)
		}
		calls, readErr := os.ReadFile(log)
		if readErr != nil {
			t.Fatal(readErr)
		}
		want := "-s device uninstall com.example.app\n"
		if !fail {
			want += "-s device install -t /app.apk\n"
		}
		if string(calls) != want {
			t.Fatalf("calls=%q want=%q", calls, want)
		}
	}
}

func TestUninstallOptions(t *testing.T) {
	for _, tc := range []struct {
		keep, user bool
		want       string
	}{
		{false, false, "-s device uninstall com.example.app"},
		{true, false, "-s device shell pm uninstall -k com.example.app"},
		{false, true, "-s device shell pm uninstall --user 0 com.example.app"},
		{true, true, "-s device shell pm uninstall -k --user 0 com.example.app"},
	} {
		got := strings.Join(uninstallArgs("device", "com.example.app", tc.keep, tc.user), " ")
		if got != tc.want {
			t.Fatalf("got=%q want=%q", got, tc.want)
		}
	}
}

func TestToolErrorsPreserveUnderlyingFailure(t *testing.T) {
	a, _ := fakeInstallADB(t, "echo 'Performing Streamed Install'\necho 'Failure [INSTALL_FAILED_UPDATE_INCOMPATIBLE]'\nexit 1\n")
	_, err := a.runToolOutput(toolADB, time.Second, "install", "/app.apk")
	if err == nil || !strings.Contains(err.Error(), "INSTALL_FAILED_UPDATE_INCOMPATIBLE") {
		t.Fatalf("got %v", err)
	}
}
