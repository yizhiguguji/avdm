//go:build !darwin

package app

import "os/exec"

func configureMirrorTypeIcon(cmd *exec.Cmd, serial, title string) {}
