package main

import (
	core "adm/internal/app"
	"errors"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestDialogMessageSelectionCopyAndReadOnly(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	message := "打开外部窗口失败：\n" + strings.Repeat("设备编号、错误详情与完整路径 /Applications/安卓设备矩阵.app\n", 100)
	body := newSelectableMessage(message)
	w := a.NewWindow("错误")
	w.SetContent(body)
	w.Resize(fyne.NewSize(640, 220))
	w.Show()
	defer w.Close()
	body.TypedShortcut(&fyne.ShortcutSelectAll{})
	if body.SelectedText() != message {
		t.Fatal("long message cannot be fully selected")
	}
	body.TypedShortcut(&fyne.ShortcutCopy{Clipboard: a.Clipboard()})
	if a.Clipboard().Content() != message {
		t.Fatal("copy truncated or changed the error details")
	}
	body.TypedRune('x')
	body.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
	body.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	body.TypedShortcut(&fyne.ShortcutCut{Clipboard: a.Clipboard()})
	body.TypedShortcut(&fyne.ShortcutPaste{Clipboard: a.Clipboard()})
	if body.Text != message {
		t.Fatal("dialog message was editable")
	}
}

func TestDialogCopyAllPreservesFullMessageWithoutSelection(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	g := &GUIApp{app: a}
	message := "  错误：第一行\n\n完整路径与诊断信息\n"
	content := g.dialogMessageContent(message, fyne.NewSize(640, 220))
	copy := cardButtons(content)["复制全部"]
	if copy == nil {
		t.Fatal("dialog has no copy-all action")
	}
	copy.OnTapped()
	if a.Clipboard().Content() != message {
		t.Fatal("copy-all changed the original whitespace or content")
	}
}

func TestToolHealthCopyKeepsUnabridgedDiagnostics(t *testing.T) {
	path := "/opt/homebrew/share/android-commandlinetools/cmdline-tools/latest/bin/avdmanager"
	err := "无法运行工具：\n第二行诊断"
	details := toolHealthDetails([]core.ToolStatus{{Name: "avdmanager", Path: path, Source: "Android SDK", Error: err}}, core.DependencyBootstrapStatus{Script: "/Applications/安卓设备矩阵.app/Contents/Resources/install-macos-deps.sh"})
	for _, want := range []string{path, err, "avdmanager：不可用", "安装脚本：/Applications/"} {
		if !strings.Contains(details, want) {
			t.Fatalf("copied diagnostics missing %q", want)
		}
	}
}

func TestEmptyDeviceWallOffersRepairUntilDependenciesAreReady(t *testing.T) {
	g := testWall(t)
	g.toolStatuses = []core.ToolStatus{{Name: "adb", Error: "未找到可执行文件"}, {Name: "avdmanager", Error: "未找到可执行文件"}}
	content := emptyDeviceWall(g)
	buttons := cardButtons(content)
	if buttons["安装/修复依赖"] == nil || buttons["安装/修复依赖"].OnTapped == nil {
		t.Fatal("missing dependencies have no repair action")
	}
	if buttons["创建模拟器"] != nil {
		t.Fatal("failed detection offers device creation instead of dependency repair")
	}
	for i := range g.toolStatuses {
		g.toolStatuses[i].Available = true
	}
	buttons = cardButtons(emptyDeviceWall(g))
	if buttons["安装/修复依赖"] != nil || buttons["创建模拟器"] == nil || buttons["重新扫描"] == nil {
		t.Fatal("healthy empty state was not restored after dependency repair")
	}
}

func TestDetectionErrorRepairOpensInstallerConfirmation(t *testing.T) {
	t.Chdir("../..")
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("设备检测")
	w.SetContent(container.NewVBox())
	w.Resize(fyne.NewSize(1000, 700))
	w.Show()
	defer w.Close()
	g := &GUIApp{app: a, window: w, backend: &core.App{}, toolStatuses: []core.ToolStatus{{Name: "adb"}}}
	g.showDeviceDetectionError(errors.New("adb 不可用：未找到可执行文件"))
	overlay := w.Canvas().Overlays().Top()
	buttons := cardButtons(overlay.(*widget.PopUp).Content)
	if buttons["安装/修复依赖"] == nil {
		t.Fatal("device detection error has no repair button")
	}
	test.Tap(buttons["安装/修复依赖"])
	next := w.Canvas().Overlays().Top()
	if next == nil || next == overlay || cardButtons(next.(*widget.PopUp).Content)["确认"] == nil {
		t.Fatal("repair action did not replace the error with installer confirmation")
	}
}
