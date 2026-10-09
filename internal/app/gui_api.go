package app

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GUIState struct {
	MirrorAlwaysOnTop map[string]bool
	CurrentDevice     string
	CurrentDeviceKey  string
	Devices           []DeviceEntry
	Tools             []ToolStatus
	LastAPKSource     string
}

type GUIInstallTarget struct {
	Key     string
	Label   string
	Serial  string
	Current bool
}

type GUIAVDActionTarget struct {
	Name      string
	Path      string
	Label     string
	Running   bool
	CanStart  bool
	CanDelete bool
	Reason    string
}

func (a *App) GUIState() (GUIState, error) {
	if a.currentDeviceSnapshot() == nil {
		a.restoreLastReadyDeviceForGUI()
	}
	entries, err := a.allDeviceEntries()
	if err != nil {
		return GUIState{}, err
	}
	// GUIToolStatuses runs adb/IO and reads cfg.ToolPaths (never written in GUI
	// mode); compute it before taking the lock.
	tools := a.GUIToolStatuses()

	a.mu.Lock()
	defer a.mu.Unlock()
	state := GUIState{
		CurrentDevice: a.currentDeviceSummary(),
		Devices:       entries,
		Tools:         tools,
		LastAPKSource: a.cfg.LastAPKSource,
	}
	if a.currentDevice != nil {
		state.CurrentDeviceKey = a.deviceAliasKey(a.currentDevice)
	}
	state.MirrorAlwaysOnTop = make(map[string]bool, len(a.cfg.MirrorAlwaysOnTop))
	for serial, enabled := range a.cfg.MirrorAlwaysOnTop {
		state.MirrorAlwaysOnTop[serial] = enabled
	}
	return state, nil
}

func (a *App) restoreLastReadyDeviceForGUI() bool {
	a.mu.Lock()
	key := strings.TrimSpace(a.cfg.LastDeviceKey)
	a.mu.Unlock()
	if key == "" {
		return false
	}
	devices, err := a.readyDevices()
	if err != nil {
		return false
	}
	for i := range devices {
		if a.deviceAliasKey(&devices[i]) == key {
			a.setCurrentDevice(&devices[i])
			return true
		}
	}
	return false
}

func (a *App) GUIToolStatuses() []ToolStatus {
	a.tools.Refresh()
	statuses := make([]ToolStatus, 0, len(allTools)+1)
	for _, name := range allTools {
		statuses = append(statuses, a.tools.Status(name))
	}
	statuses = append(statuses, accessibilityToolStatus())
	return statuses
}

func accessibilityToolStatus() ToolStatus {
	if accessibilityTrustedNative() {
		return ToolStatus{
			Name:      "macOS 辅助功能",
			Path:      "/Applications/安卓设备矩阵.app",
			Source:    "TCC",
			Available: true,
		}
	}
	return ToolStatus{
		Name:   "macOS 辅助功能",
		Source: "TCC",
		Error:  "未授权。请开启「系统设置 → 隐私与安全性 → 辅助功能」中的「安卓设备矩阵.app」",
	}
}

func (a *App) GUIInstalledSystemImages() ([]string, error) {
	return a.installedSystemImages()
}

func (a *App) GUIDeviceTemplates() ([]DeviceTemplate, error) {
	templates, err := a.deviceTemplates()
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return []DeviceTemplate{{ID: "pixel_9", Name: "Pixel 9"}, {ID: "pixel_9_pro", Name: "Pixel 9 Pro"}}, nil
	}
	return templates, nil
}

func (a *App) GUISetCurrentDevice(entryKey string) error {
	entry, ok, err := a.findDeviceEntryByKey(entryKey)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("未找到设备：%s", entryKey)
	}
	if entry.Active == nil || entry.Active.State != "device" {
		return fmt.Errorf("该设备当前不可直接选择：%s", entry.Label)
	}
	a.setCurrentDevice(entry.Active)
	if entry.Active.IsEmulator {
		_ = focusEmulatorWindow(entry.Active.AVDName, entry.Active.Serial)
	}
	return nil
}

func (a *App) GUIFocusDeviceWindow(entryKey string) error {
	entry, ok, err := a.findDeviceEntryByKey(entryKey)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("未找到设备：%s", entryKey)
	}
	if entry.Active != nil && entry.Active.IsEmulator || entry.AVD != nil && entry.Running {
		name, serial := emulatorEntryIdentity(entry)
		return focusEmulatorWindow(name, serial)
	}
	return fmt.Errorf("该条目没有可聚焦的模拟器窗口：%s", entry.Label)
}

func (a *App) GUIOpenLiveMirror(entryKey string) error {
	return a.openLiveMirror(entryKey, nil)
}

// GUISetLiveMirrorAlwaysOnTop opens/reopens the phone mirror with the chosen
// window level and remembers it only after the window starts successfully.
func (a *App) GUISetLiveMirrorAlwaysOnTop(entryKey string, enabled bool) error {
	return a.openLiveMirror(entryKey, &enabled)
}

func (a *App) openLiveMirror(entryKey string, requestedTop *bool, placements ...*scrcpyWindowPlacement) error {
	a.scrcpyLaunchMu.Lock()
	defer a.scrcpyLaunchMu.Unlock()
	entry, ok, err := a.findDeviceEntryByKey(entryKey)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("未找到设备：%s", entryKey)
	}
	return a.openLiveMirrorEntry(entry, requestedTop, focusEmulatorWindow, placements...)
}

// Keep native emulator windows when window access is available. Without access,
// a ready emulator uses a positioned scrcpy mirror instead of moving its native
// window. Starting/offline emulators still need native window access.
func (a *App) openLiveMirrorEntry(entry DeviceEntry, requestedTop *bool, focus func(string, string) error, placements ...*scrcpyWindowPlacement) error {
	if useNativeEmulatorMirror(entry, placements...) {
		if len(placements) > 0 && placements[0] != nil {
			return &AccessibilityPermissionRequiredError{Operation: "排列模拟器原生窗口"}
		}
		if requestedTop != nil {
			return fmt.Errorf("置顶开关目前支持真机独立窗")
		}
		name, serial := emulatorEntryIdentity(entry)
		if name == "" && serial == "" {
			return fmt.Errorf("未识别模拟器窗口：%s", entry.Label)
		}
		return focus(name, serial)
	}
	if entry.Active == nil {
		return fmt.Errorf("该条目当前没有可实时控制的设备：%s", entry.Label)
	}
	if entry.Active.State != "device" {
		return fmt.Errorf("该设备当前不可实时控制：%s | %s", entry.Label, entry.Active.State)
	}
	title := entry.Label
	if entry.AVD != nil && strings.TrimSpace(entry.AVD.Name) != "" {
		title = entry.AVD.Name
	} else if strings.TrimSpace(entry.Active.AVDName) != "" {
		title = entry.Active.AVDName
	}
	serial := entry.Active.Serial
	alwaysOnTop := true
	if requestedTop != nil {
		alwaysOnTop = *requestedTop
	}
	if err := a.startScrcpySession(serial, title, alwaysOnTop, placements...); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.MirrorAlwaysOnTop == nil {
		a.cfg.MirrorAlwaysOnTop = map[string]bool{}
	}
	if alwaysOnTop {
		a.cfg.MirrorAlwaysOnTop[serial] = true
	} else {
		delete(a.cfg.MirrorAlwaysOnTop, serial)
	}
	return saveConfig(a.cfg)
}

func isEmulatorEntry(entry DeviceEntry) bool {
	return entry.Active != nil && entry.Active.IsEmulator || entry.AVD != nil && entry.Running
}

func useNativeEmulatorMirror(entry DeviceEntry, placements ...*scrcpyWindowPlacement) bool {
	if !isEmulatorEntry(entry) {
		return false
	}
	return len(placements) == 0 || placements[0] == nil || entry.Active == nil || entry.Active.State != "device"
}

func (a *App) GUIOpenLiveMirrors(entryKeys []string) error {
	withoutWindowAccess := nativeWindowAccessUnavailable()
	open := a.GUIOpenLiveMirror
	if withoutWindowAccess {
		plan, err := a.planScrcpyFallbackNative(entryKeys)
		if err != nil {
			return err
		}
		open = func(key string) error {
			return plan.open(key, func(key string, placement *scrcpyWindowPlacement) error {
				return a.openLiveMirror(key, nil, placement)
			})
		}
	}
	return openLiveMirrors(entryKeys, open, func(keys []string) error {
		if withoutWindowAccess {
			return nil
		}
		return a.GUITileEmulatorWindows(keys, 0)
	})
}

func openLiveMirrors(entryKeys []string, open func(string) error, tile func([]string) error) error {
	if len(entryKeys) == 0 {
		return fmt.Errorf("请先选择要打开实时镜像的设备")
	}
	var opened []string
	var failures []error
	for _, key := range normalizedUniqueNames(entryKeys) {
		if err := open(key); err != nil {
			failures = append(failures, fmt.Errorf("%s：%w", key, err))
			continue
		}
		opened = append(opened, key)
	}
	var arrangementErr error
	if len(opened) > 0 {
		arrangementErr = tile(opened)
	}
	if len(failures) > 0 {
		if arrangementErr != nil {
			failures = append(failures, fmt.Errorf("排列已打开窗口失败：%w", arrangementErr))
		}
		return fmt.Errorf("已打开 %d 个实时镜像，部分操作失败：%w", len(opened), errors.Join(failures...))
	}
	if len(opened) == 0 {
		return fmt.Errorf("没有打开任何实时镜像")
	}
	if arrangementErr != nil {
		return fmt.Errorf("已打开 %d 个实时镜像，但排列外部窗口失败：%w", len(opened), arrangementErr)
	}
	return nil
}

func emulatorEntryIdentity(entry DeviceEntry) (name, serial string) {
	if entry.Active != nil {
		name = strings.TrimSpace(entry.Active.AVDName)
		serial = entry.Active.Serial
	}
	if name == "" && entry.AVD != nil {
		name = strings.TrimSpace(entry.AVD.Name)
	}
	return
}

func (a *App) GUITileEmulatorWindows(entryKeys []string, columns int) error {
	entries, err := a.allDeviceEntries()
	if err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, key := range entryKeys {
		key = strings.TrimSpace(key)
		if key != "" {
			selected[key] = true
		}
	}
	filterSelected := len(selected) > 0
	seen := map[string]bool{}
	seenPID := map[int]bool{}
	var targets []emulatorWindowTarget
	var pids []int
	for _, entry := range entries {
		if filterSelected && !selected[entry.Key] {
			continue
		}
		if !entry.Running && (entry.Active == nil || !entry.Active.IsEmulator) {
			continue
		}
		if entry.Active != nil && !entry.Active.IsEmulator {
			if pid, ok := runningScrcpyProcess(entry.Active.Serial); ok && !seenPID[pid] {
				seenPID[pid] = true
				pids = append(pids, pid)
			}
			continue
		}
		target := emulatorWindowTarget{}
		if entry.Active != nil && entry.Active.IsEmulator {
			target.AVDName = entry.Active.AVDName
			target.Serial = entry.Active.Serial
		}
		if target.AVDName == "" && entry.AVD != nil {
			target.AVDName = entry.AVD.Name
		}
		if target.AVDName == "" && target.Serial == "" {
			continue
		}
		key := target.AVDName + "|" + target.Serial
		if seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, target)
		if pid, ok := qemuPIDForAVD(target.AVDName); ok && !seenPID[pid] {
			seenPID[pid] = true
			pids = append(pids, pid)
		}
	}
	if len(pids) > 0 {
		return tileProcessWindows(pids, columns)
	}
	if len(targets) == 0 {
		if filterSelected {
			return fmt.Errorf("勾选项中没有可平铺的运行中外部窗口")
		}
		return fmt.Errorf("没有可平铺的运行中外部窗口")
	}
	return tileEmulatorWindows(targets, columns)
}

func (a *App) GUIDeviceScreenPNG(serial string) ([]byte, error) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return nil, fmt.Errorf("设备 serial 不能为空")
	}
	out, err := a.runToolBytes(toolADB, 8*time.Second, "-s", serial, "exec-out", "screencap", "-p")
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("设备 %s 未返回截图数据", serial)
	}
	if !bytes.HasPrefix(out, []byte{0x89, 'P', 'N', 'G'}) {
		return nil, fmt.Errorf("设备 %s 返回的不是 PNG 截图", serial)
	}
	return out, nil
}

func (a *App) GUITapDevice(serial string, x, y int) error {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return fmt.Errorf("设备 serial 不能为空")
	}
	if x < 0 || y < 0 {
		return fmt.Errorf("点击坐标无效：%d,%d", x, y)
	}
	_, err := a.runToolOutput(toolADB, 5*time.Second, "-s", serial, "shell", "input", "tap", fmt.Sprint(x), fmt.Sprint(y))
	return err
}

func (a *App) GUISwipeDevice(serial string, x1, y1, x2, y2, durationMs int) error {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return fmt.Errorf("设备 serial 不能为空")
	}
	if x1 < 0 || y1 < 0 || x2 < 0 || y2 < 0 {
		return fmt.Errorf("滑动坐标无效：%d,%d -> %d,%d", x1, y1, x2, y2)
	}
	if durationMs < 50 {
		durationMs = 50
	}
	if durationMs > 2000 {
		durationMs = 2000
	}
	_, err := a.runToolOutput(
		toolADB,
		6*time.Second,
		"-s", serial,
		"shell", "input", "swipe",
		fmt.Sprint(x1), fmt.Sprint(y1), fmt.Sprint(x2), fmt.Sprint(y2), fmt.Sprint(durationMs),
	)
	return err
}

func (a *App) GUIKeyEventDevice(serial string, keyCode int) error {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return fmt.Errorf("设备 serial 不能为空")
	}
	if keyCode <= 0 {
		return fmt.Errorf("按键码无效：%d", keyCode)
	}
	_, err := a.runToolOutput(toolADB, 5*time.Second, "-s", serial, "shell", "input", "keyevent", fmt.Sprint(keyCode))
	return err
}

func (a *App) GUIStatusBarDevice(serial, action string) error {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return fmt.Errorf("设备 serial 不能为空")
	}
	command := ""
	switch strings.TrimSpace(action) {
	case "notifications":
		command = "expand-notifications"
	case "settings":
		command = "expand-settings"
	case "collapse":
		command = "collapse"
	default:
		return fmt.Errorf("不支持的状态栏操作：%s", action)
	}
	_, err := a.runToolOutput(toolADB, 5*time.Second, "-s", serial, "shell", "cmd", "statusbar", command)
	return err
}

func (a *App) GUIStartAVD(avdName string) error {
	if strings.TrimSpace(avdName) == "" {
		return fmt.Errorf("AVD 名称不能为空")
	}
	release := a.lockAVDTask(avdName)
	defer release()
	device, err := a.startAVDUnlocked(avdName)
	if err != nil {
		return err
	}
	a.setCurrentDevice(device)
	_ = focusEmulatorWindow(avdName, device.Serial)
	return nil
}

func (a *App) startAVDUnlocked(avdName string) (*ActiveDevice, error) {
	if err := a.launchEmulator(avdName); err != nil {
		return nil, err
	}
	device, err := a.waitForAVDReady(avdName, 180*time.Second)
	if err != nil {
		return nil, err
	}
	if err := a.prepareReadyAVDDefaults(device.Serial); err != nil {
		return nil, fmt.Errorf("模拟器已启动，但启动后初始化失败：%w", err)
	}
	return device, nil
}

func (a *App) GUIStartAVDs(avdNames []string) error {
	names := normalizedUniqueNames(avdNames)
	if len(names) == 0 {
		return fmt.Errorf("请至少选择一个 AVD")
	}
	for _, name := range names {
		avd, ok, err := a.findAVD(name)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("未找到 AVD：%s", name)
		}
		running, reason, err := a.isAVDProbablyRunning(avd.Name)
		if err != nil {
			return err
		}
		if running {
			return fmt.Errorf("不能启动已经运行的 AVD：%s（%s）", avd.Name, reason)
		}
	}
	var failures []string
	var lastReady *ActiveDevice
	var mu sync.Mutex
	runLimited(names, 4, func(name string) {
		release := a.lockAVDTask(name)
		defer release()
		device, err := a.startAVDUnlocked(name)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s：%v", name, err))
			return
		}
		lastReady = device
	})
	if lastReady != nil {
		a.setCurrentDevice(lastReady)
		_ = focusEmulatorWindow(lastReady.AVDName, lastReady.Serial)
	}
	if len(failures) > 0 {
		return fmt.Errorf("批量启动部分失败：\n%s", strings.Join(failures, "\n"))
	}
	return nil
}

func runLimited[T any](items []T, limit int, fn func(T)) {
	if limit < 1 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, item := range items {
		item := item
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fn(item)
		}()
	}
	wg.Wait()
}

func (a *App) GUIRenameAVD(oldName, newName string) error {
	release := a.lockAVDTask(oldName)
	defer release()
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("原名称和新名称都不能为空")
	}
	if oldName == newName {
		return fmt.Errorf("新名称和原名称相同")
	}
	avd, ok, err := a.findAVD(oldName)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("未找到 AVD：%s", oldName)
	}
	if _, exists, err := a.findAVD(newName); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("AVD 已存在：%s", newName)
	}
	running, reason, err := a.isAVDProbablyRunning(avd.Name)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("运行中的 AVD 暂不允许改名，请先关机：%s（%s）", avd.Name, reason)
	}
	_, err = a.runToolOutput(toolAVDManager, 2*time.Minute, "move", "avd", "-n", avd.Name, "-r", newName)
	return err
}

func (a *App) GUICreateAVD(name, image, deviceID string, start bool) error {
	release := a.lockAVDTask(strings.TrimSpace(name))
	defer release()
	name = strings.TrimSpace(name)
	image = strings.TrimSpace(image)
	deviceID = strings.TrimSpace(deviceID)
	if name == "" || image == "" || deviceID == "" {
		return fmt.Errorf("名称、system image、设备模板都不能为空")
	}
	if _, exists, err := a.findAVD(name); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("AVD 已存在：%s，请换一个名称", name)
	}
	if _, err := a.runToolOutput(toolAVDManager, 3*time.Minute, "create", "avd", "-n", name, "-k", image, "-d", deviceID); err != nil {
		return err
	}
	if err := a.prepareAVDDataPartition(name); err != nil {
		return fmt.Errorf("AVD 已创建，但数据分区配置写入失败：%w", err)
	}
	if err := a.prepareAVDForKeyboardForwarding(name); err != nil {
		return fmt.Errorf("AVD 已创建，但键盘配置写入失败：%w", err)
	}
	if start {
		device, err := a.startAVDUnlocked(name)
		if err != nil {
			return err
		}
		a.setCurrentDevice(device)
		_ = focusEmulatorWindow(name, device.Serial)
		return nil
	}
	return nil
}

func (a *App) GUIDeleteAVD(avdName string, confirmed, forceClose bool) error {
	release := a.lockAVDTask(avdName)
	defer release()
	avd, ok, err := a.findAVD(avdName)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("未找到 AVD：%s", avdName)
	}
	if !confirmed {
		return fmt.Errorf("删除 AVD 前需要确认：%s", avd.Name)
	}
	running, reason, err := a.isAVDProbablyRunning(avd.Name)
	if err != nil {
		return err
	}
	if running {
		if !forceClose {
			return fmt.Errorf("该 AVD 正在运行或疑似运行。请勾选“强制关闭并删除”后重试：%s", reason)
		}
		if err := a.forceCloseAVDForDelete(avd.Name); err != nil {
			return err
		}
	}
	_, err = a.runToolOutput(toolAVDManager, 2*time.Minute, "delete", "avd", "-n", avd.Name)
	return err
}

func (a *App) GUIDeleteAVDs(avdNames []string, confirmed, forceClose bool) error {
	names := normalizedUniqueNames(avdNames)
	if len(names) == 0 {
		return fmt.Errorf("请至少选择一个 AVD")
	}
	if !confirmed {
		return fmt.Errorf("批量删除 AVD 前需要确认")
	}

	var avds []AVD
	var runningAVDs []string
	for _, name := range names {
		avd, ok, err := a.findAVD(name)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("未找到 AVD：%s", name)
		}
		running, reason, err := a.isAVDProbablyRunning(avd.Name)
		if err != nil {
			return err
		}
		if running {
			if !forceClose {
				runningAVDs = append(runningAVDs, fmt.Sprintf("%s：%s", avd.Name, reason))
			}
		}
		avds = append(avds, avd)
	}
	if len(runningAVDs) > 0 {
		return fmt.Errorf("以下 AVD 正在运行或疑似运行。请勾选“强制关闭并删除”后重试：\n%s", strings.Join(runningAVDs, "\n"))
	}

	var failures []string
	for _, avd := range avds {
		if err := a.GUIDeleteAVD(avd.Name, confirmed, forceClose); err != nil {
			failures = append(failures, fmt.Sprintf("%s：%v", avd.Name, err))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("批量删除部分失败：\n%s", strings.Join(failures, "\n"))
	}
	return nil
}

func (a *App) forceCloseAVDForDelete(avdName string) error {
	devices, err := a.activeDevices()
	if err != nil {
		return err
	}
	var serials []string
	for _, device := range devices {
		if device.AVDName == avdName {
			serials = append(serials, device.Serial)
			if device.State == "device" {
				_, _ = a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "emu", "kill")
			}
		}
	}
	for _, proc := range runningEmulatorProcessesForAVD(avdName) {
		if proc.PID != "" {
			_ = forceTerminateProcess(proc.PID)
		}
		for _, serial := range proc.Serials {
			serials = append(serials, serial)
		}
	}
	for _, serial := range normalizedUniqueNames(serials) {
		a.clearCurrentDeviceIf(serial)
		_ = a.waitForSerialGone(serial, 20*time.Second)
	}
	running, reason, err := a.isAVDProbablyRunning(avdName)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("已尝试关闭模拟器，但仍检测到运行态：%s", reason)
	}
	return nil
}

func (a *App) GUIAVDActionTargets() ([]GUIAVDActionTarget, error) {
	avds, err := a.avds()
	if err != nil {
		return nil, err
	}
	targets := make([]GUIAVDActionTarget, 0, len(avds))
	for _, avd := range avds {
		running, reason, err := a.isAVDProbablyRunning(avd.Name)
		if err != nil {
			return nil, err
		}
		status := "未启动"
		if running {
			status = "运行中"
		}
		label := fmt.Sprintf("%s | %s", avd.Name, status)
		if reason != "" {
			label += " | " + reason
		}
		targets = append(targets, GUIAVDActionTarget{
			Name:      avd.Name,
			Path:      avd.Path,
			Label:     label,
			Running:   running,
			CanStart:  !running,
			CanDelete: true,
			Reason:    reason,
		})
	}
	return targets, nil
}

func (a *App) GUIInstallAPK(source string, allowDowngrade bool) error {
	device, err := a.requireCurrentReadyDevice()
	if err != nil {
		return err
	}
	return a.installAPKSourceToDeviceForGUI(device, source, installMode{AllowDowngrade: allowDowngrade, AllowTestOnly: true})
}

// GUIInstallAPKWithReinstall installs to the current device, allowing the
// uninstall+reinstall fallback when a signature conflict or downgrade blocks replacement. This
// wipes the app's data; the GUI must confirm with the user first.
func (a *App) GUIInstallAPKWithReinstall(source string) error {
	device, err := a.requireCurrentReadyDevice()
	if err != nil {
		return err
	}
	return a.installAPKSourceToDeviceForGUI(device, source, installMode{AllowDowngrade: true, AllowTestOnly: true, AllowReinstall: true})
}

func (a *App) GUIInstallTargets() ([]GUIInstallTarget, error) {
	if a.currentDeviceSnapshot() == nil {
		a.restoreLastReadyDeviceForGUI()
	}
	entries, err := a.allDeviceEntries()
	if err != nil {
		return nil, err
	}
	currentSerial := ""
	if cur := a.currentDeviceSnapshot(); cur != nil {
		currentSerial = cur.Serial
	}
	targets := make([]GUIInstallTarget, 0, len(entries))
	for _, entry := range entries {
		if entry.Active == nil || entry.Active.State != "device" {
			continue
		}
		targets = append(targets, GUIInstallTarget{
			Key:     entry.Key,
			Label:   entry.Label,
			Serial:  entry.Active.Serial,
			Current: entry.Active.Serial == currentSerial,
		})
	}
	return targets, nil
}

func (a *App) GUIInstallAPKToDevices(source string, deviceSerials []string, allowDowngrade bool) error {
	return a.installAPKToDevices(source, deviceSerials, installMode{AllowDowngrade: allowDowngrade, AllowTestOnly: true})
}

// GUIInstallAPKToDevicesWithReinstall retries a batch install allowing the
// uninstall+reinstall fallback for devices with signature conflicts or blocked downgrades.
// This wipes app data on those devices; the GUI must confirm first.
func (a *App) GUIInstallAPKToDevicesWithReinstall(source string, deviceSerials []string) error {
	return a.installAPKToDevices(source, deviceSerials, installMode{AllowDowngrade: true, AllowTestOnly: true, AllowReinstall: true})
}

func (a *App) installAPKToDevices(source string, deviceSerials []string, mode installMode) error {
	selected := map[string]bool{}
	for _, serial := range deviceSerials {
		serial = strings.TrimSpace(serial)
		if serial != "" {
			selected[serial] = true
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("请至少选择一台设备")
	}

	devices, err := a.activeDevices()
	if err != nil {
		return err
	}
	targets, unavailable := installTargetsFromActiveDevices(selected, devices)
	if len(unavailable) > 0 {
		return fmt.Errorf("以下设备当前不可安装（需要 state=device）：\n%s", strings.Join(unavailable, "\n"))
	}
	if len(targets) == 0 {
		return fmt.Errorf("没有可安装的设备")
	}

	apkSource, apkPath, cleanup, err := prepareAPKSourceForGUI(source)
	if err != nil {
		return err
	}
	defer cleanup()

	successCount := 0
	var failures []string
	var blockedPkg string
	var blockedReasons []string
	var blockedSerials []string
	for _, target := range targets {
		release := a.lockDeviceTask(target.device.Serial)
		err := a.smartInstallAPKForGUI(target.device.Serial, apkPath, mode)
		release()
		if err == nil {
			successCount++
			continue
		}
		if blocked, ok := AsReinstallRequired(err); ok {
			if blockedPkg == "" {
				blockedPkg = blocked.Package
			}
			blockedSerials = append(blockedSerials, target.device.Serial)
			blockedReasons = append(blockedReasons, blocked.Reason)
			continue
		}
		failures = append(failures, fmt.Sprintf("%s：%v", target.label, err))
	}
	if successCount > 0 {
		a.rememberAPKSourceForGUI(apkSource)
	}
	// Surface replacement blocks distinctly so the GUI can offer the confirmed
	// uninstall+reinstall retry for exactly those devices.
	if len(blockedSerials) > 0 {
		detail := ""
		if len(failures) > 0 {
			detail = "另有其它失败：\n" + strings.Join(failures, "\n")
		}
		return &ReinstallRequiredError{Package: blockedPkg, Serials: blockedSerials, Detail: detail, Reason: strings.Join(uniqueStrings(blockedReasons), "；")}
	}
	if len(failures) > 0 {
		if successCount > 0 {
			return fmt.Errorf("已成功安装 %d/%d 台，以下设备失败：\n%s", successCount, len(targets), strings.Join(failures, "\n"))
		}
		return fmt.Errorf("所有设备安装失败：\n%s", strings.Join(failures, "\n"))
	}
	return nil
}

type installTarget struct {
	label  string
	device ActiveDevice
}

func installTargetsFromActiveDevices(selected map[string]bool, devices []ActiveDevice) ([]installTarget, []string) {
	seen := map[string]bool{}
	var targets []installTarget
	var unavailable []string
	for _, device := range devices {
		if !selected[device.Serial] {
			continue
		}
		seen[device.Serial] = true
		label := installDeviceLabel(device)
		if device.State != "device" {
			unavailable = append(unavailable, label)
			continue
		}
		targets = append(targets, installTarget{
			label:  label,
			device: device,
		})
	}
	for serial := range selected {
		if !seen[serial] {
			unavailable = append(unavailable, serial)
		}
	}
	return targets, unavailable
}

func installDeviceLabel(device ActiveDevice) string {
	if device.AVDName != "" {
		return fmt.Sprintf("%s | %s", device.AVDName, device.Serial)
	}
	if model := device.Details["model"]; model != "" {
		return fmt.Sprintf("%s | %s", model, device.Serial)
	}
	return device.Serial
}

func (a *App) GUIReinstallLastAPK(allowDowngrade bool) error {
	source := a.lastAPKSource()
	if source == "" {
		return fmt.Errorf("还没有上次 APK 记录")
	}
	return a.GUIInstallAPK(source, allowDowngrade)
}

// GUIReinstallLastAPKWithReinstall reinstalls the last APK, allowing the
// uninstall+reinstall downgrade fallback. Wipes app data; confirm first.
func (a *App) GUIReinstallLastAPKWithReinstall() error {
	source := a.lastAPKSource()
	if source == "" {
		return fmt.Errorf("还没有上次 APK 记录")
	}
	return a.GUIInstallAPKWithReinstall(source)
}

func (a *App) lastAPKSource() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg.LastAPKSource
}

// GUIListPackages limits the default list to positively identified user installs.
// Android's non-system (-3) classification alone also includes OEM provisioning.
func (a *App) GUIListPackages(userInstalledOnly bool, filter string) ([]string, error) {
	device, err := a.requireCurrentReadyDevice()
	if err != nil {
		return nil, err
	}
	return a.GUIListPackagesForDevice(device.Serial, userInstalledOnly, filter)
}

func (a *App) GUIListPackagesForDevice(serial string, userInstalledOnly bool, filter string) ([]string, error) {
	device, err := a.requireReadySerial(serial)
	if err != nil {
		return nil, err
	}
	args := []string{"-s", device.Serial, "shell", "pm", "list", "packages"}
	var userPackages map[string]bool
	if userInstalledOnly {
		userOutput, err := a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "shell", "am", "get-current-user")
		if err != nil {
			return nil, err
		}
		userID, err := strconv.Atoi(strings.TrimSpace(userOutput))
		if err != nil || userID < 0 {
			return nil, fmt.Errorf("无法识别设备当前用户：%s", userOutput)
		}
		args = append(args, "-3", "--user", strconv.Itoa(userID))
		dump, err := a.runToolOutput(toolADB, 30*time.Second, "-s", device.Serial, "shell", "dumpsys", "package", "packages")
		if err != nil {
			return nil, err
		}
		launcherOutput, err := a.runToolOutput(toolADB, 30*time.Second, "-s", device.Serial, "shell", "cmd", "package", "query-activities", "--brief", "--components", "--user", strconv.Itoa(userID), "-a", "android.intent.action.MAIN", "-c", "android.intent.category.LAUNCHER")
		if err != nil {
			return nil, fmt.Errorf("读取用户应用入口失败：%w", err)
		}
		userPackages, err = userInstalledPackageNames(dump, userID, launcherPackageNames(launcherOutput))
		if err != nil {
			return nil, err
		}
	}
	out, err := a.runToolOutput(toolADB, 30*time.Second, args...)
	if err != nil {
		return nil, err
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	var packages []string
	for _, pkg := range packageLinesToNames(out) {
		if userInstalledOnly && !userPackages[pkg] {
			continue
		}
		if filter == "" || strings.Contains(strings.ToLower(pkg), filter) {
			packages = append(packages, pkg)
		}
	}
	return packages, nil
}

func (a *App) GUIUninstallPackage(pkg string, keepData, user0 bool) error {
	device, err := a.requireCurrentReadyDevice()
	if err != nil {
		return err
	}
	return a.GUIUninstallPackageForDevice(device.Serial, pkg, keepData, user0)
}

func (a *App) GUIUninstallPackageForDevice(serial string, pkg string, keepData, user0 bool) error {
	release := a.lockDeviceTask(serial)
	defer release()
	device, err := a.requireReadySerial(serial)
	if err != nil {
		return err
	}
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return fmt.Errorf("包名不能为空")
	}
	_, runErr := a.runToolOutput(toolADB, 2*time.Minute, uninstallArgs(device.Serial, pkg, keepData, user0)...)
	if runErr == nil {
		a.mu.Lock()
		a.cfg.LastPackageName = pkg
		a.persistConfigLocked()
		a.mu.Unlock()
	}
	return runErr
}

func (a *App) GUISendText(text string, allowADBKeyboard bool) error {
	device, err := a.requireCurrentReadyDevice()
	if err != nil {
		return err
	}
	return a.GUISendTextForDevice(device.Serial, text, allowADBKeyboard)
}

func (a *App) GUISendTextForDevice(serial string, text string, allowADBKeyboard bool) error {
	release := a.lockDeviceTask(serial)
	defer release()
	device, err := a.requireReadySerial(serial)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("文本不能为空")
	}
	if isSimpleADBInputText(text) {
		return a.sendSimpleInputText(device.Serial, text)
	}
	if !allowADBKeyboard {
		return fmt.Errorf("中文/复杂文本需要启用 ADB Keyboard")
	}
	return a.sendADBKeyboardText(device.Serial, text)
}

func (a *App) GUIInstallADBKeyboard() error {
	device, err := a.requireCurrentReadyDevice()
	if err != nil {
		return err
	}
	return a.GUIInstallADBKeyboardForDevice(device.Serial)
}

func (a *App) GUIInstallADBKeyboardForDevice(serial string) error {
	release := a.lockDeviceTask(serial)
	defer release()
	device, err := a.requireReadySerial(serial)
	if err != nil {
		return err
	}
	return a.installADBKeyboardForDevice(device.Serial)
}

func (a *App) GUIRebootCurrent() error {
	device, err := a.requireCurrentReadyDevice()
	if err != nil {
		return err
	}
	return a.GUIRebootDevice(device.Serial)
}

func (a *App) GUIRebootDevice(serial string) error {
	release := a.lockDeviceTask(serial)
	defer release()
	device, err := a.requireReadySerial(serial)
	if err != nil {
		return err
	}
	before := *device
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", before.Serial, "reboot"); err != nil {
		return err
	}
	a.clearCurrentDeviceIf(before.Serial)
	if before.AVDName == "" {
		return nil
	}
	_ = a.waitForSerialGone(before.Serial, 30*time.Second)
	restored, err := a.waitForAVDReady(before.AVDName, 180*time.Second)
	if err != nil {
		return err
	}
	restoredKey := a.deviceAliasKey(&before)
	a.mu.Lock()
	if a.currentDevice == nil && a.cfg.LastDeviceKey == restoredKey {
		copied := *restored
		a.currentDevice = &copied
		a.cfg.LastDeviceKey = restoredKey
		a.persistConfigLocked()
	}
	a.mu.Unlock()
	return nil
}

func (a *App) GUICloseCurrent(serialConfirmation string) error {
	device, err := a.requireCurrentReadyDevice()
	if err != nil {
		return err
	}
	return a.GUICloseDeviceConfirmed(device.Serial, serialConfirmation)
}

func (a *App) GUICloseDevice(key string) error {
	entry, ok, err := a.findDeviceEntryByKey(key)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("未找到设备：%s", key)
	}
	return closeEmulatorEntry(entry, a.GUICloseAVD, func(serial string) error { return a.GUICloseDeviceConfirmed(serial, serial) })
}

// Non-ready AVDs use the existing process-aware close flow; no deletion occurs.
func closeEmulatorEntry(entry DeviceEntry, closeAVD func(string) error, closeSerial func(string) error) error {
	if entry.Active != nil && !entry.Active.IsEmulator {
		if !strings.Contains(entry.Active.Serial, ":") {
			return fmt.Errorf("真机关机需要输入 serial 确认：%s", entry.Active.Serial)
		}
		return closeSerial(entry.Active.Serial)
	}
	if entry.AVD != nil && entry.Running || entry.Active != nil && entry.Active.IsEmulator {
		name, serial := emulatorEntryIdentity(entry)
		if name != "" {
			return closeAVD(name)
		}
		if serial != "" {
			return closeSerial(serial)
		}
	}
	return fmt.Errorf("设备未连接或未启动：%s", entry.Label)
}

func (a *App) GUICloseDeviceConfirmed(serial, serialConfirmation string) error {
	if strings.TrimSpace(serial) == "" || serial != serialConfirmation {
		return fmt.Errorf("关闭/断开设备需要输入 serial 确认：%s", serial)
	}
	release := a.lockDeviceTask(serial)
	defer release()
	devices, err := a.activeDevices()
	if err != nil {
		return err
	}
	for _, device := range devices {
		if device.Serial == serial {
			if device.IsEmulator && device.State != "device" && device.AVDName != "" {
				// lockDeviceTask already owns the AVD lock; do not call the
				// locking public wrapper from here.
				return a.forceCloseAVDForDelete(device.AVDName)
			}
			return a.closeDeviceForGUI(device, serialConfirmation)
		}
	}
	return fmt.Errorf("设备已不可用：%s", serial)
}

func (a *App) GUICloseDevices(keys []string) error {
	keys = normalizedUniqueNames(keys)
	if len(keys) == 0 {
		return fmt.Errorf("请至少选择一个运行中的模拟器")
	}
	var targets []DeviceEntry
	for _, key := range keys {
		entry, ok, err := a.findDeviceEntryByKey(key)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("未找到设备：%s", key)
		}
		if err := validateEmulatorCloseEntry(entry); err != nil {
			return err
		}
		targets = append(targets, entry)
	}
	var failures []error
	var mu sync.Mutex
	runLimited(targets, 4, func(entry DeviceEntry) {
		err := closeEmulatorEntry(entry, a.GUICloseAVD, func(serial string) error { return a.GUICloseDeviceConfirmed(serial, serial) })
		if err != nil {
			mu.Lock()
			failures = append(failures, fmt.Errorf("%s：%w", entry.Label, err))
			mu.Unlock()
		}
	})
	if len(failures) > 0 {
		return fmt.Errorf("批量关机部分失败：%w", errors.Join(failures...))
	}
	return nil
}

func validateEmulatorCloseEntry(entry DeviceEntry) error {
	if entry.Active != nil && !entry.Active.IsEmulator {
		return fmt.Errorf("批量关机只支持模拟器：%s", entry.Label)
	}
	if entry.Active != nil && entry.Active.IsEmulator || entry.AVD != nil && entry.Running {
		return nil
	}
	return fmt.Errorf("模拟器未启动：%s", entry.Label)
}

func (a *App) GUICloseAVD(name string) error {
	release := a.lockAVDTask(name)
	defer release()
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("AVD 名称为空")
	}
	avd, ok, err := a.findAVD(name)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("未找到 AVD：%s", name)
	}
	if err := a.forceCloseAVDForDelete(avd.Name); err != nil {
		return err
	}
	return nil
}

func (a *App) findDeviceEntryByKey(key string) (DeviceEntry, bool, error) {
	entries, err := a.allDeviceEntries()
	if err != nil {
		return DeviceEntry{}, false, err
	}
	for _, entry := range entries {
		if entry.Key == key {
			return entry, true, nil
		}
	}
	return DeviceEntry{}, false, nil
}

func (a *App) findAVD(name string) (AVD, bool, error) {
	avds, err := a.avds()
	if err != nil {
		return AVD{}, false, err
	}
	for _, avd := range avds {
		if avd.Name == name {
			return avd, true, nil
		}
	}
	return AVD{}, false, nil
}

func normalizedUniqueNames(names []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, name)
	}
	return result
}

func (a *App) requireCurrentReadyDevice() (*ActiveDevice, error) {
	device := a.currentDeviceSnapshot()
	if device == nil {
		return nil, fmt.Errorf("当前设备不可用，请先选择一个 state=device 的设备")
	}
	return a.requireReadySerial(device.Serial)
}

func (a *App) installAPKSourceToDeviceForGUI(device *ActiveDevice, apkSource string, mode installMode) error {
	apkSource, apkPath, cleanup, err := prepareAPKSourceForGUI(apkSource)
	if err != nil {
		return err
	}
	defer cleanup()
	release := a.lockDeviceTask(device.Serial)
	defer release()
	if err := a.smartInstallAPKForGUI(device.Serial, apkPath, mode); err != nil {
		return err
	}
	a.rememberAPKSourceForGUI(apkSource)
	return nil
}

func prepareAPKSourceForGUI(apkSource string) (string, string, func(), error) {
	apkSource = normalizeAPKSource(apkSource)
	if apkSource == "" {
		return "", "", func() {}, fmt.Errorf("APK 路径或 URL 不能为空")
	}
	apkPath := apkSource
	cleanup := func() {}
	if isHTTPURL(apkSource) {
		downloadedPath, downloadedCleanup, err := downloadAPK(apkSource)
		if err != nil {
			return "", "", cleanup, err
		}
		apkPath = downloadedPath
		cleanup = downloadedCleanup
	} else {
		apkPath = normalizeAPKPath(apkSource)
		if !fileExists(apkPath) {
			return "", "", cleanup, fmt.Errorf("APK 文件不存在：%s", apkPath)
		}
		if strings.ToLower(filepath.Ext(apkPath)) != ".apk" {
			return "", "", cleanup, fmt.Errorf("文件扩展名不是 .apk：%s", apkPath)
		}
	}
	return apkSource, apkPath, cleanup, nil
}

func (a *App) rememberAPKSourceForGUI(apkSource string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.LastAPKSource = apkSource
	a.persistConfigLocked()
}

func (a *App) smartInstallAPKForGUI(serial, apkPath string, mode installMode) error {
	if err := a.ensureAPKInstallSpace(serial, apkPath); err != nil {
		return err
	}
	for attempt := 1; attempt <= 3; attempt++ {
		out, err := a.runToolOutput(toolADB, 10*time.Minute, installArgs(serial, apkPath, mode)...)
		if err == nil {
			return nil
		}
		combined := out + "\n" + err.Error()
		hint := analyzeInstallFailure(combined)
		changed := false
		if hint.NeedsDowngrade && !mode.AllowDowngrade {
			return fmt.Errorf("检测到降级安装。请勾选“允许降级安装”后重试")
		}
		if hint.NeedsSignatureReinstall || (hint.NeedsDowngrade && mode.AllowDowngrade) {
			// Ask before deleting data; only an explicitly confirmed retry may
			// uninstall the existing package.
			pkg, pkgErr := apkPackageName(apkPath)
			if !mode.AllowReinstall {
				reason := "正式包无法直接降级"
				if hint.NeedsSignatureReinstall {
					reason = "新旧 APK 签名不一致，无法覆盖安装"
				}
				return &ReinstallRequiredError{Package: pkg, Serials: []string{serial}, Reason: reason}
			}
			if pkgErr != nil {
				return fmt.Errorf("需要卸载重装但无法解析 APK 包名：%w", pkgErr)
			}
			return a.reinstallAfterUninstall(serial, pkg, apkPath, mode)
		}
		if hint.NeedsTestOnly && !mode.AllowTestOnly {
			mode.AllowTestOnly = true
			changed = true
		}
		if !changed {
			explanation := explainInstallFailure(combined)
			if explanation != "" {
				return fmt.Errorf("安装失败：%s\n%s", err, explanation)
			}
			return fmt.Errorf("安装失败：%s", err)
		}
	}
	return fmt.Errorf("安装失败：重试次数已用完")
}

// reinstallAfterUninstall uninstalls pkg on serial and then does a fresh
// install of apkPath. This is the only reliable way to move a release build to
// a lower versionCode; it wipes the app's data. Callers must have confirmed
// with the user before setting mode.AllowReinstall.
func (a *App) reinstallAfterUninstall(serial, pkg, apkPath string, mode installMode) error {
	if _, err := a.runToolOutput(toolADB, 2*time.Minute, "-s", serial, "uninstall", pkg); err != nil {
		return fmt.Errorf("卸载旧版本 %s 失败：%w", pkg, err)
	}
	args := []string{"-s", serial, "install"}
	if mode.AllowTestOnly {
		args = append(args, "-t")
	}
	args = append(args, apkPath)
	if out, err := a.runToolOutput(toolADB, 10*time.Minute, args...); err != nil {
		explanation := explainInstallFailure(out + "\n" + err.Error())
		if explanation != "" {
			return fmt.Errorf("卸载后重新安装失败：%s\n%s", err, explanation)
		}
		return fmt.Errorf("卸载后重新安装失败：%s", err)
	}
	return nil
}

func (a *App) closeDeviceForGUI(d ActiveDevice, serialConfirmation string) error {
	if serialConfirmation != d.Serial {
		return fmt.Errorf("关闭/断开设备需要输入 serial 确认：%s", d.Serial)
	}
	switch {
	case d.IsEmulator:
		if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", d.Serial, "emu", "kill"); err != nil {
			return err
		}
		a.clearCurrentDeviceIf(d.Serial)
		return a.waitForSerialGone(d.Serial, 20*time.Second)
	case strings.Contains(d.Serial, ":"):
		if _, err := a.runToolOutput(toolADB, 10*time.Second, "disconnect", d.Serial); err != nil {
			return err
		}
		a.clearCurrentDeviceIf(d.Serial)
		return nil
	default:
		if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", d.Serial, "shell", "reboot", "-p"); err != nil {
			return err
		}
		a.clearCurrentDeviceIf(d.Serial)
		return a.waitForSerialGone(d.Serial, 30*time.Second)
	}
}
