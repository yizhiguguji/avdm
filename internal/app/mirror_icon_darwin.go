//go:build darwin

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func configureMirrorTypeIcon(cmd *exec.Cmd, serial, title string) {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	resources := filepath.Join(filepath.Dir(executable), "..", "Resources")
	library := filepath.Join(resources, "mirror-title-icon.dylib")
	iconName := "physical-device.png"
	if strings.HasPrefix(serial, "emulator-") {
		iconName = "virtual-device.png"
	}
	icon := filepath.Join(resources, "device-icons", iconName)
	if !fileExists(library) || !fileExists(icon) {
		return
	}
	environment := os.Environ()
	libraryList := library
	if existing := os.Getenv("DYLD_INSERT_LIBRARIES"); existing != "" {
		libraryList = existing + ":" + library
	}
	values := map[string]string{"DYLD_INSERT_LIBRARIES": libraryList, "AVDM_MIRROR_ICON": icon, "AVDM_MIRROR_TITLE": title}
	result := make([]string, 0, len(environment)+len(values))
	for _, value := range environment {
		key, _, _ := strings.Cut(value, "=")
		if _, replaced := values[key]; !replaced {
			result = append(result, value)
		}
	}
	for _, key := range []string{"DYLD_INSERT_LIBRARIES", "AVDM_MIRROR_ICON", "AVDM_MIRROR_TITLE"} {
		result = append(result, key+"="+values[key])
	}
	cmd.Env = result
}
