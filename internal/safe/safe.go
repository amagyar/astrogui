// Package safe implements the write boundary: every write the tool performs
// passes through one containment check against the managed directories.
//
// The guard resolves real paths after symlink resolution, so a path that
// escapes the managed directories by way of a symbolic link is rejected on
// where it points, not on how it was spelled.
package safe

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrEscape is returned when a path resolves outside every allowed root.
var ErrEscape = errors.New("path escapes the managed directories")

// Guard confines paths to a set of allowed root directories.
type Guard struct {
	// roots are absolute, symlink-resolved allowed directories.
	roots []string
	// display roots keep the original spelling for error messages.
	display []string
}

// New creates a guard over the given directories. Roots that do not exist yet
// are created, because the tool creates the draft directories on first run.
func New(dirs ...string) (*Guard, error) {
	g := &Guard{}
	for _, dir := range dirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("safe: %w", err)
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return nil, fmt.Errorf("safe: creating %s: %w", abs, err)
		}
		real, err := realPath(abs)
		if err != nil {
			return nil, fmt.Errorf("safe: resolving %s: %w", abs, err)
		}
		g.roots = append(g.roots, real)
		g.display = append(g.display, abs)
	}
	return g, nil
}

// Roots returns the allowed directories (absolute, as configured).
func (g *Guard) Roots() []string { return append([]string(nil), g.display...) }

// realPath resolves symlinks in path. For a path whose tail does not exist
// yet (a file about to be written), it resolves the deepest existing
// ancestor and rejoins the remainder, so symlinked directories under the
// boundary are still evaluated by where they point.
func realPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	dir, rest := filepath.Split(path)
	if dir == path {
		return path, nil
	}
	base, err := realPath(filepath.Clean(dir))
	if err != nil {
		return "", err
	}
	return filepath.Join(base, rest), nil
}

// Contains reports whether path resolves inside an allowed root.
func (g *Guard) Contains(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	real, err := realPath(abs)
	if err != nil {
		return false
	}
	for _, root := range g.roots {
		if containsPath(root, real) {
			return true
		}
	}
	return false
}

// containsPath reports whether child is root itself or underneath it.
func containsPath(root, child string) bool {
	rel, err := filepath.Rel(root, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

// Check verifies that every given path stays inside the boundary, after
// symlink resolution, and returns a descriptive error otherwise. It is the
// single choke point every write passes through.
func (g *Guard) Check(paths ...string) error {
	for _, p := range paths {
		if !g.Contains(p) {
			return fmt.Errorf("%w: %s (allowed: %s)", ErrEscape, p, strings.Join(g.display, ", "))
		}
	}
	return nil
}

// Assert is Check panicking on violation; for use in wiring where a
// violation is a programming error.
func (g *Guard) Assert(paths ...string) {
	if err := g.Check(paths...); err != nil {
		panic(err)
	}
}
