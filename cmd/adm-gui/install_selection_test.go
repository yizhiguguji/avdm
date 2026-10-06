package main

import (
	core "adm/internal/app"
	"reflect"
	"testing"
)

func TestInstallUsesCheckedDevicesBeforeCurrentTarget(t *testing.T) {
	entries := []core.DeviceEntry{
		{Key: "a", Active: &core.ActiveDevice{Serial: "one", State: "device"}},
		{Key: "b", Active: &core.ActiveDevice{Serial: "two", State: "device"}},
		{Key: "c", Active: &core.ActiveDevice{Serial: "three", State: "offline"}},
	}
	for _, tt := range []struct {
		name     string
		selected map[string]bool
		want     []string
		fail     bool
	}{
		{"multiple", map[string]bool{"a": true, "b": true}, []string{"one", "two"}, false},
		{"only_checked", map[string]bool{"b": true}, []string{"two"}, false},
		{"current_fallback", nil, []string{"one"}, false},
		{"offline_not_silently_dropped", map[string]bool{"a": true, "c": true}, nil, true},
		{"disappeared_not_current", map[string]bool{"missing": true}, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := installSelectionSerials(entries, tt.selected, "a")
			if (err != nil) != tt.fail || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
}
