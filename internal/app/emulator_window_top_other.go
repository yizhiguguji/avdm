//go:build !darwin

package app

import "fmt"

func emulatorWindowTopNative(pid int, avd string) (bool, error) {
	return false, fmt.Errorf("原生模拟器置顶目前仅支持 macOS")
}
func applyEmulatorWindowTopNative(pid int, avd string, enabled bool) error {
	return fmt.Errorf("原生模拟器置顶目前仅支持 macOS")
}
