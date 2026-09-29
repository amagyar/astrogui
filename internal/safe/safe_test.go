package safe

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGuardAcceptsInsideAndRejectsOutside(t *testing.T) {
	base := t.TempDir()
	ideas := filepath.Join(base, "drafts", "ideas")
	content := filepath.Join(base, "src", "content", "blog")
	g, err := New(ideas, content)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })

	inside := []string{
		filepath.Join(ideas, "my-post", "index.md"),
		filepath.Join(content, "published-post", "cover.png"),
		content, // a root itself is inside
	}
	for _, p := range inside {
		if err := g.Check(p); err != nil {
			t.Errorf("Check(%q) = %v, want accepted", p, err)
		}
	}

	outside := []string{
		filepath.Join(base, "astro.config.mjs"),
		filepath.Join(base, "package.json"),
		filepath.Join(base, "drafts", "other", "post", "index.md"), // sibling draft dir
		filepath.Join(base, "src", "content", "notes", "x.md"),     // sibling collection
		"/etc/passwd",
	}
	for _, p := range outside {
		if err := g.Check(p); !errors.Is(err, ErrEscape) {
			t.Errorf("Check(%q) = %v, want ErrEscape", p, err)
		}
	}
}

// TestGuardRejectsSymlinkEscape verifies containment is decided on the
// resolved real path: a symlink inside a root that points outside is refused,
// and a symlinked root still admits its contents (task 2.4).
func TestGuardRejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	ideas := filepath.Join(base, "drafts", "ideas")
	if err := os.MkdirAll(filepath.Join(ideas, "post"), 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(base, "outside")
	if err := os.MkdirAll(secret, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(ideas, "post", "escape")); err != nil {
		t.Fatal(err)
	}

	g, err := New(ideas)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })
	if err := g.Check(filepath.Join(ideas, "post", "escape", "victim.txt")); !errors.Is(err, ErrEscape) {
		t.Errorf("symlink escape accepted: %v", err)
	}

	// A symlinked root: guarding through the link still contains the target.
	link := filepath.Join(base, "ideas-link")
	if err := os.Symlink(ideas, link); err != nil {
		t.Fatal(err)
	}
	g2, err := New(link)
	if err != nil {
		t.Fatalf("new via link: %v", err)
	}
	t.Cleanup(func() { _ = g2.Close() })
	if err := g2.Check(filepath.Join(link, "post", "index.md")); err != nil {
		t.Errorf("content under symlinked root rejected: %v", err)
	}
	if err := g2.Check(filepath.Join(secret, "file.txt")); !errors.Is(err, ErrEscape) {
		t.Errorf("path outside symlinked root accepted: %v", err)
	}
}

// TestNonExistentTailResolves verifies a file that does not exist yet is
// judged by its deepest existing ancestor, so a symlinked parent directory
// cannot smuggle writes outside the boundary.
func TestNonExistentTailResolves(t *testing.T) {
	base := t.TempDir()
	ideas := filepath.Join(base, "drafts", "ideas")
	outside := filepath.Join(base, "elsewhere")
	for _, d := range []string{ideas, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(ideas, "linked")); err != nil {
		t.Fatal(err)
	}
	g, err := New(ideas)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })
	// "linked/new/deeper/file.md" does not exist; its ancestor resolves outside.
	if err := g.Check(filepath.Join(ideas, "linked", "new", "file.md")); !errors.Is(err, ErrEscape) {
		t.Errorf("write through not-yet-existing path under escaping symlink accepted: %v", err)
	}
	if err := g.Check(filepath.Join(ideas, "fresh", "file.md")); err != nil {
		t.Errorf("plain new file rejected: %v", err)
	}
}

func TestGuardRejectsDanglingSymlinkAndAnchorsWrites(t *testing.T) {
	base := t.TempDir()
	managed := filepath.Join(base, "drafts", "ideas")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(managed, "dangling.png")
	target := filepath.Join(outside, "created.png")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	g, err := New(managed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Close() })

	if err := g.Check(link); !errors.Is(err, ErrEscape) {
		t.Fatalf("dangling symlink accepted: %v", err)
	}
	if _, err := g.OpenFile(link, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644); err == nil {
		t.Fatal("exclusive rooted create followed a dangling symlink")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside target was created: %v", err)
	}
}

func TestAtomicWriteRejectsEscapingParentSymlink(t *testing.T) {
	base := t.TempDir()
	managed := filepath.Join(base, "drafts", "ideas")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(filepath.Join(managed, "post"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(managed, "post", "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	g, err := New(managed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Close() })

	if err := g.AtomicWriteFile(filepath.Join(link, "victim.md"), []byte("no"), 0o644); !errors.Is(err, ErrEscape) {
		t.Fatalf("atomic write through escaping symlink = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "victim.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside file was created: %v", err)
	}
}
