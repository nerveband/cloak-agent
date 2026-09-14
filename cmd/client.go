package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
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
	OK      bool        `json:"ok,omitempty"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func (r *Response) UnmarshalJSON(data []byte) error {
	type Alias Response
	aux := struct {
		Success *bool `json:"success"`
		OK      *bool `json:"ok"`
		*Alias
	}{Alias: (*Alias)(r)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Success != nil {
		r.Success = *aux.Success
	}
	if aux.OK != nil {
		r.OK = *aux.OK
		if aux.Success == nil {
			r.Success = *aux.OK
		}
	}
	return nil
}

func (r Response) IsSuccess() bool {
	return r.Success || r.OK
}

// GetAppDir returns the path to ~/.cloak-agent.
func GetAppDir() string {
	if dir := os.Getenv("CLOAK_AGENT_INSTALL_DIR"); dir != "" {
		return dir
	}
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
	if runtime.GOOS != "windows" {
		if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
			return filepath.Join(dir, appName)
		}
	}
	return GetAppDir()
}

// GetPortForSession hashes a session into the IANA dynamic port range.
// Keep this algorithm and its test vectors in sync with daemon/src/daemon.ts.
func GetPortForSession(session string) int {
	hash := sha256.Sum256([]byte(session))
	value := binary.BigEndian.Uint16(hash[:2])
	return 49152 + (int(value) % 16384)
}

// GetSocketPath returns the path to the Unix socket for the given session.
// On Windows it returns a TCP address instead.
func GetSocketPath(session string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("127.0.0.1:%d", GetPortForSession(session))
	}
	return filepath.Join(GetSocketDir(), session+".sock")
}

// GetPidFile returns the path to the PID file for the given session.
func GetPidFile(session string) string {
	return filepath.Join(GetSocketDir(), session+".pid")
}

// GetLogFile returns the path to the daemon log file for the given session.
func GetLogFile(session string) string {
	return filepath.Join(GetSocketDir(), session+".log")
}

// GetStreamPortFile returns the path to the daemon stream port file.
func GetStreamPortFile(session string) string {
	return filepath.Join(GetSocketDir(), session+".stream")
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func executableDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}
	return filepath.Dir(exePath)
}

func hasSourceLayout(root string) bool {
	return fileExists(filepath.Join(root, "go.mod")) &&
		fileExists(filepath.Join(root, "daemon", "package.json")) &&
		fileExists(filepath.Join(root, "scripts", "install.sh"))
}

func walkUpToSourceRoot(start string) string {
	if start == "" {
		return ""
	}

	root, err := filepath.Abs(start)
	if err != nil {
		return ""
	}

	for {
		if hasSourceLayout(root) {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			return ""
		}
		root = parent
	}
}

func findSourceProjectDir() string {
	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if exeDir := executableDir(); exeDir != "" {
		candidates = append(candidates, exeDir, filepath.Join(exeDir, ".."))
	}

	for _, candidate := range candidates {
		if root := walkUpToSourceRoot(candidate); root != "" {
			return root
		}
	}

	return ""
}

func findInstalledDaemonDir() string {
	candidates := []string{}
	if exeDir := executableDir(); exeDir != "" {
		candidates = append(candidates,
			filepath.Join(exeDir, "daemon"),
			filepath.Join(exeDir, "..", "daemon"),
		)
	}
	candidates = append(candidates, filepath.Join(GetAppDir(), "daemon"))

	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if dirExists(abs) && fileExists(filepath.Join(abs, "package.json")) {
			return abs
		}
	}

	return ""
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

// findDaemonJS binds the daemon to the executing release, not the shell cwd.
func findDaemonJS() (string, error) {
	candidates := []string{}
	if dir := os.Getenv("CLOAK_AGENT_DAEMON_DIR"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "dist", "daemon.js"))
	} else {
		dir := executableDir()
		candidates = append(candidates, filepath.Join(dir, "daemon", "dist", "daemon.js"), filepath.Join(dir, "..", "daemon", "dist", "daemon.js"))
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return filepath.Abs(candidate)
		}
	}
	return "", fmt.Errorf("matching daemon.js not found beside executable; install the complete release")
}

// StartDaemon spawns the Node.js daemon process for the given session.
func StartDaemon(session string) error {
	daemonJS, err := findDaemonJS()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(daemonJS), "..", "package.json"))
	if err != nil {
		return err
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err = json.Unmarshal(raw, &pkg); err != nil {
		return err
	}
	if Version != "dev" && pkg.Version != Version {
		return fmt.Errorf("binary/daemon version mismatch (%s/%s); install the complete release", Version, pkg.Version)
	}

	// Ensure socket directory exists.
	socketDir := GetSocketDir()
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		return fmt.Errorf("failed to create socket dir: %w", err)
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("node not found in PATH; install Node.js 22+ to run cloak-agent")
	}

	cmd := exec.Command(nodePath, daemonJS)
	cmd.Env = append(os.Environ(),
		"CLOAK_AGENT_DAEMON=1",
		"CLOAK_AGENT_SESSION="+session,
		"CLOAK_AGENT_SOCKET_DIR="+socketDir,
	)
	// Detach the child process so it outlives the CLI.
	setDetachAttrs(cmd)
	logFile, err := os.OpenFile(GetLogFile(session), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open daemon log file: %w", err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Detach — we don't wait for the process.
	go cmd.Wait() //nolint:errcheck

	// Poll for the Unix socket or Windows TCP listener.
	socketPath := GetSocketPath(session)
	deadline := time.Now().Add(10 * time.Second)
	if err := waitForEndpoint(socketPath, deadline); err == nil {
		return nil
	}

	return fmt.Errorf("timed out waiting for daemon socket at %s (log: %s)", socketPath, GetLogFile(session))
}

func waitForEndpoint(endpoint string, deadline time.Time) error {
	for time.Now().Before(deadline) {
		if runtime.GOOS == "windows" {
			conn, err := net.DialTimeout("tcp", endpoint, 100*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		} else if _, err := os.Stat(endpoint); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("endpoint did not become ready: %s", endpoint)
}

// StopDaemon terminates the daemon for the given session.
func StopDaemon(session string) error {
	pidPath := GetPidFile(session)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return fmt.Errorf("daemon for session %q is not running", session)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid PID file for session %q", session)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find daemon process: %w", err)
	}
	if err := proc.Kill(); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}
	_ = os.Remove(GetSocketPath(session))
	_ = os.Remove(GetPidFile(session))
	return nil
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
	network := "unix"
	if runtime.GOOS == "windows" {
		network = "tcp"
	}
	conn, err := net.DialTimeout(network, GetSocketPath(session), timeout)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to session %q; inspect its daemon before restarting: %w", session, err)
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	exchange := func(payload []byte) ([]byte, error) {
		if _, err := conn.Write(append(payload, '\n')); err != nil {
			return nil, err
		}
		var response []byte
		for {
			part, more, err := reader.ReadLine()
			if err != nil {
				return nil, err
			}
			if len(response)+len(part) > 8<<20 {
				return nil, fmt.Errorf("daemon response exceeds 8 MiB")
			}
			response = append(response, part...)
			if !more {
				return response, nil
			}
		}
	}
	status, err := exchange([]byte(`{"id":"_version","action":"runtime_status"}`))
	if err != nil {
		return nil, err
	}
	var handshake struct {
		OK   bool `json:"ok"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err = json.Unmarshal(status, &handshake); err != nil {
		return nil, err
	}
	if !handshake.OK || (Version != "dev" && handshake.Data.Version != Version) {
		return nil, fmt.Errorf("session %q uses an incompatible daemon (%s; CLI %s). Use a new --session or explicitly restart this session; existing work was not stopped", session, handshake.Data.Version, Version)
	}
	return exchange(command)
}
