package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Lillevang/hx-ready/internal/recipes"
)

func TestRustInitialisesToolchainIndependentlyOfCargo(t *testing.T) {
	r, err := recipes.Load("rust")
	if err != nil {
		t.Fatal(err)
	}
	for _, cargoPresent := range []bool{false, true} {
		have := only("rustup")
		if cargoPresent {
			have = only("rustup", "cargo")
		}
		plan, err := Build(Debian, r, []string{"rust-analyzer"}, have)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Steps) != 1 || plan.Steps[0].Shell == "" {
			t.Fatalf("cargo present=%v: expected one conditional rustup step, got %+v", cargoPresent, plan)
		}
	}
	plan, err := Build(Debian, r, []string{"rust-analyzer"}, nothing)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 2 || plan.Steps[0].String() != "sudo apt-get install -y rustup" {
		t.Fatalf("fresh machine plan = %+v", plan)
	}
}

func TestRustToolchainSetupPreservesActiveToolchain(t *testing.T) {
	r, err := recipes.Load("rust")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Build(Debian, r, []string{"rust-analyzer"}, only("rustup", "cargo"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, active, setup, want string
		fail                      bool
	}{
		{"no active toolchain", "1", "0", "show active-toolchain\ndefault stable\ncomponent add rust-analyzer\n", false},
		{"existing default or override", "0", "0", "show active-toolchain\ncomponent add rust-analyzer\n", false},
		{"setup failure stops installation", "1", "7", "show active-toolchain\ndefault stable\n", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := filepath.Join(t.TempDir(), "calls")
			// Stub only rustup; execute the actual bundled recipe shell.
			stub := `rustup() {
  printf '%s\n' "$*" >> "$CALLS"
  case "$1" in
    show) return "$ACTIVE_STATUS" ;;
    default) return "$SETUP_STATUS" ;;
    component) return 0 ;;
    *) return 99 ;;
  esac
}
`
			cmd := exec.Command("sh", "-c", stub+plan.Steps[0].Shell)
			cmd.Env = append(os.Environ(), "CALLS="+filepath.ToSlash(calls), "ACTIVE_STATUS="+tt.active, "SETUP_STATUS="+tt.setup)
			out, err := cmd.CombinedOutput()
			if (err != nil) != tt.fail {
				t.Fatalf("err=%v output=%s", err, out)
			}
			got, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			if strings.ReplaceAll(string(got), "\r\n", "\n") != tt.want {
				t.Fatalf("calls=%q, want %q", got, tt.want)
			}
		})
	}
}
