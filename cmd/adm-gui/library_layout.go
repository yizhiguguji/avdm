package main

import "fyne.io/fyne/v2"

// Library rows keep their checkbox, identity, state and actions at narrow
// widths. Type is secondary; the narrowest view stacks state below actions.
type libraryRowLayout struct{}

func (libraryRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := float32(64)
	if len(objects) == 5 {
		height = max(height, objects[3].MinSize().Height+objects[4].MinSize().Height)
	}
	return fyne.NewSize(320, height)
}
func (libraryRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 5 {
		return
	}
	const inset float32 = 8
	const gap float32 = 4
	width := max(0, size.Width-2*inset)
	checkWidth := min(28, width)
	actionWidth := min(128, max(0, width-checkWidth-gap-60))
	narrow := size.Width < 480
	typeWidth := float32(0)
	if size.Width >= 680 {
		typeWidth = 92
	}
	stateWidth := float32(72)
	if narrow {
		stateWidth = 0
	}
	columns := 3
	if typeWidth > 0 {
		columns++
	}
	if stateWidth > 0 {
		columns++
	}
	identityWidth := max(0, width-checkWidth-actionWidth-typeWidth-stateWidth-float32(columns-1)*gap)
	widths := []float32{checkWidth, identityWidth, typeWidth, stateWidth, actionWidth}
	x := inset
	actionX := inset + width - actionWidth
	for i, object := range objects {
		if i == 3 && narrow {
			continue
		}
		if widths[i] == 0 {
			object.Hide()
			continue
		}
		object.Show()
		h := min(size.Height, object.MinSize().Height)
		y := (size.Height - h) / 2
		if i == 4 && narrow {
			h = max(0, size.Height-min(size.Height, objects[3].MinSize().Height))
			y = 0
		}
		object.Move(fyne.NewPos(x, y))
		object.Resize(fyne.NewSize(widths[i], h))
		x += widths[i] + gap
	}
	if narrow {
		state := objects[3]
		state.Show()
		y := max(0, size.Height-min(size.Height, state.MinSize().Height))
		state.Move(fyne.NewPos(actionX, y))
		state.Resize(fyne.NewSize(actionWidth, max(0, size.Height-y)))
	}
}
