package main

import (
	core "adm/internal/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"strings"
	"testing"
)

func TestFilteredSelectionRemainsExplicitAndReportsHiddenTargets(t *testing.T) {
	g := testWall(t)
	g.entries = []core.DeviceEntry{testWallEntry("one", "serial-one"), testWallEntry("two", "serial-two")}
	g.controlSelected = map[string]bool{"one": true, "two": true}
	g.installScopeLabel = widget.NewLabel("")
	g.selectionActions = container.NewHBox()
	g.currentDevice = "primary"
	g.wallSearch = "one"
	g.updateSelectedLabel()
	if g.selectedLabel.Text != "选中：2 台（隐藏 1）" || !strings.Contains(g.installScopeLabel.Text, "含筛选隐藏 1 台") {
		t.Fatalf("hidden selection omitted: %s / %s", g.selectedLabel.Text, g.installScopeLabel.Text)
	}
	plan := g.planSelectedAction(actionInstall, false)
	if len(plan.Entries) != 2 || !plan.Explicit || !g.selectionActions.Visible() {
		t.Fatal("filter changed action scope or hid direct batch actions")
	}
	g.clearWorkbenchSelection()
	if g.selectionActions.Visible() || len(g.selectedControlEntries()) != 0 || g.installScopeLabel.Text != "安装范围：主目标 · primary" {
		t.Fatal("clear did not restore primary scope and hide batch actions")
	}
}

func TestToolTargetMatchesActivePanelAcrossSelectionAndPanelChanges(t *testing.T) {
	g := testWall(t)
	g.rightHandle = newResizeHandle(true, nil)
	g.logHandle = newResizeHandle(false, nil)
	g.entries = []core.DeviceEntry{testWallEntry("one", "serial-one"), testWallEntry("two", "serial-two")}
	g.controlSelected = map[string]bool{"one": true, "two": true}
	g.currentDevice = "primary"
	g.toolTargetLabel = widget.NewLabel("")
	g.wallSearch = "one"
	for _, panel := range []string{"install", "uninstall", "message", "install"} {
		g.toggleRightPanel(panel)
		want := "作用范围：主目标 · primary"
		if panel == "install" {
			want = "作用范围：已勾选 2 台（含隐藏 1 台）"
		}
		if g.toolTargetLabel.Text != want {
			t.Fatalf("panel %s reports wrong target: %s", panel, g.toolTargetLabel.Text)
		}
	}
	g.clearWorkbenchSelection()
	if g.toolTargetLabel.Text != "作用范围：主目标 · primary" {
		t.Fatalf("clearing selection lost primary target: %s", g.toolTargetLabel.Text)
	}
	g.currentDevice = ""
	g.updateSelectedLabel()
	if g.toolTargetLabel.Text != "作用范围：主目标 · 未选择" {
		t.Fatalf("missing primary target reported as available: %s", g.toolTargetLabel.Text)
	}
}
