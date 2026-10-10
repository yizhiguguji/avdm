package main

import (
	core "adm/internal/app"
	"errors"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func visitCardObjects(object fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	visit(object)
	if group, ok := object.(*fyne.Container); ok {
		for _, child := range group.Objects {
			visitCardObjects(child, visit)
		}
	}
}
func cardButtons(object fyne.CanvasObject) map[string]*widget.Button {
	buttons := map[string]*widget.Button{}
	visitCardObjects(object, func(object fyne.CanvasObject) {
		switch button := object.(type) {
		case *widget.Button:
			buttons[button.Text] = button
		case *deviceIconButton:
			buttons[button.actionName] = &button.Button
		}
	})
	return buttons
}

func TestPreviewCardKeepsCompactActionsAndCompleteMenu(t *testing.T) {
	g := testWall(t)
	entry := testWallEntry("one", "serial-one")
	for _, density := range controlDensityOptions {
		object := g.buildControlCard(entry, density)
		buttons := cardButtons(object)
		for _, label := range []string{"主目标", "窗口", "返回", "主页", "通知", "更多"} {
			if buttons[label] == nil || buttons[label].OnTapped == nil {
				t.Fatalf("%s has no action %q", density.label, label)
			}
		}
		if len(buttons) != 6 {
			t.Fatalf("unexpected direct button count %d", len(buttons))
		}
		for _, label := range []string{"管理", "隐藏", "关闭"} {
			if buttons[label] != nil {
				t.Fatalf("%s still occupies card toolbar", label)
			}
		}
		menu := g.deviceCardMenu(g.controlCards[entry.Key])
		items := map[string]*fyne.MenuItem{}
		for _, item := range menu.Items {
			items[item.Label] = item
		}
		for _, label := range []string{"管理设备…", "隐藏画面", "关闭设备…"} {
			if items[label] == nil || items[label].Action == nil {
				t.Fatalf("missing menu action %q", label)
			}
		}
		g.controlCards[entry.Key].hidden = true
		if g.deviceCardMenu(g.controlCards[entry.Key]).Items[1].Label != "显示画面" {
			t.Fatal("hidden preview cannot be restored from menu")
		}
		object.Resize(object.MinSize())
		if object.MinSize().Width > density.cardSize.Width {
			t.Fatalf("card expanded: %v", object.MinSize())
		}
		if g.controlCards[entry.Key].preview.minSize != density.previewSize {
			t.Fatal("actions changed preview density")
		}
		var actions *fyne.Container
		visitCardObjects(object, func(object fyne.CanvasObject) {
			if group, ok := object.(*fyne.Container); ok {
				if _, ok := group.Layout.(deviceActionRowLayout); ok {
					actions = group
				}
			}
		})
		if actions == nil {
			t.Fatal("missing compact action row")
		}
		actions.Resize(fyne.NewSize(density.cardSize.Width-16, controlCardButtonRowsHeight))
		for _, child := range actions.Objects {
			if child.Position().Y != 0 || child.Position().X+child.Size().Width > actions.Size().Width+.1 {
				t.Fatalf("action clips single row: %v %v", child.Position(), child.Size())
			}
		}
	}
}

func TestCardNameAndIdentifierCopyRemainOneTap(t *testing.T) {
	g := testWall(t)
	entry := testWallEntry("one", "serial-one")
	object := g.buildControlCard(entry, g.controlDensitySpec())
	var copied []string
	visitCardObjects(object, func(object fyne.CanvasObject) {
		if tap, ok := object.(*copyTapLayer); ok {
			tap.Tapped(nil)
			copied = append(copied, g.app.Clipboard().Content())
		}
	})
	if len(copied) != 2 || copied[0] != controlCardTitle(entry) || copied[1] != "serial-one" {
		t.Fatalf("one-tap copy lost full values: %v", copied)
	}
}

func TestCardHideStopsStreamAndShowStartsFreshProbe(t *testing.T) {
	g := testWall(t)
	entry := testWallEntry("one", "emulator-test")
	entry.Active.IsEmulator = true
	g.entries = []core.DeviceEntry{entry}
	g.renderControlCenter()
	card := g.controlCards[entry.Key]
	card.realtime = true
	oldGeneration := card.wall.generation
	stops := 0
	g.controlRealtimeStops[entry.Key] = func() { stops++ }
	g.deviceCardMenu(card).Items[1].Action()
	if !card.hidden || card.realtime || stops != 1 || g.controlRealtimeStops[entry.Key] != nil || card.wall.generation <= oldGeneration {
		t.Fatal("hide did not cancel and invalidate live stream")
	}
	if g.deviceCardMenu(card).Items[1].Label != "显示画面" {
		t.Fatal("hide menu did not change to show")
	}
	lateStops := 0
	g.applyWallConnection(entry.Key, card, oldGeneration, nil, func() { lateStops++ }, nil)
	if lateStops != 1 || card.realtime {
		t.Fatal("late connection revived hidden stream")
	}
	posted := make(chan func(), 1)
	connected := make(chan struct{}, 1)
	g.wall.connect = func(core.DeviceEntry) (<-chan core.GUIEmulatorRealtimeFrame, func(), error) {
		connected <- struct{}{}
		return nil, nil, errors.New("fake probe")
	}
	g.wall.post = func(fn func()) { posted <- fn }
	g.deviceCardMenu(card).Items[1].Action()
	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("show did not probe immediately")
	}
	select {
	case fn := <-posted:
		fn()
	case <-time.After(time.Second):
		t.Fatal("probe result missing")
	}
	if card.hidden || !card.wall.probed || card.wall.connecting || g.deviceCardMenu(card).Items[1].Label != "隐藏画面" {
		t.Fatal("show did not restore fresh probe state")
	}
}

func TestCompactCardKeepsDeviceActionsAbovePreview(t *testing.T) {
	g := testWall(t)
	g.controlDensity = controlDensitySmall
	entry := testWallEntry("one", "serial-one")
	object := g.buildControlCard(entry, g.controlDensitySpec())
	object.Resize(object.MinSize())
	if object.MinSize().Height > 640 {
		t.Fatalf("compact card exceeds normal viewport: %v", object.MinSize())
	}
	var previewY float32
	actionBottom := map[string]float32{}
	var walk func(fyne.CanvasObject, fyne.Position)
	walk = func(object fyne.CanvasObject, origin fyne.Position) {
		pos := origin.Add(object.Position())
		if object == g.controlCards[entry.Key].wall.previewBox {
			previewY = pos.Y
		}
		switch button := object.(type) {
		case *widget.Button:
			actionBottom[button.Text] = pos.Y + button.Size().Height
		case *deviceIconButton:
			actionBottom[button.actionName] = pos.Y + button.Size().Height
		}
		if group, ok := object.(*fyne.Container); ok {
			for _, child := range group.Objects {
				walk(child, pos)
			}
		}
	}
	walk(object, fyne.NewPos(0, 0))
	if previewY > 68 {
		t.Fatalf("card header wastes preview space: top %.0f", previewY)
	}
	for _, label := range []string{"主目标", "窗口", "返回", "主页", "通知"} {
		if actionBottom[label] > previewY {
			t.Fatalf("%s remains below preview: bottom %.0f preview %.0f", label, actionBottom[label], previewY)
		}
	}
}

func TestExternalEmulatorMirrorPausesAndRestoresWallSnapshots(t *testing.T) {
	g := testWall(t)
	entry := testWallEntry("one", "emulator-test")
	entry.Active.IsEmulator = true
	g.buildControlCard(entry, g.controlDensitySpec())
	card := g.controlCards[entry.Key]
	active := true
	w := g.ensureDeviceWall()
	w.mirrorActive = func(string) bool { return active }
	requested := make(chan struct{}, 1)
	w.screenshot = func(string) ([]byte, error) { requested <- struct{}{}; return nil, errors.New("test screenshot") }
	completed := false
	g.refreshControlCardAsync(entry.Key, card, func() { completed = true })
	if !completed || card.refreshing || !card.wall.mirrorPaused {
		t.Fatal("external mirror did not suspend snapshot requests")
	}
	select {
	case <-requested:
		t.Fatal("competing ADB screenshot requested")
	default:
	}
	active = false
	g.refreshControlCardAsync(entry.Key, card, nil)
	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("snapshots did not resume after mirror closed")
	}
	if card.wall.mirrorPaused {
		t.Fatal("paused indicator not cleared")
	}
}

func TestCardMetadataLeftIdentifierAndRightTimestamp(t *testing.T) {
	g := testWall(t)
	entry := testWallEntry("one", "serial-one")
	for _, density := range controlDensityOptions {
		object := g.buildControlCard(entry, density)
		card := g.controlCards[entry.Key]
		card.wall.screenshotAt = time.Now()
		g.updateWallCardFreshness(card, time.Now())
		var footer *fyne.Container
		visitCardObjects(object, func(obj fyne.CanvasObject) {
			if c, ok := obj.(*fyne.Container); ok {
				if _, ok := c.Layout.(deviceCardMetadataLayout); ok {
					footer = c
				}
			}
		})
		if footer == nil {
			t.Fatal("missing metadata row")
		}
		footer.Resize(fyne.NewSize(density.cardSize.Width-16, 26))
		left, right := footer.Objects[0], footer.Objects[1]
		if left.Position().X != 0 || right.Position().X <= left.Position().X+left.Size().Width || right.Position().X+right.Size().Width > footer.Size().Width+.1 {
			t.Fatalf("metadata overlaps or clips at %s", density.label)
		}
		if card.wall.identifier.Text != "serial-one" || card.wall.identifier.Alignment != fyne.TextAlignLeading || card.status.Alignment != fyne.TextAlignTrailing {
			t.Fatal("metadata text or alignment incorrect")
		}
		if !strings.Contains(card.status.Text, card.wall.screenshotAt.Format("15:04:05")) {
			t.Fatal("timestamp missing")
		}
		for _, item := range g.deviceCardMenu(card).Items {
			if strings.Contains(item.Label, "复制") {
				t.Fatal("copy still in dropdown")
			}
		}
	}
}
