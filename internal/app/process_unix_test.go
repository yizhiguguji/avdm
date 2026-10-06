//go:build darwin || linux

package app

import (
	"os/exec"
	"testing"
)

func TestConfigureDetachedProcessSetsProcessGroup(t *testing.T) {
	cmd := exec.Command("emulator")
	configureDetachedProcess(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatalf("expected detached process attributes")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatalf("expected emulator process to start in an independent process group")
	}
}
