package app

import (
	"fmt"
	"strings"
	"sync"
)

var deviceTaskLocks sync.Map
var deviceTaskSlots = make(chan struct{}, 4)

// Resolve emulator serials to the AVD identity so lifecycle and application
// operations use the same lock. ADB helpers never acquire another task lock.
func (a *App) lockDeviceTask(serial string) func() {
	key := "serial:" + strings.TrimSpace(serial)
	if devices, err := a.activeDevices(); err == nil {
		for _, device := range devices {
			if device.Serial == serial && device.AVDName != "" {
				key = "avd:" + device.AVDName
				break
			}
		}
	}
	return lockTargetTask(key)
}
func (a *App) lockAVDTask(name string) func() {
	return lockTargetTask("avd:" + strings.TrimSpace(name))
}
func lockTargetTask(key string) func() {
	value, _ := deviceTaskLocks.LoadOrStore(key, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	deviceTaskSlots <- struct{}{}
	return func() { <-deviceTaskSlots; mu.Unlock() }
}

func (a *App) requireReadySerial(serial string) (*ActiveDevice, error) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return nil, fmt.Errorf("请先选择一台可用设备")
	}
	devices, err := a.activeDevices()
	if err != nil {
		return nil, err
	}
	for _, device := range devices {
		if device.Serial == serial {
			if device.State != "device" {
				return nil, fmt.Errorf("设备 %s 当前不可用：%s", serial, device.State)
			}
			return &device, nil
		}
	}
	return nil, fmt.Errorf("设备已不可用：%s", serial)
}
