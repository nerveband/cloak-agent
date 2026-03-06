package update

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestCacheRoundTrip(t *testing.T) {
	// Use a temp dir for cache
	tmpDir := t.TempDir()
	origFunc := cachePath
	_ = origFunc

	// Write cache manually
	cache := Cache{
		LastCheck:      time.Now(),
		LatestVersion:  "1.2.3",
		UpdateRequired: true,
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmpDir, cacheFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Read it back
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var loaded Cache
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}

	if loaded.LatestVersion != "1.2.3" {
		t.Errorf("expected 1.2.3, got %s", loaded.LatestVersion)
	}
	if !loaded.UpdateRequired {
		t.Error("expected UpdateRequired to be true")
	}
}

func TestCheckWritePermissionWritableDir(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "cloak-agent")
	if err := os.WriteFile(tmpFile, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := checkWritePermission(tmpFile); err != nil {
		t.Errorf("expected no error for writable dir, got: %v", err)
	}
}

func TestCheckWritePermissionReadOnlyDir(t *testing.T) {
	tmpDir := t.TempDir()
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(roDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(roDir, 0755) // cleanup

	exePath := filepath.Join(roDir, "cloak-agent")
	err := checkWritePermission(exePath)
	if err == nil {
		t.Error("expected permission error for read-only dir")
	}
	if !contains(err.Error(), "no write permission") {
		t.Errorf("expected 'no write permission' in error, got: %v", err)
	}
}

func TestCacheFilePermissions(t *testing.T) {
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
