package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type tailgateConfig struct {
	Routes map[string]tailgateRoute `json:"routes"`
}

type tailgateRoute struct {
	SSHHost        string `json:"sshHost"`
	SSHHostAlias   string `json:"sshHostAlias,omitempty"`
	SSHConfig      string `json:"sshConfig,omitempty"`
	IdentityFile   string `json:"identityFile"`
	KnownHostsFile string `json:"knownHostsFile"`
	SSHUser        string `json:"sshUser,omitempty"`
	SSHPort        int    `json:"sshPort,omitempty"`
	UseAgent       bool   `json:"useAgent,omitempty"`
}

type tailgateState struct {
	Route       string `json:"route"`
	Port        int    `json:"port"`
	BrowserPort int    `json:"browserPort,omitempty"`
	Profile     string `json:"profile,omitempty"`
}

type directLaunchState struct {
	Profile string `json:"profile,omitempty"`
}

func tailgateConfigPath() string {
	if value := os.Getenv("CLOAK_AGENT_TAILGATE_CONFIG"); value != "" {
		return expandUserPath(value)
	}
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		path := filepath.Join(value, "cloak-agent", "tailgate.json")
		if fileExists(path) {
			return path
		}
		legacy := filepath.Join(GetAppDir(), "tailgate.json")
		if fileExists(legacy) {
			return legacy
		}
		return path
	}
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".config", "cloak-agent", "tailgate.json")
		if fileExists(path) {
			return path
		}
		legacy := filepath.Join(GetAppDir(), "tailgate.json")
		if fileExists(legacy) {
			return legacy
		}
		return path
	}
	return filepath.Join(GetAppDir(), "tailgate.json")
}

func tailgateDir() string { return filepath.Join(GetSocketDir(), "tailgate") }

func ensureTailgateDir() error {
	dir := tailgateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("tailgate runtime directory is unavailable")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("tailgate runtime directory must not be accessible by group or other users")
	}
	return nil
}

func checkTailgateDir() error {
	info, err := os.Lstat(tailgateDir())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("tailgate runtime directory must not be accessible by group or other users")
	}
	return nil
}

func tailgateKey(session, route string) string {
	sum := sha256.Sum256([]byte(session + "\x00" + route))
	return hex.EncodeToString(sum[:8])
}

func tailgateControlPath(session, route string) string {
	return filepath.Join(tailgateDir(), tailgateKey(session, route)+".ctl")
}

func tailgateStatePath(session string) string {
	return filepath.Join(tailgateDir(), tailgateKey(session, "state")+".json")
}

func tailgateBindingPath(session string) string {
	return filepath.Join(tailgateDir(), tailgateKey(session, "browser")+".route")
}

func directLaunchStatePath(session string) string {
	return filepath.Join(tailgateDir(), tailgateKey(session, "direct")+".json")
}

func tailgateSessionLockPath(session string) string {
	return filepath.Join(tailgateDir(), tailgateKey(session, "session-lock")+".lock")
}

func tailgatePortReservationPath(port int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("tailgate-port:%d", port)))
	return filepath.Join(tailgateDir(), "port-"+hex.EncodeToString(sum[:8])+".lock")
}

func reserveTailgatePort() (int, error) {
	for attempt := 0; attempt < 16; attempt++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			continue
		}
		port := listener.Addr().(*net.TCPAddr).Port
		reservation := tailgatePortReservationPath(port)
		lock, createErr := os.OpenFile(reservation, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		_ = listener.Close()
		if createErr == nil {
			if _, writeErr := lock.WriteString(strconv.Itoa(os.Getpid())); writeErr != nil {
				_ = lock.Close()
				_ = os.Remove(reservation)
				continue
			}
			if closeErr := lock.Close(); closeErr != nil {
				_ = os.Remove(reservation)
				continue
			}
			return port, nil
		}
		if info, statErr := os.Lstat(reservation); statErr == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && time.Since(info.ModTime()) > 2*time.Minute && !tailgateSOCKSReady(port) {
			_ = os.Remove(reservation)
		}
	}
	return 0, fmt.Errorf("no available loopback port for tailgate")
}

func releaseTailgatePort(port int) {
	if port < 1 || port > 65535 {
		return
	}
	path := tailgatePortReservationPath(port)
	info, err := os.Lstat(path)
	if err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		_ = os.Remove(path)
	}
}

func acquireTailgateSessionLock(session string) (*os.File, error) {
	if err := ensureTailgateDir(); err != nil {
		return nil, err
	}
	path := tailgateSessionLockPath(session)
	lock, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
			_ = os.Remove(path)
			lock, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("tailgate session is busy; retry after the active transition finishes")
	}
	if _, err := lock.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		_ = lock.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return lock, nil
}

func releaseTailgateSessionLock(lock *os.File, session string) {
	if lock == nil {
		return
	}
	_ = lock.Close()
	_ = os.Remove(tailgateSessionLockPath(session))
}

func expandUserPath(value string) string {
	if value == "~" || strings.HasPrefix(value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	return value
}

func validateTailgateFile(path string, private bool) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("file is not a regular non-symlink file")
	}
	if private && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("private file permissions are unsafe")
	}
	if !private && info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("file is writable by group or other users")
	}
	return nil
}

func loadTailgateRoute(name string) (tailgateRoute, error) {
	if err := validateTailgateRouteName(name); err != nil {
		return tailgateRoute{}, err
	}
	path := tailgateConfigPath()
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if name == "default" {
			if discovered, discoverErr := discoverBrowserHarnessTailgate(); discoverErr == nil {
				return discovered, nil
			}
		}
		return tailgateRoute{}, fmt.Errorf("tailgate config unavailable")
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return tailgateRoute{}, fmt.Errorf("tailgate config unavailable")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return tailgateRoute{}, fmt.Errorf("tailgate config must not be accessible by group or other users")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return tailgateRoute{}, fmt.Errorf("failed to read tailgate config")
	}
	var config tailgateConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return tailgateRoute{}, fmt.Errorf("invalid tailgate config JSON")
	}
	route, ok := config.Routes[name]
	if !ok {
		if name == "default" {
			if discovered, discoverErr := discoverBrowserHarnessTailgate(); discoverErr == nil {
				return discovered, nil
			}
		}
		return tailgateRoute{}, fmt.Errorf("tailgate route %q is not configured", name)
	}
	route.IdentityFile = expandUserPath(route.IdentityFile)
	route.KnownHostsFile = expandUserPath(route.KnownHostsFile)
	if strings.TrimSpace(route.SSHHost) == "" {
		return tailgateRoute{}, fmt.Errorf("tailgate route %q requires sshHost, identityFile, and knownHostsFile", name)
	}
	if route.SSHConfig != "" {
		alias := route.SSHHost
		resolved, resolveErr := resolveSSHRoute(route.SSHHost, expandUserPath(route.SSHConfig))
		if resolveErr != nil {
			return tailgateRoute{}, resolveErr
		}
		route.SSHHost, route.SSHUser, route.SSHPort = resolved.SSHHost, resolved.SSHUser, resolved.SSHPort
		route.SSHHostAlias = resolved.SSHHostAlias
		if route.SSHHostAlias == "" {
			route.SSHHostAlias = sshHostAlias(alias)
		}
		if route.IdentityFile == "" && !route.UseAgent {
			route.IdentityFile = resolved.IdentityFile
		}
		if route.UseAgent {
			route.IdentityFile = ""
		}
		if route.KnownHostsFile == "" {
			route.KnownHostsFile = resolved.KnownHostsFile
		}
		// Runtime SSH is intentionally invoked with /dev/null as its config.
		// Persist the resolved tuple so later launches do not re-resolve an alias
		// as though it were the already-resolved hostname.
		route.SSHConfig = ""
	}
	if (!route.UseAgent && route.IdentityFile == "") || route.KnownHostsFile == "" {
		return tailgateRoute{}, fmt.Errorf("tailgate route %q requires sshHost, identityFile, and knownHostsFile", name)
	}
	if route.SSHPort == 0 {
		route.SSHPort = 22
	}
	if route.SSHPort < 1 || route.SSHPort > 65535 {
		return tailgateRoute{}, fmt.Errorf("tailgate sshPort is invalid")
	}
	if strings.HasPrefix(route.SSHHost, "-") || strings.ContainsAny(route.SSHHost, "\r\n\x00") {
		return tailgateRoute{}, fmt.Errorf("tailgate sshHost is invalid")
	}
	if strings.HasPrefix(route.SSHUser, "-") || strings.ContainsAny(route.SSHUser, "\r\n\x00") {
		return tailgateRoute{}, fmt.Errorf("tailgate sshUser is invalid")
	}
	if strings.HasPrefix(route.SSHHostAlias, "-") || strings.ContainsAny(route.SSHHostAlias, "\r\n\x00") {
		return tailgateRoute{}, fmt.Errorf("tailgate sshHostAlias is invalid")
	}
	for label, file := range map[string]string{"identityFile": route.IdentityFile, "knownHostsFile": route.KnownHostsFile} {
		if file == "" {
			continue
		}
		if err := validateTailgateFile(file, label == "identityFile"); err != nil {
			return tailgateRoute{}, fmt.Errorf("tailgate %s is unsafe: %w", label, err)
		}
	}
	return route, nil
}

// discoverBrowserHarnessTailgate adapts the established browser-harness-tailgate
// wrapper. It resolves the user's SSH alias once, then cloak-agent invokes only
// the resolved host/key/known-host tuple with a clean SSH config, preventing
// unrelated forwards or local commands from leaking into this browser tunnel.
func discoverBrowserHarnessTailgate() (tailgateRoute, error) {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "bin", "browser-harness-tailgate"),
		filepath.Join(home, ".local", "bin", "browser-use-tailgate"),
	}
	var script []byte
	target := strings.TrimSpace(os.Getenv("TAILGATE_SSH_TARGET"))
	for _, configPath := range []string{os.Getenv("BROWSER_USE_TAILGATE_CONFIG"), filepath.Join(home, ".config", "browser-use-tailgate", "config.env")} {
		if target != "" || configPath == "" {
			continue
		}
		configPath = expandUserPath(configPath)
		if discoveryFileSafe(configPath) {
			if raw, err := os.ReadFile(configPath); err == nil {
				if value := parseShellAssignment(string(raw), "TAILGATE_SSH_TARGET"); value != "" {
					target = value
				}
			}
		}
	}
	for _, candidate := range candidates {
		if !discoveryFileSafe(candidate) {
			continue
		}
		if raw, err := os.ReadFile(candidate); err == nil {
			script = raw
			break
		}
	}
	if len(script) == 0 {
		return tailgateRoute{}, fmt.Errorf("browser-harness tailgate setup not found")
	}
	if target == "" {
		text := string(script)
		for _, marker := range []string{"SSH_TARGET=\"${TAILGATE_SSH_TARGET:-", "SSH_TARGET=\"${TAILGATE_SSH_TARGET:=", "SSH_TARGET=\""} {
			if start := strings.Index(text, marker); start >= 0 {
				value := text[start+len(marker):]
				if end := strings.IndexAny(value, "}\""); end >= 0 {
					target = strings.TrimSpace(value[:end])
					break
				}
			}
		}
	}
	if target == "" || strings.HasPrefix(target, "-") || strings.ContainsAny(target, "\r\n\x00") {
		return tailgateRoute{}, fmt.Errorf("browser-harness tailgate SSH target is unavailable")
	}
	sshConfig := filepath.Join(home, ".ssh", "config")
	if !fileExists(sshConfig) {
		sshConfig = ""
	}
	return resolveSSHRoute(target, sshConfig)
}

func discoveryFileSafe(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm()&0o022 == 0
}

func resolveSSHRoute(target, sshConfig string) (tailgateRoute, error) {
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return tailgateRoute{}, fmt.Errorf("OpenSSH client not found")
	}
	args := []string{"-G"}
	if sshConfig != "" {
		if err := validateTailgateFile(sshConfig, false); err != nil {
			return tailgateRoute{}, fmt.Errorf("tailgate SSH config is unsafe")
		}
		args = append(args, "-F", sshConfig)
	}
	args = append(args, target)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, ssh, args...).Output()
	if err != nil {
		return tailgateRoute{}, fmt.Errorf("tailgate SSH target could not be resolved")
	}
	values := make(map[string][]string)
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.Fields(line)
		if len(parts) > 1 {
			values[parts[0]] = append(values[parts[0]], parts[1:]...)
		}
	}
	route := tailgateRoute{
		SSHHost:      firstValue(values["hostname"]),
		SSHUser:      firstValue(values["user"]),
		SSHHostAlias: sshHostAlias(target),
	}
	if port, parseErr := strconv.Atoi(firstValue(values["port"])); parseErr == nil {
		route.SSHPort = port
	}
	for _, value := range values["identityfile"] {
		value = expandUserPath(value)
		if value != "none" && fileExists(value) {
			route.IdentityFile = value
			break
		}
	}
	for _, value := range values["userknownhostsfile"] {
		value = expandUserPath(value)
		if value != "none" && fileExists(value) {
			route.KnownHostsFile = value
			break
		}
	}
	if firstValue(values["proxyjump"]) != "" && firstValue(values["proxyjump"]) != "none" {
		return tailgateRoute{}, fmt.Errorf("tailgate requires a direct SSH target")
	}
	if route.SSHHost == "" || route.IdentityFile == "" || route.KnownHostsFile == "" {
		return tailgateRoute{}, fmt.Errorf("tailgate SSH key or known-host file is unavailable")
	}
	if route.SSHPort == 0 {
		route.SSHPort = 22
	}
	if err := validateTailgateFile(route.IdentityFile, true); err != nil {
		return tailgateRoute{}, fmt.Errorf("tailgate SSH key is unsafe")
	}
	if err := validateTailgateFile(route.KnownHostsFile, false); err != nil {
		return tailgateRoute{}, fmt.Errorf("tailgate known-hosts file is unsafe")
	}
	return route, nil
}

func parseShellAssignment(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "export ")
		if !strings.HasPrefix(line, key+"=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, key+"="))
		value = strings.Trim(value, "\"'")
		return value
	}
	return ""
}

func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func sshHostAlias(target string) string {
	if at := strings.LastIndex(target, "@"); at >= 0 {
		target = target[at+1:]
	}
	if strings.ContainsAny(target, "\r\n\x00") || strings.HasPrefix(target, "-") {
		return ""
	}
	return target
}

func selectedTailgate(flags GlobalFlags) (string, bool) {
	if flags.Direct {
		return "", false
	}
	if flags.Tailgate {
		if flags.TailgateRoute != "" {
			return flags.TailgateRoute, true
		}
		return "default", true
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("CLOAK_AGENT_DIRECT")), "1") || strings.EqualFold(strings.TrimSpace(os.Getenv("CLOAK_AGENT_DIRECT")), "true") {
		return "", false
	}
	value := strings.TrimSpace(os.Getenv("CLOAK_AGENT_TAILGATE"))
	if value == "" || value == "0" || strings.EqualFold(value, "false") || strings.EqualFold(value, "off") {
		return "", false
	}
	if value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "on") {
		return "default", true
	}
	return value, true
}

func sshArgs(route tailgateRoute, control string) []string {
	args := []string{
		"-F", "/dev/null",
		"-p", strconv.Itoa(route.SSHPort),
		"-o", "BatchMode=yes",
		"-o", "PasswordAuthentication=no", "-o", "KbdInteractiveAuthentication=no",
		"-o", "PubkeyAuthentication=yes", "-o", "GSSAPIAuthentication=no", "-o", "HostbasedAuthentication=no",
		"-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=" + route.KnownHostsFile,
		"-o", "ExitOnForwardFailure=yes", "-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3", "-o", "ForwardAgent=no",
		"-o", "PermitLocalCommand=no",
		"-o", "LocalCommand=none", "-o", "RemoteCommand=none", "-o", "RequestTTY=no",
		"-o", "ForwardX11=no",
		"-o", "ConnectTimeout=10", "-o", "ConnectionAttempts=1",
		"-o", "ControlMaster=yes", "-S", control,
	}
	if route.IdentityFile != "" {
		args = append(args, "-i", route.IdentityFile, "-o", "IdentitiesOnly=yes", "-o", "IdentityAgent=none")
	} else if route.UseAgent {
		args = append(args, "-o", "IdentitiesOnly=no")
	} else {
		args = append(args, "-o", "IdentitiesOnly=yes", "-o", "IdentityAgent=none")
	}
	if route.SSHUser != "" {
		args = append(args, "-l", route.SSHUser)
	}
	if route.SSHHostAlias != "" {
		args = append(args, "-o", "HostKeyAlias="+route.SSHHostAlias)
	}
	return args
}

func tunnelAlive(route tailgateRoute, control string) bool {
	// OpenSSH's `-O check` can return success while creating a fresh control
	// master when ControlMaster is accidentally left enabled.  Require the
	// control path to already be a real local Unix socket before asking ssh to
	// inspect it; this keeps status/stop fail-closed and avoids false positives
	// after a daemon restart or stale descriptor recovery.
	info, err := os.Lstat(control)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeSocket == 0 {
		return false
	}
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Control operations must use only the socket and resolved host. Replaying
	// the full launch options here can make OpenSSH treat the request as a new
	// master (especially when the configured user/key differs from the host's
	// canonical tuple), which caused stop/check to report a live tunnel after a
	// successful exit request. No network authentication is needed for an
	// already-open control socket.
	args := []string{"-F", "/dev/null", "-S", control, "-o", "ControlMaster=auto", "-O", "check", route.SSHHost}
	cmd := exec.CommandContext(ctx, ssh, args...)
	return cmd.Run() == nil
}

func ensureTailgateTunnel(session, name string) (tailgateState, error) {
	if runtime.GOOS == "windows" {
		return tailgateState{}, fmt.Errorf("tailgate currently supports Linux and macOS")
	}
	route, err := loadTailgateRoute(name)
	if err != nil {
		return tailgateState{}, err
	}
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return tailgateState{}, fmt.Errorf("OpenSSH client not found")
	}
	if err := ensureTailgateDir(); err != nil {
		return tailgateState{}, err
	}
	control := tailgateControlPath(session, name)
	state := tailgateState{Route: name}
	if saved, readErr := readTailgateState(session); readErr == nil && saved.Route == name {
		// Preserve the user-facing profile across a dead tunnel recovery. The
		// descriptor is rewritten only after the replacement SOCKS endpoint is
		// verified.
		state.Profile = saved.Profile
		state.BrowserPort = saved.BrowserPort
	}
	if tunnelAlive(route, control) {
		if saved, readErr := readTailgateState(session); readErr == nil && saved.Route == name {
			if saved.Port < 1 || saved.Port > 65535 {
				return tailgateState{}, fmt.Errorf("tailgate state is invalid; stop the stale tunnel before retrying")
			}
			if !tailgateSOCKSReady(saved.Port) {
				return tailgateState{}, fmt.Errorf("tailgate control master is alive but its SOCKS endpoint is unavailable; run 'cloak-agent tailgate stop'")
			}
			return saved, nil
		}
		return tailgateState{}, fmt.Errorf("tailgate tunnel is running without valid session state")
	}
	lock, lockErr := acquireTailgateSessionLock(session)
	if lockErr != nil {
		if tunnelAlive(route, control) {
			if saved, readErr := readTailgateState(session); readErr == nil && saved.Route == name {
				return saved, nil
			}
		}
		return tailgateState{}, fmt.Errorf("tailgate tunnel start is already in progress")
	}
	defer releaseTailgateSessionLock(lock, session)
	// A dead control master must not leave a descriptor that can be mistaken for
	// a live route. Do this under the per-session lock so concurrent starts cannot
	// delete each other's recovery state.
	if err := removeTailgateControl(control); err != nil {
		return tailgateState{}, err
	}
	if saved, savedErr := readTailgateState(session); savedErr == nil {
		releaseTailgatePort(saved.Port)
	}
	_ = os.Remove(tailgateStatePath(session))
	state.Port, err = reserveTailgatePort()
	if err != nil {
		return tailgateState{}, err
	}
	args := append(sshArgs(route, control), "-f", "-N", "-T", "-D", fmt.Sprintf("127.0.0.1:%d", state.Port), route.SSHHost)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, ssh, args...).Run(); err != nil {
		releaseTailgatePort(state.Port)
		return tailgateState{}, fmt.Errorf("tailgate SSH tunnel failed; run 'cloak-agent tailgate doctor %s'", name)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tunnelAlive(route, control) {
			if !tailgateSOCKSReady(state.Port) {
				_ = stopTailgateTunnel(session, name)
				releaseTailgatePort(state.Port)
				_ = os.Remove(tailgateStatePath(session))
				return tailgateState{}, fmt.Errorf("tailgate SOCKS endpoint did not become ready")
			}
			raw, _ := json.Marshal(state)
			if err := secureTailgateWrite(tailgateStatePath(session), raw); err != nil {
				_ = stopTailgateTunnel(session, name)
				releaseTailgatePort(state.Port)
				return tailgateState{}, err
			}
			return state, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = stopTailgateTunnel(session, name)
	releaseTailgatePort(state.Port)
	return tailgateState{}, fmt.Errorf("tailgate SSH control endpoint did not become ready")
}

func isolatedTailgateProfile(session, route, profile string) string {
	if profile == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(session + "\x00" + route))
	return "tailgate-" + hex.EncodeToString(sum[:8]) + "-" + profile
}

func validateTailgateProfile(profile string) error {
	if profile == "" {
		return nil
	}
	if len(profile) > 128 || strings.TrimSpace(profile) != profile || strings.HasPrefix(profile, "-") || strings.ContainsAny(profile, "/\\\x00\r\n\t") || profile == "." || profile == ".." {
		return fmt.Errorf("tailgate profile must be a simple name without path separators")
	}
	return nil
}

func validateTailgateRouteName(name string) error {
	if name == "" {
		return fmt.Errorf("tailgate route name is required")
	}
	return validateTailgateProfile(name)
}

func prepareTailgate(command map[string]interface{}, flags GlobalFlags) error {
	name, enabled := selectedTailgate(flags)
	action, _ := command["action"].(string)
	if !enabled {
		boundName, bindingErr := readTailgateBinding(flags.Session)
		if bindingErr != nil {
			return bindingErr
		}
		state, stateErr := readTailgateState(flags.Session)
		stateMarker := fileExists(tailgateStatePath(flags.Session))
		marker := boundName != "" || stateMarker
		directSelected := flags.Direct || strings.EqualFold(strings.TrimSpace(os.Getenv("CLOAK_AGENT_DIRECT")), "1") || strings.EqualFold(strings.TrimSpace(os.Getenv("CLOAK_AGENT_DIRECT")), "true")
		if marker && !directSelected && !strings.HasPrefix(action, "tailgate_") && action != "close" {
			return fmt.Errorf("session is bound to tailgate; use --tailgate or --direct explicitly")
		}
		if directSelected && (action == "launch" || action == "navigate") {
			if flags.DryRun {
				return nil
			}
			name := boundName
			if name == "" && stateErr == nil {
				name = state.Route
			}
			if marker {
				if stateErr != nil || name == "" {
					return fmt.Errorf("stale tailgate route marker; refusing direct transition")
				}
				if err := releaseTailgateSession(flags.Session, name); err != nil {
					return err
				}
			}
			if action == "navigate" && marker {
				return directRelaunch(flags.Session)
			}
		}
		return ensureDirectProfileLaunch(flags.Session, flags, action)
	}
	if strings.HasPrefix(action, "tailgate_") {
		return nil
	}
	// Closing is handled by the daemon first and cleaned up transactionally by
	// Execute. Every other browser action must be preceded by a route-aware
	// launch/recovery check, including snapshot/click/evaluate after a daemon
	// restart.
	if action == "close" || action == "schema" || action == "profile_list" || action == "runtime_status" {
		return nil
	}
	profile, _ := command["profile"].(string)
	if profile == "" {
		profile = os.Getenv("CLOAK_AGENT_PROFILE")
	}
	if err := validateTailgateProfile(profile); err != nil {
		return err
	}
	if flags.DryRun {
		if _, err := loadTailgateRoute(name); err != nil {
			return err
		}
		if _, err := exec.LookPath("ssh"); err != nil {
			return fmt.Errorf("OpenSSH client not found")
		}
		return nil
	}
	boundName, bindingErr := readTailgateBinding(flags.Session)
	if bindingErr != nil {
		return bindingErr
	}
	stateMarker := fileExists(tailgateStatePath(flags.Session))
	state, stateErr := readTailgateState(flags.Session)
	if stateMarker && stateErr != nil {
		return fmt.Errorf("stale tailgate recovery state; run 'cloak-agent tailgate stop' after checking the local tunnel")
	}
	if boundName != "" && stateErr == nil && state.Route != boundName {
		return fmt.Errorf("tailgate binding and recovery state disagree")
	}
	if boundName == "" && stateErr == nil {
		boundName = state.Route
	}
	if boundName != "" && boundName != name {
		if err := releaseTailgateSession(flags.Session, boundName); err != nil {
			return fmt.Errorf("cannot switch tailgate routes: %w", err)
		}
	}
	if action == "launch" {
		if existingProxy, exists := command["proxy"]; exists && existingProxy != nil {
			return fmt.Errorf("tailgate owns the browser proxy; remove --proxy or use direct mode")
		}
	}
	var err error
	state, err = ensureTailgateTunnel(flags.Session, name)
	if err != nil {
		return err
	}
	if profile == "" {
		profile = state.Profile
	}
	if profile == "" {
		profile = "default"
	}
	// A recovered tunnel gets a fresh loopback port. The existing browser
	// context still points at the dead port, so it must be relaunched before
	// any browser-dependent command is sent. The browserPort descriptor remains
	// old until that relaunch commits, making a failed relaunch fail closed on
	// the next command instead of silently using a dead proxy.
	tunnelReplaced := state.BrowserPort != state.Port
	proxy := fmt.Sprintf("socks5://127.0.0.1:%d", state.Port)
	proxyConfig := map[string]interface{}{"server": proxy, "bypass": "<-loopback>"}
	originalProfile := profile
	profile = isolatedTailgateProfile(flags.Session, name, profile)
	if action == "launch" {
		command["proxy"] = proxyConfig
		if profile != "" {
			command["profile"] = profile
		}
		// Keep these bookkeeping values out of the daemon schema. Execute commits
		// them only after the daemon confirms the browser launch succeeded.
		command["_tailgateRoute"] = name
		command["_tailgateProfile"] = originalProfile
		return nil
	}
	if !tunnelReplaced && boundName == name && tailgateBrowserLaunched(flags.Session) {
		if route, routeErr := loadTailgateRoute(name); routeErr == nil && tunnelAlive(route, tailgateControlPath(flags.Session, name)) {
			if !fileExists(tailgateBindingPath(flags.Session)) {
				if err := secureTailgateWrite(tailgateBindingPath(flags.Session), []byte(name)); err != nil {
					return err
				}
			}
			return nil
		}
	}
	launch := map[string]interface{}{"id": generateID(), "action": "launch", "proxy": proxyConfig}
	if profile != "" {
		launch["profile"] = profile
	}
	raw, _ := json.Marshal(launch)
	respRaw, err := SendCommand(flags.Session, raw, defaultTimeout)
	if err != nil {
		return err
	}
	var resp Response
	if err := json.Unmarshal(respRaw, &resp); err != nil || !resp.IsSuccess() {
		return fmt.Errorf("failed to launch tailgate browser session")
	}
	state.Profile = originalProfile
	return commitTailgateLaunch(flags.Session, name, state)
}

func clearTailgateBinding(session string) error {
	if err := checkTailgateDir(); err != nil {
		return err
	}
	if state, err := readTailgateState(session); err == nil {
		releaseTailgatePort(state.Port)
	}
	for _, path := range []string{tailgateBindingPath(session), tailgateStatePath(session)} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clear tailgate session state")
		}
	}
	return nil
}

func readDirectLaunchProfile(session string) (string, error) {
	if err := checkTailgateDir(); err != nil {
		return "", err
	}
	path := directLaunchStatePath(session)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("direct launch state is unavailable or insecure")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var state directLaunchState
	if err := json.Unmarshal(raw, &state); err != nil || validateTailgateProfile(state.Profile) != nil {
		return "", fmt.Errorf("direct launch state is invalid")
	}
	return state.Profile, nil
}

func commitDirectLaunchProfile(session, profile string) error {
	if err := validateTailgateProfile(profile); err != nil {
		return err
	}
	lock, err := acquireTailgateSessionLock(session)
	if err != nil {
		return err
	}
	defer releaseTailgateSessionLock(lock, session)
	path := directLaunchStatePath(session)
	if profile == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clear direct launch state")
		}
		return nil
	}
	raw, _ := json.Marshal(directLaunchState{Profile: profile})
	return secureTailgateWrite(path, raw)
}

func ensureDirectProfileLaunch(session string, flags GlobalFlags, action string) error {
	if action == "launch" || action == "close" || action == "schema" || action == "profile_list" || action == "runtime_status" {
		return nil
	}
	profile, err := readDirectLaunchProfile(session)
	if err != nil {
		return err
	}
	if profile == "" || tailgateBrowserLaunched(session) {
		return nil
	}
	if flags.DryRun {
		return nil
	}
	launch := map[string]interface{}{"id": generateID(), "action": "launch", "profile": profile}
	raw, _ := json.Marshal(launch)
	respRaw, err := SendCommand(session, raw, defaultTimeout)
	if err != nil {
		return err
	}
	var resp Response
	if err := json.Unmarshal(respRaw, &resp); err != nil || !resp.IsSuccess() {
		return fmt.Errorf("failed to restore direct browser profile")
	}
	return nil
}

func tailgateSOCKSReady(port int) bool {
	if port < 1 || port > 65535 {
		return false
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return false
	}
	response := make([]byte, 2)
	if _, err := io.ReadFull(conn, response); err != nil {
		return false
	}
	return response[0] == 0x05 && response[1] == 0x00
}

func stopTailgateTunnel(session, name string) error {
	route, err := loadTailgateRoute(name)
	if err != nil {
		return err
	}
	control := tailgateControlPath(session, name)
	saved, stateErr := readTailgateState(session)
	if !tunnelAlive(route, control) {
		if stateErr == nil && tailgateSOCKSReady(saved.Port) {
			return fmt.Errorf("tailgate SOCKS endpoint remains active after control termination")
		}
		return removeTailgateControl(control)
	}
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("OpenSSH client not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	args := []string{"-F", "/dev/null", "-S", control, "-o", "ControlMaster=auto", "-O", "exit", route.SSHHost}
	err = exec.CommandContext(ctx, ssh, args...).Run()
	cancel()
	if err != nil && tunnelAlive(route, control) {
		return fmt.Errorf("failed to stop tailgate tunnel")
	}
	// A browser can still have a few in-flight SOCKS connections when close or
	// stop asks the control master to exit. Keep the wait bounded, but allow a
	// normal drain window before reporting a stop failure.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if !tunnelAlive(route, control) && (stateErr != nil || !tailgateSOCKSReady(saved.Port)) {
			return removeTailgateControl(control)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("tailgate tunnel is still running after stop request")
}

func removeTailgateControl(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to inspect tailgate control endpoint")
	}
	if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("tailgate control endpoint is not a safe local socket")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stale tailgate control endpoint")
	}
	return nil
}

func releaseTailgateSession(session, name string) error {
	lock, err := acquireTailgateSessionLock(session)
	if err != nil {
		return err
	}
	defer releaseTailgateSessionLock(lock, session)
	if name == "" {
		return clearTailgateBinding(session)
	}
	bound, bindingErr := readTailgateBinding(session)
	if bindingErr != nil {
		return bindingErr
	}
	if bound != "" && bound != name {
		return fmt.Errorf("tailgate route marker does not match the requested route")
	}
	state, stateErr := readTailgateState(session)
	if stateErr != nil && bound != "" {
		return fmt.Errorf("stale tailgate route marker; run 'cloak-agent tailgate stop' after repairing state")
	}
	if stateErr == nil && state.Route != name {
		return fmt.Errorf("tailgate binding and recovery state disagree")
	}
	if err := stopTailgateTunnel(session, name); err != nil {
		return err
	}
	return clearTailgateBinding(session)
}

func cleanupTailgateAfterClose(session string) error {
	bound, bindingErr := readTailgateBinding(session)
	if bindingErr != nil {
		return bindingErr
	}
	state, stateErr := readTailgateState(session)
	if bound == "" && stateErr != nil {
		return nil
	}
	if stateErr != nil || (bound != "" && state.Route != bound) {
		return fmt.Errorf("stale tailgate route marker; refusing to remove recovery state")
	}
	return releaseTailgateSession(session, state.Route)
}

func commitTailgateLaunch(session, name string, state tailgateState) error {
	lock, err := acquireTailgateSessionLock(session)
	if err != nil {
		return err
	}
	defer releaseTailgateSessionLock(lock, session)
	current, stateErr := readTailgateState(session)
	if stateErr != nil || current.Route != name || current.Port != state.Port {
		return fmt.Errorf("tailgate recovery state changed during browser launch")
	}
	state.Route = name
	state.BrowserPort = state.Port
	if state.Profile == "" {
		state.Profile = "default"
	}
	raw, _ := json.Marshal(state)
	if err := secureTailgateWrite(tailgateStatePath(session), raw); err != nil {
		return err
	}
	return secureTailgateWrite(tailgateBindingPath(session), []byte(name))
}

func secureTailgateWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tailgate-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func directRelaunch(session string) error {
	launch := map[string]interface{}{"id": generateID(), "action": "launch"}
	raw, _ := json.Marshal(launch)
	respRaw, err := SendCommand(session, raw, defaultTimeout)
	if err != nil {
		return err
	}
	var resp Response
	if err := json.Unmarshal(respRaw, &resp); err != nil || !resp.IsSuccess() {
		return fmt.Errorf("failed to switch session to direct browser egress")
	}
	return commitDirectLaunchProfile(session, "")
}

func tailgateBrowserLaunched(session string) bool {
	status, err := queryRuntimeStatus(session)
	if err != nil {
		return false
	}
	data, ok := status.(map[string]interface{})
	if !ok {
		return false
	}
	launched, _ := data["launched"].(bool)
	return launched
}

func readTailgateState(session string) (tailgateState, error) {
	var state tailgateState
	if err := checkTailgateDir(); err != nil {
		return state, err
	}
	info, statErr := os.Lstat(tailgateStatePath(session))
	if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return state, fmt.Errorf("tailgate state is unavailable or insecure")
	}
	raw, err := os.ReadFile(tailgateStatePath(session))
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(raw, &state)
	if err == nil {
		if validateTailgateRouteName(state.Route) != nil || state.Port < 1 || state.Port > 65535 || validateTailgateProfile(state.Profile) != nil {
			return tailgateState{}, fmt.Errorf("tailgate state is invalid")
		}
	}
	return state, err
}

func readTailgateBinding(session string) (string, error) {
	if err := checkTailgateDir(); err != nil {
		return "", err
	}
	path := tailgateBindingPath(session)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("tailgate binding is unavailable or insecure")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(raw))
	if err := validateTailgateRouteName(name); err != nil {
		return "", fmt.Errorf("tailgate binding is invalid")
	}
	return name, nil
}

func tailgateDoctorData(session, requested string) map[string]interface{} {
	name := requested
	if name == "" {
		if selected, enabled := selectedTailgate(GlobalFlags{Session: session}); enabled {
			name = selected
		}
		if state, err := readTailgateState(session); err == nil {
			name = state.Route
		} else {
			if name == "" {
				name = "default"
			}
		}
	}
	route, routeErr := loadTailgateRoute(name)
	_, sshErr := exec.LookPath("ssh")
	runtimeErr := checkTailgateDir()
	data := map[string]interface{}{"session": session, "route": name, "configured": routeErr == nil, "sshAvailable": sshErr == nil, "running": false, "socksReady": false, "loopbackOnly": true, "strictHostKeyChecking": true, "keyOnly": true, "controlEndpointLocal": true, "runtimeDirSecure": runtimeErr == nil, "stateValid": false, "staleState": false, "bindingPresent": false, "bindingConsistent": true}
	state, stateErr := readTailgateState(session)
	if stateErr == nil {
		data["stateValid"] = true
		if name == "" {
			name = state.Route
		}
	}
	if routeErr == nil {
		data["running"] = tunnelAlive(route, tailgateControlPath(session, name))
		if running, _ := data["running"].(bool); running && stateErr == nil {
			data["socksReady"] = tailgateSOCKSReady(state.Port)
		} else {
			data["socksReady"] = false
		}
	}
	if routeErr != nil {
		data["configError"] = routeErr.Error()
	}
	if runtimeErr != nil {
		data["runtimeError"] = runtimeErr.Error()
		data["staleState"] = true
	}
	if stateErr != nil && fileExists(tailgateStatePath(session)) {
		data["staleState"] = true
	}
	bound, bindingErr := readTailgateBinding(session)
	data["bindingPresent"] = bound != ""
	if bindingErr != nil || (bound != "" && stateErr != nil) || (bound != "" && stateErr == nil && state.Route != bound) {
		data["staleState"] = true
		data["bindingConsistent"] = false
	} else {
		data["bindingConsistent"] = true
	}
	return data
}

func newTailgateCLIError(message string, details map[string]interface{}) *CLIError {
	return &CLIError{
		Code: "tailgate_error", Message: message,
		Hint:  "Inspect the redacted doctor fields, repair the local route, then retry.",
		Retry: true, Details: details, ExitCode: ExitInternal,
	}
}

func handleTailgateCommand(action string, command map[string]interface{}, flags GlobalFlags) error {
	if flags.DryRun && action == "tailgate_stop" {
		printSpecialResponse(flags, map[string]interface{}{"session": flags.Session, "planned": true, "action": "stop tailgate tunnel"})
		return nil
	}
	requested, _ := command["route"].(string)
	if state, err := readTailgateState(flags.Session); err == nil && requested != "" && requested != state.Route {
		return fmt.Errorf("tailgate route does not match the session's active route")
	}
	data := tailgateDoctorData(flags.Session, requested)
	if action == "tailgate_doctor" {
		configured, _ := data["configured"].(bool)
		sshAvailable, _ := data["sshAvailable"].(bool)
		if !configured || !sshAvailable {
			return newTailgateCLIError("tailgate doctor found an unusable route; run 'cloak-agent tailgate setup' and verify OpenSSH", data)
		}
		if secure, _ := data["runtimeDirSecure"].(bool); !secure {
			return newTailgateCLIError("tailgate runtime directory is insecure; repair its permissions before retrying", data)
		}
		if stale, _ := data["staleState"].(bool); stale {
			return newTailgateCLIError("tailgate doctor found stale recovery state; run 'cloak-agent tailgate stop' after checking the local tunnel", data)
		}
		running, _ := data["running"].(bool)
		valid, _ := data["stateValid"].(bool)
		consistent, _ := data["bindingConsistent"].(bool)
		if !consistent || (running && !valid) {
			return newTailgateCLIError("tailgate doctor found inconsistent recovery state; run 'cloak-agent tailgate stop' after checking the local tunnel", data)
		}
		if valid {
			ready, _ := data["socksReady"].(bool)
			if !running || !ready {
				return newTailgateCLIError("tailgate doctor found a dead or unavailable SOCKS tunnel; run 'cloak-agent tailgate stop' then retry", data)
			}
		}
	}
	if action == "tailgate_stop" {
		name, _ := data["route"].(string)
		boundName, bindingErr := readTailgateBinding(flags.Session)
		if bindingErr != nil {
			return bindingErr
		}
		if state, stateErr := readTailgateState(flags.Session); stateErr == nil {
			if boundName != "" && state.Route != boundName {
				return fmt.Errorf("tailgate binding and recovery state disagree")
			}
			if name != "" && state.Route != name {
				return fmt.Errorf("tailgate route does not match the session's active route")
			}
			name = state.Route
		} else if boundName != "" || fileExists(tailgateStatePath(flags.Session)) {
			return fmt.Errorf("stale tailgate route marker; refusing to remove recovery state")
		}
		// Stop is intentionally idempotent for a session that has never used
		// Tailgate. In particular, a fresh clone should be able to run cleanup
		// before any route has been configured; do not attempt to load a
		// non-existent default route in that case.
		if boundName == "" && !fileExists(tailgateStatePath(flags.Session)) {
			if running, _ := data["running"].(bool); !running {
				data["running"] = false
				data["stopped"] = true
				printSpecialResponse(flags, data)
				return nil
			}
		}
		if name == "" {
			data["stopped"] = true
			data["running"] = false
			printSpecialResponse(flags, data)
			return nil
		}
		if err := releaseTailgateSession(flags.Session, name); err != nil {
			return err
		}
		for _, path := range []string{tailgateStatePath(flags.Session), tailgateBindingPath(flags.Session)} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove tailgate recovery state")
			}
			if fileExists(path) {
				return fmt.Errorf("tailgate recovery state remains after stop")
			}
		}
		data["running"] = false
		data["stopped"] = true
	}
	printSpecialResponse(flags, data)
	return nil
}

func handleTailgateSetup(action string, command map[string]interface{}, flags GlobalFlags) error {
	name := "default"
	if value, ok := command["route"].(string); ok && strings.TrimSpace(value) != "" {
		name = value
	}
	if err := validateTailgateProfile(name); err != nil {
		return fmt.Errorf("invalid tailgate route name")
	}
	var route tailgateRoute
	if action == "tailgate_import" {
		var err error
		route, err = discoverBrowserHarnessTailgate()
		if err != nil {
			return fmt.Errorf("Browser Harness Tailgate migration is unavailable")
		}
	} else {
		host, _ := command["host"].(string)
		identity, _ := command["identityFile"].(string)
		known, _ := command["knownHostsFile"].(string)
		route = tailgateRoute{SSHHost: host, IdentityFile: identity, KnownHostsFile: known}
		route.SSHUser, _ = command["user"].(string)
		route.SSHConfig, _ = command["sshConfig"].(string)
		route.UseAgent, _ = command["useAgent"].(bool)
		if value, ok := command["port"].(string); ok {
			port, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("tailgate setup port must be numeric")
			}
			route.SSHPort = port
		}
	}
	if route.SSHHost == "" || ((!route.UseAgent && route.IdentityFile == "") && route.SSHConfig == "") || (route.KnownHostsFile == "" && route.SSHConfig == "") {
		return fmt.Errorf("tailgate setup requires host, key path, and known-hosts path")
	}
	route.IdentityFile = expandUserPath(route.IdentityFile)
	route.KnownHostsFile = expandUserPath(route.KnownHostsFile)
	if route.SSHConfig != "" {
		route.SSHConfig = expandUserPath(route.SSHConfig)
	}
	var err error
	if route, err = validateTailgateRoute(route); err != nil {
		return err
	}
	if flags.DryRun {
		printSpecialResponse(flags, map[string]interface{}{"planned": true, "action": action, "route": name, "configured": true, "mutates": false})
		return nil
	}
	configPath := tailgateConfigPath()
	config := tailgateConfig{Routes: map[string]tailgateRoute{}}
	if info, statErr := os.Lstat(configPath); statErr == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
			return fmt.Errorf("existing tailgate config is insecure")
		}
		raw, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return fmt.Errorf("existing tailgate config could not be read")
		}
		if err := json.Unmarshal(raw, &config); err != nil {
			return fmt.Errorf("existing tailgate config is invalid")
		}
		if config.Routes == nil {
			config.Routes = map[string]tailgateRoute{}
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("existing tailgate config could not be inspected")
	}
	config.Routes[name] = route
	raw, _ := json.MarshalIndent(config, "", "  ")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return err
	}
	if info, statErr := os.Stat(filepath.Dir(configPath)); statErr != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("tailgate config directory must not be accessible by group or other users")
	}
	if err := secureTailgateWrite(configPath, raw); err != nil {
		return err
	}
	data := map[string]interface{}{"configured": true, "route": name, "source": action, "keyOnly": true, "strictHostKeyChecking": true, "loopbackOnly": true}
	printSpecialResponse(flags, data)
	return nil
}

func validateTailgateRoute(route tailgateRoute) (tailgateRoute, error) {
	if strings.TrimSpace(route.SSHHost) == "" {
		return route, fmt.Errorf("tailgate SSH host is required")
	}
	if strings.HasPrefix(route.SSHHost, "-") || strings.ContainsAny(route.SSHHost, "\r\n\x00") {
		return route, fmt.Errorf("tailgate SSH host is invalid")
	}
	if strings.HasPrefix(route.SSHUser, "-") || strings.ContainsAny(route.SSHUser, "\r\n\x00") {
		return route, fmt.Errorf("tailgate SSH user is invalid")
	}
	if strings.HasPrefix(route.SSHHostAlias, "-") || strings.ContainsAny(route.SSHHostAlias, "\r\n\x00") {
		return route, fmt.Errorf("tailgate SSH host alias is invalid")
	}
	if route.SSHConfig != "" {
		alias := route.SSHHost
		resolved, err := resolveSSHRoute(route.SSHHost, route.SSHConfig)
		if err != nil {
			return route, err
		}
		route.SSHHost, route.SSHUser, route.SSHPort = resolved.SSHHost, resolved.SSHUser, resolved.SSHPort
		route.SSHHostAlias = resolved.SSHHostAlias
		if route.SSHHostAlias == "" {
			route.SSHHostAlias = sshHostAlias(alias)
		}
		if route.IdentityFile == "" && !route.UseAgent {
			route.IdentityFile = resolved.IdentityFile
		}
		if route.UseAgent {
			route.IdentityFile = ""
		}
		if route.KnownHostsFile == "" {
			route.KnownHostsFile = resolved.KnownHostsFile
		}
		route.SSHConfig = ""
	}
	if route.SSHPort == 0 {
		route.SSHPort = 22
	}
	if route.SSHPort < 1 || route.SSHPort > 65535 {
		return route, fmt.Errorf("tailgate SSH port is invalid")
	}
	for label, file := range map[string]string{"identityFile": route.IdentityFile, "knownHostsFile": route.KnownHostsFile} {
		if file == "" {
			if label == "knownHostsFile" {
				return route, fmt.Errorf("tailgate knownHostsFile is required")
			}
			continue
		}
		if err := validateTailgateFile(file, label == "identityFile"); err != nil {
			return route, fmt.Errorf("tailgate %s is unsafe: %w", label, err)
		}
	}
	return route, nil
}
