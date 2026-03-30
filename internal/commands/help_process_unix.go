//go:build unix

// For GHA test

package commands

import (
	"os/exec"
	"syscall"
)

func configureHelpCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killHelpCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
