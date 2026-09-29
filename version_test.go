package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionStampedAtBuildTime verifies the version reported by the binary
// matches the value passed to the linker, so the release pipeline's stamp is
// the version users see (task 1.2).
func TestVersionStampedAtBuildTime(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the go toolchain to build a binary")
	}
	bin := filepath.Join(t.TempDir(), "astrogui-test")
	const want = "9.9.9-test"

	build := exec.Command("go", "build", "-ldflags", "-X main.version="+want, "-o", bin, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	for _, arg := range []string{"--version", "version"} {
		out, err := exec.Command(bin, arg).CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v", arg, err)
		}
		if got := strings.TrimSpace(string(out)); got != want {
			t.Errorf("%s reported %q, want %q", arg, got, want)
		}
	}
}
