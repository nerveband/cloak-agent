package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nerveband/cloak-agent/cmd/update"
)

// The bootstrap reads the pointer once. The selected binary locates its daemon
// relative to its immutable release directory, never relative to the working directory.
func dispatchInstalled(args []string) (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return false, err
	}
	if filepath.Base(filepath.Dir(exe)) != "bin" {
		return false, nil
	}
	root := filepath.Dir(filepath.Dir(exe))
	selected, err := update.CurrentRoot(root)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	target := filepath.Join(selected, update.BinaryName())
	if !fileExists(target) {
		return false, fmt.Errorf("managed release executable missing: %s", target)
	}
	command := exec.Command(target, args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = os.Environ()
	if os.Getenv("CLOAK_AGENT_INSTALL_DIR") == "" {
		command.Env = append(command.Env, "CLOAK_AGENT_INSTALL_DIR="+root)
	}
	err = command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		os.Exit(exit.ExitCode())
	}
	return true, err
}
