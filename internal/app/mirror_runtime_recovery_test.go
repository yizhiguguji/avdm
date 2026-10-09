package app

import (
	"os/exec"
	"testing"
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
