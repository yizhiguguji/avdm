package main

import (
	core "adm/internal/app"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func inspectManagementObjects(g *GUIApp, visit func(fyne.CanvasObject)) {
	seen := map[fyne.CanvasObject]bool{}
	var walk func(fyne.CanvasObject)
	walk = func(object fyne.CanvasObject) {
		if object == nil || seen[object] {
			return
		}
		seen[object] = true
		visit(object)
		if group, ok := object.(*fyne.Container); ok {
			for _, child := range group.Objects {
				walk(child)
			}
		}
		if w, ok := object.(fyne.Widget); ok {
			for _, child := range test.WidgetRenderer(w).Objects() {
				walk(child)
			}
		}
	}
	for _, object := range g.window.Canvas().Overlays().List() {
		walk(object)
	}
}

func TestManagementDeleteTargetsOnlyThatAVD(t *testing.T) {
	for _, running := range []bool{false, true} {
		g := testWall(t)
		g.window = g.app.NewWindow("设备管理")
		g.window.SetContent(g.controlGrid)
		g.window.Show()
		t.Cleanup(g.window.Close)
		target := core.DeviceEntry{Key: "avd:Demo_AVD", Label: "Demo_AVD", AVD: &core.AVD{Name: "Demo_AVD"}, Running: running}
		g.entries = []core.DeviceEntry{target, {Key: "avd:Other_AVD", AVD: &core.AVD{Name: "Other_AVD"}}}
		g.controlSelected = map[string]bool{"avd:Other_AVD": true}
		g.showControlDeviceManageDialog(target)
		var remove *widget.Button
		inspectManagementObjects(g, func(object fyne.CanvasObject) {
			if button, ok := object.(*widget.Button); ok && button.Text == "删除此模拟器" {
				remove = button
			}
		})
		if remove == nil {
			t.Fatalf("running=%t: management lacks delete action", running)
		}
		remove.OnTapped()
		var labels []string
		var force *widget.Check
		inspectManagementObjects(g, func(object fyne.CanvasObject) {
			if label, ok := object.(*widget.Label); ok {
				labels = append(labels, label.Text)
			}
			if check, ok := object.(*widget.Check); ok && check.Text == "关闭正在运行的模拟器后删除" {
				force = check
			}
		})
		text := strings.Join(labels, "\n")
		if !strings.Contains(text, "Demo_AVD") || strings.Contains(text, "Other_AVD") {
			t.Fatalf("wrong deletion scope: %s", text)
		}
		if force == nil || force.Visible() != running {
			t.Fatalf("running=%t: wrong shutdown confirmation", running)
		}
	}
}
