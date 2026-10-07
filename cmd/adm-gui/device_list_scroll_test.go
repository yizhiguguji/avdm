package main

import (
	core "adm/internal/app"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestStoppedListActionsClearExpandedScrollbar(t *testing.T) {
	g := testWall(t)
	entries := []core.DeviceEntry{
		{Key: "stopped", Label: "stopped", AVD: &core.AVD{Name: "stopped"}},
		{Key: "waiting", Label: "waiting", AVD: &core.AVD{Name: "waiting"}, Running: true},
		{Key: "offline", Label: "offline", AVD: &core.AVD{Name: "offline"}, Running: true, Active: &core.ActiveDevice{State: "offline", IsEmulator: true}},
	}
	for _, width := range []float32{244, 360} {
		rows := container.NewVBox()
		for _, entry := range entries {
			rows.Add(g.buildStoppedDeviceCard(entry))
		}
		scroll := deviceListScroll(rows)
		scroll.Resize(fyne.NewSize(width, 120))
		testRight := width - theme.ScrollBarSize() - theme.Padding()/2
		buttons := 0
		var check func(fyne.CanvasObject, float32)
		check = func(object fyne.CanvasObject, x float32) {
			x += object.Position().X
			if button, ok := object.(*widget.Button); ok {
				buttons++
				if x+button.Size().Width > testRight+0.5 || button.Size().Width < button.MinSize().Width {
					t.Fatalf("%s clipped at width %v: x=%v size=%v right=%v", button.Text, width, x, button.Size(), testRight)
				}
			}
			if c, ok := object.(*fyne.Container); ok {
				for _, child := range c.Objects {
					check(child, x)
				}
			}
		}
		check(scroll.Content, 0)
		if buttons != 8 {
			t.Fatalf("expected eight list actions, got %d", buttons)
		}
	}
}
