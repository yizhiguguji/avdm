package main

import (
	core "adm/internal/app"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Use the log's selection and read-only behavior without its monospace style
// or external scrolling. Long messages scroll inside a bounded dialog.
func newSelectableMessage(message string) *selectableLog {
	body := newSelectableLog()
	body.menuTitle = "文本"
	body.Wrapping = fyne.TextWrapWord
	body.Scroll = fyne.ScrollVerticalOnly
	body.TextStyle = fyne.TextStyle{}
	body.SetText(message)
	return body
}

func (g *GUIApp) dialogMessageContent(message string, size fyne.Size) fyne.CanvasObject {
	body := newSelectableMessage(message)
	copyButton := widget.NewButton("复制全部", func() {
		g.app.Clipboard().SetContent(message)
	})
	footer := container.NewBorder(nil, nil, mutedText("拖选或 ⌘A / ⌘C 复制"), copyButton)
	return container.NewGridWrap(size, container.NewBorder(nil, footer, nil, nil, body))
}

func toolHealthDetails(statuses []core.ToolStatus, bootstrap core.DependencyBootstrapStatus) string {
	var lines []string
	for _, status := range statuses {
		state := "可用"
		if !status.Available {
			state = "不可用"
		}
		lines = append(lines, fmt.Sprintf("%s：%s\n路径：%s\n来源：%s\n错误：%s", status.Name, state, status.Path, status.Source, status.Error))
	}
	lines = append(lines, "安装脚本："+bootstrap.Script)
	if bootstrap.Error != "" {
		lines = append(lines, "安装入口错误："+bootstrap.Error)
	}
	return strings.Join(lines, "\n\n")
}
