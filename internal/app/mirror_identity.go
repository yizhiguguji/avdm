package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func mirrorOwnerPath(dir string, pid int) string {
	return filepath.Join(dir, fmt.Sprintf("scrcpy_%d.owner", pid))
}

func mirrorProcessStart(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	if err != nil {
		return "", err
	}
	start := strings.TrimSpace(string(out))
	if start == "" {
		return "", fmt.Errorf("读取镜像进程启动时间失败")
	}
	return start, nil
}

func registerScrcpyOwner(dir string, pid int) error {
	start, err := mirrorProcessStart(pid)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(mirrorOwnerPath(dir, pid), []byte(start), 0600)
}

func registeredScrcpyOwner(dir string, pid int) bool {
	record, err := os.ReadFile(mirrorOwnerPath(dir, pid))
	if err != nil {
		return false
	}
	start, err := mirrorProcessStart(pid)
	return err == nil && start == string(record)
}

func appOwnedScrcpyProcess(pid int, command string) bool {
	if !looksLikeScrcpyCommand(strconv.Itoa(pid) + " " + command) {
		return false
	}
	// Older installed versions identified ownership through the visible title.
	if strings.Contains(command, "--window-title=安卓设备矩阵 - ") {
		return true
	}
	dir, err := configDir()
	return err == nil && registeredScrcpyOwner(dir, pid)
}

func (a *App) liveMirrorTitle(entry DeviceEntry) string {
	if entry.Active == nil {
		return ""
	}
	device := *entry.Active
	if entry.AVD != nil && strings.TrimSpace(entry.AVD.Name) != "" {
		device.AVDName = entry.AVD.Name
	}
	if a.cfg != nil {
		if alias := strings.TrimSpace(a.cfg.Aliases[a.deviceAliasKey(&device)]); alias != "" {
			return alias
		}
	}
	if name := strings.TrimSpace(device.AVDName); name != "" {
		return name
	}
	return modelName(&device)
}
