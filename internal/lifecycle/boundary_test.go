package lifecycle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/astrogui/astrogui/internal/posts"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=astrogui-test", "GIT_AUTHOR_EMAIL=test@astrogui.dev",
		"GIT_COMMITTER_NAME=astrogui-test", "GIT_COMMITTER_EMAIL=test@astrogui.dev",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// TestCommittedMoveReportsAsRename verifies a move applied to a post with
// committed history is reported by git as a rename, and a never-committed
// post presents no history discontinuity (task 4.7).
func TestCommittedMoveReportsAsRename(t *testing.T) {
	m, base := newManager(t)
	git(t, base, "init", "--initial-branch=main")
	git(t, base, "config", "user.name", "astrogui-test")
	git(t, base, "config", "user.email", "test@astrogui.dev")

	// Committed post: create, commit at its ideas location, then move.
	mustCreate(t, m, "committed-post")
	git(t, base, "add", "-A")
	git(t, base, "commit", "-m", "capture ideas")

	if err := m.Move("committed-post", posts.StateIdeas, posts.StateWIP); err != nil {
		t.Fatalf("move: %v", err)
	}

	// The tool performs no version control operation (task 6.3): the staging
	// here is what the user's explicit commit action would do.
	git(t, base, "add", "-A")
	status := git(t, base, "status", "--porcelain")
	renamed := false
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "R ") && strings.Contains(line, "committed-post") {
			renamed = true
		}
	}
	if !renamed {
		t.Errorf("git does not report the move as a rename:\n%s", status)
	}

	// Never-committed post: a move stages nothing and breaks nothing.
	mustCreate(t, m, "never-committed")
	if err := m.Move("never-committed", posts.StateIdeas, posts.StateWIP); err != nil {
		t.Fatalf("move never-committed: %v", err)
	}
	status2 := git(t, base, "status", "--porcelain")
	for _, line := range strings.Split(status2, "\n") {
		if strings.HasPrefix(line, "D ") || strings.HasPrefix(line, " D") {
			if strings.Contains(line, "never-committed") {
				t.Errorf("never-committed post shows a deletion (history discontinuity): %s", line)
			}
		}
	}
}

// TestFullCycleTouchesNothingOutsideManagedDirs runs create → edit → publish
// against a fixture project and asserts no file outside the draft and content
// directories was created, modified, or deleted (task 2.5).
func TestFullCycleTouchesNothingOutsideManagedDirs(t *testing.T) {
	m, base := newManager(t)

	// The project's unrelated files: configuration, manifest, env, source.
	projectFiles := map[string]string{
		"astro.config.mjs": "export default {}",
		"package.json":     `{"name":"fixture","dependencies":{"astro":"^7"}}`,
		".env":             "SECRET=1",
		filepath.Join("src", "content.config.ts"):              "export const collections = {}",
		filepath.Join("src", "pages", "index.astro"):           "---\n---\n<h1>hi</h1>",
		filepath.Join("src", "content", "blog", "existing.md"): "---\ntitle: Existing\n---\nalready published",
	}
	for rel, content := range projectFiles {
		path := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	before := snapshotOutsideManaged(t, base, m)

	// The full cycle: capture an idea, write it with an image, publish it.
	idea, err := m.Create(posts.StateIdeas, "cycle-post", map[string]any{
		"title": "Cycle Post",
		"date":  "2026-01-01",
	}, []byte("A first line.\n"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := os.WriteFile(filepath.Join(base, "drafts", "ideas", idea.Name, "pic.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Editing changes the body only; frontmatter survives untouched.
	if err := posts.SaveBody(idea.File, []byte("A first line.\n\nWords and ![pic](pic.png)\n"), ideaModTime(t, idea.File)); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if err := m.Move(idea.Name, posts.StateIdeas, posts.StateWIP); err != nil {
		t.Fatalf("move to wip: %v", err)
	}
	if err := m.Move(idea.Name, posts.StateWIP, posts.StatePublished); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if after := snapshotOutsideManaged(t, base, m); !snapshotsEqual(after, before) {
		t.Errorf("the cycle touched files outside the managed directories:\nbefore %v\nafter  %v", before, after)
	}

	// The post arrived published, intact.
	published := filepath.Join(base, "src", "content", "blog", idea.Name, "index.md")
	if data, err := os.ReadFile(published); err != nil || len(data) == 0 {
		t.Errorf("published post missing or empty: %v", err)
	}
}

func ideaModTime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime()
}

// snapshotOutsideManaged records path -> hash for everything in the project
// except the draft and content directories.
func snapshotOutsideManaged(t *testing.T, base string, m *Manager) map[string]string {
	t.Helper()
	out := map[string]string{}
	managed := []string{m.Ideas, m.WIP, m.Content}
	filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		for _, root := range managed {
			if path == root {
				if d.IsDir() {
					return filepath.SkipDir
				}
			}
		}
		for _, root := range managed {
			if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil // inside a managed dir
			}
		}
		if !d.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[path] = shortHash(data)
		}
		return nil
	})
	return out
}

func snapshotsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func shortHash(b []byte) string {
	h := 0
	for _, c := range b {
		h = h*31 + int(c)
	}
	s := ""
	if h == 0 {
		return "0"
	}
	for h > 0 {
		s = string(rune('a'+h%16)) + s
		h /= 16
	}
	return s
}
