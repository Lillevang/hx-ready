package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := map[string]struct {
		content string
		want    string // platform name, "" for unsupported
	}{
		"fedora":        {"NAME=\"Fedora Linux\"\nVERSION=\"43 (Workstation Edition)\"\nID=fedora\nID_LIKE=\n", "fedora"},
		"fedora quoted": {"ID=\"fedora\"\n", "fedora"},
		"ubuntu":        {"NAME=\"Ubuntu\"\nID=ubuntu\nID_LIKE=debian\n", "debian"},
		"debian":        {"ID=debian\n", "debian"},
		"mint":          {"ID=linuxmint\nID_LIKE=\"ubuntu debian\"\n", "debian"},
		"arch":          {"ID=arch\n", ""},
		"empty":         {"", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "os-release")
			if err := os.WriteFile(path, []byte(c.content), 0o644); err != nil {
				t.Fatal(err)
			}
			old := OSReleasePath
			OSReleasePath = path
			t.Cleanup(func() { OSReleasePath = old })
			p, err := Detect()
			if c.want == "" {
				if !errors.Is(err, ErrUnsupportedPlatform) {
					t.Errorf("err = %v, want ErrUnsupportedPlatform", err)
				}
				if err != nil && !strings.Contains(err.Error(), "ID=") {
					t.Errorf("error should show what was found: %v", err)
				}
				return
			}
			if err != nil || p.Name != c.want {
				t.Errorf("Detect() = %q, %v; want %q", p.Name, err, c.want)
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		old := OSReleasePath
		OSReleasePath = filepath.Join(t.TempDir(), "nope")
		t.Cleanup(func() { OSReleasePath = old })
		if _, err := Detect(); !errors.Is(err, ErrUnsupportedPlatform) {
			t.Errorf("err = %v, want ErrUnsupportedPlatform", err)
		}
	})
}
