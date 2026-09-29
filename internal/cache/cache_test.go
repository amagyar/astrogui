package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withTestCache isolates the tool's own directory for the test.
func withTestCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", filepath.Join(dir, "AppData"))
}

// TestFirstSeenRecordedOnce verifies the first-seen timestamp is recorded the
// first time and stable afterwards (task 3.5).
func TestFirstSeenRecordedOnce(t *testing.T) {
	withTestCache(t)
	c, err := Open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	first := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	got, newly := c.FirstSeen("/blog", "post-a", first)
	if !newly || !got.Equal(first) {
		t.Fatalf("first call: got %v newly=%v", got, newly)
	}
	later := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	got, newly = c.FirstSeen("/blog", "post-a", later)
	if newly || !got.Equal(first) {
		t.Fatalf("second call: got %v newly=%v, want the original timestamp", got, newly)
	}
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	reopened, err := Open()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got, _ := reopened.FirstSeen("/blog", "post-a", later); !got.Equal(first) {
		t.Errorf("after reopen: got %v, want %v", got, first)
	}
}

// TestDeletingCacheLosesOnlyDerivedHistory verifies the cache is disposable:
// deleting the file loses first-seen ages and transition history but no board
// content (tasks 3.5 and 3.6).
func TestDeletingCacheLosesOnlyDerivedHistory(t *testing.T) {
	withTestCache(t)
	c, err := Open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	c.FirstSeen("/blog", "post-a", time.Now())
	c.RecordTransition("/blog", "post-a", "ideas", "wip", time.Now())
	c.RecordTransition("/blog", "post-a", "wip", "published", time.Now())
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// The board's content comes from the filesystem; with the cache present
	// the funnel reports recorded progression.
	f := c.FunnelFor("/blog", map[string]int{"ideas": 1, "wip": 0, "published": 1})
	if f.Counts["published"] != 1 || f.Advanced["published"] != 1 {
		t.Errorf("funnel with cache = %+v", f)
	}

	if err := os.Remove(c.Path()); err != nil {
		t.Fatalf("delete cache: %v", err)
	}
	fresh, err := Open()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	f2 := fresh.FunnelFor("/blog", map[string]int{"ideas": 1, "wip": 0, "published": 1})
	if f2.Counts["published"] != 1 {
		t.Errorf("current counts must survive cache deletion: %+v", f2)
	}
	if f2.Advanced["published"] != 0 {
		t.Errorf("derived progression should be gone after deletion: %+v", f2)
	}
	// First-seen falls back to the filesystem (checked in posts tests): the
	// cache contributes nothing the board cannot reconstruct.
}

// TestTransitionsSurviveRestart verifies a move recorded in one session is
// reported by the funnel after a restart (task 3.6).
func TestTransitionsSurviveRestart(t *testing.T) {
	withTestCache(t)
	c, err := Open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	at := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	c.RecordTransition("/blog", "p1", "ideas", "wip", at)
	c.RecordTransition("/blog", "p2", "ideas", "wip", at)
	c.RecordTransition("/blog", "p1", "wip", "published", at)
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	reopened, err := Open()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	f := reopened.FunnelFor("/blog", map[string]int{"ideas": 0, "wip": 1, "published": 1})
	if f.Advanced["wip"] != 2 {
		t.Errorf("advanced into wip = %d, want 2", f.Advanced["wip"])
	}
	if f.Advanced["published"] != 1 {
		t.Errorf("advanced into published = %d, want 1", f.Advanced["published"])
	}
}
