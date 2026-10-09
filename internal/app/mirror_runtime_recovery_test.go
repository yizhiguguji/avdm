package app

import (
	"os/exec"
	"testing"
	"time"
)

func TestRuntimeRecoveryOnlyRestoresUnexpectedDisconnects(t *testing.T) {
	exit := func(code string) error { return exec.Command("sh", "-c", "exit "+code).Run() }
	disconnected, startFailure := exit("2"), exit("1")
	for _, row := range []struct {
		err             error
		opened, stopped bool
		attempts        int
		want            bool
	}{
		{disconnected, true, false, 0, true},
		{disconnected, true, false, 2, true},
		{disconnected, true, false, 3, false},
		{disconnected, true, true, 0, false},
		{disconnected, false, false, 0, false},
		{startFailure, true, false, 0, false},
		{nil, true, false, 0, false},
	} {
		if got := recoverDisconnectedMirror(row.err, row.opened, row.stopped, row.attempts); got != row.want {
			t.Fatalf("unexpected recovery decision: %+v got=%v", row, got)
		}
	}
}

func TestStableMirrorResetsConsecutiveFailureBudget(t *testing.T) {
	now := time.Now()
	for _, row := range []struct {
		openedAt time.Time
		want     int
	}{
		{time.Time{}, 3},
		{now.Add(-59 * time.Second), 3},
		{now.Add(-time.Minute), 0},
		{now.Add(-17 * time.Minute), 0},
	} {
		attempts := consecutiveMirrorRecoveryAttempts(3, row.openedAt, now)
		if attempts != row.want {
			t.Fatalf("openedAt=%v: got %d, want %d", row.openedAt, attempts, row.want)
		}
		if got := recoverDisconnectedMirror(exec.Command("sh", "-c", "exit 2").Run(), true, false, attempts); got != (row.want == 0) {
			t.Fatalf("unexpected recovery eligibility after stable-period reset: %v", got)
		}
	}
}
