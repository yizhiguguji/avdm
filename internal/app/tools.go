package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	toolADB        = "adb"
	toolEmulator   = "emulator"
	toolAVDManager = "avdmanager"
	toolSDKManager = "sdkmanager"
	toolScrcpy     = "scrcpy"
)

var allTools = []string{toolADB, toolEmulator, toolAVDManager, toolSDKManager, toolScrcpy}

type ToolStatus struct {
	Name      string
	Path      string
	Source    string
	Available bool
	Error     string
}

type ToolResolver struct {
	cfg      *Config
	mu       sync.RWMutex
	statuses map[string]ToolStatus
}

func newToolResolver(cfg *Config) *ToolResolver {
	r := &ToolResolver{cfg: cfg, statuses: map[string]ToolStatus{}}
	r.Refresh()
	return r
}

func (r *ToolResolver) Refresh() {
	statuses := map[string]ToolStatus{}
	for _, name := range allTools {
		statuses[name] = r.resolve(name)
	}
	r.mu.Lock()
	r.statuses = statuses
	r.mu.Unlock()
}

func (r *ToolResolver) Status(name string) ToolStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if status, ok := r.statuses[name]; ok {
		return status
	}
	return ToolStatus{Name: name, Error: "未探测"}
}

func (r *ToolResolver) Path(name string) (string, error) {
	status := r.Status(name)
	if !status.Available {
		if status.Error == "" {
			status.Error = "未找到可调用工具"
		}
		return "", fmt.Errorf("%s 不可用：%s", name, status.Error)
	}
	return status.Path, nil
}

func (r *ToolResolver) resolve(name string) ToolStatus {
	candidates := r.candidates(name)
	seen := map[string]bool{}
	lastError := ""
	for _, candidate := range candidates {
		if candidate.Path == "" {
			continue
		}
		path := expandPath(candidate.Path)
		if seen[path] {
			continue
		}
		seen[path] = true
		if !isExecutable(path) {
			continue
		}
		if err := validateTool(name, path); err != nil {
			lastError = fmt.Sprintf("%s (%s): %s", path, candidate.Source, err.Error())
			continue
		}
		return ToolStatus{Name: name, Path: path, Source: candidate.Source, Available: true}
	}
	if lastError != "" {
		return ToolStatus{Name: name, Error: "候选路径不可调用：" + lastError}
	}
	return ToolStatus{Name: name, Error: "未找到可执行文件"}
}

type toolCandidate struct {
	Path   string
	Source string
}

func (r *ToolResolver) candidates(name string) []toolCandidate {
	var out []toolCandidate
	if p := strings.TrimSpace(r.cfg.ToolPaths[name]); p != "" {
		out = append(out, toolCandidate{Path: p, Source: "配置"})
	}

	for _, root := range androidSDKRoots() {
		for _, p := range toolPathsInSDK(root, name) {
			out = append(out, toolCandidate{Path: p, Source: "Android SDK"})
		}
	}

	if p, err := exec.LookPath(name); err == nil {
		out = append(out, toolCandidate{Path: p, Source: "PATH"})
	}

	if runtime.GOOS == "darwin" {
		for _, p := range darwinFallbackPaths(name) {
			out = append(out, toolCandidate{Path: p, Source: "常见路径"})
		}
	}
	return out
}

func androidSDKRoots() []string {
	var roots []string
	for _, key := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			roots = append(roots, v)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, "Library", "Android", "sdk"))
		roots = append(roots, filepath.Join(home, "Android", "Sdk"))
	}
	roots = append(roots,
		"/opt/android-sdk",
		"/usr/local/share/android-sdk",
		"/opt/homebrew/share/android-sdk",
		"/opt/homebrew/share/android-commandlinetools",
	)
	return uniqueStrings(roots)
}

func toolPathsInSDK(root, name string) []string {
	switch name {
	case toolADB:
		return []string{filepath.Join(root, "platform-tools", executableName("adb"))}
	case toolEmulator:
		return []string{filepath.Join(root, "emulator", executableName("emulator"))}
	case toolAVDManager:
		return []string{
			filepath.Join(root, "cmdline-tools", "latest", "bin", executableName("avdmanager")),
			filepath.Join(root, "cmdline-tools", "bin", executableName("avdmanager")),
			filepath.Join(root, "tools", "bin", executableName("avdmanager")),
		}
	case toolSDKManager:
		return []string{
			filepath.Join(root, "cmdline-tools", "latest", "bin", executableName("sdkmanager")),
			filepath.Join(root, "cmdline-tools", "bin", executableName("sdkmanager")),
			filepath.Join(root, "tools", "bin", executableName("sdkmanager")),
		}
	default:
		return nil
	}
}

func darwinFallbackPaths(name string) []string {
	return []string{
		filepath.Join("/opt/homebrew/bin", executableName(name)),
		filepath.Join("/usr/local/bin", executableName(name)),
	}
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func validateTool(name, path string) error {
	args := []string{"--version"}
	switch name {
	case toolADB:
		args = []string{"version"}
	case toolEmulator:
		args = []string{"-version"}
	case toolAVDManager:
		args = []string{"list", "avd"}
	case toolSDKManager:
		args = []string{"--version"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.New("调用超时")
		}
		text := strings.TrimSpace(string(out))
		if text == "" {
			text = err.Error()
		}
		return errors.New(firstLine(text))
	}
	return nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func expandPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "\"'")
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return os.ExpandEnv(path)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
