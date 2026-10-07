package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"testing"
)

func TestPreviewGeometryGrowsCentersAndFits(t *testing.T) {
	small := calculatePreviewGeometry(fyne.NewSize(780, 620), 2, 260, 124)
	wide := calculatePreviewGeometry(fyne.NewSize(2100, 1280), 2, 260, 124)
	if wide.preview.Height <= small.preview.Height || wide.columns != 2 {
		t.Fatalf("does not grow: %+v / %+v", small, wide)
	}
	for _, size := range []fyne.Size{fyne.NewSize(320, 400), fyne.NewSize(780, 620), fyne.NewSize(2100, 1280)} {
		for _, count := range []int{1, 2, 10, 30} {
			geom := calculatePreviewGeometry(size, count, 260, 124)
			width := geom.card.Width*float32(geom.columns) + geom.gap*float32(geom.columns-1)
			if width > size.Width+1 {
				t.Fatalf("overflow %.0f > %.0f", width, size.Width)
			}
			if absFloat32(geom.card.Height-geom.preview.Height-124) > 0.01 {
				t.Fatal("unused card padding")
			}
		}
	}
}
func TestWallWorkspaceSeparatesLibraryAndPreservesSessions(t *testing.T) {
	g := testWall(t)
	g.buildWallWorkspace()
	entries := toolbarTestEntries()
	g.entries = entries
	g.renderControlCenter()
	g.wallWorkspace.Resize(fyne.NewSize(1200, 700))
	if g.wallLibraryGrid.Size().Width > 620 {
		t.Fatal("library stretches across workbench")
	}
	if !g.wallWorkspace.Objects[1].Visible() || len(g.controlGrid.Objects) != 2 || len(g.wallLibraryGrid.Objects) != 4 {
		t.Fatal("online and library not separated")
	}
	card := g.controlCards["phone"]
	g.wallWorkspace.Resize(fyne.NewSize(780, 700))
	if g.wallWorkspace.Objects[1].Visible() {
		t.Fatal("narrow library crowds previews")
	}
	g.libraryCollapsed = true
	g.wallWorkspace.Resize(fyne.NewSize(1200, 700))
	if g.wallWorkspace.Objects[1].Visible() || g.controlCards["phone"] != card {
		t.Fatal("collapse disrupted preview")
	}
}
func TestLogSelectionCopyAndReadOnly(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	log := newSelectableLog()
	log.SetText("第一行 日志\nsecond line")
	original := log.Text
	log.TypedRune('x')
	log.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
	log.TypedShortcut(&fyne.ShortcutSelectAll{})
	if log.SelectedText() != original {
		t.Fatalf("selection failed %q", log.SelectedText())
	}
	log.TypedShortcut(&fyne.ShortcutCopy{Clipboard: a.Clipboard()})
	if a.Clipboard().Content() != original {
		t.Fatal("copy failed")
	}
	log.TypedShortcut(&fyne.ShortcutCut{Clipboard: a.Clipboard()})
	log.TypedShortcut(&fyne.ShortcutPaste{Clipboard: a.Clipboard()})
	if log.Text != original {
		t.Fatal("read only log was edited")
	}
}

func TestLogSelectionSnapshotSurvivesIncomingLines(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	g := &GUIApp{app: a, logLabel: newSelectableLog(), logLines: []string{"待复制的日志"}}
	g.renderLogLines()
	g.logLabel.TypedShortcut(&fyne.ShortcutSelectAll{})
	selected := g.logLabel.SelectedText()
	g.logPaused = true
	g.appendLog("INFO", "新的日志")
	if g.logLabel.SelectedText() != selected || g.logLabel.Text != selected {
		t.Fatal("新日志覆盖正在复制的选区")
	}
	g.logPaused = false
	g.renderLogLines()
	if g.logLabel.Text == selected {
		t.Fatal("跟随最新未恢复显示")
	}
}
