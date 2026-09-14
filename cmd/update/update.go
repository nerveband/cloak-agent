package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
	CurrentVersion string    `json:"current_version"`
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
	if err == nil && cacheIsFreshForVersion(cached, currentVersion, time.Now()) {
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
		CurrentVersion: currentVersion,
		LastCheck:      time.Now(),
		LatestVersion:  latestVer,
		UpdateRequired: hasUpdate,
	})

	return hasUpdate, latestVer, nil
}
func cacheIsFreshForVersion(cache *Cache, currentVersion string, now time.Time) bool {
	return cache.CurrentVersion == currentVersion &&
		now.Sub(cache.LastCheck) >= 0 &&
		now.Sub(cache.LastCheck) < 24*time.Hour
}

// Upgrade installs the full platform archive, including the locked daemon tree.
// repair reinstalls an equal version; an older upstream release is never selected.
func Upgrade(currentVersion string, repair bool) (bool, error) {
	if currentVersion == "dev" {
		return false, fmt.Errorf("development builds must be installed from their source checkout")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	updater, err := newUpdater()
	if err != nil {
		return false, err
	}
	latest, found, err := updater.DetectLatest(ctx, selfupdate.NewRepositorySlug(repoOwner, repoName))
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("no release found")
	}
	if latest.LessThan(currentVersion) || (latest.Equal(currentVersion) && !repair) {
		fmt.Printf("Already up to date (latest: %s)\n", latest.Version())
		return false, nil
	}
	if filepath.Base(latest.AssetName) != latest.AssetName {
		return false, fmt.Errorf("invalid release asset name")
	}
	for _, address := range []string{latest.AssetURL, latest.ValidationAssetURL} {
		parsed, err := url.Parse(address)
		if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" {
			return false, fmt.Errorf("invalid GitHub release asset URL")
		}
	}
	dir, err := os.MkdirTemp("", "cloak-agent-upgrade-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)
	archive := filepath.Join(dir, latest.AssetName)
	manifest := filepath.Join(dir, "checksums.txt")
	if err = download(ctx, latest.ValidationAssetURL, manifest, 1<<20); err != nil {
		return false, err
	}
	if err = download(ctx, latest.AssetURL, archive, maxArchiveBytes); err != nil {
		return false, err
	}
	if err = verifyChecksum(archive, manifest, latest.AssetName); err != nil {
		return false, err
	}
	bundle := filepath.Join(dir, "bundle")
	if err = os.Mkdir(bundle, 0700); err != nil {
		return false, err
	}
	if err = extractBundle(archive, bundle); err != nil {
		return false, err
	}
	root, err := appDir()
	if err != nil {
		return false, err
	}
	if err = InstallBundle(ctx, bundle, root, latest.Version()); err != nil {
		return false, err
	}
	_ = saveCache(Cache{CurrentVersion: latest.Version(), LastCheck: time.Now(), LatestVersion: latest.Version()})
	fmt.Printf("Installed complete release %s. Existing sessions retain their original daemon; start a new session to use this release.\n", latest.Version())
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
	if root := os.Getenv("CLOAK_AGENT_INSTALL_DIR"); root != "" {
		return filepath.Abs(root)
	}
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
