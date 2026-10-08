//go:build darwin

package app

import (
	"slices"
	"testing"
)

func TestMirrorLayoutKeepsSixPointGapAcrossDifferentAspectRatios(t *testing.T) {
	frames := []ExternalWindowFrame{{Width: 266, Height: 626}, {Width: 274, Height: 626}, {Width: 480, Height: 626}}
	area := ExternalWindowFrame{X: 16, Y: 100, Width: 1600, Height: 1000}
	placements := planMirrorWindowLayout(area, frames)
	for i, p := range placements {
		if p.Y != 100 || p.Height != 624 {
			t.Fatalf("lost work area or uniform height: %+v", p)
		}
		if i > 0 && p.X-(placements[i-1].X+placements[i-1].Width) != 6 {
			t.Fatalf("natural-width windows do not have 6 point gaps: %+v", placements)
		}
	}
	if placements[0].Width == placements[1].Width || placements[1].Width == placements[2].Width {
		t.Fatal("different aspect ratios were forced to equal widths")
	}
}

func TestMirrorLayoutScalesAndWrapsWithinWorkArea(t *testing.T) {
	area := ExternalWindowFrame{X: 6, Y: 42, Width: 600, Height: 1200}
	frames := make([]ExternalWindowFrame, 8)
	for i := range frames {
		frames[i] = ExternalWindowFrame{Width: 274, Height: 626}
	}
	placements := planMirrorWindowLayout(area, frames)
	wrapped := false
	for i, p := range placements {
		if p.Height >= 624 || p.Width <= 0 || p.Height != placements[0].Height || p.X+p.Width > 606 || p.Y+p.Height > 1242 {
			t.Fatalf("window exceeds work area or lost uniform scaling: %+v", p)
		}
		if i == 0 {
			continue
		}
		previous := placements[i-1]
		if p.Y == previous.Y {
			if p.X-previous.X-previous.Width != 6 {
				t.Fatal("wrong horizontal gap")
			}
		} else {
			wrapped = true
			if p.X != 6 || p.Y-previous.Y-previous.Height != 6 {
				t.Fatal("wrong row origin or vertical gap")
			}
		}
	}
	if !wrapped {
		t.Fatal("windows never wrapped")
	}
}

func TestMirrorPlacementArgsMatchOuterFramePlan(t *testing.T) {
	p := &scrcpyWindowPlacement{X: 16, Y: 100, Width: 265, Height: 624}
	args := scrcpyPlacementArgs(scrcpyArgs("phone", "mirror", true), p)
	for _, want := range []string{"--window-x=16", "--window-y=132", "--window-height=592", "--window-width=0"} {
		if !slices.Contains(args, want) {
			t.Fatalf("missing %s: %v", want, args)
		}
	}
	if slices.Contains(args, "--window-height=594") {
		t.Fatal("old fixed height overrides planned size")
	}
}

func TestMirrorDisplaySizeUsesOverride(t *testing.T) {
	w, h := parseMirrorDisplaySize("Physical size: 1440x3200\nOverride size: 1080x2400\n")
	if w != 1080 || h != 2400 {
		t.Fatalf("wrong display dimensions: %dx%d", w, h)
	}
}

func TestOriginalTileWorkAreaScreenOrigin(t *testing.T) {
	for _, bounds := range []ExternalWindowFrame{{Width: 2560, Height: 1440}, {X: 2560, Y: -200, Width: 1920, Height: 1080}} {
		want := ExternalWindowFrame{X: bounds.X + 6, Y: bounds.Y + 42, Width: bounds.Width - 12, Height: bounds.Height - 48}
		got := tileWorkAreaNative(bounds)
		if got != want {
			t.Fatalf("got %+v want %+v", got, want)
		}
		frames := []ExternalWindowFrame{{X: 625, Y: 268, Width: 265, Height: 624}, {X: 896, Y: 268, Width: 273, Height: 624}}
		for pass := 0; pass < 2; pass++ {
			placements := planMirrorWindowLayout(got, frames)
			if placements[0].X != int(want.X) || placements[0].Y != int(want.Y) {
				t.Fatalf("old window position affected screen origin: %+v", placements[0])
			}
			frames[0].X += 400
			frames[0].Y += 200
		}
	}
}
