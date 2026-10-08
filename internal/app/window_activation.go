package app

import (
	"fmt"
	"time"
)

// SDL windows can appear before Launch Services registers their application.
// Retry rejected activation requests briefly without hiding a persistent error.
func activateProcessWithRetry(pid int, activate func(int) bool, wait func(time.Duration)) error {
	if pid <= 0 {
		return fmt.Errorf("无效的进程 pid：%d", pid)
	}
	const attempts = 9
	for attempt := 0; attempt < attempts; attempt++ {
		if activate(pid) {
			return nil
		}
		if attempt < attempts-1 {
			wait(250 * time.Millisecond)
		}
	}
	return fmt.Errorf("系统未允许激活外部窗口进程：%d，请点击已打开的镜像窗口切换到前台", pid)
}
