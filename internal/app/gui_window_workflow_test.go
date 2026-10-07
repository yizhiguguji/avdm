package app

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNormalMirrorOpensReadyOfflineAndStartingEmulatorWindows(t *testing.T) {
	for _, entry := range []DeviceEntry{
		{Label: "ready", Active: &ActiveDevice{Serial: "emulator-fake", State: "device", IsEmulator: true, AVDName: "test-avd"}},
		{Label: "offline", AVD: &AVD{Name: "test-avd"}, Running: true, Active: &ActiveDevice{Serial: "emulator-fake", State: "offline", IsEmulator: true}},
		{Label: "starting", AVD: &AVD{Name: "test-avd"}, Running: true},
	} {
		t.Run(entry.Label, func(t *testing.T) {
			a := &App{}
			calls := 0
			err := a.openLiveMirrorEntry(entry, nil, func(name, serial string) error {
				calls++
				if name != "test-avd" {
					t.Fatalf("lost AVD identity: %q", name)
				}
				if entry.Active != nil && serial != entry.Active.Serial {
					t.Fatalf("wrong serial %q", serial)
				}
				return nil
			})
			if err != nil || calls != 1 {
				t.Fatalf("normal mirror did not reach native focus: calls=%d err=%v", calls, err)
			}
			enabled := true
			err = a.openLiveMirrorEntry(entry, &enabled, func(string, string) error { t.Fatal("explicit phone-only top setting reached emulator"); return nil })
			if err == nil || !strings.Contains(err.Error(), "置顶") {
				t.Fatalf("explicit top guard lost: %v", err)
			}
		})
	}
}

func TestPartialMirrorFailureStillArrangesOnlySuccessfulWindows(t *testing.T) {
	openErr := errors.New("scrcpy failed")
	permission := &AccessibilityPermissionRequiredError{Operation: "平铺外部窗口"}
	for _, tileErr := range []error{nil, permission} {
		var opened, tiled []string
		err := openLiveMirrors([]string{"one", "bad", "one", "two"}, func(key string) error {
			opened = append(opened, key)
			if key == "bad" {
				return openErr
			}
			return nil
		}, func(keys []string) error { tiled = append([]string(nil), keys...); return tileErr })
		if !reflect.DeepEqual(opened, []string{"one", "bad", "two"}) || !reflect.DeepEqual(tiled, []string{"one", "two"}) {
			t.Fatalf("opened=%v tiled=%v", opened, tiled)
		}
		if !errors.Is(err, openErr) {
			t.Fatalf("lost open error: %v", err)
		}
		if tileErr != nil && !IsAccessibilityPermissionRequired(err) {
			t.Fatalf("lost permission recovery: %v", err)
		}
	}
}

func TestMirrorAllFailuresNeverArrangeUnopenedWindows(t *testing.T) {
	failure := errors.New("failed")
	err := openLiveMirrors([]string{"bad"}, func(string) error { return failure }, func([]string) error { t.Fatal("arranged unopened targets"); return nil })
	if !errors.Is(err, failure) {
		t.Fatalf("lost failure: %v", err)
	}
}

func TestNonReadyEmulatorCloseUsesAVDRecoveryWithoutDeletion(t *testing.T) {
	for _, entry := range []DeviceEntry{
		{Label: "starting", AVD: &AVD{Name: "test-close-avd"}, Running: true},
		{Label: "offline", AVD: &AVD{Name: "test-close-avd"}, Running: true, Active: &ActiveDevice{Serial: "emulator-fake", State: "offline", IsEmulator: true}},
	} {
		if err := validateEmulatorCloseEntry(entry); err != nil {
			t.Fatal(err)
		}
		calls := 0
		err := closeEmulatorEntry(entry, func(name string) error {
			calls++
			if name != "test-close-avd" {
				t.Fatalf("wrong AVD: %s", name)
			}
			return nil
		}, func(string) error { t.Fatal("non-ready AVD used serial-only close"); return nil })
		if err != nil || calls != 1 {
			t.Fatalf("calls=%d err=%v", calls, err)
		}
	}
}

func TestBatchAndUnconfirmedCloseKeepPhysicalDeviceBoundary(t *testing.T) {
	for _, serial := range []string{"phone", "192.0.2.1:5555"} {
		entry := DeviceEntry{Label: serial, Active: &ActiveDevice{Serial: serial, State: "device"}}
		if err := validateEmulatorCloseEntry(entry); err == nil {
			t.Fatalf("batch accepted physical target %s", serial)
		}
		if serial == "phone" {
			err := closeEmulatorEntry(entry, func(string) error { t.Fatal("physical close reached AVD"); return nil }, func(string) error { t.Fatal("physical shutdown bypassed serial confirmation"); return nil })
			if err == nil {
				t.Fatal("unconfirmed physical shutdown accepted")
			}
		}
	}
	if err := validateEmulatorCloseEntry(DeviceEntry{AVD: &AVD{Name: "stopped"}}); err == nil {
		t.Fatal("batch accepted stopped AVD")
	}
}

func TestPublicCloseAPIsCloseOfflineEmulatorWithoutDeletingAVD(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[batch], func(t *testing.T) {
			a, log := fakeInstallADB(t, `
closed="$(dirname "$0")/closed"
if [ "$1" = devices ]; then
 echo 'List of devices attached'
 if [ ! -f "$closed" ]; then echo 'emulator-65520 offline'; fi
 exit 0
fi
if [ "$1" = list ]; then exit 0; fi
if [ "$3" = emu ] && [ "$4" = kill ]; then touch "$closed"; echo OK; exit 0; fi
echo 'unexpected command' >&2
exit 1
`)
			a.cfg = defaultConfig()
			// Resolve AVD inventory through the same fake tool; no real SDK is used.
			a.tools.statuses[toolAVDManager] = a.tools.statuses[toolADB]
			key := aliasKeyForSerial("emulator-65520")
			var err error
			if batch {
				err = a.GUICloseDevices([]string{key})
			} else {
				err = a.GUICloseDevice(key)
			}
			if err != nil {
				t.Fatal(err)
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(calls), "-s emulator-65520 emu kill") {
				t.Fatalf("offline close did not execute: %s", calls)
			}
			if strings.Contains(string(calls), "delete") || strings.Contains(string(calls), "reboot -p") {
				t.Fatalf("unsafe recovery path: %s", calls)
			}
		})
	}
}

func TestPublicBatchCloseRejectsMixedPhysicalSelectionBeforeMutating(t *testing.T) {
	a, log := fakeInstallADB(t, `
if [ "$1" = devices ]; then printf 'List of devices attached\nemulator-65520 offline\nphysical-test device\n'; exit 0; fi
if [ "$1" = list ]; then exit 0; fi
echo unexpected >&2
exit 1
`)
	a.cfg = defaultConfig()
	a.tools.statuses[toolAVDManager] = a.tools.statuses[toolADB]
	err := a.GUICloseDevices([]string{aliasKeyForSerial("emulator-65520"), aliasKeyForSerial("physical-test")})
	if err == nil || !strings.Contains(err.Error(), "只支持模拟器") {
		t.Fatalf("physical target allowed: %v", err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "kill") || strings.Contains(string(calls), "delete") || strings.Contains(string(calls), "reboot") {
		t.Fatalf("mutation before target validation: %s", calls)
	}
}
