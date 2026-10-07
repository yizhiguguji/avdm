//go:build darwin

package app

import (
	"math"
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
	originals := make(map[int]ExternalWindowFrame)
	for _, pid := range pids {
		frame, err := ReadProcessWindowFrame(pid)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("before pid=%d frame=%+v", pid, frame)
		originals[pid] = frame
	}
	if err := tileProcessWindows(pids, 0); err != nil {
		t.Fatalf("tile process windows: %v", err)
	}
	var common ExternalWindowFrame
	for i, pid := range pids {
		frame, err := ReadProcessWindowFrame(pid)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("after pid=%d frame=%+v", pid, frame)
		old := originals[pid]
		if math.Abs(frame.Width/(frame.Height-30)-old.Width/(old.Height-30)) > 0.005 {
			t.Fatalf("device content aspect changed: before=%+v after=%+v", old, frame)
		}
		if frame.Width <= 0 || frame.Height <= 0 {
			t.Fatalf("invalid frame for pid=%d: %+v", pid, frame)
		}
		if i == 0 {
			common = frame
		} else if math.Abs(frame.Height-common.Height) > 2 {
			t.Fatalf("nonuniform outer heights: first=%+v pid=%d frame=%+v", common, pid, frame)
		}
		if frame.Height > float64(scrcpyWindowHeight)+2 {
			t.Fatalf("oversized tile: %+v", frame)
		}
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

func TestLiveReadProcessWindowFrames(t *testing.T) {
	if os.Getenv("ADM_LIVE_READ_WINDOW_TEST") != "1" {
		t.Skip("set ADM_LIVE_READ_WINDOW_TEST=1 for read-only AX frame diagnostics")
	}
	pids := liveProcessWindowPIDs()
	if len(pids) == 0 {
		t.Fatal("set ADM_LIVE_PROCESS_PIDS")
	}
	for _, pid := range pids {
		frame, err := ReadProcessWindowFrame(pid)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("pid=%d frame=%+v", pid, frame)
	}
}

func TestUniformExternalTileSizeFitsWholeGrid(t *testing.T) {
	for _, tt := range []struct {
		name       string
		w, h       float64
		cols, rows int
		aux        float64
		full       bool
	}{
		{"two phones", 1200, 800, 2, 1, 0, true},
		{"mixed toolbar", 1200, 800, 2, 1, 61, true},
		{"short screen", 800, 500, 2, 1, 0, false},
		{"multiple rows", 1200, 800, 3, 2, 61, false},
		{"narrow screen", 300, 900, 2, 1, 61, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w, h := uniformExternalTileSize(tt.w, tt.h, tt.cols, tt.rows, tt.aux)
			if w <= 0 || h <= 0 || (w+tt.aux+6)*float64(tt.cols) > tt.w+1 || (h+6)*float64(tt.rows) > tt.h+1 {
				t.Fatalf("tile %.0fx%.0f does not fit grid %.0fx%.0f", w, h, tt.w, tt.h)
			}
			if math.Abs(w/h-float64(scrcpyWindowWidth)/float64(scrcpyWindowHeight)) > .012 {
				t.Fatalf("scaled tile changed ratio: %.0fx%.0f", w, h)
			}
			if tt.full && (w != float64(scrcpyWindowWidth) || h != float64(scrcpyWindowHeight)) {
				t.Fatalf("expected full compact default, got %.0fx%.0f", w, h)
			}
		})
	}
}

func TestLiveProcessWindowReuseRestoresUniformSize(t *testing.T) {
	if os.Getenv("ADM_LIVE_REUSE_SIZE_TEST") != "1" {
		t.Skip("set ADM_LIVE_REUSE_SIZE_TEST=1 to resize and re-tile existing windows")
	}
	pids := liveProcessWindowPIDs()
	if len(pids) < 2 {
		t.Fatal("set ADM_LIVE_PROCESS_PIDS to at least two existing mirrors")
	}
	for i, pid := range pids {
		if err := resizeProcessWindow(pid, 0, 540+i*60); err != nil {
			t.Fatalf("simulate manual resize for pid=%d: %v", pid, err)
		}
		frame, err := ReadProcessWindowFrame(pid)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("manually resized pid=%d frame=%+v", pid, frame)
	}
	var firstPass []ExternalWindowFrame
	for pass := 0; pass < 2; pass++ {
		if err := tileProcessWindows(pids, 0); err != nil {
			t.Fatalf("re-tile pass %d: %v", pass, err)
		}
		var common ExternalWindowFrame
		for i, pid := range pids {
			frame, err := ReadProcessWindowFrame(pid)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("pass=%d pid=%d frame=%+v", pass, pid, frame)
			if i == 0 {
				common = frame
			} else if math.Abs(frame.Height-common.Height) > 2 {
				t.Fatalf("nonuniform height after reuse: %+v versus %+v", common, frame)
			}
			if pass == 0 {
				firstPass = append(firstPass, frame)
			} else if math.Abs(frame.Width-firstPass[i].Width) > 2 || math.Abs(frame.Height-firstPass[i].Height) > 2 {
				t.Fatalf("size drift across repeated tiling: %+v versus %+v", firstPass[i], frame)
			}
		}
	}
}
