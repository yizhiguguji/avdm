package main

import (
	core "adm/internal/app"
	"fmt"
	"strings"
)

func installSelectionSerials(entries []core.DeviceEntry, selected map[string]bool, currentKey string) ([]string, error) {
	wanted := map[string]bool{}
	for key, checked := range selected {
		if checked {
			wanted[key] = true
		}
	}
	if len(wanted) == 0 {
		wanted[currentKey] = true
	}
	var serials []string
	seen := map[string]bool{}
	for _, entry := range entries {
		if !wanted[entry.Key] {
			continue
		}
		delete(wanted, entry.Key)
		if entry.Active == nil || entry.Active.State != "device" {
			return nil, fmt.Errorf("所选设备 %s 当前不可安装，请连接设备或取消勾选", entry.Label)
		}
		if !seen[entry.Active.Serial] {
			serials = append(serials, entry.Active.Serial)
			seen[entry.Active.Serial] = true
		}
	}
	if len(wanted) > 0 || len(serials) == 0 {
		return nil, fmt.Errorf("所选设备已不可用，请刷新设备列表后重新选择")
	}
	return serials, nil
}

func (g *GUIApp) confirmInstallSelection(title, source string) {
	source = strings.TrimSpace(source)
	if source == "" {
		g.showInfo("还没有上次 APK 记录，请先选择 APK。")
		return
	}
	serials, err := installSelectionSerials(g.entries, g.controlSelected, g.currentDeviceKey)
	if err != nil {
		g.showError(err)
		return
	}
	allowDowngrade := g.allowDowngrade.Checked
	g.confirmAction("确认安装", fmt.Sprintf("目标设备：%d 台\n%s\nAPK：%s\n允许降级：%v", len(serials), strings.Join(serials, "\n"), source, allowDowngrade), func() {
		g.runInstallAction(title, func() error { return g.backend.GUIInstallAPKToDevices(source, serials, allowDowngrade) }, func(blocked []string) error { return g.backend.GUIInstallAPKToDevicesWithReinstall(source, blocked) })
	})
}
