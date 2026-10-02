package project

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeAstroProject creates a minimal Astro project fixture.
func makeAstroProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("astro.config.mjs", "// @ts-check\nimport { defineConfig } from 'astro/config';\nexport default defineConfig({});\n")
	write("package.json", `{"name":"fixture-blog","dependencies":{"astro":"^7.0.0"}}`)
	write(filepath.Join("src", "content.config.ts"), `
import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';

const blog = defineCollection({
  loader: glob({ pattern: '**/*.{md,mdx}', base: './src/content/blog' }),
});

export const collections = { blog };
`)
	write(filepath.Join("src", "content", "blog", ".gitkeep"), "")
	return root
}

// TestFindFromNestedSubdirectory verifies detection walks up (task 2.1).
func TestFindFromNestedSubdirectory(t *testing.T) {
	root := makeAstroProject(t)
	deep := filepath.Join(root, "src", "components", "nested")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := Find(deep)
	if err != nil {
		t.Fatalf("find from subdirectory: %v", err)
	}
	if p.Root != root {
		t.Errorf("Root = %q, want %q", p.Root, root)
	}
}

// TestFindFailsOutsideProjectWithNamedDirectory verifies the failure names
// the directory searched (task 2.1).
func TestFindFailsOutsideProjectWithNamedDirectory(t *testing.T) {
	empty := t.TempDir()
	_, err := Find(empty)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), empty) {
		t.Errorf("error %q does not name the searched directory %q", err, empty)
	}
}

// TestDetectConventionalLayout verifies the glob() base is detected (2.2).
func TestDetectConventionalLayout(t *testing.T) {
	root := makeAstroProject(t)
	cols, err := DetectCollections(root)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(cols) != 1 {
		t.Fatalf("got %d collections (%+v), want 1", len(cols), cols)
	}
	if cols[0].Name != "blog" {
		t.Errorf("Name = %q, want blog", cols[0].Name)
	}
	want := filepath.Join(root, "src", "content", "blog")
	if cols[0].Dir != want {
		t.Errorf("Dir = %q, want %q", cols[0].Dir, want)
	}
}

// TestDetectMultipleCollections verifies several collections are all reported
// and none is silently chosen (task 2.3).
func TestDetectMultipleCollections(t *testing.T) {
	root := makeAstroProject(t)
	config := `
import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';

const blog = defineCollection({ loader: glob({ pattern: '**/*.md', base: './src/content/blog' }) });
const notes = defineCollection({ loader: glob({ pattern: '**/*.md', base: './src/content/notes' }) });

export const collections = { blog, notes };
`
	if err := os.WriteFile(filepath.Join(root, "src", "content.config.ts"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	cols, err := DetectCollections(root)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(cols) != 2 {
		t.Fatalf("got %d collections (%+v), want 2", len(cols), cols)
	}
}

// TestChooseCollectionRequiresExplicitAnswer verifies no selection happens
// without an answer, and an explicit answer selects exactly it (task 2.3).
func TestChooseCollectionRequiresExplicitAnswer(t *testing.T) {
	cols := []Collection{
		{Name: "blog", Dir: "/p/src/content/blog"},
		{Name: "notes", Dir: "/p/src/content/notes"},
	}

	for _, stdin := range []string{"", "\n", "  \n", "yes\n", "3\n", "0\n"} {
		var out bytes.Buffer
		chosen, err := ChooseCollection(cols, strings.NewReader(stdin), &out)
		if err == nil {
			t.Errorf("stdin %q: selected %+v without an explicit valid answer", stdin, chosen)
		}
	}

	var out bytes.Buffer
	chosen, err := ChooseCollection(cols, strings.NewReader("2\n"), &out)
	if err != nil {
		t.Fatalf("explicit choice: %v", err)
	}
	if chosen.Name != "notes" {
		t.Errorf("chose %+v, want notes", chosen)
	}
	if !strings.Contains(out.String(), "blog") || !strings.Contains(out.String(), "notes") {
		t.Errorf("prompt %q does not list the collections", out.String())
	}
}

// TestSingleCollectionSkipsPrompt verifies a lone collection resolves without
// configuration or prompting (task 2.2).
func TestSingleCollectionSkipsPrompt(t *testing.T) {
	cols := []Collection{{Name: "blog", Dir: "/p/src/content/blog"}}
	var out bytes.Buffer
	chosen, err := ChooseCollection(cols, strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("choose: %v", err)
	}
	if chosen.Name != "blog" || out.Len() != 0 {
		t.Errorf("single collection should resolve silently, got prompt %q", out.String())
	}
}

func TestResolveCollectionHonorsValidPreferenceWithoutPrompt(t *testing.T) {
	cols := []Collection{
		{Name: "blog", Dir: "/p/src/content/blog"},
		{Name: "notes", Dir: "/p/src/content/notes"},
	}
	var out bytes.Buffer
	chosen, configured, err := ResolveCollection(cols, "notes", strings.NewReader(""), &out)
	if err != nil {
		t.Fatal(err)
	}
	if !configured || chosen.Name != "notes" || out.Len() != 0 {
		t.Fatalf("ResolveCollection = %+v, configured=%v, prompt=%q", chosen, configured, out.String())
	}
}

func TestResolveCollectionPromptsForMissingOrStalePreference(t *testing.T) {
	cols := []Collection{
		{Name: "blog", Dir: "/p/src/content/blog"},
		{Name: "notes", Dir: "/p/src/content/notes"},
	}
	for _, preferred := range []string{"", "removed"} {
		var out bytes.Buffer
		chosen, configured, err := ResolveCollection(cols, preferred, strings.NewReader("2\n"), &out)
		if err != nil {
			t.Fatalf("preference %q: %v", preferred, err)
		}
		if configured || chosen.Name != "notes" || !strings.Contains(out.String(), "Which collection") {
			t.Errorf("preference %q: got %+v configured=%v prompt=%q", preferred, chosen, configured, out.String())
		}
	}
}

// TestUserOverrideTakesPrecedence verifies a configured content directory
// wins over the detected one (task 2.2).
func TestUserOverrideTakesPrecedence(t *testing.T) {
	root := makeAstroProject(t)
	cols, err := DetectCollections(root)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	got := Managed(root, filepath.Join("custom", "posts"), cols[0])
	if got.Dir != filepath.Join(root, "custom", "posts") {
		t.Errorf("Dir = %q, want the override", got.Dir)
	}
	if got.Name != "blog" {
		t.Errorf("Name = %q, want the detected name blog", got.Name)
	}

	// Absolute override, no detection at all.
	solo := Managed(root, "/srv/content", Collection{})
	if solo.Dir != filepath.FromSlash("/srv/content") || solo.Name != "content" {
		t.Errorf("absolute override: got %+v", solo)
	}

	// No override: the detected collection passes through unchanged.
	kept := Managed(root, "", cols[0])
	if kept.Dir != cols[0].Dir || kept.Source != cols[0].Source {
		t.Errorf("no override should keep detection: %+v", kept)
	}
}

// TestLegacyLayoutFallback verifies src/content/<name>/ with markdown files
// is detected when no collection config declares globs.
func TestLegacyLayoutFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "astro.config.mjs"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(root, "astro.config.mjs"))
	if err := os.WriteFile(filepath.Join(root, "astro.config.mjs"), []byte("export default {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	postDir := filepath.Join(root, "src", "content", "letters")
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(postDir, "hello.md"), []byte("---\ntitle: Hi\n---\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}
	cols, err := DetectCollections(root)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(cols) != 1 || cols[0].Name != "letters" {
		t.Fatalf("got %+v, want letters", cols)
	}
}

// TestDetectLegacyLayoutWithMdxFolder verifies the legacy directory-per-
// collection fallback recognizes a collection whose only content is a
// folder post with an index.mdx entry file.
func TestDetectLegacyLayoutWithMdxFolder(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "astro.config.mjs"), []byte("// astro\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"astro":"^7.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	postDir := filepath.Join(root, "src", "content", "blog", "mdx-post")
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(postDir, "index.mdx"), []byte("---\ntitle: MDX\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}

	cols, err := DetectCollections(root)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(cols) != 1 || cols[0].Name != "blog" {
		t.Fatalf("got %+v, want one blog collection", cols)
	}
}
