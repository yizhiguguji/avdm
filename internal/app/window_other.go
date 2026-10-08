//go:build !darwin

package app

import "fmt"

func (a *App) planScrcpyFallbackNative(keys []string) (*scrcpyFallbackPlan, error) {
	return nil, fmt.Errorf("当前平台不支持镜像窗口排列回退")
}

func verifyScrcpyWindowPlacementNative(pid int, placement *scrcpyWindowPlacement) error {
	return fmt.Errorf("当前平台不支持通过启动参数校验窗口排列")
}

func activateProcessNative(pid int) error {
	return fmt.Errorf("激活外部窗口目前仅支持 macOS")
}

func MainDisplaySize() (int, int, bool) {
	return 0, 0, false
}

func accessibilityTrustedNative() bool {
	return false
}

func focusEmulatorWindowNative(avdName, serial string) error {
	return fmt.Errorf("聚焦模拟器窗口目前仅支持 macOS")
}

func focusProcessWindowNative(pid int) error {
	return fmt.Errorf("聚焦外部窗口目前仅支持 macOS")
}

func resizeProcessWindowNative(pid int, width, height int) error {
	return fmt.Errorf("调整外部窗口尺寸目前仅支持 macOS")
}

func tileEmulatorWindowsNative(targets []emulatorWindowTarget, columns int) error {
	return fmt.Errorf("平铺模拟器窗口目前仅支持 macOS")
}

func tileProcessWindowsNative(pids []int, columns int) error {
	return fmt.Errorf("平铺外部窗口目前仅支持 macOS")
}

// Native emulator window lookup is currently supported only on macOS.
func qemuPIDForAVD(avdName string) (int, bool) {
	return 0, false
}
