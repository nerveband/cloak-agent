package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCommandIDPreservesExistingID(t *testing.T) {
	command := map[string]interface{}{
		"action": "title",
		"id":     "x1",
	}

	ensureCommandID(command)

	if got := command["id"]; got != "x1" {
		t.Fatalf("expected existing id to be preserved, got %v", got)
	}
}

func TestEnsureCommandIDAddsIDWhenMissing(t *testing.T) {
	command := map[string]interface{}{"action": "title"}

	ensureCommandID(command)

	id, ok := command["id"].(string)
	if !ok || id == "" {
		t.Fatalf("expected generated string id, got %#v", command["id"])
	}
}

func TestApplyGlobalCommandFlagsSetsLaunchHeadlessFalse(t *testing.T) {
	command := map[string]interface{}{"action": "launch"}

	applyGlobalCommandFlags(command, GlobalFlags{DryRun: true, Headed: true})

	if got := command["dryRun"]; got != true {
		t.Fatalf("expected dryRun=true, got %v", got)
	}
	if got := command["headless"]; got != false {
		t.Fatalf("expected headless=false, got %v", got)
	}
}

func TestInstallScriptBootstrapsCloakBrowser(t *testing.T) {
	scriptPath := filepath.Join("..", "scripts", "install.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "cloakbrowser install") {
		t.Fatalf("expected %s to run cloakbrowser install", scriptPath)
	}
}
