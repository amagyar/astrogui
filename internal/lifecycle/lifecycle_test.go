package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/astrogui/astrogui/internal/cache"
	"github.com/astrogui/astrogui/internal/posts"
	"github.com/astrogui/astrogui/internal/safe"
)

// newManager builds a manager over a fixture project with three state dirs.
func newManager(t *testing.T) (*Manager, string) {
	t.Helper()
	withIsolatedToolDir(t)
	base := t.TempDir()
	ideas := filepath.Join(base, "drafts", "ideas")
	wip := filepath.Join(base, "drafts", "wip")
	content := filepath.Join(base, "src", "content", "blog")
	guard, err := safe.New(ideas, wip, content)
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	c, err := cache.Open()
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	m := New(ideas, wip, content, guard, c, base)
	return m, base
}

// withIsolatedToolDir keeps the tool's own directory out of the real one.
func withIsolatedToolDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", filepath.Join(dir, "AppData"))
}

func mustCreate(t *testing.T, m *Manager, name string) *posts.Post {
	t.Helper()
	p, err := m.Create(posts.StateIdeas, name, map[string]any{"title": name}, []byte("# "+name+"\n\nSome words.\n"))
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return p
}

func mustCreateQuiet(t *testing.T, m *Manager, name string) {
	t.Helper()
	if _, err := m.Create(posts.StateIdeas, name, map[string]any{"title": name}, []byte("# "+name+"\n\nSome words.\n")); err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
}

func dirExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

func hashTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[path] = hex.EncodeToString(sum[:])
		return nil
	})
	return out
}

// TestMoveIsAtomicAndBytePreserving verifies a transition is one rename whose
// contents survive byte-for-byte, image files included (tasks 4.1, 4.2).
func TestMoveIsAtomicAndBytePreserving(t *testing.T) {
	m, base := newManager(t)
	mustCreate(t, m, "async-rust")
	postDir := filepath.Join(base, "drafts", "ideas", "async-rust")
	if err := os.WriteFile(filepath.Join(postDir, "cover.jpeg"), []byte("jpegdata"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Body references the image so the publish gate sees a resolving ref.
	if err := os.WriteFile(filepath.Join(postDir, "index.md"), []byte("---\ntitle: Async Rust\nslug: async-rust\ndate: 2026-01-01\n---\n\n![cover](cover.jpeg)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := hashTree(t, postDir)

	if err := m.Move("async-rust", posts.StateIdeas, posts.StateWIP); err != nil {
		t.Fatalf("move: %v", err)
	}
	moved := filepath.Join(base, "drafts", "wip", "async-rust")
	if !dirExists(t, moved) {
		t.Fatal("post not in wip after move")
	}
	if dirExists(t, postDir) {
		t.Fatal("post still in ideas after move")
	}
	after := hashTree(t, moved)
	if len(after) != len(before) {
		t.Fatalf("file count changed: before %d after %d", len(before), len(after))
	}
	for _, k := range []string{"index.md", "cover.jpeg"} {
		rel := filepath.Base(k)
		var b, a string
		for path, sum := range before {
			if filepath.Base(path) == rel {
				b = sum
			}
		}
		for path, sum := range after {
			if filepath.Base(path) == rel {
				a = sum
			}
		}
		if b == "" || a == "" || a != b {
			t.Errorf("%s changed across the move: before %s after %s", rel, b, a)
		}
	}
}

// TestInterruptedMoveLeavesBothUnchanged verifies a failing rename leaves
// source and destination exactly as they were (task 4.1).
func TestInterruptedMoveLeavesBothUnchanged(t *testing.T) {
	m, base := newManager(t)
	mustCreate(t, m, "stalled-post")
	src := filepath.Join(base, "drafts", "ideas", "stalled-post")
	before := hashTree(t, src)

	m.rename = func(oldname, newname string) error {
		return fmt.Errorf("simulated interruption")
	}
	err := m.Move("stalled-post", posts.StateIdeas, posts.StateWIP)
	if err == nil {
		t.Fatal("expected failure")
	}
	if after := hashTree(t, src); !hashTreesEqual(after, before) {
		t.Error("source changed during a failed move")
	}
	if dirExists(t, filepath.Join(base, "drafts", "wip", "stalled-post")) {
		t.Error("destination appeared during a failed move")
	}
}

func hashTreesEqual(a, b map[string]string) bool {
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

// TestCreatePinsSlugToPostName verifies new posts are folders whose slug pins
// the entry id to the post's identity (task 4.3).
func TestCreatePinsSlugToPostName(t *testing.T) {
	m, _ := newManager(t)
	p, err := m.Create(posts.StateIdeas, "My Great Idea!", map[string]any{"title": "My Great Idea!"}, []byte("A line.\n"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Name != "my-great-idea" {
		t.Errorf("Name = %q, want my-great-idea", p.Name)
	}
	fm := p.FrontmatterMap()
	if fm["slug"] != "my-great-idea" {
		t.Errorf("slug = %v, want pinned to the folder name", fm["slug"])
	}
	if fm["title"] != "My Great Idea!" {
		t.Errorf("title = %v", fm["title"])
	}

	// No workflow-state field is written.
	for _, forbidden := range []string{"status", "state", "draft", "column"} {
		if _, ok := fm[forbidden]; ok {
			t.Errorf("frontmatter contains workflow field %q", forbidden)
		}
	}

	// Duplicate title gets a unique, still-useful name.
	p2, err := m.Create(posts.StateIdeas, "My Great Idea!", map[string]any{"title": "My Great Idea!"}, []byte("Another.\n"))
	if err != nil {
		t.Fatalf("create duplicate: %v", err)
	}
	if p2.Name == p.Name {
		t.Errorf("duplicate name collision: %q", p2.Name)
	}
}

// TestPublishGateBlocksEachFailure verifies each check failure blocks the
// move and names the specific problem (task 4.4).
func TestPublishGateBlocksEachFailure(t *testing.T) {
	m, base := newManager(t)

	cases := []struct {
		name    string
		content string
		asset   string
		wantErr string
	}{
		{"missing-image", "---\ntitle: T\ndate: 2026-01-01\n---\n\n![gone](gone.png)\n", "", "gone.png"},
		{"missing-title", "---\ndate: 2026-01-01\n---\n\nbody\n", "", "no title"},
		{"unparseable-date", "---\ntitle: T\ndate: someday\n---\n\nbody\n", "", "date"},
		{"empty-body", "---\ntitle: T\ndate: 2026-01-01\n---\n\n", "", "body is empty"},
	}
	for i, tc := range cases {
		name := fmt.Sprintf("%s-%d", tc.name, i)
		if _, err := m.Create(posts.StateIdeas, name, nil, []byte(tc.content)); err != nil {
			t.Fatalf("create: %v", err)
		}
		// Overwrite with the raw fixture content (Create adds pinned slug).
		if err := os.WriteFile(filepath.Join(base, "drafts", "ideas", name, "index.md"), []byte(tc.content), 0o644); err != nil {
			t.Fatal(err)
		}
		err := m.Move(name, posts.StateIdeas, posts.StatePublished)
		var cf *CheckFailure
		if !errors.As(err, &cf) {
			t.Errorf("%s: expected CheckFailure, got %v", tc.name, err)
			continue
		}
		found := false
		for _, pr := range cf.Problems {
			if contains(pr.Message, tc.wantErr) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: failure %q does not name the problem %q", tc.name, cf.Error(), tc.wantErr)
		}
		if dirExists(t, filepath.Join(base, "src", "content", "blog", name)) {
			t.Errorf("%s: post moved despite failing check", tc.name)
		}
		if !dirExists(t, filepath.Join(base, "drafts", "ideas", name)) {
			t.Errorf("%s: post left its original state", tc.name)
		}
	}
}

// TestFutureDateBlocked verifies a future publication date is refused.
func TestFutureDateBlocked(t *testing.T) {
	m, base := newManager(t)
	content := "---\ntitle: T\ndate: 2100-01-01\n---\n\nbody\n"
	name := "future-dated"
	if _, err := m.Create(posts.StateIdeas, name, nil, []byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "drafts", "ideas", name, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var cf *CheckFailure
	err := m.Move(name, posts.StateIdeas, posts.StatePublished)
	if !errors.As(err, &cf) {
		t.Fatalf("expected CheckFailure, got %v", err)
	}
	if !contains(cf.Error(), "future") {
		t.Errorf("error %q does not name the future date", cf.Error())
	}
}

// TestAssetOutsidePostDirFailsPreflight verifies a reference resolving
// outside the post's own directory fails with the reference named (4.5).
func TestAssetOutsidePostDirFailsPreflight(t *testing.T) {
	m, base := newManager(t)
	// A shared asset one level up from the post folder.
	shared := filepath.Join(base, "drafts", "ideas", "shared-logo.png")
	if err := os.WriteFile(shared, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	content := "---\ntitle: T\ndate: 2026-01-01\n---\n\n![logo](../shared-logo.png)\n"
	name := "outside-asset"
	if _, err := m.Create(posts.StateIdeas, name, nil, []byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "drafts", "ideas", name, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var cf *CheckFailure
	err := m.Move(name, posts.StateIdeas, posts.StatePublished)
	if !errors.As(err, &cf) {
		t.Fatalf("expected CheckFailure, got %v", err)
	}
	if !contains(cf.Error(), "../shared-logo.png") {
		t.Errorf("error %q does not name the offending reference", cf.Error())
	}
	if !dirExists(t, filepath.Join(base, "drafts", "ideas", name)) {
		t.Error("post moved despite outside-asset failure")
	}
}

// TestDestinationConflictRefused verifies a move onto an existing post is
// refused with both unchanged (task 4.6).
func TestDestinationConflictRefused(t *testing.T) {
	m, base := newManager(t)
	mustCreate(t, m, "same-name")
	wipPost, err := m.Create(posts.StateWIP, "same-name", map[string]any{"title": "WIP"}, []byte("wip\n"))
	if err != nil {
		t.Fatal(err)
	}
	_ = wipPost

	err = m.Move("same-name", posts.StateWIP, posts.StateIdeas)
	if !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("expected ErrDestinationExists, got %v", err)
	}
	if !dirExists(t, filepath.Join(base, "drafts", "wip", "same-name")) {
		t.Error("wip post vanished on refused move")
	}
	if !dirExists(t, filepath.Join(base, "drafts", "ideas", "same-name")) {
		t.Error("ideas post vanished on refused move")
	}
}

// TestCrossDeviceMoveRefused verifies a cross-filesystem rename is refused,
// names both locations, and leaves the post unchanged (task 4.8).
func TestCrossDeviceMoveRefused(t *testing.T) {
	m, base := newManager(t)
	mustCreate(t, m, "cross-device-post")
	src := filepath.Join(base, "drafts", "ideas", "cross-device-post")
	before := hashTree(t, src)

	m.rename = func(oldname, newname string) error {
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: syscall.EXDEV}
	}
	err := m.Move("cross-device-post", posts.StateIdeas, posts.StateWIP)
	if !errors.Is(err, ErrCrossDevice) {
		t.Fatalf("expected ErrCrossDevice, got %v", err)
	}
	msg := err.Error()
	if !contains(msg, "drafts/ideas") || !contains(msg, "drafts/wip") {
		t.Errorf("refusal %q does not name both locations", msg)
	}
	if after := hashTree(t, src); !hashTreesEqual(after, before) {
		t.Error("post changed by a refused cross-device move")
	}
	if dirExists(t, filepath.Join(base, "drafts", "wip", "cross-device-post")) {
		t.Error("destination appeared for a refused cross-device move")
	}
}

// TestLoosePostNeverMoved verifies read-only loose posts refuse moves.
func TestLoosePostNeverMoved(t *testing.T) {
	m, _ := newManager(t)
	loose := filepath.Join(m.Content, "an-old-post.md")
	if err := os.WriteFile(loose, []byte("---\ntitle: Old\n---\npublished once\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := m.Move("an-old-post", posts.StatePublished, posts.StateWIP)
	if err == nil || !contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only refusal, got %v", err)
	}
	if !dirExists(t, loose) {
		t.Error("loose post was moved")
	}
}

// TestMoveRecordsTransition verifies the funnel's derived data records moves.
func TestMoveRecordsTransition(t *testing.T) {
	m, _ := newManager(t)
	mustCreate(t, m, "tracked-post")
	if err := m.Move("tracked-post", posts.StateIdeas, posts.StateWIP); err != nil {
		t.Fatal(err)
	}
	transitions := m.Cache.Transitions(m.Project)
	if len(transitions) != 1 || transitions[0].Post != "tracked-post" {
		t.Fatalf("transitions = %+v", transitions)
	}
	if transitions[0].From != posts.StateIdeas || transitions[0].To != posts.StateWIP {
		t.Errorf("transition endpoints wrong: %+v", transitions[0])
	}
}

// TestMoveOutsideGuardRefused verifies every move passes the containment
// guard: a destination outside the managed directories is refused.
func TestMoveOutsideGuardRefused(t *testing.T) {
	m, _ := newManager(t)
	mustCreate(t, m, "escape-attempt")
	m.Content = "/tmp/astrogui-escape-test" // not under the guard
	defer os.RemoveAll("/tmp/astrogui-escape-test")
	err := m.Move("escape-attempt", posts.StateIdeas, posts.StatePublished)
	if err == nil || !contains(err.Error(), "escape") && !errors.Is(err, safe.ErrEscape) && !contains(err.Error(), "escapes") {
		t.Fatalf("expected boundary refusal, got %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
