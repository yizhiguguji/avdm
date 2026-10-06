package app

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	adbKeyboardAPKURL           = "https://raw.githubusercontent.com/senzhk/ADBKeyBoard/master/ADBKeyboard.apk"
	defaultAVDLocale            = "zh-CN"
	defaultAVDDataPartitionSize = "32G"
	minInstallFreeSpaceBytes    = int64(1536 * 1024 * 1024)
	installFreeSpaceMultiplier  = int64(3)
)

type uiHierarchy struct {
	Nodes []uiNode `xml:"node"`
}

type uiNode struct {
	Text        string   `xml:"text,attr"`
	ResourceID  string   `xml:"resource-id,attr"`
	ContentDesc string   `xml:"content-desc,attr"`
	Bounds      string   `xml:"bounds,attr"`
	Nodes       []uiNode `xml:"node"`
}

type uiBounds struct {
	Left   int
	Top    int
	Right  int
	Bottom int
}

type emulatorWindowTarget struct {
	AVDName string
	Serial  string
}

func (a *App) startEmulatorFlow() error {
	entries, err := a.allDeviceEntries()
	if err != nil {
		return err
	}
	var avds []DeviceEntry
	var options []string
	for _, entry := range entries {
		if entry.AVD == nil {
			continue
		}
		avds = append(avds, entry)
		label := entry.Label
		if entry.Running {
			label += "（已启动）"
		}
		options = append(options, label)
	}
	if len(avds) == 0 {
		return fmt.Errorf("没有可启动的 AVD")
	}
	idx, ok := a.selectOption("选择要启动的模拟器", options)
	if !ok {
		return nil
	}
	selected := avds[idx]
	if selected.Running {
		fmt.Println("该模拟器已经启动。")
		return nil
	}
	return a.launchEmulator(selected.AVD.Name)
}

func (a *App) launchEmulator(avdName string) error {
	path, err := a.tools.Path(toolEmulator)
	if err != nil {
		return err
	}
	if err := a.prepareAVDForKeyboardForwarding(avdName); err != nil {
		return err
	}
	grpcPort := availableEmulatorGRPCPort(avdName)
	args := emulatorLaunchArgs(avdName, grpcPort)
	logFile, err := ensureLogFile("emulator_" + avdName)
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd := exec.Command(path, args...)
	configureDetachedProcess(cmd)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Printf("已发起启动：%s，日志：%s\n", avdName, logFile.Name())
	return nil
}

func emulatorLaunchArgs(avdName string, grpcPort int) []string {
	return []string{
		"-avd", avdName,
		"-use-keycode-forwarding",
		"-no-snapshot-load",
		"-no-snapshot-save",
		"-change-locale", defaultAVDLocale,
		"-grpc", fmt.Sprint(grpcPort),
		"-grpc-use-token",
		"-idle-grpc-timeout", "300",
	}
}

func focusEmulatorWindow(avdName, serial string) error {
	return focusEmulatorWindowNative(avdName, serial)
}

func focusProcessWindow(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("无效的进程 pid：%d", pid)
	}
	return focusProcessWindowNative(pid)
}

func resizeProcessWindow(pid int, width, height int) error {
	if pid <= 0 {
		return fmt.Errorf("无效的进程 pid：%d", pid)
	}
	if width <= 0 || height <= 0 {
		return fmt.Errorf("无效的窗口尺寸：%dx%d", width, height)
	}
	return resizeProcessWindowNative(pid, width, height)
}

func tileEmulatorWindows(targets []emulatorWindowTarget, columns int) error {
	if len(targets) == 0 {
		return fmt.Errorf("没有可平铺的模拟器窗口")
	}
	return tileEmulatorWindowsNative(targets, columns)
}

func tileProcessWindows(pids []int, columns int) error {
	if len(pids) == 0 {
		return fmt.Errorf("没有可平铺的外部窗口")
	}
	return tileProcessWindowsNative(pids, columns)
}

func autoTileColumns(count int) int {
	if count <= 1 {
		return 1
	}
	columns := 1
	for columns*columns < count {
		columns++
	}
	return columns
}

func sanitizeLaunchLabel(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "emulator"
	}
	return out
}

func (a *App) viewDevicesFlow() error {
	for {
		choice, ok := a.selectOption("查看设备", []string{
			"查看活跃设备",
			"查看全部设备（含未启动模拟器）",
		})
		if !ok {
			return nil
		}
		var err error
		switch choice {
		case 0:
			err = a.PrintActiveDevices()
		case 1:
			err = a.PrintAllDevices()
		}
		if err != nil {
			fmt.Println("错误：", err)
		}
	}
}

func (a *App) createEmulatorFlow() error {
	if _, err := a.tools.Path(toolAVDManager); err != nil {
		return err
	}
	if _, err := a.tools.Path(toolSDKManager); err != nil {
		return err
	}

	templates, err := a.deviceTemplates()
	if err != nil {
		return err
	}
	if len(templates) == 0 {
		templates = []DeviceTemplate{{ID: "pixel_9", Name: "Pixel 9"}, {ID: "pixel_9_pro", Name: "Pixel 9 Pro"}}
	}
	var templateOptions []string
	for _, t := range templates {
		templateOptions = append(templateOptions, fmt.Sprintf("%s (%s)", t.Name, t.ID))
	}
	templateIdx, ok := a.selectOption("选择设备模板", templateOptions)
	if !ok {
		return nil
	}
	template := templates[templateIdx]

	images, err := a.installedSystemImages()
	if err != nil {
		return err
	}
	recommended := "system-images;android-35;google_apis_playstore;arm64-v8a"
	if len(images) == 0 {
		fmt.Println("未发现已安装的 Android system image。")
		if a.confirm("安装推荐镜像 " + recommended + "？") {
			if err := a.runToolStreaming(toolSDKManager, recommended); err != nil {
				return err
			}
			images = append(images, recommended)
		} else {
			return nil
		}
	}
	imageOptions := append([]string{}, images...)
	if !containsString(imageOptions, recommended) {
		imageOptions = append(imageOptions, "安装推荐镜像："+recommended)
	}
	imageIdx, ok := a.selectOption("选择 system image", imageOptions)
	if !ok {
		return nil
	}
	image := imageOptions[imageIdx]
	if strings.HasPrefix(image, "安装推荐镜像：") {
		image = strings.TrimPrefix(image, "安装推荐镜像：")
		if err := a.runToolStreaming(toolSDKManager, image); err != nil {
			return err
		}
	}

	defaultName := "helper_" + strings.ReplaceAll(template.ID, " ", "_") + "_" + time.Now().Format("20060102_1504")
	name := a.prompt("模拟器名称（回车使用 " + defaultName + "）: ")
	if name == "" {
		name = defaultName
	}
	fmt.Printf("将创建 AVD：name=%s, device=%s, image=%s\n", name, template.ID, image)
	if !a.confirm("确认创建？") {
		return nil
	}
	createID := deviceTemplateCreateID(template)
	if err := a.runToolStreaming(toolAVDManager, "create", "avd", "-n", name, "-k", image, "-d", createID, "--force"); err != nil {
		return err
	}
	if err := a.prepareAVDForKeyboardForwarding(name); err != nil {
		fmt.Println("AVD 已创建，但物理键盘/快捷键转发配置写入失败：", err)
	} else {
		fmt.Println("已为新 AVD 写入物理键盘输入/快捷键转发配置。")
	}
	if !a.confirm("是否现在启动并设为当前设备？") {
		return nil
	}
	if err := a.launchEmulator(name); err != nil {
		return err
	}
	fmt.Println("正在等待模拟器进入可操作状态...")
	device, err := a.waitForAVDReady(name, 180*time.Second)
	if err != nil {
		return err
	}
	a.setCurrentDevice(device)
	fmt.Println("当前设备已切换为：" + a.currentDeviceSummary())
	return nil
}

func (a *App) closeDeviceFlow() error {
	devices, err := a.activeDevices()
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return fmt.Errorf("没有活跃设备")
	}
	var options []string
	for i := range devices {
		d := &devices[i]
		label := modelName(d)
		if d.AVDName != "" {
			label = d.AVDName
		}
		options = append(options, fmt.Sprintf("%s | %s | %s", label, d.State, d.Serial))
	}
	idx, ok := a.selectOption("选择要关闭/断开的设备", options)
	if !ok {
		return nil
	}
	d := devices[idx]
	return a.closeDevice(d)
}

func (a *App) closeDevice(d ActiveDevice) error {
	switch {
	case d.IsEmulator:
		if !a.confirm("将关闭模拟器 " + d.Serial + "。确认？") {
			return nil
		}
		_, err := a.runToolOutput(toolADB, 10*time.Second, "-s", d.Serial, "emu", "kill")
		if err == nil {
			a.clearCurrentDeviceIf(d.Serial)
			if waitErr := a.waitForSerialGone(d.Serial, 20*time.Second); waitErr != nil {
				fmt.Println("已发送关闭命令，但等待设备下线超时：", waitErr)
			}
		}
		return err
	case strings.Contains(d.Serial, ":"):
		if !a.confirm("将断开 TCP 设备 " + d.Serial + "。确认？") {
			return nil
		}
		_, err := a.runToolOutput(toolADB, 10*time.Second, "disconnect", d.Serial)
		if err == nil {
			a.clearCurrentDeviceIf(d.Serial)
		}
		return err
	default:
		fmt.Println("危险操作：这是 USB/真机设备，将执行设备关机。")
		fmt.Println("目标设备：" + a.deviceLabel(&d))
		if !a.confirmExact("确认对真机执行关机？", d.Serial) {
			return nil
		}
		_, err := a.runToolOutput(toolADB, 10*time.Second, "-s", d.Serial, "shell", "reboot", "-p")
		if err == nil {
			a.clearCurrentDeviceIf(d.Serial)
			if waitErr := a.waitForSerialGone(d.Serial, 30*time.Second); waitErr != nil {
				fmt.Println("已发送关机命令，但等待设备下线超时：", waitErr)
			}
		}
		return err
	}
}

func (a *App) deleteEmulatorFlow() error {
	entries, err := a.allDeviceEntries()
	if err != nil {
		return err
	}
	var avds []DeviceEntry
	var options []string
	for _, entry := range entries {
		if entry.AVD == nil {
			continue
		}
		avds = append(avds, entry)
		options = append(options, entry.Label)
	}
	if len(avds) == 0 {
		return fmt.Errorf("没有可删除的 AVD")
	}
	idx, ok := a.selectOption("选择要删除的模拟器", options)
	if !ok {
		return nil
	}
	selected := avds[idx]
	running, reason, err := a.isAVDProbablyRunning(selected.AVD.Name)
	if err != nil {
		return err
	}
	if selected.Running || running {
		if reason == "" {
			reason = "设备列表显示正在运行"
		}
		return fmt.Errorf("该 AVD 可能正在运行，请先关闭再删除：%s", reason)
	}
	fmt.Println("删除 AVD 会移除该模拟器配置和本地数据。")
	fmt.Println("AVD 名称：" + selected.AVD.Name)
	fmt.Println("AVD 路径：" + selected.AVD.Path)
	if !a.confirmExact("确认删除该 AVD？", selected.AVD.Name) {
		return nil
	}
	return a.runToolStreaming(toolAVDManager, "delete", "avd", "-n", selected.AVD.Name)
}

func (a *App) prepareAVDForKeyboardForwarding(avdName string) error {
	avds, err := a.avds()
	if err != nil {
		return err
	}
	for _, avd := range avds {
		if avd.Name != avdName {
			continue
		}
		if avd.Path == "" {
			return fmt.Errorf("AVD %s 没有可写入的配置路径", avdName)
		}
		if _, err := setAVDHardwareKeyboardFiles(avd.Path); err != nil {
			return err
		}
		if err := enableEmulatorKeyboardForwardingPreference(); err != nil {
			fmt.Println("模拟器快捷键转发设置写入失败：", err)
		}
		return nil
	}
	return fmt.Errorf("未找到 AVD：%s", avdName)
}

func (a *App) prepareAVDDataPartition(avdName string) error {
	avds, err := a.avds()
	if err != nil {
		return err
	}
	for _, avd := range avds {
		if avd.Name != avdName {
			continue
		}
		if avd.Path == "" {
			return fmt.Errorf("AVD %s 没有可写入的配置路径", avdName)
		}
		return setAVDConfigValue(filepath.Join(avd.Path, "config.ini"), "disk.dataPartition.size", defaultAVDDataPartitionSize)
	}
	return fmt.Errorf("未找到 AVD：%s", avdName)
}

func (a *App) isAVDProbablyRunning(avdName string) (bool, string, error) {
	devices, err := a.activeDevices()
	if err != nil {
		return false, "", err
	}
	for _, device := range devices {
		if device.AVDName == avdName {
			return true, "设备列表中存在 " + device.Serial, nil
		}
	}
	if runtime.GOOS == "windows" {
		return false, "", nil
	}
	out, err := exec.Command("ps", "-axo", "args").Output()
	if err != nil {
		return false, "", fmt.Errorf("无法确认 AVD 运行态：%w", err)
	}
	for _, raw := range strings.Split(string(out), "\n") {
		fields := strings.Fields(raw)
		for i, field := range fields {
			if field == "-avd" && i+1 < len(fields) && fields[i+1] == avdName {
				return true, "发现 emulator 进程 -avd " + avdName, nil
			}
			if strings.HasSuffix(field, "@"+avdName) || field == "@"+avdName {
				return true, "发现 emulator 进程 @" + avdName, nil
			}
		}
	}
	return false, "", nil
}

func (a *App) installAPKFlow() error {
	device, ok, err := a.chooseReadyDeviceOrStartAVD("选择安装目标设备")
	if err != nil || !ok {
		return err
	}
	prompt := "APK 文件路径或 URL（可拖入文件）: "
	if a.cfg.LastAPKSource != "" {
		prompt = "APK 文件路径或 URL（回车使用上次：" + a.cfg.LastAPKSource + "）: "
	}
	apkSource := normalizeAPKSource(a.prompt(prompt))
	if apkSource == "" && a.cfg.LastAPKSource != "" {
		apkSource = a.cfg.LastAPKSource
	}
	if apkSource == "" {
		return nil
	}
	return a.installAPKSourceToDevice(device, apkSource)
}

func (a *App) reinstallLastAPKFlow() error {
	if a.cfg.LastAPKSource == "" {
		fmt.Println("还没有上次 APK 记录。")
		return nil
	}
	device, ok, err := a.chooseReadyDeviceOrStartAVD("选择安装目标设备")
	if err != nil || !ok {
		return err
	}
	fmt.Println("将重新安装上次 APK：", a.cfg.LastAPKSource)
	return a.installAPKSourceToDevice(device, a.cfg.LastAPKSource)
}

func (a *App) installAPKSourceToDevice(device *ActiveDevice, apkSource string) error {
	apkPath := apkSource
	cleanup := func() {}
	sourceIsURL := isHTTPURL(apkSource)
	if sourceIsURL {
		fmt.Println("正在下载 APK：", apkSource)
		downloadedPath, downloadedCleanup, err := downloadAPK(apkSource)
		if err != nil {
			return err
		}
		apkPath = downloadedPath
		cleanup = downloadedCleanup
		defer cleanup()
		fmt.Println("下载完成：", apkPath)
	} else {
		apkPath = normalizeAPKPath(apkSource)
		if !fileExists(apkPath) {
			return fmt.Errorf("APK 文件不存在：%s", apkPath)
		}
	}
	if !sourceIsURL && strings.ToLower(filepath.Ext(apkPath)) != ".apk" && !a.confirm("文件扩展名不是 .apk，仍继续？") {
		return nil
	}
	fmt.Printf("将安装到设备：%s\n", a.deviceLabel(device))
	if !a.confirm("确认安装？") {
		return nil
	}
	if err := a.smartInstallAPK(device.Serial, apkPath); err != nil {
		return err
	}
	a.cfg.LastAPKSource = apkSource
	a.saveConfigQuietly()
	return nil
}

func (a *App) chooseReadyDeviceOrStartAVD(title string) (*ActiveDevice, bool, error) {
	if d, ok, err := a.currentReadyDevice(); err != nil || ok {
		return d, ok, err
	}
	entry, selected, _, err := a.selectDeviceManagementTarget(title, false)
	if err != nil || !selected {
		return nil, selected, err
	}
	entry, ok, err := a.ensureDeviceEntryReady(entry)
	if err != nil || !ok {
		return nil, ok, err
	}
	return entry.Active, true, nil
}

type installMode struct {
	AllowDowngrade bool
	AllowTestOnly  bool
	// AllowReinstall permits uninstalling after a signature conflict or blocked
	// downgrade. It wipes app data and requires explicit user confirmation.
	AllowReinstall bool
}

// ReinstallRequiredError asks the caller to confirm data loss before resolving
// a signature conflict or a downgrade that cannot be installed in place.
type ReinstallRequiredError struct {
	Reason  string   // why replacement requires uninstalling
	Package string   // package name parsed from the APK ("" if unknown)
	Serials []string // devices that hit the block
	Detail  string   // additional context (e.g. other devices' failures)
}

func (e *ReinstallRequiredError) Error() string {
	pkg := e.Package
	if pkg == "" {
		pkg = "该应用"
	}
	reason := e.Reason
	if reason == "" {
		reason = "正式包无法直接降级"
	}
	msg := fmt.Sprintf("%s：需先卸载 %s 再重新安装（会清空应用数据）", reason, pkg)
	if len(e.Serials) > 0 {
		msg += "；受影响设备：" + strings.Join(e.Serials, "、")
	}
	if e.Detail != "" {
		msg += "\n" + e.Detail
	}
	return msg
}

// AsReinstallRequired reports whether err (or something it wraps) is a
// ReinstallRequiredError.
func AsReinstallRequired(err error) (*ReinstallRequiredError, bool) {
	var target *ReinstallRequiredError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

func (a *App) smartInstallAPK(serial, apkPath string) error {
	mode := installMode{}
	confirmedDowngrade := false

	if err := a.ensureAPKInstallSpace(serial, apkPath); err != nil {
		return err
	}

	for attempt := 1; attempt <= 4; attempt++ {
		args := installArgs(serial, apkPath, mode)
		fmt.Printf("执行安装（第 %d 次）：adb %s\n", attempt, strings.Join(args, " "))
		out, err := a.runToolOutput(toolADB, 10*time.Minute, args...)
		if out != "" {
			fmt.Println(out)
		}
		if err == nil {
			fmt.Println("安装完成。")
			return nil
		}

		hint := analyzeInstallFailure(out + "\n" + err.Error())
		changed := false

		if hint.NeedsSignatureReinstall {
			pkg, pkgErr := apkPackageName(apkPath)
			if pkgErr != nil {
				return fmt.Errorf("签名冲突，无法解析 APK 包名：%w", pkgErr)
			}
			fmt.Printf("新旧 APK 签名不一致。卸载重装 %s 会清空全部应用数据。\n", pkg)
			if !a.confirm("是否卸载重装？选择否将放弃安装") {
				return fmt.Errorf("已放弃安装")
			}
			return a.reinstallAfterUninstall(serial, pkg, apkPath, mode)
		}

		if hint.NeedsDowngrade && !mode.AllowDowngrade {
			if !confirmedDowngrade {
				fmt.Println("检测到这是降级安装。")
				if !a.confirm("降级安装可能覆盖较新版本，确认继续？") {
					return nil
				}
				confirmedDowngrade = true
			}
			mode.AllowDowngrade = true
			changed = true
		} else if hint.NeedsDowngrade && mode.AllowDowngrade && !mode.AllowReinstall {
			// -d did not help: release build. Offer uninstall + fresh install.
			pkg, pkgErr := apkPackageName(apkPath)
			if pkgErr != nil {
				return fmt.Errorf("正式包无法直接降级，需卸载重装，但解析 APK 包名失败：%w", pkgErr)
			}
			fmt.Printf("正式包无法直接降级（-d 无效）。卸载重装 %s 会清空其应用数据。\n", pkg)
			if !a.confirm("确认卸载旧版本后重新安装？") {
				return nil
			}
			return a.reinstallAfterUninstall(serial, pkg, apkPath, mode)
		}

		if hint.NeedsTestOnly && !mode.AllowTestOnly {
			fmt.Println("检测到 APK 标记为 testOnly，自动追加 -t 重试。")
			mode.AllowTestOnly = true
			changed = true
		}

		if !changed {
			explanation := explainInstallFailure(out + "\n" + err.Error())
			if explanation != "" {
				return fmt.Errorf("安装失败：%s\n%s", err, explanation)
			}
			return fmt.Errorf("安装失败：%s", err)
		}
	}
	return fmt.Errorf("安装失败：智能重试次数已用完")
}

func uninstallArgs(serial, pkg string, keepData, user0 bool) []string {
	args := []string{"-s", serial}
	if keepData || user0 {
		args = append(args, "shell", "pm", "uninstall")
	} else {
		args = append(args, "uninstall")
	}
	if keepData {
		args = append(args, "-k")
	}
	if user0 {
		args = append(args, "--user", "0")
	}
	return append(args, pkg)
}

func installArgs(serial, apkPath string, mode installMode) []string {
	args := []string{"-s", serial, "install", "-r"}
	if mode.AllowDowngrade {
		args = append(args, "-d")
	}
	if mode.AllowTestOnly {
		args = append(args, "-t")
	}
	return append(args, apkPath)
}

type installFailureHint struct {
	NeedsSignatureReinstall bool
	NeedsDowngrade          bool
	NeedsTestOnly           bool
}

func analyzeInstallFailure(output string) installFailureHint {
	upper := strings.ToUpper(output)
	return installFailureHint{
		NeedsSignatureReinstall: strings.Contains(upper, "INSTALL_FAILED_UPDATE_INCOMPATIBLE"),
		NeedsDowngrade: strings.Contains(upper, "INSTALL_FAILED_VERSION_DOWNGRADE") ||
			strings.Contains(upper, "VERSION_DOWNGRADE"),
		NeedsTestOnly: strings.Contains(upper, "INSTALL_FAILED_TEST_ONLY") ||
			strings.Contains(upper, "TEST_ONLY") ||
			strings.Contains(upper, "TESTONLY"),
	}
}

func explainInstallFailure(output string) string {
	upper := strings.ToUpper(output)
	switch {
	case strings.Contains(upper, "INSTALL_FAILED_UPDATE_INCOMPATIBLE") ||
		strings.Contains(upper, "INSTALL_FAILED_SHARED_USER_INCOMPATIBLE"):
		return "原因：设备上已有同包名但签名不一致的应用。\n建议：确认是否要先卸载旧应用，或安装同签名构建。"
	case strings.Contains(upper, "INSTALL_FAILED_INSUFFICIENT_STORAGE"):
		return "原因：设备存储空间不足。\n建议：清理设备空间或卸载不需要的应用后重试。"
	case strings.Contains(upper, "INSTALL_FAILED_OLDER_SDK") ||
		strings.Contains(upper, "INSTALL_FAILED_NEWER_SDK"):
		return "原因：APK 要求的 Android SDK 版本与当前设备不匹配。\n建议：换匹配系统版本的模拟器或使用兼容构建。"
	case strings.Contains(upper, "INSTALL_FAILED_NO_MATCHING_ABIS"):
		return "原因：APK 的 CPU 架构与设备不匹配。\n建议：确认 APK 是否包含当前设备 ABI，或换 x86_64/arm64 对应模拟器。"
	case strings.Contains(upper, "INSTALL_PARSE_FAILED") ||
		strings.Contains(upper, "INVALID APK"):
		return "原因：APK 文件无效或下载不完整。\n建议：重新下载 APK，确认文件来自完整构建产物。"
	case strings.Contains(upper, "DEVICE UNAUTHORIZED") ||
		strings.Contains(upper, "UNAUTHORIZED"):
		return "原因：设备未授权 USB 调试。\n建议：解锁手机，在系统弹窗中允许 USB 调试。"
	case strings.Contains(upper, "DEVICE OFFLINE") ||
		strings.Contains(upper, "OFFLINE"):
		return "原因：设备处于 offline 状态。\n建议：重启 adb、重连 USB，或冷启动模拟器后重试。"
	default:
		return ""
	}
}

func (a *App) uninstallAPKFlow() error {
	device, ok, err := a.chooseReadyDevice("选择卸载目标设备")
	if err != nil || !ok {
		return err
	}
	sourceIdx, ok := a.selectOption("选择包列表范围", []string{
		"第三方应用",
		"全部应用",
	})
	if !ok {
		return nil
	}
	args := []string{"-s", device.Serial, "shell", "pm", "list", "packages"}
	if sourceIdx == 0 {
		args = append(args, "-3")
	}
	out, err := a.runToolOutput(toolADB, 30*time.Second, args...)
	if err != nil {
		return err
	}
	packages := packageLinesToNames(out)
	keyword := strings.ToLower(a.prompt("包名过滤关键词（回车不过滤）: "))
	var filtered []string
	for _, pkg := range packages {
		if keyword == "" || strings.Contains(strings.ToLower(pkg), keyword) {
			filtered = append(filtered, pkg)
		}
	}
	if len(filtered) == 0 {
		return fmt.Errorf("没有匹配包名")
	}
	if a.cfg.LastPackageName != "" && containsString(filtered, a.cfg.LastPackageName) {
		filtered = append([]string{a.cfg.LastPackageName + "（最近使用）"}, removeString(filtered, a.cfg.LastPackageName)...)
	}
	idx, ok := a.selectOption("选择要卸载的包", filtered)
	if !ok {
		return nil
	}
	pkg := strings.TrimSuffix(filtered[idx], "（最近使用）")
	mode, ok := a.selectOption("选择卸载模式", []string{
		"完全卸载",
		"卸载但保留数据 (-k)",
		"仅当前用户卸载 (--user 0)",
	})
	if !ok {
		return nil
	}
	fmt.Printf("将从 %s 卸载：%s\n", a.deviceLabel(device), pkg)
	if !a.confirm("确认卸载？") {
		return nil
	}
	err = a.runToolStreaming(toolADB, uninstallArgs(device.Serial, pkg, mode == 1, mode == 2)...)
	if err == nil {
		a.cfg.LastPackageName = pkg
		a.saveConfigQuietly()
	}
	return err
}

func (a *App) deviceManagementFlow() error {
	for {
		if entry, ok, err := a.currentDeviceEntry(); err != nil {
			return err
		} else if ok {
			switchTarget, err := a.manageReadyDeviceFlow(entry)
			if err != nil {
				fmt.Println("错误：", err)
			}
			if !switchTarget {
				return nil
			}
		}

		entry, selected, action, err := a.selectDeviceManagementTarget("选择要管理的设备/模拟器", true)
		if err != nil {
			return err
		}
		if !selected {
			return nil
		}
		switch action {
		case "create":
			err = a.createEmulatorFlow()
			if err != nil {
				fmt.Println("错误：", err)
			}
			continue
		case "delete":
			err = a.deleteEmulatorFlow()
			if err != nil {
				fmt.Println("错误：", err)
			}
			continue
		}
		entry, ok, err := a.ensureDeviceEntryReady(entry)
		if err != nil {
			fmt.Println("错误：", err)
			continue
		}
		if !ok {
			continue
		}
		switchTarget, err := a.manageReadyDeviceFlow(entry)
		if err != nil {
			fmt.Println("错误：", err)
		}
		if !switchTarget {
			return nil
		}
	}
}

func (a *App) currentDeviceConsoleFlow() error {
	entry, ok, err := a.currentDeviceEntry()
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("当前设备不可用，请先选择或启动设备。")
		return a.selectCurrentDeviceFlow()
	}
	_, err = a.manageReadyDeviceFlow(entry)
	return err
}

func (a *App) currentDeviceEntry() (DeviceEntry, bool, error) {
	device, ok, err := a.currentReadyDevice()
	if err != nil || !ok {
		return DeviceEntry{}, ok, err
	}
	entries, err := a.allDeviceEntries()
	if err != nil {
		return DeviceEntry{}, false, err
	}
	for _, entry := range entries {
		if entry.Active != nil && entry.Active.Serial == device.Serial {
			return entry, true, nil
		}
	}
	kind := "真机"
	if device.IsEmulator {
		kind = "模拟器"
	}
	entry := DeviceEntry{
		Kind:    kind,
		Key:     a.deviceAliasKey(device),
		Active:  device,
		Running: true,
	}
	entry.Label = a.formatDeviceEntry(entry)
	return entry, true, nil
}

func (a *App) selectDeviceManagementTarget(title string, includeLifecycle bool) (DeviceEntry, bool, string, error) {
	entries, err := a.allDeviceEntries()
	if err != nil {
		return DeviceEntry{}, false, "", err
	}
	var options []string
	for _, entry := range entries {
		options = append(options, entry.Label)
	}
	if includeLifecycle {
		options = append(options, "创建模拟器（新 AVD）")
		options = append(options, "删除模拟器（AVD）")
	}
	if len(options) == 0 {
		return DeviceEntry{}, false, "", fmt.Errorf("未发现设备或模拟器")
	}
	idx, ok := a.selectOption(title, options)
	if !ok {
		return DeviceEntry{}, false, "", nil
	}
	if includeLifecycle && idx == len(entries) {
		return DeviceEntry{}, true, "create", nil
	}
	if includeLifecycle && idx == len(entries)+1 {
		return DeviceEntry{}, true, "delete", nil
	}
	return entries[idx], true, "", nil
}

func (a *App) ensureDeviceEntryReady(entry DeviceEntry) (DeviceEntry, bool, error) {
	if entry.Active != nil {
		if entry.Active.State != "device" {
			return DeviceEntry{}, false, fmt.Errorf("%s", deviceStateHelp(entry.Active.State))
		}
		a.setCurrentDevice(entry.Active)
		return entry, true, nil
	}
	if entry.AVD == nil {
		return DeviceEntry{}, false, fmt.Errorf("该条目没有可操作的设备")
	}
	fmt.Printf("模拟器 %s 当前未启动。\n", entry.AVD.Name)
	if !a.confirm("是否启动该模拟器？") {
		return DeviceEntry{}, false, nil
	}
	if err := a.launchEmulator(entry.AVD.Name); err != nil {
		return DeviceEntry{}, false, err
	}
	fmt.Println("正在等待模拟器进入可操作状态...")
	device, err := a.waitForAVDReady(entry.AVD.Name, 180*time.Second)
	if err != nil {
		return DeviceEntry{}, false, err
	}
	entry.Active = device
	entry.Running = true
	entry.Label = a.formatDeviceEntry(entry)
	a.setCurrentDevice(device)
	return entry, true, nil
}

func (a *App) manageReadyDeviceFlow(entry DeviceEntry) (bool, error) {
	for {
		if a.currentDevice == nil {
			return false, nil
		}
		fmt.Println("当前管理对象：" + a.currentDeviceSummary())
		actions := []struct {
			label        string
			run          func() error
			returnToList bool
		}{
			{label: "安装应用(APK)", run: a.installAPKFlow},
			{label: "卸载应用", run: a.uninstallAPKFlow},
			{label: "语言管理", run: a.languageManagementFlow},
			{label: "列出输入法", run: a.listIMEFlow},
			{label: "启用输入法", run: a.enableIMEFlow},
			{label: "设置默认输入法", run: a.setIMEFlow},
			{label: "开启硬件键盘时显示软键盘", run: a.showIMEWithHardKeyboardFlow},
			{label: "输入文本", run: a.textInputFlow},
			{label: "重启当前设备", run: a.rebootCurrentDeviceFlow},
			{label: "关闭/断开当前设备", run: a.closeCurrentDeviceFlow},
		}
		if entry.AVD != nil || (entry.Active != nil && entry.Active.IsEmulator) {
			actions = append(actions, struct {
				label        string
				run          func() error
				returnToList bool
			}{label: "启用模拟器物理键盘输入/快捷键转发", run: a.enableAVDHardwareKeyboardFlow})
		}
		actions = append(actions,
			struct {
				label        string
				run          func() error
				returnToList bool
			}{label: "别名管理", run: a.aliasManagementFlow},
			struct {
				label        string
				run          func() error
				returnToList bool
			}{label: "切换管理对象", returnToList: true},
		)
		var options []string
		for _, action := range actions {
			options = append(options, action.label)
		}
		choice, ok := a.selectOption("设备操作", options)
		if !ok || actions[choice].returnToList {
			return ok, nil
		}
		if err := actions[choice].run(); err != nil {
			fmt.Println("错误：", err)
			continue
		}
		if a.currentDevice == nil {
			return false, nil
		}
	}
}

func (a *App) languageManagementFlow() error {
	for {
		choice, ok := a.selectOption("语言管理", []string{
			"打开系统语言设置页",
			"尝试写入系统语言配置（高级/多数系统不会立即生效）",
		})
		if !ok {
			return nil
		}
		var err error
		switch choice {
		case 0:
			err = a.openSystemLocaleSettingsFlow()
		case 1:
			err = a.attemptSystemLanguageFlow()
		}
		if err != nil {
			fmt.Println("错误：", err)
		}
	}
}

func (a *App) chooseLocale() (string, bool) {
	idx, ok := a.selectOption("选择语言", []string{"简体中文 zh-CN", "英文 en-US", "日文 ja-JP", "自定义"})
	if !ok {
		return "", false
	}
	locale := "zh-CN"
	switch idx {
	case 1:
		locale = "en-US"
	case 2:
		locale = "ja-JP"
	case 3:
		locale = a.prompt("输入 locale，例如 zh-CN: ")
	}
	return strings.TrimSpace(locale), strings.TrimSpace(locale) != ""
}

func (a *App) openSystemLocaleSettingsFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择设备")
	if err != nil || !ok {
		return err
	}
	_, err = a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "shell", "am", "start", "-a", "android.settings.LOCALE_SETTINGS")
	return err
}

func (a *App) rebootCurrentDeviceFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择要重启的设备")
	if err != nil || !ok {
		return err
	}
	before := *device
	if !a.confirm("确认重启当前设备 " + a.deviceLabel(&before) + "？") {
		return nil
	}
	_, err = a.runToolOutput(toolADB, 10*time.Second, "-s", before.Serial, "reboot")
	if err == nil {
		a.clearCurrentDeviceIf(before.Serial)
		if before.AVDName != "" {
			fmt.Println("已发送重启命令，正在等待模拟器恢复可操作状态...")
			_ = a.waitForSerialGone(before.Serial, 30*time.Second)
			if restored, waitErr := a.waitForAVDReady(before.AVDName, 180*time.Second); waitErr == nil {
				a.setCurrentDevice(restored)
				fmt.Println("设备已恢复：" + a.currentDeviceSummary())
			} else {
				fmt.Println("设备重启命令已发送，但等待恢复超时：", waitErr)
			}
			return nil
		}
		fmt.Println("已发送重启命令。设备重连后会按上次设备自动恢复。")
	}
	return err
}

func (a *App) closeCurrentDeviceFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择要关闭/断开的设备")
	if err != nil || !ok {
		return err
	}
	return a.closeDevice(*device)
}

func (a *App) attemptSystemLanguageFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择设备")
	if err != nil || !ok {
		return err
	}
	locale, ok := a.chooseLocale()
	if !ok {
		return nil
	}
	fmt.Println("该方式只是写入 system_locales，很多 Android 版本不会立即应用系统语言。")
	if !a.confirm("确认尝试写入系统语言配置为 " + locale + "？") {
		return nil
	}
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "shell", "settings", "put", "system", "system_locales", locale); err != nil {
		return err
	}
	if stored, err := a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "shell", "settings", "get", "system", "system_locales"); err == nil {
		fmt.Println("settings 中的 system_locales：", strings.TrimSpace(stored))
	}
	config, err := a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "shell", "cmd", "activity", "get-config")
	if err != nil {
		fmt.Println("语言已写入 settings，但读取运行时配置失败：", err)
		fmt.Println("如界面未切换语言，请手动重启设备或模拟器。")
		return nil
	}
	if localeMatchesConfig(config, locale) {
		fmt.Println("语言设置已生效。")
		return nil
	}

	fmt.Println("语言已写入 settings，但当前系统运行时配置尚未切换。")
	fmt.Println("当前 Android 版本不允许 shell 无权限刷新系统语言；请使用“打开系统语言设置页”手动切换。")
	return nil
}

func (a *App) listIMEFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择设备")
	if err != nil || !ok {
		return err
	}
	out, err := a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "shell", "ime", "list", "-a")
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func (a *App) enableIMEFlow() error {
	return a.imeActionFlow("启用输入法", "enable")
}

func (a *App) setIMEFlow() error {
	return a.imeActionFlow("设置默认输入法", "set")
}

func (a *App) imeActionFlow(title, action string) error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择设备")
	if err != nil || !ok {
		return err
	}
	out, err := a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "shell", "ime", "list", "-a")
	if err != nil {
		return err
	}
	imes := parseIMEIDs(out)
	if len(imes) == 0 {
		return fmt.Errorf("没有解析到输入法")
	}
	idx, ok := a.selectOption(title, imes)
	if !ok {
		return nil
	}
	_, err = a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "shell", "ime", action, imes[idx])
	return err
}

func parseIMEIDs(out string) []string {
	var ids []string
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "mId=") {
			ids = append(ids, strings.TrimSpace(strings.TrimPrefix(line, "mId=")))
		}
	}
	return uniqueStrings(ids)
}

func (a *App) showIMEWithHardKeyboardFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择设备")
	if err != nil || !ok {
		return err
	}
	idx, ok := a.selectOption("硬件键盘时显示软键盘", []string{"开启", "关闭"})
	if !ok {
		return nil
	}
	value := "1"
	if idx == 1 {
		value = "0"
	}
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "shell", "settings", "put", "secure", "show_ime_with_hard_keyboard", value); err != nil {
		return err
	}
	stored, err := a.runToolOutput(toolADB, 10*time.Second, "-s", device.Serial, "shell", "settings", "get", "secure", "show_ime_with_hard_keyboard")
	if err != nil {
		return err
	}
	if strings.TrimSpace(stored) == value {
		fmt.Println("设置已写入并读回确认。")
	} else {
		fmt.Printf("设置已写入，但读回值为 %q，可能未被系统接受。\n", strings.TrimSpace(stored))
	}
	return nil
}

func (a *App) enableAVDHardwareKeyboardFlow() error {
	selected, ok, err := a.chooseAVDForCurrentContext("选择要启用物理键盘输入的模拟器")
	if err != nil || !ok {
		return err
	}
	return a.enableAVDHardwareKeyboard(selected)
}

func (a *App) chooseAVDForCurrentContext(title string) (DeviceEntry, bool, error) {
	entries, err := a.allDeviceEntries()
	if err != nil {
		return DeviceEntry{}, false, err
	}
	if a.currentDevice != nil {
		for _, entry := range entries {
			if entry.AVD == nil || entry.Active == nil {
				continue
			}
			if entry.Active.Serial == a.currentDevice.Serial {
				return entry, true, nil
			}
		}
		if a.currentDevice.IsEmulator {
			fmt.Println("当前设备是模拟器，但没有匹配到 AVD 配置，请从列表选择。")
		}
	}

	var avds []DeviceEntry
	var options []string
	for _, entry := range entries {
		if entry.AVD == nil {
			continue
		}
		avds = append(avds, entry)
		options = append(options, entry.Label)
	}
	if len(avds) == 0 {
		return DeviceEntry{}, false, fmt.Errorf("没有可配置的 AVD")
	}
	idx, ok := a.selectOption(title, options)
	if !ok {
		return DeviceEntry{}, false, nil
	}
	return avds[idx], true, nil
}

func (a *App) enableAVDHardwareKeyboard(selected DeviceEntry) error {
	configPath := filepath.Join(selected.AVD.Path, "config.ini")
	if !fileExists(configPath) {
		return fmt.Errorf("AVD 配置文件不存在：%s", configPath)
	}
	fmt.Println("将修改 AVD 配置：", selected.AVD.Path)
	if !a.confirm("确认启用 AVD 物理键盘输入？") {
		return nil
	}
	updated, err := setAVDHardwareKeyboardFiles(selected.AVD.Path)
	if err != nil {
		return err
	}
	for _, path := range updated {
		fmt.Println("已写入：", path)
	}
	fmt.Println("已写入：hw.keyboard=yes, hw.keyboard.charmap=qwerty2, hw.keyboard.lid=yes。")
	if err := enableEmulatorKeyboardForwardingPreference(); err != nil {
		fmt.Println("模拟器快捷键转发设置写入失败：", err)
	} else {
		fmt.Println("已写入 Emulator 设置：Send keyboard shortcuts to Virtual device，Enforce keycode forwarding。")
	}

	if selected.Running {
		fmt.Println("当前模拟器进程不会热加载该配置，需要关闭并重新启动 emulator 进程才会生效。")
		if selected.Active == nil {
			fmt.Println("无法定位运行中的 emulator serial，请手动关闭并重新启动该 AVD。")
			return nil
		}
		if !a.confirm("是否现在重启模拟器进程 " + selected.AVD.Name + "？") {
			fmt.Println("已写入配置，但当前运行实例不会立即生效。")
			return nil
		}
		if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", selected.Active.Serial, "emu", "kill"); err != nil {
			return err
		}
		a.clearCurrentDeviceIf(selected.Active.Serial)
		if err := a.waitForSerialGone(selected.Active.Serial, 20*time.Second); err != nil {
			return err
		}
		if err := a.launchEmulator(selected.AVD.Name); err != nil {
			return err
		}
		fmt.Println("已重启模拟器进程，正在等待设备恢复...")
		device, err := a.waitForAVDReady(selected.AVD.Name, 180*time.Second)
		if err != nil {
			return err
		}
		a.setCurrentDevice(device)
		if err := a.verifyHardwareKeyboardForwarding(device.Serial); err != nil {
			fmt.Println("设备已恢复，但硬件键盘验证未通过：", err)
		} else {
			fmt.Println("硬件键盘已被 Android 识别。")
		}
		return nil
	}

	fmt.Println("已启用模拟器物理键盘输入/快捷键转发。下次启动该 AVD 时生效。")
	return nil
}

func enableEmulatorKeyboardForwardingPreference() error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	if err := exec.Command("defaults", "write", "com.android.Emulator", "set.forwardShortcutsToDevice", "-bool", "true").Run(); err != nil {
		return err
	}
	return exec.Command("defaults", "write", "com.android.Emulator", "perAvd.set.enforceKeycodeForwarding", "-bool", "true").Run()
}

func setAVDHardwareKeyboardFiles(avdPath string) ([]string, error) {
	var targets []string
	for _, path := range []string{
		filepath.Join(avdPath, "config.ini"),
		filepath.Join(avdPath, "hardware-qemu.ini"),
	} {
		if fileExists(path) {
			targets = append(targets, path)
		}
	}
	snapshotDir := filepath.Join(avdPath, "snapshots")
	if info, err := os.Stat(snapshotDir); err == nil && info.IsDir() {
		err := filepath.WalkDir(snapshotDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Base(path) != "hardware.ini" {
				return nil
			}
			targets = append(targets, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("没有找到可写入的 AVD 硬件配置文件")
	}
	var updated []string
	originals := map[string][]byte{}
	stamp := time.Now().Format("20060102_150405")
	for _, path := range uniqueStrings(targets) {
		data, err := os.ReadFile(path)
		if err != nil {
			restoreFiles(originals)
			return updated, err
		}
		next := updateAVDHardwareKeyboardConfig(string(data))
		if next == string(data) {
			continue
		}
		originals[path] = data
		backupPath := path + ".adm.bak." + stamp
		if err := os.WriteFile(backupPath, data, 0o644); err != nil {
			restoreFiles(originals)
			return updated, fmt.Errorf("写入备份失败 %s：%w", backupPath, err)
		}
		if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
			restoreFiles(originals)
			return updated, err
		}
		updated = append(updated, path)
	}
	return updated, nil
}

func restoreFiles(originals map[string][]byte) {
	for path, data := range originals {
		_ = os.WriteFile(path, data, 0o644)
	}
}

func (a *App) verifyHardwareKeyboardForwarding(serial string) error {
	out, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "dumpsys", "input")
	if err != nil {
		return err
	}
	if strings.Contains(out, "KeyboardType: 2") && strings.Contains(out, "KEYBOARD") {
		return nil
	}
	return fmt.Errorf("未在 dumpsys input 中看到硬件键盘 KeyboardType: 2")
}

func (a *App) prepareReadyAVDDefaults(serial string) error {
	if strings.TrimSpace(serial) == "" {
		return fmt.Errorf("设备 serial 不能为空")
	}
	if err := a.waitForSettingsService(serial, 90*time.Second); err != nil {
		return err
	}
	if err := a.prepareReadyAVDInput(serial); err != nil {
		return err
	}
	if err := a.prepareReadyAVDLocale(serial, defaultAVDLocale); err != nil {
		return err
	}
	return nil
}

func (a *App) prepareReadyAVDInput(serial string) error {
	deadline := time.Now().Add(90 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := a.applyReadyAVDInputOnce(serial); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(2 * time.Second)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("等待输入设置服务超时")
	}
	return fmt.Errorf("启用硬件键盘和软键盘显示失败：%w", lastErr)
}

func (a *App) applyReadyAVDInputOnce(serial string) error {
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "settings", "put", "secure", "show_ime_with_hard_keyboard", "1"); err != nil {
		return fmt.Errorf("启用硬件键盘时显示软键盘失败：%w", err)
	}
	stored, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "settings", "get", "secure", "show_ime_with_hard_keyboard")
	if err != nil {
		return fmt.Errorf("读取软键盘显示设置失败：%w", err)
	}
	if strings.TrimSpace(stored) != "1" {
		return fmt.Errorf("软键盘显示设置未生效，读回值：%s", strings.TrimSpace(stored))
	}
	if err := a.verifyHardwareKeyboardForwarding(serial); err != nil {
		return err
	}
	return nil
}

func (a *App) prepareReadyAVDLocale(serial, locale string) error {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return fmt.Errorf("系统语言不能为空")
	}
	if a.runtimeLocaleMatches(serial, locale) {
		return nil
	}
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "settings", "put", "system", "system_locales", locale); err != nil {
		return fmt.Errorf("设置系统语言为中文失败：%w", err)
	}
	stored, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "settings", "get", "system", "system_locales")
	if err != nil {
		return fmt.Errorf("读取系统语言设置失败：%w", err)
	}
	if !localeSettingContains(stored, locale) {
		return fmt.Errorf("系统语言设置未生效，读回值：%s", strings.TrimSpace(stored))
	}
	if a.runtimeLocaleMatches(serial, locale) {
		return nil
	}
	if err := a.applyLocaleViaSettingsUI(serial, locale); err != nil {
		return fmt.Errorf("通过系统设置切换中文失败：%w", err)
	}
	if !a.runtimeLocaleMatches(serial, locale) {
		config, _ := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "cmd", "activity", "get-config")
		return fmt.Errorf("系统语言切换后仍未生效，运行时配置：%s", strings.TrimSpace(config))
	}
	_, _ = a.runToolOutput(toolADB, 5*time.Second, "-s", serial, "shell", "input", "keyevent", "HOME")
	return nil
}

func localeSettingContains(stored, locale string) bool {
	stored = strings.TrimSpace(stored)
	locale = strings.TrimSpace(locale)
	if stored == "" || locale == "" {
		return false
	}
	normalize := func(value string) string {
		value = strings.ToLower(strings.TrimSpace(value))
		return strings.ReplaceAll(value, "_", "-")
	}
	want := normalize(locale)
	for _, item := range strings.Split(stored, ",") {
		normalized := normalize(item)
		if normalized == want || localeTagCompatible(normalized, want) {
			return true
		}
	}
	return false
}

func localeTagCompatible(got, want string) bool {
	gotParts := strings.Split(got, "-")
	wantParts := strings.Split(want, "-")
	if len(gotParts) == 0 || len(wantParts) == 0 || gotParts[0] != wantParts[0] {
		return false
	}
	if len(wantParts) < 2 {
		return true
	}
	wantRegion := wantParts[len(wantParts)-1]
	return gotParts[len(gotParts)-1] == wantRegion
}

func (a *App) runtimeLocaleMatches(serial, locale string) bool {
	config, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "cmd", "activity", "get-config")
	return err == nil && localeMatchesConfig(config, locale)
}

func (a *App) applyLocaleViaSettingsUI(serial, locale string) error {
	if err := a.openLocaleSettings(serial); err != nil {
		return err
	}
	nodes, err := a.dumpUINodes(serial)
	if err != nil {
		return err
	}
	if !hasSimplifiedChineseLocale(nodes) {
		if err := a.addSimplifiedChineseLocale(serial); err != nil {
			return err
		}
		if err := a.openLocaleSettings(serial); err != nil {
			return err
		}
		nodes, err = a.dumpUINodes(serial)
		if err != nil {
			return err
		}
	}
	if err := a.promoteSimplifiedChineseLocale(serial, nodes); err != nil {
		return err
	}
	return nil
}

func (a *App) openLocaleSettings(serial string) error {
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "am", "start", "-a", "android.settings.LOCALE_SETTINGS"); err != nil {
		return err
	}
	time.Sleep(1200 * time.Millisecond)
	return nil
}

func (a *App) addSimplifiedChineseLocale(serial string) error {
	nodes, err := a.dumpUINodes(serial)
	if err != nil {
		return err
	}
	add, ok := findUINode(nodes, func(node uiNode) bool {
		return node.ResourceID == "com.android.settings:id/add_language" ||
			node.Text == "Add a language" ||
			node.Text == "添加语言"
	})
	if !ok {
		return fmt.Errorf("未找到添加语言按钮")
	}
	if err := a.tapUINode(serial, add); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	nodes, err = a.dumpUINodes(serial)
	if err != nil {
		return err
	}
	search, ok := findUINode(nodes, func(node uiNode) bool {
		return node.ResourceID == "android:id/locale_search_menu" ||
			node.ContentDesc == "Search" ||
			node.ContentDesc == "搜索"
	})
	if !ok {
		return fmt.Errorf("未找到语言搜索按钮")
	}
	if err := a.tapUINode(serial, search); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := a.runToolOutput(toolADB, 5*time.Second, "-s", serial, "shell", "input", "text", "Chinese"); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	nodes, err = a.dumpUINodes(serial)
	if err != nil {
		return err
	}
	simplified, ok := findUINode(nodes, func(node uiNode) bool {
		return node.Text == "简体中文" || node.ContentDesc == "Simplified Chinese"
	})
	if !ok {
		return fmt.Errorf("未找到简体中文语言项")
	}
	if err := a.tapUINode(serial, simplified); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	nodes, err = a.dumpUINodes(serial)
	if err != nil {
		return err
	}
	china, ok := findUINode(nodes, func(node uiNode) bool {
		return node.Text == "中国" || node.ContentDesc == "China"
	})
	if !ok {
		return fmt.Errorf("未找到中国地区项")
	}
	if err := a.tapUINode(serial, china); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	return nil
}

func (a *App) promoteSimplifiedChineseLocale(serial string, nodes []uiNode) error {
	chinese, ok := findUINode(nodes, isSimplifiedChineseLocaleNode)
	if !ok {
		return fmt.Errorf("未找到已添加的简体中文语言项")
	}
	chineseBounds, ok := parseUIBounds(chinese.Bounds)
	if !ok {
		return fmt.Errorf("无法解析简体中文语言项位置：%s", chinese.Bounds)
	}
	firstLabel, ok := findUINode(nodes, func(node uiNode) bool {
		return node.ResourceID == "com.android.settings:id/miniLabel" && node.Text == "1"
	})
	if !ok {
		return fmt.Errorf("未找到第一语言位置")
	}
	firstBounds, ok := parseUIBounds(firstLabel.Bounds)
	if !ok {
		return fmt.Errorf("无法解析第一语言位置：%s", firstLabel.Bounds)
	}
	if chineseBounds.CenterY() <= firstBounds.CenterY()+20 {
		return nil
	}
	handle, ok := closestDragHandle(nodes, chineseBounds.CenterY())
	if !ok {
		return fmt.Errorf("未找到简体中文拖拽手柄")
	}
	handleBounds, ok := parseUIBounds(handle.Bounds)
	if !ok {
		return fmt.Errorf("无法解析简体中文拖拽手柄位置：%s", handle.Bounds)
	}
	if _, err := a.runToolOutput(
		toolADB,
		10*time.Second,
		"-s", serial,
		"shell", "input", "swipe",
		fmt.Sprint(handleBounds.CenterX()),
		fmt.Sprint(handleBounds.CenterY()),
		fmt.Sprint(handleBounds.CenterX()),
		fmt.Sprint(firstBounds.CenterY()-40),
		"1200",
	); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	nodes, err := a.dumpUINodes(serial)
	if err != nil {
		return err
	}
	confirm, ok := findUINode(nodes, func(node uiNode) bool {
		return node.ResourceID == "android:id/button1" ||
			node.Text == "Change" ||
			node.Text == "更改" ||
			node.Text == "确定"
	})
	if ok {
		if err := a.tapUINode(serial, confirm); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
	}
	return nil
}

func (a *App) dumpUINodes(serial string) ([]uiNode, error) {
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "uiautomator", "dump", "/sdcard/adm_window.xml"); err != nil {
		return nil, err
	}
	out, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "cat", "/sdcard/adm_window.xml")
	if err != nil {
		return nil, err
	}
	start := strings.Index(out, "<hierarchy")
	if start < 0 {
		return nil, fmt.Errorf("uiautomator 输出中没有 hierarchy")
	}
	var hierarchy uiHierarchy
	if err := xml.Unmarshal([]byte(out[start:]), &hierarchy); err != nil {
		return nil, err
	}
	var nodes []uiNode
	for _, node := range hierarchy.Nodes {
		collectUINodes(node, &nodes)
	}
	return nodes, nil
}

func (a *App) tapUINode(serial string, node uiNode) error {
	bounds, ok := parseUIBounds(node.Bounds)
	if !ok {
		return fmt.Errorf("无法解析控件位置：%s", node.Bounds)
	}
	_, err := a.runToolOutput(toolADB, 5*time.Second, "-s", serial, "shell", "input", "tap", fmt.Sprint(bounds.CenterX()), fmt.Sprint(bounds.CenterY()))
	return err
}

func collectUINodes(node uiNode, nodes *[]uiNode) {
	*nodes = append(*nodes, node)
	for _, child := range node.Nodes {
		collectUINodes(child, nodes)
	}
}

func findUINode(nodes []uiNode, match func(uiNode) bool) (uiNode, bool) {
	for _, node := range nodes {
		if match(node) {
			return node, true
		}
	}
	return uiNode{}, false
}

func hasSimplifiedChineseLocale(nodes []uiNode) bool {
	_, ok := findUINode(nodes, isSimplifiedChineseLocaleNode)
	return ok
}

func isSimplifiedChineseLocaleNode(node uiNode) bool {
	text := node.Text + " " + node.ContentDesc
	return strings.Contains(text, "简体中文") ||
		strings.Contains(text, "Simplified Chinese")
}

func closestDragHandle(nodes []uiNode, targetY int) (uiNode, bool) {
	var best uiNode
	bestDistance := 0
	found := false
	for _, node := range nodes {
		if node.ResourceID != "com.android.settings:id/dragHandle" {
			continue
		}
		bounds, ok := parseUIBounds(node.Bounds)
		if !ok {
			continue
		}
		distance := absInt(bounds.CenterY() - targetY)
		if !found || distance < bestDistance {
			best = node
			bestDistance = distance
			found = true
		}
	}
	return best, found
}

func parseUIBounds(value string) (uiBounds, bool) {
	var bounds uiBounds
	n, err := fmt.Sscanf(value, "[%d,%d][%d,%d]", &bounds.Left, &bounds.Top, &bounds.Right, &bounds.Bottom)
	return bounds, err == nil && n == 4
}

func (b uiBounds) CenterX() int {
	return (b.Left + b.Right) / 2
}

func (b uiBounds) CenterY() int {
	return (b.Top + b.Bottom) / 2
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (a *App) waitForSettingsService(serial string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	lastState := ""
	for time.Now().Before(deadline) {
		booted, bootErr := a.runToolOutput(toolADB, 5*time.Second, "-s", serial, "shell", "getprop", "sys.boot_completed")
		if bootErr != nil {
			lastState = bootErr.Error()
			time.Sleep(2 * time.Second)
			continue
		}
		if strings.TrimSpace(booted) != "1" {
			lastState = "sys.boot_completed=" + strings.TrimSpace(booted)
			time.Sleep(2 * time.Second)
			continue
		}
		if _, err := a.runToolOutput(toolADB, 5*time.Second, "-s", serial, "shell", "settings", "get", "secure", "show_ime_with_hard_keyboard"); err == nil {
			return nil
		} else {
			lastState = err.Error()
		}
		time.Sleep(2 * time.Second)
	}
	if lastState != "" {
		return fmt.Errorf("等待 Android settings 服务超时，最后状态：%s", lastState)
	}
	return fmt.Errorf("等待 Android settings 服务超时")
}

func (a *App) waitForSerialGone(serial string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		devices, err := a.activeDevices()
		if err != nil {
			return err
		}
		found := false
		for _, device := range devices {
			if device.Serial == serial {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("等待模拟器 %s 关闭超时，未重新启动以避免重复实例", serial)
}

func (a *App) waitForAVDReady(avdName string, timeout time.Duration) (*ActiveDevice, error) {
	deadline := time.Now().Add(timeout)
	lastState := ""
	for time.Now().Before(deadline) {
		devices, err := a.activeDevices()
		if err != nil {
			lastState = err.Error()
			time.Sleep(time.Second)
			continue
		}
		for i := range devices {
			device := &devices[i]
			if device.AVDName != avdName {
				continue
			}
			lastState = device.State
			if device.State == "device" {
				return device, nil
			}
		}
		time.Sleep(time.Second)
	}
	if lastState != "" {
		return nil, fmt.Errorf("等待模拟器 %s 进入 device 状态超时，最后状态：%s", avdName, lastState)
	}
	return nil, fmt.Errorf("等待模拟器 %s 进入 device 状态超时", avdName)
}

func setAVDHardwareKeyboardConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	next := updateAVDHardwareKeyboardConfig(string(data))
	return os.WriteFile(path, []byte(next), 0o644)
}

func updateAVDHardwareKeyboardConfig(content string) string {
	content = updateAVDConfigValue(content, "hw.keyboard", "yes")
	content = updateAVDConfigValue(content, "hw.keyboard.charmap", "qwerty2")
	return updateAVDConfigValue(content, "hw.keyboard.lid", "yes")
}

func setAVDConfigValue(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	next := updateAVDConfigValue(string(data), key, value)
	return os.WriteFile(path, []byte(next), 0o644)
}

func updateAVDConfigValue(content, key, value string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	replacement := key + "=" + value
	found := false

	for i, line := range lines {
		name, _, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && strings.TrimSpace(name) == key {
			lines[i] = replacement
			found = true
		}
	}
	if found {
		return strings.Join(lines, "\n")
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines[len(lines)-1] = replacement
		lines = append(lines, "")
	} else {
		lines = append(lines, replacement)
	}
	return strings.Join(lines, "\n")
}

func (a *App) textInputFlow() error {
	for {
		choice, ok := a.selectOption("输入文本", []string{
			"输入文本（自动选择方式）",
			"简单 ASCII 输入（adb input text）",
			"中文/复杂文本输入（ADB Keyboard）",
			"安装/更新 ADB Keyboard（GitHub）",
			"检查 ADB Keyboard 状态",
		})
		if !ok {
			return nil
		}
		var err error
		switch choice {
		case 0:
			err = a.autoInputTextFlow()
		case 1:
			err = a.simpleInputTextFlow()
		case 2:
			err = a.adbKeyboardInputFlow()
		case 3:
			err = a.installADBKeyboardFlow()
		case 4:
			err = a.checkADBKeyboardFlow()
		}
		if err != nil {
			fmt.Println("错误：", err)
		}
	}
}

func (a *App) autoInputTextFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择设备")
	if err != nil || !ok {
		return err
	}
	text := a.prompt("输入要发送的文本: ")
	if text == "" {
		return nil
	}
	if isSimpleADBInputText(text) {
		return a.sendSimpleInputText(device.Serial, text)
	}
	return a.sendADBKeyboardText(device.Serial, text)
}

func (a *App) simpleInputTextFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择设备")
	if err != nil || !ok {
		return err
	}
	text := a.prompt("输入简单 ASCII 文本: ")
	if text == "" {
		return nil
	}
	if !isSimpleADBInputText(text) {
		fmt.Println("该模式只适合简单 ASCII。中文或复杂符号请使用 ADB Keyboard 输入。")
		return nil
	}
	return a.sendSimpleInputText(device.Serial, text)
}

func (a *App) sendSimpleInputText(serial, text string) error {
	_, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "input", "text", encodeInputText(text))
	if err == nil {
		fmt.Println("已发送输入命令。如未输入，请确认光标在可编辑输入框内。")
	}
	return err
}

func (a *App) installADBKeyboardFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择设备")
	if err != nil || !ok {
		return err
	}
	fmt.Println("将从 GitHub 下载并安装 ADB Keyboard：")
	fmt.Println(adbKeyboardAPKURL)
	if !a.confirm("确认安装/更新到当前设备 " + device.Serial + "？") {
		return nil
	}
	if err := a.installADBKeyboardForDevice(device.Serial); err != nil {
		return err
	}
	fmt.Println("ADB Keyboard 已安装并启用。使用复杂文本输入时会临时切换到它。")
	return nil
}

func (a *App) installADBKeyboardForDevice(serial string) error {
	apkPath, cleanup, err := downloadAPK(adbKeyboardAPKURL)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := a.smartInstallAPK(serial, apkPath); err != nil {
		return err
	}
	imeID, ok, err := a.findADBKeyboardIME(serial)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("ADB Keyboard 安装后仍未出现在输入法列表")
	}
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "ime", "enable", imeID); err != nil {
		return err
	}
	return nil
}

func (a *App) adbKeyboardInputFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择设备")
	if err != nil || !ok {
		return err
	}
	text := a.prompt("输入要发送的文本: ")
	if text == "" {
		return nil
	}
	return a.sendADBKeyboardText(device.Serial, text)
}

func (a *App) sendADBKeyboardText(serial, text string) error {
	imeID, ok, err := a.ensureADBKeyboardIME(serial)
	if err != nil || !ok {
		return err
	}
	previousIME, _ := a.runToolOutput(toolADB, 5*time.Second, "-s", serial, "shell", "settings", "get", "secure", "default_input_method")
	previousIME = strings.TrimSpace(previousIME)
	switched := false
	defer func() {
		if !switched || previousIME == "" || previousIME == imeID {
			return
		}
		if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "ime", "set", previousIME); err != nil {
			fmt.Println("恢复原输入法失败：", err)
		}
	}()
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "ime", "enable", imeID); err != nil {
		return err
	}
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "ime", "set", imeID); err != nil {
		return err
	}
	switched = true
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	if _, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "am", "broadcast", "-a", "ADB_INPUT_B64", "--es", "msg", encoded); err != nil {
		return err
	}
	time.Sleep(time.Second)
	fmt.Println("已通过 ADB Keyboard 发送文本。如未输入，请确认光标在可编辑输入框内。")
	return nil
}

func (a *App) ensureADBKeyboardIME(serial string) (string, bool, error) {
	imeID, ok, err := a.findADBKeyboardIME(serial)
	if err != nil || ok {
		return imeID, ok, err
	}
	fmt.Println("未检测到 ADB Keyboard，中文/复杂文本需要该输入法。")
	if !a.confirm("是否现在安装/更新 ADB Keyboard？") {
		return "", false, nil
	}
	if err := a.installADBKeyboardForDevice(serial); err != nil {
		return "", false, err
	}
	imeID, ok, err = a.findADBKeyboardIME(serial)
	if err != nil || ok {
		return imeID, ok, err
	}
	return "", false, fmt.Errorf("ADB Keyboard 安装后仍未出现在输入法列表")
}

func (a *App) checkADBKeyboardFlow() error {
	device, ok, err := a.chooseReadyDevice("当前设备不可用，请选择设备")
	if err != nil || !ok {
		return err
	}
	imeID, ok, err := a.findADBKeyboardIME(device.Serial)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("未检测到 ADB Keyboard。")
		fmt.Println("简单 ASCII 可用 adb input text；中文/复杂文本需要先在设备上安装 ADB Keyboard 输入法。")
		return nil
	}
	fmt.Println("已安装 ADB Keyboard：", imeID)
	return nil
}

func (a *App) findADBKeyboardIME(serial string) (string, bool, error) {
	out, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "ime", "list", "-s")
	if err != nil {
		return "", false, err
	}
	for _, raw := range strings.Split(out, "\n") {
		imeID := strings.TrimSpace(raw)
		if strings.Contains(strings.ToLower(imeID), "adbkeyboard") {
			return imeID, true, nil
		}
	}
	return "", false, nil
}

func (a *App) aliasManagementFlow() error {
	for {
		choice, ok := a.selectOption("别名管理", []string{
			"查看别名",
			"设置/修改别名",
			"删除别名",
		})
		if !ok {
			return nil
		}
		var err error
		switch choice {
		case 0:
			a.printAliases()
		case 1:
			err = a.setAliasFlow()
		case 2:
			err = a.deleteAliasFlow()
		}
		if err != nil {
			fmt.Println("错误：", err)
		}
	}
}

func (a *App) printAliases() {
	if len(a.cfg.Aliases) == 0 {
		fmt.Println("暂无别名。")
		return
	}
	fmt.Println("当前别名:")
	for key, alias := range a.cfg.Aliases {
		fmt.Printf("  %s => %s\n", key, alias)
	}
}

func (a *App) setAliasFlow() error {
	entries, err := a.allDeviceEntries()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("没有可设置别名的设备")
	}
	var options []string
	for _, entry := range entries {
		options = append(options, entry.Label)
	}
	idx, ok := a.selectOption("选择设备/模拟器", options)
	if !ok {
		return nil
	}
	alias := a.prompt("输入别名: ")
	if alias == "" {
		return nil
	}
	a.cfg.Aliases[entries[idx].Key] = alias
	return saveConfig(a.cfg)
}

func (a *App) deleteAliasFlow() error {
	if len(a.cfg.Aliases) == 0 {
		fmt.Println("暂无别名。")
		return nil
	}
	var keys []string
	var options []string
	for key, alias := range a.cfg.Aliases {
		keys = append(keys, key)
		options = append(options, fmt.Sprintf("%s => %s", key, alias))
	}
	idx, ok := a.selectOption("选择要删除的别名", options)
	if !ok {
		return nil
	}
	delete(a.cfg.Aliases, keys[idx])
	return saveConfig(a.cfg)
}

func (a *App) toolsFlow() error {
	for {
		a.tools.Refresh()
		a.printToolSummary()
		choice, ok := a.selectOption("工具状态/路径设置", []string{
			"重新探测工具",
			"查看工具诊断信息",
			"设置工具路径",
			"清除固定工具路径",
		})
		if !ok {
			return nil
		}
		switch choice {
		case 0:
			a.tools.Refresh()
		case 1:
			a.printToolDoctor()
		case 2:
			if err := a.setToolPathFlow(); err != nil {
				fmt.Println("错误：", err)
			}
		case 3:
			if err := a.clearToolPathFlow(); err != nil {
				fmt.Println("错误：", err)
			}
		}
	}
}

func (a *App) printToolDoctor() {
	if path, err := configPath(); err == nil {
		fmt.Println("配置文件：", path)
	}
	fmt.Println("环境变量：")
	fmt.Println("  ANDROID_HOME=" + os.Getenv("ANDROID_HOME"))
	fmt.Println("  ANDROID_SDK_ROOT=" + os.Getenv("ANDROID_SDK_ROOT"))
	fmt.Println("SDK 候选根目录：")
	for _, root := range androidSDKRoots() {
		fmt.Println("  " + root)
	}
	fmt.Println("工具候选路径：")
	for _, name := range allTools {
		status := a.tools.Status(name)
		if status.Available {
			fmt.Printf("  %s 当前使用：%s (%s)\n", name, status.Path, status.Source)
		} else {
			fmt.Printf("  %s 当前不可用：%s\n", name, status.Error)
		}
		for _, candidate := range a.tools.candidates(name) {
			if candidate.Path == "" {
				continue
			}
			marker := "不可执行"
			path := expandPath(candidate.Path)
			if isExecutable(path) {
				marker = "可执行"
			}
			fmt.Printf("    - %s [%s] %s\n", path, candidate.Source, marker)
		}
	}
}

func (a *App) setToolPathFlow() error {
	idx, ok := a.selectOption("选择工具", allTools)
	if !ok {
		return nil
	}
	name := allTools[idx]
	path := normalizeAPKPath(a.prompt("输入 " + name + " 可执行文件路径: "))
	if path == "" {
		return nil
	}
	if !isExecutable(path) {
		return fmt.Errorf("不是可执行文件：%s", path)
	}
	if err := validateTool(name, path); err != nil {
		return err
	}
	a.cfg.ToolPaths[name] = path
	if err := saveConfig(a.cfg); err != nil {
		return err
	}
	a.tools.Refresh()
	return nil
}

func (a *App) clearToolPathFlow() error {
	idx, ok := a.selectOption("选择工具", allTools)
	if !ok {
		return nil
	}
	delete(a.cfg.ToolPaths, allTools[idx])
	if err := saveConfig(a.cfg); err != nil {
		return err
	}
	a.tools.Refresh()
	return nil
}

func (a *App) installedSystemImages() ([]string, error) {
	out, err := a.runToolOutput(toolSDKManager, 30*time.Second, "--list_installed")
	if err != nil {
		return nil, err
	}
	return parseSystemImages(out), nil
}

func (a *App) deviceTemplates() ([]DeviceTemplate, error) {
	out, err := a.runToolOutput(toolAVDManager, 15*time.Second, "list", "device")
	if err != nil {
		return nil, err
	}
	return parseDeviceTemplates(out), nil
}

func deviceTemplateCreateID(template DeviceTemplate) string {
	if strings.TrimSpace(template.CreateID) != "" {
		return template.CreateID
	}
	return template.ID
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	var out []string
	for _, value := range values {
		if value == target {
			continue
		}
		out = append(out, value)
	}
	return out
}
