package main

import (
	core "adm/internal/app"
	"errors"
	"reflect"
	"sort"
	"sync"
	"testing"
)

func TestSelectedCloseExecutesOnlineOfflineAndMissingADBTargets(t *testing.T) {
	entries := []core.DeviceEntry{
		{Key: "online", Label: "在线模拟器", Running: true, AVD: &core.AVD{Name: "Online_AVD"}, Active: &core.ActiveDevice{Serial: "emulator-5554", State: "device", IsEmulator: true}},
		{Key: "starting", Label: "等待ADB", Running: true, AVD: &core.AVD{Name: "Starting_AVD"}},
		{Key: "offline", Label: "ADB离线", Running: true, AVD: &core.AVD{Name: "Offline_AVD"}, Active: &core.ActiveDevice{Serial: "emulator-5556", State: "offline", IsEmulator: true}},
	}
	plan := makeActionPlan(actionStop, TargetSelection{Entries: entries, Explicit: true}, entries, false)
	if len(plan.Entries) != 3 {
		t.Fatalf("missing close target: %+v", plan)
	}
	var mu sync.Mutex
	var calls []string
	failure := errors.New("进程未退出")
	closeAVD := func(name string) error {
		mu.Lock()
		calls = append(calls, "avd:"+name)
		mu.Unlock()
		if name == "Offline_AVD" {
			return failure
		}
		return nil
	}
	closeSerial := func(serial, confirmed string) error {
		if serial != confirmed {
			t.Errorf("confirmation does not match target: %q != %q", serial, confirmed)
		}
		mu.Lock()
		calls = append(calls, "serial:"+serial)
		mu.Unlock()
		return nil
	}
	// Exercise the real batch execution path. Merely building an action plan
	// did not catch the missing Active dereference in the previous implementation.
	results := executeEntryBatch(plan.Entries, func(entry core.DeviceEntry) error { return stopSelectedEmulator(entry, closeAVD, closeSerial) })
	sort.Strings(calls)
	want := []string{"avd:Offline_AVD", "avd:Starting_AVD", "serial:emulator-5554"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("wrong execution routes: %v", calls)
	}
	if len(results) != 3 || results[0].Err != nil || results[1].Err != nil || !errors.Is(results[2].Err, failure) {
		t.Fatalf("batch results lost target-specific errors: %+v", results)
	}
}

func TestSelectedCloseRejectsPhonesStoppedAndUnidentifiedTargets(t *testing.T) {
	for _, entry := range []core.DeviceEntry{
		{Label: "真机", Active: &core.ActiveDevice{Serial: "PHONE", State: "device"}},
		{Label: "网络真机", Active: &core.ActiveDevice{Serial: "192.0.2.1:5555", State: "device"}},
		{Label: "未启动", AVD: &core.AVD{Name: "Stopped_AVD"}},
		{Label: "无标识", Running: true, Active: &core.ActiveDevice{State: "offline", IsEmulator: true}},
	} {
		t.Run(entry.Label, func(t *testing.T) {
			called := false
			err := stopSelectedEmulator(entry, func(string) error { called = true; return nil }, func(string, string) error { called = true; return nil })
			if err == nil || called {
				t.Fatalf("unusable target invoked a close operation: called=%t err=%v", called, err)
			}
		})
	}
}

func TestSelectedCloseFallsBackToResolvedEmulatorIdentity(t *testing.T) {
	for _, tt := range []struct {
		entry core.DeviceEntry
		want  string
	}{
		{core.DeviceEntry{Active: &core.ActiveDevice{IsEmulator: true, State: "offline", AVDName: "Named_AVD", Serial: "emulator-5558"}}, "avd:Named_AVD"},
		{core.DeviceEntry{AVD: &core.AVD{Name: "Resolved_AVD"}, Active: &core.ActiveDevice{IsEmulator: true, State: "offline"}}, "avd:Resolved_AVD"},
		{core.DeviceEntry{Active: &core.ActiveDevice{IsEmulator: true, State: "offline", Serial: "emulator-5558"}}, "serial:emulator-5558"},
	} {
		got := ""
		err := stopSelectedEmulator(tt.entry, func(name string) error { got = "avd:" + name; return nil }, func(serial, confirmation string) error { got = "serial:" + serial; return nil })
		if err != nil || got != tt.want {
			t.Fatalf("unexpected fallback: got=%q want=%q err=%v", got, tt.want, err)
		}
	}
}

func TestManagementWindowActionsDescribeDeviceType(t *testing.T) {
	for _, tt := range []struct {
		name          string
		entry         core.DeviceEntry
		window, close string
		available     bool
	}{
		{"在线USB真机", core.DeviceEntry{Running: true, Active: &core.ActiveDevice{State: "device", Serial: "PHONE"}}, "打开独立窗", "关闭真机…", true},
		{"在线网络设备", core.DeviceEntry{Running: true, Active: &core.ActiveDevice{State: "device", Serial: "192.0.2.1:5555"}}, "打开独立窗", "断开网络设备…", true},
		{"在线模拟器", core.DeviceEntry{Running: true, Active: &core.ActiveDevice{State: "device", IsEmulator: true}}, "聚焦窗口", "关闭模拟器…", true},
		{"等待ADB模拟器", core.DeviceEntry{Running: true, AVD: &core.AVD{Name: "Starting_AVD"}}, "聚焦窗口", "关闭模拟器…", true},
		{"离线模拟器", core.DeviceEntry{Active: &core.ActiveDevice{State: "offline", IsEmulator: true}}, "聚焦窗口", "关闭模拟器…", true},
		{"未启动模拟器", core.DeviceEntry{AVD: &core.AVD{Name: "Stopped_AVD"}}, "", "", false},
		{"未授权真机", core.DeviceEntry{Active: &core.ActiveDevice{State: "unauthorized", Serial: "PHONE"}}, "", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			window, close, available := deviceManagementWindowActions(tt.entry)
			if window != tt.window || close != tt.close || available != tt.available {
				t.Fatalf("misleading device actions: %q %q %t", window, close, available)
			}
		})
	}
}
