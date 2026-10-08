package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Navigation actions use regular text: dense CJK bold faces are difficult to
// read at toolbar sizes. Keep Button's focus, keyboard and loading behaviour.
type readableButton struct{ widget.Button }

func newReadableButton(label string, tapped func()) *readableButton {
	b := &readableButton{Button: widget.Button{Text: label, OnTapped: tapped}}
	b.ExtendBaseWidget(b)
	b.Importance = widget.LowImportance
	return b
}

func (b *readableButton) CreateRenderer() fyne.WidgetRenderer {
	r := b.Button.CreateRenderer()
	for _, object := range r.Objects() {
		if text, ok := object.(*widget.RichText); ok {
			for _, segment := range text.Segments {
				if segment, ok := segment.(*widget.TextSegment); ok {
					segment.Style.TextStyle.Bold = false
				}
			}
			text.Refresh()
		}
	}
	return r
}
