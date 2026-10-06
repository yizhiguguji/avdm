package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const macOSDepsScriptName = "install-macos-deps.sh"

type DependencyBootstrapStatus struct {
	Supported bool
	Script    string
	Error     string
}

func (a *App) GUIDependencyBootstrapStatus() DependencyBootstrapStatus {
	if runtime.GOOS != "darwin" {
		return DependencyBootstrapStatus{Supported: false, Error: "当前只支持 macOS 依赖安装向导"}
	}
	script, err := findMacOSDepsScript()
	if err != nil {
		return DependencyBootstrapStatus{Supported: false, Error: err.Error()}
	}
	return DependencyBootstrapStatus{Supported: true, Script: script}
}

func (a *App) GUIInstallMacOSDependencies() (string, error) {
	status := a.GUIDependencyBootstrapStatus()
	if !status.Supported {
		return "", errors.New(status.Error)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/bash", status.Script)
	cmd.Dir = filepath.Dir(status.Script)
	cmd.Env = os.Environ()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	text := strings.TrimSpace(out.String())
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return text, errors.New("依赖安装超时")
	}
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("依赖安装失败：%s", firstLine(text))
	}
	a.tools.Refresh()
	normalizeProcessEnv(a.tools)
	return text, nil
}

func findMacOSDepsScript() (string, error) {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		exe = filepath.Clean(exe)
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exe), "..", "Resources", macOSDepsScriptName),
			filepath.Join(filepath.Dir(exe), "..", "..", "..", "scripts", macOSDepsScriptName),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "scripts", macOSDepsScriptName),
			filepath.Join(cwd, "..", "scripts", macOSDepsScriptName),
		)
	}

	for _, candidate := range uniqueStrings(candidates) {
		path := filepath.Clean(candidate)
		if isExecutable(path) {
			return path, nil
		}
	}
	return "", errors.New("未找到依赖安装脚本；请确认 App 包含 Contents/Resources/install-macos-deps.sh，或在源码目录运行 make deps-macos")
}
