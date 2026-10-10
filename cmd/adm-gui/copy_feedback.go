package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// A plain content layer leaves canvas focus and pointer input with the page.
// Popups and dialogs would intercept input for this routine confirmation.
func (g *GUIApp) withCopyFeedback(page fyne.CanvasObject) fyne.CanvasObject {
	g.copyFeedbackLabel = widget.NewLabel("")
	content := container.NewPadded(container.NewHBox(widget.NewIcon(theme.ConfirmIcon()), g.copyFeedbackLabel))
	g.copyFeedback = container.New(copyFeedbackLayout{}, topSurface(content))
	g.copyFeedback.Hide()
	return container.NewStack(page, g.copyFeedback)
}

func (g *GUIApp) showCopyFeedback(message string) {
	if g.copyFeedback == nil {
		return
	}
	g.copyFeedbackGeneration++
	generation := g.copyFeedbackGeneration
	g.copyFeedbackLabel.SetText(message)
	g.copyFeedback.Show()
	g.copyFeedback.Refresh()
	time.AfterFunc(2200*time.Millisecond, func() {
		fyne.Do(func() { g.dismissCopyFeedback(generation) })
	})
}

func (g *GUIApp) dismissCopyFeedback(generation uint64) {
	if !g.closed && g.copyFeedback != nil && generation == g.copyFeedbackGeneration {
		g.copyFeedback.Hide()
	}
}

type copyFeedbackLayout struct{}

func (copyFeedbackLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }
func (copyFeedbackLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		min := object.MinSize()
		object.Resize(min)
		object.Move(fyne.NewPos(max(0, (size.Width-min.Width)/2), max(0, size.Height-min.Height-56)))
	}
}
