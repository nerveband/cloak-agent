package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	appName        = "cloak-agent"
	defaultTimeout = 30 * time.Second
)

// Response represents a JSON response from the daemon.
type Response struct {
	ID      string      `json:"id"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// GetAppDir returns the path to ~/.cloak-agent.
func GetAppDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "."+appName)
}

// GetSocketDir returns the socket directory, checking CLOAK_AGENT_SOCKET_DIR
// env var first and falling back to GetAppDir().
func GetSocketDir() string {
	if dir := os.Getenv("CLOAK_AGENT_SOCKET_DIR"); dir != "" {
		return dir
	}
	return GetAppDir()
}

// GetSocketPath returns the path to the Unix socket for the given session.
// On Windows it returns a TCP address instead.
func GetSocketPath(session string) string {
	if runtime.GOOS == "windows" {
		// Use a deterministic TCP port derived from session name
		port := 9500
		for _, c := range session {
			port += int(c)
		}
		return fmt.Sprintf("127.0.0.1:%d", port)
	}
	return filepath.Join(GetSocketDir(), session+".sock")
}

// GetPidFile returns the path to the PID file for the given session.
func GetPidFile(session string) string {
	return filepath.Join(GetSocketDir(), session+".pid")
}

// IsDaemonRunning checks whether the daemon for the given session is running
// by reading its PID file and verifying the process exists.
func IsDaemonRunning(session string) bool {
	pidPath := GetPidFile(session)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return isProcessAlive(proc)
}

// findDaemonJS locates daemon.js by searching several candidate paths in order.
func findDaemonJS() (string, error) {
	candidates := []string{}

	// 1. CLOAK_AGENT_DAEMON_DIR env var
	if dir := os.Getenv("CLOAK_AGENT_DAEMON_DIR"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "dist", "daemon.js"))
	}

	// 2. Relative to the executable: <exe_dir>/../daemon/dist/daemon.js
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(exeDir, "..", "daemon", "dist", "daemon.js"))
	}

	// 3. Relative to app dir: ~/.cloak-agent/daemon/dist/daemon.js
	candidates = append(candidates, filepath.Join(GetAppDir(), "daemon", "dist", "daemon.js"))

	for _, p := range candidates {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs, nil
		}
	}

	return "", fmt.Errorf("daemon.js not found; searched: %s", strings.Join(candidates, ", "))
}

// StartDaemon spawns the Node.js daemon process for the given session.
func StartDaemon(session string) error {
	daemonJS, err := findDaemonJS()
	if err != nil {
		return err
	}

	// Ensure socket directory exists.
	socketDir := GetSocketDir()
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		return fmt.Errorf("failed to create socket dir: %w", err)
	}

	cmd := exec.Command("node", daemonJS)
	cmd.Env = append(os.Environ(),
		"CLOAK_AGENT_DAEMON=1",
		"CLOAK_AGENT_SESSION="+session,
	)
	// Detach the child process so it outlives the CLI.
	setDetachAttrs(cmd)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Detach — we don't wait for the process.
	go cmd.Wait() //nolint:errcheck

	// Poll for the socket to appear.
	socketPath := GetSocketPath(session)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for daemon socket at %s", socketPath)
}

// SendCommand sends a JSON command to the daemon over the Unix socket and
// returns the raw response bytes. It auto-starts the daemon if it is not
// already running.
func SendCommand(session string, command []byte, timeout time.Duration) ([]byte, error) {
	if !IsDaemonRunning(session) {
		if err := StartDaemon(session); err != nil {
			return nil, fmt.Errorf("failed to start daemon: %w", err)
		}
	}

	socketPath := GetSocketPath(session)

	var network string
	if runtime.GOOS == "windows" {
		network = "tcp"
	} else {
		network = "unix"
	}

	conn, err := net.DialTimeout(network, socketPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	// Write command terminated by newline.
	msg := append(command, '\n')
	if _, err := conn.Write(msg); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Read response until newline.
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if idx := indexOf(buf, '\n'); idx >= 0 {
				return buf[:idx], nil
			}
		}
		if err != nil {
			if len(buf) > 0 {
				return buf, nil
			}
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
	}
}

// indexOf returns the index of the first occurrence of b in data, or -1.
func indexOf(data []byte, b byte) int {
	for i, v := range data {
		if v == b {
			return i
		}
	}
	return -1
}
