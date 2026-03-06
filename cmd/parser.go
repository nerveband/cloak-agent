package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GlobalFlags holds CLI-wide flags extracted before the subcommand.
type GlobalFlags struct {
	Session    string // --session <name>, default "default"
	JSONOutput bool   // --json (no payload = JSON output mode)
	Timeout    int    // --timeout <ms>
	Headed     bool   // --headed
	DryRun     bool   // --dry-run
	Fields     string // --fields <comma-separated list>
}

// ParseGlobalFlags extracts global flags from args and returns the remaining
// positional arguments.
func ParseGlobalFlags(args []string) (GlobalFlags, []string) {
	gf := GlobalFlags{Session: "default"}
	var rest []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--session":
			if i+1 < len(args) {
				gf.Session = args[i+1]
				i++
			}
		case "--json":
			gf.JSONOutput = true
		case "--timeout":
			if i+1 < len(args) {
				if v, err := strconv.Atoi(args[i+1]); err == nil {
					gf.Timeout = v
				}
				i++
			}
		case "--headed":
			gf.Headed = true
		case "--dry-run":
			gf.DryRun = true
		case "--fields":
			if i+1 < len(args) {
				gf.Fields = args[i+1]
				i++
			}
		default:
			rest = append(rest, args[i])
		}
	}
	return gf, rest
}

// ParseArgs maps human-friendly CLI commands into JSON command objects (as a
// map). Returns an error for unknown or malformed commands.
func ParseArgs(args []string) (map[string]interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no command provided")
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {

	// ── navigation ──────────────────────────────────────────────────────
	case "open":
		if len(rest) < 1 {
			return nil, fmt.Errorf("open requires a URL")
		}
		m := map[string]interface{}{"action": "navigate", "url": rest[0]}
		for i := 1; i < len(rest); i++ {
			if rest[i] == "--wait" && i+1 < len(rest) {
				m["waitUntil"] = rest[i+1]
				i++
			}
		}
		return m, nil

	case "back":
		return map[string]interface{}{"action": "back"}, nil
	case "forward":
		return map[string]interface{}{"action": "forward"}, nil
	case "reload":
		return map[string]interface{}{"action": "reload"}, nil
	case "close":
		return map[string]interface{}{"action": "close"}, nil

	// ── snapshot ────────────────────────────────────────────────────────
	case "snapshot":
		m := map[string]interface{}{"action": "snapshot"}
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case "-i":
				m["interactive"] = true
			case "-c":
				m["compact"] = true
			case "-d":
				if i+1 < len(rest) {
					if n, err := strconv.Atoi(rest[i+1]); err == nil {
						m["maxDepth"] = n
					}
					i++
				}
			case "-s":
				if i+1 < len(rest) {
					m["selector"] = rest[i+1]
					i++
				}
			}
		}
		return m, nil

	// ── interaction ─────────────────────────────────────────────────────
	case "click":
		if len(rest) < 1 {
			return nil, fmt.Errorf("click requires a selector")
		}
		return map[string]interface{}{"action": "click", "selector": rest[0]}, nil

	case "dblclick":
		if len(rest) < 1 {
			return nil, fmt.Errorf("dblclick requires a selector")
		}
		return map[string]interface{}{"action": "dblclick", "selector": rest[0]}, nil

	case "fill":
		if len(rest) < 2 {
			return nil, fmt.Errorf("fill requires a selector and a value")
		}
		return map[string]interface{}{"action": "fill", "selector": rest[0], "value": rest[1]}, nil

	case "type":
		if len(rest) < 2 {
			return nil, fmt.Errorf("type requires a selector and text")
		}
		return map[string]interface{}{"action": "type", "selector": rest[0], "text": rest[1]}, nil

	case "press":
		if len(rest) < 1 {
			return nil, fmt.Errorf("press requires a key")
		}
		return map[string]interface{}{"action": "press", "key": rest[0]}, nil

	case "hover":
		if len(rest) < 1 {
			return nil, fmt.Errorf("hover requires a selector")
		}
		return map[string]interface{}{"action": "hover", "selector": rest[0]}, nil

	case "focus":
		if len(rest) < 1 {
			return nil, fmt.Errorf("focus requires a selector")
		}
		return map[string]interface{}{"action": "focus", "selector": rest[0]}, nil

	case "check":
		if len(rest) < 1 {
			return nil, fmt.Errorf("check requires a selector")
		}
		return map[string]interface{}{"action": "check", "selector": rest[0]}, nil

	case "uncheck":
		if len(rest) < 1 {
			return nil, fmt.Errorf("uncheck requires a selector")
		}
		return map[string]interface{}{"action": "uncheck", "selector": rest[0]}, nil

	case "select":
		if len(rest) < 2 {
			return nil, fmt.Errorf("select requires a selector and a value")
		}
		return map[string]interface{}{"action": "select", "selector": rest[0], "values": rest[1]}, nil

	case "upload":
		if len(rest) < 2 {
			return nil, fmt.Errorf("upload requires a selector and a file")
		}
		return map[string]interface{}{"action": "upload", "selector": rest[0], "files": rest[1]}, nil

	case "drag":
		if len(rest) < 2 {
			return nil, fmt.Errorf("drag requires a source and target")
		}
		return map[string]interface{}{"action": "drag", "source": rest[0], "target": rest[1]}, nil

	case "highlight":
		if len(rest) < 1 {
			return nil, fmt.Errorf("highlight requires a selector")
		}
		return map[string]interface{}{"action": "highlight", "selector": rest[0]}, nil

	// ── scroll ──────────────────────────────────────────────────────────
	case "scroll":
		if len(rest) < 2 {
			return nil, fmt.Errorf("scroll requires a direction and amount")
		}
		dir := rest[0]
		amount, err := strconv.Atoi(rest[1])
		if err != nil {
			return nil, fmt.Errorf("scroll amount must be numeric: %s", rest[1])
		}
		return map[string]interface{}{"action": "scroll", "direction": dir, "amount": amount}, nil

	case "scrollintoview":
		if len(rest) < 1 {
			return nil, fmt.Errorf("scrollintoview requires a selector")
		}
		return map[string]interface{}{"action": "scrollintoview", "selector": rest[0]}, nil

	// ── getters ─────────────────────────────────────────────────────────
	case "get":
		if len(rest) < 1 {
			return nil, fmt.Errorf("get requires a subcommand")
		}
		switch rest[0] {
		case "url":
			return map[string]interface{}{"action": "url"}, nil
		case "title":
			return map[string]interface{}{"action": "title"}, nil
		case "text":
			if len(rest) < 2 {
				return nil, fmt.Errorf("get text requires a selector")
			}
			return map[string]interface{}{"action": "gettext", "selector": rest[1]}, nil
		case "html":
			if len(rest) < 2 {
				return nil, fmt.Errorf("get html requires a selector")
			}
			return map[string]interface{}{"action": "innerhtml", "selector": rest[1]}, nil
		case "value":
			if len(rest) < 2 {
				return nil, fmt.Errorf("get value requires a selector")
			}
			return map[string]interface{}{"action": "inputvalue", "selector": rest[1]}, nil
		case "attr":
			if len(rest) < 3 {
				return nil, fmt.Errorf("get attr requires a selector and attribute name")
			}
			return map[string]interface{}{"action": "getattribute", "selector": rest[1], "attribute": rest[2]}, nil
		case "count":
			if len(rest) < 2 {
				return nil, fmt.Errorf("get count requires a selector")
			}
			return map[string]interface{}{"action": "count", "selector": rest[1]}, nil
		case "box":
			if len(rest) < 2 {
				return nil, fmt.Errorf("get box requires a selector")
			}
			return map[string]interface{}{"action": "boundingbox", "selector": rest[1]}, nil
		default:
			return nil, fmt.Errorf("unknown get subcommand: %s", rest[0])
		}

	// ── is queries ──────────────────────────────────────────────────────
	case "is":
		if len(rest) < 2 {
			return nil, fmt.Errorf("is requires a subcommand and selector")
		}
		switch rest[0] {
		case "visible":
			return map[string]interface{}{"action": "isvisible", "selector": rest[1]}, nil
		case "enabled":
			return map[string]interface{}{"action": "isenabled", "selector": rest[1]}, nil
		case "checked":
			return map[string]interface{}{"action": "ischecked", "selector": rest[1]}, nil
		default:
			return nil, fmt.Errorf("unknown is subcommand: %s", rest[0])
		}

	// ── screenshot / pdf ────────────────────────────────────────────────
	case "screenshot":
		m := map[string]interface{}{"action": "screenshot"}
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case "--full":
				m["fullPage"] = true
			default:
				m["path"] = rest[i]
			}
		}
		return m, nil

	case "pdf":
		if len(rest) < 1 {
			return nil, fmt.Errorf("pdf requires a path")
		}
		return map[string]interface{}{"action": "pdf", "path": rest[0]}, nil

	// ── eval ────────────────────────────────────────────────────────────
	case "eval":
		if len(rest) < 1 {
			return nil, fmt.Errorf("eval requires a script")
		}
		return map[string]interface{}{"action": "evaluate", "script": rest[0]}, nil

	// ── wait ────────────────────────────────────────────────────────────
	case "wait":
		if len(rest) < 1 {
			return nil, fmt.Errorf("wait requires an argument")
		}
		// Check for flag-based waits first.
		switch rest[0] {
		case "--text":
			if len(rest) < 2 {
				return nil, fmt.Errorf("wait --text requires a value")
			}
			return map[string]interface{}{"action": "wait", "selector": "text=" + rest[1]}, nil
		case "--url":
			if len(rest) < 2 {
				return nil, fmt.Errorf("wait --url requires a pattern")
			}
			return map[string]interface{}{"action": "waitforurl", "url": rest[1]}, nil
		case "--load":
			if len(rest) < 2 {
				return nil, fmt.Errorf("wait --load requires a state")
			}
			return map[string]interface{}{"action": "waitforloadstate", "state": rest[1]}, nil
		case "--fn":
			if len(rest) < 2 {
				return nil, fmt.Errorf("wait --fn requires an expression")
			}
			return map[string]interface{}{"action": "waitforfunction", "expression": rest[1]}, nil
		}
		// Numeric = timeout, otherwise selector.
		if ms, err := strconv.Atoi(rest[0]); err == nil {
			return map[string]interface{}{"action": "wait", "timeout": ms}, nil
		}
		return map[string]interface{}{"action": "wait", "selector": rest[0]}, nil

	// ── tabs ────────────────────────────────────────────────────────────
	case "tab":
		if len(rest) == 0 {
			return map[string]interface{}{"action": "tab_list"}, nil
		}
		switch rest[0] {
		case "new":
			m := map[string]interface{}{"action": "tab_new"}
			if len(rest) > 1 {
				m["url"] = rest[1]
			}
			return m, nil
		case "close":
			return map[string]interface{}{"action": "tab_close"}, nil
		default:
			n, err := strconv.Atoi(rest[0])
			if err != nil {
				return nil, fmt.Errorf("tab: expected 'new', 'close', or a numeric index, got %q", rest[0])
			}
			return map[string]interface{}{"action": "tab_switch", "index": n}, nil
		}

	// ── cookies ─────────────────────────────────────────────────────────
	case "cookies":
		if len(rest) == 0 {
			return map[string]interface{}{"action": "cookies_get"}, nil
		}
		switch rest[0] {
		case "set":
			if len(rest) < 3 {
				return nil, fmt.Errorf("cookies set requires a name and value")
			}
			return map[string]interface{}{
				"action":  "cookies_set",
				"cookies": []map[string]string{{"name": rest[1], "value": rest[2]}},
			}, nil
		case "clear":
			return map[string]interface{}{"action": "cookies_clear"}, nil
		default:
			return nil, fmt.Errorf("unknown cookies subcommand: %s", rest[0])
		}

	// ── storage ─────────────────────────────────────────────────────────
	case "storage":
		if len(rest) < 1 {
			return nil, fmt.Errorf("storage requires a type (local)")
		}
		stype := rest[0] // e.g. "local"
		sub := rest[1:]
		if len(sub) == 0 {
			return map[string]interface{}{"action": "storage_get", "type": stype}, nil
		}
		switch sub[0] {
		case "set":
			if len(sub) < 3 {
				return nil, fmt.Errorf("storage set requires a key and value")
			}
			return map[string]interface{}{"action": "storage_set", "type": stype, "key": sub[1], "value": sub[2]}, nil
		case "clear":
			return map[string]interface{}{"action": "storage_clear", "type": stype}, nil
		default:
			// storage local <key>
			return map[string]interface{}{"action": "storage_get", "type": stype, "key": sub[0]}, nil
		}

	// ── state ───────────────────────────────────────────────────────────
	case "state":
		if len(rest) < 2 {
			return nil, fmt.Errorf("state requires a subcommand and path")
		}
		switch rest[0] {
		case "save":
			return map[string]interface{}{"action": "state_save", "path": rest[1]}, nil
		case "load":
			return map[string]interface{}{"action": "state_load", "path": rest[1]}, nil
		default:
			return nil, fmt.Errorf("unknown state subcommand: %s", rest[0])
		}

	// ── set ─────────────────────────────────────────────────────────────
	case "set":
		if len(rest) < 1 {
			return nil, fmt.Errorf("set requires a subcommand")
		}
		switch rest[0] {
		case "viewport":
			if len(rest) < 3 {
				return nil, fmt.Errorf("set viewport requires width and height")
			}
			w, err := strconv.Atoi(rest[1])
			if err != nil {
				return nil, fmt.Errorf("viewport width must be numeric: %s", rest[1])
			}
			h, err := strconv.Atoi(rest[2])
			if err != nil {
				return nil, fmt.Errorf("viewport height must be numeric: %s", rest[2])
			}
			return map[string]interface{}{"action": "viewport", "width": w, "height": h}, nil
		case "device":
			if len(rest) < 2 {
				return nil, fmt.Errorf("set device requires a name")
			}
			return map[string]interface{}{"action": "device", "device": rest[1]}, nil
		case "geo":
			if len(rest) < 3 {
				return nil, fmt.Errorf("set geo requires latitude and longitude")
			}
			lat, err := strconv.ParseFloat(rest[1], 64)
			if err != nil {
				return nil, fmt.Errorf("latitude must be numeric: %s", rest[1])
			}
			lon, err := strconv.ParseFloat(rest[2], 64)
			if err != nil {
				return nil, fmt.Errorf("longitude must be numeric: %s", rest[2])
			}
			return map[string]interface{}{"action": "geolocation", "latitude": lat, "longitude": lon}, nil
		case "offline":
			if len(rest) < 2 {
				return nil, fmt.Errorf("set offline requires on or off")
			}
			return map[string]interface{}{"action": "offline", "offline": rest[1] == "on"}, nil
		case "headers":
			if len(rest) < 2 {
				return nil, fmt.Errorf("set headers requires a JSON string")
			}
			var headers interface{}
			if err := json.Unmarshal([]byte(rest[1]), &headers); err != nil {
				return nil, fmt.Errorf("invalid JSON for headers: %w", err)
			}
			return map[string]interface{}{"action": "headers", "headers": headers}, nil
		case "credentials":
			if len(rest) < 3 {
				return nil, fmt.Errorf("set credentials requires username and password")
			}
			return map[string]interface{}{"action": "credentials", "username": rest[1], "password": rest[2]}, nil
		case "media":
			if len(rest) < 2 {
				return nil, fmt.Errorf("set media requires a color scheme")
			}
			return map[string]interface{}{"action": "emulatemedia", "colorScheme": rest[1]}, nil
		default:
			return nil, fmt.Errorf("unknown set subcommand: %s", rest[0])
		}

	// ── console / errors ────────────────────────────────────────────────
	case "console":
		m := map[string]interface{}{"action": "console"}
		for _, a := range rest {
			if a == "--clear" {
				m["clear"] = true
			}
		}
		return m, nil

	case "errors":
		m := map[string]interface{}{"action": "errors"}
		for _, a := range rest {
			if a == "--clear" {
				m["clear"] = true
			}
		}
		return m, nil

	// ── network ─────────────────────────────────────────────────────────
	case "network":
		if len(rest) < 1 {
			return nil, fmt.Errorf("network requires a subcommand")
		}
		switch rest[0] {
		case "route":
			if len(rest) < 2 {
				return nil, fmt.Errorf("network route requires a URL pattern")
			}
			m := map[string]interface{}{"action": "route", "url": rest[1]}
			for i := 2; i < len(rest); i++ {
				switch rest[i] {
				case "--abort":
					m["abort"] = true
				case "--body":
					if i+1 < len(rest) {
						m["response"] = map[string]interface{}{"body": rest[i+1]}
						i++
					}
				}
			}
			return m, nil
		case "unroute":
			return map[string]interface{}{"action": "unroute"}, nil
		case "requests":
			m := map[string]interface{}{"action": "requests"}
			for i := 1; i < len(rest); i++ {
				if rest[i] == "--filter" && i+1 < len(rest) {
					m["filter"] = rest[i+1]
					i++
				}
			}
			return m, nil
		default:
			return nil, fmt.Errorf("unknown network subcommand: %s", rest[0])
		}

	// ── dialog ──────────────────────────────────────────────────────────
	case "dialog":
		if len(rest) < 1 {
			return nil, fmt.Errorf("dialog requires accept or dismiss")
		}
		return map[string]interface{}{"action": "dialog", "response": rest[0]}, nil

	// ── trace ───────────────────────────────────────────────────────────
	case "trace":
		if len(rest) < 1 {
			return nil, fmt.Errorf("trace requires a subcommand")
		}
		switch rest[0] {
		case "start":
			return map[string]interface{}{"action": "trace_start"}, nil
		case "stop":
			if len(rest) < 2 {
				return nil, fmt.Errorf("trace stop requires a path")
			}
			return map[string]interface{}{"action": "trace_stop", "path": rest[1]}, nil
		default:
			return nil, fmt.Errorf("unknown trace subcommand: %s", rest[0])
		}

	// ── record ──────────────────────────────────────────────────────────
	case "record":
		if len(rest) < 1 {
			return nil, fmt.Errorf("record requires a subcommand")
		}
		switch rest[0] {
		case "start":
			if len(rest) < 2 {
				return nil, fmt.Errorf("record start requires a path")
			}
			return map[string]interface{}{"action": "recording_start", "path": rest[1]}, nil
		case "stop":
			return map[string]interface{}{"action": "recording_stop"}, nil
		default:
			return nil, fmt.Errorf("unknown record subcommand: %s", rest[0])
		}

	// ── find ────────────────────────────────────────────────────────────
	case "find":
		if len(rest) < 1 {
			return nil, fmt.Errorf("find requires a locator type")
		}
		switch rest[0] {
		case "role":
			if len(rest) < 3 {
				return nil, fmt.Errorf("find role requires a role and subaction")
			}
			m := map[string]interface{}{"action": "getbyrole", "role": rest[1], "subaction": rest[2]}
			for i := 3; i < len(rest); i++ {
				if rest[i] == "--name" && i+1 < len(rest) {
					m["name"] = rest[i+1]
					i++
				}
			}
			return m, nil
		case "text":
			if len(rest) < 3 {
				return nil, fmt.Errorf("find text requires text and subaction")
			}
			return map[string]interface{}{"action": "getbytext", "text": rest[1], "subaction": rest[2]}, nil
		case "label":
			if len(rest) < 3 {
				return nil, fmt.Errorf("find label requires label and subaction")
			}
			return map[string]interface{}{"action": "getbylabel", "label": rest[1], "subaction": rest[2]}, nil
		default:
			return nil, fmt.Errorf("unknown find locator: %s", rest[0])
		}

	// ── mouse ───────────────────────────────────────────────────────────
	case "mouse":
		if len(rest) < 1 {
			return nil, fmt.Errorf("mouse requires a subcommand")
		}
		switch rest[0] {
		case "move":
			if len(rest) < 3 {
				return nil, fmt.Errorf("mouse move requires x and y")
			}
			x, err := strconv.Atoi(rest[1])
			if err != nil {
				return nil, fmt.Errorf("mouse move x must be numeric: %s", rest[1])
			}
			y, err := strconv.Atoi(rest[2])
			if err != nil {
				return nil, fmt.Errorf("mouse move y must be numeric: %s", rest[2])
			}
			return map[string]interface{}{"action": "mousemove", "x": x, "y": y}, nil
		case "down":
			if len(rest) < 2 {
				return nil, fmt.Errorf("mouse down requires a button")
			}
			return map[string]interface{}{"action": "mousedown", "button": rest[1]}, nil
		case "up":
			if len(rest) < 2 {
				return nil, fmt.Errorf("mouse up requires a button")
			}
			return map[string]interface{}{"action": "mouseup", "button": rest[1]}, nil
		case "wheel":
			if len(rest) < 2 {
				return nil, fmt.Errorf("mouse wheel requires a delta")
			}
			delta, err := strconv.Atoi(rest[1])
			if err != nil {
				return nil, fmt.Errorf("mouse wheel delta must be numeric: %s", rest[1])
			}
			return map[string]interface{}{"action": "wheel", "deltaY": delta}, nil
		default:
			return nil, fmt.Errorf("unknown mouse subcommand: %s", rest[0])
		}

	// ── schema ──────────────────────────────────────────────────────────
	case "schema":
		if len(rest) == 0 {
			return map[string]interface{}{"action": "schema", "all": true}, nil
		}
		if rest[0] == "--list" {
			return map[string]interface{}{"action": "schema", "all": true}, nil
		}
		return map[string]interface{}{"action": "schema", "command": rest[0]}, nil

	// ── cloak-agent exclusive ───────────────────────────────────────────
	case "stealth":
		if len(rest) < 1 {
			return nil, fmt.Errorf("stealth requires a subcommand")
		}
		switch rest[0] {
		case "status":
			return map[string]interface{}{"action": "stealth_status"}, nil
		default:
			return nil, fmt.Errorf("unknown stealth subcommand: %s", rest[0])
		}

	case "fingerprint":
		if len(rest) < 1 {
			return nil, fmt.Errorf("fingerprint requires a subcommand")
		}
		switch rest[0] {
		case "rotate":
			m := map[string]interface{}{"action": "fingerprint_rotate"}
			for i := 1; i < len(rest); i++ {
				if rest[i] == "--seed" && i+1 < len(rest) {
					if n, err := strconv.Atoi(rest[i+1]); err == nil {
						m["seed"] = n
					}
					i++
				}
			}
			return m, nil
		default:
			return nil, fmt.Errorf("unknown fingerprint subcommand: %s", rest[0])
		}

	case "profile":
		if len(rest) < 1 {
			return nil, fmt.Errorf("profile requires a subcommand")
		}
		switch rest[0] {
		case "create":
			if len(rest) < 2 {
				return nil, fmt.Errorf("profile create requires a name")
			}
			return map[string]interface{}{"action": "profile_create", "name": rest[1]}, nil
		case "list":
			return map[string]interface{}{"action": "profile_list"}, nil
		default:
			return nil, fmt.Errorf("unknown profile subcommand: %s", rest[0])
		}

	case "session":
		if len(rest) < 1 {
			return nil, fmt.Errorf("session requires a subcommand")
		}
		switch rest[0] {
		case "list":
			return map[string]interface{}{"action": "session_list"}, nil
		default:
			return nil, fmt.Errorf("unknown session subcommand: %s", rest[0])
		}

	default:
		return nil, fmt.Errorf("unknown command: %s", strings.Join(args, " "))
	}
}

// ParseRawJSON parses a raw JSON string into a map.
func ParseRawJSON(input string) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(input), &m); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return m, nil
}
