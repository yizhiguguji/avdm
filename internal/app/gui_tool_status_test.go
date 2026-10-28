package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGUIStateRetainsToolResultsWhenDeviceQueriesFail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool")
	// Let avdmanager pass validation once, then fail the device query. A
	// validation failure would fall back to tools installed on the host.
	script := "#!/bin/sh\ncase \"$1\" in\nlist) if [ ! -f \"$0.validated\" ]; then touch \"$0.validated\"; exit 0; fi; echo 'query failed' >&2; exit 1;;\ndevices) echo 'query failed' >&2; exit 1;;\nesac\nexit 0\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := defaultConfig()
	for _, name := range allTools {
		cfg.ToolPaths[name] = path
	}
	a := &App{cfg: cfg, tools: &ToolResolver{cfg: cfg, statuses: map[string]ToolStatus{}}}
	state, err := a.GUIState()
	if err == nil {
		t.Fatal("expected device query failure")
	}
	if len(state.Tools) != len(allTools)+1 {
		t.Fatalf("lost tool diagnostics: %+v", state.Tools)
	}
	for _, status := range state.Tools[:len(allTools)] {
		if !status.Available || status.Path != path {
			t.Fatalf("tool was not refreshed before querying devices: %+v", status)
		}
	}
}
