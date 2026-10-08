package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestFallbackFrameFailuresStillOpenAndArrangeHealthyPhones(t *testing.T) {
	readErr := errors.New("device disconnected during size query")
	for _, failedKey := range []string{"one", "two", "three"} {
		t.Run(failedKey, func(t *testing.T) {
			keys := []string{"one", "two", "three", "one"}
			var entries []DeviceEntry
			var want []string
			for _, key := range keys[:3] {
				entries = append(entries, DeviceEntry{Key: key, Active: &ActiveDevice{Serial: key, State: "device"}})
				if key != failedKey {
					want = append(want, key)
				}
			}
			var read []string
			plan := planScrcpyFallback(keys, entries, func(entry DeviceEntry) (ExternalWindowFrame, error) {
				read = append(read, entry.Key)
				if entry.Key == failedKey {
					return ExternalWindowFrame{}, readErr
				}
				return ExternalWindowFrame{Width: 266, Height: 624}, nil
			}, func(frames []ExternalWindowFrame) []*scrcpyWindowPlacement {
				if len(frames) != 2 {
					t.Fatalf("failed target reserved a layout slot: %+v", frames)
				}
				return []*scrcpyWindowPlacement{{X: 6, Y: 42, Width: 266, Height: 624}, {X: 278, Y: 42, Width: 266, Height: 624}}
			})
			var opened, tiled []string
			err := openLiveMirrors(keys, func(key string) error {
				return plan.open(key, func(key string, placement *scrcpyWindowPlacement) error {
					if placement == nil || placement.X != 6+272*len(opened) {
						t.Fatalf("healthy target lost compact placement: %s %+v", key, placement)
					}
					opened = append(opened, key)
					return nil
				})
			}, func(keys []string) error { tiled = keys; return nil })
			if !reflect.DeepEqual(read, keys[:3]) || !reflect.DeepEqual(opened, want) || !reflect.DeepEqual(tiled, want) {
				t.Fatalf("read=%v opened=%v tiled=%v want=%v", read, opened, tiled, want)
			}
			if !errors.Is(err, readErr) || !strings.Contains(err.Error(), failedKey) || !strings.Contains(err.Error(), "已打开 2") {
				t.Fatalf("partial failure lost count, target or cause: %v", err)
			}
		})
	}
}

func TestFallbackAllFrameFailuresNeverOpenOrArrange(t *testing.T) {
	failure := errors.New("invalid display size")
	plan := planScrcpyFallback([]string{"phone"}, []DeviceEntry{{Key: "phone", Active: &ActiveDevice{State: "device"}}}, func(DeviceEntry) (ExternalWindowFrame, error) {
		return ExternalWindowFrame{}, failure
	}, func([]ExternalWindowFrame) []*scrcpyWindowPlacement {
		t.Fatal("arranged failed targets")
		return nil
	})
	err := openLiveMirrors([]string{"phone"}, func(key string) error {
		return plan.open(key, func(string, *scrcpyWindowPlacement) error { t.Fatal("opened failed target"); return nil })
	}, func([]string) error { t.Fatal("tiled failed target"); return nil })
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "已打开 0") {
		t.Fatalf("all-failed summary lost failure: %v", err)
	}
}

func TestFallbackRetainsEmulatorPermissionRecovery(t *testing.T) {
	entries := []DeviceEntry{{Key: "emulator", AVD: &AVD{Name: "test-avd"}, Running: true}}
	plan := planScrcpyFallback([]string{"emulator"}, entries, func(DeviceEntry) (ExternalWindowFrame, error) {
		t.Fatal("queried phone size for emulator")
		return ExternalWindowFrame{}, nil
	}, func([]ExternalWindowFrame) []*scrcpyWindowPlacement {
		t.Fatal("planned phone layout for emulator")
		return nil
	})
	err := plan.open("emulator", func(_ string, placement *scrcpyWindowPlacement) error {
		a := &App{}
		return a.openLiveMirrorEntry(entries[0], nil, func(string, string) error { t.Fatal("focused emulator without arrangement permission"); return nil }, placement)
	})
	if !IsAccessibilityPermissionRequired(err) {
		t.Fatalf("lost emulator permission recovery: %v", err)
	}
}
