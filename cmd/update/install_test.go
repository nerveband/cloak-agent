package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	if version := os.Getenv("CLOAK_INSTALL_TEST_VERSION"); version != "" {
		fmt.Printf("cloak-agent v%s\n", version)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func bundleFixture(t *testing.T, version string) string {
	t.Helper()
	directory := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = copyFile(executable, filepath.Join(directory, BinaryName()), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(directory, "daemon", "dist"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"daemon/package.json": fmt.Sprintf(`{"version":%q}`, version), "daemon/package-lock.json": "{}", "daemon/dist/daemon.js": "export {};"} {
		if err = os.WriteFile(filepath.Join(directory, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func TestInstallFailurePreservesReleaseAndUserState(t *testing.T) {
	t.Setenv("CLOAK_INSTALL_TEST_VERSION", "0.5.1")
	source := bundleFixture(t, "0.5.1")
	root := t.TempDir()
	prepare := func(context.Context, string) error { return nil }
	if err := installBundle(context.Background(), source, root, "0.5.1", prepare); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "current.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "user-state"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	fail := func(context.Context, string) error { return fmt.Errorf("dependency install failed") }
	if err = installBundle(context.Background(), source, root, "0.5.1", fail); err == nil {
		t.Fatal("expected bootstrap failure")
	}
	after, _ := os.ReadFile(filepath.Join(root, "current.json"))
	if string(after) != string(before) {
		t.Fatal("failed install changed active release")
	}
	data, _ := os.ReadFile(filepath.Join(root, "user-state"))
	if string(data) != "keep" {
		t.Fatal("user state changed")
	}
	entries, _ := os.ReadDir(filepath.Join(root, "releases"))
	if len(entries) != 1 {
		t.Fatal("failed staging directory leaked")
	}
}

func TestInstallVersionMismatchAndRollback(t *testing.T) {
	t.Setenv("CLOAK_INSTALL_TEST_VERSION", "0.5.1")
	root := t.TempDir()
	prepare := func(context.Context, string) error { return nil }
	bad := bundleFixture(t, "0.5.0")
	if err := installBundle(context.Background(), bad, root, "0.5.1", prepare); err == nil {
		t.Fatal("accepted mismatched daemon")
	}
	source := bundleFixture(t, "0.5.1")
	if err := installBundle(context.Background(), source, root, "0.5.1", prepare); err != nil {
		t.Fatal(err)
	}
	first, _ := ReadInstallation(root)
	if err := installBundle(context.Background(), source, root, "0.5.1", prepare); err != nil {
		t.Fatal(err)
	}
	second, _ := ReadInstallation(root)
	if second.Previous != first.Current || second.Current == first.Current {
		t.Fatal("same-version repair did not retain prior release")
	}
	if err := Rollback(root); err != nil {
		t.Fatal(err)
	}
	rolled, _ := ReadInstallation(root)
	if rolled.Current != first.Current {
		t.Fatal("rollback selected wrong release")
	}
}

func TestReleaseChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "release.tar.gz")
	manifest := filepath.Join(root, "checksums.txt")
	data := []byte("verified bytes")
	os.WriteFile(archive, data, 0600)
	os.WriteFile(manifest, []byte(fmt.Sprintf("%x  release.tar.gz\n", sha256.Sum256(data))), 0600)
	if err := verifyChecksum(archive, manifest, "release.tar.gz"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(archive, []byte("tampered bytes"), 0600)
	if err := verifyChecksum(archive, manifest, "release.tar.gz"); err == nil {
		t.Fatal("accepted corrupt archive")
	}
}

func TestArchiveRejectsTraversalAndLinks(t *testing.T) {
	for _, header := range []*tar.Header{
		{Name: "../escape", Mode: 0644, Size: 1},
		{Name: "/absolute", Mode: 0644, Size: 1},
		{Name: "daemon/link", Typeflag: tar.TypeSymlink, Linkname: "../../escape", Mode: 0777},
		{Name: "C:/escape", Mode: 0644, Size: 1},
	} {
		t.Run(header.Name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "test.tar.gz")
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			gz := gzip.NewWriter(file)
			writer := tar.NewWriter(gz)
			if err = writer.WriteHeader(header); err != nil {
				t.Fatal(err)
			}
			if header.Size > 0 {
				writer.Write([]byte("x"))
			}
			writer.Close()
			gz.Close()
			file.Close()
			if err = extractBundle(archive, filepath.Join(root, "out")); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}
