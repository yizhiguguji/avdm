package main

import (
	core "adm/internal/app"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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
