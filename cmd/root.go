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

var Version = "0.4.0"

func Execute(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	flags, remaining := ParseGlobalFlags(args)
	if flags.JSONOutput || (!flags.HumanOutput && WantsJSON(args)) {
		flags.JSONOutput = true
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
	for _, arg := range args[1:] {
		if arg == "--help" || arg == "-h" {
			printUsage()
			return nil
		}
	}

	// Handle install subcommand
	if args[0] == "install" {
		return handleInstall()
	}

	// Handle upgrade subcommand
	if args[0] == "upgrade" {
		if _, err := update.Upgrade(Version); err != nil {
			return err
		}
		fmt.Println("Bootstrapping daemon dependencies and CloakBrowser runtime...")
		return handleInstall()
	}

	// Handle version subcommand
	if args[0] == "version" {
		fmt.Printf("cloak-agent v%s\n", Version)
		return nil
	}

	// Start async update check for non-meta commands
	var updateCh <-chan update.CheckResult
	if update.ShouldCheckUpdates(args) && !flags.Quiet {
		updateCh = update.CheckAsync(Version)
	}

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
	if handled, specialErr := executeSpecialCommand(command, flags); handled {
		return specialErr
	}

	ensureCommandID(command)
	if err := applyGlobalCommandFlags(command, flags); err != nil {
		return err
	}
	if err := prepareTailgate(command, flags); err != nil {
		return err
	}
	pendingTailgateRoute, _ := command["_tailgateRoute"].(string)
	pendingTailgateProfile, _ := command["_tailgateProfile"].(string)
	delete(command, "_tailgateRoute")
	delete(command, "_tailgateProfile")

	var restoreHeadedEnv func()
	if action, ok := command["action"].(string); ok && flags.Headed && action != "launch" {
		restoreHeadedEnv = setTemporaryEnv("CLOAK_AGENT_HEADED", "1")
		defer restoreHeadedEnv()
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
		if pendingTailgateRoute != "" {
			_ = releaseTailgateSession(flags.Session, pendingTailgateRoute)
		}
		return fmt.Errorf("failed to send command: %w", err)
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		if pendingTailgateRoute != "" {
			_ = releaseTailgateSession(flags.Session, pendingTailgateRoute)
		}
		return fmt.Errorf("invalid response from daemon: %w", err)
	}

	if !resp.IsSuccess() {
		if pendingTailgateRoute != "" {
			// The tunnel is only useful to a successfully launched browser. Best
			// effort cleanup is intentionally performed before returning the daemon
			// error; a later doctor/stop still fails closed if it cannot verify it.
			_ = releaseTailgateSession(flags.Session, pendingTailgateRoute)
		}
		return NewCLIError("command_failed", resp.Error, "Inspect the command response, re-snapshot stale refs, or run with --dry-run before mutating actions.", false, ExitInternal)
	}
	if pendingTailgateRoute != "" {
		state, stateErr := readTailgateState(flags.Session)
		if stateErr != nil {
			_ = releaseTailgateSession(flags.Session, pendingTailgateRoute)
			return fmt.Errorf("tailgate launch succeeded but its recovery state could not be recorded")
		}
		state.Profile = pendingTailgateProfile
		if err := commitTailgateLaunch(flags.Session, pendingTailgateRoute, state); err != nil {
			_ = releaseTailgateSession(flags.Session, pendingTailgateRoute)
			return fmt.Errorf("tailgate launch succeeded but its recovery state could not be recorded")
		}
	}
	if action, _ := command["action"].(string); action == "launch" && pendingTailgateRoute == "" && !flags.DryRun {
		// Direct launches keep only the named profile in a private per-session
		// descriptor. Proxy/host material is never persisted here; routed launches
		// use the separate Tailgate descriptor above.
		profile, _ := command["profile"].(string)
		if err := commitDirectLaunchProfile(flags.Session, profile); err != nil {
			return fmt.Errorf("direct launch succeeded but its profile state could not be recorded")
		}
	}
	if action, _ := command["action"].(string); action == "close" && !flags.DryRun {
		if err := cleanupTailgateAfterClose(flags.Session); err != nil {
			return err
		}
	}

	// Format and print only after all local recovery bookkeeping succeeds.
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

	return nil
}

func executeSpecialCommand(command map[string]interface{}, flags GlobalFlags) (bool, error) {
	action, ok := command["action"].(string)
	if !ok {
		return false, nil
	}

	switch action {
	case "session_list":
		return true, handleSessionList(flags)
	case "daemon_start":
		return true, handleDaemonStart(flags)
	case "daemon_stop":
		return true, handleDaemonStop(flags)
	case "daemon_restart":
		return true, handleDaemonRestart(flags)
	case "daemon_status":
		return true, handleDaemonStatus(flags)
	case "daemon_logs":
		return true, handleDaemonLogs(flags)
	case "doctor":
		return true, handleDoctor(flags)
	case "tailgate_status", "tailgate_stop", "tailgate_doctor":
		return true, handleTailgateCommand(action, command, flags)
	case "tailgate_setup", "tailgate_import":
		return true, handleTailgateSetup(action, command, flags)
	default:
		return false, nil
	}
}

func ensureCommandID(command map[string]interface{}) {
	if id, ok := command["id"].(string); ok && strings.TrimSpace(id) != "" {
		return
	}
	command["id"] = generateID()
}

func applyGlobalCommandFlags(command map[string]interface{}, flags GlobalFlags) error {
	if flags.DryRun {
		command["dryRun"] = true
	}
	action, _ := command["action"].(string)
	if flags.Headed && action == "launch" {
		command["headless"] = false
	}
	if action == "launch" {
		caCert := flags.CACert
		clearCACert := flags.ClearCACert
		if caCert == "" && !clearCACert {
			caCert = strings.TrimSpace(os.Getenv("CLOAK_AGENT_CA_CERT"))
			clearCACert = envBool("CLOAK_AGENT_CLEAR_CA_CERT")
		}
		if caCert != "" && clearCACert {
			return fmt.Errorf("cannot use --ca-cert with --no-ca-cert")
		}
		if caCert != "" {
			if _, hasProfile := command["profile"]; hasProfile {
				return fmt.Errorf("--ca-cert cannot be combined with --profile")
			}
			if ignore, _ := command["ignoreHTTPSErrors"].(bool); ignore {
				return fmt.Errorf("--ca-cert cannot be combined with --ignore-https-errors")
			}
			command["caCert"] = caCert
		}
		if clearCACert {
			command["clearCaCert"] = true
		}
	}
	if flags.Limit > 0 {
		command["limit"] = flags.Limit
	}
	if flags.CountOnly {
		command["countOnly"] = true
	}
	if flags.IDOnly {
		command["idOnly"] = true
	}
	if flags.Yes {
		command["yes"] = true
	}
	return nil
}

func envBool(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	return value == "1" || strings.EqualFold(value, "true")
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func setTemporaryEnv(key string, value string) func() {
	prev, hadPrev := os.LookupEnv(key)
	os.Setenv(key, value)
	return func() {
		if hadPrev {
			os.Setenv(key, prev)
			return
		}
		os.Unsetenv(key)
	}
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

	for _, binary := range []struct {
		name    string
		message string
	}{
		{"node", "node not found in PATH; install Node.js 20+ to run cloak-agent install"},
		{"npm", "npm not found in PATH; install npm to bootstrap cloak-agent"},
		{"npx", "npx not found in PATH; install npm to run cloakbrowser install"},
	} {
		if _, err := exec.LookPath(binary.name); err != nil {
			return fmt.Errorf("%s", binary.message)
		}
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
	// PID files exist for both Unix-socket and Windows-TCP sessions.
	dir := GetSocketDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if flags.JSONOutput {
			printSpecialResponse(flags, map[string]interface{}{"sessions": []map[string]string{}})
			return nil
		}
		fmt.Println("No active sessions")
		return nil
	}
	sessions := make([]map[string]string, 0)
	found := false
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".pid") {
			session := strings.TrimSuffix(name, ".pid")
			running := IsDaemonRunning(session)
			status := "stopped"
			if running {
				status = "running"
			}
			sessions = append(sessions, map[string]string{
				"session": session,
				"status":  status,
			})
			if !flags.JSONOutput {
				fmt.Printf("  %s (%s)\n", session, status)
			}
			found = true
		}
	}
	if flags.JSONOutput {
		printSpecialResponse(flags, map[string]interface{}{"sessions": sessions})
		return nil
	}
	if !found {
		fmt.Println("No active sessions")
	}
	return nil
}

func printSpecialResponse(flags GlobalFlags, data interface{}) {
	PrintResponse(Response{ID: generateID(), OK: true, Success: true, Data: data}, flags)
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
		if os.IsNotExist(err) {
			payload := map[string]interface{}{"session": flags.Session, "log": "", "missing": true}
			if flags.JSONOutput {
				printSpecialResponse(flags, payload)
				return nil
			}
			fmt.Printf("No daemon log file for session %q yet.\n", flags.Session)
			return nil
		}
		return fmt.Errorf("failed to read daemon log: %w", err)
	}
	if flags.JSONOutput {
		printSpecialResponse(flags, map[string]interface{}{"session": flags.Session, "log": string(data)})
		return nil
	}
	fmt.Print(string(data))
	return nil
}

func handleDoctor(flags GlobalFlags) error {
	nodePath, nodeErr := exec.LookPath("node")
	npmPath, npmErr := exec.LookPath("npm")
	npxPath, npxErr := exec.LookPath("npx")
	daemonDir := findInstalledDaemonDir()
	daemonJS, daemonErr := findDaemonJS()
	cloakBrowserVersion := ""
	if daemonDir != "" {
		if raw, err := os.ReadFile(filepath.Join(daemonDir, "node_modules", "cloakbrowser", "package.json")); err == nil {
			var pkg struct {
				Version string `json:"version"`
			}
			if json.Unmarshal(raw, &pkg) == nil {
				cloakBrowserVersion = pkg.Version
			}
		}
	}
	streamPort := ""
	if raw, err := os.ReadFile(GetStreamPortFile(flags.Session)); err == nil {
		streamPort = strings.TrimSpace(string(raw))
	}
	runtimeStatus, runtimeStatusErr := queryRuntimeStatus(flags.Session)
	data := map[string]interface{}{
		"version":             Version,
		"appDir":              GetAppDir(),
		"socketDir":           GetSocketDir(),
		"session":             flags.Session,
		"daemonRunning":       IsDaemonRunning(flags.Session),
		"streamPort":          streamPort,
		"runtimeStatus":       runtimeStatus,
		"runtimeStatusError":  "",
		"installedDaemon":     daemonDir,
		"daemonJS":            daemonJS,
		"daemonJSError":       "",
		"node":                nodePath,
		"npm":                 npmPath,
		"npx":                 npxPath,
		"cloakBrowserCache":   filepath.Join(os.Getenv("HOME"), ".cloakbrowser"),
		"cloakBrowserWrapper": cloakBrowserVersion,
		"checks": map[string]bool{
			"node":       nodeErr == nil,
			"npm":        npmErr == nil,
			"npx":        npxErr == nil,
			"daemonDir":  daemonDir != "",
			"daemonFile": daemonErr == nil,
		},
		"tailgate": tailgateDoctorData(flags.Session, ""),
	}
	if daemonErr != nil {
		data["daemonJSError"] = daemonErr.Error()
	}
	if runtimeStatusErr != nil {
		data["runtimeStatusError"] = runtimeStatusErr.Error()
	}
	printSpecialResponse(flags, data)
	return nil
}

func queryRuntimeStatus(session string) (interface{}, error) {
	if !IsDaemonRunning(session) {
		return nil, nil
	}
	payload := map[string]interface{}{
		"id":     generateID(),
		"action": "runtime_status",
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	respBytes, err := SendCommand(session, jsonBytes, 2*time.Second)
	if err != nil {
		return nil, err
	}
	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Data, nil
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

Examples:
  cloak-agent open https://example.com
  cloak-agent snapshot -i -c --max-depth 3
  cloak-agent --output json doctor
  cloak-agent --dry-run click @e1

Navigation:
  open [url]                     Launch blank or navigate to URL (goto/navigate aliases)
  launch [url] [flags...]        Launch browser/session with CloakBrowser options
  back, forward, reload          History navigation
  pushstate <url>                SPA navigation without a document reload
  close                          Close browser and daemon

Interaction:
  click <ref>                    Click element
  fill <ref> <text>              Fill input field
  type <ref> <text>              Type text (keystroke by keystroke)
  press <key>                    Press keyboard key
  keydown|keyup <key>            Hold or release a keyboard key
  keyboard type|inserttext <txt> Input text into the focused element
  hover, focus, check, uncheck   Element interactions
  select <ref> <value>           Select dropdown option
  scroll up|down|left|right <n>  Scroll page

Inspection:
  snapshot [-i] [-c] [-d N]      Get page structure with @refs
  snapshot --max-depth N         Alias for snapshot -d N
  read [url]                     Read agent-friendly rendered page text
  get title|url|text|html|value  Get page/element info
  get styles <ref>               Get computed styles
  screenshot [path] [--full]     Take screenshot
  is visible|enabled|checked     Check element state

Daemon / sessions:
  daemon start|stop|restart      Manage persistent daemon for a session
  daemon status|logs             Inspect daemon state and logs
  session list                   List known sessions
  doctor                         Check install, daemon, Node, and browser runtime

Tailgate routing:
  tailgate setup <host> [options]  Create a private SSH/Tailscale route
    --key <path> --known-hosts <path> --user <name> --port <n>
    --agent (use the user's loaded ssh-agent key)
    --ssh-config <path> --route <name>
  tailgate import [--route name]   Import an existing Browser Harness target
  tailgate status [route]        Inspect the session's local SOCKS tunnel
  tailgate stop [route]          Stop the session's local SOCKS tunnel
  tailgate doctor [route]        Validate SSH/config prerequisites

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
  --output human                 Force human output even when stdout is piped
  --json                         Alias for --output json
  --quiet, -q                    Suppress update notices and status noise
  --input json                   Read command JSON from stdin
  --input-file <path>            Read command JSON from file
  --timeout <ms>                 Command timeout
  --headed                       Show browser window
  --dry-run                      Validate without executing
  --yes, -y, --force             Non-interactive confirmation flag
  --tailgate                     Route this browser through the default tailgate
  --tailgate-route <name>        Route through a named configured tailgate
  --direct                       Explicitly select direct browser egress
  --fields <list>                Limit response fields (human mode)
  --limit <n>                    Limit collection output
  --id-only                      Return only identifiers where possible
  --count                        Return counts for collections where possible

Exit codes:
  0 success, 64 validation/input, 69 daemon/browser/network, 70 timeout, 1 internal/command failure

Launch flags:
  --profile <name>               Persistent profile name
  --proxy <url>                  Proxy server
  --timezone <tz>                Context timezone, e.g. America/New_York
  --locale <tag>                 Locale, e.g. en-US
  --viewport <WxH>               Viewport, e.g. 1440x900
  --geoip                        Align geolocation with proxy/IP
  --release-channel <name>       CloakBrowser stable or preview channel
  --browser-version <version>    Pin an exact CloakBrowser binary
  --extension <path>             Load an extension (repeatable)
  --humanize                     Enable human-like mouse/keyboard/scroll behavior
  --human-preset <name>          Human behavior preset: default or careful
  --human-config <json>          Human behavior config JSON object
  --fingerprint-seed <n>         Deterministic fingerprint seed
  --platform <name>              Override platform hint
  --gpu-vendor <name>            Override GPU vendor
  --gpu-renderer <name>          Override GPU renderer
  --user-agent <ua>              Override user agent
  --executable-path <path>       Use a specific browser executable
  --storage-state <path>         Apply Playwright storage state on launch
  --ignore-https-errors          Ignore TLS certificate errors
  --ca-cert <path>               Trust a CA certificate or PEM bundle (Linux; requires certutil)
  --no-ca-cert                   Clear CA trust for the new launch
  --context-options <json>       Extra Playwright context options JSON object
  --arg <flag>                   Extra Chromium/CloakBrowser arg (repeatable)

Made by Ashraf (https://ashrafali.net)`)
}
