package main

import (
	core "adm/internal/app"
	"bytes"
	"image"
	"image/png"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func testWall(t *testing.T) *GUIApp {
	t.Helper()
	a := test.NewApp()
	t.Cleanup(a.Quit)
	g := &GUIApp{app: a, controlGrid: container.NewVBox(), controlSummary: widget.NewLabel(""), selectedLabel: widget.NewLabel(""), controlSelected: map[string]bool{}, controlHidden: map[string]bool{}, controlCards: map[string]*controlCardView{}, controlRealtimeStops: map[string]func(){}}
	g.ensureDeviceWall()
	t.Cleanup(g.closeDeviceWall)
	return g
}

func testWallEntry(key, serial string) core.DeviceEntry {
	return core.DeviceEntry{Key: key, Label: key, Active: &core.ActiveDevice{Serial: serial, State: "device"}}
}

func TestDeviceWallPreservesSessionAcrossScanSelectionAndDensity(t *testing.T) {
	g := testWall(t)
	g.entries = []core.DeviceEntry{testWallEntry("one", "serial-one")}
	g.renderControlCenter()
	card := g.controlCards["one"]
	object := card.wall.object
	stopped := 0
	g.controlRealtimeStops["one"] = func() { stopped++ }
	card.realtime = true
	g.controlSelected["one"] = true
	g.controlDensity = controlDensityHD
	g.entries[0].Label = "renamed"
	g.renderControlCenter()
	if g.controlCards["one"] != card || card.wall.object != object || stopped != 0 || !card.realtime {
		t.Fatalf("unchanged device lost its preview session: stops=%d", stopped)
	}
	if !card.wall.selected.Checked || card.wall.title.Text != controlCardTitle(g.entries[0]) || card.preview.minSize != g.controlDensitySpec().previewSize {
		t.Fatal("reused card did not reconcile selection, label and density")
	}
	g.wallSearch = "no-matching-device"
	g.renderControlCenter()
	if g.controlCards["one"] != card || card.wall.visible || stopped != 0 || !card.realtime {
		t.Fatal("filter destroyed a live session")
	}
	g.wallSearch = ""
	g.controlHidden["one"] = true
	g.renderControlCenter()
	if g.controlCards["one"] != card || !card.hidden || stopped != 0 || !card.realtime {
		t.Fatal("hiding the preview destroyed a live session")
	}
	g.controlHidden["one"] = false
	g.renderControlCenter()
	if g.controlCards["one"] != card || !card.wall.visible || stopped != 0 {
		t.Fatal("restoring the preview recreated its session")
	}
}

func TestDeviceWallDestroysRemovedAndChangedSessions(t *testing.T) {
	g := testWall(t)
	g.entries = []core.DeviceEntry{testWallEntry("one", "s1"), testWallEntry("two", "s2")}
	g.renderControlCenter()
	one, two := g.controlCards["one"], g.controlCards["two"]
	stopped := 0
	g.controlRealtimeStops["one"] = func() { stopped++ }
	g.controlRealtimeStops["two"] = func() { stopped++ }
	g.entries = []core.DeviceEntry{testWallEntry("one", "replacement")}
	g.renderControlCenter()
	if stopped != 2 || g.controlCards["one"] == one || g.controlCards["two"] != nil || g.controlCards["one"] == two {
		t.Fatalf("obsolete sessions were retained: stops=%d", stopped)
	}
}

func TestWallRejectsLateConnectionAfterReplacementOrGenerationChange(t *testing.T) {
	g := testWall(t)
	g.entries = []core.DeviceEntry{testWallEntry("one", "s1")}
	g.renderControlCenter()
	card := g.controlCards["one"]
	stopped := 0
	card.wall.generation = 2
	g.applyWallConnection("one", card, 1, nil, func() { stopped++ }, nil)
	if stopped != 1 || card.realtime {
		t.Fatal("late generation connection was accepted")
	}
	delete(g.controlCards, "one")
	g.applyWallConnection("one", card, 2, nil, func() { stopped++ }, nil)
	if stopped != 2 {
		t.Fatal("connection for removed card was not cancelled")
	}
	g.closeDeviceWall()
	g.applyWallConnection("one", card, 2, nil, func() { stopped++ }, nil)
	if stopped != 3 {
		t.Fatal("connection after close was not cancelled")
	}
}

func TestWallScreenshotsHaveGlobalConcurrencyLimitAndCloseCancelsQueue(t *testing.T) {
	g := testWall(t)
	var current, maximum, calls atomic.Int32
	entered := make(chan struct{}, 12)
	release := make(chan struct{})
	g.wall.screenshot = func(string) ([]byte, error) {
		n := current.Add(1)
		calls.Add(1)
		for old := maximum.Load(); n > old && !maximum.CompareAndSwap(old, n); old = maximum.Load() {
		}
		entered <- struct{}{}
		<-release
		current.Add(-1)
		return nil, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		card := &controlCardView{serial: "serial", wall: &wallCardState{visible: true}}
		wg.Add(1)
		g.refreshControlCardAsync("unused", card, wg.Done)
	}
	for i := 0; i < 3; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("screenshot workers did not start")
		}
	}
	select {
	case <-entered:
		t.Fatal("more than three screenshots started concurrently")
	case <-time.After(30 * time.Millisecond):
	}
	g.closeDeviceWall()
	close(release)
	complete := make(chan struct{})
	go func() { wg.Wait(); close(complete) }()
	select {
	case <-complete:
	case <-time.After(2 * time.Second):
		t.Fatal("queued screenshots did not stop on window close")
	}
	if maximum.Load() != 3 || calls.Load() != 3 {
		t.Fatalf("unexpected workers max=%d calls=%d", maximum.Load(), calls.Load())
	}
}

func TestWallScreenshotReplyDoesNotOverwriteRealtimeFrame(t *testing.T) {
	g := testWall(t)
	g.entries = []core.DeviceEntry{testWallEntry("one", "s1")}
	g.renderControlCenter()
	card := g.controlCards["one"]
	var data bytes.Buffer
	_ = png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	replies := make(chan func(), 1)
	g.wall.post = func(fn func()) { replies <- fn }
	g.wall.screenshot = func(string) ([]byte, error) { return data.Bytes(), nil }
	g.refreshControlCardAsync("one", card, nil)
	var reply func()
	select {
	case reply = <-replies:
	case <-time.After(2 * time.Second):
		t.Fatal("screenshot reply missing")
	}
	live := image.NewRGBA(image.Rect(0, 0, 3, 3))
	card.preview.setImage(live)
	card.realtime = true
	reply()
	if card.preview.img.Image != live {
		t.Fatal("late screenshot overwrote realtime image")
	}
}

func TestWallStalledRealtimeFallsBackAndOldScreenshotIsMarked(t *testing.T) {
	g := testWall(t)
	g.entries = []core.DeviceEntry{testWallEntry("one", "s1")}
	g.renderControlCenter()
	card := g.controlCards["one"]
	now := time.Now()
	card.realtime = true
	card.wall.lastFrame = now.Add(-6 * time.Second)
	card.wall.screenshotAt = now.Add(-40 * time.Second)
	stopped := false
	g.controlRealtimeStops["one"] = func() { stopped = true }
	g.updateWallCardFreshness(card, now)
	if !stopped || card.realtime || !card.wall.disconnected {
		t.Fatal("stalled realtime did not release the stream")
	}
	if card.status.Text == "" || !bytes.Contains([]byte(card.status.Text), []byte("画面过期")) {
		t.Fatalf("stale screenshot was not marked: %s", card.status.Text)
	}
}

func TestWallConnectionRunsOffUIAndCancelsLateReplacement(t *testing.T) {
	g := testWall(t)
	g.entries = []core.DeviceEntry{testWallEntry("one", "s1")}
	g.renderControlCenter()
	card := g.controlCards["one"]
	entered := make(chan struct{})
	release := make(chan struct{})
	replies := make(chan func(), 1)
	stopped := false
	g.wall.post = func(fn func()) { replies <- fn }
	g.wall.connect = func(core.DeviceEntry) (<-chan core.GUIEmulatorRealtimeFrame, func(), error) {
		close(entered)
		<-release
		return nil, func() { stopped = true }, nil
	}
	g.startControlRealtimeStreamFor("one", card)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("background connection did not start")
	}
	// A scan changes the device identity while the connection is pending.
	g.entries = []core.DeviceEntry{testWallEntry("one", "replacement")}
	delete(g.controlCards, "one")
	close(release)
	select {
	case reply := <-replies:
		reply()
	case <-time.After(2 * time.Second):
		t.Fatal("late connection reply missing")
	}
	if !stopped {
		t.Fatal("connection for replaced card was not stopped")
	}
}

func TestWallPreviewCardsFitEveryDensity(t *testing.T) {
	g := testWall(t)
	g.entries = []core.DeviceEntry{testWallEntry("long-device-name-example", "serial-one")}
	for _, density := range controlDensityOptions {
		g.controlDensity = density.key
		g.renderControlCenter()
		card := g.controlCards[g.entries[0].Key]
		if card.wall.object.MinSize().Width > density.cardSize.Width {
			t.Fatalf("%s card minimum %.0f exceeds reserved width %.0f", density.label, card.wall.object.MinSize().Width, density.cardSize.Width)
		}
	}
}

func TestWallRealtimeFramesCoalesceBeforeUIConsumes(t *testing.T) {
	g := testWall(t)
	g.entries = []core.DeviceEntry{testWallEntry("one", "s1")}
	g.renderControlCenter()
	card := g.controlCards["one"]
	card.wall.generation = 1
	card.realtime = true
	var data bytes.Buffer
	_ = png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	frames := make(chan core.GUIEmulatorRealtimeFrame, 100)
	for i := 0; i < 100; i++ {
		frames <- core.GUIEmulatorRealtimeFrame{PNG: data.Bytes(), Timestamp: time.Now()}
	}
	close(frames)
	replies := make(chan func(), 200)
	g.wall.post = func(fn func()) { replies <- fn }
	g.consumeControlRealtimeFrames("one", card, 1, frames)
	// One image update and one stream-end notification may be queued. The
	// remaining images must not pile up behind a busy UI thread.
	if len(replies) != 2 {
		t.Fatalf("queued %d UI callbacks for 100 frames", len(replies))
	}
	(<-replies)()
	(<-replies)()
}
