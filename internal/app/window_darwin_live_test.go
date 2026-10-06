//go:build darwin

package app

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestLiveNativeEmulatorWindowControl(t *testing.T) {
	if os.Getenv("ADM_LIVE_WINDOW_TEST") != "1" {
		t.Skip("set ADM_LIVE_WINDOW_TEST=1 to exercise native emulator window control")
	}
	targets := liveWindowTargets()
	if len(targets) == 0 {
		t.Fatal("set ADM_LIVE_TARGETS or ADM_LIVE_AVD/ADM_LIVE_SERIAL")
	}
	avdName := targets[0].AVDName
	serial := targets[0].Serial
	if avdName == "" && serial == "" {
		t.Fatal("set ADM_LIVE_AVD or ADM_LIVE_SERIAL")
	}
	if err := focusEmulatorWindow(avdName, serial); err != nil {
		t.Fatalf("focus emulator window: %v", err)
	}
	if err := tileEmulatorWindows(targets, 0); err != nil {
		t.Fatalf("tile emulator window: %v", err)
	}
}

func TestLiveNativeProcessWindowTile(t *testing.T) {
	if os.Getenv("ADM_LIVE_PROCESS_WINDOW_TEST") != "1" {
		t.Skip("set ADM_LIVE_PROCESS_WINDOW_TEST=1 to exercise mixed native window tiling")
	}
	pids := liveProcessWindowPIDs()
	if len(pids) == 0 {
		t.Fatal("set ADM_LIVE_PROCESS_PIDS")
	}
	if err := tileProcessWindows(pids, 0); err != nil {
		t.Fatalf("tile process windows: %v", err)
	}
}

func TestCommandLineHasAVD(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		avd    string
		want   bool
	}{
		{
			name:   "direct emulator at argument",
			fields: []string{"/opt/android/emulator/qemu-system-aarch64", "@demo_5050", "-port", "5554"},
			avd:    "demo_5050",
			want:   true,
		},
		{
			name:   "dash avd argument",
			fields: []string{"/opt/android/emulator/emulator", "-avd", "demo_5051", "-port", "5556"},
			avd:    "demo_5051",
			want:   true,
		},
		{
			name:   "different avd",
			fields: []string{"/opt/android/emulator/qemu-system-aarch64", "@demo_5052", "-port", "5558"},
			avd:    "demo_5050",
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commandLineHasAVD(tt.fields, tt.avd); got != tt.want {
				t.Fatalf("commandLineHasAVD() = %v, want %v", got, tt.want)
			}
		})
	}
}

func liveWindowTargets() []emulatorWindowTarget {
	rawTargets := strings.TrimSpace(os.Getenv("ADM_LIVE_TARGETS"))
	if rawTargets == "" {
		return []emulatorWindowTarget{{
			AVDName: os.Getenv("ADM_LIVE_AVD"),
			Serial:  os.Getenv("ADM_LIVE_SERIAL"),
		}}
	}
	var targets []emulatorWindowTarget
	for _, raw := range strings.Split(rawTargets, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, "|", 2)
		target := emulatorWindowTarget{AVDName: strings.TrimSpace(parts[0])}
		if len(parts) == 2 {
			target.Serial = strings.TrimSpace(parts[1])
		}
		targets = append(targets, target)
	}
	return targets
}

func liveProcessWindowPIDs() []int {
	rawPIDs := strings.TrimSpace(os.Getenv("ADM_LIVE_PROCESS_PIDS"))
	if rawPIDs == "" {
		return nil
	}
	var pids []int
	for _, raw := range strings.Split(rawPIDs, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		pid, err := strconv.Atoi(raw)
		if err != nil || pid <= 0 {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}
