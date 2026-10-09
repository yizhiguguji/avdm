package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// The caller owns this exact child process and its Wait goroutine. Never use
// this path for emulator processes or mirrors launched by another application.
func stopManagedScrcpyProcess(process *os.Process, done <-chan struct{}, grace time.Duration) error {
	select {
	case <-done:
		return nil
	default:
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("关闭旧独立窗失败：%w", err)
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("回收旧独立窗失败：%w", err)
	}
	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("旧独立窗强制回收后仍未退出")
	}
}

func transientScrcpyStartupFailure(err error) bool {
	var exited *scrcpyStartupExitError
	if !errors.As(err, &exited) {
		return false
	}
	for _, message := range []string{"Server connection failed", "Could not retrieve device information", "device offline", "protocol fault", "Demuxer error", "Device disconnected"} {
		if strings.Contains(err.Error(), message) {
			return true
		}
	}
	return false
}

func (a *App) startScrcpySession(serial, title string, alwaysOnTop bool, placements ...*scrcpyWindowPlacement) error {
	return retryScrcpyStartup(func() error { return a.startScrcpySessionOnce(serial, title, alwaysOnTop, placements...) }, func() error { return a.waitMirrorADBReady(serial) })
}

func retryScrcpyStartup(start func() error, ready func() error) error {
	first := start()
	if !transientScrcpyStartupFailure(first) {
		return first
	}
	if err := ready(); err != nil {
		return fmt.Errorf("镜像连接失败，等待设备恢复失败：%w\n首次失败：%v", err, first)
	}
	if err := start(); err != nil {
		return fmt.Errorf("镜像连接恢复后重试仍失败：%w", err)
	}
	return nil
}

func (a *App) waitMirrorADBReady(serial string) error {
	deadline := time.Now().Add(8 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		// An ADB listing can say device before shell/service requests work.
		output, err := a.runToolOutput(toolADB, 2*time.Second, "-s", serial, "shell", "echo", "avdm-ready")
		if err == nil && strings.TrimSpace(output) == "avdm-ready" {
			return nil
		}
		last = err
		if last == nil {
			last = fmt.Errorf("设备尚未响应连接检查")
		}
		time.Sleep(250 * time.Millisecond)
	}
	return last
}
