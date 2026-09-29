package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// trickyBody exercises the syntaxes that most easily die in a document round
// trip: code-fence metadata, raw HTML, component tags, footnotes, tables,
// and trailing-space line breaks.
const trickyBody = "# Tricky\n\n" +
	"```js {filename=main.js showLineNumbers}\nconsole.log('hi')\n```\n\n" +
	"<div class=\"raw\">raw HTML</div>\n\n" +
	"<Counter client:load />\n\n" +
	"A note[^n].\n\n" +
	"| a | b |\n| -- | -- |\n| 1 | 2 |\n\n" +
	"line one  \nline two\n\n[^n]: the note\n"

// TestTrickySyntaxByteIdenticalThroughSave verifies open → edit → save leaves
// the author's syntax exactly as written (task 8.2).
func TestTrickySyntaxByteIdenticalThroughSave(t *testing.T) {
	f := newFixture(t)
	name := f.createIdea("tricky syntax")

	// Stage 1: save with the tricky body via the API.
	_, entry := f.do("GET", "/api/collections/blog/entries/"+name, nil)
	res, out := f.do("PUT", "/api/collections/blog/entries/"+name+"/body", map[string]any{
		"body":    trickyBody,
		"modTime": entry["modTime"],
	})
	if res.StatusCode != 200 {
		t.Fatalf("save tricky body: %d %v", res.StatusCode, out)
	}

	// Stage 2: re-open, re-save the same body — byte-identical, untouched.
	_, entry2 := f.do("GET", "/api/collections/blog/entries/"+name, nil)
	file := filepath.Join(f.base, "drafts", "ideas", name, "index.md")
	before, _ := os.ReadFile(file)
	res, out = f.do("PUT", "/api/collections/blog/entries/"+name+"/body", map[string]any{
		"body":    entry2["body"].(string),
		"modTime": entry2["modTime"],
	})
	if res.StatusCode != 200 {
		t.Fatalf("re-save: %d %v", res.StatusCode, out)
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Errorf("file changed across an identical save:\nbefore %q\nafter  %q", before, after)
	}

	// The stored bytes contain every tricky construct verbatim.
	for _, want := range []string{
		"```js {filename=main.js showLineNumbers}",
		`<div class="raw">raw HTML</div>`,
		"<Counter client:load />",
		"[^n]: the note",
		"| a | b |",
		"line one  \nline two",
	} {
		if !strings.Contains(string(after), want) {
			t.Errorf("stored body lost %q", want)
		}
	}

	// Stage 3: an edit appends text without normalizing the rest.
	res, _ = f.do("PUT", "/api/collections/blog/entries/"+name+"/body", map[string]any{
		"body":    trickyBody + "\nAdded later.\n",
		"modTime": out["modTime"],
	})
	if res.StatusCode != 200 {
		t.Fatal("edit save failed")
	}
	edited, _ := os.ReadFile(file)
	if !strings.Contains(string(edited), "line one  \nline two") {
		t.Error("edit normalized the trailing-space line break")
	}
	if !strings.HasSuffix(string(edited), "Added later.\n") {
		t.Errorf("edit did not store the new text verbatim: %q", edited[len(edited)-40:])
	}
}

// TestUIServesCSPHeader verifies the interface is served with a
// Content-Security-Policy that forbids inline script, the server-side layer
// that keeps post content from executing even if a sanitizer miss occurred
// (task 8.11).
func TestUIServesCSPHeader(t *testing.T) {
	f := newFixture(t)
	res, err := f.ts.Client().Get(f.baseURL + "/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	csp := res.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("interface served without Content-Security-Policy")
	}
	for _, want := range []string{"script-src 'self'", "object-src 'none'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q: %s", want, csp)
		}
	}
	// Inline scripts are forbidden: no 'unsafe-inline' for scripts.
	if strings.Contains(csp, "script-src 'unsafe-inline'") || strings.Contains(csp, "script-src *") {
		t.Errorf("CSP would allow inline script: %s", csp)
	}
	// Referrer policy keeps the fragment-delivered token private.
	if got := res.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q", got)
	}
}

// TestUIAssetsServed verifies the embedded interface files arrive.
func TestUIAssetsServed(t *testing.T) {
	f := newFixture(t)
	for _, path := range []string{"/index.html", "/app.js", "/style.css", "/markdown.js"} {
		res, err := f.ts.Client().Get(f.baseURL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
	}
}
