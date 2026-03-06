//go:build windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

// setDetachAttrs sets SysProcAttr to detach the child process on Windows.
func setDetachAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// isProcessAlive checks whether a process is still running.
// On Windows, FindProcess always succeeds, so we assume alive
// if the caller already found it via PID file.
func isProcessAlive(proc *os.Process) bool {
	_ = proc
	return true
}
