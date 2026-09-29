package posts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const trickyPost = `---
title: Async Rust   # trailing comment
date: 2026-09-01
tags: [rust, notes]
custom_field: keep me
---

# Async Rust

Body with **bold**, a ![cover](cover.jpeg) reference, and a code fence:

` + "```rust {filename=main.rs}" + `
async fn main() {}
` + "```" + `

Trailing double-space line breaks survive  
because bytes are never rewritten.
`

// writePostFolder creates a folder post with the given content and assets.
func writePostFolder(t *testing.T, dir, name, content string, assets ...string) string {
	t.Helper()
	postDir := filepath.Join(dir, name)
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(postDir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, a := range assets {
		if err := os.WriteFile(filepath.Join(postDir, a), []byte("asset"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return postDir
}

// TestPostFolderRoundTripsUnchanged verifies reading a post folder into the
// model and asking for its bytes back is byte-identical (task 3.1).
func TestPostFolderRoundTripsUnchanged(t *testing.T) {
	dir := t.TempDir()
	writePostFolder(t, dir, "async-rust", trickyPost, "cover.jpeg", "diagram.png")

	p, err := Read(filepath.Join(dir, "async-rust", "index.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if p.Name != "async-rust" {
		t.Errorf("Name = %q, want async-rust", p.Name)
	}
	if !p.HasFrontmatter() {
		t.Fatal("frontmatter not detected")
	}
	if string(p.Bytes()) != trickyPost {
		t.Errorf("round trip changed the file:\n got %q\nwant %q", p.Bytes(), trickyPost)
	}

	// Frontmatter and body are exact slices of the original bytes.
	fm := string(p.Frontmatter())
	if want := "title: Async Rust   # trailing comment\ndate: 2026-09-01\ntags: [rust, notes]\ncustom_field: keep me\n"; fm != want {
		t.Errorf("frontmatter = %q, want %q", fm, want)
	}
	// The body is verbatim from after the closing delimiter: the author's
	// leading blank line is part of the body, not trimmed away.
	if body := string(p.Body()); !strings.HasPrefix(body, "\n# Async Rust") {
		t.Errorf("body starts with %q", body[:20])
	}

	// Assets and parsed fields.
	if got := p.Assets(); len(got) != 2 || got[0] != "cover.jpeg" || got[1] != "diagram.png" {
		t.Errorf("Assets = %v", got)
	}
	if refs := p.ImageRefs(); len(refs) != 1 || refs[0] != "cover.jpeg" {
		t.Errorf("ImageRefs = %v", refs)
	}
	if p.Title() != "Async Rust" {
		t.Errorf("Title = %q", p.Title())
	}
	if want := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC); !p.Date().Equal(want) {
		t.Errorf("Date = %v, want %v", p.Date(), want)
	}
}

// TestNoFrontmatterPost verifies a bare body parses with no frontmatter and
// still round-trips.
func TestNoFrontmatterPost(t *testing.T) {
	dir := t.TempDir()
	body := "# Just a heading\n\nNo frontmatter here.\n"
	writePostFolder(t, dir, "bare", body)
	p, err := Read(filepath.Join(dir, "bare", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if p.HasFrontmatter() {
		t.Error("frontmatter falsely detected")
	}
	if string(p.Bytes()) != body || string(p.Body()) != body {
		t.Error("round trip changed a frontmatter-less file")
	}
}

// fixtureDirs creates ideas, wip and content directories with a mix of post
// shapes: folder posts, a loose published file, and a non-post directory.
func fixtureDirs(t *testing.T) (ideas, wip, content string) {
	t.Helper()
	base := t.TempDir()
	ideas = filepath.Join(base, "drafts", "ideas")
	wip = filepath.Join(base, "drafts", "wip")
	content = filepath.Join(base, "src", "content", "blog")
	for _, d := range []string{ideas, wip, content} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writePostFolder(t, ideas, "idea-one", "---\ntitle: One\n---\nA line.\n")
	writePostFolder(t, ideas, "idea-two", "---\ntitle: Two\n---\nAnother.\n")
	writePostFolder(t, wip, "half-written", "---\ntitle: Half\n---\nHalf a post.\n", "shot.png")
	writePostFolder(t, content, "old-published", "---\ntitle: Old\n---\nPublished folder post.\n")
	if err := os.WriteFile(filepath.Join(content, "loose-post.md"), []byte("---\ntitle: Loose\n---\nLoose file.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(content, "notes.txt"), []byte("not a post"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(content, "not-a-post"), 0o755); err != nil {
		t.Fatal(err)
	}
	return ideas, wip, content
}

// TestListShowsBothShapesWithoutRewriting verifies folder posts and loose
// markdown files appear, loose ones flagged, and listing rewrites nothing
// (task 3.2).
func TestListShowsBothShapesWithoutRewriting(t *testing.T) {
	ideas, wip, content := fixtureDirs(t)

	before := snapshotTree(content)

	l, err := List(ideas, wip, content)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(l.Ideas) != 2 {
		t.Errorf("ideas = %d posts, want 2", len(l.Ideas))
	}
	if len(l.WIP) != 1 || l.WIP[0].Name != "half-written" {
		t.Errorf("wip = %+v", l.WIP)
	}
	if len(l.Published) != 2 {
		t.Fatalf("published = %d posts, want 2 (folder + loose)", len(l.Published))
	}
	looseCount, folderCount := 0, 0
	for _, p := range l.Published {
		if p.Loose {
			looseCount++
			if p.Name != "loose-post" {
				t.Errorf("loose post name = %q", p.Name)
			}
		} else {
			folderCount++
		}
	}
	if looseCount != 1 || folderCount != 1 {
		t.Errorf("loose=%d folder=%d, want 1 and 1", looseCount, folderCount)
	}
	for _, p := range l.All() {
		if p.State == "" {
			t.Errorf("post %q has no state", p.Name)
		}
	}

	if after := snapshotTree(content); !mapsEqual(after, before) {
		t.Errorf("listing rewrote the content directory:\nbefore %v\nafter  %v", before, after)
	}
}

// mapsEqual compares two snapshot trees.
func mapsEqual(a, b map[string]string) bool {
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

// snapshotTree records path -> modtime -> size for a directory tree.
func snapshotTree(root string) map[string]string {
	out := map[string]string{}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			out[path] = info.ModTime().Format(time.RFC3339Nano) + "|" + itoa(info.Size())
		}
		return nil
	})
	return out
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// TestDeriveMetaMatchesFilesystem verifies derived metadata against a fixture
// with known timestamps (task 3.3).
func TestDeriveMetaMatchesFilesystem(t *testing.T) {
	ideas, _, _ := fixtureDirs(t)
	file := filepath.Join(ideas, "idea-one", "index.md")

	firstSeen := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	lastMod := time.Date(2026, 9, 15, 12, 30, 0, 0, time.UTC)
	if err := os.Chtimes(file, lastMod, lastMod); err != nil {
		t.Fatal(err)
	}

	p, err := Read(file)
	if err != nil {
		t.Fatal(err)
	}
	meta := DeriveMeta(p, firstSeen)
	if !meta.FirstSeen.Equal(firstSeen) {
		t.Errorf("FirstSeen = %v, want %v", meta.FirstSeen, firstSeen)
	}
	if !meta.LastModified.Equal(lastMod) {
		t.Errorf("LastModified = %v, want %v", meta.LastModified, lastMod)
	}
	body := string(p.Body())
	if meta.Size != len(body) {
		t.Errorf("Size = %d, want %d", meta.Size, len(body))
	}
	if meta.Title != "One" {
		t.Errorf("Title = %q", meta.Title)
	}

	// Zero firstSeen falls back to filesystem timestamps: the board stays
	// reconstructible with no cache at all (task 3.5's property).
	fallback := DeriveMeta(p, time.Time{})
	if fallback.FirstSeen.IsZero() {
		t.Error("FirstSeen fallback is zero")
	}
}
