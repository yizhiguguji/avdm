package main

import (
	core "adm/internal/app"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func menuGroup(label string, menu *fyne.Menu) *fyne.MenuItem {
	item := fyne.NewMenuItem(label, nil)
	item.ChildMenu = menu
	return item
}

func (g *GUIApp) selectionMenu() *fyne.Menu {
	return fyne.NewMenu("选择",
		fyne.NewMenuItem("选中当前显示的设备", g.selectVisibleWallEntries),
		fyne.NewMenuItem("选中显示的在线设备", func() { g.selectVisibleWallState(wallStateReady) }),
		fyne.NewMenuItem("选中显示的未启动模拟器", func() { g.selectVisibleWallState(wallStateStopped) }),
		fyne.NewMenuItem("清除全部选择", g.clearWorkbenchSelection),
	)
}

func (g *GUIApp) requireSelection(action func()) func() {
	return func() {
		if len(g.selectedControlEntries()) == 0 {
			g.showInfo("请先勾选要操作的设备。")
			return
		}
		action()
	}
}

// This complete menu is shared by the top overflow and batch bar. Its order
// does not depend on window width or on which inline buttons happen to fit.
func (g *GUIApp) selectedDeviceMenu() *fyne.Menu {
	return fyne.NewMenu("选中设备操作",
		fyne.NewMenuItem("查看选中设备", g.showSelectedDeviceDetails),
		fyne.NewMenuItem("打开选中设备镜像", g.requireSelection(g.openExternalDeviceWindows)),
		fyne.NewMenuItem("启动选中模拟器", g.startWorkbenchSelection),
		fyne.NewMenuItem("重启选中设备", g.requireSelection(g.rebootCurrentDevice)),
		fyne.NewMenuItem("关闭选中模拟器", g.stopWorkbenchSelection),
		fyne.NewMenuItem("设为主目标", g.setWorkbenchTarget),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("删除选中模拟器", g.showSelectedDeleteAVDDialog),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("清除全部选择", g.clearWorkbenchSelection),
	)
}

func (g *GUIApp) workbenchMoreMenu() *fyne.Menu {
	view := fyne.NewMenu("视图",
		fyne.NewMenuItem("刷新设备画面", g.refreshControlScreensAsync),
		fyne.NewMenuItem("打开设备镜像", g.openExternalDeviceWindows),
		fyne.NewMenuItem("切换设备墙与设备库", func() {
			g.wallLibraryMode = !g.wallLibraryMode
			g.updateWallViewButton()
			g.renderControlCenter()
			g.wallWorkspace.Refresh()
		}),
	)
	density := fyne.NewMenu("卡片大小")
	for _, spec := range controlDensityOptions {
		value := spec.key
		item := fyne.NewMenuItem(spec.label, func() { g.setControlDensity(value) })
		item.Checked = g.controlDensity == value
		density.Items = append(density.Items, item)
	}
	view.Items = append(view.Items, menuGroup("卡片大小", density))
	diagnostics := fyne.NewMenu("诊断",
		fyne.NewMenuItem("检查工具", g.showToolHealthDialog),
		fyne.NewMenuItem("设备统计", func() { g.showInfo(controlSummaryText(g.entries)) }),
		fyne.NewMenuItem("展开或收起日志", func() { g.logCollapsed = !g.logCollapsed; g.applyWorkbenchCollapseState() }),
	)
	return fyne.NewMenu("更多",
		menuGroup("视图", view),
		fyne.NewMenuItem("创建模拟器", g.showCreateAVDDialog),
		menuGroup("选择", g.selectionMenu()),
		menuGroup("选中设备操作", g.selectedDeviceMenu()),
		fyne.NewMenuItemSeparator(),
		menuGroup("诊断", diagnostics),
	)
}

func (g *GUIApp) showSelectedDeviceDetails() {
	entries := g.selectedControlEntries()
	if len(entries) == 0 {
		g.showInfo("尚未勾选设备。")
		return
	}
	lines := []string{fmt.Sprintf("已勾选 %d 台设备：", len(entries))}
	for i, entry := range entries {
		lines = append(lines, fmt.Sprintf("%d. %s\n   编号：%s · %s", i+1, controlCardTitle(entry), controlCopyIdentifier(entry), workbenchDeviceState(entry)))
	}
	g.confirmActionWithLabels("选中设备", strings.Join(lines, "\n"), "关闭", "返回", func() {})
}

func (g *GUIApp) buildBatchActionBar() fyne.CanvasObject {
	g.batchScopeLabel = widget.NewLabel("已勾选 0 台")
	g.batchScopeLabel.TextStyle.Bold = true
	g.batchScopeLabel.Truncation = fyne.TextTruncateEllipsis
	scope := container.NewStack(g.batchScopeLabel, newCopyTapLayer(g.showSelectedDeviceDetails))
	start := compactButton("启动模拟器", g.startWorkbenchSelection)
	stop := compactButton("关闭模拟器", g.stopWorkbenchSelection)
	remove := compactButton("删除模拟器", g.showSelectedDeleteAVDDialog)
	clear := compactButton("清除选择", g.clearWorkbenchSelection)
	more := newReadableButton("更多操作", nil)
	more.SetIcon(theme.MenuDropDownIcon())
	more.OnTapped = func() { g.showWorkbenchMenu(more, g.selectedDeviceMenu()) }
	g.registerActionButtons(start, stop, remove)
	return container.New(workbenchRowInsetLayout{vertical: 4}, container.New(batchActionLayout{}, scope, start, stop, remove, clear, more))
}

// Preserve full action labels. Lower-priority inline actions move into the
// complete menu rather than shrinking, clipping or changing their wording.
type batchActionLayout struct{}

func (batchActionLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(320, controlCompactControlHeight)
}
func (batchActionLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 6 {
		return
	}
	gap := theme.Padding()
	widths := make([]float32, 6)
	for i := 1; i < 6; i++ {
		widths[i] = objects[i].MinSize().Width
	}
	show := []bool{true, false, false, false, true, true}
	used := widths[4] + widths[5] + 2*gap
	for _, i := range []int{1, 2, 3} {
		if used+gap+widths[i]+120 <= size.Width {
			show[i] = true
			used += gap + widths[i]
		}
	}
	scopeWidth := max(0, size.Width-used)
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(scopeWidth, size.Height))
	x := scopeWidth + gap
	for i := 1; i < 6; i++ {
		if !show[i] {
			objects[i].Hide()
			continue
		}
		objects[i].Show()
		objects[i].Move(fyne.NewPos(x, (size.Height-controlCompactControlHeight)/2))
		objects[i].Resize(fyne.NewSize(widths[i], controlCompactControlHeight))
		x += widths[i] + gap
	}
}

// Shared text for every device-local destructive action.
func deviceCloseActionLabel(entry core.DeviceEntry) string {
	if entry.Active != nil && !entry.Active.IsEmulator {
		if physicalPowerTarget(entry) {
			return "关闭此真机"
		}
		return "断开此网络设备"
	}
	return "关闭此模拟器"
}
