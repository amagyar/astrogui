package config

import (
	"path/filepath"
	"testing"
	"time"
)

// withTestConfig points the config file at a temp dir for the test's duration.
// os.UserConfigDir honours XDG_CONFIG_HOME on Linux, $HOME on macOS, and
// AppData on Windows, so all three are redirected.
func withTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", filepath.Join(dir, "AppData"))
	return dir
}

// TestConfigRoundTrips verifies a saved configuration loads back identical
// (task 1.3).
func TestConfigRoundTrips(t *testing.T) {
	withTestConfig(t)
	root := "/blog"
	in := File{Projects: map[string]Project{
		root: {
			IdeasDir:   "notes/ideas",
			WipDir:     "notes/wip",
			ContentDir: "src/content/blog",
			Staleness:  14 * 24 * time.Hour,
			Collection: "blog",
		},
	}}
	if err := in.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(out.Projects) != 1 {
		t.Fatalf("loaded %d projects, want 1", len(out.Projects))
	}
	got := out.Projects[root]
	want := in.Projects[root]
	if got != want {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

// TestAbsentKeysFallBackToDefaults verifies each unset field yields the
// conventional layout (task 1.3).
func TestAbsentKeysFallBackToDefaults(t *testing.T) {
	withTestConfig(t)
	f, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(f.Projects) != 0 {
		t.Fatalf("expected no projects, got %d", len(f.Projects))
	}
	p := f.For("/blog")
	if p.IdeasDir != "drafts/ideas" {
		t.Errorf("IdeasDir = %q, want drafts/ideas", p.IdeasDir)
	}
	if p.WipDir != "drafts/wip" {
		t.Errorf("WipDir = %q, want drafts/wip", p.WipDir)
	}
	if p.ContentDir != "" {
		t.Errorf("ContentDir = %q, want empty (detected)", p.ContentDir)
	}
	if p.EffectiveStaleness() != DefaultStaleness {
		t.Errorf("Staleness = %v, want %v", p.EffectiveStaleness(), DefaultStaleness)
	}
	if got, want := p.IdeasPath("/blog"), filepath.FromSlash("/blog/drafts/ideas"); got != want {
		t.Errorf("IdeasPath = %q, want %q", got, want)
	}
}

// TestPartialOverrideKeepsOtherDefaults verifies a config that sets one field
// does not disturb the defaults of the rest.
func TestPartialOverrideKeepsOtherDefaults(t *testing.T) {
	withTestConfig(t)
	var f File
	f.Set("/blog", Project{ContentDir: "src/posts"})
	if err := f.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	p := reloaded.For("/blog")
	if got, want := p.ContentPath("/blog"), filepath.FromSlash("/blog/src/posts"); got != want {
		t.Errorf("ContentPath = %q, want %q", got, want)
	}
	if p.IdeasDir != "drafts/ideas" || p.WipDir != "drafts/wip" {
		t.Errorf("unexpected defaults disturbed: %+v", p)
	}
}
