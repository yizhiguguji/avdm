package main

import (
	core "adm/internal/app"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestToolDiagnosticsCanUpdateWithoutDeviceState(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	g := &GUIApp{toolSummary: widget.NewLabel("工具：检测中")}
	statuses := []core.ToolStatus{{Name: "adb", Error: "未找到可执行文件"}}
	g.applyToolStatuses(statuses)
	if g.toolSummary.Text != "工具不可用：adb" {
		t.Fatal(g.toolSummary.Text)
	}
	if len(g.toolStatuses) != 1 || !strings.Contains(g.toolDetails, "未找到可执行文件") {
		t.Fatal("missing failure diagnostics")
	}
}

func TestToolActionSummaryNamesRequiredAction(t *testing.T) {
	statuses := []core.ToolStatus{{Name: "adb", Available: true}, {Name: "macOS 辅助功能", Error: "未授权"}}
	if got := toolActionSummary(statuses); got != "辅助功能未授权" {
		t.Fatal(got)
	}
	statuses[0].Available = false
	if got := toolActionSummary(statuses); got != "工具不可用：adb；辅助功能未授权" {
		t.Fatal(got)
	}
	for i := range statuses {
		statuses[i].Available = true
	}
	if got := toolActionSummary(statuses); got != "工具：已就绪" {
		t.Fatal(got)
	}
}
