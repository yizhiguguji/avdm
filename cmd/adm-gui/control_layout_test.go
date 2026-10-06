package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"testing"
)

func TestControlButtonsReserveEveryRow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	for _, density := range controlDensityOptions {
		t.Run(density.key, func(t *testing.T) {
			var buttons []fyne.CanvasObject
			for _, label := range []string{"主目标", "独立窗", "置顶独立窗", "不看", "关机", "主页", "返回", "通知", "管理"} {
				buttons = append(buttons, widget.NewButton(label, nil))
			}
			rows := controlButtonRows(buttons, density.cardSize.Width-16).(*fyne.Container)
			card := controlCardSurface(container.NewVBox(widget.NewLabel("Pixel_10_Pro"), container.NewGridWrap(density.previewSize, widget.NewLabel("preview")), rows))
			size := density.cardSize
			if size.Height < card.MinSize().Height {
				size.Height = card.MinSize().Height
			}
			card.Resize(size)
			rows.Resize(fyne.NewSize(density.cardSize.Width-16, rows.MinSize().Height))
			for _, button := range buttons {
				if button.Position().Y+button.Size().Height > rows.Size().Height+0.1 || button.Position().X+button.Size().Width > rows.Size().Width+0.1 {
					t.Fatalf("button outside rows: pos=%v size=%v rows=%v", button.Position(), button.Size(), rows.Size())
				}
			}
		})
	}
}

func TestControlToolbarKeepsDensityAndActionsSeparateFromHeading(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	for _, standalone := range []bool{false, true} {
		t.Run(fmt.Sprintf("standalone=%t", standalone), func(t *testing.T) {
			summary := widget.NewLabel("运行中 12 台设备 | 已选择 8 台")
			summary.Wrapping = fyne.TextTruncate
			var heading fyne.CanvasObject = container.NewBorder(nil, nil, widget.NewLabel("设备墙"), nil, summary)
			if standalone {
				heading = container.NewVBox(widget.NewLabel("设备墙"), summary)
			}
			selector := widget.NewSelect(controlDensityLabels(), nil)
			objects := []fyne.CanvasObject{compactSelectBox(selector, controlDensitySelectWidth)}
			for _, label := range []string{"外部窗", "扫描", "画面", "可用", "清空", "启动", "关机"} {
				objects = append(objects, compactButtonBox(compactButton(label, nil), 66))
			}
			actions := container.NewHBox(objects...)
			toolbar := controlToolbar(heading, actions)
			window := test.NewWindow(toolbar)
			defer window.Close()
			scroll := toolbar.Objects[0].(*container.Scroll)
			for _, width := range []float32{1200, 320, 500, 800, 320} {
				for _, density := range controlDensityOptions {
					selector.SetSelected(density.label)
					toolbar.Resize(fyne.NewSize(width, toolbar.MinSize().Height))
					toolbar.Refresh()
					if scroll.Position().Y < heading.Position().Y+heading.Size().Height {
						t.Fatalf("actions overlap heading at width %v", width)
					}
					if scroll.Position().X < 0 || scroll.Position().X+scroll.Size().Width > width+0.1 {
						t.Fatalf("actions viewport outside toolbar at width %v: pos=%v size=%v", width, scroll.Position(), scroll.Size())
					}
					for i, object := range objects {
						if i > 0 && object.Position().X < objects[i-1].Position().X+objects[i-1].Size().Width {
							t.Fatalf("controls overlap after selecting %s at width %v", density.label, width)
						}
					}
				}
			}
		})
	}
}
