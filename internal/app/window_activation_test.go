package app

import (
	"strings"
	"testing"
	"time"
)

func TestActivationRetriesUntilWindowRegistration(t *testing.T) {
	calls, waits := 0, 0
	err := activateProcessWithRetry(42, func(pid int) bool {
		if pid != 42 {
			t.Fatal("wrong activation target")
		}
		calls++
		return calls == 3
	}, func(delay time.Duration) {
		waits++
		if delay != 250*time.Millisecond {
			t.Fatal("unexpected retry delay")
		}
	})
	if err != nil || calls != 3 || waits != 2 {
		t.Fatalf("calls=%d waits=%d err=%v", calls, waits, err)
	}
}

func TestActivationPersistentRejectionRemainsAnError(t *testing.T) {
	calls, waits := 0, 0
	err := activateProcessWithRetry(42, func(int) bool { calls++; return false }, func(time.Duration) { waits++ })
	if err == nil || !strings.Contains(err.Error(), "42") || calls != 9 || waits != 8 {
		t.Fatalf("calls=%d waits=%d err=%v", calls, waits, err)
	}
}

func TestActivationSuccessDoesNotWait(t *testing.T) {
	err := activateProcessWithRetry(42, func(int) bool { return true }, func(time.Duration) { t.Fatal("successful activation waited") })
	if err != nil {
		t.Fatal(err)
	}
}
