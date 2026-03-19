package cmd

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nerveband/cloak-agent/cmd/update"
)

var Version = "0.1.1"

func Execute(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	// Handle --version
	if args[0] == "--version" || args[0] == "-v" {
		fmt.Printf("cloak-agent v%s\n", Version)
		return nil
	}

	// Handle --help
	if args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}

	// Handle install subcommand
	if args[0] == "install" {
		return handleInstall()
	}

	// Handle upgrade subcommand
	if args[0] == "upgrade" {
		_, err := update.Upgrade(Version)
		return err
	}

	// Handle version subcommand
	if args[0] == "version" {
		fmt.Printf("cloak-agent v%s\n", Version)
		return nil
	}

	// Start async update check for non-meta commands
	var updateCh <-chan update.CheckResult
	if update.ShouldCheckUpdates(args) {
		updateCh = update.CheckAsync(Version)
	}

	// Parse global flags
	flags, remaining := ParseGlobalFlags(args)

	if len(remaining) == 0 && flags.InputFile == "" && flags.InputMode == "" {
		printUsage()
		return nil
	}

	var command map[string]interface{}
	var err error

	switch {
	case flags.InputFile != "":
		payload, readErr := os.ReadFile(flags.InputFile)
		if readErr != nil {
			return fmt.Errorf("failed to read input file: %w", readErr)
		}
		command, err = ParseRawJSON(string(payload))
		if err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", flags.InputFile, err)
		}
	case flags.InputMode == "json":
		payload, readErr := ioReadAll(os.Stdin)
		if readErr != nil {
			return fmt.Errorf("failed to read JSON input from stdin: %w", readErr)
		}
		command, err = ParseRawJSON(strings.TrimSpace(string(payload)))
		if err != nil {
			return fmt.Errorf("invalid JSON from stdin: %w", err)
		}
	case len(remaining) > 0 && len(remaining[0]) > 0 && remaining[0][0] == '{':
		command, err = ParseRawJSON(remaining[0])
		if err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
	default:
		command, err = ParseArgs(remaining)
		if err != nil {
			return err
		}
	}

	// Handle special non-daemon commands
	if action, ok := command["action"].(string); ok {
		switch action {
		case "session_list":
			return handleSessionList(flags)
		case "daemon_start":
			return handleDaemonStart(flags)
		case "daemon_stop":
			return handleDaemonStop(flags)
		case "daemon_restart":
			return handleDaemonRestart(flags)
		case "daemon_status":
			return handleDaemonStatus(flags)
		case "daemon_logs":
			return handleDaemonLogs(flags)
		}
	}

	// Add command ID
	command["id"] = generateID()

	// Add dry-run flag
	if flags.DryRun {
		command["dryRun"] = true
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	// Send to daemon
	timeout := time.Duration(flags.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = defaultTimeout
	}

	respBytes, err := SendCommand(flags.Session, jsonBytes, timeout)
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return fmt.Errorf("invalid response from daemon: %w", err)
	}

	// Format and print
	PrintResponse(resp, flags)

	// Show update notice if available (non-blocking)
	if updateCh != nil {
		select {
		case result := <-updateCh:
			if notice := update.FormatNotice(result); notice != "" {
				fmt.Fprint(os.Stderr, notice)
			}
		default:
		}
	}

	if !resp.IsSuccess() {
		os.Exit(1)
	}

	return nil
}


func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func handleInstall() error {
	if projectDir := findSourceProjectDir(); projectDir != "" {
		installScript := filepath.Join(projectDir, "scripts", "install.sh")
		cmd := exec.Command(installScript)
		cmd.Dir = projectDir
		cmd.Env = append(os.Environ(), "CLOAK_AGENT_INSTALL_DIR="+GetAppDir())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("==> source install from %s\n", projectDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("source install failed: %w", err)
		}
		return nil
	}

	daemonDir := findInstalledDaemonDir()
	if daemonDir == "" {
		return fmt.Errorf("install requires either a cloak-agent source checkout (with scripts/install.sh) or an installed daemon layout under %s", GetAppDir())
	}

	steps := []struct {
		name string
		cmd  *exec.Cmd
	}{
		{"npm install --omit=dev", exec.Command("npm", "install", "--omit=dev")},
		{"cloakbrowser install", exec.Command("npx", "cloakbrowser", "install")},
	}

	for _, step := range steps {
		step.cmd.Dir = daemonDir
		step.cmd.Stdout = os.Stdout
		step.cmd.Stderr = os.Stderr
		fmt.Printf("==> %s\n", step.name)
		if err := step.cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", step.name, err)
		}
	}

	fmt.Printf("cloak-agent install complete (daemon dir: %s).\n", daemonDir)
	return nil
}

func handleSessionList(flags GlobalFlags) error {
	// List .sock files in socket dir
	dir := GetSocketDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("No active sessions")
		return nil
	}
	found := false
	for _, e := range entries {
		name := e.Name()
		if len(name) > 5 && name[len(name)-5:] == ".sock" {
			session := name[:len(name)-5]
			running := IsDaemonRunning(session)
			status := "stopped"
			if running {
				status = "running"
			}
			fmt.Printf("  %s (%s)\n", session, status)
			found = true
		}
	}
	if !found {
		fmt.Println("No active sessions")
	}
	return nil
}

func printSpecialResponse(flags GlobalFlags, data interface{}) {
	PrintResponse(Response{Success: true, Data: data}, flags)
}

func daemonStatusData(session string) map[string]interface{} {
	running := IsDaemonRunning(session)
	status := "stopped"
	if running {
		status = "running"
	}
	return map[string]interface{}{
		"session": session,
		"status":  status,
		"socket":  GetSocketPath(session),
		"pidfile": GetPidFile(session),
		"log":     GetLogFile(session),
	}
}

func handleDaemonStart(flags GlobalFlags) error {
	alreadyRunning := IsDaemonRunning(flags.Session)
	if !alreadyRunning {
		if err := StartDaemon(flags.Session); err != nil {
			return err
		}
	}
	data := daemonStatusData(flags.Session)
	data["message"] = fmt.Sprintf("cloak-agent daemon %s for session %q", map[bool]string{true: "already running", false: "started"}[alreadyRunning], flags.Session)
	printSpecialResponse(flags, data)
	return nil
}

func handleDaemonStop(flags GlobalFlags) error {
	if err := StopDaemon(flags.Session); err != nil {
		return err
	}
	data := daemonStatusData(flags.Session)
	data["message"] = fmt.Sprintf("cloak-agent daemon stopped for session %q", flags.Session)
	printSpecialResponse(flags, data)
	return nil
}

func handleDaemonRestart(flags GlobalFlags) error {
	_ = StopDaemon(flags.Session)
	if err := StartDaemon(flags.Session); err != nil {
		return err
	}
	data := daemonStatusData(flags.Session)
	data["message"] = fmt.Sprintf("cloak-agent daemon restarted for session %q", flags.Session)
	printSpecialResponse(flags, data)
	return nil
}

func handleDaemonStatus(flags GlobalFlags) error {
	printSpecialResponse(flags, daemonStatusData(flags.Session))
	return nil
}

func handleDaemonLogs(flags GlobalFlags) error {
	data, err := os.ReadFile(GetLogFile(flags.Session))
	if err != nil {
		return fmt.Errorf("failed to read daemon log: %w", err)
	}
	if flags.JSONOutput {
		printSpecialResponse(flags, map[string]interface{}{"session": flags.Session, "log": string(data)})
		return nil
	}
	fmt.Print(string(data))
	return nil
}

func ioReadAll(f *os.File) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(f)
	return buf.Bytes(), err
}

func printUsage() {
	fmt.Println(`cloak-agent - stealth browser automation CLI for AI agents

Usage:
  cloak-agent <command> [args...]
  cloak-agent --output json <command> [args...]
  cloak-agent --input json [--output json] < payload.json
  cloak-agent --input-file payload.json [--output json]
  cloak-agent --json '{"action":"navigate","url":"..."}'   # legacy shorthand

Navigation:
  open <url>                     Navigate to URL
  launch [url] [flags...]        Launch browser/session with CloakBrowser options
  back, forward, reload          History navigation
  close                          Close browser and daemon

Interaction:
  click <ref>                    Click element
  fill <ref> <text>              Fill input field
  type <ref> <text>              Type text (keystroke by keystroke)
  press <key>                    Press keyboard key
  hover, focus, check, uncheck   Element interactions
  select <ref> <value>           Select dropdown option
  scroll down|up <amount>        Scroll page

Inspection:
  snapshot [-i] [-c] [-d N]      Get page structure with @refs
  get title|url|text|html|value  Get page/element info
  screenshot [path] [--full]     Take screenshot
  is visible|enabled|checked     Check element state

Daemon / sessions:
  daemon start|stop|restart      Manage persistent daemon for a session
  daemon status|logs             Inspect daemon state and logs
  session list                   List known sessions

Stealth (cloak-agent exclusive):
  stealth status                 Run bot detection tests
  fingerprint rotate [--seed N]  New browser fingerprint
  profile create <name>          Create persistent profile
  profile list                   List profiles

Schema (for AI agents):
  schema                         List all available commands
  schema <command>               Show command parameters

Updates:
  install                        Bootstrap source checkout or installed daemon deps/browser
  upgrade                        Upgrade to the latest version
  version                        Print version

Global Flags:
  --session <name>               Use named session (default: "default")
  --output json                  Stable machine-readable output
  --json                         Alias for --output json
  --input json                   Read command JSON from stdin
  --input-file <path>            Read command JSON from file
  --timeout <ms>                 Command timeout
  --headed                       Show browser window
  --dry-run                      Validate without executing
  --fields <list>                Limit response fields (human mode)

Launch flags:
  --profile <name>               Persistent profile name
  --proxy <url>                  Proxy server
  --timezone <tz>                Context timezone, e.g. America/New_York
  --locale <tag>                 Locale, e.g. en-US
  --viewport <WxH>               Viewport, e.g. 1440x900
  --geoip                        Align geolocation with proxy/IP
  --fingerprint-seed <n>         Deterministic fingerprint seed
  --platform <name>              Override platform hint
  --gpu-vendor <name>            Override GPU vendor
  --gpu-renderer <name>          Override GPU renderer
  --user-agent <ua>              Override user agent
  --arg <flag>                   Extra Chromium/CloakBrowser arg (repeatable)

Made by Ashraf (https://ashrafali.net)`)
}
