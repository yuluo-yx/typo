//go:build !unix

package commands

import "os/exec"

func configureHelpCommand(cmd *exec.Cmd) {}

func killHelpCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
