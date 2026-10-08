package main

import (
	core "adm/internal/app"
	"fyne.io/fyne/v2"
)

// Injected by packaging so Fyne preferences and the signed bundle share an ID.
var applicationID = "com.zhuiguang.advm"

func (g *GUIApp) runExternalWindowAction(keys []string) {
	targets := append([]string(nil), keys...)
	g.pendingWindowArrangement = targets
	g.runActionWithCompletion("打开并排列外部窗口", func() error {
		return g.backend.GUIOpenLiveMirrors(targets)
	}, func(err error) {
		if !core.IsAccessibilityPermissionRequired(err) {
			g.pendingWindowArrangement = nil
		}
		if err == nil && !g.backend.GUIAccessibilityTrusted() {
			g.appendLog("INFO", "外部镜像已按顺序重排。辅助功能权限不可用，已通过重开镜像窗口应用位置。")
		}
	})
}

func (g *GUIApp) checkAccessibilityAndResume() {
	g.runModalLoad("检查辅助功能权限", func() (any, error) {
		return g.backend.GUIAccessibilityTrusted(), nil
	}, func(value any) {
		if !value.(bool) {
			g.showAccessibilityPermissionDialog("当前进程仍未获得辅助功能权限。\n\n" + core.AccessibilityPermissionGuide)
			return
		}
		g.refreshAsync(false)
		keys := append([]string(nil), g.pendingWindowArrangement...)
		if len(keys) == 0 {
			g.showInfo("辅助功能权限已开启。")
			return
		}
		g.runActionWithCompletion("继续排列已打开的外部窗口", func() error {
			return g.backend.GUITileEmulatorWindows(keys, 0)
		}, func(err error) {
			if err == nil {
				g.pendingWindowArrangement = nil
			}
		})
	})
}

func (g *GUIApp) showInstallSystemImageDialog() {
	text := "尚未安装 Android 系统镜像。\n\n可在本应用下载推荐镜像：" + core.GUIRecommendedSystemImage() + "\n下载可能需要数 GB 空间和数分钟时间。SDK 许可需已接受；也可先在 Android Studio 的 SDK Manager 安装。"
	body := g.dialogMessageContent(text, fyne.NewSize(640, 220))
	g.showActionDialog("安装系统镜像", "下载并安装", false, body, func() {
		g.runActionWithCompletion("安装推荐系统镜像", g.backend.GUIInstallRecommendedSystemImage, func(err error) {
			if err == nil {
				g.showCreateAVDDialog()
			}
		})
	})
}
