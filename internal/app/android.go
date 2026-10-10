package app

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ActiveDevice struct {
	Serial     string
	State      string
	Raw        string
	Details    map[string]string
	IsEmulator bool
	AVDName    string
}

type AVD struct {
	Name   string
	Device string
	Path   string
	Target string
}

type DeviceTemplate struct {
	ID       string
	Name     string
	CreateID string
}

type DeviceEntry struct {
	Kind    string
	Label   string
	Key     string
	Active  *ActiveDevice
	AVD     *AVD
	Running bool
}

type runningEmulatorProcess struct {
	PID      string
	AVDName  string
	Serials  []string
	GRPCPort int
}

func (a *App) runToolOutput(tool string, timeout time.Duration, args ...string) (string, error) {
	path, err := a.tools.Path(tool)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return text, fmt.Errorf("%s 调用超时\n路径：%s\n参数：%s", tool, path, strings.Join(args, " "))
		}
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("%s 调用失败：%s\n路径：%s\n参数：%s", tool, text, path, strings.Join(args, " "))
	}
	return text, nil
}

func (a *App) runToolBytes(tool string, timeout time.Duration, args ...string) ([]byte, error) {
	path, err := a.tools.Path(tool)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		text := strings.TrimSpace(stderr.String())
		if ctx.Err() == context.DeadlineExceeded {
			return out, fmt.Errorf("%s 调用超时\n路径：%s\n参数：%s", tool, path, strings.Join(args, " "))
		}
		if text == "" {
			text = err.Error()
		}
		return out, fmt.Errorf("%s 调用失败：%s\n路径：%s\n参数：%s", tool, firstLine(text), path, strings.Join(args, " "))
	}
	return out, nil
}

func (a *App) runToolStreaming(tool string, args ...string) error {
	path, err := a.tools.Path(tool)
	if err != nil {
		return err
	}
	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s 调用失败：%w\n路径：%s\n参数：%s", tool, err, path, strings.Join(args, " "))
	}
	return nil
}

func (a *App) ensureAPKInstallSpace(serial, apkPath string) error {
	info, err := os.Stat(apkPath)
	if err != nil {
		return fmt.Errorf("读取 APK 大小失败：%w", err)
	}
	free, err := a.deviceDataFreeBytes(serial)
	if err != nil {
		return fmt.Errorf("安装前检查设备存储空间失败：%w", err)
	}
	required := requiredInstallFreeBytes(info.Size())
	if free >= required {
		return nil
	}
	return fmt.Errorf("设备 %s 的 /data 可用空间不足：可用 %s，APK %s，建议至少保留 %s（APK 大小的 3 倍且不低于 1.5 GiB）。请清理应用数据、卸载无用应用，或使用 32 GiB 数据分区重新创建模拟器",
		serial, formatBytesIEC(free), formatBytesIEC(info.Size()), formatBytesIEC(required))
}

func (a *App) deviceDataFreeBytes(serial string) (int64, error) {
	out, err := a.runToolOutput(toolADB, 10*time.Second, "-s", serial, "shell", "df", "-k", "/data")
	if err != nil {
		return 0, err
	}
	return parseDataFreeBytesFromDF(out)
}

func requiredInstallFreeBytes(apkBytes int64) int64 {
	required := apkBytes * installFreeSpaceMultiplier
	if required < minInstallFreeSpaceBytes {
		return minInstallFreeSpaceBytes
	}
	return required
}

func parseDataFreeBytesFromDF(output string) (int64, error) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || strings.EqualFold(fields[0], "Filesystem") {
			continue
		}
		availableKB, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			continue
		}
		return availableKB * 1024, nil
	}
	return 0, fmt.Errorf("无法解析 df 输出：%s", firstLine(output))
}

func formatBytesIEC(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PiB", value/unit)
}

func (a *App) activeDevices() ([]ActiveDevice, error) {
	out, err := a.runToolOutput(toolADB, 10*time.Second, "devices", "-l")
	if err != nil {
		return nil, err
	}
	devices := parseADBDevices(out)
	processAVDs := runningEmulatorAVDNamesBySerial()
	for i := range devices {
		if !devices[i].IsEmulator {
			continue
		}
		if devices[i].State == "device" {
			if name, ok := a.emulatorAVDName(devices[i].Serial); ok {
				devices[i].AVDName = name
				continue
			}
		}
		if name := processAVDs[devices[i].Serial]; name != "" {
			devices[i].AVDName = name
		}
	}
	return devices, nil
}

func (a *App) avds() ([]AVD, error) {
	out, err := a.runToolOutput(toolAVDManager, 15*time.Second, "list", "avd")
	if err != nil {
		return nil, err
	}
	return parseAVDList(out), nil
}

func (a *App) allDeviceEntries() ([]DeviceEntry, error) {
	active, activeErr := a.activeDevices()
	avds, avdErr := a.avds()
	if activeErr != nil && avdErr != nil {
		return nil, fmt.Errorf("活跃设备和 AVD 均查询失败：%v；%v", activeErr, avdErr)
	}

	activeByAVD := map[string]*ActiveDevice{}
	usedSerials := map[string]bool{}
	for i := range active {
		d := &active[i]
		if d.AVDName != "" {
			activeByAVD[d.AVDName] = d
		}
	}

	var entries []DeviceEntry
	for i := range avds {
		avd := &avds[i]
		d := activeByAVD[avd.Name]
		entry := DeviceEntry{
			Kind:    "模拟器",
			Key:     aliasKeyForAVD(avd.Name),
			AVD:     avd,
			Active:  d,
			Running: d != nil,
		}
		if d != nil {
			usedSerials[d.Serial] = true
		}
		entry.Label = a.formatDeviceEntry(entry)
		entries = append(entries, entry)
	}

	for i := range active {
		d := &active[i]
		if usedSerials[d.Serial] {
			continue
		}
		kind := "真机"
		key := aliasKeyForSerial(d.Serial)
		if d.IsEmulator {
			kind = "模拟器"
			if d.AVDName != "" {
				key = aliasKeyForAVD(d.AVDName)
			}
		}
		entry := DeviceEntry{
			Kind:    kind,
			Key:     key,
			Active:  d,
			Running: d.State == "device",
		}
		entry.Label = a.formatDeviceEntry(entry)
		entries = append(entries, entry)
	}

	sortDeviceEntries(entries)
	return entries, nil
}

func sortDeviceEntries(entries []DeviceEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		physicalI := entries[i].Active != nil && !entries[i].Active.IsEmulator
		physicalJ := entries[j].Active != nil && !entries[j].Active.IsEmulator
		if physicalI != physicalJ {
			return physicalI
		}
		if entries[i].Running != entries[j].Running {
			return entries[i].Running
		}
		return entries[i].Label < entries[j].Label
	})
}

func (a *App) emulatorAVDName(serial string) (string, bool) {
	out, err := a.runToolOutput(toolADB, 2*time.Second, "-s", serial, "emu", "avd", "name")
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" {
			continue
		}
		return line, true
	}
	return "", false
}

func runningEmulatorAVDNamesBySerial() map[string]string {
	result := map[string]string{}
	for _, proc := range runningEmulatorProcesses() {
		if proc.AVDName == "" {
			continue
		}
		for _, serial := range proc.Serials {
			result[serial] = proc.AVDName
		}
	}
	return result
}

func runningEmulatorProcesses() []runningEmulatorProcess {
	out, err := exec.Command("ps", "-axo", "pid,args").Output()
	if err != nil {
		return nil
	}
	var processes []runningEmulatorProcess
	for _, raw := range strings.Split(string(out), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "PID ") {
			continue
		}
		pid, args, ok := strings.Cut(raw, " ")
		if !ok {
			continue
		}
		avdName, ports, grpcPort := parseEmulatorProcessArgs(args)
		if avdName == "" {
			continue
		}
		var serials []string
		for _, port := range ports {
			if port != "" {
				serials = append(serials, "emulator-"+port)
			}
		}
		processes = append(processes, runningEmulatorProcess{
			PID:      pid,
			AVDName:  avdName,
			Serials:  serials,
			GRPCPort: grpcPort,
		})
	}
	return processes
}

func runningEmulatorProcessesForAVD(avdName string) []runningEmulatorProcess {
	var matches []runningEmulatorProcess
	for _, proc := range runningEmulatorProcesses() {
		if proc.AVDName == avdName {
			matches = append(matches, proc)
		}
	}
	return matches
}

func forceTerminateProcess(pid string) error {
	return exec.Command("kill", "-TERM", pid).Run()
}

func parseEmulatorProcessArgs(raw string) (string, []string, int) {
	if !strings.Contains(raw, "qemu-system") && !strings.Contains(raw, "emulator") {
		return "", nil, 0
	}
	fields := strings.Fields(raw)
	avdName := ""
	var ports []string
	grpcPort := 0
	for i, field := range fields {
		switch {
		case field == "-avd" && i+1 < len(fields):
			avdName = fields[i+1]
		case strings.HasPrefix(field, "-avd="):
			avdName = strings.TrimPrefix(field, "-avd=")
		case strings.HasPrefix(field, "@") && len(field) > 1:
			avdName = strings.TrimPrefix(field, "@")
		case field == "-port" && i+1 < len(fields):
			ports = append(ports, fields[i+1])
		case strings.HasPrefix(field, "-port="):
			ports = append(ports, strings.TrimPrefix(field, "-port="))
		case field == "-ports" && i+1 < len(fields):
			portPair := strings.Split(fields[i+1], ",")
			if len(portPair) > 0 {
				ports = append(ports, portPair[0])
			}
		case strings.HasPrefix(field, "-ports="):
			portPair := strings.Split(strings.TrimPrefix(field, "-ports="), ",")
			if len(portPair) > 0 {
				ports = append(ports, portPair[0])
			}
		case field == "-grpc" && i+1 < len(fields):
			grpcPort, _ = strconv.Atoi(fields[i+1])
		case strings.HasPrefix(field, "-grpc="):
			grpcPort, _ = strconv.Atoi(strings.TrimPrefix(field, "-grpc="))
		}
	}
	if avdName == "" {
		return "", nil, 0
	}
	return avdName, ports, grpcPort
}

func emulatorGRPCPortForAVDName(avdName string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(avdName))
	return 30000 + int(hash.Sum32()%10000)
}

func availableEmulatorGRPCPort(avdName string) int {
	start := emulatorGRPCPortForAVDName(avdName)
	for offset := 0; offset < 1000; offset++ {
		port := 30000 + ((start - 30000 + offset) % 10000)
		if isTCPPortAvailable(port) {
			return port
		}
	}
	return start
}

func isTCPPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func parseADBDevices(out string) []ActiveDevice {
	var devices []ActiveDevice
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "* daemon") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		d := ActiveDevice{
			Serial:     fields[0],
			State:      fields[1],
			Raw:        line,
			Details:    map[string]string{},
			IsEmulator: strings.HasPrefix(fields[0], "emulator-"),
		}
		for _, field := range fields[2:] {
			if k, v, ok := strings.Cut(field, ":"); ok {
				d.Details[k] = v
			}
		}
		devices = append(devices, d)
	}
	return devices
}

func parseAVDList(out string) []AVD {
	var avds []AVD
	var current *AVD
	flush := func() {
		if current != nil && current.Name != "" {
			avds = append(avds, *current)
		}
		current = nil
	}

	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || line == "---------" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Name:"):
			flush()
			current = &AVD{Name: strings.TrimSpace(strings.TrimPrefix(line, "Name:"))}
		case current != nil && strings.HasPrefix(line, "Device:"):
			current.Device = strings.TrimSpace(strings.TrimPrefix(line, "Device:"))
		case current != nil && strings.HasPrefix(line, "Path:"):
			current.Path = strings.TrimSpace(strings.TrimPrefix(line, "Path:"))
		case current != nil && strings.HasPrefix(line, "Target:"):
			current.Target = strings.TrimSpace(strings.TrimPrefix(line, "Target:"))
		case current != nil && strings.HasPrefix(line, "Based on:"):
			extra := strings.TrimSpace(line)
			if current.Target == "" {
				current.Target = extra
			} else {
				current.Target += " " + extra
			}
		}
	}
	flush()
	return avds
}

func parseSystemImages(out string) []string {
	var images []string
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "system-images;") {
			continue
		}
		if before, _, ok := strings.Cut(line, "|"); ok {
			line = strings.TrimSpace(before)
		}
		images = append(images, line)
	}
	sort.Strings(images)
	return uniqueStrings(images)
}

func parseDeviceTemplates(out string) []DeviceTemplate {
	var templates []DeviceTemplate
	var current *DeviceTemplate
	flush := func() {
		if current != nil && current.ID != "" {
			templates = append(templates, *current)
		}
		current = nil
	}
	idPattern := regexp.MustCompile(`"([^"]+)"`)
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "id:") {
			flush()
			value := strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			id := value
			if m := idPattern.FindStringSubmatch(value); len(m) == 2 {
				id = m[1]
			} else {
				id = strings.Fields(value)[0]
			}
			current = &DeviceTemplate{ID: id}
			continue
		}
		if current != nil && strings.HasPrefix(line, "Name:") {
			current.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		}
	}
	flush()
	var filtered []DeviceTemplate
	for _, template := range templates {
		if isModernPhoneOrTabletTemplate(template) {
			filtered = append(filtered, template)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		li := deviceTemplateRank(filtered[i])
		lj := deviceTemplateRank(filtered[j])
		if li != lj {
			return li < lj
		}
		return filtered[i].Name < filtered[j].Name
	})
	return filtered
}

func isModernPhoneOrTabletTemplate(template DeviceTemplate) bool {
	text := strings.ToLower(template.ID + " " + template.Name)
	excluded := []string{
		"automotive", "auto", "desktop", "television", " tv_", "android-tv",
		"wear", "watch", "xr", "glasses", "headset", "resizable", "freeform",
		"rollable",
	}
	for _, word := range excluded {
		if strings.Contains(text, word) {
			return false
		}
	}
	if isOldNamedDeviceTemplate(text) {
		return false
	}
	included := []string{"pixel", "phone", "tablet"}
	for _, word := range included {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func isOldNamedDeviceTemplate(text string) bool {
	oldPrefixes := []string{"nexus", "galaxy nexus", "pixel c", "pixel xl"}
	for _, prefix := range oldPrefixes {
		if strings.Contains(text, prefix) {
			return true
		}
	}
	oldPixelGenerations := []string{
		"pixel_2", "pixel 2",
		"pixel_3", "pixel 3",
		"pixel_4", "pixel 4",
		"pixel_5", "pixel 5",
		"pixel_6", "pixel 6",
		"pixel_7", "pixel 7",
	}
	for _, generation := range oldPixelGenerations {
		if strings.Contains(text, generation) {
			return true
		}
	}
	return text == "pixel pixel"
}

func deviceTemplateRank(template DeviceTemplate) int {
	text := strings.ToLower(template.ID + " " + template.Name)
	if strings.Contains(text, "pixel_10") || strings.Contains(text, "pixel 10") {
		return 0
	}
	if strings.Contains(text, "pixel_9") || strings.Contains(text, "pixel 9") {
		return 10
	}
	if strings.Contains(text, "pixel_8") || strings.Contains(text, "pixel 8") {
		return 20
	}
	if strings.Contains(text, "pixel_7") || strings.Contains(text, "pixel 7") {
		return 30
	}
	if strings.Contains(text, "pixel_6") || strings.Contains(text, "pixel 6") {
		return 40
	}
	if strings.Contains(text, "pixel") && (strings.Contains(text, "fold") || strings.Contains(text, "tablet")) {
		return 45
	}
	if strings.Contains(text, "pixel") {
		return 50
	}
	if strings.Contains(text, "phone") {
		return 60
	}
	if strings.Contains(text, "tablet") || strings.Contains(text, "fold") {
		return 70
	}
	if strings.Contains(text, "nexus") {
		return 80
	}
	return 90
}

func normalizeAPKPath(input string) string {
	path := strings.TrimSpace(input)
	path = strings.Trim(path, "\"'")
	return expandPath(path)
}

func normalizeAPKSource(input string) string {
	source := strings.TrimSpace(input)
	return strings.Trim(source, "\"'")
}

func isHTTPURL(input string) bool {
	u, err := url.Parse(strings.TrimSpace(input))
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func apkNameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "download.apk"
	}
	name := filepath.Base(u.Path)
	if name == "." || name == "/" || name == "" {
		return "download.apk"
	}
	return name
}

func downloadAPK(rawURL string) (string, func(), error) {
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("下载失败：HTTP %d", resp.StatusCode)
	}

	name := apkNameFromDownload(rawURL, resp.Header)
	tmpFile, err := os.CreateTemp("", "adm-*-"+name)
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = os.Remove(tmpFile.Name())
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return "", nil, err
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return tmpFile.Name(), cleanup, nil
}

func apkNameFromDownload(rawURL string, header http.Header) string {
	if disposition := strings.TrimSpace(header.Get("Content-Disposition")); disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			if filename := strings.TrimSpace(params["filename"]); filename != "" {
				return filepath.Base(filename)
			}
		}
	}
	return apkNameFromURL(rawURL)
}

func encodeInputText(text string) string {
	replacer := strings.NewReplacer(
		" ", "%s",
		"\n", "%n",
		"\t", "%t",
		"%", "\\%",
		"'", "\\'",
		"\"", "\\\"",
		"(", "\\(",
		")", "\\)",
		"&", "\\&",
		"<", "\\<",
		">", "\\>",
		";", "\\;",
		"*", "\\*",
		"|", "\\|",
		"~", "\\~",
		"`", "\\`",
	)
	return replacer.Replace(text)
}

func packageLinesToNames(out string) []string {
	var packages []string
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "package:")
		if line != "" {
			packages = append(packages, line)
		}
	}
	sort.Strings(packages)
	return uniqueStrings(packages)
}

func localeMatchesConfig(configOutput, locale string) bool {
	language, region := splitLocale(locale)
	if language == "" {
		return false
	}
	config := strings.ToLower(configOutput)
	primaryConfig := config
	if idx := strings.Index(primaryConfig, ","); idx >= 0 {
		primaryConfig = primaryConfig[:idx]
	}
	language = strings.ToLower(language)
	region = strings.ToLower(region)
	token := "-" + language
	if region != "" {
		token += "-r" + region
	}
	if strings.Contains(primaryConfig, token) {
		return true
	}
	if !strings.Contains(primaryConfig, "b+"+language) {
		return false
	}
	if region == "" {
		return true
	}
	idx := strings.Index(primaryConfig, "b+"+language)
	if idx < 0 {
		return false
	}
	return strings.Contains(primaryConfig[idx:], "+"+region)
}

func isSimpleADBInputText(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < 32 || r > 126 {
			return false
		}
	}
	return true
}

func splitLocale(locale string) (string, string) {
	locale = strings.TrimSpace(locale)
	locale = strings.ReplaceAll(locale, "_", "-")
	if locale == "" {
		return "", ""
	}
	parts := strings.Split(locale, "-")
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func ensureLogFile(name string) (*os.File, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	safeName := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(name)
	return os.OpenFile(filepath.Join(dir, safeName+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

func aliasKeyForSerial(serial string) string {
	return "serial:" + serial
}

func aliasKeyForAVD(name string) string {
	return "avd:" + name
}
