package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const maxArchiveBytes int64 = 128 << 20
const maxExpandedBytes int64 = 256 << 20

type Installation struct {
	Current  string `json:"current"`
	Previous string `json:"previous,omitempty"`
}

func BinaryName() string {
	if runtime.GOOS == "windows" {
		return "cloak-agent.exe"
	}
	return "cloak-agent"
}

func ReadInstallation(root string) (Installation, error) {
	var state Installation
	raw, err := os.ReadFile(filepath.Join(root, "current.json"))
	if err != nil {
		return state, err
	}
	if err = json.Unmarshal(raw, &state); err != nil {
		return state, err
	}
	for _, entry := range []string{state.Current, state.Previous} {
		if entry != "" && (filepath.Base(entry) != entry || entry == "." || entry == "..") {
			return state, fmt.Errorf("invalid installation pointer")
		}
	}
	if state.Current == "" {
		return state, fmt.Errorf("empty installation pointer")
	}
	return state, nil
}

func CurrentRoot(root string) (string, error) {
	state, err := ReadInstallation(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "releases", state.Current), nil
}

func lockInstallation(root string) (func(), error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	lock := filepath.Join(root, ".install-lock")
	if err := os.Mkdir(lock, 0700); err != nil {
		return nil, fmt.Errorf("installation lock unavailable at %s; another install may be active: %w", lock, err)
	}
	return func() { _ = os.Remove(lock) }, nil
}

func writeInstallation(root string, state Installation) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(root, ".current-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), filepath.Join(root, "current.json"))
}

func copyFile(source, destination string, mode fs.FileMode) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("expected regular file: %s", source)
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func copyBundle(source, destination string) error {
	for _, name := range []string{BinaryName(), "daemon/package.json", "daemon/package-lock.json"} {
		mode := fs.FileMode(0644)
		if name == BinaryName() {
			mode = 0755
		}
		if err := copyFile(filepath.Join(source, name), filepath.Join(destination, name), mode); err != nil {
			return err
		}
	}
	root := filepath.Join(source, "daemon", "dist")
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, "daemon", "dist", relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target, 0644)
	})
}

func ValidateBundle(ctx context.Context, root, expected string) error {
	for _, name := range []string{BinaryName(), "daemon/dist/daemon.js", "daemon/package.json", "daemon/package-lock.json"} {
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("invalid bundle entry: %s", name)
		}
	}
	var pkg struct {
		Version string `json:"version"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "daemon", "package.json"))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &pkg); err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, filepath.Join(root, BinaryName()), "version").Output()
	if err != nil {
		return fmt.Errorf("validate release executable: %w", err)
	}
	binaryVersion := strings.TrimPrefix(strings.TrimSpace(string(output)), "cloak-agent v")
	if binaryVersion != pkg.Version || (expected != "" && binaryVersion != expected) {
		return fmt.Errorf("component version mismatch: binary=%s daemon=%s expected=%s", binaryVersion, pkg.Version, expected)
	}
	return nil
}

func bootstrap(ctx context.Context, root string) error {
	node, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("install Node.js 22+ first: %w", err)
	}
	output, err := exec.CommandContext(ctx, node, "--version").Output()
	if err != nil {
		return err
	}
	var major int
	if _, err = fmt.Sscanf(string(output), "v%d.", &major); err != nil || major < 22 {
		return fmt.Errorf("cloak-agent requires Node.js 22 or newer")
	}
	npmCommand := exec.CommandContext(ctx, "npm", "ci", "--omit=dev", "--no-audit", "--no-fund")
	if runtime.GOOS == "windows" {
		// Windows command scripts require the command processor; this command is constant.
		npmCommand = exec.CommandContext(ctx, "cmd.exe", "/d", "/s", "/c", "npm.cmd ci --omit=dev --no-audit --no-fund")
	}
	commands := []*exec.Cmd{
		npmCommand,
		exec.CommandContext(ctx, node, filepath.Join(root, "daemon", "node_modules", "cloakbrowser", "dist", "cli.js"), "install"),
		exec.CommandContext(ctx, node, "--input-type=module", "-e", "await import('./dist/actions.js'); await import('./dist/protocol.js')"),
	}
	for _, command := range commands {
		command.Dir = filepath.Join(root, "daemon")
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("installation bootstrap failed: %w", err)
		}
	}
	return nil
}

// InstallBundle never mutates a live release directory. One pointer activates both components.
func InstallBundle(ctx context.Context, source, root, expected string) error {
	return installBundle(ctx, source, root, expected, bootstrap)
}

func installBundle(ctx context.Context, source, root, expected string, prepare func(context.Context, string) error) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	unlock, err := lockInstallation(root)
	if err != nil {
		return err
	}
	defer unlock()
	releases := filepath.Join(root, "releases")
	if err = os.MkdirAll(releases, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(releases, "release-")
	if err != nil {
		return err
	}
	activated := false
	defer func() {
		if !activated {
			_ = os.RemoveAll(stage)
		}
	}()
	if err = copyBundle(source, stage); err != nil {
		return err
	}
	if err = ValidateBundle(ctx, stage, expected); err != nil {
		return err
	}
	if err = prepare(ctx, stage); err != nil {
		return err
	}
	previous, err := ReadInstallation(root)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// A stable bootstrap executable dispatches through current.json. It is installed
	// once; subsequent upgrades leave running executables and daemon trees alone.
	binDir := filepath.Join(root, "bin")
	if err = os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	bin := filepath.Join(binDir, BinaryName())
	migrate := previous.Current == ""
	backup := bin + ".pre-managed"
	if migrate {
		temporary := bin + ".new"
		if err = copyFile(filepath.Join(stage, BinaryName()), temporary, 0755); err != nil {
			return err
		}
		defer os.Remove(temporary)
		hadBinary := false
		if _, err = os.Lstat(bin); err == nil {
			if _, err = os.Lstat(backup); !os.IsNotExist(err) {
				return fmt.Errorf("migration backup already exists: %s", backup)
			}
			if err = os.Rename(bin, backup); err != nil {
				return err
			}
			hadBinary = true
		} else if !os.IsNotExist(err) {
			return err
		}
		if err = os.Rename(temporary, bin); err != nil {
			if hadBinary {
				_ = os.Rename(backup, bin)
			}
			return err
		}
		defer func() {
			if !activated {
				_ = os.Remove(bin)
				if hadBinary {
					_ = os.Rename(backup, bin)
				}
			}
		}()
	}
	if err = writeInstallation(root, Installation{Current: filepath.Base(stage), Previous: previous.Current}); err != nil {
		return err
	}
	activated = true
	return nil
}

func Rollback(root string) error {
	unlock, err := lockInstallation(root)
	if err != nil {
		return err
	}
	defer unlock()
	state, err := ReadInstallation(root)
	if err != nil {
		return err
	}
	if state.Previous == "" {
		return fmt.Errorf("no previous managed release is available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = ValidateBundle(ctx, filepath.Join(root, "releases", state.Previous), ""); err != nil {
		return err
	}
	return writeInstallation(root, Installation{Current: state.Previous, Previous: state.Current})
}

func download(ctx context.Context, url, path string, limit int64) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(file, io.LimitReader(response.Body, limit+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > limit {
		return fmt.Errorf("download exceeds size limit")
	}
	return nil
}

func verifyChecksum(archive, manifest, name string) error {
	raw, err := os.ReadFile(manifest)
	if err != nil {
		return err
	}
	expected := ""
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			if expected != "" {
				return fmt.Errorf("duplicate checksum entry")
			}
			expected = fields[0]
		}
	}
	if len(expected) != 64 {
		return fmt.Errorf("missing or malformed checksum for %s", name)
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expected) {
		return fmt.Errorf("release checksum mismatch")
	}
	return nil
}

func extractBundle(archive, destination string) error {
	var total int64
	count := 0
	extract := func(name string, mode fs.FileMode, size int64, reader io.Reader) error {
		count++
		total += size
		if count > 10000 || size < 0 || total > maxExpandedBytes {
			return fmt.Errorf("release archive exceeds extraction limits")
		}
		if strings.Contains(name, "\\") || strings.Contains(name, ":") || strings.HasPrefix(name, "/") {
			return fmt.Errorf("unsafe archive path: %s", name)
		}
		clean := filepath.Clean(filepath.FromSlash(name))
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			return fmt.Errorf("unsafe archive path: %s", name)
		}
		target := filepath.Join(destination, clean)
		if mode.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !mode.IsRegular() {
			return fmt.Errorf("archive links and special files are not allowed: %s", name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm()&0755)
		if err != nil {
			return err
		}
		n, copyErr := io.CopyN(output, reader, size)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if n != size {
			return io.ErrUnexpectedEOF
		}
		return closeErr
	}
	if strings.HasSuffix(archive, ".zip") {
		zipFile, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer zipFile.Close()
		for _, entry := range zipFile.File {
			if entry.UncompressedSize64 > uint64(maxExpandedBytes) {
				return fmt.Errorf("archive entry too large")
			}
			reader, err := entry.Open()
			if err != nil {
				return err
			}
			err = extract(entry.Name, entry.Mode(), int64(entry.UncompressedSize64), reader)
			reader.Close()
			if err != nil {
				return err
			}
		}
	} else {
		file, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer file.Close()
		gz, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gz.Close()
		reader := tar.NewReader(gz)
		for {
			entry, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if err = extract(entry.Name, entry.FileInfo().Mode(), entry.Size, reader); err != nil {
				return err
			}
		}
	}
	return nil
}
