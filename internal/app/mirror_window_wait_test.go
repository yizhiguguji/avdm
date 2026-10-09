package app

import (
	"errors"
	"testing"
	"time"
)

func TestMirrorWindowWaitAllowsDelayedWindowCreation(t *testing.T) {
	placement := &scrcpyWindowPlacement{X: 6, Y: 42, Width: 266, Height: 624}
	queries := 0
	err := waitScrcpyWindowPlacement(placement, make(chan error), func() (ExternalWindowFrame, error) {
		queries++
		if queries < 5 {
			return ExternalWindowFrame{}, errors.New("not-found")
		}
		return ExternalWindowFrame{X: 6, Y: 42, Width: 266, Height: 624}, nil
	}, time.Second, time.Millisecond)
	if err != nil || queries != 5 {
		t.Fatalf("delayed window rejected: queries=%d err=%v", queries, err)
	}
}
func TestMirrorWindowWaitDetectsExitDuringDiscovery(t *testing.T) {
	done := make(chan error, 1)
	cause := errors.New("decoder disconnected")
	err := waitScrcpyWindowPlacement(&scrcpyWindowPlacement{}, done, func() (ExternalWindowFrame, error) {
		done <- cause
		return ExternalWindowFrame{}, errors.New("not-found")
	}, time.Second, time.Millisecond)
	var exited *scrcpyStartupExitError
	if !errors.As(err, &exited) || !errors.Is(err, cause) {
		t.Fatalf("exit misreported as tiling failure: %v", err)
	}
}
func TestMirrorWindowWaitKeepsRealPositionFailure(t *testing.T) {
	err := waitScrcpyWindowPlacement(&scrcpyWindowPlacement{X: 6, Y: 42, Width: 266, Height: 624}, make(chan error), func() (ExternalWindowFrame, error) {
		return ExternalWindowFrame{X: 300, Y: 42, Width: 266, Height: 624}, nil
	}, 5*time.Millisecond, time.Millisecond)
	var exited *scrcpyStartupExitError
	if err == nil || errors.As(err, &exited) {
		t.Fatalf("wrong window position accepted: %v", err)
	}
}
