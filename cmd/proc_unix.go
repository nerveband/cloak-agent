//go:build !windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

// setDetachAttrs sets SysProcAttr to detach the child process on Unix.
func setDetachAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// isProcessAlive checks whether a process is still running.
func isProcessAlive(proc *os.Process) bool {
	return proc.Signal(syscall.Signal(0)) == nil
}
