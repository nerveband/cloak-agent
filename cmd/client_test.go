package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetAppDir(t *testing.T) {
	dir := GetAppDir()
	if !strings.Contains(dir, ".cloak-agent") {
		t.Errorf("expected .cloak-agent in path, got %s", dir)
	}
}

func TestGetSocketPath(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	path := GetSocketPath("default")
	if runtime.GOOS == "windows" {
		expected := "127.0.0.1:63400"
		if path != expected {
			t.Fatalf("expected %s, got %s", expected, path)
		}
		return
	}
	if !strings.HasSuffix(path, "default.sock") {
		t.Errorf("expected default.sock suffix, got %s", path)
	}
	if !strings.Contains(path, ".cloak-agent") {
		t.Errorf("expected .cloak-agent in path, got %s", path)
	}
}

func TestGetSocketPathCustomSession(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	path := GetSocketPath("test-session")
	if runtime.GOOS == "windows" {
		expected := "127.0.0.1:51523"
		if path != expected {
			t.Fatalf("expected %s, got %s", expected, path)
		}
		return
	}
	if !strings.HasSuffix(path, "test-session.sock") {
		t.Errorf("expected test-session.sock suffix, got %s", path)
	}
}

func TestGetPidFile(t *testing.T) {
	path := GetPidFile("default")
	if !strings.HasSuffix(path, "default.pid") {
		t.Errorf("expected default.pid suffix, got %s", path)
	}
}

func TestIsDaemonRunningFalse(t *testing.T) {
	// With no PID file, should return false
	if IsDaemonRunning("nonexistent-session-xyz") {
		t.Error("expected daemon not running for nonexistent session")
	}
}

func TestGetAppDirEnv(t *testing.T) {
	dir := GetSocketDir()
	if dir == "" {
		t.Error("expected non-empty socket dir")
	}
}

func TestGetSocketDirCustom(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", tmp)
	dir := GetSocketDir()
	if dir != tmp {
		t.Errorf("expected %s, got %s", tmp, dir)
	}
}

func TestGetSocketDirUsesXDGRuntimeDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG runtime directories are a Unix convention")
	}
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", "")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1234")
	expected := filepath.Join("/run/user/1234", "cloak-agent")
	if got := GetSocketDir(); got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestGetPortForSessionSharedVectors(t *testing.T) {
	vectors := map[string]int{
		"default":      63400,
		"test-session": 51523,
		"alpha":        52947,
		"":             58288,
	}
	for session, expected := range vectors {
		if got := GetPortForSession(session); got != expected {
			t.Errorf("GetPortForSession(%q) = %d, want %d", session, got, expected)
		}
	}
}

func TestGetSocketPathIncludesDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses TCP instead of filesystem sockets")
	}
	tmp := t.TempDir()
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", tmp)
	path := GetSocketPath("mysession")
	expected := filepath.Join(tmp, "mysession.sock")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestGetPidFileIncludesDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", tmp)
	path := GetPidFile("mysession")
	expected := filepath.Join(tmp, "mysession.pid")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestIsDaemonRunningWithBadPid(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLOAK_AGENT_SOCKET_DIR", tmp)
	// Write a PID file with an invalid PID
	pidFile := filepath.Join(tmp, "bad-pid.pid")
	os.WriteFile(pidFile, []byte("not-a-number"), 0o644)
	if IsDaemonRunning("bad-pid") {
		t.Error("expected daemon not running for invalid PID")
	}
}

func TestFindInstalledDaemonDirFromAppDir(t *testing.T) {
	home := t.TempDir()
	daemonDir := filepath.Join(home, ".cloak-agent", "daemon")
	t.Setenv("CLOAK_AGENT_INSTALL_DIR", filepath.Join(home, ".cloak-agent"))
	if err := os.MkdirAll(daemonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(daemonDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := findInstalledDaemonDir()
	if got != daemonDir {
		t.Fatalf("expected installed daemon dir %s, got %s", daemonDir, got)
	}
}

func TestFindSourceProjectDirFromPackageDir(t *testing.T) {
	got := findSourceProjectDir()
	if got == "" {
		t.Fatal("expected source project dir to be found")
	}
	if !strings.HasSuffix(got, "cloak-agent") {
		t.Fatalf("expected source project dir to end with cloak-agent, got %s", got)
	}
	if !fileExists(filepath.Join(got, "go.mod")) {
		t.Fatalf("expected go.mod at %s", got)
	}
	if !fileExists(filepath.Join(got, "daemon", "package.json")) {
		t.Fatalf("expected daemon/package.json at %s", got)
	}
}
