package main

import (
	"image"
	"testing"

	"fyne.io/fyne/v2"
)

func TestPreviewTapToDevicePortraitLetterbox(t *testing.T) {
	x, y, ok := previewTapToDevice(
		fyne.NewPos(180, 110),
		fyne.NewSize(360, 220),
		image.Pt(1080, 2400),
	)
	if !ok {
		t.Fatal("expected tap to hit preview image")
	}
	if x != 540 || y != 1200 {
		t.Fatalf("unexpected mapped coordinate: %d,%d", x, y)
	}
}

func TestPreviewTapToDeviceOutsideLetterbox(t *testing.T) {
	_, _, ok := previewTapToDevice(
		fyne.NewPos(10, 110),
		fyne.NewSize(360, 220),
		image.Pt(1080, 2400),
	)
	if ok {
		t.Fatal("expected tap in side letterbox to be ignored")
	}
}

func TestPreviewSwipeToDevicePortraitLetterbox(t *testing.T) {
	x1, y1, x2, y2, ok := previewSwipeToDevice(
		fyne.NewPos(180, 182),
		fyne.NewPos(180, 38),
		fyne.NewSize(360, 220),
		image.Pt(1080, 2400),
	)
	if !ok {
		t.Fatal("expected swipe to hit preview image")
	}
	if x1 != 540 || y1 != 1985 || x2 != 540 || y2 != 414 {
		t.Fatalf("unexpected mapped swipe: %d,%d -> %d,%d", x1, y1, x2, y2)
	}
}

func TestPreviewSwipeToDeviceRejectsLetterboxEndpoint(t *testing.T) {
	_, _, _, _, ok := previewSwipeToDevice(
		fyne.NewPos(180, 110),
		fyne.NewPos(10, 110),
		fyne.NewSize(360, 220),
		image.Pt(1080, 2400),
	)
	if ok {
		t.Fatal("expected swipe ending in side letterbox to be ignored")
	}
}
