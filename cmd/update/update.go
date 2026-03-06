package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/creativeprojects/go-selfupdate"
)

const (
	repoOwner = "nerveband"
	repoName  = "cloak-agent"
	cacheFile = "update_cache.json"
)

// Cache stores the last version check result.
type Cache struct {
	LastCheck      time.Time `json:"last_check"`
	LatestVersion  string    `json:"latest_version"`
	UpdateRequired bool      `json:"update_required"`
}

// CheckResult holds the result of an update check.
type CheckResult struct {
	HasUpdate     bool
	LatestVersion string
	Err           error
}

// CheckAsync runs an update check in the background.
// Returns a channel that will receive the result.
func CheckAsync(currentVersion string) <-chan CheckResult {
	ch := make(chan CheckResult, 1)
	go func() {
		hasUpdate, latestVersion, err := Check(currentVersion)
		ch <- CheckResult{
			HasUpdate:     hasUpdate,
			LatestVersion: latestVersion,
			Err:           err,
		}
	}()
	return ch
}

// Check checks if a new version is available (with 24h cache).
func Check(currentVersion string) (hasUpdate bool, latestVersion string, err error) {
	if currentVersion == "dev" {
		return false, "", nil
	}

	cached, err := loadCache()
	if err == nil && time.Since(cached.LastCheck) < 24*time.Hour {
		return cached.UpdateRequired, cached.LatestVersion, nil
	}

	updater, err := newUpdater()
	if err != nil {
		return false, "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	latest, found, err := updater.DetectLatest(ctx, selfupdate.NewRepositorySlug(repoOwner, repoName))
	if err != nil || !found {
		return false, "", err
	}

	hasUpdate = latest.GreaterThan(currentVersion)
	latestVer := latest.Version()

	saveCache(Cache{
		LastCheck:      time.Now(),
		LatestVersion:  latestVer,
		UpdateRequired: hasUpdate,
	})

	return hasUpdate, latestVer, nil
}

// Upgrade downloads and installs the latest version.
// Returns true if an upgrade was performed.
func Upgrade(currentVersion string) (bool, error) {
	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Println("Checking for updates...")

	if currentVersion == "dev" {
		fmt.Println("Running dev build — use 'make build' or 'make install' to update.")
		return false, nil
	}

	updater, err := newUpdater()
	if err != nil {
		return false, fmt.Errorf("failed to create updater: %w", err)
	}

	latest, found, err := updater.DetectLatest(context.Background(), selfupdate.NewRepositorySlug(repoOwner, repoName))
	if err != nil {
		return false, fmt.Errorf("failed to check for updates: %w", err)
	}

	if !found {
		fmt.Println("No releases found")
		return false, nil
	}

	if latest.LessOrEqual(currentVersion) {
		fmt.Printf("Already up to date (latest: %s)\n", latest.Version())
		return false, nil
	}

	fmt.Printf("New version available: %s\n", latest.Version())
	fmt.Printf("Downloading for %s/%s...\n", runtime.GOOS, runtime.GOARCH)

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return false, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Check write permission before downloading
	if err := checkWritePermission(exe); err != nil {
		return false, err
	}

	if err := updater.UpdateTo(context.Background(), latest, exe); err != nil {
		return false, fmt.Errorf("failed to update: %w", err)
	}

	// macOS: re-sign the binary after replacement
	if runtime.GOOS == "darwin" {
		if signErr := exec.Command("codesign", "-s", "-", "-f", exe).Run(); signErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: codesign failed (non-fatal): %v\n", signErr)
		}
	}

	fmt.Printf("Successfully upgraded to %s\n", latest.Version())

	// Clear cache so next check sees new version
	saveCache(Cache{
		LastCheck:      time.Now(),
		LatestVersion:  latest.Version(),
		UpdateRequired: false,
	})

	return true, nil
}

// FormatNotice returns a formatted update notification string, or empty if no update.
func FormatNotice(result CheckResult) string {
	if result.Err != nil || !result.HasUpdate {
		return ""
	}
	return fmt.Sprintf("\nUpdate available: %s\nRun 'cloak-agent upgrade' to update\n\n", result.LatestVersion)
}

// ShouldCheckUpdates returns false for commands that shouldn't trigger update checks.
func ShouldCheckUpdates(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "version", "upgrade", "--version", "-v", "--help", "-h", "help", "install":
		return false
	}
	for _, arg := range args {
		if arg == "--json" {
			return false
		}
	}
	return true
}

// checkWritePermission verifies the binary can be replaced without sudo.
func checkWritePermission(exePath string) error {
	dir := filepath.Dir(exePath)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", dir, err)
	}
	// Check if the directory is writable by opening a temp file
	tmpFile := filepath.Join(dir, ".cloak-agent-update-test")
	f, err := os.Create(tmpFile)
	if err != nil {
		_ = info // used above
		return fmt.Errorf("no write permission to %s\nThe binary is at: %s\nEither move it to a user-owned directory or run: sudo cloak-agent upgrade", dir, exePath)
	}
	f.Close()
	os.Remove(tmpFile)
	return nil
}

func newUpdater() (*selfupdate.Updater, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, err
	}
	return selfupdate.NewUpdater(selfupdate.Config{
		Source:    source,
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
}

func appDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".cloak-agent"), nil
}

func cachePath() (string, error) {
	dir, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheFile), nil
}

func loadCache() (*Cache, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func saveCache(cache Cache) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
