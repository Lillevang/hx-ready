package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Lillevang/hx-ready/internal/installer"
)

// TestMain pins the detected platform to Fedora so tests do not depend on
// the machine they run on. Individual tests override with usePlatform.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "hx-ready-test")
	if err != nil {
		panic(err)
	}
	installer.OSReleasePath = writeOSRelease(dir, "fedora", "")
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func writeOSRelease(dir, id, like string) string {
	path := filepath.Join(dir, "os-release-"+id)
	content := "ID=" + id + "\n"
	if like != "" {
		content += "ID_LIKE=" + like + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		panic(err)
	}
	return path
}

// usePlatform makes Detect see the given os-release ID for one test.
func usePlatform(t *testing.T, id, like string) {
	t.Helper()
	old := installer.OSReleasePath
	installer.OSReleasePath = writeOSRelease(t.TempDir(), id, like)
	t.Cleanup(func() { installer.OSReleasePath = old })
}
