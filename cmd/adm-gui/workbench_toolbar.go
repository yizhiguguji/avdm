package main

import (
	core "adm/internal/app"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// TargetSelection keeps explicit selection separate from the optional default
// scope. An invalid explicit selection must never expand to other devices.
type TargetSelection struct {
	Entries  []core.DeviceEntry
	Explicit bool
}

type actionKind string

const (
	actionMirror    actionKind = "外部窗"
	actionStart     actionKind = "启动模拟器"
	actionStop      actionKind = "关闭模拟器"
	actionSetTarget actionKind = "设为主目标"
	actionDelete    actionKind = "删除模拟器"
	actionInstall   actionKind = "安装应用"
)

type actionSkip struct {
	Entry  core.DeviceEntry
	Reason string
}

type actionPlan struct {
	Kind     actionKind
	Entries  []core.DeviceEntry
	Skipped  []actionSkip
	Explicit bool
	Default  bool
}

func (g *GUIApp) targetSelection() TargetSelection {
	selection := TargetSelection{}
	for _, selected := range g.controlSelected {
		selection.Explicit = selection.Explicit || selected
	}
	for _, entry := range g.entries {
		if g.controlSelected[entry.Key] {
			selection.Entries = append(selection.Entries, entry)
		}
	}
	return selection
}

func (g *GUIApp) planSelectedAction(kind actionKind, fallback bool) actionPlan {
	return makeActionPlan(kind, g.targetSelection(), g.entries, fallback)
}

func makeActionPlan(kind actionKind, selection TargetSelection, all []core.DeviceEntry, fallback bool) actionPlan {
	plan := actionPlan{Kind: kind, Explicit: selection.Explicit, Default: !selection.Explicit && fallback}
	entries := selection.Entries
	if !selection.Explicit && fallback {
		entries = all
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if seen[entry.Key] {
			continue
		}
		seen[entry.Key] = true
		if reason := actionUnavailableReason(kind, entry); reason != "" {
			plan.Skipped = append(plan.Skipped, actionSkip{Entry: entry, Reason: reason})
		} else {
			plan.Entries = append(plan.Entries, entry)
		}
	}
	// A primary target is an explicit single-device choice, even when only
	// one of multiple selected devices happens to be usable.
	if kind == actionSetTarget && (len(selection.Entries) != 1 || !selection.Explicit) {
		for _, entry := range plan.Entries {
			plan.Skipped = append(plan.Skipped, actionSkip{Entry: entry, Reason: "主目标需要仅勾选一台在线设备"})
		}
		plan.Entries = nil
	}
	return plan
}

func actionUnavailableReason(kind actionKind, entry core.DeviceEntry) string {
	ready := entry.Active != nil && entry.Active.State == "device"
	switch kind {
	case actionMirror, actionInstall, actionSetTarget:
		if !ready {
			return "设备未在线"
		}
	case actionStart:
		if entry.AVD == nil {
			return "真机无法启动模拟器"
		}
		if entry.Running || entry.Active != nil {
			return "模拟器已启动"
		}
	case actionStop:
		if (entry.Active == nil || !entry.Active.IsEmulator) && !(entry.AVD != nil && entry.Running) {
			return "仅支持运行中的模拟器"
		}
	case actionDelete:
		if entry.AVD == nil {
			return "真机不能删除"
		}
	default:
		return "未知操作"
	}
	return ""
}

func (p actionPlan) keys() []string {
	keys := make([]string, 0, len(p.Entries))
	for _, entry := range p.Entries {
		keys = append(keys, entry.Key)
	}
	return keys
}

func (p actionPlan) avdNames() []string {
	names := make([]string, 0, len(p.Entries))
	for _, entry := range p.Entries {
		if entry.AVD != nil {
			names = append(names, entry.AVD.Name)
		}
	}
	return names
}

func (p actionPlan) summary() string {
	scope := "勾选"
	if !p.Explicit {
		scope = "未选择"
		if p.Default {
			scope = "默认全部"
		}
	}
	return fmt.Sprintf("%s：%s范围，目标 %d 项，跳过 %d 项", p.Kind, scope, len(p.Entries), len(p.Skipped))
}

func (p actionPlan) details() string {
	lines := []string{p.summary()}
	for _, entry := range p.Entries {
		lines = append(lines, "目标："+entry.Label)
	}
	for _, skipped := range p.Skipped {
		lines = append(lines, "跳过："+skipped.Entry.Label+"（"+skipped.Reason+"）")
	}
	return strings.Join(lines, "\n")
}

func (g *GUIApp) acceptActionPlan(plan actionPlan) bool {
	if len(plan.Entries) == 0 {
		g.showInfo("没有符合条件的操作目标。\n\n" + plan.details() + "\n\n请检查勾选设备及设备状态。")
		return false
	}
	if len(plan.Skipped) > 0 {
		g.appendLog("WARN", "%s", plan.details())
	}
	return true
}

const (
	wallStateAll      = "全部"
	wallStateReady    = "在线"
	wallStateStopped  = "未启动"
	wallStateStarting = "启动中"
	wallStateError    = "异常"
)

// Each device belongs to exactly one state. A running process awaiting adb
// is starting, not stopped, and an attached unauthorized device is abnormal.
func workbenchDeviceState(entry core.DeviceEntry) string {
	if entry.Active != nil {
		if entry.Active.State == "device" {
			return wallStateReady
		}
		return wallStateError
	}
	if entry.Running {
		return wallStateStarting
	}
	return wallStateStopped
}

func filterWallEntries(entries []core.DeviceEntry, search, state string) []core.DeviceEntry {
	query := strings.ToLower(strings.TrimSpace(search))
	result := make([]core.DeviceEntry, 0, len(entries))
	for _, entry := range entries {
		if state != "" && state != wallStateAll && workbenchDeviceState(entry) != state {
			continue
		}
		identity := entry.Label + " " + entry.Key
		if entry.AVD != nil {
			identity += " " + entry.AVD.Name + " " + entry.AVD.Device
		}
		if entry.Active != nil {
			identity += " " + entry.Active.Serial + " " + entry.Active.Details["model"] + " " + entry.Active.AVDName
		}
		if query != "" && !strings.Contains(strings.ToLower(identity), query) {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func (g *GUIApp) wallVisibleEntries() []core.DeviceEntry {
	return filterWallEntries(g.entries, g.wallSearch, g.wallStateFilter)
}

func controlSummaryText(entries []core.DeviceEntry) string {
	counts := map[string]int{}
	for _, entry := range entries {
		counts[workbenchDeviceState(entry)]++
	}
	return fmt.Sprintf("共 %d 项 · 在线 %d · 未启动 %d · 启动中 %d · 异常 %d", len(entries), counts[wallStateReady], counts[wallStateStopped], counts[wallStateStarting], counts[wallStateError])
}

func (g *GUIApp) buildDeviceWallPanel() fyne.CanvasObject {
	if g.controlSelected == nil {
		g.controlSelected = map[string]bool{}
	}
	if g.controlHidden == nil {
		g.controlHidden = map[string]bool{}
	}
	if g.controlDensity == "" {
		g.controlDensity = g.app.Preferences().StringWithFallback("wall.density", controlDensitySmall)
	}
	g.controlSummary = widget.NewLabel("")
	g.controlGrid = container.NewVBox()
	g.controlCards = map[string]*controlCardView{}
	g.controlRealtimeStops = map[string]func(){}

	scan := compactButton("扫描", func() { g.refreshAsync(false) })
	scan.SetIcon(theme.ViewRefreshIcon())
	frames := compactButton("画面", g.refreshControlScreensAsync)
	var view *widget.Button
	view = compactButton("视图", func() { g.showWallViewDialog(view) })
	external := compactButton("外部窗", g.openExternalDeviceWindows)
	create := compactButton("创建", g.showCreateAVDDialog)
	create.SetIcon(theme.ContentAddIcon())
	start := compactButton("启动选中", g.startWorkbenchSelection)
	selection := compactButton("选择", nil)
	selection.SetIcon(theme.MenuDropDownIcon())
	selectionMenu := fyne.NewMenu("选择",
		fyne.NewMenuItem("选中当前显示", g.selectVisibleWallEntries),
		fyne.NewMenuItem("选中显示的在线设备", func() { g.selectVisibleWallState(wallStateReady) }),
		fyne.NewMenuItem("选中显示的未启动模拟器", func() { g.selectVisibleWallState(wallStateStopped) }),
		fyne.NewMenuItem("清空全部选择", func() {
			g.controlSelected = map[string]bool{}
			g.updateSelectedLabel()
			g.renderControlCenter()
		}),
	)
	selection.OnTapped = func() { g.showWorkbenchMenu(selection, selectionMenu) }
	more := compactButton("更多", nil)
	more.SetIcon(theme.MenuDropDownIcon())
	more.OnTapped = func() {
		menu := fyne.NewMenu("更多")
		// Responsive layout puts hidden frequent actions here, preserving a
		// complete keyboard-accessible path at narrow workbench widths.
		for _, item := range []struct {
			button *widget.Button
			label  string
		}{{frames, "刷新设备画面"}, {external, "打开外部窗口"}, {create, "创建模拟器"}, {selection, "选择设备"}, {start, "启动选中模拟器"}} {
			if !item.button.Visible() {
				button := item.button
				if button == selection {
					menu.Items = append(menu.Items, fyne.NewMenuItem(item.label, func() { g.showWorkbenchMenu(more, selectionMenu) }))
				} else {
					menu.Items = append(menu.Items, fyne.NewMenuItem(item.label, button.OnTapped))
				}
			}
		}
		menu.Items = append(menu.Items,
			fyne.NewMenuItem("关闭选中模拟器…", g.stopWorkbenchSelection),
			fyne.NewMenuItem("删除选中模拟器…", g.showSelectedDeleteAVDDialog),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("设备统计", func() { g.showInfo(controlSummaryText(g.entries)) }),
		)
		g.showWorkbenchMenu(more, menu)
	}
	g.registerActionButtons(scan, frames, external, create, start, selection, more)

	buttons := []*widget.Button{scan, frames, view, external, create, selection, start, more}
	objects := make([]fyne.CanvasObject, len(buttons))
	for i, button := range buttons {
		objects[i] = button
	}
	toolbar := container.New(workbenchToolbarLayout{}, objects...)
	workspace := g.buildWallWorkspace()
	search := newCenteredSearchEntry()
	search.OnChanged = func(value string) {
		g.wallSearch = strings.TrimSpace(value)
		g.renderControlCenter()
	}
	g.wallSearchEntry = &search.Entry
	density := newCenteredDensitySelect([]string{"画面：小", "画面：标准", "画面：高清"}, func(label string) {
		g.setControlDensity(controlDensityKeyByLabel(strings.TrimPrefix(label, "画面：")))
	})
	g.controlDensitySelect = &density.Select
	g.controlDensitySelect.SetSelected("画面：" + controlDensityLabel(g.controlDensity))
	left := container.New(centeredRowLayout{}, g.wallLibraryButton, container.New(centeredControlLayout{width: 112}, density), container.New(centeredControlLayout{width: 160}, container.NewThemeOverride(search, toolbarSearchTheme{g.app.Settings().Theme()})))
	row := container.NewBorder(nil, nil, left, nil, toolbar)
	return container.New(flexibleMinWidthLayout{width: deviceWallPanelMinWidth}, container.NewBorder(topSurface(container.New(workbenchRowInsetLayout{vertical: theme.Padding()}, row)), nil, nil, nil, container.NewPadded(workspace)))
}

func (g *GUIApp) showWorkbenchMenu(button *widget.Button, menu *fyne.Menu) {
	driver := g.app.Driver()
	popup := widget.NewPopUpMenu(menu, driver.CanvasForObject(button))
	position := driver.AbsolutePositionForObject(button)
	popup.ShowAtPosition(position.Add(fyne.NewPos(0, button.Size().Height)))
}

func (g *GUIApp) showWallViewDialog(view *widget.Button) {
	search := widget.NewEntry()
	search.SetPlaceHolder("设备名称、编号或型号")
	search.SetText(g.wallSearch)
	state := widget.NewSelect([]string{wallStateAll, wallStateReady, wallStateStopped, wallStateStarting, wallStateError}, nil)
	state.SetSelected(wallStateAll)
	if g.wallStateFilter != "" {
		state.SetSelected(g.wallStateFilter)
	}
	density := widget.NewSelect(controlDensityLabels(), nil)
	labels := map[string]string{controlDensitySmall: "小", controlDensityStandard: "标准", controlDensityHD: "高清"}
	density.SetSelected(labels[g.controlDensity])
	reset := compactButton("重置筛选", func() { search.SetText(""); state.SetSelected(wallStateAll) })
	content := container.NewVBox(
		widget.NewLabel("搜索设备"), search,
		widget.NewLabel("设备状态"), state,
		widget.NewLabel("画面大小"), density,
		reset, wrappedLabel("筛选只影响显示，已勾选的隐藏设备仍在选择范围内。"),
	)
	d := dialog.NewCustomConfirm("设备视图", "应用", "取消", content, func(ok bool) {
		if !ok {
			return
		}
		g.wallSearch = strings.TrimSpace(search.Text)
		if g.wallSearchEntry != nil {
			g.wallSearchEntry.SetText(g.wallSearch)
		}
		g.wallStateFilter = state.Selected
		for key, label := range labels {
			if label == density.Selected {
				g.setControlDensity(key)
			}
		}
		if g.wallSearch != "" || (g.wallStateFilter != "" && g.wallStateFilter != wallStateAll) {
			view.SetText("筛选")
		} else {
			view.SetText("视图")
		}
		g.renderControlCenter()
	}, g.activeDialogWindow())
	d.Resize(fyne.NewSize(420, 390))
	d.Show()
	g.activeDialogWindow().Canvas().Focus(search)
}

func (g *GUIApp) selectVisibleWallEntries() { g.selectVisibleWallState(wallStateAll) }

func (g *GUIApp) selectVisibleWallState(state string) {
	// A new selection command replaces the whole selection, including hidden
	// entries, so a filter cannot leave unintended devices selected.
	g.controlSelected = map[string]bool{}
	for _, entry := range g.wallVisibleEntries() {
		if state == wallStateAll || (workbenchDeviceState(entry) == state && (state != wallStateStopped || entry.AVD != nil)) {
			g.controlSelected[entry.Key] = true
		}
	}
	g.updateSelectedLabel()
	g.renderControlCenter()
}

func (g *GUIApp) startWorkbenchSelection() {
	plan := g.planSelectedAction(actionStart, false)
	if !g.acceptActionPlan(plan) {
		return
	}
	g.runEntryBatchAction(plan.summary(), plan.Entries, func(entry core.DeviceEntry) error {
		return g.backend.GUIStartAVDs([]string{entry.AVD.Name})
	})
}

func (g *GUIApp) stopWorkbenchSelection() {
	plan := g.planSelectedAction(actionStop, false)
	if !g.acceptActionPlan(plan) {
		return
	}
	g.confirmAction("确认关闭选中模拟器", plan.details(), func() {
		g.runEntryBatchAction(plan.summary(), plan.Entries, func(entry core.DeviceEntry) error {
			return g.backend.GUICloseDeviceConfirmed(entry.Active.Serial, entry.Active.Serial)
		})
	})
}

func (g *GUIApp) setWorkbenchTarget() {
	plan := g.planSelectedAction(actionSetTarget, false)
	if !g.acceptActionPlan(plan) {
		return
	}
	key := plan.Entries[0].Key
	g.runAction(plan.summary(), func() error { return g.backend.GUISetCurrentDevice(key) })
}

func (g *GUIApp) openExternalDeviceWindows() {
	plan := g.planSelectedAction(actionMirror, true)
	if !g.acceptActionPlan(plan) {
		return
	}
	keys := plan.keys()
	g.appendLog("INFO", "%s", plan.summary())
	g.runExternalWindowAction(keys)
}

// The row always retains scan, view and more. Optional actions move to the
// more menu at narrow widths instead of requiring horizontal scrolling.
type workbenchToolbarLayout struct{}

func (workbenchToolbarLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(deviceWallPanelMinWidth, controlCompactControlHeight)
}

func (workbenchToolbarLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 8 {
		return
	}
	widths := []float32{60, 60, 60, 74, 60, 78, 94, 78}
	gap := theme.Padding()
	show := []bool{true, false, true, false, false, false, false, true}
	used := widths[0] + widths[2] + widths[7] + 2*gap
	// Selection and start remain inline ahead of secondary operational tools.
	for _, index := range []int{5, 6, 1, 3, 4} {
		if used+gap+widths[index] <= size.Width {
			show[index] = true
			used += gap + widths[index]
		}
	}
	x := float32(0)
	for index, object := range objects {
		if !show[index] {
			if object.Visible() {
				object.Hide()
			}
			continue
		}
		if !object.Visible() {
			object.Show()
		}
		object.Move(fyne.NewPos(x, (size.Height-controlCompactControlHeight)/2))
		object.Resize(fyne.NewSize(widths[index], controlCompactControlHeight))
		x += widths[index] + gap
	}
}

// The toolbar and collapsed status row use the same horizontal inset.
// Vertical padding is independent so the status row retains its 36px height.
type workbenchRowInsetLayout struct{ vertical float32 }

func (l workbenchRowInsetLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	for _, object := range objects {
		if object.Visible() {
			size = size.Max(object.MinSize())
		}
	}
	return size.Add(fyne.NewSize(2*theme.Padding(), 2*l.vertical))
}
func (l workbenchRowInsetLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		object.Move(fyne.NewPos(theme.Padding(), l.vertical))
		object.Resize(fyne.NewSize(max(0, size.Width-2*theme.Padding()), max(0, size.Height-2*l.vertical)))
	}
}

const toolbarTextOpticalOffset float32 = 3

// Select and Entry use regular text; adjacent actions use bold text. Align
// the visible regular glyphs without moving their shared 36px control bounds.
type centeredDensitySelect struct{ widget.Select }

func newCenteredDensitySelect(options []string, onChanged func(string)) *centeredDensitySelect {
	s := &centeredDensitySelect{Select: widget.Select{Options: options, OnChanged: onChanged}}
	s.ExtendBaseWidget(s)
	return s
}
func (s *centeredDensitySelect) CreateRenderer() fyne.WidgetRenderer {
	base := s.Select.CreateRenderer()
	s.ExtendBaseWidget(s)
	return &centeredDensityRenderer{WidgetRenderer: base, selectWidget: s}
}

type centeredDensityRenderer struct {
	fyne.WidgetRenderer
	selectWidget *centeredDensitySelect
}

func (r *centeredDensityRenderer) Layout(size fyne.Size) {
	r.WidgetRenderer.Layout(size)
	for _, object := range r.WidgetRenderer.Objects() {
		if _, ok := object.(*widget.RichText); ok {
			object.Move(object.Position().Add(fyne.NewPos(0, toolbarTextOpticalOffset)))
		}
	}
}
func (r *centeredDensityRenderer) Refresh() {
	r.WidgetRenderer.Refresh()
	r.Layout(r.selectWidget.Size())
}

// Every visible row item occupies a shared 36px slot so buttons, labels and
// input controls retain aligned bounds and centers.
type centeredRowLayout struct{}

func (centeredRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, controlCompactControlHeight)
	count := 0
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		min := object.MinSize()
		size.Width += min.Width
		size.Height = controlCompactControlHeight
		count++
	}
	if count > 1 {
		size.Width += float32(count-1) * theme.Padding()
	}
	return size
}
func (centeredRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		min := object.MinSize()
		object.Resize(fyne.NewSize(min.Width, controlCompactControlHeight))
		object.Move(fyne.NewPos(x, (size.Height-controlCompactControlHeight)/2))
		x += min.Width + theme.Padding()
	}
}

// Input controls use the same 36px bounds as adjacent toolbar buttons.
type centeredControlLayout struct{ width float32 }

func (l centeredControlLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.width, controlCompactControlHeight)
}
func (l centeredControlLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		height := controlCompactControlHeight
		// Labels lay out their text from the top; center their intrinsic
		// text height inside the common slot instead of stretching it.
		if _, ok := object.(*widget.Label); ok {
			height = object.MinSize().Height
		}
		object.Resize(fyne.NewSize(size.Width, height))
		object.Move(fyne.NewPos(0, (size.Height-height)/2))
	}
}

// Draw the empty search hint at the actual visual center; the standard Entry
// placeholder is positioned from its text inset rather than its bounds.
type centeredSearchEntry struct {
	widget.Entry
	focused bool
}

func newCenteredSearchEntry() *centeredSearchEntry {
	e := &centeredSearchEntry{}
	e.ExtendBaseWidget(e)
	return e
}
func (e *centeredSearchEntry) FocusGained() { e.focused = true; e.Entry.FocusGained(); e.Refresh() }
func (e *centeredSearchEntry) FocusLost()   { e.focused = false; e.Entry.FocusLost(); e.Refresh() }
func (e *centeredSearchEntry) CreateRenderer() fyne.WidgetRenderer {
	hint := canvas.NewText("搜索设备", admColorMuted)
	hint.TextSize = 13
	return &centeredSearchRenderer{WidgetRenderer: e.Entry.CreateRenderer(), entry: e, hint: hint}
}

type centeredSearchRenderer struct {
	fyne.WidgetRenderer
	entry *centeredSearchEntry
	hint  *canvas.Text
}

func (r *centeredSearchRenderer) Objects() []fyne.CanvasObject {
	return append(r.WidgetRenderer.Objects(), r.hint)
}
func (r *centeredSearchRenderer) Layout(size fyne.Size) {
	r.WidgetRenderer.Layout(size)
	hintSize := r.hint.MinSize()
	r.hint.Resize(hintSize)
	r.hint.Move(fyne.NewPos((size.Width-hintSize.Width)/2, (size.Height-hintSize.Height)/2+toolbarTextOpticalOffset))
}
func (r *centeredSearchRenderer) Refresh() {
	r.WidgetRenderer.Refresh()
	if r.entry.Text != "" || r.entry.focused {
		r.hint.Hide()
	} else {
		r.hint.Show()
	}
	r.Layout(r.entry.Size())
	r.hint.Refresh()
}

func (g *GUIApp) setControlDensity(key string) {
	g.controlDensity = key
	g.app.Preferences().SetString("wall.density", key)
	if g.controlDensitySelect != nil {
		label := "画面：" + controlDensityLabel(key)
		if g.controlDensitySelect.Selected != label {
			g.controlDensitySelect.SetSelected(label)
		}
	}
	g.renderControlCenter()
}
