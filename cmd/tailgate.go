package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type tailgateConfig struct {
	Routes map[string]tailgateRoute `json:"routes"`
}

type tailgateRoute struct {
	SSHHost        string `json:"sshHost"`
	IdentityFile   string `json:"identityFile"`
	KnownHostsFile string `json:"knownHostsFile"`
}

type tailgateState struct {
	Route   string `json:"route"`
	Port    int    `json:"port"`
	Profile string `json:"profile,omitempty"`
}

func tailgateConfigPath() string {
	if value := os.Getenv("CLOAK_AGENT_TAILGATE_CONFIG"); value != "" {
		return expandUserPath(value)
	}
	return filepath.Join(GetAppDir(), "tailgate.json")
}

func tailgateDir() string { return filepath.Join(GetSocketDir(), "tailgate") }

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

func expandUserPath(value string) string {
	if value == "~" || strings.HasPrefix(value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	return value
}

func loadTailgateRoute(name string) (tailgateRoute, error) {
	path := tailgateConfigPath()
	info, err := os.Stat(path)
	if err != nil {
		return tailgateRoute{}, fmt.Errorf("tailgate config unavailable at %s", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return tailgateRoute{}, fmt.Errorf("tailgate config must not be accessible by group or other users: chmod 600 %s", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return tailgateRoute{}, fmt.Errorf("failed to read tailgate config: %w", err)
	}
	var config tailgateConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return tailgateRoute{}, fmt.Errorf("invalid tailgate config JSON: %w", err)
	}
	route, ok := config.Routes[name]
	if !ok {
		return tailgateRoute{}, fmt.Errorf("tailgate route %q is not configured", name)
	}
	route.IdentityFile = expandUserPath(route.IdentityFile)
	route.KnownHostsFile = expandUserPath(route.KnownHostsFile)
	if strings.TrimSpace(route.SSHHost) == "" || route.IdentityFile == "" || route.KnownHostsFile == "" {
		return tailgateRoute{}, fmt.Errorf("tailgate route %q requires sshHost, identityFile, and knownHostsFile", name)
	}
	if strings.HasPrefix(route.SSHHost, "-") || strings.ContainsAny(route.SSHHost, "\r\n\x00") {
		return tailgateRoute{}, fmt.Errorf("tailgate sshHost is invalid")
	}
	for label, file := range map[string]string{"identityFile": route.IdentityFile, "knownHostsFile": route.KnownHostsFile} {
		if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
			return tailgateRoute{}, fmt.Errorf("tailgate %s is not a regular file", label)
		}
	}
	if info, err := os.Stat(route.IdentityFile); err != nil || info.Mode().Perm()&0o077 != 0 {
		return tailgateRoute{}, fmt.Errorf("tailgate identityFile must not be accessible by group or other users")
	}
	return route, nil
}

func selectedTailgate(flags GlobalFlags) (string, bool) {
	if flags.Tailgate {
		if flags.TailgateRoute != "" {
			return flags.TailgateRoute, true
		}
		return "default", true
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

func tailgatePort(session, route string) int {
	sum := sha256.Sum256([]byte("tailgate\x00" + session + "\x00" + route))
	return 32768 + int(sum[0])<<4 + int(sum[1]&0x0f)
}

func sshArgs(route tailgateRoute, control string) []string {
	return []string{
		"-F", "/dev/null",
		"-i", route.IdentityFile,
		"-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes",
		"-o", "PasswordAuthentication=no", "-o", "KbdInteractiveAuthentication=no",
		"-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=" + route.KnownHostsFile,
		"-o", "ExitOnForwardFailure=yes", "-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3", "-o", "ForwardAgent=no",
		"-o", "PermitLocalCommand=no", "-o", "ControlMaster=yes", "-S", control,
	}
}

func tunnelAlive(route tailgateRoute, control string) bool {
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return false
	}
	cmd := exec.Command(ssh, append(sshArgs(route, control), "-O", "check", route.SSHHost)...)
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
	if err := os.MkdirAll(tailgateDir(), 0o700); err != nil {
		return tailgateState{}, err
	}
	control := tailgateControlPath(session, name)
	state := tailgateState{Route: name, Port: tailgatePort(session, name)}
	if tunnelAlive(route, control) {
		if saved, readErr := readTailgateState(session); readErr == nil && saved.Route == name {
			return saved, nil
		}
		return state, nil
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", state.Port))
	if err != nil {
		return tailgateState{}, fmt.Errorf("tailgate loopback port is unavailable for this session")
	}
	_ = listener.Close()
	args := append(sshArgs(route, control), "-f", "-N", "-T", "-D", fmt.Sprintf("127.0.0.1:%d", state.Port), route.SSHHost)
	if err := exec.Command(ssh, args...).Run(); err != nil {
		return tailgateState{}, fmt.Errorf("tailgate SSH tunnel failed; run 'cloak-agent tailgate doctor %s'", name)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tunnelAlive(route, control) {
			raw, _ := json.Marshal(state)
			if err := os.WriteFile(tailgateStatePath(session), raw, 0o600); err != nil {
				return tailgateState{}, err
			}
			return state, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return tailgateState{}, fmt.Errorf("tailgate SSH control endpoint did not become ready")
}

func isolatedTailgateProfile(route, profile string) string {
	if profile == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(route))
	return "tailgate-" + hex.EncodeToString(sum[:4]) + "-" + profile
}

func validateTailgateProfile(profile string) error {
	if profile == "" {
		return nil
	}
	if profile == "." || profile == ".." || strings.ContainsAny(profile, `/\\`) {
		return fmt.Errorf("tailgate profile must be a simple name without path separators")
	}
	return nil
}

func prepareTailgate(command map[string]interface{}, flags GlobalFlags) error {
	name, enabled := selectedTailgate(flags)
	action, _ := command["action"].(string)
	if !enabled {
		if action == "launch" {
			_ = os.Remove(tailgateBindingPath(flags.Session))
		}
		return nil
	}
	if strings.HasPrefix(action, "tailgate_") {
		return nil
	}
	if action != "launch" && action != "navigate" {
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
	state, err := ensureTailgateTunnel(flags.Session, name)
	if err != nil {
		return err
	}
	if profile == "" {
		profile = state.Profile
	}
	proxy := fmt.Sprintf("socks5://127.0.0.1:%d", state.Port)
	originalProfile := profile
	profile = isolatedTailgateProfile(name, profile)
	if action == "launch" {
		command["proxy"] = proxy
		if profile != "" {
			command["profile"] = profile
		}
		state.Profile = originalProfile
		raw, _ := json.Marshal(state)
		if err := os.WriteFile(tailgateStatePath(flags.Session), raw, 0o600); err != nil {
			return err
		}
		return os.WriteFile(tailgateBindingPath(flags.Session), []byte(name), 0o600)
	}
	bound, _ := os.ReadFile(tailgateBindingPath(flags.Session))
	if strings.TrimSpace(string(bound)) == name && tailgateBrowserLaunched(flags.Session) {
		return nil
	}
	launch := map[string]interface{}{"id": generateID(), "action": "launch", "proxy": proxy}
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
	return os.WriteFile(tailgateBindingPath(flags.Session), []byte(name), 0o600)
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
	raw, err := os.ReadFile(tailgateStatePath(session))
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(raw, &state)
	return state, err
}

func tailgateDoctorData(session, requested string) map[string]interface{} {
	name := requested
	if name == "" {
		if state, err := readTailgateState(session); err == nil {
			name = state.Route
		} else {
			name = "default"
		}
	}
	route, routeErr := loadTailgateRoute(name)
	_, sshErr := exec.LookPath("ssh")
	data := map[string]interface{}{"session": session, "route": name, "configured": routeErr == nil, "sshAvailable": sshErr == nil, "running": false, "loopbackOnly": true, "strictHostKeyChecking": true, "keyOnly": true}
	if routeErr == nil {
		data["running"] = tunnelAlive(route, tailgateControlPath(session, name))
	}
	if routeErr != nil {
		data["configError"] = routeErr.Error()
	}
	return data
}

func handleTailgateCommand(action string, command map[string]interface{}, flags GlobalFlags) error {
	requested, _ := command["route"].(string)
	data := tailgateDoctorData(flags.Session, requested)
	if action == "tailgate_stop" {
		name, _ := data["route"].(string)
		route, err := loadTailgateRoute(name)
		if err != nil {
			return err
		}
		if ssh, err := exec.LookPath("ssh"); err == nil && tunnelAlive(route, tailgateControlPath(flags.Session, name)) {
			if err := exec.Command(ssh, append(sshArgs(route, tailgateControlPath(flags.Session, name)), "-O", "exit", route.SSHHost)...).Run(); err != nil {
				return fmt.Errorf("failed to stop tailgate tunnel")
			}
			if tunnelAlive(route, tailgateControlPath(flags.Session, name)) {
				return fmt.Errorf("tailgate tunnel is still running after stop request")
			}
		}
		_ = os.Remove(tailgateStatePath(flags.Session))
		_ = os.Remove(tailgateBindingPath(flags.Session))
		data["running"] = false
		data["stopped"] = true
	}
	printSpecialResponse(flags, data)
	return nil
}
