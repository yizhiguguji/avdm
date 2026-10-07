package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"math"
)

// The device library has its own scroll area. At narrow widths it opens in a
// dialog, leaving the entire workbench available for online previews.
type wallWorkspaceLayout struct{ g *GUIApp }

func (l wallWorkspaceLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(320, 240) }
func (l wallWorkspaceLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 2 {
		return
	}
	online, library := objects[0], objects[1]
	g := l.g
	g.wallWorkspaceWidth = size.Width
	if g.wallOnlineCount == 0 && g.wallLibraryCount > 0 {
		online.Hide()
		library.Show()
		width := min(size.Width, 620)
		library.Move(fyne.NewPos((size.Width-width)/2, 0))
		library.Resize(fyne.NewSize(width, size.Height))
		return
	}
	online.Show()
	width := size.Width
	if size.Width >= 900 && g.wallLibraryCount > 0 && !g.libraryCollapsed {
		library.Show()
		libraryWidth := float32(300)
		width -= libraryWidth + 12
		library.Move(fyne.NewPos(width+12, 0))
		library.Resize(fyne.NewSize(libraryWidth, size.Height))
	} else {
		library.Hide()
	}
	online.Move(fyne.NewPos(0, 0))
	if grid, ok := g.controlGrid.Layout.(*adaptivePreviewLayout); ok {
		grid.viewport = fyne.NewSize(width, size.Height)
	}
	online.Resize(fyne.NewSize(width, size.Height))
}

type previewGeometry struct {
	columns       int
	card, preview fyne.Size
	gap           float32
}

func calculatePreviewGeometry(viewport fyne.Size, count int, minWidth, overhead float32) previewGeometry {
	gap := float32(12)
	width := max(viewport.Width, 320)
	columns := max(1, int((width+gap)/(minWidth+gap)))
	columns = min(columns, max(count, 1))
	cellWidth := (width - gap*float32(columns-1)) / float32(columns)
	height := max(240, min(viewport.Height-overhead-24, (cellWidth-20)/0.50))
	height = min(height, 760)
	preview := fyne.NewSize(height*0.50, height)
	return previewGeometry{columns: columns, preview: preview, card: fyne.NewSize(min(cellWidth, max(minWidth, preview.Width+20)), height+overhead), gap: gap}
}

type adaptivePreviewLayout struct {
	g        *GUIApp
	viewport fyne.Size
}

func (l *adaptivePreviewLayout) geometry(objects []fyne.CanvasObject) previewGeometry {
	overhead := float32(96)
	for _, card := range l.g.controlCards {
		if card.wall.visible {
			overhead = max(overhead, card.wall.object.MinSize().Height-card.preview.minSize.Height)
		}
	}
	viewport := l.viewport
	if viewport.Width == 0 {
		viewport = fyne.NewSize(760, 640)
	}
	return calculatePreviewGeometry(viewport, len(objects), l.g.controlDensitySpec().cardSize.Width, overhead)
}
func (l *adaptivePreviewLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(320, 100)
	}
	geom := l.geometry(objects)
	rows := int(math.Ceil(float64(len(objects)) / float64(geom.columns)))
	return fyne.NewSize(320, float32(rows)*geom.card.Height+float32(rows-1)*geom.gap+16)
}
func (l *adaptivePreviewLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	geom := l.geometry(objects)
	for _, card := range l.g.controlCards {
		if card.wall.visible {
			spec := l.g.controlDensitySpec()
			spec.previewSize = geom.preview
			l.g.updateWallCardSize(card, spec)
		}
	}
	for i, obj := range objects {
		row, col := i/geom.columns, i%geom.columns
		inRow := min(geom.columns, len(objects)-row*geom.columns)
		rowWidth := float32(inRow)*geom.card.Width + float32(inRow-1)*geom.gap
		x := max(0, (size.Width-rowWidth)/2) + float32(col)*(geom.card.Width+geom.gap)
		rows := (len(objects) + geom.columns - 1) / geom.columns
		totalHeight := float32(rows)*geom.card.Height + float32(rows-1)*geom.gap
		y := max(8, (l.viewport.Height-totalHeight)/2)
		obj.Move(fyne.NewPos(x, y+float32(row)*(geom.card.Height+geom.gap)))
		obj.Resize(geom.card)
	}
}

func (g *GUIApp) buildWallWorkspace() fyne.CanvasObject {
	g.controlGrid.Layout = &adaptivePreviewLayout{g: g}
	g.wallLibraryGrid = container.NewVBox()
	g.wallOnlineLabel = widget.NewLabelWithStyle("在线设备", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	g.wallLibraryButton = compactButton("设备库", func() {
		if g.wallWorkspaceWidth < 900 {
			g.showDeviceLibraryDialog()
			return
		}
		g.libraryCollapsed = !g.libraryCollapsed
		g.wallWorkspace.Refresh()
	})
	online := container.NewBorder(nil, nil, nil, nil, container.NewVScroll(g.controlGrid))
	library := panelSurface("设备库", "未启动及异常设备", container.NewVScroll(g.wallLibraryGrid))
	g.wallWorkspace = container.New(wallWorkspaceLayout{g: g}, online, library)
	header := container.NewBorder(nil, nil, g.wallOnlineLabel, g.wallLibraryButton, nil)
	return container.NewBorder(header, nil, nil, nil, g.wallWorkspace)
}

func (g *GUIApp) updateWallWorkspace(cards, rows []fyne.CanvasObject) {
	g.wallOnlineCount = len(cards)
	g.wallLibraryCount = len(rows)
	g.controlGrid.Objects = cards
	if g.wallLibraryGrid == nil { // legacy unit fixtures
		g.controlGrid.Refresh()
		return
	}
	g.wallLibraryGrid.Objects = rows
	g.wallLibraryGrid.Refresh()
	g.wallOnlineLabel.SetText(fmt.Sprintf("在线设备 · %d", len(cards)))
	g.wallLibraryButton.SetText(fmt.Sprintf("设备库 (%d)", len(rows)))
	if len(rows) == 0 {
		g.wallLibraryButton.Disable()
	} else {
		g.wallLibraryButton.Enable()
	}
	g.wallWorkspace.Refresh()
	g.controlGrid.Refresh()
}

func (g *GUIApp) showDeviceLibraryDialog() {
	var rows []fyne.CanvasObject
	for _, entry := range g.wallVisibleEntries() {
		if !readyWallEntry(entry) {
			rows = append(rows, g.buildControlCompactRow(entry))
		}
	}
	if len(rows) == 0 {
		g.showInfo("没有未启动或异常设备。")
		return
	}
	content := container.NewVScroll(container.NewVBox(rows...))
	content.SetMinSize(fyne.NewSize(500, 360))
	d := dialog.NewCustom("设备库", "关闭", content, g.activeDialogWindow())
	d.SetOnClosed(g.renderControlCenter)
	d.Show()
}

// Cap reading width on very large displays instead of magnifying a phone to
// an entire monitor. The workspace still responds to tool and log panes.
type workbenchWidthLayout struct{}

func (workbenchWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(320, 240)
	}
	return fyne.NewSize(320, objects[0].MinSize().Height)
}
func (workbenchWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, obj := range objects {
		width := min(size.Width, 1600)
		obj.Move(fyne.NewPos((size.Width-width)/2, 0))
		obj.Resize(fyne.NewSize(width, size.Height))
	}
}
