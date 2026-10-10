package main

import (
	core "adm/internal/app"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Copy identities at click time so refreshes and selection changes cannot
// redirect a confirmation to a different device.
func (g *GUIApp) powerTargets() []core.DeviceEntry {
	entries := g.selectedControlEntries()
	explicit := false
	for _, selected := range g.controlSelected {
		explicit = explicit || selected
	}
	if !explicit {
		for _, entry := range g.entries {
			if entry.Key == g.currentDeviceKey {
				entries = append(entries, entry)
				break
			}
		}
	}
	for i := range entries {
		if entries[i].Active != nil {
			device := *entries[i].Active
			entries[i].Active = &device
		}
		if entries[i].AVD != nil {
			avd := *entries[i].AVD
			entries[i].AVD = &avd
		}
		entries[i].Label = controlCardTitle(entries[i])
	}
	return entries
}

func physicalPowerTarget(entry core.DeviceEntry) bool {
	return entry.Active != nil && !entry.Active.IsEmulator && !strings.Contains(entry.Active.Serial, ":")
}

func powerTargetProblem(entry core.DeviceEntry, reboot bool) string {
	if reboot {
		if entry.Active == nil || entry.Active.State != "device" || strings.TrimSpace(entry.Active.Serial) == "" {
			return "设备未就绪，无法重启"
		}
	} else if entry.Active != nil && !entry.Active.IsEmulator {
		if entry.Active.State != "device" || strings.TrimSpace(entry.Active.Serial) == "" {
			return "设备未就绪，无法关闭或断开"
		}
	} else if !(entry.AVD != nil && entry.Running || entry.Active != nil && entry.Active.IsEmulator && (entry.Active.Serial != "" || entry.Active.AVDName != "")) {
		return "模拟器未启动或缺少设备标识"
	}
	return ""
}

func powerConfirmationMessage(entries []core.DeviceEntry, reboot bool) string {
	action := "关闭"
	if reboot {
		action = "重启"
	}
	lines := []string{fmt.Sprintf("将%s以下 %d 台设备：", action, len(entries))}
	for i, entry := range entries {
		operation := action
		if !reboot && entry.Active != nil && !entry.Active.IsEmulator {
			operation = "真机关机"
			if !physicalPowerTarget(entry) {
				operation = "断开网络连接"
			}
		}
		lines = append(lines, fmt.Sprintf("%d. %s\n   编号：%s · %s", i+1, controlCardTitle(entry), controlCopyIdentifier(entry), operation))
	}
	lines = append(lines, "", "未保存的操作可能丢失。")
	return strings.Join(lines, "\n")
}

func validatePowerConfirmations(entries []core.DeviceEntry, confirmations map[string]string) error {
	for _, entry := range entries {
		if physicalPowerTarget(entry) && confirmations[entry.Key] != entry.Active.Serial {
			return fmt.Errorf("%s 的设备编号不匹配，未执行任何关机操作。", controlCardTitle(entry))
		}
	}
	return nil
}

func closePowerTarget(entry core.DeviceEntry, confirmation string, closeAVD func(string) error, closeSerial func(string, string) error) error {
	if entry.Active != nil && !entry.Active.IsEmulator {
		serial := entry.Active.Serial
		if physicalPowerTarget(entry) && confirmation != serial {
			return fmt.Errorf("设备编号不匹配：%s", controlCardTitle(entry))
		}
		return closeSerial(serial, serial)
	}
	return stopSelectedEmulator(entry, closeAVD, closeSerial)
}

func (g *GUIApp) showPowerDialog(reboot bool) {
	targets := g.powerTargets()
	if len(targets) == 0 {
		g.showInfo("请勾选设备或设置主目标。")
		return
	}
	var problems []string
	for _, entry := range targets {
		if problem := powerTargetProblem(entry, reboot); problem != "" {
			problems = append(problems, controlCardTitle(entry)+"："+problem)
		}
	}
	if len(problems) > 0 {
		g.showInfo("所选设备无法执行操作：\n" + strings.Join(problems, "\n"))
		return
	}
	action := "关闭"
	if reboot {
		action = "重启"
	}
	message := powerConfirmationMessage(targets, reboot)
	content := container.NewVBox(g.dialogMessageContent(message, fyne.NewSize(640, 240)))
	fields := map[string]*widget.Entry{}
	if !reboot {
		for _, entry := range targets {
			if !physicalPowerTarget(entry) {
				continue
			}
			field := widget.NewEntry()
			field.SetPlaceHolder(entry.Active.Serial)
			fields[entry.Key] = field
			content.Add(wrappedLabel("请输入 " + controlCardTitle(entry) + " 的设备编号，确认真机关机："))
			content.Add(field)
		}
	}
	g.showActionDialog(fmt.Sprintf("确认%s %d 台设备", action, len(targets)), action, true, content, func() {
		confirmations := map[string]string{}
		for key, field := range fields {
			confirmations[key] = field.Text
		}
		if !reboot {
			if err := validatePowerConfirmations(targets, confirmations); err != nil {
				g.showInfo(err.Error())
				return
			}
		}
		g.runEntryBatchAction(fmt.Sprintf("%s %d 台设备", action, len(targets)), targets, func(entry core.DeviceEntry) error {
			if reboot {
				return g.backend.GUIRebootDevice(entry.Active.Serial)
			}
			return closePowerTarget(entry, confirmations[entry.Key], g.backend.GUICloseAVD, g.backend.GUICloseDeviceConfirmed)
		})
	})
}
