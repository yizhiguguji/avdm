//go:build !darwin && !linux

package app

import "os/exec"

func configureDetachedProcess(cmd *exec.Cmd) {
}
