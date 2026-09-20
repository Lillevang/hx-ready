package installer

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Lillevang/hx-ready/internal/recipes"
)

// Platform is one supported operating system family: how its packages are
// installed and which recipe block applies. Adding a platform means adding
// an entry here, a field on recipes.Recipe, and blocks in the recipes.
type Platform struct {
	// Name is the recipe block name and what messages call the platform.
	Name string
	// Display is the human name, e.g. "Fedora" or "Ubuntu/Debian".
	Display string
	// PackageArgs is the argv prefix for installing packages; sudo is added
	// by the executor. Non-interactive (D-011).
	PackageArgs []string
	// PackageSearch renders a "look for it yourself" hint for a binary.
	PackageSearch func(bin string) string
	// Block returns the recipe's block for this platform, or nil.
	Block func(r *recipes.Recipe) *recipes.Platform
}

// Fedora is the first supported platform (D-008).
var Fedora = Platform{
	Name:          "fedora",
	Display:       "Fedora",
	PackageArgs:   []string{"dnf", "install", "-y"},
	PackageSearch: func(bin string) string { return "dnf search " + bin },
	Block:         func(r *recipes.Recipe) *recipes.Platform { return r.Fedora },
}

// Debian covers Debian and Ubuntu (D-019).
var Debian = Platform{
	Name:          "debian",
	Display:       "Ubuntu/Debian",
	PackageArgs:   []string{"apt-get", "install", "-y"},
	PackageSearch: func(bin string) string { return "apt-cache search " + bin },
	Block:         func(r *recipes.Recipe) *recipes.Platform { return r.Debian },
}

// Platforms lists every supported platform, in detection order.
var Platforms = []Platform{Fedora, Debian}

// OSReleasePath is where the distribution identifies itself. A variable so
// tests can point it at a fixture.
var OSReleasePath = "/etc/os-release"

// ErrUnsupportedPlatform is returned by Detect when /etc/os-release names
// no supported platform. The error text says what was found.
var ErrUnsupportedPlatform = errors.New("unsupported platform")

// Detect reads /etc/os-release and returns the matching platform (D-008,
// D-019): ID=fedora; ID=debian or ID=ubuntu or ID_LIKE containing debian.
func Detect() (Platform, error) {
	f, err := os.Open(OSReleasePath)
	if err != nil {
		return Platform{}, fmt.Errorf("%w: could not read %s: %v", ErrUnsupportedPlatform, OSReleasePath, err)
	}
	defer f.Close()
	return detect(f)
}

func detect(r io.Reader) (Platform, error) {
	fields := osRelease(r)
	id := fields["ID"]
	like := strings.Fields(fields["ID_LIKE"])
	switch {
	case id == "fedora":
		return Fedora, nil
	case id == "debian" || id == "ubuntu" || contains(like, "debian"):
		return Debian, nil
	}
	found := "ID=" + id
	if len(like) > 0 {
		found += " ID_LIKE=" + strings.Join(like, " ")
	}
	return Platform{}, fmt.Errorf("%w: %s reports %s", ErrUnsupportedPlatform, OSReleasePath, found)
}

// osRelease parses KEY=VALUE lines, unquoting values.
func osRelease(r io.Reader) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		out[k] = strings.Trim(v, `"'`)
	}
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
