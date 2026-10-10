package main

import (
	core "adm/internal/app"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

func TestRailPowerDialogsUseAllSelectedCardNames(t *testing.T) {
	for _, reboot := range []bool{true, false} {
		g := testWall(t)
		g.window = g.app.NewWindow("设备")
		g.window.SetContent(g.controlGrid)
		g.window.Show()
		t.Cleanup(g.window.Close)
		g.entries = []core.DeviceEntry{
			{Key: "one", Active: &core.ActiveDevice{Serial: "emulator-5554", IsEmulator: true, State: "device", Details: map[string]string{"model": "第一台"}}},
			{Key: "two", AVD: &core.AVD{Name: "Demo_AVD"}, Active: &core.ActiveDevice{Serial: "emulator-5556", IsEmulator: true, State: "device"}},
			{Key: "main", Active: &core.ActiveDevice{Serial: "PHONE", State: "device", Details: map[string]string{"model": "其他主目标"}}},
		}
		g.currentDeviceKey = "main"
		g.controlSelected = map[string]bool{"one": true, "two": true}
		g.wallSearch = "第一台" // A hidden selected target must remain in scope.
		if reboot {
			g.rebootCurrentDevice()
		} else {
			g.closeCurrentDevice()
		}
		var messages []string
		var walk func(fyne.CanvasObject)
		walk = func(obj fyne.CanvasObject) {
			if text, ok := obj.(*selectableLog); ok {
				messages = append(messages, text.Text)
			}
			if c, ok := obj.(*fyne.Container); ok {
				for _, child := range c.Objects {
					walk(child)
				}
			}
		}
		for _, overlay := range g.window.Canvas().Overlays().List() {
			walk(overlay)
		}
		// The popup renderer contains the custom dialog content.
		if len(messages) == 0 {
			for _, overlay := range g.window.Canvas().Overlays().List() {
				if widget, ok := overlay.(fyne.Widget); ok {
					for _, obj := range widget.CreateRenderer().Objects() {
						walk(obj)
					}
				}
			}
		}
		message := strings.Join(messages, "\n")
		for _, want := range []string{"2 台设备", "第一台", "Demo_AVD", "emulator-5554", "emulator-5556"} {
			if !strings.Contains(message, want) {
				t.Fatalf("reboot=%v: confirmation missing %q: %s", reboot, want, message)
			}
		}
		if strings.Contains(message, "其他主目标") {
			t.Fatal("selected action used main target")
		}
	}
}

func TestPowerTargetsSnapshotAndMainFallback(t *testing.T) {
	g := testWall(t)
	g.entries = []core.DeviceEntry{{Key: "one", Active: &core.ActiveDevice{Serial: "PHONE", State: "device", Details: map[string]string{"model": "其他主目标"}}}}
	g.currentDeviceKey = "one"
	targets := g.powerTargets()
	g.entries[0].Active.Serial = "CHANGED"
	if len(targets) != 1 || targets[0].Active.Serial != "PHONE" {
		t.Fatal("confirmation target changed after refresh")
	}
	g.controlSelected = map[string]bool{"missing": true}
	if len(g.powerTargets()) != 0 {
		t.Fatal("missing explicit selection fell back to main target")
	}
	g.controlSelected = nil
	g.currentDeviceKey = "missing"
	if len(g.powerTargets()) != 0 {
		t.Fatal("missing target widened scope")
	}
}

func TestMixedClosePreservesPhysicalConfirmationAndDispatchesEveryTarget(t *testing.T) {
	entries := []core.DeviceEntry{
		{Key: "phone", Active: &core.ActiveDevice{Serial: "PHONE", State: "device", Details: map[string]string{"model": "其他主目标"}}},
		{Key: "network", Active: &core.ActiveDevice{Serial: "192.0.2.1:5555", State: "device"}},
		{Key: "emu", Running: true, AVD: &core.AVD{Name: "Demo_AVD"}},
	}
	if validatePowerConfirmations(entries, nil) == nil {
		t.Fatal("phone shutdown needs exact serial")
	}
	confirmations := map[string]string{"phone": "PHONE"}
	if err := validatePowerConfirmations(entries, confirmations); err != nil {
		t.Fatal(err)
	}
	var calls []string
	results := executeEntryBatch(entries, func(entry core.DeviceEntry) error {
		// executeEntryBatch is concurrent; individual dispatch is checked below.
		return closePowerTarget(entry, confirmations[entry.Key], func(string) error { return nil }, func(string, string) error { return nil })
	})
	for _, result := range results {
		if result.Err != nil {
			t.Fatal(result.Err)
		}
	}
	for _, entry := range entries {
		err := closePowerTarget(entry, confirmations[entry.Key], func(name string) error { calls = append(calls, "avd:"+name); return nil }, func(serial, confirmation string) error {
			if serial != confirmation {
				t.Fatal("wrong serial confirmation")
			}
			calls = append(calls, serial)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if strings.Join(calls, ",") != "PHONE,192.0.2.1:5555,avd:Demo_AVD" {
		t.Fatalf("wrong targets: %v", calls)
	}
	if err := closePowerTarget(entries[0], "wrong", func(string) error { t.Fatal("unexpected close"); return nil }, func(string, string) error { t.Fatal("unconfirmed shutdown"); return nil }); err == nil {
		t.Fatal("unconfirmed phone accepted")
	}
}
