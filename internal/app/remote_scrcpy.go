package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	scrcpyWindowWidth  = 288
	scrcpyWindowHeight = 624
)

type scrcpySession struct {
	AlwaysOnTop bool
	Done        chan struct{}
	Serial      string
	Title       string
	Cmd         *exec.Cmd
	LogPath     string
	StartedAt   time.Time
	Starting    bool
}

type scrcpyWindowPlacement struct {
	X, Y, Width, Height int
}

// startScrcpySession is called with scrcpyLaunchMu held by openLiveMirror.
func (a *App) startScrcpySession(serial, title string, alwaysOnTop bool, placements ...*scrcpyWindowPlacement) error {
	var placement *scrcpyWindowPlacement
	if len(placements) > 0 {
		placement = placements[0]
	}
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return fmt.Errorf("设备 serial 不能为空")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = serial
	}
	path, err := a.tools.Path(toolScrcpy)
	if err != nil {
		return err
	}
	windowTitle := fmt.Sprintf("安卓设备矩阵 - %s | %s", title, serial)

	a.remoteMu.Lock()
	existing := a.scrcpySessions[serial]
	a.remoteMu.Unlock()
	if existing != nil && existing.Cmd != nil && existing.Cmd.Process != nil {
		select {
		case <-existing.Done:
		default:
			if canReuseScrcpyWindow(strings.Join(existing.Cmd.Args, " "), alwaysOnTop, placement) {
				return prepareScrcpyWindow(existing.Cmd.Process.Pid)
			}
			if err := existing.Cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return fmt.Errorf("关闭旧独立窗失败：%w", err)
			}
			select {
			case <-existing.Done:
			case <-time.After(3 * time.Second):
				return fmt.Errorf("旧独立窗未退出，请关闭后重试")
			}
		}
	}
	if pid, command, ok := runningScrcpyProcessInfo(serial); ok {
		if canReuseScrcpyWindow(command, alwaysOnTop, placement) {
			return prepareScrcpyWindow(pid)
		}
		if !strings.Contains(command, "--window-title=安卓设备矩阵 - ") {
			return fmt.Errorf("该设备已有其他程序打开的镜像，请先关闭该窗口，再应用窗口尺寸或置顶设置")
		}
		if err := stopPreviousScrcpyWindow(pid); err != nil {
			return err
		}
	}
	session := &scrcpySession{
		AlwaysOnTop: alwaysOnTop,
		Done:        make(chan struct{}),
		Serial:      serial,
		Title:       title,
		StartedAt:   time.Now(),
		Starting:    true,
	}
	a.remoteMu.Lock()
	if a.scrcpySessions == nil {
		a.scrcpySessions = map[string]*scrcpySession{}
	}
	a.scrcpySessions[serial] = session
	a.remoteMu.Unlock()

	logFile, err := ensureLogFile("scrcpy_" + serial)
	if err != nil {
		a.clearScrcpySession(serial, session)
		return err
	}
	logPath := logFile.Name()
	args := scrcpyArgs(serial, windowTitle, alwaysOnTop)
	if placement != nil {
		args = scrcpyPlacementArgs(args, placement)
	}
	cmd := exec.Command(path, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		a.clearScrcpySession(serial, session)
		return fmt.Errorf("启动实时镜像失败：%w\n日志：%s", err, logPath)
	}

	a.remoteMu.Lock()
	if current := a.scrcpySessions[serial]; current == session {
		session.Cmd = cmd
		session.LogPath = logPath
		session.Starting = false
	}
	a.remoteMu.Unlock()

	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		_ = logFile.Close()
		a.clearScrcpySession(serial, session)
		close(session.Done)
		done <- err
	}()

	select {
	case err := <-done:
		excerpt := scrcpyLogExcerpt(logPath)
		if err != nil {
			return fmt.Errorf("实时镜像启动后退出：%w\n日志：%s%s", err, logPath, excerpt)
		}
		return fmt.Errorf("实时镜像启动后立即退出\n日志：%s%s", logPath, excerpt)
	case <-time.After(700 * time.Millisecond):
		if placement != nil {
			if err := verifyScrcpyWindowPlacementNative(cmd.Process.Pid, placement); err != nil {
				return fmt.Errorf("实时镜像已启动，但窗口排列未生效：%w", err)
			}
		}
		return prepareScrcpyWindow(cmd.Process.Pid)
	}
}

func canReuseScrcpyWindow(command string, alwaysOnTop bool, placement *scrcpyWindowPlacement) bool {
	// Launch arguments cannot reveal whether the user moved the current window.
	// A requested placement must recreate our mirror when AX movement is unavailable.
	return placement == nil && scrcpyCommandAlwaysOnTop(command) == alwaysOnTop && scrcpyCommandUniformGeometry(command)
}

func (a *App) clearScrcpySession(serial string, session *scrcpySession) {
	a.remoteMu.Lock()
	defer a.remoteMu.Unlock()
	if current := a.scrcpySessions[serial]; current == session {
		delete(a.scrcpySessions, serial)
	}
}

func scrcpyArgs(serial, windowTitle string, alwaysOnTop bool) []string {
	args := []string{
		"--serial=" + serial,
		"--max-size=1920",
		"--max-fps=60",
		"--video-codec=h264",
		"--no-audio",
		"--render-fit=letterbox",
		"--keyboard=uhid",
		"--window-title=" + windowTitle,
		"--window-width=0",
		fmt.Sprintf("--window-height=%d", scrcpyWindowHeight-30),
		"--no-clipboard-autosync",
		"--keep-active",
		"--stay-awake",
	}
	if alwaysOnTop {
		args = append(args, "--always-on-top")
	}
	return args
}

func runningScrcpyProcess(serial string) (int, bool) {
	pid, _, ok := runningScrcpyProcessInfo(serial)
	return pid, ok
}

func runningScrcpyProcessInfo(serial string) (int, string, bool) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return 0, "", false
	}
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return 0, "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !looksLikeScrcpyCommand(line) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		command := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		if scrcpyCommandUsesSerial(command, serial) {
			return pid, command, true
		}
	}
	return 0, "", false
}

func looksLikeScrcpyCommand(line string) bool {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return false
	}
	name := fields[1]
	return strings.HasSuffix(name, "/scrcpy") || name == "scrcpy" || strings.HasSuffix(name, "/scrcpy.exe") || name == "scrcpy.exe"
}

func scrcpyCommandUsesSerial(command, serial string) bool {
	fields := strings.Fields(command)
	for i, field := range fields {
		if field == "--serial="+serial {
			return true
		}
		if (field == "--serial" || field == "-s") && i+1 < len(fields) && fields[i+1] == serial {
			return true
		}
	}
	return false
}

func scrcpyCommandAlwaysOnTop(command string) bool {
	return slices.Contains(strings.Fields(command), "--always-on-top")
}

func stopPreviousScrcpyWindow(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	defer process.Release()
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("关闭旧独立窗失败：%w", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "pid=").Run(); err != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Some SDL mirrors ignore SIGTERM. Only force-close an app-owned
	// scrcpy still associated with this PID; never kill a foreign process.
	command, readErr := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if readErr != nil {
		return nil
	}
	if !looksLikeScrcpyCommand(strconv.Itoa(pid)+" "+string(command)) || !strings.Contains(string(command), "--window-title=安卓设备矩阵 - ") {
		return fmt.Errorf("旧独立窗未退出，请关闭后重试")
	}
	if err := process.Kill(); err != nil {
		return fmt.Errorf("关闭旧独立窗失败：%w", err)
	}
	for attempt := 0; attempt < 20; attempt++ {
		if exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "pid=").Run() != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("旧独立窗未退出，请关闭后重试")
}

func scrcpyLogExcerpt(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return ""
	}
	const maxBytes = 4096
	if stat.Size() > maxBytes {
		if _, err := file.Seek(stat.Size()-maxBytes, io.SeekStart); err != nil {
			return ""
		}
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	return "\n最近日志：\n" + strings.Join(lines, "\n")
}

func prepareScrcpyWindow(pid int) error {
	return prepareScrcpyWindowWithOps(pid, !nativeWindowAccessUnavailable(), resizeProcessWindow, focusProcessWindow)
}

func prepareScrcpyWindowWithOps(pid int, canResize bool, resize func(int, int, int) error, focus func(int) error) error {
	if canResize {
		if err := resize(pid, 0, scrcpyWindowHeight); err != nil {
			return fmt.Errorf("实时镜像已打开，但统一窗口尺寸失败：%w", err)
		}
	}
	if err := focus(pid); err != nil {
		return fmt.Errorf("实时镜像已打开，但聚焦窗口失败：%w", err)
	}
	return nil
}

// Replace app-owned fixed-width mirrors with natural-aspect windows.
// Keep a common height while allowing each device its own width.
func scrcpyCommandUniformGeometry(command string) bool {
	fields := strings.Fields(command)
	return !slices.Contains(fields, "--no-window-aspect-ratio-lock") && slices.Contains(fields, "--window-width=0") && slices.Contains(fields, "--render-fit=letterbox")
}

// scrcpy can place its own new window without Accessibility permission.
// Use separate slots while preserving the natural aspect ratio set above.
func scrcpyPlacementArgs(args []string, placement *scrcpyWindowPlacement) []string {
	result := make([]string, 0, len(args)+2)
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--window-height=") {
			result = append(result, arg)
		}
	}
	// SDL positions the content; the measured macOS outer frame starts 32 points above it.
	return append(result, fmt.Sprintf("--window-x=%d", placement.X), fmt.Sprintf("--window-y=%d", placement.Y+32), fmt.Sprintf("--window-height=%d", placement.Height-32))
}
