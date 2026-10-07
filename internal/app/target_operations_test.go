package app

import (
	"os"
	"strings"
	"sync"
	"testing"
)

func TestExplicitDeviceOperationsIgnoreCurrentChanges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, log := fakeInstallADB(t, `
if [ "$1" = devices ]; then
 printf 'List of devices attached\none device model:One\ntwo device model:Two\n'
 exit 0
fi
if [ "$3" = shell ] && [ "$4" = pm ]; then echo 'package:com.example.one'; exit 0; fi
echo Success
`)
	a.cfg = defaultConfig()
	a.setCurrentDevice(&ActiveDevice{Serial: "two", State: "device"})
	packages, err := a.GUIListPackagesForDevice("one", false, "")
	if err != nil || len(packages) != 1 {
		t.Fatalf("packages=%v err=%v", packages, err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			a.setCurrentDevice(&ActiveDevice{Serial: "two", State: "device"})
		}
	}()
	if err := a.GUIUninstallPackageForDevice("one", "com.example.one", false, false); err != nil {
		t.Fatal(err)
	}
	if err := a.GUISendTextForDevice("one", "hello", false); err != nil {
		t.Fatal(err)
	}
	if err := a.GUIRebootDevice("one"); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"-s one shell pm list packages", "-s one uninstall com.example.one", "-s one shell input text hello", "-s one reboot"} {
		if !strings.Contains(string(calls), expected) {
			t.Errorf("missing %q in %s", expected, calls)
		}
	}
	if strings.Contains(string(calls), "-s two") {
		t.Fatalf("operation leaked to current device: %s", calls)
	}
	if cur := a.currentDeviceSnapshot(); cur == nil || cur.Serial != "two" {
		t.Fatalf("explicit operation changed current: %v", cur)
	}
}

func TestCloseSerialConfirmationPrecedesCommands(t *testing.T) {
	a, log := fakeInstallADB(t, "echo Success\n")
	if err := a.GUICloseDeviceConfirmed("physical", "wrong"); err == nil {
		t.Fatal("accepted mismatched serial")
	}
	if data, err := os.ReadFile(log); err == nil && len(data) > 0 {
		t.Fatalf("ran commands before confirmation: %s", data)
	}
}

func TestDeviceTaskSerializationAndIndependentTargets(t *testing.T) {
	first := lockTargetTask("serial:test-first")
	defer func() {
		if first != nil {
			first()
		}
	}()
	blocked := make(chan struct{})
	started := make(chan struct{})
	go func() { close(started); release := lockTargetTask("serial:test-first"); close(blocked); release() }()
	<-started
	independent := lockTargetTask("serial:test-second")
	independent()
	select {
	case <-blocked:
		t.Fatal("same-target task acquired lock before release")
	default:
	}
	first()
	first = nil
	<-blocked
}

func TestConcurrentUninstallsOnOneDeviceDoNotOverlap(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, log := fakeInstallADB(t, `
if [ "$1" = devices ]; then printf 'List of devices attached\none device\n'; exit 0; fi
if [ "$3" = uninstall ]; then
 guard="$(dirname "$0")/uninstall-active"
 if ! mkdir "$guard"; then echo overlap; exit 1; fi
 sleep 0.03
 rmdir "$guard"
fi
echo Success
`)
	a.cfg = defaultConfig()
	var wg sync.WaitGroup
	errors := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- a.GUIUninstallPackageForDevice("one", "com.example.one", false, false)
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(calls), "-s one uninstall com.example.one"); count != 4 {
		t.Fatalf("executed %d uninstalls", count)
	}
}
