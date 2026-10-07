package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Dock geometry. All values are pixels, not ratios, so panes keep their size
// when the window resizes; the center content absorbs the difference.
const (
	dockIconBarWidth    float32 = 64  // width of an always-visible edge icon bar
	dockIconGap         float32 = 6   // gap between an icon bar and its neighbour
	dockRailHeight      float32 = 36  // height of the collapsed bottom rail
	dockHandleThickness float32 = 6   // draggable divider thickness
	dockSideMinWidth    float32 = 200 // min expanded width of a side pane
	dockSideMaxWidth    float32 = 480 // absolute max expanded width of a side pane
	dockCenterMinWidth  float32 = 320 // reserved min width for the device wall
	dockLogMinHeight    float32 = 120 // min expanded height of the log pane
	dockLogMaxFraction  float32 = 0.6 // log pane may take at most this fraction of height

	dockRightDefaultWidth float32 = 340
	dockLogDefaultHeight  float32 = 220
)

var (
	dockHandleColor      = color.NRGBA{R: 15, G: 23, B: 42, A: 0}
	dockHandleHoverColor = color.NRGBA{R: 49, G: 112, B: 211, A: 50}
	dockGripColor        = admColorBorder
	dockGripHoverColor   = admColorPrimary
)

// setPaneCollapsed toggles a docked pane between its full panel and its
// collapsed rail. Hiding the resize handle when collapsed is what makes the
// pane physically undraggable — Fyne routes drag events only to visible
// objects, so a hidden handle cannot be grabbed.
func setPaneCollapsed(panel, rail fyne.CanvasObject, handle *resizeHandle, collapsed bool) {
	if collapsed {
		if panel != nil {
			panel.Hide()
		}
		if rail != nil {
			rail.Show()
		}
		if handle != nil {
			handle.Hide()
		}
		return
	}
	if panel != nil {
		panel.Show()
	}
	if rail != nil {
		rail.Hide()
	}
	if handle != nil {
		handle.Show()
	}
}

func clampWidth(v, min, max float32) float32 {
	if max < min {
		max = min
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// sideMaxWidth caps a side pane so it can never crowd out the center wall.
func sideMaxWidth(total float32) float32 {
	if total <= 0 {
		return dockSideMaxWidth
	}
	max := total * 0.4
	if max > dockSideMaxWidth {
		max = dockSideMaxWidth
	}
	if max < dockSideMinWidth {
		max = dockSideMinWidth
	}
	return max
}

// rightPaneWidth is zero when no tool panel is active; otherwise the active
// panel gets the shared right width.
func (g *GUIApp) rightPaneWidth(total float32) float32 {
	if g.rightActive == "" {
		return 0
	}
	return clampWidth(g.rightW, dockSideMinWidth, sideMaxWidth(total))
}

func (g *GUIApp) logPaneHeight(total float32) float32 {
	if g.logCollapsed {
		return dockRailHeight
	}
	max := total * dockLogMaxFraction
	if max < dockLogMinHeight {
		max = dockLogMinHeight
	}
	return clampWidth(g.logH, dockLogMinHeight, max)
}

// resizeRightBy: the right divider sits to the left of the right pane, so
// dragging it right (dx>0) shrinks the pane.
func (g *GUIApp) resizeRightBy(dx float32) {
	total := float32(0)
	if g.hDock != nil {
		total = g.hDock.Size().Width
	}
	g.rightW = clampWidth(g.rightW-dx, dockSideMinWidth, sideMaxWidth(total))
	if g.hDock != nil {
		g.hDock.Refresh()
	}
}

// resizeLogBy: the log divider sits above the log pane, so dragging it down
// (dy>0) shrinks the pane.
func (g *GUIApp) resizeLogBy(dy float32) {
	total := float32(0)
	if g.vDock != nil {
		total = g.vDock.Size().Height
	}
	max := total * dockLogMaxFraction
	if max < dockLogMinHeight {
		max = dockLogMinHeight
	}
	g.logH = clampWidth(g.logH-dy, dockLogMinHeight, max)
	if g.vDock != nil {
		g.vDock.Refresh()
	}
}

// edgeBarLayout gives the device workbench all space up to the right tool rail.
type edgeBarLayout struct{}

func (l edgeBarLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := float32(0)
	for _, obj := range objects {
		if obj != nil && obj.Visible() && obj.MinSize().Height > height {
			height = obj.MinSize().Height
		}
	}
	return fyne.NewSize(dockCenterMinWidth+dockIconBarWidth+dockIconGap, height)
}

func (l edgeBarLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	width := size.Width - dockIconBarWidth - dockIconGap
	if width < dockCenterMinWidth {
		width = dockCenterMinWidth
	}
	placeCol(objects[0], 0, width, size.Height)
	placeCol(objects[1], width+dockIconGap, dockIconBarWidth, size.Height)
}

// horizontalDockLayout arranges the wall, right divider and active tool panel.
type horizontalDockLayout struct{ g *GUIApp }

func (l horizontalDockLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := float32(0)
	for _, obj := range objects {
		if obj != nil && obj.Visible() && obj.MinSize().Height > height {
			height = obj.MinSize().Height
		}
	}
	return fyne.NewSize(dockCenterMinWidth, height)
}

func (l horizontalDockLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 3 {
		return
	}
	rw := l.g.rightPaneWidth(size.Width)
	rh := dockHandleThickness
	if l.g.rightActive == "" {
		rh = 0
	}
	cw := size.Width - rw - rh
	if cw < dockCenterMinWidth {
		cw = dockCenterMinWidth
	}
	x := placeCol(objects[0], 0, cw, size.Height)
	x = placeCol(objects[1], x, rh, size.Height)
	placeCol(objects[2], x, rw, size.Height)
}

// placeCol positions obj at x with the given width/height and returns x+width.
func placeCol(obj fyne.CanvasObject, x, w, h float32) float32 {
	if obj != nil {
		obj.Move(fyne.NewPos(x, 0))
		obj.Resize(fyne.NewSize(w, h))
	}
	return x + w
}

// verticalDockLayout arranges: top (horizontal dock) / logHandle / logStack.
type verticalDockLayout struct {
	g *GUIApp
}

func (l verticalDockLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := float32(0)
	for _, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		if m := obj.MinSize(); m.Width > width {
			width = m.Width
		}
	}
	return fyne.NewSize(width, dockLogMinHeight+dockCenterMinWidth)
}

func (l verticalDockLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 3 {
		return
	}
	g := l.g
	bh := g.logPaneHeight(size.Height)
	hh := dockHandleThickness
	if g.logCollapsed {
		hh = 0
	}
	th := size.Height - hh - bh
	if th < dockCenterMinWidth {
		th = dockCenterMinWidth
	}
	y := placeRow(objects[0], 0, th, size.Width)
	y = placeRow(objects[1], y, hh, size.Width)
	placeRow(objects[2], y, bh, size.Width)
}

func placeRow(obj fyne.CanvasObject, y, h, w float32) float32 {
	if obj != nil {
		obj.Move(fyne.NewPos(0, y))
		obj.Resize(fyne.NewSize(w, h))
	}
	return y + h
}

// resizeHandle is a thin draggable divider. vertical=true means the bar is
// vertical and drags horizontally (resizes width); vertical=false means a
// horizontal bar dragging vertically (resizes height). It reports a resize
// cursor and highlights on hover.
type resizeHandle struct {
	widget.BaseWidget
	vertical bool
	onDrag   func(delta float32)
	hovered  bool
}

func newResizeHandle(vertical bool, onDrag func(float32)) *resizeHandle {
	h := &resizeHandle{vertical: vertical, onDrag: onDrag}
	h.ExtendBaseWidget(h)
	return h
}

func (h *resizeHandle) Dragged(e *fyne.DragEvent) {
	if h.onDrag == nil {
		return
	}
	if h.vertical {
		h.onDrag(e.Dragged.DX)
	} else {
		h.onDrag(e.Dragged.DY)
	}
}

func (h *resizeHandle) DragEnd() {}

func (h *resizeHandle) Cursor() desktop.Cursor {
	if h.vertical {
		return desktop.HResizeCursor
	}
	return desktop.VResizeCursor
}

func (h *resizeHandle) MouseIn(*desktop.MouseEvent) {
	h.hovered = true
	h.Refresh()
}

func (h *resizeHandle) MouseMoved(*desktop.MouseEvent) {}

func (h *resizeHandle) MouseOut() {
	h.hovered = false
	h.Refresh()
}

func (h *resizeHandle) CreateRenderer() fyne.WidgetRenderer {
	bar := canvas.NewRectangle(dockHandleColor)
	grip := canvas.NewRectangle(dockGripColor)
	grip.CornerRadius = 1.5
	return &resizeHandleRenderer{handle: h, bar: bar, grip: grip, objects: []fyne.CanvasObject{bar, grip}}
}

type resizeHandleRenderer struct {
	handle  *resizeHandle
	bar     *canvas.Rectangle
	grip    *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (r *resizeHandleRenderer) Layout(size fyne.Size) {
	r.bar.Resize(size)
	r.bar.Move(fyne.NewPos(0, 0))
	// Draw a short centered grip along the handle's long axis.
	if r.handle.vertical {
		gw := float32(2)
		gh := size.Height * 0.18
		if gh < 16 {
			gh = 16
		}
		if gh > size.Height {
			gh = size.Height
		}
		r.grip.Resize(fyne.NewSize(gw, gh))
		r.grip.Move(fyne.NewPos((size.Width-gw)/2, (size.Height-gh)/2))
	} else {
		gh := float32(2)
		gw := size.Width * 0.18
		if gw < 16 {
			gw = 16
		}
		if gw > size.Width {
			gw = size.Width
		}
		r.grip.Resize(fyne.NewSize(gw, gh))
		r.grip.Move(fyne.NewPos((size.Width-gw)/2, (size.Height-gh)/2))
	}
}

func (r *resizeHandleRenderer) MinSize() fyne.Size {
	return fyne.NewSize(dockHandleThickness, dockHandleThickness)
}

func (r *resizeHandleRenderer) Refresh() {
	if r.handle.hovered {
		r.bar.FillColor = dockHandleHoverColor
		r.grip.FillColor = dockGripHoverColor
	} else {
		r.bar.FillColor = dockHandleColor
		r.grip.FillColor = dockGripColor
	}
	r.bar.Refresh()
	r.grip.Refresh()
}

func (r *resizeHandleRenderer) Destroy() {}

func (r *resizeHandleRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

// Standard labeled buttons provide Fyne focus traversal and keyboard activation.
// Text alone fits the narrow rail; the resource remains accepted for callers
// that identify the corresponding action icon.
func newWorkbenchRailButton(label string, resource fyne.Resource, onTap func()) *widget.Button {
	button := widget.NewButton(label, onTap)
	button.Importance = widget.LowImportance
	button.SetIcon(resource)
	return button
}

func setWorkbenchToolActive(button *widget.Button, active bool) {
	if button == nil {
		return
	}
	if active {
		button.Importance = widget.MediumImportance
	} else {
		button.Importance = widget.LowImportance
	}
	button.Refresh()
}
