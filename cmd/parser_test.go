package cmd

import (
	"fmt"
	"testing"
)

// assertEq checks that m[key] equals want, using fmt.Sprintf for comparison
// so that int(2) and float64(2) match.
func assertEq(t *testing.T, m map[string]interface{}, key string, want interface{}) {
	t.Helper()
	got, ok := m[key]
	if !ok {
		t.Errorf("key %q not found in result", key)
		return
	}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Errorf("key %q = %v (%T), want %v (%T)", key, got, got, want, want)
	}
}

func assertNoKey(t *testing.T, m map[string]interface{}, key string) {
	t.Helper()
	if _, ok := m[key]; ok {
		t.Errorf("expected key %q to be absent, but found %v", key, m[key])
	}
}

// ─── ParseArgs tests ────────────────────────────────────────────────────────

func TestParseArgs_OpenURL(t *testing.T) {
	m, err := ParseArgs([]string{"open", "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "navigate")
	assertEq(t, m, "url", "https://example.com")
}

func TestParseArgs_OpenURLWithWait(t *testing.T) {
	m, err := ParseArgs([]string{"open", "https://example.com", "--wait", "networkidle"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "navigate")
	assertEq(t, m, "waitUntil", "networkidle")
}

func TestParseArgs_LaunchWithOptions(t *testing.T) {
	m, err := ParseArgs([]string{"launch", "https://example.com", "--profile", "shop", "--proxy", "http://proxy:8080", "--timezone", "America/New_York", "--locale", "en-US", "--viewport", "1440x900", "--geoip", "--fingerprint-seed", "42", "--arg", "--disable-gpu"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "launch")
	assertEq(t, m, "url", "https://example.com")
	assertEq(t, m, "profile", "shop")
	assertEq(t, m, "proxy", "http://proxy:8080")
	assertEq(t, m, "timezone", "America/New_York")
	assertEq(t, m, "locale", "en-US")
	assertEq(t, m, "geoip", true)
	assertEq(t, m, "fingerprintSeed", 42)
	vp, ok := m["viewport"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected viewport map, got %T", m["viewport"])
	}
	if fmt.Sprintf("%v", vp["width"]) != "1440" || fmt.Sprintf("%v", vp["height"]) != "900" {
		t.Fatalf("unexpected viewport: %#v", vp)
	}
	args, ok := m["args"].([]string)
	if !ok || len(args) != 1 || args[0] != "--disable-gpu" {
		t.Fatalf("unexpected args: %#v", m["args"])
	}
}

func TestParseArgs_LaunchWithAdvancedOptions(t *testing.T) {
	m, err := ParseArgs([]string{
		"launch",
		"--user-agent", "CustomAgent/1.0",
		"--executable-path", "/tmp/cloakbrowser",
		"--storage-state", "state.json",
		"--ignore-https-errors",
		"--humanize",
		"--human-preset", "careful",
		"--human-config", `{"clickDelay":120}`,
		"--context-options", `{"permissions":["geolocation"]}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "launch")
	assertEq(t, m, "userAgent", "CustomAgent/1.0")
	assertEq(t, m, "executablePath", "/tmp/cloakbrowser")
	assertEq(t, m, "storageState", "state.json")
	assertEq(t, m, "ignoreHTTPSErrors", true)
	assertEq(t, m, "humanize", true)
	assertEq(t, m, "humanPreset", "careful")
	humanConfig, ok := m["humanConfig"].(map[string]interface{})
	if !ok || fmt.Sprintf("%v", humanConfig["clickDelay"]) != "120" {
		t.Fatalf("unexpected humanConfig: %#v", m["humanConfig"])
	}
	contextOptions, ok := m["contextOptions"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected contextOptions: %#v", m["contextOptions"])
	}
	permissions, ok := contextOptions["permissions"].([]interface{})
	if !ok || len(permissions) != 1 || permissions[0] != "geolocation" {
		t.Fatalf("unexpected permissions: %#v", contextOptions["permissions"])
	}
}

func TestParseArgs_SnapshotInteractive(t *testing.T) {
	m, err := ParseArgs([]string{"snapshot", "-i"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "snapshot")
	assertEq(t, m, "interactive", true)
}

func TestParseArgs_SnapshotCompact(t *testing.T) {
	m, err := ParseArgs([]string{"snapshot", "-c"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "snapshot")
	assertEq(t, m, "compact", true)
}

func TestParseArgs_SnapshotDepth(t *testing.T) {
	m, err := ParseArgs([]string{"snapshot", "-d", "3"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "snapshot")
	assertEq(t, m, "maxDepth", 3)
}

func TestParseArgs_SnapshotSelector(t *testing.T) {
	m, err := ParseArgs([]string{"snapshot", "-s", "#main"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "snapshot")
	assertEq(t, m, "selector", "#main")
}

func TestParseArgs_Click(t *testing.T) {
	m, err := ParseArgs([]string{"click", "@e1"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "click")
	assertEq(t, m, "selector", "@e1")
}

func TestParseArgs_Fill(t *testing.T) {
	m, err := ParseArgs([]string{"fill", "@e2", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "fill")
	assertEq(t, m, "selector", "@e2")
	assertEq(t, m, "value", "hello")
}

func TestParseArgs_StealthStatus(t *testing.T) {
	m, err := ParseArgs([]string{"stealth", "status"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "stealth_status")
}

func TestParseArgs_FingerprintRotate(t *testing.T) {
	m, err := ParseArgs([]string{"fingerprint", "rotate"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "fingerprint_rotate")
	assertNoKey(t, m, "seed")
}

func TestParseArgs_FingerprintRotateWithSeed(t *testing.T) {
	m, err := ParseArgs([]string{"fingerprint", "rotate", "--seed", "42"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "fingerprint_rotate")
	assertEq(t, m, "seed", 42)
}

func TestParseArgs_ProfileCreate(t *testing.T) {
	m, err := ParseArgs([]string{"profile", "create", "myprofile"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "profile_create")
	assertEq(t, m, "name", "myprofile")
}

func TestParseArgs_ProfileList(t *testing.T) {
	m, err := ParseArgs([]string{"profile", "list"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "profile_list")
}

func TestParseArgs_Close(t *testing.T) {
	m, err := ParseArgs([]string{"close"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "close")
}

func TestParseArgs_ScreenshotFull(t *testing.T) {
	m, err := ParseArgs([]string{"screenshot", "--full", "path.png"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "screenshot")
	assertEq(t, m, "fullPage", true)
	assertEq(t, m, "path", "path.png")
}

func TestParseArgs_ScreenshotNoArgs(t *testing.T) {
	m, err := ParseArgs([]string{"screenshot"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "screenshot")
	assertNoKey(t, m, "path")
	assertNoKey(t, m, "fullPage")
}

func TestParseArgs_GetText(t *testing.T) {
	m, err := ParseArgs([]string{"get", "text", "@e1"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "gettext")
	assertEq(t, m, "selector", "@e1")
}

func TestParseArgs_GetAttrUsesNameField(t *testing.T) {
	m, err := ParseArgs([]string{"get", "attr", "@e1", "href"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "getattribute")
	assertEq(t, m, "selector", "@e1")
	assertEq(t, m, "name", "href")
	assertNoKey(t, m, "attribute")
}

func TestParseArgs_GetTitle(t *testing.T) {
	m, err := ParseArgs([]string{"get", "title"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "title")
}

func TestParseArgs_GetURL(t *testing.T) {
	m, err := ParseArgs([]string{"get", "url"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "url")
}

func TestParseArgs_Back(t *testing.T) {
	m, err := ParseArgs([]string{"back"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "back")
}

func TestParseArgs_Forward(t *testing.T) {
	m, err := ParseArgs([]string{"forward"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "forward")
}

func TestParseArgs_Reload(t *testing.T) {
	m, err := ParseArgs([]string{"reload"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "reload")
}

func TestParseArgs_Tab(t *testing.T) {
	m, err := ParseArgs([]string{"tab"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "tab_list")
}

func TestParseArgs_TabNew(t *testing.T) {
	m, err := ParseArgs([]string{"tab", "new"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "tab_new")
	assertNoKey(t, m, "url")
}

func TestParseArgs_TabNewURL(t *testing.T) {
	m, err := ParseArgs([]string{"tab", "new", "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "tab_new")
	assertEq(t, m, "url", "https://example.com")
}

func TestParseArgs_TabSwitch(t *testing.T) {
	m, err := ParseArgs([]string{"tab", "2"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "tab_switch")
	assertEq(t, m, "index", 2)
}

func TestParseArgs_TabClose(t *testing.T) {
	m, err := ParseArgs([]string{"tab", "close"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "tab_close")
}

func TestParseArgs_SelectReturnsArrayValues(t *testing.T) {
	m, err := ParseArgs([]string{"select", "@e1", "blue"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "select")
	values, ok := m["values"].([]string)
	if !ok {
		t.Fatalf("expected []string values, got %T", m["values"])
	}
	if len(values) != 1 || values[0] != "blue" {
		t.Fatalf("unexpected values: %#v", values)
	}
}

func TestParseArgs_UploadReturnsArrayFiles(t *testing.T) {
	m, err := ParseArgs([]string{"upload", "@e1", "file.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "upload")
	files, ok := m["files"].([]string)
	if !ok {
		t.Fatalf("expected []string files, got %T", m["files"])
	}
	if len(files) != 1 || files[0] != "file.pdf" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestParseArgs_ScrollDownMapsToPositiveY(t *testing.T) {
	m, err := ParseArgs([]string{"scroll", "down", "500"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "scroll")
	assertEq(t, m, "x", 0)
	assertEq(t, m, "y", 500)
	assertNoKey(t, m, "direction")
	assertNoKey(t, m, "amount")
}

func TestParseArgs_ScrollUpMapsToNegativeY(t *testing.T) {
	m, err := ParseArgs([]string{"scroll", "up", "300"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "scroll")
	assertEq(t, m, "x", 0)
	assertEq(t, m, "y", -300)
}

func TestParseArgs_EvalUsesExpressionField(t *testing.T) {
	m, err := ParseArgs([]string{"eval", "document.title"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "evaluate")
	assertEq(t, m, "expression", "document.title")
	assertNoKey(t, m, "script")
}

func TestParseArgs_SetDeviceUsesNameField(t *testing.T) {
	m, err := ParseArgs([]string{"set", "device", "iPhone 14"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "device")
	assertEq(t, m, "name", "iPhone 14")
	assertNoKey(t, m, "device")
}

func TestParseArgs_SetOfflineUsesEnabledField(t *testing.T) {
	m, err := ParseArgs([]string{"set", "offline", "on"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "offline")
	assertEq(t, m, "enabled", true)
	assertNoKey(t, m, "offline")
}

func TestParseArgs_DialogAcceptWithPrompt(t *testing.T) {
	m, err := ParseArgs([]string{"dialog", "accept", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "dialog")
	assertEq(t, m, "accept", true)
	assertEq(t, m, "promptText", "hello")
	assertNoKey(t, m, "response")
}

func TestParseArgs_DialogDismiss(t *testing.T) {
	m, err := ParseArgs([]string{"dialog", "dismiss"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "dialog")
	assertEq(t, m, "accept", false)
}

func TestParseArgs_NetworkRouteAbort(t *testing.T) {
	m, err := ParseArgs([]string{"network", "route", "https://example.com", "--abort"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "route")
	assertEq(t, m, "url", "https://example.com")
	assertEq(t, m, "handler", "abort")
	assertNoKey(t, m, "abort")
}

func TestParseArgs_NetworkRouteFulfill(t *testing.T) {
	m, err := ParseArgs([]string{"network", "route", "https://example.com", "--body", "{}"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "route")
	assertEq(t, m, "url", "https://example.com")
	assertEq(t, m, "handler", "fulfill")
	assertEq(t, m, "body", "{}")
	assertNoKey(t, m, "response")
}

func TestParseArgs_NetworkUnrouteOptionalURL(t *testing.T) {
	m, err := ParseArgs([]string{"network", "unroute", "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "unroute")
	assertEq(t, m, "url", "https://example.com")
}

func TestParseArgs_FindLabelFill(t *testing.T) {
	m, err := ParseArgs([]string{"find", "label", "Email", "fill", "user@test.com"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "getbylabel")
	assertEq(t, m, "text", "Email")
	assertEq(t, m, "subaction", "fill")
	assertEq(t, m, "value", "user@test.com")
}

func TestParseArgs_MouseWheelIncludesDeltaX(t *testing.T) {
	m, err := ParseArgs([]string{"mouse", "wheel", "100"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "wheel")
	assertEq(t, m, "deltaX", 0)
	assertEq(t, m, "deltaY", 100)
}

func TestParseArgs_Schema(t *testing.T) {
	m, err := ParseArgs([]string{"schema"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "schema")
	assertEq(t, m, "all", true)
}

func TestParseArgs_SchemaCommand(t *testing.T) {
	m, err := ParseArgs([]string{"schema", "navigate"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "schema")
	assertEq(t, m, "command", "navigate")
}

func TestParseArgs_DaemonStart(t *testing.T) {
	m, err := ParseArgs([]string{"daemon", "start"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "daemon_start")
}

func TestParseArgs_DaemonStatus(t *testing.T) {
	m, err := ParseArgs([]string{"daemon", "status"})
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "daemon_status")
}

func TestParseArgs_UnknownCommand(t *testing.T) {
	_, err := ParseArgs([]string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

// ─── ParseRawJSON tests ────────────────────────────────────────────────────

func TestParseRawJSON_Valid(t *testing.T) {
	m, err := ParseRawJSON(`{"action":"navigate","url":"https://example.com"}`)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, m, "action", "navigate")
	assertEq(t, m, "url", "https://example.com")
}

func TestParseRawJSON_Invalid(t *testing.T) {
	_, err := ParseRawJSON(`{not json}`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ─── ParseGlobalFlags tests ────────────────────────────────────────────────

func TestParseGlobalFlags_Defaults(t *testing.T) {
	gf, rest := ParseGlobalFlags([]string{"open", "https://example.com"})
	if gf.Session != "default" {
		t.Errorf("Session = %q, want %q", gf.Session, "default")
	}
	if gf.JSONOutput {
		t.Error("JSONOutput should be false by default")
	}
	if gf.DryRun {
		t.Error("DryRun should be false by default")
	}
	if len(rest) != 2 || rest[0] != "open" || rest[1] != "https://example.com" {
		t.Errorf("remaining args = %v, want [open https://example.com]", rest)
	}
}

func TestParseGlobalFlags_Session(t *testing.T) {
	gf, rest := ParseGlobalFlags([]string{"--session", "mysess", "snapshot"})
	if gf.Session != "mysess" {
		t.Errorf("Session = %q, want %q", gf.Session, "mysess")
	}
	if len(rest) != 1 || rest[0] != "snapshot" {
		t.Errorf("remaining args = %v, want [snapshot]", rest)
	}
}

func TestParseGlobalFlags_JSON(t *testing.T) {
	gf, rest := ParseGlobalFlags([]string{"--json", "close"})
	if !gf.JSONOutput {
		t.Error("JSONOutput should be true")
	}
	if len(rest) != 1 || rest[0] != "close" {
		t.Errorf("remaining args = %v, want [close]", rest)
	}
}

func TestParseGlobalFlags_OutputAndInputFlags(t *testing.T) {
	gf, rest := ParseGlobalFlags([]string{"--output", "json", "--input", "json", "open", "https://example.com"})
	if !gf.JSONOutput {
		t.Error("JSONOutput should be true when --output json is provided")
	}
	if gf.InputMode != "json" {
		t.Errorf("InputMode = %q, want json", gf.InputMode)
	}
	if len(rest) != 2 || rest[0] != "open" || rest[1] != "https://example.com" {
		t.Errorf("remaining args = %v, want [open https://example.com]", rest)
	}
}

func TestParseGlobalFlags_InputFile(t *testing.T) {
	gf, rest := ParseGlobalFlags([]string{"--input-file", "payload.json"})
	if gf.InputFile != "payload.json" {
		t.Errorf("InputFile = %q, want payload.json", gf.InputFile)
	}
	if len(rest) != 0 {
		t.Errorf("remaining args = %v, want []", rest)
	}
}

func TestParseGlobalFlags_DryRun(t *testing.T) {
	gf, rest := ParseGlobalFlags([]string{"--dry-run", "click", "@e1"})
	if !gf.DryRun {
		t.Error("DryRun should be true")
	}
	if len(rest) != 2 || rest[0] != "click" {
		t.Errorf("remaining args = %v, want [click @e1]", rest)
	}
}

func TestParseGlobalFlags_AllFlags(t *testing.T) {
	gf, rest := ParseGlobalFlags([]string{
		"--session", "s1",
		"--json",
		"--timeout", "5000",
		"--headed",
		"--dry-run",
		"--fields", "action,url",
		"open", "https://example.com",
	})
	if gf.Session != "s1" {
		t.Errorf("Session = %q, want %q", gf.Session, "s1")
	}
	if !gf.JSONOutput {
		t.Error("JSONOutput should be true")
	}
	if gf.Timeout != 5000 {
		t.Errorf("Timeout = %d, want 5000", gf.Timeout)
	}
	if !gf.Headed {
		t.Error("Headed should be true")
	}
	if !gf.DryRun {
		t.Error("DryRun should be true")
	}
	if gf.Fields != "action,url" {
		t.Errorf("Fields = %q, want %q", gf.Fields, "action,url")
	}
	if len(rest) != 2 || rest[0] != "open" {
		t.Errorf("remaining args = %v, want [open https://example.com]", rest)
	}
}
