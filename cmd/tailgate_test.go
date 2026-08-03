package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeTailgateFixture(t *testing.T, mode os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	identity := filepath.Join(dir, "id")
	knownHosts := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(identity, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(knownHosts, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := tailgateConfig{Routes: map[string]tailgateRoute{"default": {
		SSHHost: "private-alias", IdentityFile: identity, KnownHostsFile: knownHosts,
	}}}
	raw, _ := json.Marshal(config)
	path := filepath.Join(dir, "tailgate.json")
	if err := os.WriteFile(path, raw, mode); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLOAK_AGENT_TAILGATE_CONFIG", path)
	return path
}

func TestTailgateConfigRequiresPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission semantics")
	}
	path := writeTailgateFixture(t, 0o644)
	_, err := loadTailgateRoute("default")
	if err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("expected permission error for %s, got %v", path, err)
	}
}

func TestTailgateSSHArgsEnforceKeyOnlyStrictContract(t *testing.T) {
	route := tailgateRoute{SSHHost: "alias", IdentityFile: "/keys/id", KnownHostsFile: "/keys/known"}
	joined := strings.Join(sshArgs(route, "/run/user/1/control"), " ")
	for _, required := range []string{"IdentitiesOnly=yes", "BatchMode=yes", "PasswordAuthentication=no", "KbdInteractiveAuthentication=no", "StrictHostKeyChecking=yes", "ExitOnForwardFailure=yes", "UserKnownHostsFile=/keys/known"} {
		if !strings.Contains(joined, required) {
			t.Errorf("missing %s", required)
		}
	}
}

func TestTailgateProfilesAreIsolatedAndStable(t *testing.T) {
	a := isolatedTailgateProfile("route-a", "shopping")
	b := isolatedTailgateProfile("route-b", "shopping")
	if a == "shopping" || a == b || a != isolatedTailgateProfile("route-a", "shopping") {
		t.Fatalf("profiles are not isolated and stable: %q %q", a, b)
	}
}

func TestTailgateRejectsProfileTraversal(t *testing.T) {
	for _, profile := range []string{"../shared", `nested\\shared`, ".."} {
		if err := validateTailgateProfile(profile); err == nil {
			t.Fatalf("expected profile %q to be rejected", profile)
		}
	}
}

func TestSelectedTailgateFlagAndEnvironment(t *testing.T) {
	if name, ok := selectedTailgate(GlobalFlags{Tailgate: true}); !ok || name != "default" {
		t.Fatalf("flag selection failed: %q %v", name, ok)
	}
	t.Setenv("CLOAK_AGENT_TAILGATE", "egress-a")
	if name, ok := selectedTailgate(GlobalFlags{}); !ok || name != "egress-a" {
		t.Fatalf("env selection failed: %q %v", name, ok)
	}
}

func TestTailgateStatePathsDoNotContainSessionOrRoute(t *testing.T) {
	path := tailgateControlPath("private-session", "private-route")
	if strings.Contains(path, "private-session") || strings.Contains(path, "private-route") {
		t.Fatalf("private names leaked into endpoint path: %s", path)
	}
}

func TestDirectLaunchClearsStaleTailgateBinding(t *testing.T) {
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", t.TempDir())
	if err := os.MkdirAll(tailgateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	path := tailgateBindingPath("same-session")
	if err := os.WriteFile(path, []byte("default"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareTailgate(map[string]interface{}{"action": "launch"}, GlobalFlags{Session: "same-session"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stale binding still exists: %v", err)
	}
}

func TestDryRunDoesNotCreateTunnelState(t *testing.T) {
	writeTailgateFixture(t, 0o600)
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", t.TempDir())
	err := prepareTailgate(map[string]interface{}{"action": "launch"}, GlobalFlags{Session: "dry", Tailgate: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tailgateStatePath("dry")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created state: %v", err)
	}
}
