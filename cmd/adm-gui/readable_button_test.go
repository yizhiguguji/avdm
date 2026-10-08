package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestNavigationButtonKeepsRegularTextAfterStateChanges(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(admTheme{base: theme.DefaultTheme()})
	b := newWorkbenchRailButton("安装", theme.DownloadIcon(), nil)
	r := test.WidgetRenderer(b)
	b.SetText("日志")
	setWorkbenchToolActive(b, true)
	b.Disable()
	b.Enable()
	found := false
	for _, object := range r.Objects() {
		if text, ok := object.(*widget.RichText); ok {
			segment := text.Segments[0].(*widget.TextSegment)
			found = true
			if segment.Style.TextStyle.Bold || segment.Text != "日志" {
				t.Fatalf("navigation text lost regular weight or label: %+v", segment)
			}
		}
	}
	if !found {
		t.Fatal("navigation label was not rendered")
	}
	if b.MinSize().Width > dockIconBarWidth {
		t.Fatalf("rail clips its label and icon: %v", b.MinSize())
	}
}

func TestToolbarReservesActualButtonTextAndIconWidth(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(admTheme{base: theme.DefaultTheme()})
	objects := make([]fyne.CanvasObject, 8)
	for i := range objects {
		b := newReadableButton("刷新画面", nil)
		b.SetIcon(theme.ViewRefreshIcon())
		objects[i] = b
	}
	workbenchToolbarLayout{}.Layout(objects, fyne.NewSize(1200, 36))
	for _, object := range objects {
		if object.Visible() && object.Size().Width < object.MinSize().Width {
			t.Fatalf("toolbar clips action: size=%v min=%v", object.Size(), object.MinSize())
		}
	}
}
