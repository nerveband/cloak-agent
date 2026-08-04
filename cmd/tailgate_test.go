package cmd

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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
	if err == nil || !strings.Contains(err.Error(), "must not be accessible") {
		t.Fatalf("expected permission error for %s, got %v", path, err)
	}
}

func TestTailgateRejectsSymlinkAndWritableSSHFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink and permission semantics")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "key")
	known := filepath.Join(dir, "known")
	if err := os.WriteFile(key, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(known, []byte("known"), 0o666); err != nil {
		t.Fatal(err)
	}
	// The process umask may remove group/other write bits from the mode passed
	// to WriteFile (notably on macOS).  Set the unsafe mode explicitly so this
	// test exercises the same invariant on every Unix runner.
	if err := os.Chmod(known, 0o666); err != nil {
		t.Fatal(err)
	}
	if err := validateTailgateFile(known, false); err == nil {
		t.Fatal("expected writable known-hosts file to be rejected")
	}
	symlink := filepath.Join(dir, "key-link")
	if err := os.Symlink(key, symlink); err != nil {
		t.Fatal(err)
	}
	if err := validateTailgateFile(symlink, true); err == nil {
		t.Fatal("expected symlinked key to be rejected")
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
	a := isolatedTailgateProfile("session-a", "route-a", "shopping")
	b := isolatedTailgateProfile("session-a", "route-b", "shopping")
	if a == "shopping" || a == b || a != isolatedTailgateProfile("session-a", "route-a", "shopping") {
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
	writeTailgateFixture(t, 0o600)
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", t.TempDir())
	if err := os.MkdirAll(tailgateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	path := tailgateBindingPath("same-session")
	if err := os.WriteFile(path, []byte("default"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, _ := json.Marshal(tailgateState{Route: "default", Port: 43210, Profile: "default"})
	if err := os.WriteFile(tailgateStatePath("same-session"), state, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareTailgate(map[string]interface{}{"action": "launch"}, GlobalFlags{Session: "same-session", Direct: true}); err != nil {
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

func TestTailgateSOCKSReadyRequiresNoAuthHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 3)
		if _, err := conn.Read(buf); err == nil {
			_, _ = conn.Write([]byte{0x05, 0x00})
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	if !tailgateSOCKSReady(port) {
		t.Fatal("expected SOCKS no-auth handshake")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SOCKS fixture did not finish")
	}
}

func TestTailgateDryRunRouteSwitchDoesNotMutate(t *testing.T) {
	writeTailgateFixture(t, 0o600)
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", t.TempDir())
	if err := os.MkdirAll(tailgateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tailgateBindingPath("dry-switch"), []byte("default"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, _ := json.Marshal(tailgateState{Route: "default", Port: 40001, Profile: "default"})
	if err := os.WriteFile(tailgateStatePath("dry-switch"), state, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareTailgate(map[string]interface{}{"action": "launch"}, GlobalFlags{Session: "dry-switch", Tailgate: true, TailgateRoute: "missing", DryRun: true}); err == nil {
		t.Fatal("expected missing route error")
	}
	if _, err := os.Stat(tailgateBindingPath("dry-switch")); err != nil {
		t.Fatalf("dry-run removed binding: %v", err)
	}
	if _, err := os.Stat(tailgateStatePath("dry-switch")); err != nil {
		t.Fatalf("dry-run removed state: %v", err)
	}
}

func TestDirectTransitionClearsStateWithoutBinding(t *testing.T) {
	writeTailgateFixture(t, 0o600)
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", t.TempDir())
	if err := os.MkdirAll(tailgateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	state, _ := json.Marshal(tailgateState{Route: "default", Port: 40002, Profile: "default"})
	if err := os.WriteFile(tailgateStatePath("direct-state"), state, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareTailgate(map[string]interface{}{"action": "launch"}, GlobalFlags{Session: "direct-state", Direct: true}); err != nil {
		t.Fatal(err)
	}
	if fileExists(tailgateStatePath("direct-state")) || fileExists(tailgateBindingPath("direct-state")) {
		t.Fatal("direct transition left route state")
	}
}

func TestTailgateSSHAgentArgs(t *testing.T) {
	route := tailgateRoute{SSHHost: "alias", KnownHostsFile: "/keys/known", UseAgent: true}
	joined := strings.Join(sshArgs(route, "/run/user/1/control"), " ")
	if strings.Contains(joined, "-i ") || !strings.Contains(joined, "IdentitiesOnly=no") || strings.Contains(joined, "IdentityAgent=none") {
		t.Fatalf("unexpected agent SSH args: %s", joined)
	}
}

func TestTailgateBindingStateMismatchFailsClosed(t *testing.T) {
	writeTailgateFixture(t, 0o600)
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", t.TempDir())
	if err := os.MkdirAll(tailgateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tailgateBindingPath("mismatch"), []byte("default"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, _ := json.Marshal(tailgateState{Route: "other", Port: 40003, Profile: "default"})
	if err := os.WriteFile(tailgateStatePath("mismatch"), state, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := releaseTailgateSession("mismatch", "default"); err == nil {
		t.Fatal("expected mismatched markers to fail closed")
	}
	if !fileExists(tailgateBindingPath("mismatch")) || !fileExists(tailgateStatePath("mismatch")) {
		t.Fatal("mismatched markers were removed")
	}
}

func TestTailgateRuntimeDirectoryRejectsPermissiveMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission semantics")
	}
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", t.TempDir())
	if err := os.MkdirAll(tailgateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checkTailgateDir(); err == nil {
		t.Fatal("expected permissive runtime directory to fail closed")
	}
}

func TestDirectLaunchProfileDescriptorIsPrivateAndRecoverable(t *testing.T) {
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", t.TempDir())
	if err := commitDirectLaunchProfile("direct-descriptor", "customer-profile"); err != nil {
		t.Fatal(err)
	}
	profile, err := readDirectLaunchProfile("direct-descriptor")
	if err != nil || profile != "customer-profile" {
		t.Fatalf("profile descriptor did not recover: %q %v", profile, err)
	}
	info, err := os.Stat(directLaunchStatePath("direct-descriptor"))
	if err != nil {
		t.Fatalf("profile descriptor is not recoverable: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("profile descriptor permissions are unsafe: %v", err)
	}
	if err := commitDirectLaunchProfile("direct-descriptor", ""); err != nil {
		t.Fatal(err)
	}
	if fileExists(directLaunchStatePath("direct-descriptor")) {
		t.Fatal("cleared direct profile descriptor remains")
	}
}

func TestTailgatePortReservationIsAtomicAndReleasable(t *testing.T) {
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", t.TempDir())
	if err := ensureTailgateDir(); err != nil {
		t.Fatal(err)
	}
	port, err := reserveTailgatePort()
	if err != nil || port < 1 || port > 65535 {
		t.Fatalf("failed to reserve loopback port: %d %v", port, err)
	}
	reservation := tailgatePortReservationPath(port)
	if !fileExists(reservation) {
		t.Fatal("port reservation marker was not created")
	}
	releaseTailgatePort(port)
	if fileExists(reservation) {
		t.Fatal("port reservation marker was not released")
	}
}
