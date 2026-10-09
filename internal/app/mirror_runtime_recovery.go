package app

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type MirrorEvent struct {
	Level   string
	Message string
}

func (a *App) recordMirrorEvent(level, message string) {
	a.remoteMu.Lock()
	defer a.remoteMu.Unlock()
	a.mirrorEvents = append(a.mirrorEvents, MirrorEvent{Level: level, Message: message})
	if len(a.mirrorEvents) > 200 {
		a.mirrorEvents = a.mirrorEvents[len(a.mirrorEvents)-200:]
	}
}

// GUIDrainMirrorEvents transfers asynchronous mirror failures to the copyable
// task log. Runtime exits must not silently disappear after the launch task.
func (a *App) GUIDrainMirrorEvents() []MirrorEvent {
	a.remoteMu.Lock()
	defer a.remoteMu.Unlock()
	events := a.mirrorEvents
	a.mirrorEvents = nil
	return events
}

// A healthy session breaks the consecutive-failure streak. Sporadic transport
// drops must not permanently exhaust recovery over the lifetime of a window.
func consecutiveMirrorRecoveryAttempts(attempts int, openedAt, now time.Time) int {
	if !openedAt.IsZero() && now.Sub(openedAt) >= time.Minute {
		return 0
	}
	return attempts
}

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
		a.recordMirrorEvent("ERROR", fmt.Sprintf("%s：断流恢复失败：%v\n日志：%s", session.Title, readyErr, session.LogPath))
		return
	}
	err := retryScrcpyStartup(func() error {
		return a.startScrcpySessionWithRecovery(session.Serial, session.Title, session.AlwaysOnTop, session.RecoveryAttempts+1, session.Placement)
	}, func() error { return a.waitMirrorADBReady(session.Serial) })
	if err != nil {
		a.clearScrcpySession(session.Serial, session)
		a.recordMirrorEvent("ERROR", fmt.Sprintf("%s：重开镜像失败：%v", session.Title, err))
	} else {
		a.recordMirrorEvent("DONE", fmt.Sprintf("%s：断流镜像已恢复", session.Title))
	}
}
