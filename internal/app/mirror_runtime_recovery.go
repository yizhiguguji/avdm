package app

import (
	"errors"
	"os/exec"
	"strings"
	"time"
)

// scrcpy documents exit code 2 as device disconnect; 0 is a user close and 1
// is a startup failure. Recover only an established app-owned emulator mirror.
func recoverDisconnectedMirror(err error, opened, stopped bool, attempts int) bool {
	var status *exec.ExitError
	return opened && !stopped && attempts < 3 && errors.As(err, &status) && status.ExitCode() == 2
}

func (a *App) recoverDisconnectedSession(session *scrcpySession) {
	if !strings.HasPrefix(session.Serial, "emulator-") {
		a.clearScrcpySession(session.Serial, session)
		return
	}
	time.Sleep(time.Second)
	readyErr := a.waitMirrorADBReady(session.Serial)
	a.scrcpyLaunchMu.Lock()
	defer a.scrcpyLaunchMu.Unlock()
	a.remoteMu.Lock()
	current := a.scrcpySessions[session.Serial] == session && !session.Stopped
	a.remoteMu.Unlock()
	if !current {
		return
	} // a newer user action already replaced this session
	if readyErr != nil {
		a.clearScrcpySession(session.Serial, session)
		return
	}
	err := retryScrcpyStartup(func() error {
		return a.startScrcpySessionWithRecovery(session.Serial, session.Title, session.AlwaysOnTop, session.RecoveryAttempts+1, session.Placement)
	}, func() error { return a.waitMirrorADBReady(session.Serial) })
	if err != nil {
		a.clearScrcpySession(session.Serial, session)
	}
}
