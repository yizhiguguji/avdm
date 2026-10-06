//go:build !darwin

package app

import "fmt"

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
