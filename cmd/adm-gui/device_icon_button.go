package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Icon actions retain Button's keyboard/focus behavior and explain themselves
// on hover. Low-frequency actions remain named in the device menu.
type deviceIconButton struct {
	widget.Button
	actionName string
	hint       *fyne.Container
}

func newDeviceIconButton(name string, icon fyne.Resource, tapped func()) *deviceIconButton {
	b := &deviceIconButton{Button: widget.Button{Icon: icon, Importance: widget.LowImportance}, actionName: name}
	b.Button.OnTapped = func() {
		b.hideHint()
		if tapped != nil {
			tapped()
		}
	}
	b.ExtendBaseWidget(b)
	return b
}
func (b *deviceIconButton) CreateRenderer() fyne.WidgetRenderer {
	r := b.Button.CreateRenderer()
	b.ExtendBaseWidget(b)
	label := container.NewThemeOverride(widget.NewLabel(b.actionName), captionTheme{fyne.CurrentApp().Settings().Theme()})
	background := canvas.NewRectangle(theme.OverlayBackgroundColor())
	background.CornerRadius = 4
	b.hint = container.NewStack(background, label)
	b.hint.Hide()
	return &deviceIconRenderer{WidgetRenderer: r, button: b}
}

// Keep the explanation in the button renderer. A PopUp owns the canvas input
// layer, so opening it on MouseIn immediately generates MouseOut on the button.
type deviceIconRenderer struct {
	fyne.WidgetRenderer
	button *deviceIconButton
}

func (r *deviceIconRenderer) Objects() []fyne.CanvasObject {
	return append(append([]fyne.CanvasObject{}, r.WidgetRenderer.Objects()...), r.button.hint)
}
func (r *deviceIconRenderer) Layout(size fyne.Size) {
	r.WidgetRenderer.Layout(size)
	hint := r.button.hint
	hint.Resize(hint.MinSize())
	hint.Move(fyne.NewPos((size.Width-hint.Size().Width)/2, -hint.Size().Height-4))
}
func (b *deviceIconButton) MouseIn(event *desktop.MouseEvent) {
	b.Button.MouseIn(event)
	if !b.Disabled() && b.hint != nil {
		b.hint.Show()
	}
}
func (b *deviceIconButton) MouseOut() { b.hideHint(); b.Button.MouseOut() }
func (b *deviceIconButton) hideHint() {
	if b.hint != nil {
		b.hint.Hide()
	}
}

type deviceActionRowLayout struct{}

func (deviceActionRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := float32(0)
	for _, object := range objects {
		width += object.MinSize().Width
	}
	return fyne.NewSize(width+float32(max(0, len(objects)-1))*4, controlCardButtonRowsHeight)
}
func (deviceActionRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for i, object := range objects {
		width := object.MinSize().Width
		if i == 2 {
			iconsW := float32(0)
			for _, icon := range objects[2:] {
				iconsW += icon.MinSize().Width + 4
			}
			x = max(x, size.Width-iconsW+4)
		}
		object.Move(fyne.NewPos(x, (size.Height-controlCardButtonRowsHeight)/2))
		object.Resize(fyne.NewSize(width, controlCardButtonRowsHeight))
		x += width + 4
	}
}

// The card uses a tighter vertical rhythm than the workbench toolbar.
type deviceCardStackLayout struct{}

func (deviceCardStackLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, float32(max(0, len(objects)-2))*4)
	for _, object := range objects {
		min := object.MinSize()
		size.Width = max(size.Width, min.Width)
		size.Height += min.Height
	}
	return size
}
func (deviceCardStackLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for i, object := range objects {
		height := object.MinSize().Height
		object.Move(fyne.NewPos(0, y))
		object.Resize(fyne.NewSize(size.Width, height))
		y += height
		if i > 0 {
			y += 4
		}
	}
}

// Checkboxes and icon buttons have generous default minimum heights. Keep the
// title row compact while retaining the text baseline and centred hit targets.
type deviceCardHeaderLayout struct{}

func (deviceCardHeaderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := float32(12)
	for _, object := range objects {
		width += object.MinSize().Width
	}
	return fyne.NewSize(width, controlCardHeaderHeight)
}
func (deviceCardHeaderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 4 {
		return
	}
	checkW, statusW, moreW := objects[0].MinSize().Width, objects[2].MinSize().Width, objects[3].MinSize().Width
	widths := []float32{checkW, max(0, size.Width-checkW-statusW-moreW-12), statusW, moreW}
	x := float32(0)
	for i, object := range objects {
		object.Move(fyne.NewPos(x, 0))
		object.Resize(fyne.NewSize(widths[i], controlCardHeaderHeight))
		x += widths[i] + 4
	}
}

// Keep metadata in one compact row without long serials expanding the card.
type deviceCardMetadataLayout struct{}

func (deviceCardMetadataLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 26)
}
func (deviceCardMetadataLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 2 {
		return
	}
	gap := float32(4)
	left := max(0, (size.Width-gap)*.56)
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(left, size.Height))
	objects[1].Move(fyne.NewPos(left+gap, 0))
	objects[1].Resize(fyne.NewSize(max(0, size.Width-left-gap), size.Height))
}
