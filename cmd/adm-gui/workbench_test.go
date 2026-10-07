package main

import (
	core "adm/internal/app"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestWallSelectionSurvivesRefreshAndRemovesMissingDevices(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	g := &GUIApp{
		entries:         []core.DeviceEntry{{Key: "one", Label: "One"}, {Key: "two", Label: "Two"}},
		controlSelected: map[string]bool{"one": true, "two": true, "gone": true},
		selectedLabel:   widget.NewLabel(""),
	}
	g.restoreSelection()
	if len(g.selectedControlEntries()) != 2 || g.controlSelected["gone"] || g.selectedLabel.Text != "选中：2 台" {
		t.Fatalf("unexpected refreshed selection: %v, %s", g.controlSelected, g.selectedLabel.Text)
	}
	g.entries = g.entries[:1]
	g.restoreSelection()
	if !g.controlSelected["one"] || g.controlSelected["two"] || g.selectedLabel.Text != "选中：One" {
		t.Fatalf("unexpected single selection: %v, %s", g.controlSelected, g.selectedLabel.Text)
	}
}

func TestWorkbenchWallUsesSpaceWithoutLeftPanel(t *testing.T) {
	wall := &canvas.Rectangle{}
	handle := &canvas.Rectangle{}
	panel := &canvas.Rectangle{}
	g := &GUIApp{rightW: 380}
	l := horizontalDockLayout{g: g}
	objects := []fyne.CanvasObject{wall, handle, panel}
	l.Layout(objects, fyne.NewSize(1200, 800))
	if wall.Position().X != 0 || wall.Size().Width != 1200 {
		t.Fatalf("wall should fill available width: %v %v", wall.Position(), wall.Size())
	}
	g.rightActive = "install"
	l.Layout(objects, fyne.NewSize(1200, 800))
	if wall.Position().X != 0 || wall.Size().Width != 814 || panel.Position().X+panel.Size().Width != 1200 {
		t.Fatalf("tool panel should share width with wall: wall=%v panel=%v", wall.Size(), panel.Position())
	}
}

func TestDeviceRailOpensAndSwitchesToolsInOneClick(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	path := widget.NewEntry()
	path.SetText("/tmp/draft.apk")
	draft := widget.NewEntry()
	draft.SetText("保留输入草稿")
	g := &GUIApp{app: a, logCollapsed: true, rightHandle: newResizeHandle(true, nil), logHandle: newResizeHandle(false, nil),
		installPanel: container.NewVBox(path), uninstallPanel: container.NewVBox(), messagePanel: container.NewVBox(draft),
		rightPanel: container.NewVBox(), logPanel: container.NewVBox(), logRail: container.NewVBox()}
	rail := g.buildRightIconBar()
	g.applyWorkbenchCollapseState()
	test.Tap(g.installIcon)
	if g.rightActive != "install" || !g.installPanel.Visible() || g.uninstallPanel.Visible() || g.messagePanel.Visible() || !g.rightPanel.Visible() {
		t.Fatal("install must open directly with one click")
	}
	test.Tap(g.messageIcon)
	if g.rightActive != "message" || !g.messagePanel.Visible() || g.installPanel.Visible() || path.Text != "/tmp/draft.apk" {
		t.Fatal("switching tools must preserve the APK draft")
	}
	test.Tap(g.uninstallIcon)
	if g.rightActive != "uninstall" || !g.uninstallPanel.Visible() || draft.Text != "保留输入草稿" {
		t.Fatal("switching tools must preserve the text draft")
	}
	test.Tap(g.uninstallIcon)
	if g.rightActive != "" || g.rightPanel.Visible() || !rail.Visible() {
		t.Fatal("second click must collapse the panel while keeping the rail visible")
	}
	test.Tap(g.logToolIcon)
	if g.logCollapsed || !g.logPanel.Visible() || g.logRail.Visible() {
		t.Fatal("log must open directly from the rail")
	}
	test.Tap(g.logToolIcon)
	if !g.logCollapsed || !g.logRail.Visible() {
		t.Fatal("second log click must restore status row")
	}
}
