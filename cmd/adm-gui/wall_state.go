package main

import (
	core "adm/internal/app"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"sync"
	"sync/atomic"
	"time"
)

// wallState owns preview work for the lifetime of the main window. Mutable
// view state is only accessed on the Fyne thread; workers use done/closed.
type wallState struct {
	closed       atomic.Bool
	done         chan struct{}
	once         sync.Once
	screenshots  chan struct{}
	connections  chan struct{}
	screenshot   func(string) ([]byte, error)
	mirrorActive func(string) bool
	connect      func(core.DeviceEntry) (<-chan core.GUIEmulatorRealtimeFrame, func(), error)
	post         func(func())
}

type wallCardState struct {
	object             fyne.CanvasObject
	title              *widget.Label
	selected           *widget.Check
	previewBox         *fyne.Container
	visible            bool
	connecting         bool
	probed             bool
	lastProbe          time.Time
	lastFrame          time.Time
	screenshotAt       time.Time
	disconnected       bool
	mirrorPaused       bool
	generation         uint64
	connectedAt        time.Time
	frameUpdatePending atomic.Bool
}

func (g *GUIApp) ensureDeviceWall() *wallState {
	if g.wall != nil {
		return g.wall
	}
	w := &wallState{done: make(chan struct{}), screenshots: make(chan struct{}, 3), connections: make(chan struct{}, 2), post: fyne.Do}
	if g.backend != nil {
		w.screenshot = g.backend.GUIDeviceScreenPNG
		w.mirrorActive = g.backend.GUIHasLiveMirror
		w.connect = func(entry core.DeviceEntry) (<-chan core.GUIEmulatorRealtimeFrame, func(), error) {
			return g.backend.GUIStreamEmulatorRealtimeForEntry(entry, 0, 0)
		}
	}
	g.wall = w
	return w
}

func (g *GUIApp) closeDeviceWall() {
	w := g.ensureDeviceWall()
	w.closed.Store(true)
	w.once.Do(func() { close(w.done) })
	g.stopControlPreviewLoop()
}

func readyWallEntry(entry core.DeviceEntry) bool {
	return entry.Active != nil && entry.Active.State == "device"
}

func sameWallSession(card *controlCardView, entry core.DeviceEntry) bool {
	return card != nil && readyWallEntry(entry) && card.serial == entry.Active.Serial &&
		card.entry.Active != nil && card.entry.Active.IsEmulator == entry.Active.IsEmulator
}

func (g *GUIApp) currentWallCard(key string, card *controlCardView) bool {
	return g.wall != nil && !g.wall.closed.Load() && g.controlCards[key] == card
}

func (g *GUIApp) updateWallCardSize(card *controlCardView, spec controlDensitySpec) {
	if card.preview.minSize == spec.previewSize {
		return
	}
	// The image's native size is never changed: only its display box follows density.
	card.preview.minSize = spec.previewSize
	card.preview.Refresh()
	card.wall.previewBox.Layout = layout.NewGridWrapLayout(spec.previewSize)
	card.wall.previewBox.Refresh()
}
