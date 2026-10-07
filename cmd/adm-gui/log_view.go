package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Remain enabled for normal text selection and contrast, while rejecting all
// user edits. Updates are paused during selection so incoming logs cannot
// replace the text the user is copying.
type selectableLog struct {
	widget.Entry
	onFocus   func()
	menuTitle string
}

func newSelectableLog() *selectableLog {
	l := &selectableLog{menuTitle: "日志"}
	l.MultiLine = true
	l.Wrapping = fyne.TextWrapOff
	l.Scroll = fyne.ScrollNone
	l.TextStyle = fyne.TextStyle{Monospace: true}
	l.ExtendBaseWidget(l)
	return l
}
func (l *selectableLog) AcceptsTab() bool { return false }
func (l *selectableLog) TypedRune(rune)   {}
func (l *selectableLog) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeyLeft, fyne.KeyRight, fyne.KeyUp, fyne.KeyDown, fyne.KeyHome, fyne.KeyEnd, fyne.KeyPageUp, fyne.KeyPageDown:
		l.Entry.TypedKey(event)
	}
}
func (l *selectableLog) TypedShortcut(shortcut fyne.Shortcut) {
	switch shortcut.(type) {
	case *fyne.ShortcutCopy, *fyne.ShortcutSelectAll:
		l.Entry.TypedShortcut(shortcut)
	}
}
func (l *selectableLog) TappedSecondary(event *fyne.PointEvent) {
	canvas := fyne.CurrentApp().Driver().CanvasForObject(l)
	if canvas == nil {
		return
	}
	menu := fyne.NewMenu(l.menuTitle, fyne.NewMenuItem("复制选中", func() { l.TypedShortcut(&fyne.ShortcutCopy{Clipboard: fyne.CurrentApp().Clipboard()}) }), fyne.NewMenuItem("全选", func() { l.TypedShortcut(&fyne.ShortcutSelectAll{}) }))
	widget.ShowPopUpMenuAtPosition(menu, canvas, event.AbsolutePosition)
}

func (l *selectableLog) FocusGained() {
	l.Entry.FocusGained()
	if l.onFocus != nil {
		l.onFocus()
	}
}
