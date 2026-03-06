package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nerveband/cloak-agent/cmd/update"
)

var Version = "0.1.0"

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

	if len(remaining) == 0 {
		printUsage()
		return nil
	}

	// Check for raw JSON mode: --json '{"action":...}'
	var command map[string]interface{}
	var err error

	// Detect raw JSON: if first remaining arg starts with {
	if len(remaining) > 0 && len(remaining[0]) > 0 && remaining[0][0] == '{' {
		command, err = ParseRawJSON(remaining[0])
		if err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
	} else {
		command, err = ParseArgs(remaining)
		if err != nil {
			return err
		}
	}

	// Handle special non-daemon commands
	if action, ok := command["action"].(string); ok && action == "session_list" {
		return handleSessionList(flags)
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

	if !resp.Success {
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
	fmt.Println("Installing cloak-agent daemon dependencies...")
	fmt.Println("Run: cd daemon && npm install && npm run build")
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

func printUsage() {
	fmt.Println(`cloak-agent - stealth browser automation CLI for AI agents

Usage:
  cloak-agent <command> [args...]
  cloak-agent --json '{"action":"navigate","url":"..."}'

Navigation:
  open <url>                     Navigate to URL
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

Stealth (cloak-agent exclusive):
  stealth status                 Run bot detection tests
  fingerprint rotate [--seed N]  New browser fingerprint
  profile create <name>          Create persistent profile
  profile list                   List profiles

Tabs:
  tab                            List tabs
  tab new [url]                  New tab
  tab <n>                        Switch to tab
  tab close                      Close tab

Settings:
  set viewport <w> <h>           Set viewport size
  set device <name>              Emulate device
  set geo <lat> <lon>            Set geolocation
  set media dark|light           Color scheme

Schema (for AI agents):
  schema                         List all available commands
  schema <command>               Show command parameters

Updates:
  upgrade                        Upgrade to the latest version
  version                        Print version

Global Flags:
  --session <name>               Use named session (default: "default")
  --json                         JSON output mode
  --json '{"action":...}'        Raw JSON payload mode
  --timeout <ms>                 Command timeout
  --headed                       Show browser window
  --dry-run                      Validate without executing
  --fields <list>                Limit response fields

Made by Ashraf (https://ashrafali.net)`)
}
