// Package safe implements the write boundary: every write the tool performs
// passes through one containment check against the managed directories.
//
// The guard resolves real paths after symlink resolution, so a path that
// escapes the managed directories by way of a symbolic link is rejected on
// where it points, not on how it was spelled.
package safe

import (
	"crypto/rand"
	"encoding/hex"
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
	// handles stay anchored to the directories opened when the guard is made.
	handles []*os.Root
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
			_ = g.Close()
			return nil, fmt.Errorf("safe: creating %s: %w", abs, err)
		}
		real, err := realPath(abs)
		if err != nil {
			_ = g.Close()
			return nil, fmt.Errorf("safe: resolving %s: %w", abs, err)
		}
		handle, err := os.OpenRoot(abs)
		if err != nil {
			for _, opened := range g.handles {
				_ = opened.Close()
			}
			return nil, fmt.Errorf("safe: opening %s: %w", abs, err)
		}
		g.roots = append(g.roots, real)
		g.display = append(g.display, abs)
		g.handles = append(g.handles, handle)
	}
	return g, nil
}

// Close releases the directory handles held by the guard.
func (g *Guard) Close() error {
	var first error
	for _, root := range g.handles {
		if err := root.Close(); err != nil && first == nil {
			first = err
		}
	}
	g.handles = nil
	return first
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
	// A missing file is a valid tail for a path about to be written, but an
	// unresolved symlink is not: joining its lexical name would make it appear
	// contained even though the eventual open follows the link target.
	if info, lerr := os.Lstat(path); lerr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("safe: unresolved symlink %s: %w", path, err)
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

// OpenFile opens a path beneath one of the managed roots. The opened root
// handle prevents later path-component changes from redirecting the operation
// outside that directory tree.
func (g *Guard) OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	root, rel, err := g.rootFor(path)
	if err != nil {
		return nil, err
	}
	if err := g.Check(path); err != nil {
		return nil, err
	}
	return root.OpenFile(rel, flag, perm)
}

// MkdirAll creates directories beneath a managed root using its anchored
// handle, so a replaced path component cannot redirect creation elsewhere.
func (g *Guard) MkdirAll(path string, perm os.FileMode) error {
	root, rel, err := g.rootFor(path)
	if err != nil {
		return err
	}
	if err := g.Check(path); err != nil {
		return err
	}
	return root.MkdirAll(rel, perm)
}

// Remove removes a path beneath a managed root through its anchored handle.
func (g *Guard) Remove(path string) error {
	root, rel, err := g.rootFor(path)
	if err != nil {
		return err
	}
	return root.Remove(rel)
}

// AtomicWriteFile atomically replaces path with content using a temporary file
// created and renamed beneath the same anchored managed root.
func (g *Guard) AtomicWriteFile(path string, content []byte, perm os.FileMode) error {
	root, rel, err := g.rootFor(path)
	if err != nil {
		return err
	}
	if err := g.Check(path); err != nil {
		return err
	}
	dir, base := filepath.Split(rel)
	if base == "" || base == "." {
		return fmt.Errorf("safe: refusing to replace a managed directory")
	}
	var temp string
	var file *os.File
	for tries := 0; tries < 10; tries++ {
		random := make([]byte, 12)
		if _, err := rand.Read(random); err != nil {
			return fmt.Errorf("safe: creating temporary name: %w", err)
		}
		temp = filepath.Join(dir, ".astrogui-save-"+hex.EncodeToString(random))
		file, err = root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("safe: creating temporary file: %w", err)
		}
	}
	if file == nil {
		return fmt.Errorf("safe: could not allocate a unique temporary file")
	}
	defer root.Remove(temp)
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("safe: writing temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("safe: closing temporary file: %w", err)
	}
	if err := root.Rename(temp, rel); err != nil {
		return fmt.Errorf("safe: replacing %s: %w", path, err)
	}
	return nil
}

// rootFor selects the most specific managed root containing path, using the
// configured spelling to derive a relative path for the already-open handle.
func (g *Guard) rootFor(path string) (*os.Root, string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	best, bestLen := -1, -1
	var rel string
	for i, display := range g.display {
		candidate, err := filepath.Rel(display, abs)
		if err != nil || !containsPath(display, abs) {
			continue
		}
		if len(display) > bestLen {
			best, bestLen, rel = i, len(display), candidate
		}
	}
	if best < 0 {
		return nil, "", fmt.Errorf("%w: %s", ErrEscape, path)
	}
	return g.handles[best], rel, nil
}

// Assert is Check panicking on violation; for use in wiring where a
// violation is a programming error.
func (g *Guard) Assert(paths ...string) {
	if err := g.Check(paths...); err != nil {
		panic(err)
	}
}
