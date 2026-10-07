package main

import (
	core "adm/internal/app"
	"fyne.io/fyne/v2/test"
	"testing"
)

func TestPackageListRejectsStaleAndReturningTargetResponses(t *testing.T) {
	var binding packageListBinding
	first := binding.begin("one")
	binding.invalidate()
	binding.begin("two")
	binding.invalidate()
	returning := binding.begin("one")
	if binding.accept("one", first) {
		t.Fatal("accepted old response after switching away and back")
	}
	if binding.ready("one") {
		t.Fatal("unloaded list allowed uninstall")
	}
	if !binding.accept("one", returning) || !binding.ready("one") {
		t.Fatal("current response rejected")
	}
	if binding.ready("two") {
		t.Fatal("list allowed uninstall on another device")
	}
	newest := binding.begin("one")
	if binding.accept("one", returning) || binding.ready("one") {
		t.Fatal("superseded request remained valid")
	}
	if !binding.accept("one", newest) {
		t.Fatal("new response rejected")
	}
}

func TestMainTargetChangeClearsPackagePicker(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	g := &GUIApp{packagePicker: newPackagePicker()}
	generation := g.packageListBinding.begin("serial-one")
	g.packageListBinding.accept("serial-one", generation)
	g.packagePicker.setPackages([]string{"com.example.one"})
	g.onTargetChanged("key-one", "key-two")
	if g.packagePicker.selectedPackage() != "" || len(g.packagePicker.all) != 0 || g.packageListBinding.ready("serial-one") {
		t.Fatal("old device package list remained actionable")
	}
	if g.packageListBinding.accept("serial-one", generation) {
		t.Fatal("late result revived old device list")
	}
	// Reconnecting the same AVD key with a different serial also invalidates it.
	generation = g.packageListBinding.begin("old-serial")
	g.packageListBinding.accept("old-serial", generation)
	g.packagePicker.setPackages([]string{"com.example.old"})
	g.entries = []core.DeviceEntry{{Key: "same-avd", Active: &core.ActiveDevice{Serial: "new-serial", State: "device"}}}
	g.onTargetChanged("same-avd", "same-avd")
	if g.packagePicker.selectedPackage() != "" {
		t.Fatal("reconnected device retained old list")
	}
}
