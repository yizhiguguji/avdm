package main

import (
	"fmt"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// The wall and complete library share the workspace; neither reserves a
// permanent column while the other view is active.
type wallWorkspaceLayout struct{ g *GUIApp }

func (l wallWorkspaceLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(320, 240) }
func (l wallWorkspaceLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 2 {
		return
	}
	online, library := objects[0], objects[1]
	l.g.wallWorkspaceWidth = size.Width
	if grid, ok := l.g.controlGrid.Layout.(*adaptivePreviewLayout); ok {
		grid.viewport = size
	}
	if l.g.wallLibraryMode {
		online.Hide()
		library.Show()
	} else {
		online.Show()
		library.Hide()
	}
	for _, object := range objects {
		object.Move(fyne.NewPos(0, 0))
		object.Resize(size)
	}
}

type previewGeometry struct {
	columns       int
	card, preview fyne.Size
	gap           float32
}

// A resize changes the number of columns, never the chosen device density.
// Keep this helper's signature for callers that compare layout geometry.
func calculatePreviewGeometry(viewport fyne.Size, count int, minWidth, overhead float32) previewGeometry {
	preview := fyne.NewSize(max(1, minWidth-40), max(1, minWidth-40)*1.6)
	for _, spec := range controlDensityOptions {
		if spec.cardSize.Width == minWidth {
			preview = spec.previewSize
			break
		}
	}
	gap := float32(16)
	columns := max(1, int((viewport.Width+gap)/(minWidth+gap)))
	columns = min(columns, max(count, 1))
	return previewGeometry{columns: columns, preview: preview, card: fyne.NewSize(minWidth, preview.Height+overhead), gap: gap}
}

type adaptivePreviewLayout struct {
	g        *GUIApp
	viewport fyne.Size
}

func (l *adaptivePreviewLayout) geometry(objects []fyne.CanvasObject) previewGeometry {
	spec := l.g.controlDensitySpec()
	overhead := max(0, spec.cardSize.Height-spec.previewSize.Height)
	for _, card := range l.g.controlCards {
		if card.wall.visible {
			overhead = max(overhead, card.wall.object.MinSize().Height-card.preview.minSize.Height)
		}
	}
	viewport := l.viewport
	if viewport.Width == 0 {
		viewport.Width = 760
	}
	geom := calculatePreviewGeometry(viewport, len(objects), spec.cardSize.Width, overhead)
	geom.preview = spec.previewSize
	geom.card.Height = spec.previewSize.Height + overhead
	return geom
}
func (l *adaptivePreviewLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(320, 100)
	}
	if l.g.wallOnlineCount == 0 {
		return fyne.NewSize(320, objects[0].MinSize().Height)
	}
	geom := l.geometry(objects)
	rows := int(math.Ceil(float64(len(objects)) / float64(geom.columns)))
	return fyne.NewSize(320, float32(rows)*geom.card.Height+float32(rows-1)*geom.gap)
}
func (l *adaptivePreviewLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	if l.g.wallOnlineCount == 0 {
		for _, object := range objects {
			object.Move(fyne.NewPos(0, 0))
			object.Resize(fyne.NewSize(size.Width, object.MinSize().Height))
		}
		return
	}
	l.viewport.Width = size.Width
	geom := l.geometry(objects)
	for _, card := range l.g.controlCards {
		if card.wall.visible {
			l.g.updateWallCardSize(card, l.g.controlDensitySpec())
		}
	}
	for i, object := range objects {
		row, col := i/geom.columns, i%geom.columns
		object.Move(fyne.NewPos(float32(col)*(geom.card.Width+geom.gap), float32(row)*(geom.card.Height+geom.gap)))
		object.Resize(geom.card)
	}
}

func (g *GUIApp) buildWallWorkspace() fyne.CanvasObject {
	g.controlGrid.Layout = &adaptivePreviewLayout{g: g}
	g.wallLibraryGrid = container.NewVBox()
	// Retained as a status model for callers; it is not an extra workspace row.
	g.wallOnlineLabel = widget.NewLabelWithStyle("在线设备", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	g.wallLibraryButton = compactButton("设备库 (0)", func() {
		g.wallLibraryMode = !g.wallLibraryMode
		g.updateWallViewButton()
		g.renderControlCenter()
		g.wallWorkspace.Refresh()
	})
	g.wallStoppedGrid = container.NewVBox()
	g.wallStoppedPanel = container.New(stableMinWidthLayout{width: 260}, panelSurface("未启动与异常设备", "可直接启动，完整列表见设备库", container.NewVScroll(g.wallStoppedGrid)))
	online := container.NewBorder(nil, nil, nil, g.wallStoppedPanel, container.NewVScroll(g.controlGrid))
	library := container.NewBorder(libraryTableHeader(), nil, nil, nil, container.NewVScroll(g.wallLibraryGrid))
	g.wallWorkspace = container.New(wallWorkspaceLayout{g: g}, online, library)
	return g.wallWorkspace
}

func (g *GUIApp) updateWallViewButton() {
	if g.wallLibraryButton == nil {
		return
	}
	if g.wallLibraryMode {
		g.wallLibraryButton.SetText(fmt.Sprintf("设备墙 (%d)", g.wallOnlineCount))
	} else {
		g.wallLibraryButton.SetText(fmt.Sprintf("设备库 (%d)", g.wallLibraryCount))
	}
	// Returning to the wall remains available even when filtering has no results.
	g.wallLibraryButton.Enable()
}

func (g *GUIApp) updateWallWorkspace(cards, rows []fyne.CanvasObject) {
	g.wallOnlineCount = len(cards)
	g.wallLibraryCount = len(rows)
	g.controlGrid.Objects = cards
	if g.wallLibraryGrid == nil {
		g.controlGrid.Refresh()
		return
	}
	if len(cards) == 0 {
		message := "没有在线设备。连接手机并开启 USB 调试，或从设备库启动模拟器。"
		if len(g.entries) > 0 && len(rows) == 0 {
			message = "没有符合筛选条件的在线设备。"
		}
		openLibrary := compactButton("查看设备库", g.showDeviceLibraryDialog)
		g.controlGrid.Objects = []fyne.CanvasObject{container.NewVBox(wrappedLabel(message), container.NewHBox(openLibrary))}
	}
	g.wallLibraryGrid.Objects = rows
	g.wallLibraryGrid.Refresh()
	g.wallOnlineLabel.SetText(fmt.Sprintf("在线设备 · %d", len(cards)))
	g.updateWallViewButton()
	g.wallWorkspace.Refresh()
	g.controlGrid.Refresh()
}

// Older callers use this name; the library now occupies the same workbench.
func (g *GUIApp) showDeviceLibraryDialog() {
	g.wallLibraryMode = true
	g.updateWallViewButton()
	g.renderControlCenter()
	if g.wallWorkspace != nil {
		g.wallWorkspace.Refresh()
	}
}

// Kept for existing toolbar construction, with no reading-width cap or centering.
type workbenchWidthLayout struct{}

func (workbenchWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(320, 240)
	}
	return fyne.NewSize(320, objects[0].MinSize().Height)
}
func (workbenchWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		object.Move(fyne.NewPos(0, 0))
		object.Resize(size)
	}
}
