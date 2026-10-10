package main

import (
	core "adm/internal/app"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"strings"
)

func (g *GUIApp) buildInstallPanel() fyne.CanvasObject {
	g.apkEntry = widget.NewMultiLineEntry()
	g.apkEntry.SetPlaceHolder("APK 文件路径或 URL")
	g.apkEntry.Wrapping = fyne.TextWrapBreak
	g.apkEntry.SetMinRowsVisible(2)
	g.allowDowngrade = widget.NewCheck("允许降级安装", nil)
	g.installScopeLabel = wrappedLabel("安装范围：优先使用勾选设备；未勾选时使用主目标。")
	downgradeHint := wrappedLabel("安装旧版本时开启。日常更新通常无需勾选。")

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

	installButton := widget.NewButton("安装到当前范围", func() {
		source := g.apkSourceOrLast()
		if source == "" {
			g.showInfo("请输入 APK 路径或 URL。")
			return
		}
		g.confirmInstallSelection("安装 APK", source)
	})
	installButton.Alignment = widget.ButtonAlignCenter
	installButton.Importance = widget.HighImportance

	multiInstallButton := widget.NewButton("另选安装设备…", func() {
		source := g.apkSourceOrLast()
		if source == "" {
			g.showInfo("请输入 APK 路径或 URL。")
			return
		}
		g.showMultiInstallDialog(source)
	})
	multiInstallButton.Alignment = widget.ButtonAlignCenter
	multiInstallButton.Importance = widget.MediumImportance

	reinstallButton := widget.NewButton("用上次 APK 安装", func() {
		g.confirmInstallSelection("重新安装上次 APK", g.lastAPKSource)
	})
	reinstallButton.Alignment = widget.ButtonAlignCenter
	reinstallButton.Importance = widget.MediumImportance

	historyButton := compactButton("上次来源", func() {
		if strings.TrimSpace(g.lastAPKSource) == "" {
			g.showInfo("还没有上次安装来源。")
			return
		}
		g.showMessageDialog("上次 APK 来源", g.lastAPKSource, false)
	})
	g.registerActionButtons(browseButton, installButton, multiInstallButton, reinstallButton)

	return container.NewVBox(
		sectionTitle("应用安装"),
		g.installScopeLabel,
		wrappedLabel("本地 APK 或 HTTP/HTTPS URL"),
		g.apkEntry,
		container.NewVBox(container.NewHBox(compactButtonBox(browseButton, 110), compactButtonBox(historyButton, 90)), g.allowDowngrade),
		container.NewThemeOverride(downgradeHint, captionTheme{g.app.Settings().Theme()}),
		container.NewVBox(
			compactButtonBox(installButton, 170),
			compactButtonBox(multiInstallButton, 170),
			compactButtonBox(reinstallButton, 170),
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

		info := g.dialogMessageContent(fmt.Sprintf("只会安装到下面勾选的设备。\nAPK：%s\n允许降级：%v", source, allowDowngrade), fyne.NewSize(700, 140))
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
		serial, ok := g.snapshotMainSerial()
		if !ok {
			return
		}
		allowKeyboard := g.useADBKeyboard.Checked
		g.runAction("发送文本", func() error {
			return g.backend.GUISendTextForDevice(serial, text, allowKeyboard)
		})
	})
	sendButton.Importance = widget.HighImportance

	installKeyboardButton := widget.NewButton("安装 ADB Keyboard", func() {
		serial, ok := g.snapshotMainSerial()
		if !ok {
			return
		}
		g.runAction("安装 ADB Keyboard", func() error { return g.backend.GUIInstallADBKeyboardForDevice(serial) })
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
	g.user0Only = widget.NewCheck("仅用户 0 (--user 0)", nil)

	var allPackages []string
	applyPackageFilter := func() {
		g.packagePicker.applyFilter(g.packageFilter.Text)
	}
	refreshPackages := func() {
		serial, ok := g.snapshotMainSerial()
		if !ok {
			return
		}
		userOnly := g.thirdPartyOnly.Checked
		generation := g.packageListBinding.begin(serial)
		g.packagePicker.setPackages(nil)
		g.runAction("刷新包列表", func() error {
			packages, err := g.backend.GUIListPackagesForDevice(serial, userOnly, "")
			if err != nil {
				return err
			}
			fyne.Do(func() {
				if !g.packageListBinding.accept(serial, generation) {
					return
				}
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
		serial, ok := g.snapshotMainSerial()
		if !ok {
			return
		}
		if !g.packageListBinding.ready(serial) {
			g.showInfo("目标设备已变化，请重新刷新应用列表。")
			return
		}
		keepData, user0 := g.keepData.Checked, g.user0Only.Checked
		message := fmt.Sprintf("目标设备：%s\n包名：%s\n保留数据：%v\n仅用户 0：%v", serial, pkg, keepData, user0)
		g.confirmAction("确认卸载", message, func() {
			g.runAction("卸载应用 "+pkg, func() error {
				return g.backend.GUIUninstallPackageForDevice(serial, pkg, keepData, user0)
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

// Rail power actions prefer the explicit selection and otherwise use the main target.
func (g *GUIApp) rebootCurrentDevice() { g.showPowerDialog(true) }

func (g *GUIApp) closeCurrentDevice() { g.showPowerDialog(false) }

// All close entry points share the same device-type warning and fixed target.
func (g *GUIApp) showCloseEntryDialog(entry core.DeviceEntry) {
	entry.Label = controlCardTitle(entry)
	if entry.Active == nil {
		if entry.AVD != nil && entry.Running {
			name := entry.AVD.Name
			g.confirmAction("确认关闭模拟器", "将关闭模拟器："+name, func() { g.runAction("关闭模拟器 "+name, func() error { return g.backend.GUICloseAVD(name) }) })
			return
		}
		g.showInfo("该设备当前未连接或未启动。")
		return
	}
	device := *entry.Active
	title, warning := "关闭模拟器", "将关闭该模拟器，未保存的操作可能丢失。"
	physical := !device.IsEmulator && !strings.Contains(device.Serial, ":")
	if !device.IsEmulator {
		title, warning = "断开网络设备", "将断开此设备的 ADB 网络连接。"
	}
	if physical {
		title, warning = "关闭真机", "将对真机执行关机，设备会停止运行。请完整输入下方 Serial 确认。"
	}
	serialEntry := widget.NewEntry()
	serialEntry.SetPlaceHolder(device.Serial)
	content := container.NewVBox(g.dialogMessageContent("目标设备："+entry.Label+"\nSerial："+device.Serial+"\n\n"+warning, fyne.NewSize(640, 200)))
	if physical {
		content.Add(serialEntry)
	}
	g.showActionDialog("确认"+title, title, true, content, func() {
		confirmation := device.Serial
		if physical {
			confirmation = serialEntry.Text
			if confirmation != device.Serial {
				g.showInfo("Serial 不匹配，未执行关机。")
				return
			}
		}
		g.runAction(title+" "+entry.Label, func() error {
			if device.IsEmulator {
				return g.backend.GUICloseDevice(entry.Key)
			}
			return g.backend.GUICloseDeviceConfirmed(device.Serial, confirmation)
		})
	})
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
	sections = append(sections, container.NewHBox(
		compactButton("复制设备名称", func() { g.copyControlText("设备名称", controlCardTitle(entry)) }),
		compactButton("复制设备编号", func() { g.copyControlText("设备编号", controlCopyIdentifier(entry)) }),
	))
	if windowLabel, closeLabel, available := deviceManagementWindowActions(entry); available {
		sections = append(sections, container.NewHBox(
			compactButton(windowLabel, func() { g.openIndependentDeviceWindow(entry.Key, entry.Label) }),
			compactButton(closeLabel, func() { g.showCloseEntryDialog(entry) }),
		))
	}
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
		serial := entry.Active.Serial
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
			allowKeyboard := useADBKeyboard.Checked
			g.runAction("发送文本到 "+entry.Label, func() error {
				return g.backend.GUISendTextForDevice(serial, text, allowKeyboard)
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
			allowDowngrade := g.allowDowngrade != nil && g.allowDowngrade.Checked
			g.runInstallAction("安装 APK 到 "+entry.Label, func() error {
				return g.backend.GUIInstallAPKToDevices(source, []string{serial}, allowDowngrade)
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
		user0Only := widget.NewCheck("仅用户 0", nil)
		var allPackages []string
		applyPackageFilter := func() {
			packagePicker.applyFilter(packageFilter.Text)
		}
		var binding packageListBinding
		refreshPackagesButton := widget.NewButton("刷新应用列表", func() {
			userOnly := thirdPartyOnly.Checked
			generation := binding.begin(serial)
			packagePicker.setPackages(nil)
			g.runAction("刷新 "+entry.Label+" 应用列表", func() error {
				packages, err := g.backend.GUIListPackagesForDevice(serial, userOnly, "")
				if err != nil {
					return err
				}
				fyne.Do(func() {
					if !binding.accept(serial, generation) {
						return
					}
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
			if !binding.ready(serial) {
				g.showInfo("请先刷新应用列表。")
				return
			}
			keep, user0 := keepData.Checked, user0Only.Checked
			g.confirmAction("确认卸载", fmt.Sprintf("目标设备：%s\n包名：%s\n保留数据：%v\n仅用户 0：%v", serial, pkg, keep, user0), func() {
				g.runAction("卸载应用 "+pkg, func() error {
					return g.backend.GUIUninstallPackageForDevice(serial, pkg, keep, user0)
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

// Button labels must describe the action performed by showCloseEntryDialog.
// Running is also true for online phones, so it is not a device-type check.
func deviceManagementWindowActions(entry core.DeviceEntry) (string, string, bool) {
	if entry.Active != nil && entry.Active.IsEmulator || entry.AVD != nil && entry.Running {
		return "聚焦窗口", "关闭模拟器…", true
	}
	if entry.Active != nil && entry.Active.State == "device" {
		if strings.Contains(entry.Active.Serial, ":") {
			return "打开独立窗", "断开网络设备…", true
		}
		return "打开独立窗", "关闭真机…", true
	}
	return "", "", false
}

// A list belongs to one device and one request generation. Switching target or
// requesting another list invalidates every older response, even A -> B -> A.
type packageListBinding struct {
	serial     string
	generation uint64
	loaded     bool
}

func (b *packageListBinding) begin(serial string) uint64 {
	b.serial = serial
	b.generation++
	b.loaded = false
	return b.generation
}
func (b *packageListBinding) invalidate() { b.generation++; b.loaded = false; b.serial = "" }
func (b *packageListBinding) accept(serial string, generation uint64) bool {
	if b.serial != serial || b.generation != generation {
		return false
	}
	b.loaded = true
	return true
}
func (b *packageListBinding) ready(serial string) bool { return b.loaded && b.serial == serial }
func (g *GUIApp) onTargetChanged(previous, next string) {
	if previous == next {
		for _, entry := range g.entries {
			if entry.Key == next && entry.Active != nil && entry.Active.State == "device" && (g.packageListBinding.serial == "" || g.packageListBinding.serial == entry.Active.Serial) {
				return
			}
		}
	}
	g.packageListBinding.invalidate()
	if g.packagePicker != nil {
		g.packagePicker.setPackages(nil)
	}
}
func (g *GUIApp) snapshotMainSerial() (string, bool) {
	for _, entry := range g.entries {
		if entry.Key == g.currentDeviceKey && entry.Active != nil && entry.Active.State == "device" {
			return entry.Active.Serial, true
		}
	}
	g.showInfo("请先设置一台可用设备为主目标。")
	return "", false
}
