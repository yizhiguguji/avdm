package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"image"
	"image/color"
)

type previewPane struct {
	widget.BaseWidget
	bg             *canvas.Rectangle
	img            *canvas.Image
	message        *widget.Label
	messageIcon    *canvas.Text
	minSize        fyne.Size
	imageWidth     int
	imageHeight    int
	onTap          func(fyne.Position, fyne.Size, image.Point)
	onSecondaryTap func()
	onSwipe        func(fyne.Position, fyne.Position, fyne.Size, image.Point)
	dragging       bool
	dragStart      fyne.Position
	dragEnd        fyne.Position
}

type previewInputLayer struct {
	widget.BaseWidget
	preview   *previewPane
	dragging  bool
	dragStart fyne.Position
	dragEnd   fyne.Position
}

type copyTapLayer struct {
	widget.BaseWidget
	onTap func()
}

type previewPaneRenderer struct {
	pane    *previewPane
	objects []fyne.CanvasObject
}

func newPreviewPane(message string, minSize fyne.Size) *previewPane {
	p := &previewPane{
		bg:          canvas.NewRectangle(admColorPreviewBG),
		img:         canvas.NewImageFromImage(nil),
		message:     widget.NewLabel(message),
		messageIcon: canvas.NewText("", admColorText),
		minSize:     minSize,
	}
	p.img.FillMode = canvas.ImageFillContain
	p.img.ScaleMode = canvas.ImageScaleSmooth
	p.message.Alignment = fyne.TextAlignCenter
	p.messageIcon.TextSize = 38
	p.messageIcon.Hide()
	p.ExtendBaseWidget(p)
	return p
}

func newPreviewInputLayer(preview *previewPane) *previewInputLayer {
	layer := &previewInputLayer{preview: preview}
	layer.ExtendBaseWidget(layer)
	return layer
}

func newCopyTapLayer(onTap func()) *copyTapLayer {
	layer := &copyTapLayer{onTap: onTap}
	layer.ExtendBaseWidget(layer)
	return layer
}

func (l *copyTapLayer) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(color.NRGBA{A: 1})
	return widget.NewSimpleRenderer(rect)
}

func (l *copyTapLayer) MinSize() fyne.Size {
	return fyne.NewSize(1, 1)
}

func (l *copyTapLayer) Tapped(*fyne.PointEvent) {
	if l.onTap != nil {
		l.onTap()
	}
}

func (l *copyTapLayer) TappedSecondary(*fyne.PointEvent) {
	if l.onTap != nil {
		l.onTap()
	}
}

func (l *previewInputLayer) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(color.NRGBA{A: 1})
	return widget.NewSimpleRenderer(rect)
}

func (l *previewInputLayer) MinSize() fyne.Size {
	if l.preview == nil {
		return fyne.NewSize(1, 1)
	}
	return l.preview.minSize
}

func (l *previewInputLayer) Tapped(ev *fyne.PointEvent) {
	if l.preview == nil || l.preview.onTap == nil || l.preview.imageWidth <= 0 || l.preview.imageHeight <= 0 {
		return
	}
	l.preview.onTap(ev.Position, l.Size(), image.Pt(l.preview.imageWidth, l.preview.imageHeight))
}

func (l *previewInputLayer) TappedSecondary(*fyne.PointEvent) {
	if l.preview == nil || l.preview.onSecondaryTap == nil {
		return
	}
	l.preview.onSecondaryTap()
}

func (l *previewInputLayer) Dragged(ev *fyne.DragEvent) {
	if l.preview == nil || l.preview.imageWidth <= 0 || l.preview.imageHeight <= 0 {
		return
	}
	if !l.dragging {
		l.dragStart = ev.Position.Subtract(fyne.NewDelta(ev.Dragged.DX, ev.Dragged.DY))
		l.dragging = true
	}
	l.dragEnd = ev.Position
}

func (l *previewInputLayer) DragEnd() {
	if !l.dragging {
		return
	}
	start := l.dragStart
	end := l.dragEnd
	l.dragging = false
	if l.preview == nil || l.preview.onSwipe == nil || l.preview.imageWidth <= 0 || l.preview.imageHeight <= 0 {
		return
	}
	if absFloat32(end.X-start.X) < 4 && absFloat32(end.Y-start.Y) < 4 {
		return
	}
	l.preview.onSwipe(start, end, l.Size(), image.Pt(l.preview.imageWidth, l.preview.imageHeight))
}

func previewInteractiveObject(preview *previewPane) fyne.CanvasObject {
	return container.NewStack(preview, newPreviewInputLayer(preview))
}

func (p *previewPane) CreateRenderer() fyne.WidgetRenderer {
	return &previewPaneRenderer{
		pane:    p,
		objects: []fyne.CanvasObject{p.bg, p.img, p.message, p.messageIcon},
	}
}

func (p *previewPane) Tapped(ev *fyne.PointEvent) {
	if p.onTap == nil || p.imageWidth <= 0 || p.imageHeight <= 0 {
		return
	}
	p.onTap(ev.Position, p.Size(), image.Pt(p.imageWidth, p.imageHeight))
}

func (p *previewPane) TappedSecondary(*fyne.PointEvent) {
	if p.onSecondaryTap == nil {
		return
	}
	p.onSecondaryTap()
}

func (p *previewPane) Dragged(ev *fyne.DragEvent) {
	if p.imageWidth <= 0 || p.imageHeight <= 0 {
		return
	}
	if !p.dragging {
		p.dragStart = ev.Position.Subtract(fyne.NewDelta(ev.Dragged.DX, ev.Dragged.DY))
		p.dragging = true
	}
	p.dragEnd = ev.Position
}

func (p *previewPane) DragEnd() {
	if !p.dragging {
		return
	}
	start := p.dragStart
	end := p.dragEnd
	p.dragging = false
	if p.onSwipe == nil || p.imageWidth <= 0 || p.imageHeight <= 0 {
		return
	}
	if absFloat32(end.X-start.X) < 4 && absFloat32(end.Y-start.Y) < 4 {
		return
	}
	p.onSwipe(start, end, p.Size(), image.Pt(p.imageWidth, p.imageHeight))
}

func (p *previewPane) setImage(img image.Image) {
	p.img.Image = img
	if img == nil {
		p.imageWidth = 0
		p.imageHeight = 0
		p.message.Show()
		p.messageIcon.Hide()
	} else {
		bounds := img.Bounds()
		p.imageWidth = bounds.Dx()
		p.imageHeight = bounds.Dy()
		p.message.Hide()
		p.messageIcon.Hide()
	}
	p.img.Refresh()
	p.Refresh()
}

func (p *previewPane) setMessage(message string) {
	p.img.Image = nil
	p.imageWidth = 0
	p.imageHeight = 0
	p.message.SetText(message)
	if message == "🙈" {
		p.message.Hide()
		p.messageIcon.Text = message
		p.messageIcon.Show()
	} else {
		p.message.Show()
		p.messageIcon.Hide()
	}
	p.img.Refresh()
	p.Refresh()
}

func (r *previewPaneRenderer) Layout(size fyne.Size) {
	r.pane.bg.Move(fyne.NewPos(0, 0))
	r.pane.bg.Resize(size)
	r.pane.img.Move(fyne.NewPos(0, 0))
	r.pane.img.Resize(size)
	centerObject(r.pane.message, size)
	centerObject(r.pane.messageIcon, size)
}

func (r *previewPaneRenderer) MinSize() fyne.Size {
	return r.pane.minSize
}

func (r *previewPaneRenderer) Refresh() {
	for _, obj := range r.objects {
		obj.Refresh()
	}
}

func (r *previewPaneRenderer) Destroy() {}

func (r *previewPaneRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func centerObject(obj fyne.CanvasObject, size fyne.Size) {
	min := obj.MinSize()
	if min.Width > size.Width {
		min.Width = size.Width
	}
	if min.Height > size.Height {
		min.Height = size.Height
	}
	obj.Resize(min)
	obj.Move(fyne.NewPos((size.Width-min.Width)/2, (size.Height-min.Height)/2))
}
