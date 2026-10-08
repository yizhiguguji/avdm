package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"testing"
)

func TestPreviewGeometryKeepsDensityAcrossWindowAndLogSizes(t *testing.T) {
	for _, spec := range controlDensityOptions {
		overhead := spec.cardSize.Height - spec.previewSize.Height
		baseline := calculatePreviewGeometry(fyne.NewSize(780, 620), 2, spec.cardSize.Width, overhead)
		for _, viewport := range []fyne.Size{fyne.NewSize(780, 280), fyne.NewSize(2100, 1280), fyne.NewSize(320, 400)} {
			geom := calculatePreviewGeometry(viewport, 2, spec.cardSize.Width, overhead)
			if geom.card != baseline.card || geom.preview != spec.previewSize {
				t.Fatalf("window resize changed density: %+v / %+v", baseline, geom)
			}
		}
		for _, count := range []int{1, 2, 10, 30} {
			viewport := fyne.NewSize(2100, 1280)
			geom := calculatePreviewGeometry(viewport, count, spec.cardSize.Width, overhead)
			used := geom.card.Width*float32(geom.columns) + geom.gap*float32(geom.columns-1)
			if used > viewport.Width || geom.columns > count {
				t.Fatalf("invalid columns: %+v", geom)
			}
		}
	}
}

func TestPreviewGridAnchorsAtTopLeftAndUsesFixedDensity(t *testing.T) {
	g := testWall(t)
	g.wallOnlineCount = 2
	objects := []fyne.CanvasObject{canvas.NewRectangle(nil), canvas.NewRectangle(nil)}
	l := &adaptivePreviewLayout{g: g, viewport: fyne.NewSize(1200, 700)}
	l.Layout(objects, fyne.NewSize(1200, 700))
	firstSize := objects[0].Size()
	origin := float32(0)
	if objects[0].Position() != fyne.NewPos(origin, 0) || objects[1].Position().Y != 0 {
		t.Fatal("device grid is not left aligned")
	}
	if objects[1].Position().X != origin+firstSize.Width+16 {
		t.Fatal("grid spacing changed")
	}
	l.viewport = fyne.NewSize(2400, 300)
	l.Layout(objects, fyne.NewSize(2400, 300))
	if objects[0].Size() != firstSize || objects[0].Position() != fyne.NewPos(0, 0) {
		t.Fatal("maximizing or opening logs changes density or group alignment")
	}
}

func TestWallWorkspaceSwitchesCompleteLibraryAndPreservesSessions(t *testing.T) {
	g := testWall(t)
	if g.buildWallWorkspace() != g.wallWorkspace {
		t.Fatal("workspace has an extra header wrapper")
	}
	entries := toolbarTestEntries()
	g.entries = entries
	g.renderControlCenter()
	rows := make([]fyne.CanvasObject, len(entries))
	for i, e := range entries {
		rows[i] = widget.NewLabel(e.Label)
	}
	g.updateWallWorkspace(g.controlGrid.Objects, rows)
	card := g.controlCards["phone"]
	for _, size := range []fyne.Size{fyne.NewSize(1200, 700), fyne.NewSize(780, 400), fyne.NewSize(2600, 1400)} {
		g.wallWorkspace.Resize(size)
		if !g.wallWorkspace.Objects[0].Visible() || g.wallWorkspace.Objects[1].Visible() {
			t.Fatal("library reserves a permanent column")
		}
		if g.wallWorkspace.Objects[0].Size() != size {
			t.Fatal("wall does not fill workspace")
		}
	}
	g.wallLibraryButton.OnTapped()
	if !g.wallLibraryMode || g.wallWorkspace.Objects[0].Visible() || !g.wallWorkspace.Objects[1].Visible() {
		t.Fatal("library view did not replace wall")
	}
	if len(g.wallLibraryGrid.Objects) != len(entries) || g.wallLibraryButton.Text != "设备墙 (2)" {
		t.Fatal("complete library or return count missing")
	}
	g.wallWorkspace.Resize(fyne.NewSize(780, 400))
	if !g.wallWorkspace.Objects[1].Visible() {
		t.Fatal("library disappears at narrow widths")
	}
	g.wallLibraryButton.OnTapped()
	if g.wallLibraryMode || g.controlCards["phone"] != card || g.wallLibraryButton.Text != "设备库 (6)" {
		t.Fatal("view switch disrupted session or count")
	}
}

func TestWallWithoutOnlineDevicesKeepsLibraryEntryPoint(t *testing.T) {
	g := testWall(t)
	g.buildWallWorkspace()
	g.updateWallWorkspace(nil, []fyne.CanvasObject{widget.NewLabel("Stopped AVD")})
	g.wallWorkspace.Resize(fyne.NewSize(1200, 700))
	if g.wallLibraryMode || !g.wallWorkspace.Objects[0].Visible() || g.wallWorkspace.Objects[1].Visible() || len(g.controlGrid.Objects) != 1 {
		t.Fatal("empty wall switched automatically or lost next action")
	}
	g.showDeviceLibraryDialog()
	if !g.wallLibraryMode || !g.wallWorkspace.Objects[1].Visible() {
		t.Fatal("empty wall library action failed")
	}
}

func TestWorkbenchWidthFillsMaximizedWindow(t *testing.T) {
	object := canvas.NewRectangle(nil)
	workbenchWidthLayout{}.Layout([]fyne.CanvasObject{object}, fyne.NewSize(3200, 1600))
	if object.Position() != fyne.NewPos(0, 0) || object.Size() != fyne.NewSize(3200, 1600) {
		t.Fatal("workspace remains capped or centered")
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

func TestStoppedListReclaimsSecondPreviewColumnForToolPanel(t *testing.T) {
	g := testWall(t)
	g.controlDensity = controlDensitySmall
	g.buildWallWorkspace()
	g.wallOnlineCount = 2
	g.wallStoppedGrid.Objects = []fyne.CanvasObject{widget.NewLabel("Stopped AVD"), widget.NewLabel("Offline AVD")}
	online := g.wallWorkspace.Objects[0].(*fyne.Container)
	l := online.Layout.(*onlineWorkspaceLayout)
	online.Resize(fyne.NewSize(1200, 700))
	if !g.wallStoppedPanel.Visible() || l.entry.Visible() {
		t.Fatal("normal wide view lost visible stopped list")
	}
	width := online.Objects[0].Size().Width
	if calculatePreviewGeometry(fyne.NewSize(width, 700), 2, g.controlDensitySpec().cardSize.Width, 100).columns != 2 {
		t.Fatal("normal stopped list unnecessarily crowds previews")
	}
	online.Resize(fyne.NewSize(850, 700))
	if g.wallStoppedPanel.Visible() || !l.entry.Visible() || l.entry.Text != "未启动与异常设备 (2) · 展开" {
		t.Fatal("narrow view lost stopped list count or reclaim action")
	}
	if online.Objects[0].Size().Width != 850 || calculatePreviewGeometry(online.Objects[0].Size(), 2, g.controlDensitySpec().cardSize.Width, 100).columns != 2 {
		t.Fatal("tool panel did not reclaim second preview column")
	}
	online.Resize(fyne.NewSize(1200, 700))
	if !g.wallStoppedPanel.Visible() || l.entry.Visible() {
		t.Fatal("stopped list does not return after tool closes")
	}
}

func TestStoppedListRemainsVisibleWhenNoDevicesAreOnline(t *testing.T) {
	g := testWall(t)
	g.buildWallWorkspace()
	g.wallStoppedGrid.Objects = []fyne.CanvasObject{widget.NewLabel("Stopped AVD")}
	online := g.wallWorkspace.Objects[0].(*fyne.Container)
	online.Resize(fyne.NewSize(360, 500))
	if !g.wallStoppedPanel.Visible() || g.wallStoppedPanel.Size() != fyne.NewSize(360, 500) || online.Objects[0].Visible() {
		t.Fatal("no-online view lost directly accessible stopped devices")
	}
}

func TestCollapsedStoppedListOpensWithoutChangingWorkbenchView(t *testing.T) {
	g := testWall(t)
	g.window = g.app.NewWindow("test")
	g.buildWallWorkspace()
	g.window.SetContent(g.wallWorkspace)
	g.entries = toolbarTestEntries()
	g.renderControlCenter()
	online := g.wallWorkspace.Objects[0].(*fyne.Container)
	online.Resize(fyne.NewSize(850, 700))
	entry := online.Layout.(*onlineWorkspaceLayout).entry
	if !entry.Visible() {
		t.Fatal("collapsed stopped list has no visible entry")
	}
	entry.OnTapped()
	if g.wallLibraryMode || len(g.window.Canvas().Overlays().List()) != 1 {
		t.Fatal("one-click list entry changes primary view or fails to open")
	}
}
