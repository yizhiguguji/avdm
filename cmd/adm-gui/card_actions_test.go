package main

import (
	core "adm/internal/app"
	"errors"
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
		if button, ok := object.(*widget.Button); ok {
			buttons[button.Text] = button
		}
	})
	return buttons
}

func TestPreviewCardKeepsFrequentActionsDirectAndTwoRows(t *testing.T) {
	g := testWall(t)
	entry := testWallEntry("one", "serial-one")
	for _, density := range controlDensityOptions {
		object := g.buildControlCard(entry, density)
		buttons := cardButtons(object)
		for _, label := range []string{"主目标", "管理", "窗口", "隐藏", "返回", "主页", "通知", "关闭"} {
			if buttons[label] == nil || buttons[label].OnTapped == nil {
				t.Fatalf("%s density has no direct action %q", density.label, label)
			}
		}
		if len(buttons) != 9 {
			t.Fatalf("unexpected card button count %d", len(buttons))
		}
		object.Resize(object.MinSize())
		if object.MinSize().Width > density.cardSize.Width {
			t.Fatalf("%s action rows expand card: width=%v expected <=%v", density.label, object.MinSize().Width, density.cardSize.Width)
		}
		if g.controlCards[entry.Key].preview.minSize != density.previewSize {
			t.Fatal("adding actions changed preview dimensions")
		}
		var actions *fyne.Container
		visitCardObjects(object, func(object fyne.CanvasObject) {
			if group, ok := object.(*fyne.Container); ok && len(group.Objects) == 8 {
				if _, ok := group.Objects[0].(*widget.Button); ok {
					actions = group
				}
			}
		})
		if actions == nil {
			t.Fatal("missing compact action grid")
		}
		actions.Resize(fyne.NewSize(density.cardSize.Width-16, actions.MinSize().Height))
		rows := map[float32]bool{}
		for _, child := range actions.Objects {
			rows[child.Position().Y] = true
			if child.Position().X+child.Size().Width > actions.Size().Width+0.1 || child.Position().Y+child.Size().Height > actions.Size().Height+0.1 {
				t.Fatal("action clipped outside rows")
			}
		}
		if len(rows) != 2 {
			t.Fatalf("actions use %d rows", len(rows))
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
	cardButtons(card.wall.object)["隐藏"].OnTapped()
	if !card.hidden || card.realtime || stops != 1 || g.controlRealtimeStops[entry.Key] != nil || card.wall.generation <= oldGeneration {
		t.Fatal("hide did not cancel and invalidate live stream")
	}
	if cardButtons(card.wall.object)["显示"] == nil {
		t.Fatal("hide button did not change to show")
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
	cardButtons(card.wall.object)["显示"].OnTapped()
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
	if card.hidden || !card.wall.probed || card.wall.connecting || cardButtons(card.wall.object)["隐藏"] == nil {
		t.Fatal("show did not restore fresh probe state")
	}
}
