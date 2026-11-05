package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func (g *GUIApp) showWirelessDialog() {
	pairAddress := widget.NewEntry()
	pairAddress.SetPlaceHolder("配对码弹窗中的 IP 地址:端口")
	code := widget.NewPasswordEntry()
	code.SetPlaceHolder("6 位数字")
	address := widget.NewEntry()
	address.SetPlaceHolder("无线调试主页的 IP 地址:端口")
	result := newSelectableMessage("正在发现同一网络中开启了无线调试的手机…")
	pair := newActionButton("配对", nil)
	connect := newActionButton("连接", nil)
	disconnect := newActionButton("断开连接", nil)
	discover := newActionButton("重新发现", nil)
	pair.Importance = widget.HighImportance
	connect.Importance = widget.HighImportance
	var d dialog.Dialog
	var tabs *container.AppTabs
	var busy bool
	var rowActions []*readableButton
	closeButton := newActionButton("关闭", func() { d.Hide() })
	rows := container.NewVBox()
	setBusy := func(value bool) {
		busy = value
		for _, button := range append([]*readableButton{pair, connect, disconnect, discover, closeButton}, rowActions...) {
			if value {
				button.Disable()
			} else {
				button.Enable()
			}
		}
	}
	var loadDevices func()
	run := func(operation, endpoint, pairingCode string) {
		if busy {
			return
		}
		setBusy(true)
		result.SetText("正在" + wirelessLabel(operation) + "，请保持手机无线调试页面开启…")
		go func() {
			err := g.backend.GUIWirelessOperation(operation, endpoint, pairingCode)
			fyne.Do(func() {
				if g.closed {
					return
				}
				setBusy(false)
				code.SetText("")
				if err != nil {
					result.SetText(err.Error() + "\n首次使用请点击设备旁的“配对”，按手机提示完成授权后再连接。")
					g.appendLog("ERROR", "无线%s失败：%v", wirelessLabel(operation), err)
					return
				}
				g.appendLog("DONE", "无线%s成功：%s", wirelessLabel(operation), endpoint)
				g.refreshAsync(false)
				if operation == "pair" {
					tabs.SelectIndex(0)
					loadDevices()
					return
				}
				result.SetText("无线" + wirelessLabel(operation) + "成功，设备列表正在刷新。")
				loadDevices()
			})
		}()
	}
	loadDevices = func() {
		if busy {
			return
		}
		setBusy(true)
		rows.Objects = nil
		rows.Add(widget.NewLabel("正在查找手机广播，请稍候…"))
		result.SetText("正在发现同一网络中开启了无线调试的手机…")
		go func() {
			devices, err := g.backend.GUIWirelessDevices()
			fyne.Do(func() {
				if g.closed {
					return
				}
				rows.Objects = nil
				rowActions = nil
				setBusy(false)
				if err != nil {
					result.SetText(err.Error())
					rows.Add(widget.NewLabel("发现失败，可重新发现或使用手动连接。"))
					return
				}
				if len(devices) == 0 {
					empty := widget.NewLabel("暂未发现无线手机。\n请确认手机已开启无线调试，且电脑与手机处于同一可互访网络。\n广播被网络屏蔽时，可切换到“手动连接 / 配对”输入地址。")
					empty.Wrapping = fyne.TextWrapWord
					rows.Add(empty)
					result.SetText("未发现设备。发现仅查找无线调试服务，不扫描全网端口。")
					return
				}
				for _, device := range devices {
					device := device
					name := widget.NewLabelWithStyle(device.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
					name.Truncation = fyne.TextTruncateEllipsis
					endpoint := widget.NewLabel(device.Address)
					status := widget.NewLabel(device.Status)
					status.Importance = widget.LowImportance
					actions := container.NewHBox()
					if device.Connected {
						actions.Add(status)
					} else {
						join := newActionButton("连接", func() { address.SetText(device.Address); run("connect", device.Address, "") })
						join.Importance = widget.HighImportance
						pairGuide := newActionButton("配对", func() {
							address.SetText(device.Address)
							pairAddress.SetText("")
							code.SetText("")
							tabs.SelectIndex(1)
							result.SetText("在手机点击“使用配对码配对设备”，填写该弹窗中的配对地址和 6 位配对码。配对成功后返回发现列表连接。")
							g.activeDialogWindow().Canvas().Focus(pairAddress)
						})
						rowActions = append(rowActions, join, pairGuide)
						actions.Add(join)
						actions.Add(pairGuide)
					}
					metadata := container.NewHBox(endpoint)
					if !device.Connected {
						metadata.Add(status)
					}
					rows.Add(container.NewBorder(nil, nil, nil, container.NewCenter(actions), container.NewVBox(name, metadata)))
					rows.Add(widget.NewSeparator())
				}
				result.SetText(fmt.Sprintf("发现 %d 台无线设备。已配对的手机可直接连接；首次使用请点击配对。", len(devices)))
				rows.Refresh()
			})
		}()
	}
	discover.OnTapped = loadDevices
	pair.OnTapped = func() { run("pair", pairAddress.Text, code.Text) }
	connect.OnTapped = func() { run("connect", address.Text, "") }
	disconnect.OnTapped = func() { run("disconnect", address.Text, "") }
	intro := widget.NewLabel("自动读取手机广播的名称、IP 和当前端口，无需逐个扫描。")
	intro.Wrapping = fyne.TextWrapWord
	discoveryPage := container.NewBorder(container.NewBorder(nil, nil, nil, discover, intro), nil, nil, nil, container.NewVScroll(rows))
	help := widget.NewLabel("电脑与手机连接同一网络，并在手机「开发者选项 → 无线调试」中开启调试。")
	help.Wrapping = fyne.TextWrapWord
	pairHint := widget.NewLabel("首次使用：打开手机“使用配对码配对设备”，填写弹窗中的地址和配对码。")
	pairHint.Wrapping = fyne.TextWrapWord
	connectHint := widget.NewLabel("手动连接：填写无线调试主页的 IP 地址和端口，与配对端口不同。")
	connectHint.Wrapping = fyne.TextWrapWord
	manual := container.NewVBox(help, widget.NewSeparator(), pairHint,
		widget.NewForm(widget.NewFormItem("配对地址", pairAddress), widget.NewFormItem("配对码", code)), container.NewHBox(pair),
		widget.NewSeparator(), connectHint, widget.NewForm(widget.NewFormItem("连接地址", address)), container.NewHBox(connect, disconnect))
	qrHelp := widget.NewLabel("使用手机无线调试中的二维码扫描器完成配对，无需输入地址和配对码。")
	qrHelp.Wrapping = fyne.TextWrapWord
	qrButton := newActionButton("生成配对二维码", g.showWirelessQRDialog)
	qrButton.Importance = widget.HighImportance
	qrPage := container.NewVBox(qrHelp, container.NewHBox(qrButton))
	tabContent := func(content fyne.CanvasObject) fyne.CanvasObject {
		return container.New(layout.NewCustomPaddedLayout(12, 8, 0, 0), content)
	}
	tabs = container.NewAppTabs(container.NewTabItem("发现设备", tabContent(discoveryPage)), container.NewTabItem("手动连接 / 配对", tabContent(container.NewVScroll(manual))), container.NewTabItem("扫码配对", tabContent(qrPage)))
	footer := container.NewVBox(container.NewGridWrap(fyne.NewSize(620, 85), result), container.NewCenter(closeButton))
	content := container.NewBorder(nil, footer, nil, nil, tabs)
	d = dialog.NewCustomWithoutButtons("Wi-Fi 连接手机", container.NewGridWrap(fyne.NewSize(640, 490), content), g.activeDialogWindow())
	d.Show()
	loadDevices()
}

func wirelessLabel(operation string) string {
	switch operation {
	case "pair":
		return "配对"
	case "disconnect":
		return "断开"
	default:
		return "连接"
	}
}
