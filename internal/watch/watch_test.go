package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// awaitEvent waits for an event matching op on path.
func awaitEvent(t *testing.T, events <-chan Event, op Op, path string) bool {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return false
			}
			if ev.Op == op && samePath(ev.Path, path) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	if err1 == nil && err2 == nil {
		return ra == rb
	}
	// b may not exist (delete events): compare lexically after cleaning.
	return filepath.Clean(a) == filepath.Clean(b)
}

// TestPostCreatedOutsideToolProducesEvent verifies a post created by hand
// while the watcher runs is observed (task 3.4).
func TestPostCreatedOutsideToolProducesEvent(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	w, err := New(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer w.Close()

	// A post folder created after startup, including a directory created
	// after startup (the watcher must attach to new directories).
	postDir := filepath.Join(nested, "new-idea")
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(postDir, "index.md")
	if err := os.WriteFile(file, []byte("idea"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !awaitEvent(t, w.Events(), Create, file) {
		t.Fatal("create event for the post file never arrived")
	}
}

// TestModifyRenameDeleteEvents verifies each change kind maps to its event.
func TestModifyRenameDeleteEvents(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer w.Close()

	file := filepath.Join(dir, "post.md")
	if err := os.WriteFile(file, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !awaitEvent(t, w.Events(), Create, file) {
		t.Fatal("create event missing")
	}

	// A non-truncating write: on some platforms (macOS kqueue) a truncating
	// rewrite of a watched file surfaces as create rather than write, so the
	// modify mapping is verified with an ordinary in-place append.
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(" more"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if !awaitEvent(t, w.Events(), Modify, file) {
		t.Fatal("modify event missing")
	}

	renamed := filepath.Join(dir, "post-renamed.md")
	if err := os.Rename(file, renamed); err != nil {
		t.Fatal(err)
	}
	// fsnotify reports Rename on the old name, Create on the new; both are
	// acceptable as long as the change is observed on one of them.
	if !awaitEvent(t, w.Events(), Rename, file) && !awaitEvent(t, w.Events(), Create, renamed) {
		t.Fatal("rename change never arrived")
	}

	if err := os.Remove(renamed); err != nil {
		t.Fatal(err)
	}
	if !awaitEvent(t, w.Events(), Delete, renamed) {
		t.Fatal("delete event missing")
	}
}

// TestEventsStayUnderWatchedRoots verifies paths outside the watched
// directories produce nothing.
func TestEventsStayUnderWatchedRoots(t *testing.T) {
	base := t.TempDir()
	watched := filepath.Join(base, "watched")
	other := filepath.Join(base, "other")
	for _, d := range []string{watched, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w, err := New(watched)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer w.Close()

	if err := os.WriteFile(filepath.Join(other, "x.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Drain briefly: nothing should name the outside path.
	deadline := time.After(300 * time.Millisecond)
	for {
		select {
		case ev := <-w.Events():
			t.Errorf("event outside watched roots: %v", ev)
		case <-deadline:
			return
		}
	}
}
