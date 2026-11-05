package main

import (
	core "adm/internal/app"
	"context"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/skip2/go-qrcode"
	"image/color"
)

func (g *GUIApp) showWirelessQRDialog() {
	session, err := core.NewQRPairingSession()
	if err != nil {
		g.showError(err)
		return
	}
	qr, err := qrcode.New(session.Payload(), qrcode.Medium)
	if err != nil {
		g.showError(err)
		return
	}
	image := canvas.NewImageFromImage(qr.Image(320))
	image.FillMode = canvas.ImageFillContain
	image.ScaleMode = canvas.ImageScalePixels
	image.SetMinSize(fyne.NewSize(256, 256))
	status := widget.NewLabel("等待手机扫码，二维码 2 分钟内有效。")
	status.Wrapping = fyne.TextWrapWord
	guide := widget.NewLabel("电脑与手机连接同一网络。在 Android 11 及以上手机中打开：\n开发者选项 → 无线调试 → 使用二维码配对设备，扫描下方二维码。")
	guide.Wrapping = fyne.TextWrapWord
	ctx, cancel := context.WithCancel(context.Background())
	var d dialog.Dialog
	closeButton := newActionButton("关闭", func() { d.Hide() })
	regenerate := newActionButton("重新生成", func() { d.Hide(); g.showWirelessQRDialog() })
	regenerate.Importance = widget.MediumImportance
	closeButton.Importance = widget.MediumImportance
	qrBox := container.NewCenter(container.NewStack(canvas.NewRectangle(color.White), image))
	content := container.NewVBox(guide, qrBox, status, container.NewCenter(container.NewHBox(regenerate, closeButton)))
	d = dialog.NewCustomWithoutButtons("扫码配对手机", container.NewGridWrap(fyne.NewSize(500, 430), content), g.activeDialogWindow())
	d.SetOnClosed(cancel)
	d.Show()
	go func() {
		err := g.backend.GUIWaitQRPairing(ctx, session)
		fyne.Do(func() {
			if ctx.Err() != nil || g.closed {
				return
			}
			cancel()
			regenerate.Importance = widget.HighImportance
			regenerate.Refresh()
			if err != nil {
				qrBox.Hide()
				status.SetText(err.Error())
				return
			}
			qrBox.Hide()
			status.SetText("配对成功，正在刷新设备。ADB 将自动连接；若设备暂未出现，请返回发现列表点击连接。")
			g.appendLog("DONE", "二维码配对成功")
			g.refreshAsync(false)
		})
	}()
}
