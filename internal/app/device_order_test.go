package app

import "testing"

func TestDeviceOrderPutsAllPhysicalDevicesBeforeEmulators(t *testing.T) {
	entries := []DeviceEntry{
		{Label: "A emulator", Running: true, Active: &ActiveDevice{IsEmulator: true}},
		{Label: "Z stopped", AVD: &AVD{Name: "Demo_AVD"}},
		{Label: "B phone", Active: &ActiveDevice{State: "offline"}},
		{Label: "Z phone", Running: true, Active: &ActiveDevice{State: "device"}},
		{Label: "A phone", Running: true, Active: &ActiveDevice{State: "device"}},
	}
	sortDeviceEntries(entries)
	for i, want := range []string{"A phone", "Z phone", "B phone", "A emulator", "Z stopped"} {
		if entries[i].Label != want {
			t.Fatalf("position %d: got %s, want %s", i, entries[i].Label, want)
		}
	}
}
