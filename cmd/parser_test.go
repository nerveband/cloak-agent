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
