package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestLibraryRowsRetainActionsAndStateWithoutNarrowOverflow(t *testing.T) {
	objects := make([]fyne.CanvasObject, 5)
	for i := range objects {
		r := canvas.NewRectangle(nil)
		r.SetMinSize(fyne.NewSize(24, 28))
		objects[i] = r
	}
	objects[4].(*canvas.Rectangle).SetMinSize(fyne.NewSize(128, 36))
	for _, width := range []float32{1000, 680, 520, 480, 479, 360, 320, 680} {
		libraryRowLayout{}.Layout(objects, fyne.NewSize(width, 64))
		for i, object := range objects {
			if !object.Visible() {
				continue
			}
			if object.Position().X < 0 || object.Position().X+object.Size().Width > width || object.Position().Y < 0 || object.Position().Y+object.Size().Height > 64 {
				t.Fatalf("column %d overflows width %.0f: %v %v", i, width, object.Position(), object.Size())
			}
		}
		for _, i := range []int{0, 1, 3, 4} {
			if !objects[i].Visible() {
				t.Fatalf("essential column %d hidden at %.0f", i, width)
			}
		}
		if objects[2].Visible() != (width >= 680) {
			t.Fatal("type column breakpoint is inconsistent")
		}
		if width < 480 {
			if objects[3].Position().X != objects[4].Position().X || objects[3].Position().Y < objects[4].Position().Y+objects[4].Size().Height {
				t.Fatal("narrow state overlaps actions")
			}
		} else if objects[3].Position().X+objects[3].Size().Width > objects[4].Position().X {
			t.Fatal("state and action columns overlap")
		}
	}
}

func TestNarrowLibraryPreservesActualStateAndButtonTextHeight(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(admTheme{base: theme.DefaultTheme()})
	state := widget.NewLabel("未启动")
	actions := container.NewHBox(compactButton("启动", nil), compactButton("管理", nil))
	objects := []fyne.CanvasObject{widget.NewCheck("", nil), widget.NewLabel("demo_5050"), widget.NewLabel("模拟器"), state, actions}
	l := libraryRowLayout{}
	size := l.MinSize(objects)
	l.Layout(objects, size)
	if state.Size().Height < state.MinSize().Height || actions.Size().Height < actions.MinSize().Height {
		t.Fatal("narrow library clips state text or action buttons")
	}
	if state.Position().Y < actions.Position().Y+actions.Size().Height {
		t.Fatal("state overlays action buttons")
	}
	for _, object := range objects {
		if object.Visible() && object.Position().X+object.Size().Width > size.Width {
			t.Fatal("actual library controls exceed minimum width")
		}
	}
}
