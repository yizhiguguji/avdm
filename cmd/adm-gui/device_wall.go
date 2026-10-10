package main

import (
	core "adm/internal/app"
	"bytes"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"image"
	"time"
)

var controlDensityOptions = []controlDensitySpec{
	{key: controlDensitySmall, label: "小", cardSize: controlCardSize(284, 440), previewSize: fyne.NewSize(244, 440)},
	{key: controlDensityStandard, label: "标准", cardSize: controlCardSize(350, 584), previewSize: fyne.NewSize(310, 584)},
	{key: controlDensityHD, label: "高清", cardSize: controlCardSize(430, 744), previewSize: fyne.NewSize(390, 744)},
}

func (g *GUIApp) renderControlCenter() {
	w := g.ensureDeviceWall()
	if w.closed.Load() || g.controlGrid == nil || g.controlSummary == nil {
		return
	}
	if g.controlCards == nil {
		g.controlCards = map[string]*controlCardView{}
	}
	if g.controlRealtimeStops == nil {
		g.controlRealtimeStops = map[string]func(){}
	}
	valid := map[string]core.DeviceEntry{}
	for _, entry := range g.entries {
		valid[entry.Key] = entry
	}
	for key, card := range g.controlCards {
		entry, exists := valid[key]
		if !exists || !sameWallSession(card, entry) {
			if stop := g.controlRealtimeStops[key]; stop != nil {
				stop()
				delete(g.controlRealtimeStops, key)
			}
			delete(g.controlCards, key)
		} else {
			card.entry = entry
			card.title = controlCardTitle(entry)
			card.detail = controlCardIdentity(entry)
			card.wall.title.SetText(card.title)
			card.wall.identifier.SetText(controlCopyIdentifier(entry))
			card.wall.selected.SetChecked(g.controlSelected[key])
			card.wall.visible = false
		}
	}
	spec := g.controlDensitySpec()
	var cards, rows, stopped []fyne.CanvasObject
	for _, entry := range g.wallVisibleEntries() {
		rows = append(rows, g.buildControlCompactRow(entry))
		if !readyWallEntry(entry) {
			stopped = append(stopped, g.buildStoppedDeviceCard(entry))
			continue
		}
		card := g.controlCards[entry.Key]
		if card == nil {
			g.buildControlCard(entry, spec)
			card = g.controlCards[entry.Key]
		}
		card.wall.visible = true
		g.reconcileControlCardHidden(entry.Key, card, g.controlHidden[entry.Key])
		g.updateWallCardSize(card, spec)
		if card.hidden {
			card.preview.setMessage("画面已隐藏")
		} else if card.preview.message.Text == "画面已隐藏" {
			card.preview.setMessage("等待画面")
		}
		if height := card.wall.object.MinSize().Height; height > spec.cardSize.Height {
			spec.cardSize.Height = height
		}
		cards = append(cards, card.wall.object)
	}
	if g.wallStoppedGrid != nil {
		g.wallStoppedGrid.Objects = stopped
		g.wallStoppedGrid.Refresh()
		if len(stopped) == 0 {
			g.wallStoppedPanel.Hide()
		} else {
			g.wallStoppedPanel.Show()
		}
	}
	g.updateWallWorkspace(cards, rows)
	if len(cards) == 0 && len(rows) == 0 {
		if len(g.entries) == 0 {
			g.controlGrid.Objects = []fyne.CanvasObject{emptyDeviceWall(g)}
		} else {
			g.controlGrid.Objects = []fyne.CanvasObject{widget.NewLabel("没有符合筛选条件的设备")}
		}
		g.controlGrid.Refresh()
	}
	g.refreshControlScreensAsync()
	g.probeControlRealtimeAsync()
}

func (g *GUIApp) buildControlCard(entry core.DeviceEntry, spec controlDensitySpec) fyne.CanvasObject {
	key, serial := entry.Key, entry.Active.Serial
	selected := widget.NewCheck("", nil)
	selected.SetChecked(g.controlSelected[key])
	selected.OnChanged = func(checked bool) {
		if g.controlSelected == nil {
			g.controlSelected = map[string]bool{}
		}
		g.controlSelected[key] = checked
		g.updateSelectedLabel()
	}
	title := widget.NewLabelWithStyle(controlCardTitle(entry), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Truncation = fyne.TextTruncateEllipsis
	status := widget.NewLabel("等待截图")
	status.Alignment = fyne.TextAlignTrailing
	identifier := widget.NewLabel(controlCopyIdentifier(entry))
	identifier.Truncation = fyne.TextTruncateEllipsis
	status.Truncation = fyne.TextTruncateEllipsis
	preview := newPreviewPane("等待截图", spec.previewSize)
	card := &controlCardView{entryKey: key, entry: entry, serial: serial, title: controlCardTitle(entry), detail: controlCardIdentity(entry), preview: preview, status: status,
		wall: &wallCardState{title: title, identifier: identifier, selected: selected, visible: true}}
	g.controlCards[key] = card
	preview.onTap = func(pos fyne.Position, size fyne.Size, img image.Point) {
		if !card.hidden {
			g.tapDevicePreview(key, serial, pos, size, img)
		}
	}
	preview.onSecondaryTap = func() {
		if !card.hidden {
			g.backDevicePreview(serial)
		}
	}
	preview.onSwipe = func(start, end fyne.Position, size fyne.Size, img image.Point) {
		if !card.hidden {
			g.swipeDevicePreview(key, serial, start, end, size, img)
		}
	}
	independent := compactButton("窗口", func() { g.openIndependentDeviceWindow(key, card.entry.Label) })
	home := newDeviceIconButton("主页", iconDeviceHome, func() { g.keyEventDevice(serial, "主页", 3) })
	back := newDeviceIconButton("返回", iconDeviceBack, func() { g.keyEventDevice(serial, "返回", 4) })
	target := compactButton("主目标", func() {
		g.runAction("设为主目标 "+card.entry.Label, func() error { return g.backend.GUISetCurrentDevice(key) })
	})
	notifications := newDeviceIconButton("通知", iconDeviceNotifications, func() { g.statusBarDevice(serial, "通知栏", "notifications") })
	more := newDeviceIconButton("更多", iconDeviceMore, nil)
	more.OnTapped = func() {
		more.hideHint()
		menu := g.deviceCardMenu(card)
		driver := g.app.Driver()
		popup := widget.NewPopUpMenu(menu, driver.CanvasForObject(more))
		popup.ShowAtPosition(driver.AbsolutePositionForObject(more).Add(fyne.NewPos(0, more.Size().Height)))
	}
	g.registerActionButtons(independent, &home.Button, &back.Button, target, &notifications.Button)
	card.wall.previewBox = container.NewGridWrap(spec.previewSize, previewInteractiveObject(preview))
	connection := "USB"
	if entry.Active.IsEmulator {
		connection = "模拟器"
	}
	connectionLabel := canvas.NewText(connection, admColorSuccess)
	connectionLabel.TextSize = 11
	name := container.NewStack(title, newCopyTapLayer(func() { g.copyControlText("设备名称", controlCardTitle(card.entry)) }))
	header := container.New(deviceCardHeaderLayout{}, selected, name, container.NewCenter(connectionLabel), more)
	actions := container.New(deviceActionRowLayout{}, target, independent, back, home, notifications)
	identifierCopy := container.NewStack(container.NewThemeOverride(identifier, captionTheme{g.app.Settings().Theme()}), newCopyTapLayer(func() { g.copyControlText("设备编号", controlCopyIdentifier(card.entry)) }))
	identity := container.New(deviceCardMetadataLayout{}, identifierCopy, container.NewThemeOverride(status, captionTheme{g.app.Settings().Theme()}))
	card.wall.object = controlCardSurface(container.New(deviceCardStackLayout{}, header, actions, container.NewCenter(card.wall.previewBox), identity))
	return card.wall.object
}

func emptyDeviceWall(g *GUIApp) fyne.CanvasObject {
	scan := widget.NewButton("重新扫描", func() { g.refreshAsync(false) })
	create := widget.NewButton("创建模拟器", g.showCreateAVDDialog)
	g.prepareActionButton(scan)
	return container.NewVBox(widget.NewLabel("没有检测到设备。连接手机并开启 USB 调试，或创建一个模拟器。"), container.NewHBox(scan, create))
}

func (g *GUIApp) buildControlCompactRow(entry core.DeviceEntry) fyne.CanvasObject {
	selected := widget.NewCheck("", nil)
	selected.SetChecked(g.controlSelected[entry.Key])
	selected.OnChanged = func(checked bool) {
		if g.controlSelected == nil {
			g.controlSelected = map[string]bool{}
		}
		g.controlSelected[entry.Key] = checked
		g.updateSelectedLabel()
	}
	title := widget.NewLabelWithStyle(controlCardTitle(entry), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Truncation = fyne.TextTruncateEllipsis
	state := workbenchDeviceState(entry)
	status := widget.NewLabel(state)
	if state == wallStateReady {
		status.TextStyle.Bold = true
	}
	manage := compactButton("管理", func() { g.showControlDeviceManageDialog(entry) })
	actions := container.NewHBox()
	if entry.AVD != nil && !entry.Running && entry.Active == nil {
		name := entry.AVD.Name
		start := compactButton("启动", func() { g.runAction("启动模拟器 "+name, func() error { return g.backend.GUIStartAVD(name) }) })
		start.Importance = widget.MediumImportance
		g.prepareActionButton(start)
		actions.Add(start)
	} else if entry.Running || readyWallEntry(entry) {
		actions.Add(compactButton("窗口", func() { g.openIndependentDeviceWindow(entry.Key, entry.Label) }))
	}
	actions.Add(manage)
	detail := controlCardIdentity(entry)
	if entry.Active == nil && entry.AVD != nil {
		detail = entry.AVD.Device
	}
	identityLabel := widget.NewLabel(detail)
	identityLabel.Truncation = fyne.TextTruncateEllipsis
	identity := container.NewVBox(g.copyableControlText(title, "设备名称", controlCardTitle(entry)), g.copyableControlText(container.NewThemeOverride(identityLabel, captionTheme{g.app.Settings().Theme()}), "设备编号", controlCopyIdentifier(entry)))
	kind := "模拟器"
	if entry.Active != nil && !entry.Active.IsEmulator {
		kind = "真机"
	}
	row := container.New(libraryRowLayout{}, selected, identity, widget.NewLabel(kind), status, actions)
	return container.NewStack(canvas.NewRectangle(admColorPanelBG), container.NewBorder(nil, canvas.NewLine(admColorBorder), nil, nil, row))

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
	w := g.ensureDeviceWall()
	if w.closed.Load() {
		return
	}
	stop := make(chan struct{})
	g.controlPreviewStop = stop
	go func() {
		ticker := time.NewTicker(controlPreviewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.post(func() {
					if !w.closed.Load() {
						for _, event := range g.backend.GUIDrainMirrorEvents() {
							g.appendLog(event.Level, "%s", event.Message)
						}
						g.refreshControlScreensAsync()
						g.probeControlRealtimeAsync()
						g.updateWallFreshness(time.Now())
					}
				})
			case <-stop:
				return
			case <-w.done:
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

func (g *GUIApp) probeControlRealtimeAsync() {
	w := g.ensureDeviceWall()
	if w.closed.Load() || w.connect == nil {
		return
	}
	for key, card := range g.controlCards {
		if card.wall == nil || card.hidden || !card.wall.visible || card.realtime || card.wall.connecting || !card.entry.Active.IsEmulator {
			continue
		}
		if card.wall.probed && time.Since(card.wall.lastProbe) < 15*time.Second {
			continue
		}
		g.startControlRealtimeStreamFor(key, card)
	}
}

func (g *GUIApp) startControlRealtimeStreamFor(key string, card *controlCardView) {
	w := g.ensureDeviceWall()
	if w.closed.Load() || w.connect == nil || card.wall.connecting || card.realtime {
		return
	}
	card.wall.connecting = true
	card.wall.generation++
	generation := card.wall.generation
	card.wall.probed = true
	card.wall.lastProbe = time.Now()
	entry := card.entry
	go func() {
		select {
		case w.connections <- struct{}{}:
		case <-w.done:
			return
		}
		defer func() { <-w.connections }()
		if w.closed.Load() {
			return
		}
		frames, stop, err := w.connect(entry)
		// Closing while a blocking connection is in progress must release it immediately.
		if w.closed.Load() {
			if stop != nil {
				stop()
			}
			return
		}
		w.post(func() { g.applyWallConnection(key, card, generation, frames, stop, err) })
	}()
}

func (g *GUIApp) applyWallConnection(key string, card *controlCardView, generation uint64, frames <-chan core.GUIEmulatorRealtimeFrame, stop func(), err error) {
	if !g.currentWallCard(key, card) || card.wall.generation != generation {
		if stop != nil {
			stop()
		}
		return
	}
	card.wall.connecting = false
	if err != nil {
		if stop != nil {
			stop()
		}
		card.realtime = false
		g.updateWallCardFreshness(card, time.Now())
		return
	}
	card.realtime = true
	card.wall.disconnected = false
	card.wall.connectedAt = time.Now()
	card.wall.lastFrame = time.Time{}
	g.controlRealtimeStops[key] = stop
	g.updateWallCardFreshness(card, time.Now())
	go g.consumeControlRealtimeFrames(key, card, generation, frames)
}

func (g *GUIApp) consumeControlRealtimeFrames(key string, card *controlCardView, generation uint64, frames <-chan core.GUIEmulatorRealtimeFrame) {
	w := g.wall
	for {
		if w.closed.Load() {
			return
		}
		select {
		case <-w.done:
			return
		case frame, ok := <-frames:
			if !ok {
				w.post(func() {
					if card.wall.generation == generation {
						g.wallStreamEnded(key, card, "")
					}
				})
				return
			}
			if frame.Error != "" {
				reason := frame.Error
				w.post(func() {
					if card.wall.generation == generation {
						g.wallStreamEnded(key, card, reason)
					}
				})
				return
			}
			if !card.wall.frameUpdatePending.CompareAndSwap(false, true) {
				continue
			}
			img, _, err := image.Decode(bytes.NewReader(frame.PNG))
			if err != nil {
				card.wall.frameUpdatePending.Store(false)
				continue
			}
			captured := frame.Timestamp
			if captured.IsZero() {
				captured = time.Now()
			}
			if w.closed.Load() {
				card.wall.frameUpdatePending.Store(false)
				return
			}
			w.post(func() {
				defer card.wall.frameUpdatePending.Store(false)
				if g.currentWallCard(key, card) && card.wall.generation == generation {
					card.wall.lastFrame = captured
					if !card.hidden && card.wall.visible {
						card.preview.setImage(img)
					}
					g.updateWallCardFreshness(card, time.Now())
				}
			})
		}
	}
}

func (g *GUIApp) wallStreamEnded(key string, card *controlCardView, reason string) {
	if !g.currentWallCard(key, card) {
		return
	}
	if stop := g.controlRealtimeStops[key]; stop != nil {
		stop()
	}
	delete(g.controlRealtimeStops, key)
	card.realtime = false
	card.wall.generation++
	card.wall.disconnected = true
	g.updateWallCardFreshness(card, time.Now())
	g.refreshControlCardAsync(key, card, nil)
}

func (g *GUIApp) stopControlRealtimeStreams() {
	for key, stop := range g.controlRealtimeStops {
		if stop != nil {
			stop()
		}
		delete(g.controlRealtimeStops, key)
		if card := g.controlCards[key]; card != nil {
			card.realtime = false
			if card.wall != nil {
				card.wall.generation++
				card.wall.connecting = false
			}
		}
	}
}

func (g *GUIApp) refreshControlScreensAsync() {
	w := g.ensureDeviceWall()
	if w.closed.Load() {
		return
	}
	for key, card := range g.controlCards {
		if card.wall != nil && !card.hidden && card.wall.visible && !card.realtime {
			g.refreshControlCardAsync(key, card, nil)
		}
	}
}

func (g *GUIApp) refreshControlCardAsync(key string, card *controlCardView, done func()) {
	w := g.ensureDeviceWall()
	if w.closed.Load() || w.screenshot == nil || card == nil || card.serial == "" || card.refreshing || card.hidden || (card.wall != nil && !card.wall.visible) {
		if done != nil {
			done()
		}
		return
	}
	if g.pauseEmulatorSnapshotForMirror(card) {
		if done != nil {
			done()
		}
		return
	}
	card.refreshing = true
	serial := card.serial
	go func() {
		if done != nil {
			defer done()
		}
		select {
		case w.screenshots <- struct{}{}:
		case <-w.done:
			return
		}
		defer func() { <-w.screenshots }()
		if w.closed.Load() {
			return
		}
		png, err := w.screenshot(serial)
		var img image.Image
		if err == nil {
			img, _, err = image.Decode(bytes.NewReader(png))
		}
		captured := time.Now()
		if w.closed.Load() {
			return
		}
		w.post(func() {
			if !g.currentWallCard(key, card) {
				return
			}
			card.refreshing = false
			if card.realtime {
				return
			}
			if err != nil {
				card.status.SetText("截图失败 · " + firstLine(err.Error()))
				return
			}
			card.wall.screenshotAt = captured
			if !card.hidden && card.wall.visible {
				card.preview.setImage(img)
			}
			g.updateWallCardFreshness(card, time.Now())
		})
	}()
}

func (g *GUIApp) updateWallFreshness(now time.Time) {
	for _, card := range g.controlCards {
		g.updateWallCardFreshness(card, now)
	}
}
func (g *GUIApp) updateWallCardFreshness(card *controlCardView, now time.Time) {
	if card.wall == nil {
		return
	}
	if card.hidden {
		card.status.SetText("画面已隐藏")
		return
	}
	if card.wall.mirrorPaused {
		card.status.SetText("外部镜像运行中 · 内嵌截图已暂停")
		return
	}
	if card.realtime {
		last := card.wall.lastFrame
		if last.IsZero() {
			last = card.wall.connectedAt
		}
		if !last.IsZero() && now.Sub(last) > 5*time.Second {
			g.wallStreamEnded(card.entryKey, card, "实时画面停止更新")
			return
		}
		card.status.SetText(controlRealtimeStatusText("", now, card.wall.lastFrame))
		return
	}
	prefix := "截图"
	if card.wall.disconnected {
		prefix = "实时已断流 · 截图"
	}
	if card.wall.screenshotAt.IsZero() {
		card.status.SetText(prefix + "等待中")
		return
	}
	age := now.Sub(card.wall.screenshotAt)
	text := fmt.Sprintf("%s %s", prefix, card.wall.screenshotAt.Format("15:04:05"))
	if age > 30*time.Second {
		text += " · 画面过期"
	}
	if age > 5*time.Second {
		text += fmt.Sprintf(" · %d秒前", int(age.Seconds()))
	}
	card.status.SetText(text)
}

func controlRealtimeStatusText(detail string, now, lastFrame time.Time) string {
	if lastFrame.IsZero() {
		return "实时 · 等待首帧"
	}
	age := now.Sub(lastFrame)
	if age > 3*time.Second {
		return fmt.Sprintf("实时 · %d秒未更新", int(age.Seconds()))
	}
	return "实时 · 正在更新"
}
func controlLatencyStatusText(detail string, latency time.Duration) string {
	return fmt.Sprintf("截图 · 耗时 %dms", latency.Milliseconds())
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

// A shared table rhythm keeps metadata and row actions aligned across devices.
func libraryTableHeader() fyne.CanvasObject {
	labels := []fyne.CanvasObject{widget.NewLabel(""), widget.NewLabelWithStyle("设备", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewLabel("类型"), widget.NewLabel("状态"), widget.NewLabel("操作")}
	return container.NewStack(canvas.NewRectangle(admColorPanelBG2), container.New(libraryRowLayout{}, labels...))
}

func (g *GUIApp) buildStoppedDeviceCard(entry core.DeviceEntry) fyne.CanvasObject {
	selected := widget.NewCheck("", nil)
	selected.SetChecked(g.controlSelected[entry.Key])
	selected.OnChanged = func(checked bool) {
		if g.controlSelected == nil {
			g.controlSelected = map[string]bool{}
		}
		g.controlSelected[entry.Key] = checked
		g.updateSelectedLabel()
	}
	name := widget.NewLabelWithStyle(controlCardTitle(entry), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	name.Truncation = fyne.TextTruncateEllipsis
	manage := compactButton("管理", func() { g.showControlDeviceManageDialog(entry) })
	actions := container.NewHBox(manage)
	if entry.AVD != nil && !entry.Running && entry.Active == nil {
		avd := entry.AVD.Name
		start := compactButton("启动", func() { g.runAction("启动模拟器 "+avd, func() error { return g.backend.GUIStartAVD(avd) }) })
		start.Importance = widget.MediumImportance
		g.prepareActionButton(start)
		actions.Add(start)
	} else if entry.Running || (entry.Active != nil && entry.Active.IsEmulator) {
		actions.Add(compactButton("窗口", func() { g.openIndependentDeviceWindow(entry.Key, entry.Label) }))
		actions.Add(compactButton("关闭", func() { g.showCloseEntryDialog(entry) }))
	}
	return container.NewVBox(container.NewBorder(nil, nil, selected, nil, g.copyableControlText(name, "设备名称", controlCardTitle(entry))), container.NewBorder(nil, nil, widget.NewLabel(workbenchDeviceState(entry)), actions, nil), widget.NewSeparator())
}

func controlCopyIdentifier(entry core.DeviceEntry) string {
	if entry.Active != nil {
		return entry.Active.Serial
	}
	if entry.AVD != nil {
		return entry.AVD.Name
	}
	return entry.Key
}

// Hiding is a resource decision, not just a paint flag. Invalidate pending
// connection generations before cancellation so late callbacks cannot revive it.
func (g *GUIApp) reconcileControlCardHidden(key string, card *controlCardView, hidden bool) {
	if card.hidden != hidden {
		card.hidden = hidden
		card.wall.generation++
		card.wall.connecting = false
		card.wall.probed = false
		card.wall.lastProbe = time.Time{}
		if hidden {
			if stop := g.controlRealtimeStops[key]; stop != nil {
				stop()
			}
			delete(g.controlRealtimeStops, key)
			card.realtime = false
			card.wall.disconnected = false
		}
	}
}
func (g *GUIApp) setControlCardHidden(key string, hidden bool) {
	if g.controlHidden == nil {
		g.controlHidden = map[string]bool{}
	}
	g.controlHidden[key] = hidden
	if card := g.controlCards[key]; card != nil {
		g.reconcileControlCardHidden(key, card, hidden)
	}
	g.renderControlCenter()
}

func (g *GUIApp) deviceCardMenu(card *controlCardView) *fyne.Menu {
	hideLabel := "隐藏画面"
	if card.hidden {
		hideLabel = "显示画面"
	}
	return fyne.NewMenu("设备操作",
		fyne.NewMenuItem("管理设备…", func() { g.showControlDeviceManageDialog(card.entry) }),
		fyne.NewMenuItem(hideLabel, func() { g.setControlCardHidden(card.entryKey, !card.hidden) }),
		fyne.NewMenuItem("关闭设备…", func() { g.showCloseEntryDialog(card.entry) }),
	)
}

func (g *GUIApp) pauseEmulatorSnapshotForMirror(card *controlCardView) bool {
	w := g.ensureDeviceWall()
	paused := card.wall != nil && card.entry.Active != nil && card.entry.Active.IsEmulator && w.mirrorActive != nil && w.mirrorActive(card.serial)
	if card.wall != nil {
		card.wall.mirrorPaused = paused
	}
	if paused {
		card.status.SetText("外部镜像运行中 · 内嵌截图已暂停")
		if card.wall.screenshotAt.IsZero() && card.wall.lastFrame.IsZero() {
			card.preview.setMessage("外部镜像运行中")
		}
	}
	return paused
}
