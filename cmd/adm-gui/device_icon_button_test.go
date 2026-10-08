package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"testing"
)

func TestDeviceIconExplainsActionAndKeepsKeyboardActivation(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	calls := 0
	button := newDeviceIconButton("返回", iconDeviceBack, func() { calls++ })
	w := test.NewWindow(button)
	defer w.Close()
	button.Resize(fyne.NewSize(36, 36))
	button.MouseIn(&desktop.MouseEvent{})
	if len(w.Canvas().Overlays().List()) != 0 {
		t.Fatal("hover explanation captures canvas input")
	}
	if button.hint == nil || !button.hint.Visible() {
		t.Fatal("icon action has no hover explanation")
	}
	button.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if calls != 1 || button.hint.Visible() {
		t.Fatal("keyboard activation lost callback or left hover overlay")
	}
	button.MouseIn(&desktop.MouseEvent{})
	button.MouseOut()
	if button.hint.Visible() {
		t.Fatal("hover explanation remains after leaving icon")
	}
	button.Disable()
	button.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if calls != 1 {
		t.Fatal("disabled icon still executes")
	}
}

func TestDeviceIconHoverRemainsStableAcrossMouseMovement(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	button := newDeviceIconButton("返回", iconDeviceBack, nil)
	w := test.NewWindow(button)
	defer w.Close()
	w.Resize(fyne.NewSize(100, 100))
	test.MoveMouse(w.Canvas(), fyne.NewPos(10, 10))
	hint := button.hint
	for i := 0; i < 20; i++ {
		test.MoveMouse(w.Canvas(), fyne.NewPos(10+float32(i%4), 10))
		if button.hint != hint || !hint.Visible() {
			t.Fatal("moving within the icon toggles the hover explanation")
		}
		if len(w.Canvas().Overlays().List()) != 0 {
			t.Fatal("hover explanation takes over the input layer")
		}
	}
	test.MoveMouse(w.Canvas(), fyne.NewPos(200, 200))
	if hint.Visible() {
		t.Fatal("hover explanation remains visible after leaving")
	}
}
