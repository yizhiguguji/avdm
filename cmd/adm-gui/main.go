package main

import (
	core "adm/internal/app"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const maxLogLines = 200
const controlPreviewInterval = 2 * time.Second
const controlDensitySmall = "small"
const controlDensityStandard = "standard"
const controlDensityHD = "hd"
const controlCardHeaderHeight float32 = 34
const controlCardStatusHeight float32 = 24
const controlCardButtonRowsHeight float32 = 36
const controlCardPaddingHeight float32 = 32
const mainWindowDefaultWidth float32 = 1280
const mainWindowDefaultHeight float32 = 820
const controlWindowDefaultWidth float32 = 1280
const controlWindowDefaultHeight float32 = 860
const windowScreenMargin float32 = 80
const deviceWallPanelMinWidth float32 = 320
const workspacePanelMinWidth float32 = 240
const workspacePackageListWidth float32 = 228
const dialogPackageListWidth float32 = 560
const controlDensitySelectWidth float32 = 88
const controlCompactControlHeight float32 = 36
const controlToolbarButtonWidth float32 = 58
const topBarTargetWidth float32 = 240
const topBarSelectionWidth float32 = 150
const topBarToolWidth float32 = 170
const topBarTaskWidth float32 = 150
const topBarActivitySize float32 = 28

type controlDensitySpec struct {
	key         string
	label       string
	cardSize    fyne.Size
	previewSize fyne.Size
}

type stableMinWidthLayout struct {
	width float32
}

type flexibleMinWidthLayout struct {
	width float32
}

func mainWindowInitialSize() fyne.Size {
	width := mainWindowDefaultWidth
	height := mainWindowDefaultHeight
	if screenW, screenH, ok := core.MainDisplaySize(); ok {
		maxWidth := float32(screenW) - windowScreenMargin
		maxHeight := float32(screenH) - windowScreenMargin
		if maxWidth > 0 && width > maxWidth {
			width = maxWidth
		}
		if maxHeight > 0 && height > maxHeight {
			height = maxHeight
		}
	}
	return fyne.NewSize(width, height)
}

func (l stableMinWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := float32(0)
	for _, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		min := obj.MinSize()
		if min.Height > height {
			height = min.Height
		}
	}
	return fyne.NewSize(l.width, height)
}

func (l stableMinWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		obj.Move(fyne.NewPos(0, 0))
		obj.Resize(size)
	}
}

func (l flexibleMinWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := float32(0)
	for _, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		min := obj.MinSize()
		if min.Height > height {
			height = min.Height
		}
	}
	return fyne.NewSize(l.width, height)
}

func (l flexibleMinWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, obj := range objects {
		if obj == nil || !obj.Visible() {
			continue
		}
		obj.Move(fyne.NewPos(0, 0))
		obj.Resize(size)
	}
}

type GUIApp struct {
	wall                     *wallState
	wallSearch               string
	wallStateFilter          string
	packageListBinding       packageListBinding
	pendingWindowArrangement []string
	closed                   bool

	backend *core.App
	app     fyne.App
	window  fyne.Window

	// UI tasks retain immutable targets and parameters. Device operations are
	// coordinated by the backend; the UI tracks progress per launching button.
	activeTasks int

	// Refresh coalescing: refreshes are triggered both manually and after
	// every action, so overlapping requests collapse into a single re-run.
	refreshInFlight         bool
	refreshPending          bool
	refreshPendingRevealTop bool

	entries           []core.DeviceEntry
	currentDeviceKey  string
	currentDevice     string
	toolDetails       string
	toolStatuses      []core.ToolStatus
	lastAPKSource     string
	mirrorAlwaysOnTop map[string]bool

	installScopeLabel     *widget.Label
	selectionActions      fyne.CanvasObject
	currentLabel          *widget.Label
	selectedLabel         *widget.Label
	toolSummary           *widget.Label
	busyLabel             *widget.Label
	progress              *widget.Activity
	logLabel              *selectableLog
	logScroll             *container.Scroll
	logFollow             bool
	logPaused             bool
	logHint               *widget.Label
	logProgrammaticScroll bool
	logLines              []string

	wallSearchEntry      *widget.Entry
	wallLibraryMode      bool
	wallWorkspace        *fyne.Container
	wallLibraryGrid      *fyne.Container
	wallStoppedGrid      *fyne.Container
	wallStoppedPanel     fyne.CanvasObject
	wallOnlineLabel      *widget.Label
	wallLibraryButton    *widget.Button
	wallWorkspaceWidth   float32
	wallOnlineCount      int
	wallLibraryCount     int
	libraryCollapsed     bool
	controlWindow        fyne.Window
	controlGrid          *fyne.Container
	controlSummary       *widget.Label
	controlDensity       string
	controlDensitySelect *widget.Select
	controlSelected      map[string]bool
	controlHidden        map[string]bool
	controlCards         map[string]*controlCardView
	controlRealtimeStops map[string]func()
	controlPreviewStop   chan struct{}

	// Collapsible dock state. Widths/height are stored in fixed pixels (not
	// ratios) so the sidebars keep their size when the window is resized; the
	// center device wall absorbs the change. See dock.go for the layouts.
	//
	// logCollapsed is a boolean; the right side instead tracks
	// which tool panel is open in rightActive ("" = none, else "install" /
	// "uninstall" / "message"), because its edge icon bar switches between
	// several mutually-exclusive panels JetBrains-style.
	rightActive  string
	logCollapsed bool
	rightW       float32
	logH         float32

	hDock           *fyne.Container
	vDock           *fyne.Container
	logStack        *fyne.Container
	logRail         fyne.CanvasObject
	logRailSummary  *canvas.Text
	rightHandle     *resizeHandle
	logHandle       *resizeHandle
	toolTargetLabel *widget.Label
	rightPanel      fyne.CanvasObject
	logPanel        fyne.CanvasObject

	// Edge icon bars (always visible) and their tappable icons. The icons are
	// the single collapse/expand control per pane: highlighted when the pane
	// is open, dimmed when collapsed. See standard buttons in dock.go.
	rightIconBar  fyne.CanvasObject
	logToolIcon   *widget.Button
	installIcon   *widget.Button
	uninstallIcon *widget.Button
	messageIcon   *widget.Button

	// The right pane hosts three mutually-exclusive tool panels stacked in
	// rightPanel; only the one named by rightActive is visible at a time.
	installPanel   fyne.CanvasObject
	uninstallPanel fyne.CanvasObject
	messagePanel   fyne.CanvasObject

	apkEntry       *widget.Entry
	allowDowngrade *widget.Check
	textEntry      *widget.Entry
	useADBKeyboard *widget.Check
	textHint       *widget.Label
	packageFilter  *widget.Entry
	thirdPartyOnly *widget.Check
	packagePicker  *packagePicker
	keepData       *widget.Check
	user0Only      *widget.Check

	preparedActionButtons    map[*widget.Button]bool
	pendingActionButton      *widget.Button
	pendingActionButtonLabel string
	// loadingButtons tracks buttons currently showing a "…" loading state,
	// mapping each to its original label so it can be restored. Multiple
	// buttons can load at once because actions run concurrently.
	loadingButtons map[*widget.Button]string
}

type controlCardView struct {
	wall *wallCardState

	entryKey   string
	entry      core.DeviceEntry
	serial     string
	title      string
	detail     string
	realtime   bool
	hidden     bool
	refreshing bool
	preview    *previewPane
	status     *widget.Label
}

type packagePicker struct {
	all      []string
	filter   string
	filtered []string
	selected string
	list     *widget.List
}

func main() {
	backend, err := core.New()
	if err != nil {
		panic(err)
	}
	gui := newGUIApp(backend)
	gui.show()
}

func newGUIApp(backend *core.App) *GUIApp {
	a := fyneapp.NewWithID(applicationID)
	a.Settings().SetTheme(admTheme{base: theme.DefaultTheme()})
	w := a.NewWindow("安卓设备矩阵")
	initialSize := mainWindowInitialSize()
	w.Resize(initialSize)

	gui := &GUIApp{
		backend:      backend,
		app:          a,
		window:       w,
		logCollapsed: true,
	}
	gui.build()
	w.SetOnClosed(func() { gui.closed = true; gui.closeDeviceWall() })
	w.Resize(initialSize)
	return gui
}

func (g *GUIApp) show() {
	go func() {
		time.Sleep(150 * time.Millisecond)
		fyne.Do(func() {
			g.refreshAsync(true)
		})
	}()
	g.startControlPreviewLoop()
	g.window.ShowAndRun()
}

func (g *GUIApp) build() {
	g.currentLabel = widget.NewLabelWithStyle("主目标：未选择", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	g.currentLabel.Truncation = fyne.TextTruncateEllipsis
	g.selectedLabel = widget.NewLabel("选中：未选择")
	g.selectedLabel.Truncation = fyne.TextTruncateEllipsis
	g.toolSummary = widget.NewLabel("工具：检测中")
	g.busyLabel = widget.NewLabel("任务：空闲")
	g.progress = widget.NewActivity()
	g.progress.Hide()

	g.logLabel = newSelectableLog()
	g.logLabel.onFocus = func() {
		g.logPaused = true
		g.logFollow = false
		if g.logHint != nil {
			g.logHint.SetText("已暂停显示更新 · 可选择复制，点击跟随最新恢复")
		}
	}
	g.logFollow = true

	wall := g.buildDeviceWallPanel()

	// The three right-side tool panels are built once and only toggled visible;
	// keeping them alive preserves their form state (entered APK path, package
	// filter, drafted text) across icon switches.
	g.installPanel = rightToolPanel(g.buildInstallPanel())
	g.uninstallPanel = rightToolPanel(g.buildUninstallPanel())
	g.messagePanel = rightToolPanel(g.buildMessagePanel())
	g.toolTargetLabel = widget.NewLabel("主目标：未选择")
	g.toolTargetLabel.Wrapping = fyne.TextWrapWord
	dismissTool := compactButton("收起", func() { g.rightActive = ""; g.applyWorkbenchCollapseState() })
	toolHeader := container.NewBorder(nil, canvas.NewLine(admColorBorder), nil, dismissTool, g.toolTargetLabel)
	g.rightPanel = container.NewBorder(topSurface(container.NewPadded(toolHeader)), nil, nil, nil, container.NewStack(g.installPanel, g.uninstallPanel, g.messagePanel))

	g.logPanel = g.buildLogPanel()

	// Keep the tool panel and log sizes stable when toggling them.
	if g.rightW == 0 {
		g.rightW = dockRightDefaultWidth
	}
	if g.logH == 0 {
		g.logH = dockLogDefaultHeight
	}

	g.logRail = g.buildLogRail()

	// The log pane uses a summary rail or full panel, toggled from the right rail.
	g.logStack = container.NewStack(g.logPanel, g.logRail)

	g.rightHandle = newResizeHandle(true, g.resizeRightBy)
	g.logHandle = newResizeHandle(false, g.resizeLogBy)

	// The device wall fills the workbench beside the optional right tool panel.
	g.hDock = container.New(horizontalDockLayout{g: g},
		wall, g.rightHandle, g.rightPanel)
	g.vDock = container.New(verticalDockLayout{g: g}, g.hDock, g.logHandle, g.logStack)

	// Root: working area followed by the right tool rail.
	root := container.NewBorder(nil, nil, nil, g.buildRightIconBar(), g.vDock)
	g.applyWorkbenchCollapseState()

	page := root
	g.window.SetContent(appFrame(page))
}

// applyWorkbenchCollapseState reconciles the docked panes with the current
// state (rightActive / logCollapsed). It only toggles child
// visibility and relays out the docks — no window.Resize hack, and no
// Split.Offset ratios (those were the source of the "still draggable when
// collapsed" and scaling-drift bugs). A collapsed pane's resize handle is
// hidden, which makes it physically undraggable; the always-visible edge icon
// bars are the way back.
func (g *GUIApp) applyWorkbenchCollapseState() {
	// Right: rightActive names the one visible tool panel ("" = none).
	g.reconcileRightPanels()

	// Log: narrow summary rail (collapsed) vs full panel (expanded).
	setPaneCollapsed(g.logPanel, g.logRail, g.logHandle, g.logCollapsed)
	if g.logRailSummary != nil {
		g.logRailSummary.Text = g.logCollapsedSummary()
		g.logRailSummary.Refresh()
	}

	g.syncIconHighlights()

	if g.hDock != nil {
		g.hDock.Refresh()
	}
	if g.vDock != nil {
		g.vDock.Refresh()
	}
}

// reconcileRightPanels shows the single tool panel named by rightActive and
// hides the rest. When nothing is active the whole right pane and its resize
// handle collapse away (the dock layout gives them zero width).
func (g *GUIApp) reconcileRightPanels() {
	open := g.rightActive != ""
	setVisible(g.installPanel, g.rightActive == "install")
	setVisible(g.uninstallPanel, g.rightActive == "uninstall")
	setVisible(g.messagePanel, g.rightActive == "message")
	setVisible(g.rightPanel, open)
	setVisible(g.rightHandle, open)
}

// toggleRightPanel opens the named tool panel, or closes it if it is already
// the active one (click the lit icon again to dismiss).
func (g *GUIApp) toggleRightPanel(name string) {
	if g.rightActive == name {
		g.rightActive = ""
	} else {
		g.rightActive = name
	}
	g.applyWorkbenchCollapseState()
}

// syncIconHighlights lights the edge icon of each open pane and dims the rest,
// keeping the icon bars, panel visibility and dock layout on a single source
// of truth.
func (g *GUIApp) syncIconHighlights() {
	if g.logToolIcon != nil {
		setWorkbenchToolActive(g.logToolIcon, !g.logCollapsed)
	}
	if g.installIcon != nil {
		setWorkbenchToolActive(g.installIcon, g.rightActive == "install")
	}
	if g.uninstallIcon != nil {
		setWorkbenchToolActive(g.uninstallIcon, g.rightActive == "uninstall")
	}
	if g.messageIcon != nil {
		setWorkbenchToolActive(g.messageIcon, g.rightActive == "message")
	}
}

func setVisible(obj fyne.CanvasObject, visible bool) {
	if obj == nil {
		return
	}
	if visible {
		obj.Show()
	} else {
		obj.Hide()
	}
}

// buildRightIconBar is the always-visible right edge bar. The top three icons
// switch between mutually-exclusive tool panels (install / uninstall / message,
// lit when open); below a divider, two momentary action icons fire the reboot
// and close/disconnect flows directly with a confirmation dialog.
func (g *GUIApp) buildRightIconBar() fyne.CanvasObject {
	g.installIcon = newWorkbenchRailButton("安装", theme.DownloadIcon(), func() { g.toggleRightPanel("install") })
	g.uninstallIcon = newWorkbenchRailButton("卸载", theme.DeleteIcon(), func() { g.toggleRightPanel("uninstall") })
	g.messageIcon = newWorkbenchRailButton("输入", theme.ContentPasteIcon(), func() { g.toggleRightPanel("message") })
	reboot := newWorkbenchRailButton("重启", theme.ViewRefreshIcon(), g.rebootCurrentDevice)
	close := newWorkbenchRailButton("关闭", theme.CancelIcon(), g.closeCurrentDevice)
	g.prepareActionButton(reboot)
	g.logToolIcon = newWorkbenchRailButton("日志", theme.DocumentIcon(), func() {
		g.logCollapsed = !g.logCollapsed
		g.applyWorkbenchCollapseState()
	})
	target := newWorkbenchRailButton("设主目标", nil, g.setWorkbenchTarget)
	return iconBarColumn(g.installIcon, g.uninstallIcon, g.messageIcon, iconBarDivider(), target, reboot, close, iconBarDivider(), g.logToolIcon)
}

func iconBarColumn(items ...fyne.CanvasObject) fyne.CanvasObject {
	for i, item := range items {
		if _, ok := item.(*widget.Button); ok {
			items[i] = container.NewGridWrap(fyne.NewSize(dockIconBarWidth, controlCompactControlHeight), item)
		}
	}
	return container.NewStack(
		roundedRect(admColorPanelBG, 8),
		container.NewVBox(items...),
	)
}

func iconBarDivider() fyne.CanvasObject {
	line := canvas.NewRectangle(admColorBorder)
	line.SetMinSize(fyne.NewSize(18, 1))
	return container.NewGridWrap(fyne.NewSize(dockIconBarWidth, 11), container.NewCenter(line))
}

func (g *GUIApp) buildLogRail() fyne.CanvasObject {
	g.logRailSummary = canvas.NewText(g.logCollapsedSummary(), admColorMuted)
	g.logRailSummary.TextSize = 12
	details := compactButton("检查工具", g.showToolHealthDialog)
	g.busyLabel.Truncation = fyne.TextTruncateEllipsis
	left := container.New(centeredRowLayout{}, details, footerStatus(g.toolSummary, 100, 2), footerStatus(g.busyLabel, 140, 2), container.NewCenter(g.progress))
	right := container.New(centeredRowLayout{}, footerStatus(g.currentLabel, 180, 0), footerStatus(g.selectedLabel, 190, 0))
	start := compactButton("启动", g.startWorkbenchSelection)
	stop := compactButton("关闭…", g.stopWorkbenchSelection)
	remove := compactButton("删除…", g.showSelectedDeleteAVDDialog)
	clear := compactButton("清选", g.clearWorkbenchSelection)
	g.registerActionButtons(start, stop, remove)
	g.selectionActions = container.New(centeredRowLayout{}, start, stop, remove, clear)
	g.selectionActions.Hide()
	row := container.NewThemeOverride(container.NewBorder(nil, nil, left, right, g.selectionActions), captionTheme{g.app.Settings().Theme()})
	return collapsedBar(container.New(workbenchRowInsetLayout{}, row))
}

func (g *GUIApp) buildTopBar() fyne.CanvasObject {
	toolDetailsButton := compactButton("工具", func() {
		g.showToolHealthDialog()
	})

	title := canvas.NewText("设备工作台", admColorText)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 15
	g.currentLabel.Wrapping = fyne.TextTruncate
	g.selectedLabel.Wrapping = fyne.TextTruncate
	g.toolSummary.Wrapping = fyne.TextTruncate
	g.busyLabel.Wrapping = fyne.TextTruncate
	targetBlock := container.NewHBox(
		compactStatus(g.currentLabel, topBarTargetWidth),
		container.NewThemeOverride(compactStatus(g.selectedLabel, topBarSelectionWidth), captionTheme{g.app.Settings().Theme()}),
	)
	healthBlock := container.NewHBox(
		container.NewThemeOverride(compactStatus(g.toolSummary, topBarToolWidth), captionTheme{g.app.Settings().Theme()}),
		compactStatus(g.busyLabel, topBarTaskWidth),
		container.NewGridWrap(fyne.NewSize(topBarActivitySize, topBarActivitySize), g.progress),
	)
	status := container.NewHBox(targetBlock, mutedText("  "), healthBlock)
	return topSurface(container.NewBorder(nil, nil, title, toolDetailsButton, status))
}

func (g *GUIApp) showToolHealthDialog() {
	statuses := g.toolStatuses
	if len(statuses) == 0 {
		statuses = []core.ToolStatus{{Name: "工具状态", Available: false, Error: "暂无工具状态，请先刷新设备列表"}}
	}
	bootstrap := g.backend.GUIDependencyBootstrapStatus()
	missing := missingInstallableToolStatuses(statuses)
	rows := make([]fyne.CanvasObject, 0, len(statuses))
	for _, status := range statuses {
		rows = append(rows, toolStatusCompactRow(status))
	}

	refreshButton := compactButton("重新检查", func() {
		g.refreshAsync(false)
	})
	refreshButton.Importance = widget.MediumImportance
	g.prepareActionButton(refreshButton)
	installButton := compactButton("安装/修复依赖", nil)
	installButton.Importance = widget.HighImportance
	installButton.OnTapped = func() {
		g.confirmDependencyBootstrap(bootstrap, missing)
	}
	g.prepareActionButton(installButton)
	if !bootstrap.Supported {
		installButton.Disable()
	}
	closeButton := compactButton("关闭", nil)
	permissionButton := compactButton("辅助功能授权", func() {
		g.showAccessibilityPermissionDialog("")
	})
	list := container.NewVScroll(container.NewVBox(rows...))
	list.SetMinSize(fyne.NewSize(720, 280))

	summary := widget.NewLabel(toolHealthSummary(missing, bootstrap))
	summary.Wrapping = fyne.TextWrapWord
	scriptHint := widget.NewLabel(bootstrapHint(bootstrap))
	scriptHint.Wrapping = fyne.TextWrapWord
	scriptHint.Importance = widget.LowImportance
	copyButton := widget.NewButton("复制检测结果", func() {
		g.app.Clipboard().SetContent(toolHealthDetails(statuses, bootstrap))
	})
	footer := container.NewVBox(mutedText("Android SDK / Homebrew / PATH 检测结果"), container.NewCenter(container.NewHBox(copyButton, permissionButton, installButton, refreshButton, closeButton)))
	content := container.NewStack(
		roundedRect(admColorPanelBG, 10),
		container.NewPadded(container.NewBorder(
			container.NewVBox(sectionTitle("工具健康状态"), summary, scriptHint),
			footer,
			nil,
			nil,
			list,
		)),
	)
	d := dialog.NewCustomWithoutButtons("工具健康状态", container.NewGridWrap(fyne.NewSize(800, 480), content), g.activeDialogWindow())
	closeButton.OnTapped = func() {
		d.Hide()
	}
	d.Show()
}

func (g *GUIApp) confirmDependencyBootstrap(bootstrap core.DependencyBootstrapStatus, missing []core.ToolStatus) {
	if !bootstrap.Supported {
		g.showInfo(bootstrap.Error)
		return
	}
	target := "当前没有缺失项，将重新执行依赖修复。"
	if len(missing) > 0 {
		target = "将尝试安装/修复：" + strings.Join(toolStatusNames(missing), "、")
	}
	message := target + "\n\n该操作会调用 Homebrew 和 sdkmanager，可能需要联网并耗时数分钟。Homebrew 本身不会被自动安装。"
	g.confirmAction("安装/修复 macOS 依赖", message, func() {
		g.runDependencyBootstrap()
	})
}

func (g *GUIApp) runDependencyBootstrap() {
	name := "安装/修复 macOS 依赖"
	btn := g.beginTask(name)
	g.appendLog("INFO", "开始：%s", name)
	go func() {
		output, err := g.backend.GUIInstallMacOSDependencies()
		fyne.Do(func() {
			g.endTask(btn)
			if strings.TrimSpace(output) != "" {
				g.appendLog("INFO", "依赖安装输出：\n%s", compactMultiline(output, 2400))
			}
			if err != nil {
				g.appendLog("ERROR", "失败：%s：%v", name, err)
				g.showMessageDialog("错误", err.Error(), true)
			} else {
				g.appendLog("DONE", "完成：%s", name)
			}
			g.refreshAsync(false)
		})
	}()
}

func toolStatusCompactRow(status core.ToolStatus) fyne.CanvasObject {
	state := "✓"
	stateColor := admColorSuccess
	detail := compactPathMiddle(strings.TrimSpace(status.Path), 72)
	if !status.Available {
		state = "!"
		stateColor = admColorDanger
		detail = strings.TrimSpace(status.Error)
	}
	if detail == "" {
		detail = "未检测到路径"
	}
	icon := canvas.NewText(state, stateColor)
	icon.TextSize = 15
	icon.TextStyle = fyne.TextStyle{Bold: true}
	name := widget.NewLabelWithStyle(status.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	source := widget.NewLabel(sourceOrDefault(strings.TrimSpace(status.Source)))
	source.Importance = widget.LowImportance
	path := widget.NewLabel(detail)
	path.Wrapping = fyne.TextWrapWord
	return compactSurface(container.NewBorder(
		nil,
		nil,
		container.NewGridWrap(fyne.NewSize(28, 28), container.NewCenter(icon)),
		nil,
		container.NewVBox(container.NewHBox(name, source), path),
	))
}

func sourceOrDefault(source string) string {
	if strings.TrimSpace(source) == "" {
		return "-"
	}
	return source
}

func compactPathMiddle(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if maxRunes < 12 || len(runes) <= maxRunes {
		return text
	}
	head := maxRunes/2 - 2
	tail := maxRunes - head - 3
	return string(runes[:head]) + "..." + string(runes[len(runes)-tail:])
}

func sectionTitle(text string) fyne.CanvasObject {
	label := canvas.NewText(text, admColorText)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.TextSize = 13
	return label
}

func mutedText(text string) fyne.CanvasObject {
	label := canvas.NewText(text, admColorMuted)
	label.TextSize = 12
	return label
}

func roundedRect(fill color.Color, radius float32) *canvas.Rectangle {
	rect := canvas.NewRectangle(fill)
	rect.CornerRadius = radius
	return rect
}

func appFrame(content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(
		canvas.NewRectangle(admColorAppBG),
		content,
	)
}

func collapsedBar(content fyne.CanvasObject) fyne.CanvasObject {
	bg := roundedRect(admColorPanelBG2, 8)
	bg.StrokeColor = admColorBorder
	bg.StrokeWidth = 1
	return container.NewStack(bg, content)
}

func topSurface(content fyne.CanvasObject) fyne.CanvasObject {
	bg := roundedRect(admColorPanelBG, 8)
	bg.StrokeColor = admColorBorder
	bg.StrokeWidth = 1
	return container.NewStack(bg, content)
}

func panelSurface(title, subtitle string, content fyne.CanvasObject) fyne.CanvasObject {
	return panelSurfaceWithActions(title, subtitle, nil, content)
}

func panelSurfaceWithActions(title, subtitle string, actions fyne.CanvasObject, content fyne.CanvasObject) fyne.CanvasObject {
	var body fyne.CanvasObject = content
	if title != "" || subtitle != "" || actions != nil {
		header := container.NewVBox()
		if title != "" {
			header.Add(sectionTitle(title))
		}
		if subtitle != "" {
			header.Add(mutedText(subtitle))
		}
		body = container.NewBorder(container.NewBorder(nil, nil, nil, actions, header), nil, nil, nil, content)
	}
	return container.NewStack(
		roundedRect(admColorPanelBG, 8),
		container.NewPadded(body),
	)
}

func wrappedLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

func compactSurface(content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(
		roundedRect(admColorPanelBG2, 8),
		container.NewPadded(content),
	)
}

func stableWorkspacePanel(content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(stableMinWidthLayout{width: workspacePanelMinWidth}, content)
}

func controlCardSurface(content fyne.CanvasObject) fyne.CanvasObject {
	bg := roundedRect(admColorPanelBG, 6)
	bg.StrokeColor = admColorBorder
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.NewPadded(content))
}

// rightToolPanel wraps one right-side tool panel's content in a scrollable
// panel surface with a stable minimum width, so switching panels never jumps
// the layout.
func rightToolPanel(content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(stableMinWidthLayout{width: workspacePanelMinWidth},
		panelSurface("", "", container.NewVScroll(content)))
}

func (g *GUIApp) buildLogPanel() fyne.CanvasObject {
	clearButton := widget.NewButton("清空日志", func() {
		g.logLines = nil
		g.logPaused = false
		if g.logHint != nil {
			g.logHint.SetText("最近 200 行 · 拖选或 ⌘A / ⌘C")
		}
		g.logLabel.SetText("")
		g.renderLogLines()
		g.logFollow = true
		g.scrollLogToBottom()
	})
	clearButton.Importance = widget.LowImportance
	copyButton := widget.NewButton("复制全部", func() {
		g.app.Clipboard().SetContent(strings.Join(g.logLines, "\n"))
	})
	copyButton.Importance = widget.LowImportance
	copySelection := widget.NewButton("复制选中", func() {
		if text := g.logLabel.SelectedText(); text != "" {
			g.app.Clipboard().SetContent(text)
		}
	})
	followButton := widget.NewButton("跟随最新", func() {
		if canvas := g.app.Driver().CanvasForObject(g.logLabel); canvas != nil {
			canvas.Unfocus()
		}
		g.logPaused = false
		g.renderLogLines()
		g.logLabel.Entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
		g.logHint.SetText("最近 200 行 · 拖选或 ⌘A / ⌘C")
		g.scrollLogToBottom()
	})
	copySelection.Importance = widget.LowImportance
	followButton.Importance = widget.LowImportance
	g.logScroll = container.NewScroll(g.logLabel)
	g.logScroll.OnScrolled = func(_ fyne.Position) {
		if g.logProgrammaticScroll {
			return
		}
		g.logFollow = g.logScrollAtBottom()
	}
	// Collapse is driven by the log icon in the right rail; the header keeps
	// only the clear/copy actions.
	g.logHint = widget.NewLabel("最近 200 行 · 拖选或 ⌘A / ⌘C")
	header := container.NewBorder(nil, nil, container.NewVBox(sectionTitle("任务/日志"), container.NewThemeOverride(g.logHint, captionTheme{g.app.Settings().Theme()})), container.NewHBox(followButton, clearButton, copySelection, copyButton), nil)
	return panelSurface("", "", container.NewBorder(
		header, nil, nil, nil, g.logScroll,
	))
}

func (g *GUIApp) refreshAsync(revealTop bool) {
	// Coalesce bursts: many actions each trigger a refresh on completion, so if
	// one is already running just flag that another is wanted and let the
	// in-flight refresh re-run once when it finishes.
	if g.refreshInFlight {
		g.refreshPending = true
		g.refreshPendingRevealTop = g.refreshPendingRevealTop || revealTop
		return
	}
	g.refreshInFlight = true
	btn := g.beginTask("刷新列表")
	g.appendLog("INFO", "开始：刷新列表")
	go func() {
		state, err := g.backend.GUIState()
		fyne.Do(func() {
			g.endTask(btn)
			g.refreshInFlight = false
			if err != nil {
				g.appendLog("ERROR", "刷新失败：%v", err)
				g.showMessageDialog("错误", err.Error(), true)
			} else {
				g.applyState(state, revealTop)
				g.appendLog("DONE", "完成：刷新列表")
			}
			if g.refreshPending {
				g.refreshPending = false
				again := g.refreshPendingRevealTop
				g.refreshPendingRevealTop = false
				g.refreshAsync(again)
			}
		})
	}()
}

func (g *GUIApp) applyState(state core.GUIState, revealTop bool) {
	previousKey := g.currentDeviceKey
	g.entries = state.Devices
	g.currentDevice = state.CurrentDevice
	g.currentDeviceKey = state.CurrentDeviceKey
	g.onTargetChanged(previousKey, state.CurrentDeviceKey)
	g.lastAPKSource = state.LastAPKSource
	g.mirrorAlwaysOnTop = state.MirrorAlwaysOnTop
	g.currentLabel.SetText("主目标：" + state.CurrentDevice)
	if g.toolTargetLabel != nil {
		g.toolTargetLabel.SetText("主目标：" + state.CurrentDevice)
	}
	missing := 0
	for _, tool := range state.Tools {
		if !tool.Available {
			missing++
		}
	}
	if missing == 0 {
		g.toolSummary.SetText("工具：已就绪")
	} else {
		g.toolSummary.SetText(fmt.Sprintf("工具：%d 项待处理", missing))
	}
	g.toolDetails = detailedToolStatus(state.Tools)
	g.toolStatuses = append([]core.ToolStatus(nil), state.Tools...)
	if g.apkEntry != nil && strings.TrimSpace(g.apkEntry.Text) == "" {
		if state.LastAPKSource != "" {
			g.apkEntry.SetPlaceHolder("留空使用上次：" + state.LastAPKSource)
		} else {
			g.apkEntry.SetPlaceHolder("APK 文件路径或 URL")
		}
	}
	g.restoreSelection()
	if g.controlGrid != nil {
		g.renderControlCenter()
	}
}

func (g *GUIApp) restoreSelection() {
	valid := map[string]bool{}
	for _, entry := range g.entries {
		valid[entry.Key] = true
	}
	for key := range g.controlSelected {
		if !valid[key] {
			delete(g.controlSelected, key)
		}
	}
	g.updateSelectedLabel()
}

func (g *GUIApp) selectionVisibilityCounts() (int, int) {
	visible := map[string]bool{}
	for _, entry := range g.wallVisibleEntries() {
		visible[entry.Key] = true
	}
	selected, hidden := 0, 0
	for _, entry := range g.selectedControlEntries() {
		selected++
		if !visible[entry.Key] {
			hidden++
		}
	}
	return selected, hidden
}

func (g *GUIApp) updateSelectedLabel() {
	entries := g.selectedControlEntries()
	selected, hidden := g.selectionVisibilityCounts()
	text := "选中：未选择"
	if selected == 1 {
		text = "选中：" + entries[0].Label
	} else if selected > 1 {
		text = fmt.Sprintf("选中：%d 台", selected)
	}
	if hidden > 0 {
		text = fmt.Sprintf("选中：%d 台（隐藏 %d）", selected, hidden)
	}
	if g.selectedLabel != nil {
		g.selectedLabel.SetText(text)
	}
	setVisible(g.selectionActions, selected > 0)
	g.updateInstallScopeLabel(selected, hidden)
	if g.logRail != nil {
		g.logRail.Refresh()
	}
}

func (g *GUIApp) updateInstallScopeLabel(selected, hidden int) {
	if g.installScopeLabel == nil {
		return
	}
	text := "安装范围：主目标 · 未选择"
	if selected > 0 {
		text = fmt.Sprintf("安装范围：已勾选 %d 台", selected)
		if hidden > 0 {
			text += fmt.Sprintf("（含筛选隐藏 %d 台）", hidden)
		}
	} else if g.currentDevice != "" {
		text = "安装范围：主目标 · " + g.currentDevice
	}
	g.installScopeLabel.SetText(text)
}

func (g *GUIApp) clearWorkbenchSelection() {
	g.controlSelected = map[string]bool{}
	g.updateSelectedLabel()
	g.renderControlCenter()
}

func (g *GUIApp) runAction(name string, fn func() error) {
	g.runActionWithCompletion(name, fn, nil)
}

func (g *GUIApp) runActionWithCompletion(name string, fn func() error, done func(error)) {
	btn := g.beginTask(name)
	g.appendLog("INFO", "开始：%s", name)
	go func() {
		err := fn()
		fyne.Do(func() {
			g.endTask(btn)
			if g.closed {
				return
			}
			if done != nil {
				done(err)
			}
			if err != nil {
				if core.IsAccessibilityPermissionRequired(err) {
					g.appendLog("INFO", "%s：%v", name, err)
					g.showAccessibilityPermissionDialog(err.Error())
				} else if core.IsAccessibilityPermissionPrompted(err) {
					g.appendLog("INFO", "%s：%v", name, err)
				} else {
					g.appendLog("ERROR", "失败：%s：%v", name, err)
					g.showMessageDialog("错误", err.Error(), true)
				}
			} else {
				g.appendLog("DONE", "完成：%s", name)
			}
			if shouldRefreshAfterAction(name) {
				g.refreshAsync(false)
			}
		})
	}()
}

// runInstallAction asks before uninstalling to resolve signature conflicts or
// blocked downgrades. The retry is restricted to the affected serials.
func (g *GUIApp) runInstallAction(name string, install func() error, force func([]string) error) {
	btn := g.beginTask(name)
	g.appendLog("INFO", "开始：%s", name)
	go func() {
		err := install()
		fyne.Do(func() {
			g.endTask(btn)
			if err == nil {
				g.appendLog("DONE", "完成：%s", name)
				g.refreshAsync(false)
				return
			}
			if blocked, ok := core.AsReinstallRequired(err); ok {
				g.appendLog("INFO", "%s：%v", name, err)
				g.confirmInstallReinstall(name, blocked, force)
				return
			}
			g.appendLog("ERROR", "失败：%s：%v", name, err)
			g.showMessageDialog("错误", err.Error(), true)
		})
	}()
}

func (g *GUIApp) confirmInstallReinstall(name string, blocked *core.ReinstallRequiredError, force func([]string) error) {
	pkg := blocked.Package
	if pkg == "" {
		pkg = "该应用"
	}
	msg := fmt.Sprintf("%s\n\n需要先卸载 %s 再全新安装，这会清空该应用的全部数据（登录态、聊天记录等）。", blocked.Reason, pkg)
	if len(blocked.Serials) > 0 {
		msg += "\n\n受影响设备：\n" + strings.Join(blocked.Serials, "\n")
	}
	if strings.TrimSpace(blocked.Detail) != "" {
		msg += "\n\n" + strings.TrimSpace(blocked.Detail)
	}
	msg += "\n\n确认卸载后重新安装吗？"
	g.confirmActionWithLabels("需要卸载重装", msg, "卸载重装", "放弃", func() {
		g.runAction(name+"（卸载重装）", func() error { return force(blocked.Serials) })
	})
}

func shouldRefreshAfterAction(name string) bool {
	name = strings.TrimSpace(name)
	noRefreshPrefixes := []string{
		"刷新包列表",
		"发送文本",
		"安装 APK",
		"重新安装上次 APK",
		"卸载应用",
		"聚焦原生窗口",
		"平铺原生窗口",
		"刷新 ",
	}
	for _, prefix := range noRefreshPrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

// beginTask registers a starting action, claims the button that launched it
// (showing a "…" loading state on just that button), and returns the button so
// the caller can release it on completion. All calls happen on the UI thread.
func (g *GUIApp) beginTask(name string) *widget.Button {
	btn := g.claimPendingButton()
	g.activeTasks++
	g.updateBusyIndicator(name)
	return btn
}

// endTask marks an action finished and restores its launching button.
func (g *GUIApp) endTask(btn *widget.Button) {
	if g.activeTasks > 0 {
		g.activeTasks--
	}
	g.releaseActionButton(btn)
	g.updateBusyIndicator("")
}

// claimPendingButton consumes the button that triggered the current action and
// puts it into the loading state. Returns nil when there is no such button or
// it is already loading (guards against double-triggering the same button).
func (g *GUIApp) claimPendingButton() *widget.Button {
	btn := g.pendingActionButton
	label := g.pendingActionButtonLabel
	g.pendingActionButton = nil
	g.pendingActionButtonLabel = ""
	if btn == nil {
		return nil
	}
	if label == "" {
		label = btn.Text
	}
	if g.loadingButtons == nil {
		g.loadingButtons = map[*widget.Button]string{}
	}
	if _, loading := g.loadingButtons[btn]; loading {
		return nil
	}
	g.loadingButtons[btn] = label
	btn.SetText(label + "...")
	btn.Disable()
	return btn
}

func (g *GUIApp) releaseActionButton(btn *widget.Button) {
	if btn == nil {
		return
	}
	if label, ok := g.loadingButtons[btn]; ok {
		btn.SetText(label)
		btn.Enable()
		delete(g.loadingButtons, btn)
	}
}

func (g *GUIApp) updateBusyIndicator(name string) {
	if g.activeTasks <= 0 {
		g.busyLabel.SetText("任务：空闲")
		g.progress.Stop()
		g.progress.Hide()
		return
	}
	if g.activeTasks == 1 && strings.TrimSpace(name) != "" {
		g.busyLabel.SetText("任务：正在执行 " + name)
	} else if strings.TrimSpace(name) != "" {
		g.busyLabel.SetText(fmt.Sprintf("任务：%s 等 %d 个进行中", name, g.activeTasks))
	} else {
		g.busyLabel.SetText(fmt.Sprintf("任务：%d 个进行中", g.activeTasks))
	}
	g.progress.Show()
	g.progress.Start()
}

func (g *GUIApp) registerActionButtons(buttons ...*widget.Button) {
	for _, button := range buttons {
		g.prepareActionButton(button)
	}
}

func (g *GUIApp) prepareActionButton(button *widget.Button) {
	if button == nil {
		return
	}
	if g.preparedActionButtons == nil {
		g.preparedActionButtons = map[*widget.Button]bool{}
	}
	if g.preparedActionButtons[button] {
		return
	}
	g.preparedActionButtons[button] = true
	original := button.OnTapped
	button.OnTapped = func() {
		// A button whose action is still running is disabled, so a tap here
		// means it is idle. Record it as the pending trigger; the task runner
		// claims it (showing loading) or, for actions that only open a dialog,
		// the trailing clear below resets it.
		if _, loading := g.loadingButtons[button]; loading {
			return
		}
		g.pendingActionButton = button
		g.pendingActionButtonLabel = button.Text
		if original != nil {
			original()
		}
		if g.pendingActionButton == button {
			g.pendingActionButton = nil
			g.pendingActionButtonLabel = ""
		}
	}
}

func (g *GUIApp) appendLog(level, format string, args ...any) {
	line := fmt.Sprintf("%s  %-5s  %s", time.Now().Format("15:04:05"), level, fmt.Sprintf(format, args...))
	follow := g.logFollow || g.logScrollAtBottom()
	g.logLines = append(g.logLines, line)
	if len(g.logLines) > maxLogLines {
		g.logLines = g.logLines[len(g.logLines)-maxLogLines:]
	}
	g.renderLogLines()
	if follow && !g.logPaused {
		g.logFollow = true
		g.scrollLogToBottom()
	}
}

func (g *GUIApp) copyControlText(label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	g.app.Clipboard().SetContent(value)
	g.appendLog("DONE", "已复制%s：%s", label, value)
}

func (g *GUIApp) copyableControlText(content fyne.CanvasObject, label, value string) fyne.CanvasObject {
	return container.NewStack(content, newCopyTapLayer(func() {
		g.copyControlText(label, value)
	}))
}

func (g *GUIApp) renderLogLines() {
	if g.logLabel == nil {
		return
	}
	if g.logPaused {
		return
	}
	g.logLabel.SetText(strings.Join(g.logLines, "\n"))
	g.logLabel.Refresh()
}

func (g *GUIApp) logCollapsedSummary() string {
	if len(g.logLines) == 0 {
		return "日志"
	}
	last := g.logLines[len(g.logLines)-1]
	if len([]rune(last)) > 36 {
		runes := []rune(last)
		last = string(runes[:36]) + "..."
	}
	return "日志 · " + last
}

func (g *GUIApp) logScrollAtBottom() bool {
	if g.logScroll == nil || g.logScroll.Content == nil {
		return true
	}
	maxY := g.logScroll.Content.MinSize().Height - g.logScroll.Size().Height
	if maxY <= 0 {
		return true
	}
	return g.logScroll.Offset.Y >= maxY-4
}

func (g *GUIApp) scrollLogToBottom() {
	if g.logScroll == nil {
		return
	}
	g.logProgrammaticScroll = true
	g.logScroll.ScrollToBottom()
	g.logScroll.Refresh()
	go func() {
		time.Sleep(80 * time.Millisecond)
		fyne.Do(func() {
			g.logProgrammaticScroll = false
			g.logFollow = true
		})
	}()
}

func (g *GUIApp) activeDialogWindow() fyne.Window {
	if g.controlWindow != nil {
		return g.controlWindow
	}
	return g.window
}

func (g *GUIApp) confirmAction(title, message string, ok func()) {
	g.confirmActionWithLabels(title, message, "确认", "取消", ok)
}

func (g *GUIApp) confirmActionWithLabels(title, message, confirmLabel, cancelLabel string, ok func()) {
	body := g.dialogMessageContent(message, fyne.NewSize(640, 220))
	sourceButton := g.pendingActionButton
	sourceLabel := g.pendingActionButtonLabel

	var d dialog.Dialog
	cancelButton := widget.NewButton(cancelLabel, func() {
		d.Hide()
	})
	cancelButton.Importance = widget.MediumImportance
	confirmButton := widget.NewButton(confirmLabel, func() {
		d.Hide()
		if sourceButton != nil {
			g.pendingActionButton = sourceButton
			g.pendingActionButtonLabel = sourceLabel
		}
		ok()
	})
	confirmButton.Importance = widget.HighImportance

	content := container.NewGridWrap(fyne.NewSize(720, 360), container.NewBorder(
		nil,
		container.NewCenter(container.NewHBox(cancelButton, confirmButton)),
		nil,
		nil,
		container.NewPadded(body),
	))
	d = dialog.NewCustomWithoutButtons(title, content, g.activeDialogWindow())
	d.Show()
}

func (g *GUIApp) showActionDialog(title, actionLabel string, destructive bool, content fyne.CanvasObject, action func()) {
	var d dialog.Dialog
	cancelButton := widget.NewButton("取消", func() {
		d.Hide()
	})
	cancelButton.Importance = widget.MediumImportance
	actionButton := widget.NewButton(actionLabel, func() {
		d.Hide()
		action()
	})
	if destructive {
		actionButton.Importance = widget.DangerImportance
	} else {
		actionButton.Importance = widget.HighImportance
	}
	g.prepareActionButton(actionButton)

	wrapped := container.NewBorder(
		nil,
		container.NewCenter(container.NewHBox(cancelButton, actionButton)),
		nil,
		nil,
		content,
	)
	d = dialog.NewCustomWithoutButtons(title, wrapped, g.activeDialogWindow())
	d.Show()
}

func (g *GUIApp) showInfo(message string) {
	g.appendLog("INFO", "%s", message)
	g.showMessageDialog("提示", message, false)
}

func (g *GUIApp) showError(err error) {
	if core.IsAccessibilityPermissionRequired(err) {
		g.appendLog("INFO", "%v", err)
		g.showAccessibilityPermissionDialog(err.Error())
		return
	}
	g.appendLog("ERROR", "%v", err)
	g.showMessageDialog("错误", err.Error(), true)
}

func (g *GUIApp) showAccessibilityPermissionDialog(message string) {
	if message == "" {
		message = "排列和控制外部窗口需要 macOS 辅助功能权限。\n\n" + core.AccessibilityPermissionGuide
	}
	body := g.dialogMessageContent(message, fyne.NewSize(640, 220))
	var d dialog.Dialog
	openButton := widget.NewButton("打开系统设置", func() {
		settingsURL, err := url.Parse(core.AccessibilitySettingsURL)
		if err == nil {
			err = g.app.OpenURL(settingsURL)
		}
		if err != nil {
			g.showError(fmt.Errorf("打开系统设置失败：%w", err))
		}
	})
	openButton.Importance = widget.HighImportance
	closeButton := widget.NewButton("稍后", func() { d.Hide() })
	checkButton := widget.NewButton("已开启，重新检查", func() {
		d.Hide()
		g.checkAccessibilityAndResume()
	})
	content := container.NewBorder(nil, container.NewCenter(container.NewHBox(closeButton, checkButton, openButton)), nil, nil, container.NewPadded(body))
	d = dialog.NewCustomWithoutButtons("开启辅助功能权限", content, g.activeDialogWindow())
	d.Resize(fyne.NewSize(680, 320))
	d.Show()
}

func (g *GUIApp) showMessageDialog(title, message string, danger bool) {
	body := g.dialogMessageContent(message, fyne.NewSize(640, 220))

	var d dialog.Dialog
	okButton := widget.NewButton("关闭", func() {
		d.Hide()
	})
	okButton.Importance = widget.HighImportance

	content := container.NewGridWrap(fyne.NewSize(720, 340), container.NewBorder(
		nil,
		container.NewCenter(okButton),
		nil,
		nil,
		body,
	))
	d = dialog.NewCustomWithoutButtons(title, content, g.activeDialogWindow())
	d.Show()
}

func (g *GUIApp) focusNativeEmulatorWindow(entryKey, label string) {
	g.runAction("打开外部窗口 "+label, func() error {
		return g.backend.GUIFocusDeviceWindow(entryKey)
	})
}

func (g *GUIApp) openIndependentDeviceWindow(entryKey, label string) {
	g.runAction("打开独立窗口 "+label, func() error {
		return g.backend.GUIOpenLiveMirror(entryKey)
	})
}

func (g *GUIApp) tapDevicePreview(entryKey, serial string, pos fyne.Position, size fyne.Size, imageSize image.Point) {
	x, y, ok := previewTapToDevice(pos, size, imageSize)
	if !ok {
		g.appendLog("WARN", "点击未命中设备画面 %s：pos=%.0f,%.0f size=%.0fx%.0f image=%dx%d", serial, pos.X, pos.Y, size.Width, size.Height, imageSize.X, imageSize.Y)
		if card, exists := g.controlCards[entryKey]; exists {
			card.status.SetText(fmt.Sprintf("%s | 点击未命中画面 %.0f,%.0f", card.detail, pos.X, pos.Y))
		}
		return
	}
	g.appendLog("INFO", "点击设备 %s：%d,%d", serial, x, y)
	useRealtime := false
	if card, ok := g.controlCards[entryKey]; ok && card.realtime {
		useRealtime = true
	}
	go func() {
		method := "adb"
		var realtimeErr error
		var err error
		if useRealtime {
			method = "内嵌触控"
			err = g.backend.GUITapEmulatorRealtime(entryKey, x, y)
			if err != nil {
				realtimeErr = err
				method = "adb"
				err = g.backend.GUITapDevice(serial, x, y)
			}
		} else {
			err = g.backend.GUITapDevice(serial, x, y)
		}
		fyne.Do(func() {
			if err != nil {
				g.showError(fmt.Errorf("点击设备 %s 失败：%w", serial, err))
				return
			}
			if realtimeErr != nil {
				g.appendLog("WARN", "内嵌触控失败，已回退 adb：%s：%v", serial, realtimeErr)
			}
			if card, ok := g.controlCards[entryKey]; ok {
				card.status.SetText(fmt.Sprintf("%s | 已点击 %d,%d (%s)", card.detail, x, y, method))
			}
		})
		time.Sleep(350 * time.Millisecond)
		fyne.Do(func() {
			if card, ok := g.controlCards[entryKey]; ok && !card.realtime {
				g.refreshControlCardAsync(entryKey, card, nil)
			}
		})
	}()
}

func (g *GUIApp) swipeDevicePreview(entryKey, serial string, start, end fyne.Position, size fyne.Size, imageSize image.Point) {
	x1, y1, x2, y2, ok := previewSwipeToDevice(start, end, size, imageSize)
	if !ok {
		g.appendLog("WARN", "滑动未命中设备画面 %s：start=%.0f,%.0f end=%.0f,%.0f size=%.0fx%.0f image=%dx%d", serial, start.X, start.Y, end.X, end.Y, size.Width, size.Height, imageSize.X, imageSize.Y)
		if card, exists := g.controlCards[entryKey]; exists {
			card.status.SetText(fmt.Sprintf("%s | 滑动未命中画面 %.0f,%.0f", card.detail, start.X, start.Y))
		}
		return
	}
	durationMs := previewSwipeDurationMs(start, end)
	g.appendLog("INFO", "滑动设备 %s：%d,%d -> %d,%d", serial, x1, y1, x2, y2)
	useRealtime := false
	if card, ok := g.controlCards[entryKey]; ok && card.realtime {
		useRealtime = true
	}
	go func() {
		method := "adb"
		var realtimeErr error
		var err error
		if useRealtime {
			method = "内嵌触控"
			err = g.backend.GUISwipeEmulatorRealtime(entryKey, x1, y1, x2, y2, durationMs)
			if err != nil {
				realtimeErr = err
				method = "adb"
				err = g.backend.GUISwipeDevice(serial, x1, y1, x2, y2, durationMs)
			}
		} else {
			err = g.backend.GUISwipeDevice(serial, x1, y1, x2, y2, durationMs)
		}
		fyne.Do(func() {
			if err != nil {
				g.showError(fmt.Errorf("滑动设备 %s 失败：%w", serial, err))
				return
			}
			if realtimeErr != nil {
				g.appendLog("WARN", "内嵌滑动失败，已回退 adb：%s：%v", serial, realtimeErr)
			}
			if card, ok := g.controlCards[entryKey]; ok {
				card.status.SetText(fmt.Sprintf("%s | 已滑动 (%s)", card.detail, method))
			}
		})
		time.Sleep(350 * time.Millisecond)
		fyne.Do(func() {
			if card, ok := g.controlCards[entryKey]; ok && !card.realtime {
				g.refreshControlCardAsync(entryKey, card, nil)
			}
		})
	}()
}

func (g *GUIApp) keyEventDevice(serial, label string, keyCode int) {
	g.runDeviceControl(serial, label, func() error {
		return g.backend.GUIKeyEventDevice(serial, keyCode)
	})
}

func (g *GUIApp) backDevicePreview(serial string) {
	g.keyEventDevice(serial, "右键返回", 4)
}

func (g *GUIApp) statusBarDevice(serial, label, action string) {
	g.runDeviceControl(serial, label, func() error {
		return g.backend.GUIStatusBarDevice(serial, action)
	})
}

func (g *GUIApp) runDeviceControl(serial, label string, fn func() error) {
	g.appendLog("INFO", "%s：%s", label, serial)
	go func() {
		err := fn()
		fyne.Do(func() {
			if err != nil {
				g.showError(fmt.Errorf("%s失败：%w", label, err))
				return
			}
			g.appendLog("DONE", "%s：%s", label, serial)
		})
		time.Sleep(300 * time.Millisecond)
		fyne.Do(func() {
			g.refreshControlScreensAsync()
		})
	}()
}

func previewTapToDevice(pos fyne.Position, size fyne.Size, imageSize image.Point) (int, int, bool) {
	if size.Width <= 0 || size.Height <= 0 || imageSize.X <= 0 || imageSize.Y <= 0 {
		return 0, 0, false
	}
	scaleX := size.Width / float32(imageSize.X)
	scaleY := size.Height / float32(imageSize.Y)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	drawW := float32(imageSize.X) * scale
	drawH := float32(imageSize.Y) * scale
	offsetX := (size.Width - drawW) / 2
	offsetY := (size.Height - drawH) / 2
	if pos.X < offsetX || pos.X > offsetX+drawW || pos.Y < offsetY || pos.Y > offsetY+drawH {
		return 0, 0, false
	}
	x := int((pos.X - offsetX) / scale)
	y := int((pos.Y - offsetY) / scale)
	if x < 0 || y < 0 || x >= imageSize.X || y >= imageSize.Y {
		return 0, 0, false
	}
	return x, y, true
}

func previewSwipeToDevice(start, end fyne.Position, size fyne.Size, imageSize image.Point) (int, int, int, int, bool) {
	x1, y1, ok := previewTapToDevice(start, size, imageSize)
	if !ok {
		return 0, 0, 0, 0, false
	}
	x2, y2, ok := previewTapToDevice(end, size, imageSize)
	if !ok {
		return 0, 0, 0, 0, false
	}
	return x1, y1, x2, y2, true
}

func previewSwipeDurationMs(start, end fyne.Position) int {
	dx := absFloat32(end.X - start.X)
	dy := absFloat32(end.Y - start.Y)
	if dx > 160 || dy > 160 {
		return 450
	}
	return 260
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func controlCardSize(width, previewHeight float32) fyne.Size {
	return fyne.NewSize(
		width,
		previewHeight+controlCardHeaderHeight+controlCardStatusHeight+controlCardButtonRowsHeight+controlCardPaddingHeight,
	)
}

func absFloat32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func compactButton(label string, fn func()) *widget.Button {
	button := widget.NewButton(label, fn)
	button.Alignment = widget.ButtonAlignCenter
	button.Importance = widget.LowImportance
	return button
}

// Keep actions on their own row so a larger device card cannot crowd the
// density selector and neighbouring buttons out of the toolbar.
func controlToolbar(heading, actions fyne.CanvasObject) *fyne.Container {
	scroll := container.NewHScroll(actions)
	scroll.SetMinSize(fyne.NewSize(120, controlCompactControlHeight))
	return container.NewBorder(heading, nil, nil, nil, scroll)
}

func compactSelectBox(selectWidget *widget.Select, width float32) fyne.CanvasObject {
	return container.NewGridWrap(
		fyne.NewSize(width, controlCompactControlHeight),
		selectWidget,
	)
}

func compactButtonBox(button *widget.Button, width float32) fyne.CanvasObject {
	button.Alignment = widget.ButtonAlignCenter
	return container.NewGridWrap(fyne.NewSize(width, controlCompactControlHeight), button)
}

func (g *GUIApp) compactActionButton(label string, fn func()) *widget.Button {
	button := compactButton(label, fn)
	g.prepareActionButton(button)
	return button
}

func (g *GUIApp) controlActionButton(label string, fn func()) fyne.CanvasObject {
	return g.compactActionButton(label, fn)
}

func compactStatus(status *widget.Label, width float32) fyne.CanvasObject {
	if width < 96 {
		width = 96
	}
	return container.New(centeredControlLayout{width: width}, status)
}

func footerStatus(status *widget.Label, width, opticalOffset float32) fyne.CanvasObject {
	return container.New(footerStatusLayout{width: width, opticalOffset: opticalOffset}, status)
}

// Align tool/task glyphs with actions and target labels without shifting
// their shared control bounds or horizontal insets.
type footerStatusLayout struct{ width, opticalOffset float32 }

func (l footerStatusLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.width, controlCompactControlHeight)
}

func (l footerStatusLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		label := object.(*widget.Label)
		height := label.MinSize().Height
		label.Resize(fyne.NewSize(size.Width, height))
		label.Move(fyne.NewPos(0, (size.Height-height)/2+l.opticalOffset))
	}
}

func controlButtonRows(buttons []fyne.CanvasObject, width float32) fyne.CanvasObject {
	if len(buttons) == 0 {
		return widget.NewLabel("")
	}
	cellWidth := float32(74)
	for _, button := range buttons {
		if w := button.MinSize().Width; w > cellWidth {
			cellWidth = w
		}
	}
	columns := int((width + theme.Padding()) / (cellWidth + theme.Padding()))
	if columns < 1 {
		columns = 1
	}
	return container.NewGridWithColumns(columns, buttons...)
}

func (g *GUIApp) selectedControlKeys() []string {
	var keys []string
	for _, entry := range g.entries {
		if g.controlSelected[entry.Key] {
			keys = append(keys, entry.Key)
		}
	}
	return keys
}

func (g *GUIApp) selectedReadyControlKeys() []string {
	var keys []string
	for _, entry := range g.entries {
		if g.controlSelected[entry.Key] && entry.Active != nil && entry.Active.State == "device" {
			keys = append(keys, entry.Key)
		}
	}
	return keys
}

func (g *GUIApp) readyControlKeys() []string {
	var keys []string
	for _, entry := range g.entries {
		if entry.Active != nil && entry.Active.State == "device" {
			keys = append(keys, entry.Key)
		}
	}
	return keys
}

func (g *GUIApp) selectedStoppedAVDNames() []string {
	var names []string
	for _, entry := range g.entries {
		if g.controlSelected[entry.Key] && entry.AVD != nil && !entry.Running {
			names = append(names, entry.AVD.Name)
		}
	}
	return names
}

func (g *GUIApp) selectedRunningEmulatorKeys() []string {
	var keys []string
	for _, entry := range g.entries {
		if g.controlSelected[entry.Key] && entry.Active != nil && entry.Active.IsEmulator && entry.Active.State == "device" {
			keys = append(keys, entry.Key)
		}
	}
	return keys
}

func (g *GUIApp) selectControlEntries(match func(core.DeviceEntry) bool) {
	if g.controlSelected == nil {
		g.controlSelected = map[string]bool{}
	}
	for _, entry := range g.entries {
		g.controlSelected[entry.Key] = match(entry)
	}
	g.updateSelectedLabel()
	g.renderControlCenter()
}

func (g *GUIApp) showCreateAVDDialog() {
	g.runModalLoad("读取创建模拟器选项", func() (any, error) {
		templates, err := g.backend.GUIDeviceTemplates()
		if err != nil {
			return nil, err
		}
		images, err := g.backend.GUIInstalledSystemImages()
		if err != nil {
			return nil, err
		}
		return struct {
			templates []core.DeviceTemplate
			images    []string
		}{templates: templates, images: images}, nil
	}, func(value any) {
		data := value.(struct {
			templates []core.DeviceTemplate
			images    []string
		})
		if len(data.images) == 0 {
			g.showInstallSystemImageDialog()
			return
		}
		templateByLabel := map[string]string{}
		var templateOptions []string
		for _, item := range data.templates {
			label := item.Name + " (" + item.ID + ")"
			templateOptions = append(templateOptions, label)
			templateByLabel[label] = deviceTemplateCreateIDForGUI(item)
		}
		nameEntry := widget.NewEntry()
		nameEntry.SetText("adm_" + time.Now().Format("20060102_1504"))
		templateSelect := widget.NewSelect(templateOptions, nil)
		if len(templateOptions) > 0 {
			templateSelect.SetSelected(templateOptions[0])
		}
		imageSelect := widget.NewSelect(data.images, nil)
		imageSelect.SetSelected(data.images[0])
		startCheck := widget.NewCheck("创建后启动", nil)
		startCheck.SetChecked(true)
		content := container.NewGridWrap(fyne.NewSize(760, 240),
			container.NewVBox(
				widget.NewLabel("名称"),
				nameEntry,
				widget.NewLabel("设备模板"),
				templateSelect,
				widget.NewLabel("镜像"),
				imageSelect,
				startCheck,
			),
		)
		g.showActionDialog("创建模拟器", "创建", false, content, func() {
			deviceID := templateByLabel[templateSelect.Selected]
			name, imageID, start := strings.TrimSpace(nameEntry.Text), imageSelect.Selected, startCheck.Checked
			if name == "" {
				g.showInfo("请输入模拟器名称。")
				return
			}
			g.runAction("创建模拟器 "+name, func() error {
				return g.backend.GUICreateAVD(name, imageID, deviceID, start)
			})
		})
	})
}

func (g *GUIApp) selectedControlEntries() []core.DeviceEntry {
	var entries []core.DeviceEntry
	for _, entry := range g.entries {
		if g.controlSelected[entry.Key] {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (g *GUIApp) showSelectedDeleteAVDDialog() {
	plan := g.planSelectedAction(actionDelete, false)
	if !g.acceptActionPlan(plan) {
		return
	}
	names := plan.avdNames()
	running := false
	for _, entry := range plan.Entries {
		running = running || entry.Running
	}
	if len(names) == 0 {
		g.showInfo("请在设备墙勾选要删除的模拟器；真机不会被删除。")
		return
	}
	warning := wrappedLabel("将删除以下勾选模拟器的配置和数据：\n" + plan.summary())
	list := container.NewVScroll(wrappedLabel(strings.Join(names, "\n")))
	list.SetMinSize(fyne.NewSize(560, 160))
	confirm := widget.NewCheck("我确认删除以上模拟器的配置和数据", nil)
	forceClose := widget.NewCheck("关闭正在运行的模拟器后删除", nil)
	if !running {
		forceClose.Hide()
	}
	content := container.NewVBox(warning, list, forceClose, confirm)
	g.showActionDialog("确认删除选中模拟器", "删除", true, content, func() {
		if !confirm.Checked {
			g.showInfo("请先勾选确认删除。")
			return
		}
		if running && !forceClose.Checked {
			g.showInfo("选中项包含运行中或 offline 的模拟器，请确认关闭后删除。")
			return
		}
		force := forceClose.Checked
		g.runEntryBatchAction("批量删除模拟器", plan.Entries, func(entry core.DeviceEntry) error {
			return g.backend.GUIDeleteAVDs([]string{entry.AVD.Name}, true, force)
		})
	})
}

func deviceTemplateCreateIDForGUI(template core.DeviceTemplate) string {
	if strings.TrimSpace(template.CreateID) != "" {
		return template.CreateID
	}
	return template.ID
}

func (g *GUIApp) runModalLoad(name string, fn func() (any, error), done func(any)) {
	btn := g.beginTask(name)
	go func() {
		value, err := fn()
		fyne.Do(func() {
			g.endTask(btn)
			if g.closed {
				return
			}
			if err != nil {
				g.showError(err)
				return
			}
			done(value)
		})
	}()
}

func entryDetail(entry core.DeviceEntry) string {
	var parts []string
	if entry.Active != nil && entry.Active.Serial != "" {
		parts = append(parts, entry.Active.Serial)
	}
	if entry.AVD != nil && entry.AVD.Device != "" {
		parts = append(parts, entry.AVD.Device)
	}
	if len(parts) == 0 {
		return entry.Label
	}
	return strings.Join(parts, " | ")
}

func entryStatus(entry core.DeviceEntry) string {
	if entry.Active != nil {
		switch entry.Active.State {
		case "device":
			return "可用"
		case "offline":
			return "离线"
		case "unauthorized":
			return "未授权"
		default:
			return entry.Active.State
		}
	}
	return "未启动"
}

func controlCardTitle(entry core.DeviceEntry) string {
	if entry.AVD != nil {
		return entry.AVD.Name
	}
	if entry.Active != nil {
		if model := entry.Active.Details["model"]; model != "" {
			return model
		}
		return entry.Active.Serial
	}
	return entry.Label
}

func controlCardStatus(entry core.DeviceEntry) string {
	var parts []string
	parts = append(parts, entry.Kind)
	parts = append(parts, entryStatus(entry))
	if entry.Key != "" {
		parts = append(parts, entry.Key)
	}
	if entry.Active != nil {
		if version := entry.Active.Details["product"]; version != "" {
			parts = append(parts, "product="+version)
		}
	}
	return strings.Join(parts, " | ")
}

func controlCardIdentity(entry core.DeviceEntry) string {
	serial := "未启动"
	if entry.Active != nil && entry.Active.Serial != "" {
		serial = entry.Active.Serial
	}
	model := "unknown"
	if entry.AVD != nil && entry.AVD.Device != "" {
		model = compactDeviceModel(entry.AVD.Device)
	} else if entry.Active != nil {
		if product := entry.Active.Details["product"]; product != "" {
			model = compactDeviceModel(product)
		}
	}
	return fmt.Sprintf("%s | %s", serial, model)
}

func controlCardCopySerial(entry core.DeviceEntry) string {
	if entry.Active != nil && strings.TrimSpace(entry.Active.Serial) != "" {
		return entry.Active.Serial
	}
	if entry.AVD != nil && strings.TrimSpace(entry.AVD.Name) != "" {
		return entry.AVD.Name
	}
	return entry.Label
}

func compactDeviceModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "unknown"
	}
	if i := strings.Index(model, " ("); i > 0 {
		model = model[:i]
	}
	if len([]rune(model)) <= 18 {
		return model
	}
	runes := []rune(model)
	return string(runes[:18]) + "..."
}

func controlCardCompactStatus(entry core.DeviceEntry, suffix string) string {
	parts := []string{controlCardIdentity(entry)}
	if suffix != "" {
		parts = append(parts, compactStatusSuffix(suffix))
	} else if entry.Active != nil {
		switch entry.Active.State {
		case "device":
			parts = append(parts, "在线")
		case "offline":
			parts = append(parts, "offline")
		case "unauthorized":
			parts = append(parts, "未授权")
		default:
			parts = append(parts, entry.Active.State)
		}
	} else if entry.Running {
		parts = append(parts, "等待 adb")
	} else {
		parts = append(parts, "未启动")
	}
	return strings.Join(parts, " | ")
}

func compactStatusSuffix(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if len([]rune(text)) <= 18 {
		return text
	}
	runes := []rune(text)
	return string(runes[:18]) + "..."
}

func compactToolStatus(statuses []core.ToolStatus) string {
	var parts []string
	for _, status := range statuses {
		if status.Available {
			parts = append(parts, status.Name+" ✅")
		} else {
			parts = append(parts, status.Name+" ❌")
		}
	}
	return strings.Join(parts, "    ")
}

func missingInstallableToolStatuses(statuses []core.ToolStatus) []core.ToolStatus {
	var missing []core.ToolStatus
	for _, status := range statuses {
		if status.Available || !isInstallableToolName(status.Name) {
			continue
		}
		missing = append(missing, status)
	}
	return missing
}

func isInstallableToolName(name string) bool {
	switch strings.TrimSpace(name) {
	case "adb", "emulator", "avdmanager", "sdkmanager", "scrcpy":
		return true
	default:
		return false
	}
}

func toolStatusNames(statuses []core.ToolStatus) []string {
	names := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if strings.TrimSpace(status.Name) != "" {
			names = append(names, status.Name)
		}
	}
	return names
}

func toolHealthSummary(missing []core.ToolStatus, bootstrap core.DependencyBootstrapStatus) string {
	if len(missing) == 0 {
		return "运行依赖已就绪。需要重装或修复时，也可以手动执行安装/修复。"
	}
	text := fmt.Sprintf("缺失 %d 项运行依赖：%s。", len(missing), strings.Join(toolStatusNames(missing), "、"))
	if bootstrap.Supported {
		return text + " 可直接点击安装/修复依赖。"
	}
	return text + " " + bootstrap.Error
}

func bootstrapHint(bootstrap core.DependencyBootstrapStatus) string {
	if bootstrap.Supported {
		return "安装脚本：" + compactPathMiddle(bootstrap.Script, 96)
	}
	if strings.TrimSpace(bootstrap.Error) != "" {
		return bootstrap.Error
	}
	return "未找到可用安装入口。"
}

func compactMultiline(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return text
	}
	head := maxRunes / 2
	tail := maxRunes - head
	return string(runes[:head]) + "\n...\n" + string(runes[len(runes)-tail:])
}

func detailedToolStatus(statuses []core.ToolStatus) string {
	var lines []string
	for _, status := range statuses {
		if status.Available {
			lines = append(lines, fmt.Sprintf("%s: %s (%s)", status.Name, status.Path, status.Source))
		} else {
			lines = append(lines, fmt.Sprintf("%s: 不可用（%s）", status.Name, status.Error))
		}
	}
	return strings.Join(lines, "\n")
}

func needsADBKeyboard(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < 32 || r > 126 {
			return true
		}
	}
	return false
}
