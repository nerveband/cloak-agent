package cmd

import "testing"

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
func TestApplyGlobalCommandFlagsAddsCACert(t *testing.T) {
	command := map[string]interface{}{"action": "launch"}
	if err := applyGlobalCommandFlags(command, GlobalFlags{CACert: "/tmp/proxy-ca.pem"}); err != nil {
		t.Fatal(err)
	}
	if got := command["caCert"]; got != "/tmp/proxy-ca.pem" {
		t.Fatalf("expected caCert, got %v", got)
	}
}

func TestApplyGlobalCommandFlagsReadsCACertEnvironment(t *testing.T) {
	t.Setenv("CLOAK_AGENT_CA_CERT", "/tmp/env-ca.pem")
	command := map[string]interface{}{"action": "launch"}
	if err := applyGlobalCommandFlags(command, GlobalFlags{}); err != nil {
		t.Fatal(err)
	}
	if got := command["caCert"]; got != "/tmp/env-ca.pem" {
		t.Fatalf("expected environment caCert, got %v", got)
	}
}

func TestApplyGlobalCommandFlagsRejectsUnsafeCACombinations(t *testing.T) {
	tests := []map[string]interface{}{
		{"action": "launch", "profile": "named"},
		{"action": "launch", "ignoreHTTPSErrors": true},
	}
	for _, command := range tests {
		if err := applyGlobalCommandFlags(command, GlobalFlags{CACert: "/tmp/proxy-ca.pem"}); err == nil {
			t.Fatalf("expected CA conflict for %#v", command)
		}
	}
}

func TestApplyGlobalCommandFlagsClearCAWinsOverEnvironment(t *testing.T) {
	t.Setenv("CLOAK_AGENT_CA_CERT", "/tmp/env-ca.pem")
	command := map[string]interface{}{"action": "launch"}
	if err := applyGlobalCommandFlags(command, GlobalFlags{ClearCACert: true}); err != nil {
		t.Fatal(err)
	}
	if got := command["clearCaCert"]; got != true {
		t.Fatalf("expected clearCaCert=true, got %v", got)
	}
	if _, ok := command["caCert"]; ok {
		t.Fatal("clear flag must suppress environment CA")
	}
}
