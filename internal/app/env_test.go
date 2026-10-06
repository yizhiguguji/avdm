package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrependPathAddsMissingDirsAndKeepsOrder(t *testing.T) {
	current := "/usr/bin:/bin"
	got := prependPath(current, []string{"/opt/homebrew/bin", "/usr/local/bin"})
	want := "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"
	if got != want {
		t.Fatalf("prependPath = %q, want %q", got, want)
	}
}

func TestPrependPathIsIdempotent(t *testing.T) {
	current := "/opt/homebrew/bin:/usr/bin:/bin"
	// /opt/homebrew/bin already present, only /usr/local/bin is new.
	got := prependPath(current, []string{"/opt/homebrew/bin", "/usr/local/bin"})
	want := "/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin"
	if got != want {
		t.Fatalf("prependPath = %q, want %q", got, want)
	}
	// Running again with the same dirs must not change anything.
	if again := prependPath(got, []string{"/opt/homebrew/bin", "/usr/local/bin"}); again != got {
		t.Fatalf("prependPath not idempotent: %q -> %q", got, again)
	}
}

func TestUniqueExistingDirsFiltersMissingAndDuplicates(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")
	got := uniqueExistingDirs([]string{dir, dir, missing, ""})
	if len(got) != 1 || got[0] != dir {
		t.Fatalf("uniqueExistingDirs = %v, want [%s]", got, dir)
	}
}

func TestNormalizeProcessEnvRepairsMinimalPATH(t *testing.T) {
	// Simulate a Finder-launched GUI app: a minimal PATH plus a resolvable
	// tool living in a directory that is NOT on that PATH.
	toolDir := t.TempDir()
	adbPath := filepath.Join(toolDir, executableName(toolADB))
	if err := os.WriteFile(adbPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write fake adb: %v", err)
	}

	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("ADB", "")

	resolver := &ToolResolver{statuses: map[string]ToolStatus{
		toolADB: {Name: toolADB, Path: adbPath, Available: true},
	}}

	normalizeProcessEnv(resolver)

	pathDirs := filepath.SplitList(os.Getenv("PATH"))
	if !containsDir(pathDirs, toolDir) {
		t.Fatalf("resolved tool dir %s not added to PATH: %v", toolDir, pathDirs)
	}
	if got := os.Getenv("ADB"); got != adbPath {
		t.Fatalf("ADB = %q, want %q", got, adbPath)
	}
}

func TestNormalizeProcessEnvKeepsExistingADB(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("ADB", "/custom/adb")

	resolver := &ToolResolver{statuses: map[string]ToolStatus{
		toolADB: {Name: toolADB, Path: "/opt/homebrew/bin/adb", Available: true},
	}}

	normalizeProcessEnv(resolver)

	if got := os.Getenv("ADB"); got != "/custom/adb" {
		t.Fatalf("ADB overwritten: got %q, want /custom/adb", got)
	}
}

func containsDir(dirs []string, target string) bool {
	for _, d := range dirs {
		if strings.TrimSpace(d) == target {
			return true
		}
	}
	return false
}
