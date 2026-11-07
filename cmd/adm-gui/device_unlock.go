package main

import (
	core "adm/internal/app"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"strconv"
	"strings"
	"time"
)

func (g *GUIApp) showDeviceUnlockSettings(entry core.DeviceEntry) {
	if entry.Active == nil || entry.Active.State != "device" {
		g.showInfo("请先连接设备再设置解锁")
		return
	}
	serial := entry.Active.Serial
	g.runModalLoad("读取解锁设置", func() (any, error) { return g.backend.GUIGetDeviceUnlockSettings(serial) }, func(value any) { g.buildDeviceUnlockSettings(entry, value.(core.DeviceUnlockSettings)) })
}
func (g *GUIApp) buildDeviceUnlockSettings(entry core.DeviceEntry, settings core.DeviceUnlockSettings) {
	serial := entry.Active.Serial
	kind := widget.NewSelect([]string{"PIN", "密码", "图案"}, nil)
	if settings.Kind == "" {
		settings.Kind = "PIN"
	}
	kind.SetSelected(settings.Kind)
	secret := widget.NewPasswordEntry()
	secret.SetPlaceHolder("输入现有手机锁屏凭据；留空保留已保存值")
	confirm := widget.NewPasswordEntry()
	confirm.SetPlaceHolder("再次输入新设置的凭据")
	automatic := widget.NewCheck("检测到锁屏时自动解锁", nil)
	automatic.SetChecked(settings.Automatic)
	hint := widget.NewLabel("")
	hint.Wrapping = fyne.TextWrapWord
	status := widget.NewLabel("尚未配置解锁。凭据保存在此 Mac 的钥匙串中，按设备硬件标识区分。")
	status.Wrapping = fyne.TextWrapWord
	if settings.Configured {
		status.SetText("已保存解锁设置。输入新凭据可修改，留空保留现有值。")
	}
	if settings.Paused {
		status.SetText("自动解锁已暂停。请确认手机结果及凭据，重新保存后恢复。")
	}
	left, top, right, bottom := widget.NewEntry(), widget.NewEntry(), widget.NewEntry(), widget.NewEntry()
	if settings.Right == 0 {
		settings.Left = 25
		settings.Right = 75
		settings.Top = 50
		settings.Bottom = 80
	}
	left.SetText(strconv.Itoa(settings.Left))
	right.SetText(strconv.Itoa(settings.Right))
	top.SetText(strconv.Itoa(settings.Top))
	bottom.SetText(strconv.Itoa(settings.Bottom))
	calibrationHint := widget.NewLabel("图案位置需按手机校准：填写左上点和右下点中心占屏幕宽高的百分比。默认值仅为起点；黑屏时请对照手机屏幕。")
	calibrationHint.Wrapping = fyne.TextWrapWord
	calibration := container.NewVBox(calibrationHint, widget.NewForm(widget.NewFormItem("左 / 右（%）", container.NewGridWithColumns(2, left, right)), widget.NewFormItem("上 / 下（%）", container.NewGridWithColumns(2, top, bottom))))
	update := func() {
		if kind.Selected == "图案" {
			hint.SetText("九宫格从左到右、从上到下编号 1–9。输入滑动顺序，不重复；跨点时包含中间点。图案需先校准位置。")
			calibration.Show()
		} else if kind.Selected == "密码" {
			hint.SetText("支持英文、数字和符号密码（4–128 位）。暂不支持中文等非 ASCII 字符。")
			calibration.Hide()
		} else {
			hint.SetText("输入手机当前使用的 4–64 位数字 PIN。")
			calibration.Hide()
		}
	}
	kind.OnChanged = func(string) { update() }
	update()
	var d dialog.Dialog
	closeButton := newActionButton("取消", func() { d.Hide() })
	save := newActionButton("保存设置", nil)
	save.Importance = widget.HighImportance
	remove := newActionButton("清除已保存凭据", nil)
	if !settings.Configured {
		remove.Disable()
	}
	save.OnTapped = func() {
		if secret.Text != confirm.Text {
			status.SetText("两次输入不一致，请重新输入。")
			return
		}
		s := core.DeviceUnlockSettings{Kind: kind.Selected, Secret: secret.Text, Automatic: automatic.Checked}
		if s.Kind == "图案" {
			var err error
			fields := []*widget.Entry{left, top, right, bottom}
			values := []*int{&s.Left, &s.Top, &s.Right, &s.Bottom}
			for i, f := range fields {
				*values[i], err = strconv.Atoi(strings.TrimSpace(f.Text))
				if err != nil {
					status.SetText("图案位置请输入整数百分比。")
					return
				}
			}
		}
		save.Disable()
		closeButton.Disable()
		remove.Disable()
		go func() {
			err := g.backend.GUISaveDeviceUnlockSettings(serial, s)
			fyne.Do(func() {
				save.Enable()
				closeButton.Enable()
				if settings.Configured {
					remove.Enable()
				}
				if err != nil {
					status.SetText(err.Error())
					return
				}
				secret.SetText("")
				confirm.SetText("")
				d.Hide()
				g.showInfo("解锁设置已保存。可在设备“…”菜单选择“解锁此设备”；自动解锁失败后会暂停。")
			})
		}()
	}
	remove.OnTapped = func() {
		g.confirmAction("清除解锁设置", "将删除这台设备在此 Mac 钥匙串中的凭据，并关闭自动解锁。手机上的锁屏密码保持不变。", func() {
			g.runModalLoad("清除解锁设置", func() (any, error) { return nil, g.backend.GUIDeleteDeviceUnlockSettings(serial) }, func(any) { d.Hide(); g.showInfo("已清除本机保存的解锁设置。") })
		})
	}
	note := widget.NewLabel("此设置用于输入手机现有密码，不修改手机锁屏方式。自动解锁每次只尝试一次，失败即暂停；无 ADB 授权或无法确认锁屏状态时不输入。")
	note.Wrapping = fyne.TextWrapWord
	form := container.NewVBox(status, widget.NewForm(widget.NewFormItem("解锁方式", kind), widget.NewFormItem("凭据", secret), widget.NewFormItem("确认凭据", confirm)), hint, calibration, automatic, note)
	content := container.NewBorder(nil, container.NewBorder(nil, nil, remove, container.NewHBox(closeButton, save), nil), nil, nil, container.NewVScroll(form))
	d = dialog.NewCustomWithoutButtons("解锁设置 · "+compactPathMiddle(controlCardTitle(entry), 32), container.NewGridWrap(fyne.NewSize(600, 520), content), g.activeDialogWindow())
	d.SetOnClosed(func() { secret.SetText(""); confirm.SetText("") })
	d.Show()
}
func (g *GUIApp) unlockEntry(entry core.DeviceEntry) {
	if entry.Active == nil {
		return
	}
	serial := entry.Active.Serial
	g.runAction("解锁此设备", func() error { return g.backend.GUIUnlockDevice(serial) })
}

func (g *GUIApp) startDeviceUnlockMonitor() {
	wall := g.ensureDeviceWall()
	running := false
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-wall.done:
				return
			case <-ticker.C:
				fyne.Do(func() {
					if g.closed || running {
						return
					}
					var serials []string
					for _, entry := range g.entries {
						if entry.Active != nil && entry.Active.State == "device" && !entry.Active.IsEmulator {
							serials = append(serials, entry.Active.Serial)
						}
					}
					if len(serials) == 0 {
						return
					}
					running = true
					go func() {
						for _, serial := range serials {
							select {
							case <-wall.done:
								return
							default:
							}
							attempted, err := g.backend.GUIAutoUnlockDevice(serial)
							if attempted {
								fyne.Do(func() {
									if g.closed {
										return
									}
									if err != nil {
										g.appendLog("ERROR", "自动解锁已暂停：%v", err)
										g.showInfo(err.Error())
									} else {
										g.appendLog("DONE", "设备自动解锁成功")
									}
								})
							}
						}
						fyne.Do(func() { running = false })
					}()
				})
			}
		}
	}()
}
