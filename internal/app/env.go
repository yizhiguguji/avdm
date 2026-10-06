package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// normalizeProcessEnv repairs the current process environment so that every
// subprocess the app launches — and every child of those subprocesses (for
// example scrcpy -> adb, or the macOS dependency installer -> brew) — can
// locate the tools it needs.
//
// GUI apps launched from Finder/Dock on macOS inherit a minimal PATH
// (/usr/bin:/bin:/usr/sbin:/sbin) that omits Homebrew (/opt/homebrew/bin) and
// the Android SDK. Fixing the process PATH once, at the root, means every
// current and future exec.Command inherits a correct environment instead of
// each call site having to remember to build its own cmd.Env.
//
// It is idempotent: directories already present in PATH are not re-added, so
// it is safe to call again after installing new dependencies.
func normalizeProcessEnv(resolver *ToolResolver) {
	if dirs := extraBinDirs(resolver); len(dirs) > 0 {
		os.Setenv("PATH", prependPath(os.Getenv("PATH"), dirs))
	}
	// Pin scrcpy's adb to the exact binary the app resolved. scrcpy honours the
	// ADB environment variable, and this guards against a missing or
	// version-mismatched adb making live mirrors exit immediately.
	if resolver != nil && strings.TrimSpace(os.Getenv("ADB")) == "" {
		if adb, err := resolver.Path(toolADB); err == nil {
			os.Setenv("ADB", adb)
		}
	}
}

// extraBinDirs collects directories that should be present in PATH: the
// directory of every resolved tool (adb/emulator/scrcpy and the SDK bins) plus
// the standard Homebrew locations so a fresh machine can still find brew for
// the installer even before any tool is resolved.
func extraBinDirs(resolver *ToolResolver) []string {
	var dirs []string
	if resolver != nil {
		for _, name := range allTools {
			if status := resolver.Status(name); status.Available && status.Path != "" {
				dirs = append(dirs, filepath.Dir(status.Path))
			}
		}
	}
	if runtime.GOOS == "darwin" {
		dirs = append(dirs, "/opt/homebrew/bin", "/usr/local/bin")
	}
	return uniqueExistingDirs(dirs)
}

func uniqueExistingDirs(dirs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			out = append(out, dir)
		}
	}
	return out
}

// prependPath prepends dirs (in order) to current, skipping any already
// present so the result stays stable across repeated calls.
func prependPath(current string, dirs []string) string {
	existing := map[string]bool{}
	for _, p := range filepath.SplitList(current) {
		if p != "" {
			existing[p] = true
		}
	}
	var prefix []string
	for _, dir := range dirs {
		if !existing[dir] {
			prefix = append(prefix, dir)
		}
	}
	if len(prefix) == 0 {
		return current
	}
	sep := string(os.PathListSeparator)
	if current == "" {
		return strings.Join(prefix, sep)
	}
	return strings.Join(prefix, sep) + sep + current
}
