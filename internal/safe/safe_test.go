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
	// "linked/new/deeper/file.md" does not exist; its ancestor resolves outside.
	if err := g.Check(filepath.Join(ideas, "linked", "new", "file.md")); !errors.Is(err, ErrEscape) {
		t.Errorf("write through not-yet-existing path under escaping symlink accepted: %v", err)
	}
	if err := g.Check(filepath.Join(ideas, "fresh", "file.md")); err != nil {
		t.Errorf("plain new file rejected: %v", err)
	}
}
