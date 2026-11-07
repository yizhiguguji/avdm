package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DeviceUnlockSettings struct {
	Kind                     string
	Secret                   string
	Automatic                bool
	Left, Top, Right, Bottom int  // Percent positions of the outer pattern dot centers.
	Configured               bool `json:"-"`
	Paused                   bool `json:"-"`
}
type unlockCacheEntry struct {
	settings DeviceUnlockSettings
	paused   bool
}
type deviceUnlockState struct {
	mu       sync.Mutex
	profiles map[string]unlockCacheEntry
	busy     map[string]bool
}

func (a *App) unlockState() *deviceUnlockState {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.unlock == nil {
		a.unlock = &deviceUnlockState{profiles: map[string]unlockCacheEntry{}, busy: map[string]bool{}}
	}
	return a.unlock
}
func (a *App) unlockIdentity(serial string) (string, error) {
	identity, err := a.runToolOutput(toolADB, 3*time.Second, "-s", serial, "shell", "getprop", "ro.serialno")
	identity = strings.TrimSpace(identity)
	if err != nil || identity == "" || identity == "unknown" {
		return "", fmt.Errorf("无法确认设备硬件标识，未读取或输入解锁凭据")
	}
	return identity, nil
}
func (a *App) unlockProfile(identity string) (unlockCacheEntry, error) {
	state := a.unlockState()
	state.mu.Lock()
	entry, ok := state.profiles[identity]
	state.mu.Unlock()
	if ok {
		return entry, nil
	}
	raw, err := unlockSecretRead(identity)
	if err != nil {
		return entry, err
	}
	if raw != "" {
		if json.Unmarshal([]byte(raw), &entry.settings) != nil {
			return entry, fmt.Errorf("解锁设置损坏，请重新保存")
		}
		entry.settings.Configured = true
	}
	state.mu.Lock()
	if old, exists := state.profiles[identity]; exists {
		entry = old
	} else {
		state.profiles[identity] = entry
	}
	state.mu.Unlock()
	return entry, nil
}
func (a *App) GUIGetDeviceUnlockSettings(serial string) (DeviceUnlockSettings, error) {
	id, err := a.unlockIdentity(serial)
	if err != nil {
		return DeviceUnlockSettings{}, err
	}
	entry, err := a.unlockProfile(id)
	settings := entry.settings
	settings.Secret = ""
	settings.Paused = entry.paused
	return settings, err
}
func validateDeviceUnlockSettings(s DeviceUnlockSettings) error {
	switch s.Kind {
	case "PIN":
		if len(s.Secret) < 4 || len(s.Secret) > 64 || strings.IndexFunc(s.Secret, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return fmt.Errorf("PIN 需为 4–64 位数字")
		}
	case "密码":
		if len(s.Secret) < 4 || len(s.Secret) > 128 || strings.IndexFunc(s.Secret, func(r rune) bool { return r < 32 || r > 126 }) >= 0 {
			return fmt.Errorf("密码支持 4–128 位英文、数字和符号；暂不支持中文等非 ASCII 字符")
		}
	case "图案":
		if len(s.Secret) < 4 || len(s.Secret) > 9 {
			return fmt.Errorf("图案需为 4–9 个不重复的点，使用数字 1–9 表示")
		}
		seen := map[byte]bool{}
		for i, p := range []byte(s.Secret) {
			if p < '1' || p > '9' || seen[p] {
				return fmt.Errorf("图案只能包含 1–9，且不能重复")
			}
			if i > 0 {
				prev := s.Secret[i-1] - '1'
				next := p - '1'
				x1, y1 := int(prev%3), int(prev/3)
				x2, y2 := int(next%3), int(next/3)
				if (x1+x2)%2 == 0 && (y1+y2)%2 == 0 {
					mid := byte((x1+x2)/2+(y1+y2)/2*3) + '1'
					if mid != s.Secret[i-1] && mid != p && !seen[mid] {
						return fmt.Errorf("图案跨过未选中的点，请在顺序中加入中间点")
					}
				}
			}
			seen[p] = true
		}
		if s.Left < 1 || s.Top < 1 || s.Right > 99 || s.Bottom > 99 || s.Left >= s.Right || s.Top >= s.Bottom {
			return fmt.Errorf("图案网格位置需在屏幕 1–99%% 范围内，左小于右、上小于下")
		}
	default:
		return fmt.Errorf("请选择 PIN、密码或图案")
	}
	return nil
}
func (a *App) lockUnlockSettings(identity string) (func(), error) {
	state := a.unlockState()
	state.mu.Lock()
	if state.busy[identity] {
		state.mu.Unlock()
		return nil, fmt.Errorf("设备正在解锁，请等待结果后再修改设置")
	}
	state.busy[identity] = true
	state.mu.Unlock()
	return func() { state.mu.Lock(); delete(state.busy, identity); state.mu.Unlock() }, nil
}

func (a *App) GUISaveDeviceUnlockSettings(serial string, s DeviceUnlockSettings) error {
	id, err := a.unlockIdentity(serial)
	if err != nil {
		return err
	}
	release, err := a.lockUnlockSettings(id)
	if err != nil {
		return err
	}
	defer release()

	if s.Secret == "" {
		old, err := a.unlockProfile(id)
		if err != nil {
			return err
		}
		if old.settings.Kind == s.Kind {
			s.Secret = old.settings.Secret
		}
	}
	if err := validateDeviceUnlockSettings(s); err != nil {
		return err
	}
	s.Configured = true
	s.Paused = false
	raw, _ := json.Marshal(s)
	if err := unlockSecretSave(id, string(raw)); err != nil {
		return err
	}
	state := a.unlockState()
	state.mu.Lock()
	state.profiles[id] = unlockCacheEntry{settings: s}
	state.mu.Unlock()
	return nil
}
func (a *App) GUIDeleteDeviceUnlockSettings(serial string) error {
	id, err := a.unlockIdentity(serial)
	if err != nil {
		return err
	}
	release, err := a.lockUnlockSettings(id)
	if err != nil {
		return err
	}
	defer release()

	if err := unlockSecretDelete(id); err != nil {
		return err
	}
	state := a.unlockState()
	state.mu.Lock()
	state.profiles[id] = unlockCacheEntry{}
	state.mu.Unlock()
	return nil
}

var unlockKeyguardPattern = regexp.MustCompile(`(?m)^\s*(?:showing|mIsShowing|mShowingLockscreen|mKeyguardShowing)=(true|false)\s*$`)

func deviceLockState(output string) (bool, error) {
	if index := strings.Index(output, "KeyguardServiceDelegate"); index >= 0 {
		output = output[index:]
	}
	matches := unlockKeyguardPattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return false, fmt.Errorf("无法确认锁屏状态，已停止输入。请在手机上解锁")
	}
	locked := matches[0][1] == "true"
	for _, m := range matches {
		if (m[1] == "true") != locked {
			return false, fmt.Errorf("锁屏状态不一致，已停止输入")
		}
	}
	return locked, nil
}
func (a *App) deviceLocked(serial string) (bool, error) {
	out, err := a.runToolOutput(toolADB, 4*time.Second, "-s", serial, "shell", "dumpsys", "window", "policy")
	if err != nil {
		return false, fmt.Errorf("读取锁屏状态失败，未输入凭据")
	}
	return deviceLockState(out)
}

var unlockSizePattern = regexp.MustCompile(`(\d+)x(\d+)`)

func unlockScreenSize(output string) (int, int, error) {
	matches := unlockSizePattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return 0, 0, fmt.Errorf("无法读取屏幕尺寸")
	}
	m := matches[len(matches)-1]
	w, _ := strconv.Atoi(m[1])
	h, _ := strconv.Atoi(m[2])
	if w < 100 || h < 100 {
		return 0, 0, fmt.Errorf("屏幕尺寸无效")
	}
	return w, h, nil
}
func unlockShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
func unlockCredentialScript(s DeviceUnlockSettings, w, h int) string {
	if s.Kind != "图案" {
		var commands []string
		for i, part := range strings.Split(s.Secret, "%") {
			if i > 0 {
				commands = append(commands, "input text '%'")
			}
			if part != "" {
				commands = append(commands, "input text "+unlockShellQuote(strings.ReplaceAll(part, " ", "%s")))
			}
		}
		return "set -e\n" + strings.Join(commands, "\n") + "\n"
	}
	var commands []string
	for i, p := range []byte(s.Secret) {
		n := int(p - '1')
		x := w * (s.Left + (s.Right-s.Left)*(n%3)/2) / 100
		y := h * (s.Top + (s.Bottom-s.Top)*(n/3)/2) / 100
		action := "MOVE"
		if i == 0 {
			action = "DOWN"
		}
		commands = append(commands, fmt.Sprintf("input motionevent %s %d %d", action, x, y), "sleep 0.05")
		if i == len(s.Secret)-1 {
			commands = append(commands, fmt.Sprintf("input motionevent UP %d %d", x, y))
		}
	}
	return "set -e\ntrap 'input motionevent CANCEL' EXIT\n" + strings.Join(commands, "\n") + "\n"
}
func (a *App) GUIUnlockDevice(serial string) error {
	_, err := a.unlockDevice(serial, false)
	return err
}
func (a *App) GUIAutoUnlockDevice(serial string) (bool, error) { return a.unlockDevice(serial, true) }
func (a *App) unlockDevice(serial string, automatic bool) (bool, error) {
	id, err := a.unlockIdentity(serial)
	if err != nil {
		return false, err
	}
	entry, err := a.unlockProfile(id)
	if err != nil {
		return false, err
	}
	if automatic && (!entry.settings.Automatic || entry.paused) {
		return false, nil
	}
	if !entry.settings.Configured {
		return false, fmt.Errorf("请先在设备“…”菜单的解锁设置中保存凭据")
	}
	if err := validateDeviceUnlockSettings(entry.settings); err != nil {
		return false, err
	}
	state := a.unlockState()
	state.mu.Lock()
	if state.busy[id] {
		state.mu.Unlock()
		return false, nil
	}
	entry = state.profiles[id]
	if automatic && (!entry.settings.Automatic || entry.paused) {
		state.mu.Unlock()
		return false, nil
	}
	if !entry.settings.Configured {
		state.mu.Unlock()
		return false, fmt.Errorf("解锁设置已清除，请重新设置")
	}
	state.busy[id] = true
	state.mu.Unlock()
	defer func() { state.mu.Lock(); delete(state.busy, id); state.mu.Unlock() }()
	locked, err := a.deviceLocked(serial)
	if err != nil || !locked {
		return false, err
	}
	// Pause before sending any credential; failures remain paused until settings are saved or a manual attempt succeeds.
	state.mu.Lock()
	current := state.profiles[id]
	current.paused = true
	state.profiles[id] = current
	state.mu.Unlock()
	err = a.performDeviceUnlock(serial, id, entry.settings)
	if err == nil {
		state.mu.Lock()
		current = state.profiles[id]
		current.paused = false
		state.profiles[id] = current
		state.mu.Unlock()
	}
	return true, err
}
func (a *App) performDeviceUnlock(serial, identity string, s DeviceUnlockSettings) error {
	size, err := a.runToolOutput(toolADB, 3*time.Second, "-s", serial, "shell", "wm", "size")
	if err != nil {
		return fmt.Errorf("读取屏幕尺寸失败，已暂停自动解锁")
	}
	w, h, err := unlockScreenSize(size)
	if err != nil {
		return err
	}
	if err := a.GUIKeyEventDevice(serial, 224); err != nil {
		return fmt.Errorf("唤醒失败，已暂停自动解锁")
	}
	time.Sleep(400 * time.Millisecond)
	if err := a.GUISwipeDevice(serial, w/2, h*85/100, w/2, h/4, 300); err != nil {
		return fmt.Errorf("进入密码界面失败，已暂停自动解锁")
	}
	time.Sleep(500 * time.Millisecond)
	locked, err := a.deviceLocked(serial)
	if err != nil || !locked {
		return err
	}
	path, err := a.tools.Path(toolADB)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-s", serial, "shell", "sh")
	cmd.Stdin = strings.NewReader("[ \"$(getprop ro.serialno)\" = " + unlockShellQuote(identity) + " ] || exit 1\n" + unlockCredentialScript(s, w, h))
	if cmd.Run() != nil {
		return fmt.Errorf("解锁输入未完成，已暂停自动解锁；请检查设备是否支持输入注入")
	}
	// Some PIN screens auto-confirm. Never send Enter into an already unlocked app.
	time.Sleep(200 * time.Millisecond)
	locked, err = a.deviceLocked(serial)
	if err != nil {
		return fmt.Errorf("输入后无法确认锁屏状态，已暂停自动解锁")
	}
	if !locked {
		return nil
	}
	if s.Kind != "图案" {
		if err := a.GUIKeyEventDevice(serial, 66); err != nil {
			return fmt.Errorf("提交解锁失败，已暂停自动解锁")
		}
	}
	for i := 0; i < 8; i++ {
		time.Sleep(400 * time.Millisecond)
		locked, err = a.deviceLocked(serial)
		if err == nil && !locked {
			return nil
		}
	}
	return fmt.Errorf("未确认解锁成功，已暂停自动解锁。请在手机查看结果，检查密码或图案位置后重新保存设置；不会自动重试")
}
