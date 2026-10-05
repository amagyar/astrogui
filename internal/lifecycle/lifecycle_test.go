package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	trash := filepath.Join(base, "drafts", "trash")
	guard, err := safe.New(ideas, wip, content, trash)
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	t.Cleanup(func() { _ = guard.Close() })
	c, err := cache.Open()
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	m := New(ideas, wip, content, trash, guard, c, base)
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

func TestCreatePreservesYamlSignificantTitle(t *testing.T) {
	m, _ := newManager(t)
	title := "Plan #1: \"quoted\"\nsecond line"
	p, err := m.Create(posts.StateIdeas, "yaml-title", map[string]any{"title": title}, []byte("Body.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.FrontmatterMap()["title"]; got != title {
		t.Fatalf("title round trip = %q, want %q", got, title)
	}
	if len(p.FrontmatterMap()) != 2 {
		t.Fatalf("unexpected frontmatter fields were introduced: %#v", p.FrontmatterMap())
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
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: crossDeviceErrno()}
	}
	err := m.Move("cross-device-post", posts.StateIdeas, posts.StateWIP)
	if !errors.Is(err, ErrCrossDevice) {
		t.Fatalf("expected ErrCrossDevice, got %v", err)
	}
	msg := err.Error()
	if !contains(msg, filepath.Join("drafts", "ideas")) || !contains(msg, filepath.Join("drafts", "wip")) {
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

// TestMdxFolderPostFindsMovesAndPublishes verifies an index.mdx folder post
// is a first-class managed post: found by name, moved between states, and
// gated by the publish pre-flight like an index.md folder.
func TestMdxFolderPostFindsMovesAndPublishes(t *testing.T) {
	m, base := newManager(t)
	postDir := filepath.Join(base, "drafts", "ideas", "mdx-idea")
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ntitle: An MDX idea\nslug: mdx-idea\ndate: 2026-01-01\n---\n\nBody with substance.\n"
	if err := os.WriteFile(filepath.Join(postDir, "index.mdx"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := m.Find("mdx-idea")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if p == nil {
		t.Fatal("index.mdx folder post not found")
	}
	if p.Loose {
		t.Error("index.mdx folder post found as loose")
	}

	if err := m.Move("mdx-idea", posts.StateIdeas, posts.StateWIP); err != nil {
		t.Fatalf("move ideas->wip: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "drafts", "wip", "mdx-idea", "index.mdx")); err != nil {
		t.Fatalf("post not in wip as index.mdx: %v", err)
	}

	// The publish gate reads the entry file whichever form it takes.
	if err := m.Move("mdx-idea", posts.StateWIP, posts.StatePublished); err != nil {
		t.Fatalf("publish (gate): %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "src", "content", "blog", "mdx-idea", "index.mdx")); err != nil {
		t.Fatalf("post not published as index.mdx: %v", err)
	}
}

// TestSlugifyKeepsUnicodeLetters verifies names derive from the title's
// letters and numbers in any script, ASCII behavior is unchanged, and a
// title with nothing usable falls back to the placeholder.
func TestSlugifyKeepsUnicodeLetters(t *testing.T) {
	cases := map[string]string{
		"こんにちは世界":             "こんにちは世界",
		"Hello 世界!":           "hello-世界",
		"My  Great -- Idea!!": "my-great-idea",
		"Café Chronicles":     "café-chronicles",
		"!!! ??? ***":         "untitled",
		"Быстрая идея":        "быстрая-идея",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRenameKeepsSlugPinnedAndRefusesConflicts verifies a folder post renames
// atomically within its state, the slug pin follows the new name, and taken
// names and loose files are refused with both posts unchanged.
func TestRenameKeepsSlugPinnedAndRefusesConflicts(t *testing.T) {
	m, base := newManager(t)

	dir := filepath.Join(base, "drafts", "ideas", "my-great-idea")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ntitle: My Great Idea\nslug: my-great-idea\ndate: 2026-01-01\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	renamed, err := m.Rename("my-great-idea", posts.StateIdeas, "A Better Name!!")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Name != "a-better-name" {
		t.Errorf("renamed name = %q, want a-better-name", renamed.Name)
	}
	if _, err := os.Stat(filepath.Join(base, "drafts", "ideas", "a-better-name", "index.md")); err != nil {
		t.Fatalf("folder did not move: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(base, "drafts", "ideas", "a-better-name", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "slug: a-better-name") {
		t.Errorf("slug pin did not follow the rename:\n%s", data)
	}
	// The rest of the frontmatter survives the slug edit.
	if !strings.Contains(string(data), "title: My Great Idea") || !strings.Contains(string(data), "date: 2026-01-01") {
		t.Errorf("rename disturbed unrelated frontmatter:\n%s", data)
	}

	// A taken name is refused; both posts stay unchanged.
	if err := os.MkdirAll(filepath.Join(base, "drafts", "ideas", "my-great-idea"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "drafts", "ideas", "my-great-idea", "index.md"), []byte("---\ntitle: Other\n---\nx"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Rename("a-better-name", posts.StateIdeas, "My Great Idea"); !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("taken name err = %v, want ErrDestinationExists", err)
	}
	if _, err := os.Stat(filepath.Join(base, "drafts", "ideas", "a-better-name", "index.md")); err != nil {
		t.Fatal("refused rename still moved the post")
	}

	// Loose files are never renamed.
	loose := filepath.Join(base, "drafts", "ideas", "loose.md")
	if err := os.WriteFile(loose, []byte("loose"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Rename("loose", posts.StateIdeas, "elsewhere"); err == nil {
		t.Error("loose file was renamed")
	}
}

// TestRenameWithoutFrontmatterLeavesFileAlone verifies a folder post with no
// frontmatter renames without gaining (or being corrupted by) a slug line.
func TestRenameWithoutFrontmatterLeavesFileAlone(t *testing.T) {
	m, base := newManager(t)
	dir := filepath.Join(base, "drafts", "wip", "plain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("just a body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Rename("plain", posts.StateWIP, "renamed plain"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(base, "drafts", "wip", "renamed-plain", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "just a body\n" {
		t.Errorf("frontmatter-free post was rewritten:\n%s", data)
	}
}

// TestDiscardMovesToTrashWithCollisionSuffix verifies discarding moves the
// folder into the trash directory intact, suffixes on collision, and refuses
// loose files.
func TestDiscardMovesToTrashWithCollisionSuffix(t *testing.T) {
	m, base := newManager(t)
	ideas := filepath.Join(base, "drafts", "ideas")
	trash := filepath.Join(base, "drafts", "trash")

	dir := filepath.Join(ideas, "doomed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\ntitle: Doomed\n---\nbye"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cover.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := m.Discard("doomed", posts.StateIdeas); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ideas, "doomed")); !os.IsNotExist(err) {
		t.Fatal("post still in ideas after discard")
	}
	if _, err := os.Stat(filepath.Join(trash, "doomed", "index.md")); err != nil {
		t.Fatalf("post not in trash: %v", err)
	}
	if _, err := os.Stat(filepath.Join(trash, "doomed", "cover.png")); err != nil {
		t.Fatalf("image not discarded with the post: %v", err)
	}

	// Discarding the same name again suffixes rather than overwrites.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\ntitle: Doomed 2\n---\nbye"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Discard("doomed", posts.StateIdeas); err != nil {
		t.Fatalf("second discard: %v", err)
	}
	if _, err := os.Stat(filepath.Join(trash, "doomed-2", "index.md")); err != nil {
		t.Fatalf("collision not suffixed: %v", err)
	}

	// Loose files are never discarded.
	if err := os.WriteFile(filepath.Join(ideas, "loose.md"), []byte("loose"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Discard("loose", posts.StateIdeas); err == nil {
		t.Error("loose file was discarded")
	}
}

// TestFindInRefusesNonElementNames is the CodeQL path-injection regression:
// a request-derived name carrying separators or traversal must be refused at
// the choke point every entry name passes through, never reaching a
// filesystem path.
func TestFindInRefusesNonElementNames(t *testing.T) {
	m, _ := newManager(t)
	for _, name := range []string{"../escape", "a/b", `a\b`, ".", "..", "trailing/", "/leading", "nul\x00byte"} {
		if p, err := m.FindIn(name, posts.StateIdeas); err == nil {
			t.Errorf("FindIn(%q) accepted the name (post=%v)", name, p)
		}
		if p, err := m.Find(name); err == nil {
			t.Errorf("Find(%q) accepted the name (post=%v)", name, p)
		}
	}
	// Slugify's output always satisfies the name rule, including for
	// traversal-shaped input.
	for _, title := range []string{"../../../etc/passwd", "..\\..\\windows", "a/b/c"} {
		if !validPostName(Slugify(title)) {
			t.Errorf("Slugify(%q) produced a non-element name %q", title, Slugify(title))
		}
	}
}
