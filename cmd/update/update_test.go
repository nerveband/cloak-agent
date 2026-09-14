package update

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestCheckDevVersion(t *testing.T) {
	hasUpdate, _, err := Check("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasUpdate {
		t.Error("dev builds should never report updates")
	}
}

func TestCheckAsyncDevVersion(t *testing.T) {
	ch := CheckAsync("dev")
	result := <-ch
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.HasUpdate {
		t.Error("dev builds should never report updates")
	}
}

func TestFormatNoticeNoUpdate(t *testing.T) {
	result := CheckResult{HasUpdate: false, LatestVersion: "1.0.0"}
	notice := FormatNotice(result)
	if notice != "" {
		t.Errorf("expected empty notice, got %q", notice)
	}
}

func TestFormatNoticeWithError(t *testing.T) {
	result := CheckResult{HasUpdate: true, LatestVersion: "1.0.0", Err: os.ErrNotExist}
	notice := FormatNotice(result)
	if notice != "" {
		t.Errorf("expected empty notice on error, got %q", notice)
	}
}

func TestFormatNoticeWithUpdate(t *testing.T) {
	result := CheckResult{HasUpdate: true, LatestVersion: "2.0.0"}
	notice := FormatNotice(result)
	if notice == "" {
		t.Fatal("expected non-empty notice")
	}
	if expected := "2.0.0"; !contains(notice, expected) {
		t.Errorf("expected notice to contain %q, got %q", expected, notice)
	}
	if expected := "cloak-agent upgrade"; !contains(notice, expected) {
		t.Errorf("expected notice to contain %q, got %q", expected, notice)
	}
}

func TestCacheIsScopedToCurrentVersion(t *testing.T) {
	now := time.Now()
	cache := &Cache{
		LastCheck:      now.Add(-time.Hour),
		CurrentVersion: "0.3.1",
		LatestVersion:  "0.4.2",
		UpdateRequired: true,
	}
	if cacheIsFreshForVersion(cache, "0.4.2", now) {
		t.Fatal("cache from an older installed version must not be reused")
	}
	if !cacheIsFreshForVersion(cache, "0.3.1", now) {
		t.Fatal("cache should remain valid for the version that produced it")
	}
}

func TestCacheFilePermissions(t *testing.T) {
	t.Setenv("CLOAK_AGENT_INSTALL_DIR", t.TempDir())
	if runtime.GOOS == "windows" {
		t.Skip("Windows file permissions are controlled by ACLs, not Unix mode bits")
	}

	// Save a cache and check that the file is 0600
	cache := Cache{
		LastCheck:      time.Now(),
		LatestVersion:  "1.0.0",
		UpdateRequired: false,
	}
	err := saveCache(cache)
	if err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	path, err := cachePath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected cache file permissions 0600, got %04o", perm)
	}
}

func TestShouldCheckUpdates(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"empty", []string{}, false},
		{"version", []string{"version"}, false},
		{"upgrade", []string{"upgrade"}, false},
		{"help flag", []string{"--help"}, false},
		{"h flag", []string{"-h"}, false},
		{"help cmd", []string{"help"}, false},
		{"install", []string{"install"}, false},
		{"version flag", []string{"--version"}, false},
		{"v flag", []string{"-v"}, false},
		{"json flag", []string{"open", "https://example.com", "--json"}, false},
		{"normal command", []string{"open", "https://example.com"}, true},
		{"snapshot", []string{"snapshot", "-i"}, true},
		{"click", []string{"click", "@e1"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldCheckUpdates(tt.args)
			if got != tt.want {
				t.Errorf("shouldCheckUpdates(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
