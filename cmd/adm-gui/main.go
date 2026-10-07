package main

import (
	core "adm/internal/app"
	"bytes"
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
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const maxLogLines = 200
const controlPreviewInterval = 2 * time.Second
const controlDensitySmall = "small"
const controlDensityStandard = "standard"
const controlDensityHD = "hd"
const controlCardHeaderHeight float32 = 34
const controlCardStatusHeight float32 = 34
const controlCardButtonRowsHeight float32 = 96
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

var controlDensityOptions = []controlDensitySpec{
	{key: controlDensitySmall, label: "小", cardSize: controlCardSize(340, 584), previewSize: fyne.NewSize(300, 584)},
	{key: controlDensityStandard, label: "标准", cardSize: controlCardSize(410, 724), previewSize: fyne.NewSize(360, 724)},
	{key: controlDensityHD, label: "高清", cardSize: controlCardSize(500, 920), previewSize: fyne.NewSize(440, 920)},
}

type GUIApp struct {
	backend *core.App
	app     fyne.App
	window  fyne.Window

	// activeTasks counts in-flight background actions. Actions run
	// concurrently — the backend serializes its own shared state — so instead
	// of a single global "busy" lock the UI just tracks how many tasks are
	// running to drive the progress indicator, and disables only the button
	// that launched each action.
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

	currentLabel          *widget.Label
	selectedLabel         *widget.Label
	toolSummary           *widget.Label
	busyLabel             *widget.Label
	progress              *widget.Activity
	logLabel              *widget.Label
	logScroll             *container.Scroll
	logFollow             bool
	logProgrammaticScroll bool
	logLines              []string

	controlWindow        fyne.Window
	controlGrid          *fyne.Container
	controlSummary       *widget.Label
	controlDensity       string
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

	hDock          *fyne.Container
	vDock          *fyne.Container
	logStack       *fyne.Container
	logRail        fyne.CanvasObject
	logRailSummary *canvas.Text
	rightHandle    *resizeHandle
	logHandle      *resizeHandle
	rightPanel     fyne.CanvasObject
	logPanel       fyne.CanvasObject

	// Edge icon bars (always visible) and their tappable icons. The icons are
	// the single collapse/expand control per pane: highlighted when the pane
	// is open, dimmed when collapsed. See toolIcon in dock.go.
	rightIconBar  fyne.CanvasObject
	logToolIcon   *toolIcon
	installIcon   *toolIcon
	uninstallIcon *toolIcon
	messageIcon   *toolIcon

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

	actionButtons            []*widget.Button
	preparedActionButtons    map[*widget.Button]bool
	pendingActionButton      *widget.Button
	pendingActionButtonLabel string
	// loadingButtons tracks buttons currently showing a "…" loading state,
	// mapping each to its original label so it can be restored. Multiple
	// buttons can load at once because actions run concurrently.
	loadingButtons map[*widget.Button]string
}

type admTheme struct {
	base fyne.Theme
}

var (
	admColorAppBG      = color.NRGBA{R: 9, G: 14, B: 25, A: 255}
	admColorPanelBG    = color.NRGBA{R: 15, G: 23, B: 42, A: 255}
	admColorPanelBG2   = color.NRGBA{R: 17, G: 24, B: 39, A: 255}
	admColorBorder     = color.NRGBA{R: 51, G: 65, B: 85, A: 255}
	admColorText       = color.NRGBA{R: 226, G: 232, B: 240, A: 255}
	admColorMuted      = color.NRGBA{R: 148, G: 163, B: 184, A: 255}
	admColorPrimary    = color.NRGBA{R: 37, G: 99, B: 235, A: 255}
	admColorDanger     = color.NRGBA{R: 239, G: 68, B: 68, A: 255}
	admColorSuccess    = color.NRGBA{R: 16, G: 185, B: 129, A: 255}
	admColorPreviewBG  = color.NRGBA{R: 3, G: 7, B: 18, A: 255}
	admColorPreviewAlt = color.NRGBA{R: 8, G: 13, B: 24, A: 255}
)

func (t admTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground, theme.ColorNameHeaderBackground, theme.ColorNameMenuBackground:
		return admColorAppBG
	case theme.ColorNameButton:
		return color.NRGBA{R: 34, G: 50, B: 72, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 22, G: 32, B: 48, A: 255}
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		return admColorMuted
	case theme.ColorNameError:
		return admColorDanger
	case theme.ColorNameForeground, theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError:
		return admColorText
	case theme.ColorNameFocus, theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return admColorPrimary
	case theme.ColorNameHover, theme.ColorNamePressed:
		return color.NRGBA{R: 51, G: 65, B: 85, A: 210}
	case theme.ColorNameInputBackground, theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 11, G: 20, B: 36, A: 255}
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return color.NRGBA{R: 82, G: 97, B: 122, A: 255}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 100, G: 116, B: 139, A: 220}
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 15, G: 23, B: 42, A: 120}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 37, G: 99, B: 235, A: 120}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 90}
	case theme.ColorNameSuccess:
		return admColorSuccess
	case theme.ColorNameWarning:
		return color.NRGBA{R: 245, G: 158, B: 11, A: 255}
	default:
		return t.base.Color(name, theme.VariantDark)
	}
}

func (t admTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t admTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t admTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameText:
		return 14
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameSeparatorThickness:
		return 1
	default:
		return t.base.Size(name)
	}
}

type controlCardView struct {
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

type controlCardSnapshot struct {
	image       image.Image
	imageWidth  int
	imageHeight int
	status      string
	message     string
}

type packagePicker struct {
	all      []string
	filter   string
	filtered []string
	selected string
	list     *widget.List
}

type previewPane struct {
	widget.BaseWidget
	bg             *canvas.Rectangle
	img            *canvas.Image
	message        *widget.Label
	messageIcon    *canvas.Text
	minSize        fyne.Size
	imageWidth     int
	imageHeight    int
	onTap          func(fyne.Position, fyne.Size, image.Point)
	onSecondaryTap func()
	onSwipe        func(fyne.Position, fyne.Position, fyne.Size, image.Point)
	dragging       bool
	dragStart      fyne.Position
	dragEnd        fyne.Position
}

type previewInputLayer struct {
	widget.BaseWidget
	preview   *previewPane
	dragging  bool
	dragStart fyne.Position
	dragEnd   fyne.Position
}

type copyTapLayer struct {
	widget.BaseWidget
	onTap func()
}

type previewPaneRenderer struct {
	pane    *previewPane
	objects []fyne.CanvasObject
}

func newPreviewPane(message string, minSize fyne.Size) *previewPane {
	p := &previewPane{
		bg:          canvas.NewRectangle(admColorPreviewBG),
		img:         canvas.NewImageFromImage(nil),
		message:     widget.NewLabel(message),
		messageIcon: canvas.NewText("", admColorText),
		minSize:     minSize,
	}
	p.img.FillMode = canvas.ImageFillContain
	p.img.ScaleMode = canvas.ImageScaleSmooth
	p.message.Alignment = fyne.TextAlignCenter
	p.messageIcon.TextSize = 38
	p.messageIcon.Hide()
	p.ExtendBaseWidget(p)
	return p
}

func newPreviewInputLayer(preview *previewPane) *previewInputLayer {
	layer := &previewInputLayer{preview: preview}
	layer.ExtendBaseWidget(layer)
	return layer
}

func newCopyTapLayer(onTap func()) *copyTapLayer {
	layer := &copyTapLayer{onTap: onTap}
	layer.ExtendBaseWidget(layer)
	return layer
}

func (l *copyTapLayer) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(color.NRGBA{A: 1})
	return widget.NewSimpleRenderer(rect)
}

func (l *copyTapLayer) MinSize() fyne.Size {
	return fyne.NewSize(1, 1)
}

func (l *copyTapLayer) Tapped(*fyne.PointEvent) {
	if l.onTap != nil {
		l.onTap()
	}
}

func (l *copyTapLayer) TappedSecondary(*fyne.PointEvent) {
	if l.onTap != nil {
		l.onTap()
	}
}

func (l *previewInputLayer) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(color.NRGBA{A: 1})
	return widget.NewSimpleRenderer(rect)
}

func (l *previewInputLayer) MinSize() fyne.Size {
	if l.preview == nil {
		return fyne.NewSize(1, 1)
	}
	return l.preview.minSize
}

func (l *previewInputLayer) Tapped(ev *fyne.PointEvent) {
	if l.preview == nil || l.preview.onTap == nil || l.preview.imageWidth <= 0 || l.preview.imageHeight <= 0 {
		return
	}
	l.preview.onTap(ev.Position, l.Size(), image.Pt(l.preview.imageWidth, l.preview.imageHeight))
}

func (l *previewInputLayer) TappedSecondary(*fyne.PointEvent) {
	if l.preview == nil || l.preview.onSecondaryTap == nil {
		return
	}
	l.preview.onSecondaryTap()
}

func (l *previewInputLayer) Dragged(ev *fyne.DragEvent) {
	if l.preview == nil || l.preview.imageWidth <= 0 || l.preview.imageHeight <= 0 {
		return
	}
	if !l.dragging {
		l.dragStart = ev.Position.Subtract(fyne.NewDelta(ev.Dragged.DX, ev.Dragged.DY))
		l.dragging = true
	}
	l.dragEnd = ev.Position
}

func (l *previewInputLayer) DragEnd() {
	if !l.dragging {
		return
	}
	start := l.dragStart
	end := l.dragEnd
	l.dragging = false
	if l.preview == nil || l.preview.onSwipe == nil || l.preview.imageWidth <= 0 || l.preview.imageHeight <= 0 {
		return
	}
	if absFloat32(end.X-start.X) < 4 && absFloat32(end.Y-start.Y) < 4 {
		return
	}
	l.preview.onSwipe(start, end, l.Size(), image.Pt(l.preview.imageWidth, l.preview.imageHeight))
}

func previewInteractiveObject(preview *previewPane) fyne.CanvasObject {
	return container.NewStack(preview, newPreviewInputLayer(preview))
}

func (p *previewPane) CreateRenderer() fyne.WidgetRenderer {
	return &previewPaneRenderer{
		pane:    p,
		objects: []fyne.CanvasObject{p.bg, p.img, p.message, p.messageIcon},
	}
}

func (p *previewPane) Tapped(ev *fyne.PointEvent) {
	if p.onTap == nil || p.imageWidth <= 0 || p.imageHeight <= 0 {
		return
	}
	p.onTap(ev.Position, p.Size(), image.Pt(p.imageWidth, p.imageHeight))
}

func (p *previewPane) TappedSecondary(*fyne.PointEvent) {
	if p.onSecondaryTap == nil {
		return
	}
	p.onSecondaryTap()
}

func (p *previewPane) Dragged(ev *fyne.DragEvent) {
	if p.imageWidth <= 0 || p.imageHeight <= 0 {
		return
	}
	if !p.dragging {
		p.dragStart = ev.Position.Subtract(fyne.NewDelta(ev.Dragged.DX, ev.Dragged.DY))
		p.dragging = true
	}
	p.dragEnd = ev.Position
}

func (p *previewPane) DragEnd() {
	if !p.dragging {
		return
	}
	start := p.dragStart
	end := p.dragEnd
	p.dragging = false
	if p.onSwipe == nil || p.imageWidth <= 0 || p.imageHeight <= 0 {
		return
	}
	if absFloat32(end.X-start.X) < 4 && absFloat32(end.Y-start.Y) < 4 {
		return
	}
	p.onSwipe(start, end, p.Size(), image.Pt(p.imageWidth, p.imageHeight))
}

func (p *previewPane) setImage(img image.Image) {
	p.img.Image = img
	if img == nil {
		p.imageWidth = 0
		p.imageHeight = 0
		p.message.Show()
		p.messageIcon.Hide()
	} else {
		bounds := img.Bounds()
		p.imageWidth = bounds.Dx()
		p.imageHeight = bounds.Dy()
		p.message.Hide()
		p.messageIcon.Hide()
	}
	p.img.Refresh()
	p.Refresh()
}

func (p *previewPane) setMessage(message string) {
	p.img.Image = nil
	p.imageWidth = 0
	p.imageHeight = 0
	p.message.SetText(message)
	if message == "🙈" {
		p.message.Hide()
		p.messageIcon.Text = message
		p.messageIcon.Show()
	} else {
		p.message.Show()
		p.messageIcon.Hide()
	}
	p.img.Refresh()
	p.Refresh()
}

func (r *previewPaneRenderer) Layout(size fyne.Size) {
	r.pane.bg.Move(fyne.NewPos(0, 0))
	r.pane.bg.Resize(size)
	r.pane.img.Move(fyne.NewPos(0, 0))
	r.pane.img.Resize(size)
	centerObject(r.pane.message, size)
	centerObject(r.pane.messageIcon, size)
}

func (r *previewPaneRenderer) MinSize() fyne.Size {
	return r.pane.minSize
}

func (r *previewPaneRenderer) Refresh() {
	for _, obj := range r.objects {
		obj.Refresh()
	}
}

func (r *previewPaneRenderer) Destroy() {}

func (r *previewPaneRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func centerObject(obj fyne.CanvasObject, size fyne.Size) {
	min := obj.MinSize()
	if min.Width > size.Width {
		min.Width = size.Width
	}
	if min.Height > size.Height {
		min.Height = size.Height
	}
	obj.Resize(min)
	obj.Move(fyne.NewPos((size.Width-min.Width)/2, (size.Height-min.Height)/2))
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
	a := fyneapp.NewWithID("com.zhuiguang.advm")
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
	g.selectedLabel = widget.NewLabel("选中：未选择")
	g.toolSummary = widget.NewLabel("工具：检测中")
	g.busyLabel = widget.NewLabel("任务：空闲")
	g.progress = widget.NewActivity()
	g.progress.Hide()

	g.logLabel = widget.NewLabel("")
	g.logLabel.Wrapping = fyne.TextWrapBreak
	g.logLabel.TextStyle = fyne.TextStyle{Monospace: true}
	g.logFollow = true

	topBar := g.buildTopBar()
	wall := g.buildDeviceWallPanel()

	// The three right-side tool panels are built once and only toggled visible;
	// keeping them alive preserves their form state (entered APK path, package
	// filter, drafted text) across icon switches.
	g.installPanel = rightToolPanel(g.buildInstallPanel())
	g.uninstallPanel = rightToolPanel(g.buildUninstallPanel())
	g.messagePanel = rightToolPanel(g.buildMessagePanel())
	g.rightPanel = container.NewStack(g.installPanel, g.uninstallPanel, g.messagePanel)

	g.logPanel = g.buildLogPanel()

	// Keep the tool panel and log sizes stable when toggling them.
	if g.rightW == 0 {
		g.rightW = dockRightDefaultWidth
	}
	if g.logH == 0 {
		g.logH = dockLogDefaultHeight
	}

	g.rightIconBar = g.buildRightIconBar()
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
	root := container.New(edgeBarLayout{}, g.vDock, g.rightIconBar)
	g.applyWorkbenchCollapseState()

	page := container.NewBorder(topBar, nil, nil, nil, root)
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
		g.logToolIcon.setActive(!g.logCollapsed)
	}
	if g.installIcon != nil {
		g.installIcon.setActive(g.rightActive == "install")
	}
	if g.uninstallIcon != nil {
		g.uninstallIcon.setActive(g.rightActive == "uninstall")
	}
	if g.messageIcon != nil {
		g.messageIcon.setActive(g.rightActive == "message")
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
	g.installIcon = newToolIcon(iconInstall, false, func() { g.toggleRightPanel("install") })
	g.uninstallIcon = newToolIcon(iconUninstall, false, func() { g.toggleRightPanel("uninstall") })
	g.messageIcon = newToolIcon(iconMessage, false, func() { g.toggleRightPanel("message") })
	rebootIcon := newToolIcon(iconReboot, true, g.rebootCurrentDevice)
	closeIcon := newToolIcon(iconClose, true, g.closeCurrentDevice)
	g.logToolIcon = newToolIcon(iconLog, false, func() {
		g.logCollapsed = !g.logCollapsed
		g.applyWorkbenchCollapseState()
	})
	return iconBarColumn(
		g.installIcon, g.uninstallIcon, g.messageIcon,
		iconBarDivider(),
		rebootIcon, closeIcon, iconBarDivider(), g.logToolIcon,
	)
}

// iconBarColumn stacks edge-bar icons top-aligned on a panel surface.
func iconBarColumn(items ...fyne.CanvasObject) fyne.CanvasObject {
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
	// Expand/collapse is driven by the log icon in the right rail, so the rail
	// only shows the summary.
	return collapsedBar(container.NewBorder(nil, nil, g.logRailSummary, nil, nil))
}

func (g *GUIApp) buildTopBar() fyne.CanvasObject {
	toolDetailsButton := compactButton("工具", func() {
		g.showToolHealthDialog()
	})

	title := canvas.NewText("安卓设备矩阵", admColorText)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 16
	g.currentLabel.Wrapping = fyne.TextTruncate
	g.selectedLabel.Wrapping = fyne.TextTruncate
	g.toolSummary.Wrapping = fyne.TextTruncate
	g.busyLabel.Wrapping = fyne.TextTruncate
	targetBlock := container.NewHBox(
		compactStatus(g.currentLabel, topBarTargetWidth),
		compactStatus(g.selectedLabel, topBarSelectionWidth),
	)
	healthBlock := container.NewHBox(
		compactStatus(g.toolSummary, topBarToolWidth),
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
	footer := container.NewVBox(mutedText("Android SDK / Homebrew / PATH 检测结果"), container.NewCenter(container.NewHBox(permissionButton, installButton, refreshButton, closeButton)))
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

func toolStatusHeader() fyne.CanvasObject {
	return container.NewGridWithColumns(4,
		toolTableHeaderText("状态"),
		toolTableHeaderText("工具"),
		toolTableHeaderText("来源"),
		toolTableHeaderText("路径 / 错误"),
	)
}

func toolStatusTableRow(status core.ToolStatus) fyne.CanvasObject {
	state := "✓"
	stateColor := admColorSuccess
	source := sourceOrDefault(strings.TrimSpace(status.Source))
	detail := compactPathMiddle(strings.TrimSpace(status.Path), 58)
	if !status.Available {
		state = "!"
		stateColor = admColorDanger
		detail = compactPathMiddle(strings.TrimSpace(status.Error), 58)
		if detail == "" {
			detail = "不可用"
		}
	}
	icon := canvas.NewText(state, stateColor)
	icon.TextSize = 15
	icon.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewStack(
		roundedRect(color.NRGBA{R: 17, G: 24, B: 39, A: 255}, 7),
		container.NewPadded(container.NewGridWithColumns(4,
			container.NewCenter(icon),
			toolTableCellText(status.Name, true),
			toolTableCellText(source, false),
			toolTableCellText(detail, false),
		)),
	)
}

func toolTableHeaderText(text string) fyne.CanvasObject {
	label := canvas.NewText(text, admColorMuted)
	label.TextSize = 12
	label.TextStyle = fyne.TextStyle{Bold: true}
	return label
}

func toolTableCellText(text string, bold bool) fyne.CanvasObject {
	if strings.TrimSpace(text) == "" {
		text = "-"
	}
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextTruncate
	if bold {
		label.TextStyle = fyne.TextStyle{Bold: true}
	}
	return label
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
		container.NewPadded(content),
	)
}

func collapsedBar(content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(
		roundedRect(color.NRGBA{R: 10, G: 16, B: 28, A: 190}, 4),
		content,
	)
}

func topSurface(content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(
		roundedRect(admColorPanelBG, 8),
		content,
	)
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
	bg := roundedRect(color.NRGBA{R: 13, G: 20, B: 34, A: 255}, 8)
	border := roundedRect(color.NRGBA{R: 30, G: 41, B: 59, A: 255}, 8)
	return container.NewStack(
		border,
		container.NewPadded(container.NewStack(bg, container.NewPadded(content))),
	)
}

// rightToolPanel wraps one right-side tool panel's content in a scrollable
// panel surface with a stable minimum width, so switching panels never jumps
// the layout.
func rightToolPanel(content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(stableMinWidthLayout{width: workspacePanelMinWidth},
		panelSurface("", "", container.NewVScroll(content)))
}

func (g *GUIApp) buildDeviceWallPanel() fyne.CanvasObject {
	if g.controlSelected == nil {
		g.controlSelected = map[string]bool{}
	}
	if g.controlHidden == nil {
		g.controlHidden = map[string]bool{}
	}
	if g.controlDensity == "" {
		g.controlDensity = controlDensitySmall
	}
	g.controlSummary = widget.NewLabel("")
	g.controlGrid = container.NewVBox()
	g.controlCards = map[string]*controlCardView{}
	g.controlRealtimeStops = map[string]func(){}

	refreshButton := compactButton("重新扫描", func() { g.refreshAsync(false) })
	refreshScreensButton := compactButton("刷新画面", g.refreshControlScreensAsync)
	nativeWallButton := compactButton("外部窗", g.openExternalDeviceWindows)
	createButton := compactButton("创建模拟器", g.showCreateAVDDialog)
	densitySelect := widget.NewSelect(controlDensityLabels(), func(label string) {
		g.controlDensity = controlDensityKeyByLabel(label)
		g.renderControlCenter()
	})
	densitySelect.SetSelected(controlDensityLabel(g.controlDensity))
	selectionButton := compactButton("选择", nil)
	selectionButton.SetIcon(theme.MenuDropDownIcon())
	selectionButton.OnTapped = func() {
		menu := fyne.NewMenu("选择",
			fyne.NewMenuItem("选中可用设备", func() {
				g.selectControlEntries(func(entry core.DeviceEntry) bool { return entry.Active != nil && entry.Active.State == "device" })
			}),
			fyne.NewMenuItem("选中未启动模拟器", func() {
				g.selectControlEntries(func(entry core.DeviceEntry) bool { return entry.AVD != nil && !entry.Running })
			}),
			fyne.NewMenuItem("清空选择", func() {
				g.controlSelected = map[string]bool{}
				g.updateSelectedLabel()
				g.renderControlCenter()
			}),
		)
		driver := g.app.Driver()
		popup := widget.NewPopUpMenu(menu, driver.CanvasForObject(selectionButton))
		position := driver.AbsolutePositionForObject(selectionButton)
		popup.ShowAtPosition(position.Add(fyne.NewPos(0, selectionButton.Size().Height)))
	}
	targetButton := compactButton("设为主目标", func() {
		entries := g.selectedControlEntries()
		if len(entries) != 1 || entries[0].Active == nil || entries[0].Active.State != "device" {
			g.showInfo("请仅勾选一台可用设备，再设为主目标。")
			return
		}
		entry := entries[0]
		g.runAction("设为主目标 "+entry.Label, func() error { return g.backend.GUISetCurrentDevice(entry.Key) })
	})
	startSelectedButton := compactButton("启动选中", func() {
		names := g.selectedStoppedAVDNames()
		if len(names) == 0 {
			g.showInfo("请勾选要启动的未运行模拟器。")
			return
		}
		g.runAction(fmt.Sprintf("启动选中 %d 个 AVD", len(names)), func() error { return g.backend.GUIStartAVDs(names) })
	})
	stopSelectedButton := compactButton("关闭选中", func() {
		keys := g.selectedRunningEmulatorKeys()
		if len(keys) == 0 {
			g.showInfo("请勾选要关闭的运行中模拟器。")
			return
		}
		g.confirmAction("确认关闭选中模拟器", fmt.Sprintf("将关闭 %d 个勾选的运行中模拟器。", len(keys)), func() {
			g.runAction(fmt.Sprintf("关闭选中 %d 个模拟器", len(keys)), func() error { return g.backend.GUICloseDevices(keys) })
		})
	})
	deleteButton := compactButton("删除选中", g.showSelectedDeleteAVDDialog)
	deleteButton.Importance = widget.DangerImportance
	g.registerActionButtons(refreshButton, refreshScreensButton, nativeWallButton, createButton, selectionButton, targetButton, startSelectedButton, stopSelectedButton, deleteButton)

	refreshButton.SetText("扫描")
	refreshScreensButton.SetText("画面")
	createButton.SetText("创建")
	targetButton.SetText("主目标")
	g.controlSummary.Wrapping = fyne.TextTruncate
	actions := container.NewHBox(
		compactButtonBox(refreshButton, 58), compactButtonBox(refreshScreensButton, 58),
		mutedText("密度"), compactSelectBox(densitySelect, 78),
		compactButtonBox(nativeWallButton, 66), compactButtonBox(createButton, 58),
		compactButtonBox(selectionButton, 82), compactButtonBox(targetButton, 70),
		compactButtonBox(startSelectedButton, 86), compactButtonBox(stopSelectedButton, 86),
		compactButtonBox(deleteButton, 86),
	)
	toolbar := container.NewBorder(nil, nil, nil,
		container.NewGridWrap(fyne.NewSize(240, controlCompactControlHeight), g.controlSummary),
		controlToolbar(nil, actions),
	)
	panel := panelSurface("", "", container.NewBorder(
		toolbar,
		nil,
		nil,
		nil,
		container.NewScroll(g.controlGrid),
	))
	return container.New(flexibleMinWidthLayout{width: deviceWallPanelMinWidth}, panel)
}

func (g *GUIApp) buildInstallPanel() fyne.CanvasObject {
	g.apkEntry = widget.NewMultiLineEntry()
	g.apkEntry.SetPlaceHolder("APK 文件路径或 URL")
	g.apkEntry.Wrapping = fyne.TextWrapBreak
	g.apkEntry.SetMinRowsVisible(3)
	g.allowDowngrade = widget.NewCheck("允许降级安装", nil)
	downgradeHint := wrappedLabel("勾选后允许安装 versionCode 更低的 APK（adb install -r -d）。日常更新通常不用勾选。")

	browseButton := widget.NewButton("选择 APK", func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				g.showError(err)
				return
			}
			if reader == nil {
				return
			}
			g.apkEntry.SetText(reader.URI().Path())
			_ = reader.Close()
		}, g.window)
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".apk"}))
		fileDialog.Show()
	})
	browseButton.Alignment = widget.ButtonAlignCenter
	browseButton.Importance = widget.MediumImportance

	installButton := widget.NewButton("安装 APK", func() {
		source := g.apkSourceOrLast()
		if source == "" {
			g.showInfo("请输入 APK 路径或 URL。")
			return
		}
		g.confirmInstallSelection("安装 APK", source)
	})
	installButton.Alignment = widget.ButtonAlignCenter
	installButton.Importance = widget.HighImportance

	multiInstallButton := widget.NewButton("安装到多设备", func() {
		source := g.apkSourceOrLast()
		if source == "" {
			g.showInfo("请输入 APK 路径或 URL。")
			return
		}
		g.showMultiInstallDialog(source)
	})
	multiInstallButton.Alignment = widget.ButtonAlignCenter
	multiInstallButton.Importance = widget.HighImportance

	reinstallButton := widget.NewButton("重新安装上次 APK", func() {
		g.confirmInstallSelection("重新安装上次 APK", g.lastAPKSource)
	})
	reinstallButton.Alignment = widget.ButtonAlignCenter
	reinstallButton.Importance = widget.MediumImportance

	g.registerActionButtons(browseButton, installButton, multiInstallButton, reinstallButton)

	return container.NewVBox(
		sectionTitle("应用安装"),
		wrappedLabel("本地 APK 或 HTTP/HTTPS URL"),
		g.apkEntry,
		container.NewVBox(compactButtonBox(browseButton, 110), g.allowDowngrade),
		downgradeHint,
		container.NewVBox(
			compactButtonBox(installButton, 92),
			compactButtonBox(multiInstallButton, 126),
			compactButtonBox(reinstallButton, 154),
		),
	)
}

func (g *GUIApp) apkSourceOrLast() string {
	source := strings.TrimSpace(g.apkEntry.Text)
	if source != "" {
		return source
	}
	return strings.TrimSpace(g.lastAPKSource)
}

func (g *GUIApp) showMultiInstallDialog(source string) {
	allowDowngrade := g.allowDowngrade.Checked
	g.runModalLoad("读取可安装设备", func() (any, error) {
		return g.backend.GUIInstallTargets()
	}, func(value any) {
		targets := value.([]core.GUIInstallTarget)
		if len(targets) == 0 {
			g.showInfo("没有可安装设备。请先连接或启动 state=device 的设备。")
			return
		}

		checks := make(map[string]*widget.Check, len(targets))
		rows := make([]fyne.CanvasObject, 0, len(targets))
		for _, target := range targets {
			target := target
			check := widget.NewCheck(target.Label, nil)
			check.SetChecked(g.controlSelected[target.Key] || (len(g.selectedControlKeys()) == 0 && target.Current))
			checks[target.Serial] = check
			rows = append(rows, check)
		}

		selectAll := widget.NewCheck("全选", func(checked bool) {
			for _, check := range checks {
				check.SetChecked(checked)
			}
		})
		selectAll.SetChecked(allInstallTargetsSelected(targets, checks))

		info := widget.NewLabel(fmt.Sprintf("只会安装到下面勾选的设备。\nAPK：%s\n允许降级：%v", source, allowDowngrade))
		info.Wrapping = fyne.TextWrapWord
		list := container.NewVScroll(container.NewVBox(rows...))
		content := container.NewGridWrap(fyne.NewSize(760, 360),
			container.NewBorder(
				container.NewVBox(info, selectAll),
				nil,
				nil,
				nil,
				list,
			),
		)

		g.showActionDialog("安装到多设备", "安装", false, content, func() {
			var selectedSerials []string
			for _, target := range targets {
				if checks[target.Serial].Checked {
					selectedSerials = append(selectedSerials, target.Serial)
				}
			}
			if len(selectedSerials) == 0 {
				g.showInfo("请至少选择一台设备。")
				return
			}
			g.runInstallAction(fmt.Sprintf("安装 APK 到 %d 台设备", len(selectedSerials)), func() error {
				return g.backend.GUIInstallAPKToDevices(source, selectedSerials, allowDowngrade)
			}, func(serials []string) error {
				return g.backend.GUIInstallAPKToDevicesWithReinstall(source, serials)
			})
		})
	})
}

func allInstallTargetsSelected(targets []core.GUIInstallTarget, checks map[string]*widget.Check) bool {
	if len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		check := checks[target.Serial]
		if check == nil || !check.Checked {
			return false
		}
	}
	return true
}

func filterPackageNames(packages []string, filter string) []string {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return append([]string(nil), packages...)
	}
	tokens := strings.Fields(filter)
	filtered := make([]string, 0, len(packages))
	for _, pkg := range packages {
		name := strings.ToLower(pkg)
		matched := true
		for _, token := range tokens {
			if !strings.Contains(name, token) {
				matched = false
				break
			}
		}
		if matched {
			filtered = append(filtered, pkg)
		}
	}
	return filtered
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func newPackagePicker() *packagePicker {
	picker := &packagePicker{}
	picker.list = widget.NewList(
		func() int {
			return len(picker.filtered)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Wrapping = fyne.TextWrapBreak
			return label
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id < 0 || id >= len(picker.filtered) {
				label.SetText("")
				return
			}
			pkg := picker.filtered[id]
			prefix := "  "
			if pkg == picker.selected {
				prefix = "✓ "
			}
			label.SetText(prefix + pkg)
			picker.list.SetItemHeight(id, 30)
		},
	)
	picker.list.HideSeparators = true
	picker.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(picker.filtered) {
			return
		}
		picker.selected = picker.filtered[id]
		picker.list.UnselectAll()
		picker.list.Refresh()
	}
	return picker
}

func (p *packagePicker) setPackages(packages []string) {
	p.all = append([]string(nil), packages...)
	p.applyFilter(p.filter)
}

func (p *packagePicker) applyFilter(filter string) {
	p.filter = filter
	p.filtered = filterPackageNames(p.all, filter)
	if len(p.filtered) == 0 {
		p.selected = ""
		p.list.UnselectAll()
		p.list.Refresh()
		return
	}
	if !containsString(p.filtered, p.selected) {
		p.selected = p.filtered[0]
	}
	p.list.UnselectAll()
	p.list.Refresh()
}

func (p *packagePicker) selectedPackage() string {
	return strings.TrimSpace(p.selected)
}

func (p *packagePicker) view(width, height float32) fyne.CanvasObject {
	return container.NewGridWrap(fyne.NewSize(width, height), p.list)
}

func (g *GUIApp) buildMessagePanel() fyne.CanvasObject {
	g.textEntry = widget.NewMultiLineEntry()
	g.textEntry.SetPlaceHolder("要发送到当前设备的文本")
	g.textEntry.Wrapping = fyne.TextWrapBreak
	g.textEntry.SetMinRowsVisible(3)
	g.useADBKeyboard = widget.NewCheck("复杂文本使用 ADB Keyboard", nil)
	g.textHint = wrappedLabel("简单 ASCII 会使用 adb input text。")
	g.textEntry.OnChanged = func(text string) {
		if needsADBKeyboard(text) {
			g.textHint.SetText("检测到中文或复杂字符，将使用 ADB Keyboard。")
			g.useADBKeyboard.SetChecked(true)
			return
		}
		g.textHint.SetText("简单 ASCII 会使用 adb input text。")
	}

	sendButton := widget.NewButton("发送文本", func() {
		text := g.textEntry.Text
		if strings.TrimSpace(text) == "" {
			g.showInfo("请输入要发送的文本。")
			return
		}
		g.runAction("发送文本", func() error {
			return g.backend.GUISendText(text, g.useADBKeyboard.Checked)
		})
	})
	sendButton.Importance = widget.HighImportance

	installKeyboardButton := widget.NewButton("安装 ADB Keyboard", func() {
		g.runAction("安装 ADB Keyboard", g.backend.GUIInstallADBKeyboard)
	})
	installKeyboardButton.Importance = widget.MediumImportance

	g.registerActionButtons(sendButton, installKeyboardButton)

	return container.NewVBox(
		sectionTitle("文本输入"),
		wrappedLabel("请先让目标 App 的输入框获得焦点"),
		g.textEntry,
		g.textHint,
		g.useADBKeyboard,
		compactButtonBox(sendButton, 92),
		compactButtonBox(installKeyboardButton, 154),
	)
}

func (g *GUIApp) buildUninstallPanel() fyne.CanvasObject {
	g.packageFilter = widget.NewEntry()
	g.packageFilter.SetPlaceHolder("可选：输入关键词过滤列表")
	g.thirdPartyOnly = widget.NewCheck("仅用户安装的应用", nil)
	g.thirdPartyOnly.SetChecked(true)
	g.packagePicker = newPackagePicker()
	g.keepData = widget.NewCheck("保留数据 (-k)", nil)
	g.user0Only = widget.NewCheck("仅当前用户 (--user 0)", nil)

	var allPackages []string
	applyPackageFilter := func() {
		g.packagePicker.applyFilter(g.packageFilter.Text)
	}
	refreshPackages := func() {
		g.runAction("刷新包列表", func() error {
			packages, err := g.backend.GUIListPackages(g.thirdPartyOnly.Checked, "")
			if err != nil {
				return err
			}
			fyne.Do(func() {
				allPackages = packages
				g.packagePicker.setPackages(allPackages)
				g.packagePicker.applyFilter(g.packageFilter.Text)
			})
			return nil
		})
	}
	refreshPackagesButton := widget.NewButton("刷新应用列表", refreshPackages)
	refreshPackagesButton.Importance = widget.HighImportance
	g.thirdPartyOnly.OnChanged = func(_ bool) {
		refreshPackages()
	}
	g.packageFilter.OnChanged = func(_ string) {
		applyPackageFilter()
	}
	g.packageFilter.OnSubmitted = func(_ string) {
		applyPackageFilter()
	}

	uninstallButton := widget.NewButton("卸载应用", func() {
		pkg := g.packagePicker.selectedPackage()
		if pkg == "" {
			g.showInfo("请先刷新列表并选择一个应用包。")
			return
		}
		message := fmt.Sprintf("目标设备：%s\n包名：%s\n保留数据：%v\n仅当前用户：%v", g.currentDevice, pkg, g.keepData.Checked, g.user0Only.Checked)
		g.confirmAction("确认卸载", message, func() {
			g.runAction("卸载应用 "+pkg, func() error {
				return g.backend.GUIUninstallPackage(pkg, g.keepData.Checked, g.user0Only.Checked)
			})
		})
	})
	uninstallButton.Importance = widget.DangerImportance

	g.registerActionButtons(refreshPackagesButton, uninstallButton)

	listBox := container.NewVBox(
		sectionTitle("选择应用"),
		wrappedLabel("先刷新列表；关键词只是过滤条件，不需要记住完整包名"),
		g.packageFilter,
		compactButtonBox(refreshPackagesButton, 126),
		g.thirdPartyOnly,
		g.packagePicker.view(workspacePackageListWidth, 150),
	)
	actionBox := container.NewVBox(
		sectionTitle("执行卸载"),
		g.keepData,
		g.user0Only,
		compactButtonBox(uninstallButton, 92),
	)
	return container.NewVBox(listBox, actionBox)
}

// rebootCurrentDevice is the right icon bar's reboot action: confirm, then
// reboot the current target device.
func (g *GUIApp) rebootCurrentDevice() {
	g.confirmAction("确认重启", "目标设备："+g.currentDevice, func() {
		g.runAction("重启当前设备", g.backend.GUIRebootCurrent)
	})
}

// closeCurrentDevice is the right icon bar's close/disconnect action: it opens
// a serial-confirmation dialog before closing the emulator, disconnecting a TCP
// device or shutting down a real device.
func (g *GUIApp) closeCurrentDevice() {
	serialEntry := widget.NewEntry()
	serialEntry.SetPlaceHolder("输入当前设备 serial 确认")
	var d dialog.Dialog
	cancelButton := widget.NewButton("取消", func() {
		d.Hide()
	})
	cancelButton.Importance = widget.MediumImportance
	executeButton := widget.NewButton("执行", func() {
		d.Hide()
		g.runAction("关闭/断开当前设备", func() error {
			return g.backend.GUICloseCurrent(serialEntry.Text)
		})
	})
	executeButton.Importance = widget.DangerImportance
	g.prepareActionButton(executeButton)
	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("目标设备", widget.NewLabel(g.currentDevice)),
			widget.NewFormItem("Serial", serialEntry),
		),
		container.NewCenter(container.NewHBox(cancelButton, executeButton)),
	)
	d = dialog.NewCustomWithoutButtons("关闭/断开当前设备", container.NewGridWrap(fyne.NewSize(560, 180), content), g.activeDialogWindow())
	d.Show()
}

func (g *GUIApp) buildLogPanel() fyne.CanvasObject {
	clearButton := widget.NewButton("清空日志", func() {
		g.logLines = nil
		g.renderLogLines()
		g.logFollow = true
		g.scrollLogToBottom()
	})
	clearButton.Importance = widget.MediumImportance
	copyButton := widget.NewButton("复制日志", func() {
		g.app.Clipboard().SetContent(strings.Join(g.logLines, "\n"))
	})
	copyButton.Importance = widget.MediumImportance
	g.logScroll = container.NewScroll(g.logLabel)
	g.logScroll.OnScrolled = func(_ fyne.Position) {
		if g.logProgrammaticScroll {
			return
		}
		g.logFollow = g.logScrollAtBottom()
	}
	// Collapse is driven by the log icon in the right rail; the header keeps
	// only the clear/copy actions.
	header := container.NewBorder(nil, nil, container.NewVBox(sectionTitle("任务/日志"), mutedText("最近 200 行，可复制")), container.NewHBox(clearButton, copyButton), nil)
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
	g.entries = state.Devices
	g.currentDevice = state.CurrentDevice
	g.currentDeviceKey = state.CurrentDeviceKey
	g.lastAPKSource = state.LastAPKSource
	g.mirrorAlwaysOnTop = state.MirrorAlwaysOnTop
	g.currentLabel.SetText("主目标：" + state.CurrentDevice)
	g.toolSummary.SetText("工具：" + compactToolStatus(state.Tools))
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

func (g *GUIApp) updateSelectedLabel() {
	entries := g.selectedControlEntries()
	if len(entries) == 0 {
		g.selectedLabel.SetText("选中：未选择")
		return
	}
	if len(entries) == 1 {
		g.selectedLabel.SetText("选中：" + entries[0].Label)
		return
	}
	g.selectedLabel.SetText(fmt.Sprintf("选中：%d 台", len(entries)))
}

func (g *GUIApp) runAction(name string, fn func() error) {
	btn := g.beginTask(name)
	g.appendLog("INFO", "开始：%s", name)
	go func() {
		err := fn()
		fyne.Do(func() {
			g.endTask(btn)
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
		g.actionButtons = append(g.actionButtons, button)
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
	if follow {
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
	body := widget.NewLabel(message)
	body.Wrapping = fyne.TextWrapWord
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
	body := widget.NewLabel(message)
	body.Wrapping = fyne.TextWrapWord
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
		g.refreshAsync(false)
	})
	content := container.NewBorder(nil, container.NewCenter(container.NewHBox(closeButton, checkButton, openButton)), nil, nil, container.NewPadded(body))
	d = dialog.NewCustomWithoutButtons("开启辅助功能权限", content, g.activeDialogWindow())
	d.Resize(fyne.NewSize(680, 320))
	d.Show()
}

func (g *GUIApp) showMessageDialog(title, message string, danger bool) {
	body := widget.NewLabel(message)
	body.Wrapping = fyne.TextWrapWord
	body.Alignment = fyne.TextAlignLeading
	bodyScroll := container.NewScroll(container.NewPadded(body))
	bodyScroll.SetMinSize(fyne.NewSize(680, 260))

	var d dialog.Dialog
	okButton := widget.NewButton("关闭", func() {
		d.Hide()
	})
	okButton.Importance = widget.HighImportance

	content := container.NewGridWrap(fyne.NewSize(560, 180), container.NewBorder(
		nil,
		container.NewCenter(okButton),
		nil,
		nil,
		bodyScroll,
	))
	d = dialog.NewCustomWithoutButtons(title, content, g.activeDialogWindow())
	d.Show()
}

func (g *GUIApp) showControlCenterWindow() {
	if g.controlWindow != nil {
		g.renderControlCenter()
		g.controlWindow.RequestFocus()
		g.refreshControlScreensAsync()
		return
	}
	if g.controlSelected == nil {
		g.controlSelected = map[string]bool{}
	}
	if g.controlHidden == nil {
		g.controlHidden = map[string]bool{}
	}
	w := g.app.NewWindow("安卓设备矩阵 中控大屏")
	w.Resize(fyne.NewSize(controlWindowDefaultWidth, controlWindowDefaultHeight))
	g.controlWindow = w
	g.controlSummary = widget.NewLabel("")
	if g.controlDensity == "" {
		g.controlDensity = controlDensitySmall
	}
	g.controlGrid = container.NewVBox()
	g.controlCards = map[string]*controlCardView{}
	g.controlRealtimeStops = map[string]func(){}

	refreshButton := widget.NewButton("重新扫描", func() {
		g.refreshAsync(false)
	})
	refreshButton.Importance = widget.MediumImportance
	nativeWallButton := widget.NewButton("外部窗口", func() {
		g.openExternalDeviceWindows()
	})
	nativeWallButton.Importance = widget.HighImportance
	densitySelect := widget.NewSelect(controlDensityLabels(), func(label string) {
		g.controlDensity = controlDensityKeyByLabel(label)
		g.renderControlCenter()
	})
	densitySelect.SetSelected(controlDensityLabel(g.controlDensity))
	densityControl := compactSelectBox(densitySelect, controlDensitySelectWidth)
	startSelectedButton := widget.NewButton("启动勾选未运行", func() {
		names := g.selectedStoppedAVDNames()
		if len(names) == 0 {
			g.showInfo("勾选项中没有未运行的 AVD。")
			return
		}
		g.runAction(fmt.Sprintf("启动选中 %d 个 AVD", len(names)), func() error {
			return g.backend.GUIStartAVDs(names)
		})
	})
	startSelectedButton.Importance = widget.HighImportance
	stopSelectedButton := widget.NewButton("关闭勾选运行", func() {
		keys := g.selectedRunningEmulatorKeys()
		if len(keys) == 0 {
			g.showInfo("勾选项中没有运行中的模拟器。")
			return
		}
		g.confirmAction("确认批量关机", fmt.Sprintf("将关闭 %d 个运行中的模拟器。", len(keys)), func() {
			g.runAction(fmt.Sprintf("关闭选中 %d 个模拟器", len(keys)), func() error {
				return g.backend.GUICloseDevices(keys)
			})
		})
	})
	stopSelectedButton.Importance = widget.WarningImportance
	selectReadyButton := widget.NewButton("选中可操作", func() {
		g.selectControlEntries(func(entry core.DeviceEntry) bool {
			return entry.Active != nil && entry.Active.State == "device"
		})
	})
	selectReadyButton.Importance = widget.MediumImportance
	clearSelectionButton := widget.NewButton("清空选择", func() {
		g.controlSelected = map[string]bool{}
		g.renderControlCenter()
	})
	clearSelectionButton.Importance = widget.MediumImportance
	g.registerActionButtons(refreshButton, nativeWallButton, startSelectedButton, stopSelectedButton, selectReadyButton, clearSelectionButton)

	title := canvas.NewText("设备墙", admColorText)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	toolbar := topSurface(controlToolbar(
		container.NewVBox(title, g.controlSummary),
		container.NewHBox(
			densityControl,
			compactButtonBox(nativeWallButton, 78),
			compactButtonBox(refreshButton, controlToolbarButtonWidth),
			compactButtonBox(selectReadyButton, 86),
			compactButtonBox(clearSelectionButton, 86),
			compactButtonBox(startSelectedButton, 126),
			compactButtonBox(stopSelectedButton, 126),
		),
	))
	w.SetContent(appFrame(container.NewBorder(
		toolbar,
		nil,
		nil,
		nil,
		container.NewVScroll(g.controlGrid),
	)))
	w.SetOnClosed(func() {
		g.stopControlPreviewLoop()
		g.controlWindow = nil
		g.controlGrid = nil
		g.controlSummary = nil
		g.controlCards = nil
		g.stopControlRealtimeStreams()
		g.controlRealtimeStops = nil
	})
	g.renderControlCenter()
	w.Show()
	g.startControlPreviewLoop()
	g.refreshAsync(false)
}

func (g *GUIApp) tileNativeEmulatorWindows() {
	keys := g.selectedControlKeys()
	label := "全部运行中模拟器"
	if len(keys) > 0 {
		label = fmt.Sprintf("勾选的 %d 个模拟器", len(keys))
	}
	g.runAction("排列外部窗口 "+label, func() error {
		return g.backend.GUITileEmulatorWindows(keys, 0)
	})
}

func (g *GUIApp) openExternalDeviceWindows() {
	keys := g.selectedReadyControlKeys()
	label := "勾选的可用设备"
	if len(keys) == 0 {
		keys = g.readyControlKeys()
		label = "全部可用设备"
	}
	if len(keys) == 0 {
		g.showInfo("没有可打开外部窗口的可用设备。")
		return
	}
	g.runAction("打开外部窗口 "+label, func() error {
		return g.backend.GUIOpenLiveMirrors(keys)
	})
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

func (g *GUIApp) renderControlCenter() {
	if g.controlGrid == nil || g.controlSummary == nil {
		return
	}
	g.controlSummary.SetText(controlSummaryText(g.entries))
	if len(g.entries) == 0 {
		g.stopControlRealtimeStreams()
		g.controlCards = map[string]*controlCardView{}
		g.controlGrid.Objects = []fyne.CanvasObject{emptyDeviceWall(g)}
		g.controlGrid.Refresh()
		return
	}
	snapshots := g.controlCardSnapshots()
	g.stopControlRealtimeStreams()
	g.controlCards = map[string]*controlCardView{}
	spec := g.controlDensitySpec()
	cards := make([]fyne.CanvasObject, 0, len(g.entries))
	for _, entry := range g.entries {
		card := g.buildControlCard(entry, spec)
		if height := card.MinSize().Height; height > spec.cardSize.Height {
			spec.cardSize.Height = height
		}
		cards = append(cards, card)
	}
	sections := []fyne.CanvasObject{
		container.NewPadded(container.New(layout.NewGridWrapLayout(spec.cardSize), cards...)),
	}
	g.controlGrid.Objects = sections
	g.controlGrid.Refresh()
	g.restoreControlCardSnapshots(snapshots)
	g.refreshControlScreensAsync()
	// Probe each card's realtime capability off the UI thread; cards that
	// support gRPC get upgraded to a live stream when their probe returns.
	g.probeControlRealtimeAsync()
}

func (g *GUIApp) buildControlCard(entry core.DeviceEntry, spec controlDensitySpec) fyne.CanvasObject {
	selected := widget.NewCheck("选择", nil)
	selected.SetChecked(g.controlSelected[entry.Key])
	selected.OnChanged = func(checked bool) {
		if g.controlSelected == nil {
			g.controlSelected = map[string]bool{}
		}
		g.controlSelected[entry.Key] = checked
		g.updateSelectedLabel()
	}

	titleText := controlCardTitle(entry)
	title := widget.NewLabelWithStyle(titleText, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Wrapping = fyne.TextWrapBreak
	status := widget.NewLabel(controlCardCompactStatus(entry, ""))
	status.Wrapping = fyne.TextWrapBreak
	preview := newPreviewPane("等待截图", spec.previewSize)
	realtime := false
	hidden := g.controlHidden != nil && g.controlHidden[entry.Key]

	var buttons []fyne.CanvasObject
	if entry.Active != nil && entry.Active.State == "device" {
		entryKey := entry.Key
		serial := entry.Active.Serial
		if hidden {
			preview.setMessage("🙈")
			status.SetText(controlCardCompactStatus(entry, "已隐藏"))
		} else {
			// The gRPC realtime probe re-enumerates devices and is too slow to
			// run for every card on the UI thread (it froze the whole window on
			// refresh with many devices). Build the card as screenshot-only and
			// let probeControlRealtimeAsync upgrade it to realtime off-thread.
			preview.onTap = func(pos fyne.Position, size fyne.Size, imageSize image.Point) {
				g.tapDevicePreview(entryKey, serial, pos, size, imageSize)
			}
			preview.onSecondaryTap = func() {
				g.backDevicePreview(serial)
			}
			preview.onSwipe = func(start, end fyne.Position, size fyne.Size, imageSize image.Point) {
				g.swipeDevicePreview(entryKey, serial, start, end, size, imageSize)
			}
		}
		g.controlCards[entry.Key] = &controlCardView{
			entryKey: entry.Key,
			entry:    entry,
			serial:   serial,
			title:    controlCardTitle(entry),
			detail:   controlCardIdentity(entry),
			realtime: realtime,
			hidden:   hidden,
			preview:  preview,
			status:   status,
		}
		currentButton := g.controlActionButton("主目标", func() {
			g.runAction("设为主目标 "+entry.Label, func() error {
				return g.backend.GUISetCurrentDevice(entry.Key)
			})
		})
		buttons = append(buttons, currentButton)
		buttons = append(buttons, g.controlActionButton("独立窗", func() {
			g.openIndependentDeviceWindow(entryKey, entry.Label)
		}))
		if hidden {
			buttons = append(buttons, g.controlActionButton("显示", func() {
				g.controlHidden[entryKey] = false
				g.renderControlCenter()
			}))
		} else {
			buttons = append(buttons, g.controlActionButton("不看", func() {
				g.controlHidden[entryKey] = true
				if stop := g.controlRealtimeStops[entryKey]; stop != nil {
					stop()
					delete(g.controlRealtimeStops, entryKey)
				}
				g.renderControlCenter()
			}))
		}
		buttons = append(buttons, g.controlActionButton("关机", func() {
			g.confirmAction("确认关机", "目标设备："+controlCardIdentity(entry), func() {
				g.runAction("关闭设备 "+entry.Label, func() error {
					return g.backend.GUICloseDevice(entryKey)
				})
			})
		}))
		buttons = append(buttons, g.controlActionButton("主页", func() {
			g.keyEventDevice(serial, "主页", 3)
		}))
		buttons = append(buttons, g.controlActionButton("返回", func() {
			g.keyEventDevice(serial, "返回", 4)
		}))
		buttons = append(buttons, g.controlActionButton("通知", func() {
			g.statusBarDevice(serial, "通知栏", "notifications")
		}))
		buttons = append(buttons, g.controlActionButton("管理", func() {
			g.showControlDeviceManageDialog(entry)
		}))
	} else if entry.Active != nil {
		preview.setMessage(entryStatus(entry))
		entryKey := entry.Key
		if entry.Active.IsEmulator || entry.AVD != nil {
			buttons = append(buttons, g.controlActionButton("独立窗", func() {
				g.focusNativeEmulatorWindow(entryKey, entry.Label)
			}))
		}
		buttons = append(buttons, g.controlActionButton("管理", func() {
			g.showControlDeviceManageDialog(entry)
		}))
		buttons = append(buttons, g.controlActionButton("关机", func() {
			g.confirmAction("确认关机", "目标设备："+entry.Label, func() {
				g.runAction("关闭设备 "+entry.Label, func() error {
					return g.backend.GUICloseDevice(entryKey)
				})
			})
		}))
	} else if entry.Running {
		preview.setMessage("运行中，等待 adb 连接")
		if entry.AVD != nil {
			avdName := entry.AVD.Name
			entryKey := entry.Key
			buttons = append(buttons, g.controlActionButton("独立窗", func() {
				g.focusNativeEmulatorWindow(entryKey, avdName)
			}))
			buttons = append(buttons, g.controlActionButton("管理", func() {
				g.showControlDeviceManageDialog(entry)
			}))
			buttons = append(buttons, g.controlActionButton("关机", func() {
				g.confirmAction("确认关机", "目标模拟器："+avdName, func() {
					g.runAction("关闭模拟器 "+avdName, func() error {
						return g.backend.GUICloseAVD(avdName)
					})
				})
			}))
		}
	} else {
		preview.setMessage("未启动")
	}
	if entry.AVD != nil && !entry.Running {
		buttons = append(buttons, g.controlActionButton("管理", func() {
			g.showControlDeviceManageDialog(entry)
		}))
		startButton := widget.NewButton("开机", func() {
			g.runAction("启动模拟器 "+entry.AVD.Name, func() error {
				return g.backend.GUIStartAVD(entry.AVD.Name)
			})
		})
		startButton.Importance = widget.HighImportance
		g.prepareActionButton(startButton)
		buttons = append(buttons, startButton)
	}
	if len(buttons) == 0 {
		buttons = append(buttons, widget.NewLabel("暂无操作"))
	}

	header := container.NewBorder(nil, nil, selected, nil, g.copyableControlText(title, "设备名称", titleText))
	content := container.NewVBox(
		header,
		container.NewCenter(previewInteractiveObject(preview)),
		g.copyableControlText(compactStatus(status, spec.cardSize.Width-36), "设备编号", controlCardCopySerial(entry)),
		controlButtonRows(buttons, spec.cardSize.Width-4*theme.Padding()),
	)
	return controlCardSurface(content)
}

func emptyDeviceWall(g *GUIApp) fyne.CanvasObject {
	refreshButton := widget.NewButton("重新扫描", func() {
		g.refreshAsync(false)
	})
	refreshButton.Importance = widget.MediumImportance
	createButton := widget.NewButton("创建模拟器", g.showCreateAVDDialog)
	createButton.Importance = widget.HighImportance
	g.prepareActionButton(refreshButton)
	g.prepareActionButton(createButton)
	return container.NewCenter(compactSurface(container.NewVBox(
		sectionTitle("暂无设备"),
		mutedText("连接 Android 设备，或创建/启动一个模拟器。"),
		container.NewHBox(refreshButton, createButton),
	)))
}

func (g *GUIApp) renameAVD(oldName, newName string) bool {
	newName, ok := g.validateAVDRename(oldName, newName)
	if !ok {
		return false
	}
	g.runAction("修改模拟器名称 "+oldName, func() error {
		return g.backend.GUIRenameAVD(oldName, newName)
	})
	return true
}

func (g *GUIApp) renameRunningAVDWithShutdown(oldName, newName string) bool {
	newName, ok := g.validateAVDRename(oldName, newName)
	if !ok {
		return false
	}
	g.confirmAction("确认关机并改名",
		fmt.Sprintf("将先关闭模拟器 %s，等待退出后改名为 %s。", oldName, newName),
		func() {
			g.runAction("关闭并改名模拟器 "+oldName, func() error {
				if err := g.backend.GUICloseAVD(oldName); err != nil {
					return err
				}
				return g.backend.GUIRenameAVD(oldName, newName)
			})
		})
	return true
}

func (g *GUIApp) validateAVDRename(oldName, newName string) (string, bool) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		g.showInfo("请输入新的模拟器名称。")
		return "", false
	}
	if newName == oldName {
		g.showInfo("新名称和原名称相同。")
		return "", false
	}
	return newName, true
}

func (g *GUIApp) showControlDeviceManageDialog(entry core.DeviceEntry) {
	title := controlCardTitle(entry)
	targetLabel := widget.NewLabel(controlCardIdentity(entry))
	targetLabel.Wrapping = fyne.TextWrapWord

	var sections []fyne.CanvasObject
	if entry.AVD != nil {
		nameEntry := widget.NewEntry()
		nameEntry.SetText(entry.AVD.Name)
		nameEntry.SetPlaceHolder("新的模拟器名称")
		renameButton := widget.NewButton("改名", func() {
			if entry.Running {
				g.renameRunningAVDWithShutdown(entry.AVD.Name, nameEntry.Text)
				return
			}
			g.renameAVD(entry.AVD.Name, nameEntry.Text)
		})
		renameButton.Importance = widget.HighImportance
		g.prepareActionButton(renameButton)
		renameHint := "运行中的模拟器会先确认关机，等待退出后再改名。"
		if !entry.Running {
			renameHint = "仅修改 AVD 名称，不会启动模拟器。"
		}
		sections = append(sections,
			sectionTitle("模拟器改名"),
			nameEntry,
			container.NewHBox(renameButton, mutedText(renameHint)),
		)
	}

	if entry.Active != nil && entry.Active.State == "device" {
		entryKey := entry.Key
		textEntry := widget.NewMultiLineEntry()
		textEntry.SetPlaceHolder("发送到该设备的文本")
		textEntry.Wrapping = fyne.TextWrapBreak
		textEntry.SetMinRowsVisible(3)
		useADBKeyboard := widget.NewCheck("复杂文本使用 ADB Keyboard", nil)
		textEntry.OnChanged = func(text string) {
			if needsADBKeyboard(text) {
				useADBKeyboard.SetChecked(true)
			}
		}
		sendButton := widget.NewButton("发送文本", func() {
			text := textEntry.Text
			if strings.TrimSpace(text) == "" {
				g.showInfo("请输入要发送的文本。")
				return
			}
			g.runAction("发送文本到 "+entry.Label, func() error {
				if err := g.backend.GUISetCurrentDevice(entryKey); err != nil {
					return err
				}
				return g.backend.GUISendText(text, useADBKeyboard.Checked)
			})
		})
		sendButton.Importance = widget.HighImportance
		g.prepareActionButton(sendButton)

		apkEntry := widget.NewMultiLineEntry()
		apkEntry.SetPlaceHolder("APK 路径或 URL；留空使用上次")
		apkEntry.Wrapping = fyne.TextWrapBreak
		apkEntry.SetMinRowsVisible(3)
		installButton := widget.NewButton("安装 APK", func() {
			source := strings.TrimSpace(apkEntry.Text)
			if source == "" {
				source = g.apkSourceOrLast()
			}
			if source == "" {
				g.showInfo("请输入 APK 路径或 URL。")
				return
			}
			g.runInstallAction("安装 APK 到 "+entry.Label, func() error {
				return g.backend.GUIInstallAPKToDevices(source, []string{entry.Active.Serial}, g.allowDowngrade != nil && g.allowDowngrade.Checked)
			}, func(serials []string) error {
				return g.backend.GUIInstallAPKToDevicesWithReinstall(source, serials)
			})
		})
		installButton.Importance = widget.HighImportance
		g.prepareActionButton(installButton)

		packageFilter := widget.NewEntry()
		packageFilter.SetPlaceHolder("输入关键词过滤应用包")
		thirdPartyOnly := widget.NewCheck("仅用户安装的应用", nil)
		thirdPartyOnly.SetChecked(true)
		packagePicker := newPackagePicker()
		keepData := widget.NewCheck("保留数据", nil)
		user0Only := widget.NewCheck("仅当前用户", nil)
		var allPackages []string
		applyPackageFilter := func() {
			packagePicker.applyFilter(packageFilter.Text)
		}
		refreshPackagesButton := widget.NewButton("刷新应用列表", func() {
			g.runAction("刷新 "+entry.Label+" 应用列表", func() error {
				if err := g.backend.GUISetCurrentDevice(entryKey); err != nil {
					return err
				}
				packages, err := g.backend.GUIListPackages(thirdPartyOnly.Checked, "")
				if err != nil {
					return err
				}
				fyne.Do(func() {
					allPackages = packages
					packagePicker.setPackages(allPackages)
					packagePicker.applyFilter(packageFilter.Text)
				})
				return nil
			})
		})
		refreshPackagesButton.Importance = widget.HighImportance
		g.prepareActionButton(refreshPackagesButton)
		thirdPartyOnly.OnChanged = func(_ bool) {
			refreshPackagesButton.OnTapped()
		}
		packageFilter.OnChanged = func(_ string) {
			applyPackageFilter()
		}
		packageFilter.OnSubmitted = func(_ string) {
			applyPackageFilter()
		}
		uninstallButton := widget.NewButton("卸载应用", func() {
			pkg := packagePicker.selectedPackage()
			if pkg == "" {
				g.showInfo("请先刷新列表并选择一个应用包。")
				return
			}
			g.confirmAction("确认卸载", "目标设备："+controlCardIdentity(entry)+"\n包名："+pkg, func() {
				g.runAction("卸载应用 "+pkg, func() error {
					if err := g.backend.GUISetCurrentDevice(entryKey); err != nil {
						return err
					}
					return g.backend.GUIUninstallPackage(pkg, keepData.Checked, user0Only.Checked)
				})
			})
		})
		uninstallButton.Importance = widget.DangerImportance
		g.prepareActionButton(uninstallButton)

		sections = append(sections,
			sectionTitle("键盘输入"),
			textEntry,
			container.NewHBox(useADBKeyboard, sendButton),
			sectionTitle("安装/卸载"),
			apkEntry,
			installButton,
			sectionTitle("卸载应用"),
			container.NewBorder(nil, nil, nil, refreshPackagesButton, packageFilter),
			container.NewHBox(thirdPartyOnly, keepData, user0Only),
			packagePicker.view(dialogPackageListWidth, 140),
			uninstallButton,
		)
	}

	if len(sections) == 0 {
		sections = append(sections, mutedText("该设备当前没有可用管理操作。"))
	}
	content := container.NewGridWrap(fyne.NewSize(620, 520), container.NewVScroll(container.NewVBox(
		sectionTitle(title),
		targetLabel,
		container.NewPadded(container.NewVBox(sections...)),
	)))
	var d dialog.Dialog
	closeButton := compactButton("关闭", func() {
		d.Hide()
	})
	closeButton.Importance = widget.HighImportance
	d = dialog.NewCustomWithoutButtons("设备管理", container.NewVBox(content, container.NewCenter(closeButton)), g.activeDialogWindow())
	d.Show()
}

func (g *GUIApp) controlCardSnapshots() map[string]controlCardSnapshot {
	if len(g.controlCards) == 0 {
		return nil
	}
	snapshots := make(map[string]controlCardSnapshot, len(g.controlCards))
	for key, card := range g.controlCards {
		if card == nil || card.preview == nil {
			continue
		}
		snapshot := controlCardSnapshot{
			imageWidth:  card.preview.imageWidth,
			imageHeight: card.preview.imageHeight,
		}
		if card.preview.img != nil {
			snapshot.image = card.preview.img.Image
		}
		if card.status != nil {
			snapshot.status = card.status.Text
		}
		if card.preview.message != nil {
			snapshot.message = card.preview.message.Text
		}
		snapshots[key] = snapshot
	}
	return snapshots
}

func (g *GUIApp) restoreControlCardSnapshots(snapshots map[string]controlCardSnapshot) {
	if len(snapshots) == 0 {
		return
	}
	for key, snapshot := range snapshots {
		card := g.controlCards[key]
		if card == nil || card.preview == nil {
			continue
		}
		if card.hidden {
			continue
		}
		if snapshot.image != nil {
			card.preview.setImage(snapshot.image)
			card.preview.imageWidth = snapshot.imageWidth
			card.preview.imageHeight = snapshot.imageHeight
		} else if snapshot.message != "" {
			card.preview.setMessage(snapshot.message)
		}
		if snapshot.status != "" && card.status != nil {
			card.status.SetText(snapshot.status)
		}
	}
}

func (g *GUIApp) buildControlCompactRow(entry core.DeviceEntry) fyne.CanvasObject {
	selected := widget.NewCheck("选择", nil)
	selected.SetChecked(g.controlSelected[entry.Key])
	selected.OnChanged = func(checked bool) {
		if g.controlSelected == nil {
			g.controlSelected = map[string]bool{}
		}
		g.controlSelected[entry.Key] = checked
		g.updateSelectedLabel()
	}
	title := widget.NewLabelWithStyle(controlCardTitle(entry), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	status := widget.NewLabel(controlCardStatus(entry))
	detail := widget.NewLabel(entryDetail(entry))
	detail.Wrapping = fyne.TextWrapBreak
	var action fyne.CanvasObject = widget.NewLabel("")
	if entry.AVD != nil && !entry.Running {
		avdName := entry.AVD.Name
		actionButton := widget.NewButton("启动", func() {
			g.runAction("启动模拟器 "+avdName, func() error {
				return g.backend.GUIStartAVD(avdName)
			})
		})
		actionButton.Importance = widget.HighImportance
		g.prepareActionButton(actionButton)
		action = container.NewCenter(actionButton)
	}
	text := container.NewVBox(title, status, detail)
	row := container.NewBorder(nil, nil, selected, action, text)
	return container.NewPadded(row)
}

func (g *GUIApp) findEntryByKey(key string) (core.DeviceEntry, bool) {
	for _, entry := range g.entries {
		if entry.Key == key {
			return entry, true
		}
	}
	return core.DeviceEntry{}, false
}

func (g *GUIApp) startControlPreviewLoop() {
	g.stopControlPreviewLoop()
	stop := make(chan struct{})
	g.controlPreviewStop = stop
	go func() {
		ticker := time.NewTicker(controlPreviewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fyne.Do(func() {
					g.refreshControlScreensAsync()
				})
			case <-stop:
				return
			}
		}
	}()
}

func (g *GUIApp) stopControlPreviewLoop() {
	if g.controlPreviewStop != nil {
		close(g.controlPreviewStop)
		g.controlPreviewStop = nil
	}
	g.stopControlRealtimeStreams()
}

// probeControlRealtimeAsync checks each visible emulator card's gRPC realtime
// capability off the UI thread (the probe is too slow to run inline for many
// devices), then upgrades capable cards to a live stream.
func (g *GUIApp) probeControlRealtimeAsync() {
	if g.controlGrid == nil {
		return
	}
	for key, card := range g.controlCards {
		if card.hidden || card.realtime {
			continue
		}
		entry := card.entry
		if entry.Active == nil || entry.Active.State != "device" || !entry.Active.IsEmulator {
			continue
		}
		k := key
		go func() {
			target := g.backend.GUIEmulatorRealtimeTargetForEntry(entry)
			fyne.Do(func() {
				g.applyControlRealtimeProbe(k, target)
			})
		}()
	}
}

func (g *GUIApp) applyControlRealtimeProbe(key string, target core.GUIEmulatorRealtimeTarget) {
	card := g.controlCards[key]
	if card == nil || card.hidden || card.realtime {
		return
	}
	if target.Available {
		g.startControlRealtimeStreamFor(key, card)
		return
	}
	if strings.TrimSpace(target.Reason) != "" {
		card.status.SetText(controlCardCompactStatus(card.entry, firstLine(target.Reason)))
	}
}

func (g *GUIApp) startControlRealtimeStreamFor(key string, card *controlCardView) {
	if g.controlRealtimeStops == nil {
		g.controlRealtimeStops = map[string]func(){}
	}
	if g.controlRealtimeStops[key] != nil {
		return
	}
	frames, stop, err := g.backend.GUIStreamEmulatorRealtimeForEntry(card.entry, 0, 0)
	if err != nil {
		card.realtime = false
		card.status.SetText(fmt.Sprintf("%s | 内嵌不可用：%s", card.detail, firstLine(err.Error())))
		return
	}
	card.realtime = true
	card.preview.setMessage("等待实时画面")
	card.status.SetText(controlCardCompactStatus(card.entry, "实时"))
	g.controlRealtimeStops[key] = stop
	go g.consumeControlRealtimeFrames(key, card, frames)
}

func (g *GUIApp) consumeControlRealtimeFrames(key string, card *controlCardView, frames <-chan core.GUIEmulatorRealtimeFrame) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastFrame time.Time
	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				return
			}
			if frame.Error != "" {
				fyne.Do(func() {
					if current := g.controlCards[key]; current == card {
						card.realtime = false
						card.status.SetText(fmt.Sprintf("%s | 内嵌画面失败：%s", card.detail, frame.Error))
						delete(g.controlRealtimeStops, key)
						g.refreshControlScreensAsync()
					}
				})
				return
			}
			img, _, err := image.Decode(bytes.NewReader(frame.PNG))
			if err != nil {
				fyne.Do(func() {
					if current := g.controlCards[key]; current == card {
						card.status.SetText(fmt.Sprintf("%s | 内嵌画面解码失败：%s", card.detail, firstLine(err.Error())))
					}
				})
				continue
			}
			lastFrame = frame.Timestamp
			fyne.Do(func() {
				if current := g.controlCards[key]; current == card {
					card.preview.setImage(img)
					card.status.SetText(controlRealtimeStatusText(card.detail, time.Now(), lastFrame))
				}
			})
		case now := <-ticker.C:
			fyne.Do(func() {
				if current := g.controlCards[key]; current == card {
					card.status.SetText(controlRealtimeStatusText(card.detail, now, lastFrame))
				}
			})
		}
	}
}

func (g *GUIApp) stopControlRealtimeStreams() {
	for key, stop := range g.controlRealtimeStops {
		if stop != nil {
			stop()
		}
		delete(g.controlRealtimeStops, key)
	}
}

func (g *GUIApp) refreshControlScreensAsync() {
	if g.controlGrid == nil || len(g.controlCards) == 0 {
		return
	}
	cards := make(map[string]*controlCardView, len(g.controlCards))
	for key, card := range g.controlCards {
		if card.hidden || card.realtime {
			continue
		}
		cards[key] = card
	}
	if len(cards) == 0 {
		return
	}
	for key, card := range cards {
		g.refreshControlCardAsync(key, card, func() {})
	}
}

func (g *GUIApp) refreshControlCardAsync(key string, card *controlCardView, done func()) {
	if done == nil {
		done = func() {}
	}
	if g.controlGrid == nil || card == nil || card.serial == "" || card.refreshing {
		done()
		return
	}
	card.refreshing = true
	started := time.Now()
	go func() {
		defer done()
		png, err := g.backend.GUIDeviceScreenPNG(card.serial)
		if err != nil {
			fyne.Do(func() {
				if current := g.controlCards[key]; current == card {
					card.refreshing = false
					card.preview.setMessage("截图失败")
					card.status.SetText(fmt.Sprintf("%s | 截图失败：%s", card.detail, firstLine(err.Error())))
				}
			})
			return
		}
		img, _, err := image.Decode(bytes.NewReader(png))
		if err != nil {
			fyne.Do(func() {
				if current := g.controlCards[key]; current == card {
					card.refreshing = false
					card.preview.setMessage("解码失败")
					card.status.SetText(fmt.Sprintf("%s | 解码失败：%s", card.detail, firstLine(err.Error())))
				}
			})
			return
		}
		fyne.Do(func() {
			if current := g.controlCards[key]; current == card {
				card.refreshing = false
				card.preview.setImage(img)
				card.status.SetText(controlLatencyStatusText(card.detail, time.Since(started)))
			}
		})
	}()
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

func (g *GUIApp) showFloatingPreview(entry core.DeviceEntry) {
	if entry.Active == nil || entry.Active.State != "device" {
		g.showInfo("该设备当前没有可操作画面：" + entry.Label)
		return
	}
	entryKey := entry.Key
	serial := entry.Active.Serial
	title := controlCardTitle(entry)
	w := g.app.NewWindow("安卓设备矩阵 预览 - " + title)
	status := widget.NewLabel(title + " | 等待截图")
	preview := newPreviewPane("等待截图", fyne.NewSize(360, 640))
	preview.onTap = func(pos fyne.Position, size fyne.Size, imageSize image.Point) {
		g.tapDevicePreview(entryKey, serial, pos, size, imageSize)
	}
	preview.onSecondaryTap = func() {
		g.backDevicePreview(serial)
	}
	preview.onSwipe = func(start, end fyne.Position, size fyne.Size, imageSize image.Point) {
		g.swipeDevicePreview(entryKey, serial, start, end, size, imageSize)
	}
	var refreshButton *widget.Button
	refresh := func() {
		if refreshButton != nil {
			refreshButton.SetText("刷新截图...")
			refreshButton.Disable()
		}
		go func() {
			png, err := g.backend.GUIDeviceScreenPNG(serial)
			if err != nil {
				fyne.Do(func() {
					if refreshButton != nil {
						refreshButton.SetText("刷新截图")
						refreshButton.Enable()
					}
					preview.setMessage("截图失败")
					status.SetText(fmt.Sprintf("%s | 截图失败：%s", title, firstLine(err.Error())))
				})
				return
			}
			img, _, err := image.Decode(bytes.NewReader(png))
			if err != nil {
				fyne.Do(func() {
					if refreshButton != nil {
						refreshButton.SetText("刷新截图")
						refreshButton.Enable()
					}
					preview.setMessage("解码失败")
					status.SetText(fmt.Sprintf("%s | 解码失败：%s", title, firstLine(err.Error())))
				})
				return
			}
			fyne.Do(func() {
				if refreshButton != nil {
					refreshButton.SetText("刷新截图")
					refreshButton.Enable()
				}
				preview.setImage(img)
				status.SetText(fmt.Sprintf("%s | 截图 %s", title, time.Now().Format("15:04:05")))
			})
		}()
	}
	refreshButton = widget.NewButton("刷新截图", refresh)
	refreshButton.Importance = widget.MediumImportance
	w.SetContent(container.NewBorder(container.NewHBox(status, refreshButton), nil, nil, nil, previewInteractiveObject(preview)))
	w.Resize(fyne.NewSize(560, 960))
	stop := make(chan struct{})
	w.SetOnClosed(func() {
		close(stop)
	})
	w.Show()
	refresh()
	go func() {
		ticker := time.NewTicker(controlPreviewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refresh()
			case <-stop:
				return
			}
		}
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

func controlRealtimeStatusText(detail string, now, lastFrame time.Time) string {
	if lastFrame.IsZero() {
		return fmt.Sprintf("%s | 等待画面", detail)
	}
	return controlLatencyStatusText(detail, now.Sub(lastFrame))
}

func controlLatencyStatusText(detail string, latency time.Duration) string {
	ageMs := latency.Milliseconds()
	if ageMs < 0 {
		ageMs = 0
	}
	return fmt.Sprintf("%s | %dms", detail, ageMs)
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

func (g *GUIApp) controlDensitySpec() controlDensitySpec {
	key := g.controlDensity
	if key == "" {
		key = controlDensityStandard
	}
	for _, option := range controlDensityOptions {
		if option.key == key {
			return option
		}
	}
	return controlDensityOptions[1]
}

func controlDensityLabels() []string {
	labels := make([]string, 0, len(controlDensityOptions))
	for _, option := range controlDensityOptions {
		labels = append(labels, option.label)
	}
	return labels
}

func controlDensityLabel(key string) string {
	for _, option := range controlDensityOptions {
		if option.key == key {
			return option.label
		}
	}
	return controlDensityOptions[1].label
}

func controlDensityKeyByLabel(label string) string {
	for _, option := range controlDensityOptions {
		if option.label == label {
			return option.key
		}
	}
	return controlDensityStandard
}

func compactButton(label string, fn func()) *widget.Button {
	button := widget.NewButton(label, fn)
	button.Alignment = widget.ButtonAlignCenter
	button.Importance = widget.MediumImportance
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
	return container.NewGridWrap(fyne.NewSize(width, controlCompactControlHeight), status)
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
			g.showInfo("未发现已安装的 system image，请先用 CLI 或 Android Studio 安装。")
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
			g.runAction("创建模拟器 "+nameEntry.Text, func() error {
				return g.backend.GUICreateAVD(nameEntry.Text, imageSelect.Selected, deviceID, startCheck.Checked)
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
	var names []string
	running := false
	for _, entry := range g.selectedControlEntries() {
		if entry.AVD == nil {
			continue
		}
		names = append(names, entry.AVD.Name)
		running = running || entry.Running
	}
	if len(names) == 0 {
		g.showInfo("请在设备墙勾选要删除的模拟器；真机不会被删除。")
		return
	}
	warning := wrappedLabel("将删除以下勾选模拟器的配置和数据：")
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
		g.runAction(fmt.Sprintf("批量删除 %d 个模拟器", len(names)), func() error {
			return g.backend.GUIDeleteAVDs(names, true, forceClose.Checked)
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
			if err != nil {
				g.showError(err)
				return
			}
			done(value)
		})
	}()
}

func entryPrimary(entry core.DeviceEntry) string {
	name := ""
	if entry.AVD != nil {
		name = entry.AVD.Name
	} else if entry.Active != nil {
		name = entry.Active.Serial
		if model := entry.Active.Details["model"]; model != "" {
			name = model
		}
	}
	if name == "" {
		name = entry.Label
	}
	return entry.Kind + " | " + name + " | " + entryStatus(entry)
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

func controlSummaryText(entries []core.DeviceEntry) string {
	running := 0
	ready := 0
	offline := 0
	unauthorized := 0
	stopped := 0
	for _, entry := range entries {
		if entry.Running {
			running++
		}
		if entry.Active == nil {
			stopped++
			continue
		}
		switch entry.Active.State {
		case "device":
			ready++
		case "offline":
			offline++
		case "unauthorized":
			unauthorized++
		}
	}
	return fmt.Sprintf("共 %d 项 | 运行 %d | 可用 %d | 离线 %d | 未授权 %d | 未启动 %d", len(entries), running, ready, offline, unauthorized, stopped)
}

func deviceSummaryText(entries []core.DeviceEntry) string {
	running := 0
	for _, entry := range entries {
		if entry.Running {
			running++
		}
	}
	stopped := len(entries) - running
	return fmt.Sprintf("设备：共 %d 项 | 运行 %d | 未启动 %d", len(entries), running, stopped)
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
