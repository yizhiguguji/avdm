package main

import "fyne.io/fyne/v2"

// Custom edge-bar icons. These are the Tabler outline icons (MIT licensed) used
// in the design mockup — cleaner and more on-point than Fyne's built-in theme
// glyphs. currentColor is replaced with a fixed light stroke because the app is a
// fixed dark-themed app; toolIcon handles the dim/active states itself, so the
// icon body never needs recolouring.
func svgIcon(name, body string) fyne.Resource {
	const (
		head = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#cbd5e1" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">`
		tail = `</svg>`
	)
	return fyne.NewStaticResource(name, []byte(head+body+tail))
}

var (
	// 控制台：adjustments-horizontal（滑块/筛选）
	iconConsole = svgIcon("adm-console",
		`<path d="M12 6a2 2 0 1 0 4 0a2 2 0 1 0 -4 0" /><path d="M4 6l8 0" /><path d="M16 6l4 0" /><path d="M6 12a2 2 0 1 0 4 0a2 2 0 1 0 -4 0" /><path d="M4 12l2 0" /><path d="M10 12l10 0" /><path d="M15 18a2 2 0 1 0 4 0a2 2 0 1 0 -4 0" /><path d="M4 18l11 0" /><path d="M19 18l1 0" />`)

	// 应用安装：package-import（箱子带下箭头）
	iconInstall = svgIcon("adm-install",
		`<path d="M12 21l-8 -4.5v-9l8 -4.5l8 4.5v4.5" /><path d="M12 12l8 -4.5" /><path d="M12 12v9" /><path d="M12 12l-8 -4.5" /><path d="M22 18h-7" /><path d="M18 15l-3 3l3 3" />`)

	// 应用卸载：trash
	iconUninstall = svgIcon("adm-uninstall",
		`<path d="M4 7l16 0" /><path d="M10 11l0 6" /><path d="M14 11l0 6" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" /><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" />`)

	// 消息发送：message-2（对话气泡）
	iconMessage = svgIcon("adm-message",
		`<path d="M8 9h8" /><path d="M8 13h6" /><path d="M9 18h-3a3 3 0 0 1 -3 -3v-8a3 3 0 0 1 3 -3h12a3 3 0 0 1 3 3v8a3 3 0 0 1 -3 3h-3l-3 3l-3 -3" />`)

	// 重启：refresh（环形箭头）
	iconReboot = svgIcon("adm-reboot",
		`<path d="M20 11a8.1 8.1 0 0 0 -15.5 -2m-.5 -4v4h4" /><path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4" />`)

	// 关闭/断开：power
	iconClose = svgIcon("adm-close",
		`<path d="M7 6a7.75 7.75 0 1 0 10 0" /><path d="M12 4l0 8" />`)

	// 任务/日志：article（带横线的文档）
	iconLog = svgIcon("adm-log",
		`<path d="M3 6a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2l0 -12" /><path d="M7 8h10" /><path d="M7 12h10" /><path d="M7 16h10" />`)
)
