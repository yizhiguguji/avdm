//go:build darwin

package app

import "testing"

func TestMainWindowSelectionExcludesRetainedQtControlWindows(t *testing.T) {
	for _, title := range []string{"", "Extended controls", "Extended Controls - Demo_AVD", "Demo_AVD - Extended controls"} {
		if nativeWindowTitleRank(title) != 0 {
			t.Fatalf("auxiliary window eligible for tiling: %q", title)
		}
	}
	if nativeWindowTitleRank("Android Emulator - Demo_AVD:5554") <= nativeWindowTitleRank("Other Qt window") {
		t.Fatal("native emulator main window must outrank unrelated Qt windows regardless of area")
	}
	if nativeWindowTitleRank("Demo Phone") == 0 {
		t.Fatal("phone mirror is not eligible for mixed tiling")
	}
}
