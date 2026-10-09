package app

import (
	"fmt"
	"math"
	"time"
)

type scrcpyStartupExitError struct{ cause error }

func (e *scrcpyStartupExitError) Error() string {
	if e.cause == nil {
		return "镜像进程已退出"
	}
	return "镜像进程已退出：" + e.cause.Error()
}
func (e *scrcpyStartupExitError) Unwrap() error { return e.cause }

// ADB/encoder initialization can outlast window discovery. Wait for the frame,
// but stop immediately when the process exits instead of reporting bad tiling.
func waitScrcpyWindowPlacement(placement *scrcpyWindowPlacement, done <-chan error, read func() (ExternalWindowFrame, error), timeout, interval time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(interval)
	defer tick.Stop()
	var frame ExternalWindowFrame
	var err error
	for {
		select {
		case cause := <-done:
			return &scrcpyStartupExitError{cause: cause}
		default:
		}
		frame, err = read()
		if err == nil && math.Abs(frame.X-float64(placement.X)) <= 3 && math.Abs(frame.Y-float64(placement.Y)) <= 3 && math.Abs(frame.Width-float64(placement.Width)) <= 3 && math.Abs(frame.Height-float64(placement.Height)) <= 3 {
			// The process may have exited while its last window bounds were queried.
			select {
			case cause := <-done:
				return &scrcpyStartupExitError{cause: cause}
			default:
				return nil
			}
		}
		select {
		case cause := <-done:
			return &scrcpyStartupExitError{cause: cause}
		case <-deadline.C:
			if err != nil {
				return fmt.Errorf("等待镜像窗口超时（%s）：%w", timeout, err)
			}
			return fmt.Errorf("期望窗口位置 (%d,%d)、尺寸 %dx%d，实际窗口位置 (%.0f,%.0f)、尺寸 %.0fx%.0f", placement.X, placement.Y, placement.Width, placement.Height, frame.X, frame.Y, frame.Width, frame.Height)
		case <-tick.C:
		}
	}
}
