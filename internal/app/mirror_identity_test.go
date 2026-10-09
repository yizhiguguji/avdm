package app

import (
	"os"
	"os/exec"
	"testing"
)

func TestShortMirrorTitleKeepsDeviceIdentity(t *testing.T) {
	a := &App{cfg: &Config{Aliases: map[string]string{aliasKeyForSerial("PHONE-TEST-001"): "测试手机"}}}
	for _, row := range []struct {
		entry DeviceEntry
		want  string
	}{
		{DeviceEntry{Label: "真机 | model | device | serial", Active: &ActiveDevice{Serial: "PHONE-TEST-002", Details: map[string]string{"model": "Demo_Phone"}}}, "Demo_Phone"},
		{DeviceEntry{Active: &ActiveDevice{Serial: "PHONE-TEST-001", Details: map[string]string{"model": "Demo_Phone"}}}, "测试手机"},
		{DeviceEntry{AVD: &AVD{Name: "demo_avd"}, Active: &ActiveDevice{Serial: "emulator-5554", Details: map[string]string{"model": "sdk_gphone64"}}}, "demo_avd"},
		{DeviceEntry{Active: &ActiveDevice{Serial: "PHONE-TEST-003"}}, "PHONE-TEST-003"},
	} {
		if got := a.liveMirrorTitle(row.entry); got != row.want {
			t.Fatalf("title=%q, want %q", got, row.want)
		}
	}
}

func TestMirrorOwnerRecordSurvivesTitleChangeAndRejectsStalePID(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	pid := cmd.Process.Pid
	if registeredScrcpyOwner(dir, pid) {
		t.Fatal("unregistered process matched")
	}
	if err := registerScrcpyOwner(dir, pid); err != nil {
		t.Fatal(err)
	}
	if !registeredScrcpyOwner(dir, pid) {
		t.Fatal("registered process not recognized")
	}
	if err := os.WriteFile(mirrorOwnerPath(dir, pid), []byte("stale process start"), 0600); err != nil {
		t.Fatal(err)
	}
	if registeredScrcpyOwner(dir, pid) {
		t.Fatal("stale process identity matched")
	}
	if !appOwnedScrcpyProcess(0, "scrcpy --window-title=安卓设备矩阵 - Demo_Phone | PHONE-TEST-001 --no-audio") {
		t.Fatal("legacy mirror no longer recognized")
	}
	if appOwnedScrcpyProcess(os.Getpid(), "adb --window-title=Demo_Phone") {
		t.Fatal("non-mirror process matched")
	}
}
