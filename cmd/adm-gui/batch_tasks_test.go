package main

import (
	core "adm/internal/app"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestEntryBatchKeepsResultsAndLimitsConcurrency(t *testing.T) {
	entries := make([]core.DeviceEntry, 12)
	for i := range entries {
		entries[i].Key = string(rune('a' + i))
	}
	var active, peak atomic.Int32
	results := executeEntryBatch(entries, func(entry core.DeviceEntry) error {
		n := active.Add(1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		defer active.Add(-1)
		time.Sleep(time.Millisecond)
		if entry.Key == "b" {
			return errors.New("失败")
		}
		return nil
	})
	if peak.Load() > 4 {
		t.Fatalf("并发超限: %d", peak.Load())
	}
	for i, result := range results {
		if result.Entry.Key != entries[i].Key {
			t.Fatal("目标结果串台")
		}
	}
	if results[1].Err == nil || results[0].Err != nil {
		t.Fatal("未保留每个目标结果")
	}
}
