package app

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

type App struct {
	cfg    *Config
	tools  *ToolResolver
	reader *bufio.Reader

	// mu guards the mutable shared state accessed concurrently by the GUI:
	// currentDevice and the cfg "Last*" fields (plus config persistence). It
	// is only ever held around in-memory field access and the fast local
	// config write — never around adb/network IO or the emulator boot wait —
	// so concurrent operations (e.g. refreshing while an emulator boots) do
	// not block each other. cfg.Aliases/cfg.ToolPaths are never written in GUI
	// mode, so their reads need no lock.
	mu            sync.Mutex
	currentDevice *ActiveDevice

	scrcpyLaunchMu sync.Mutex // serializes window reuse and replacement
	remoteMu       sync.Mutex
	scrcpySessions map[string]*scrcpySession
}

func New() (*App, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	app := &App{
		cfg:            cfg,
		reader:         bufio.NewReader(os.Stdin),
		scrcpySessions: map[string]*scrcpySession{},
	}
	app.tools = newToolResolver(cfg)
	normalizeProcessEnv(app.tools)
	return app, nil
}

func (a *App) RunInteractive() error {
	a.printToolSummary()
	if !a.restoreLastReadyDevice() {
		a.autoSelectSingleReadyDevice()
	}
	for {
		fmt.Println()
		fmt.Println("==== 安卓设备矩阵 ====")
		fmt.Println("当前设备：" + a.currentDeviceSummary())
		options := []string{
			"选择/切换当前设备",
			"安装应用(APK)",
			"卸载应用",
			"当前设备操作台",
			"全部设备/模拟器管理",
			"查看设备",
			"别名管理",
			"工具诊断/路径设置",
		}
		if a.currentDevice != nil && a.cfg.LastAPKSource != "" {
			options = append(options[:2], append([]string{"重新安装上次 APK"}, options[2:]...)...)
		}
		choice, ok := a.selectOptionWithCancel("主菜单", options, "退出")
		if !ok {
			return nil
		}
		var err error
		option := options[choice]
		switch option {
		case "选择/切换当前设备":
			err = a.selectCurrentDeviceFlow()
		case "安装应用(APK)":
			err = a.installAPKFlow()
		case "重新安装上次 APK":
			err = a.reinstallLastAPKFlow()
		case "卸载应用":
			err = a.uninstallAPKFlow()
		case "当前设备操作台":
			err = a.currentDeviceConsoleFlow()
		case "全部设备/模拟器管理":
			err = a.deviceManagementFlow()
		case "查看设备":
			err = a.viewDevicesFlow()
		case "别名管理":
			err = a.aliasManagementFlow()
		case "工具诊断/路径设置":
			err = a.toolsFlow()
		}
		if err != nil {
			fmt.Println("错误：", err)
		}
	}
}

func (a *App) selectOption(title string, options []string) (int, bool) {
	return a.selectOptionWithCancel(title, options, "返回上一级")
}

func (a *App) selectOptionWithCancel(title string, options []string, cancelLabel string) (int, bool) {
	for {
		fmt.Println()
		fmt.Println(title + ":")
		for i, option := range options {
			fmt.Printf("  %d. %s\n", i+1, option)
		}
		fmt.Println("  0. " + cancelLabel)
		input := a.prompt("请选择: ")
		if input == "" || input == "0" {
			return -1, false
		}
		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > len(options) {
			fmt.Println("请输入列表里的数字。")
			continue
		}
		return n - 1, true
	}
}

func (a *App) prompt(label string) string {
	fmt.Print(label)
	text, _ := a.reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func (a *App) confirm(message string) bool {
	input := strings.ToLower(a.prompt(message + " 输入 y 确认: "))
	return input == "y" || input == "yes"
}

func (a *App) confirmExact(message, expected string) bool {
	input := a.prompt(message + " 请输入 " + expected + " 确认: ")
	return strings.TrimSpace(input) == expected
}

func (a *App) printToolSummary() {
	fmt.Println("工具探测:")
	for _, name := range allTools {
		status := a.tools.Status(name)
		if status.Available {
			fmt.Printf("  %-10s %s (%s)\n", name, status.Path, status.Source)
			continue
		}
		fmt.Printf("  %-10s 不可用：%s\n", name, status.Error)
	}
}

func (a *App) formatDeviceEntry(entry DeviceEntry) string {
	alias := strings.TrimSpace(a.cfg.Aliases[entry.Key])
	status := "未启动"
	serial := ""
	if entry.Active != nil {
		status = entry.Active.State
		serial = entry.Active.Serial
	}
	name := ""
	if entry.AVD != nil {
		name = entry.AVD.Name
	} else if entry.Active != nil && entry.Active.AVDName != "" {
		name = entry.Active.AVDName
	}
	if name == "" && entry.Active != nil {
		name = modelName(entry.Active)
	}
	if alias != "" {
		name = fmt.Sprintf("%s [%s]", alias, name)
	}
	if serial != "" {
		return fmt.Sprintf("%s | %s | %s | %s", entry.Kind, name, status, serial)
	}
	return fmt.Sprintf("%s | %s | %s", entry.Kind, name, status)
}

func modelName(d *ActiveDevice) string {
	if d == nil {
		return ""
	}
	if model := d.Details["model"]; model != "" {
		return model
	}
	if product := d.Details["product"]; product != "" {
		return product
	}
	return d.Serial
}

func (a *App) deviceAliasKey(d *ActiveDevice) string {
	key := aliasKeyForSerial(d.Serial)
	if d.AVDName != "" {
		key = aliasKeyForAVD(d.AVDName)
	}
	return key
}

func (a *App) deviceDisplayName(d *ActiveDevice) string {
	name := modelName(d)
	if d.AVDName != "" {
		name = d.AVDName
	}
	if alias := a.cfg.Aliases[a.deviceAliasKey(d)]; alias != "" {
		name = fmt.Sprintf("%s [%s]", alias, name)
	}
	return name
}

func (a *App) deviceLabel(d *ActiveDevice) string {
	if d == nil {
		return "未选择"
	}
	return fmt.Sprintf("%s | %s", a.deviceDisplayName(d), d.Serial)
}

func (a *App) currentDeviceSummary() string {
	if a.currentDevice == nil {
		return "未选择"
	}
	return a.deviceLabel(a.currentDevice)
}

func (a *App) autoSelectSingleReadyDevice() {
	devices, err := a.readyDevices()
	if err != nil || len(devices) != 1 {
		return
	}
	a.setCurrentDevice(&devices[0])
	fmt.Println("已自动选择当前设备：" + a.currentDeviceSummary())
}

func (a *App) restoreLastReadyDevice() bool {
	key := strings.TrimSpace(a.cfg.LastDeviceKey)
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
			fmt.Println("已恢复上次当前设备：" + a.currentDeviceSummary())
			return true
		}
	}
	return false
}

func (a *App) setCurrentDevice(d *ActiveDevice) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if d == nil {
		a.currentDevice = nil
		return
	}
	copied := *d
	a.currentDevice = &copied
	a.cfg.LastDeviceKey = a.deviceAliasKey(&copied)
	a.persistConfigLocked()
}

func (a *App) clearCurrentDeviceIf(serial string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.currentDevice != nil && a.currentDevice.Serial == serial {
		a.currentDevice = nil
	}
}

// currentDeviceSnapshot returns a copy of the current device (or nil) under
// the lock, safe to use without further synchronization.
func (a *App) currentDeviceSnapshot() *ActiveDevice {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.currentDevice == nil {
		return nil
	}
	copied := *a.currentDevice
	return &copied
}

// persistConfigLocked writes cfg to disk. The caller must hold a.mu.
func (a *App) persistConfigLocked() {
	if err := saveConfig(a.cfg); err != nil {
		fmt.Println("配置保存失败：", err)
	}
}

func (a *App) saveConfigQuietly() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.persistConfigLocked()
}

func (a *App) PrintActiveDevices() error {
	devices, err := a.activeDevices()
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("未发现活跃设备。")
		return nil
	}
	fmt.Println("活跃设备:")
	for i := range devices {
		d := &devices[i]
		label := a.deviceDisplayName(d)
		fmt.Printf("  %d. %s | %s | %s\n", i+1, label, d.State, d.Serial)
	}
	return nil
}

func (a *App) PrintAllDevices() error {
	entries, err := a.allDeviceEntries()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("未发现设备或模拟器。")
		return nil
	}
	fmt.Println("全部设备:")
	for i, entry := range entries {
		fmt.Printf("  %d. %s\n", i+1, entry.Label)
	}
	return nil
}

func (a *App) readyDevices() ([]ActiveDevice, error) {
	devices, err := a.activeDevices()
	if err != nil {
		return nil, err
	}
	var ready []ActiveDevice
	for _, d := range devices {
		if d.State != "device" {
			continue
		}
		ready = append(ready, d)
	}
	return ready, nil
}

func deviceStateHelp(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "offline":
		return "设备处于离线状态。请重启 adb、重连 USB，或冷启动模拟器后重试。"
	case "unauthorized":
		return "设备未授权。请解锁手机，并在系统弹窗中允许 USB 调试。"
	case "":
		return "设备状态未知，只有 state=device 才能操作。"
	default:
		return fmt.Sprintf("设备状态为 %s，只有 state=device 才能操作。", state)
	}
}

func (a *App) currentReadyDevice() (*ActiveDevice, bool, error) {
	cur := a.currentDeviceSnapshot()
	if cur == nil {
		return nil, false, nil
	}
	ready, err := a.readyDevices()
	if err != nil {
		return nil, false, err
	}
	for i := range ready {
		if ready[i].Serial == cur.Serial {
			a.setCurrentDevice(&ready[i])
			return a.currentDeviceSnapshot(), true, nil
		}
	}
	a.clearCurrentDeviceIf(cur.Serial)
	return nil, false, nil
}

func (a *App) chooseReadyDevice(title string) (*ActiveDevice, bool, error) {
	if d, ok, err := a.currentReadyDevice(); err != nil || ok {
		return d, ok, err
	}
	return a.selectReadyDevice(title)
}

func (a *App) selectReadyDevice(title string) (*ActiveDevice, bool, error) {
	ready, err := a.readyDevices()
	if err != nil {
		return nil, false, err
	}
	if len(ready) == 0 {
		return nil, false, fmt.Errorf("没有 state=device 的可用设备")
	}
	var options []string
	for i := range ready {
		options = append(options, a.deviceLabel(&ready[i]))
	}
	idx, ok := a.selectOption(title, options)
	if !ok {
		return nil, false, nil
	}
	a.setCurrentDevice(&ready[idx])
	return a.currentDevice, true, nil
}

func (a *App) selectCurrentDeviceFlow() error {
	entry, selected, _, err := a.selectDeviceManagementTarget("选择当前设备", false)
	if err != nil || !selected {
		return err
	}
	entry, ok, err := a.ensureDeviceEntryReady(entry)
	if err != nil || !ok {
		return err
	}
	fmt.Println("当前设备已切换为：" + a.deviceLabel(entry.Active))
	return nil
}
