package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/astrogui/astrogui/internal/cache"
	"github.com/astrogui/astrogui/internal/config"
	"github.com/astrogui/astrogui/internal/lifecycle"
	"github.com/astrogui/astrogui/internal/project"
	"github.com/astrogui/astrogui/internal/safe"
)

// fixture builds a project fixture with a live server over it.
type fixture struct {
	t       *testing.T
	base    string
	app     *App
	srv     *Server
	ts      *httptest.Server
	baseURL string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", filepath.Join(dir, "AppData"))

	base := t.TempDir()
	ideas := filepath.Join(base, "drafts", "ideas")
	wip := filepath.Join(base, "drafts", "wip")
	content := filepath.Join(base, "src", "content", "blog")
	for _, d := range []string{ideas, wip, content} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	guard, err := safe.New(ideas, wip, content)
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	derived, err := cache.Open()
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	f := &fixture{t: t, base: base}
	f.app = &App{
		Project:    base,
		Collection: project.Collection{Name: "blog", Dir: content},
		Config:     config.Project{}.FillDefaults(),
		Version:    "test",
		Guard:      guard,
		Cache:      derived,
		Manager:    lifecycle.New(ideas, wip, content, guard, derived, base),
	}
	f.srv, err = New(f.app, UI())
	if err != nil {
		t.Fatalf("server: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.ts = &httptest.Server{Listener: ln, Config: &http.Server{Handler: f.srv.Handler(ln)}}
	f.ts.Start()
	f.baseURL = f.ts.URL
	t.Cleanup(f.ts.Close)
	return f
}

// do performs a token-carrying API request.
func (f *fixture) do(method, path string, body any) (*http.Response, map[string]any) {
	f.t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, f.baseURL+path, reader)
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set("X-AstroGUI-Token", f.srv.Token())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := f.ts.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	out := map[string]any{}
	json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	return res, out
}

func (f *fixture) createIdea(line string) string {
	f.t.Helper()
	res, body := f.do("POST", "/api/collections/blog/entries", map[string]any{"line": line})
	if res.StatusCode != 201 {
		f.t.Fatalf("create idea: %d %v", res.StatusCode, body)
	}
	return body["name"].(string)
}

// TestCollectionAddressableByName verifies the API is rooted at
// /api/collections/:name and a wrong name is not silently served (5.1).
func TestCollectionAddressableByName(t *testing.T) {
	f := newFixture(t)

	res, body := f.do("GET", "/api/collections", nil)
	if res.StatusCode != 200 {
		t.Fatalf("collections: %d", res.StatusCode)
	}
	if fmt.Sprint(body["collections"]) != "[blog]" {
		t.Errorf("collections = %v", body["collections"])
	}

	res, _ = f.do("GET", "/api/collections/blog/board", nil)
	if res.StatusCode != 200 {
		t.Fatalf("board by name: %d", res.StatusCode)
	}
	res, body = f.do("GET", "/api/collections/otherthings/board", nil)
	if res.StatusCode != 404 {
		t.Errorf("unknown collection: %d, want 404", res.StatusCode)
	}
	if !strings.Contains(fmt.Sprint(body["error"]), "otherthings") {
		t.Errorf("error should name the unknown collection: %v", body["error"])
	}
}

// TestListenerIsLoopbackOnly verifies the listener binds loopback only, so
// the port is unreachable from other machines (5.2).
func TestListenerIsLoopbackOnly(t *testing.T) {
	ln, conflict, err := Listen(0) // port 0: any free port
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if conflict != "" {
		t.Errorf("unexpected conflict for a free port: %s", conflict)
	}
	host, _, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ip := net.ParseIP(host)
	if !ip.IsLoopback() {
		t.Errorf("listener bound to %s, want a loopback address", host)
	}
}

// TestHostHeaderValidated verifies a rebinding-style Host header is refused
// before routing (5.3).
func TestHostHeaderValidated(t *testing.T) {
	f := newFixture(t)

	req, _ := http.NewRequest("GET", f.baseURL+"/api/health", nil)
	req.Host = "evil.example.com" // DNS rebinding: attacker domain resolving here
	req.Header.Set("X-AstroGUI-Token", f.srv.Token())
	res, err := f.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("rebinding host accepted: %d", res.StatusCode)
	}

	// The literal origin is accepted.
	res2, _ := f.do("GET", "/api/health", nil)
	if res2.StatusCode != 200 {
		t.Errorf("own origin refused: %d", res2.StatusCode)
	}
}

// TestTokenRequiredOnEveryAPICall verifies an API call without the session
// token is refused with no filesystem effect (5.4, 5.7).
func TestTokenRequiredOnEveryAPICall(t *testing.T) {
	f := newFixture(t)
	before := countFiles(f.base)

	req, _ := http.NewRequest("POST", f.baseURL+"/api/collections/blog/entries", strings.NewReader(`{"line":"no token"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := f.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("request without token: %d, want 401", res.StatusCode)
	}

	// Also with a wrong token.
	req2, _ := http.NewRequest("POST", f.baseURL+"/api/collections/blog/entries", strings.NewReader(`{"line":"bad token"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-AstroGUI-Token", "000000000000000000000000000000000000000000000000")
	res2, _ := f.ts.Client().Do(req2)
	res2.Body.Close()
	if res2.StatusCode != http.StatusUnauthorized {
		t.Errorf("request with wrong token: %d, want 401", res2.StatusCode)
	}

	if after := countFiles(f.base); after != before {
		t.Errorf("refused request had a filesystem effect: before %d after %d", before, after)
	}

	// The announced URL carries the token in the fragment, not the query.
	url := f.srv.Announce("127.0.0.1:41990")
	if !strings.Contains(url, "#token=") || strings.Contains(url, "?token=") {
		t.Errorf("announced URL misplaces the token: %s", url)
	}
}

func countFiles(root string) int {
	n := 0
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// TestAssetConfinement verifies asset serving is confined to the post's own
// directory by real path: traversal and symlink escapes refused, legitimate
// images served (5.5).
func TestAssetConfinement(t *testing.T) {
	f := newFixture(t)
	name := f.createIdea("an idea with an image")
	postDir := filepath.Join(f.base, "drafts", "ideas", name)
	if err := os.WriteFile(filepath.Join(postDir, "cover.png"), []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Legitimate image: served.
	res, body := f.do("GET", "/api/collections/blog/entries/"+name+"/assets/cover.png", nil)
	if res.StatusCode != 200 {
		t.Fatalf("legitimate image: %d %v", res.StatusCode, body)
	}

	// Traversal out of the post directory: refused.
	for _, evil := range []string{
		"..%2F..%2F..%2Fpackage.json",
		"..%5C..%5Cpackage.json",
	} {
		res, _ = f.do("GET", "/api/collections/blog/entries/"+name+"/assets/"+evil, nil)
		if res.StatusCode == 200 {
			t.Errorf("traversal %s served a file", evil)
		}
	}

	// Symlink escape: refused by resolved real path.
	secret := filepath.Join(f.base, "outside-secret.txt")
	if err := os.WriteFile(secret, []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(postDir, "escape.png")); err != nil {
		t.Fatal(err)
	}
	res, body = f.do("GET", "/api/collections/blog/entries/"+name+"/assets/escape.png", nil)
	if res.StatusCode != http.StatusForbidden && res.StatusCode != http.StatusNotFound {
		t.Errorf("symlink escape served: %d %v", res.StatusCode, body)
	}
}

// TestPortConflictSelectsFreePort verifies a busy preferred port is reported,
// a free port is selected and announced, and the first listener is untouched
// (5.6).
func TestPortConflictSelectsFreePort(t *testing.T) {
	owner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	port := owner.Addr().(*net.TCPAddr).Port

	ln, conflict, err := Listen(port)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if conflict == "" {
		t.Error("port conflict not reported")
	}
	if !strings.Contains(conflict, fmt.Sprint(port)) {
		t.Errorf("conflict %q does not name the busy port", conflict)
	}
	newPort := ln.Addr().(*net.TCPAddr).Port
	if newPort == port {
		t.Error("took over the busy port instead of selecting a free one")
	}
	// The original owner still owns its port: it was never closed and a new
	// bind of the same port still fails.
	if probe, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
		probe.Close()
		t.Error("the busy port was taken over: a second bind succeeded")
	}
}

// TestBoardAndEditorFlow exercises the API-level board and editor behavior:
// restart-identical boards, stalled markings, moves, saves, conflicts.
func TestBoardAndEditorFlow(t *testing.T) {
	f := newFixture(t)
	name := f.createIdea("async rust idea")
	other := f.createIdea("second idea")

	res, body := f.do("GET", "/api/collections/blog/board", nil)
	if res.StatusCode != 200 {
		t.Fatalf("board: %d", res.StatusCode)
	}
	cols := body["columns"].(map[string]any)
	ideas := cols["ideas"].([]any)
	if len(ideas) != 2 {
		t.Fatalf("ideas column = %v", ideas)
	}

	// Restart identity: a fresh listing equals the previous one (7.1).
	res2, body2 := f.do("GET", "/api/collections/blog/board", nil)
	if fmt.Sprint(body2["columns"]) != fmt.Sprint(cols) || res2.StatusCode != 200 {
		t.Error("board differs between reads without a filesystem change")
	}

	// Signals on cards (7.2): the created idea carries meta.
	card := ideas[0].(map[string]any)
	meta := card["meta"].(map[string]any)
	if meta["size"].(float64) <= 0 {
		t.Errorf("card size = %v", meta["size"])
	}
	if _, ok := meta["firstSeen"]; !ok {
		t.Error("card lacks firstSeen")
	}

	// Stalled marking recomputes with the threshold (7.3): a year-old mtime
	// under a 30-day threshold is stalled.
	old := time.Now().Add(-365 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(f.base, "drafts", "ideas", name, "index.md"), old, old); err != nil {
		t.Fatal(err)
	}
	_, body3 := f.do("GET", "/api/collections/blog/board", nil)
	for _, c := range body3["columns"].(map[string]any)["ideas"].([]any) {
		cm := c.(map[string]any)
		if cm["name"] == name {
			if cm["stalled"] != true {
				t.Errorf("year-old post not marked stalled")
			}
		} else if cm["name"] == other && cm["stalled"] == true {
			t.Errorf("fresh post marked stalled")
		}
	}

	// Entry detail feeds the editor.
	res, entry := f.do("GET", "/api/collections/blog/entries/"+name, nil)
	if res.StatusCode != 200 || entry["body"] == "" {
		t.Fatalf("entry: %d %v", res.StatusCode, entry)
	}
	modTime := entry["modTime"].(string)

	// Save without a change: file untouched (8.3, 8.7).
	before := statMod(t, filepath.Join(f.base, "drafts", "ideas", name, "index.md"))
	res, out := f.do("PUT", "/api/collections/blog/entries/"+name+"/body", map[string]any{
		"body":    entry["body"],
		"modTime": modTime,
	})
	if res.StatusCode != 200 {
		t.Fatalf("no-change save: %d %v", res.StatusCode, out)
	}
	if got := statMod(t, filepath.Join(f.base, "drafts", "ideas", name, "index.md")); !got.Equal(before) {
		t.Error("no-change save rewrote the file")
	}

	// Real body change: saved, new modTime returned.
	res, out = f.do("PUT", "/api/collections/blog/entries/"+name+"/body", map[string]any{
		"body":    "---\ntitle: T\ndate: 2026-01-01\n---\n\nBody now real.\n",
		"modTime": out["modTime"],
	})
	if res.StatusCode != 200 {
		t.Fatalf("body save: %d %v", res.StatusCode, out)
	}

	// Concurrent external change: save refused, on-disk version preserved
	// (8.10).
	external := time.Now().Add(2 * time.Second)
	file := filepath.Join(f.base, "drafts", "ideas", name, "index.md")
	if err := os.Chtimes(file, external, external); err != nil {
		t.Fatal(err)
	}
	onDiskBefore, _ := os.ReadFile(file)
	res, out = f.do("PUT", "/api/collections/blog/entries/"+name+"/body", map[string]any{
		"body":    "stale editor buffer\n",
		"modTime": out["modTime"],
	})
	if res.StatusCode != 409 {
		t.Fatalf("conflicting save: %d %v, want 409", res.StatusCode, out)
	}
	onDiskAfter, _ := os.ReadFile(file)
	if !bytes.Equal(onDiskBefore, onDiskAfter) {
		t.Error("conflict overwrote the on-disk version")
	}
}

// TestStructuredFieldEditPreservesComments verifies the API path preserves
// comments and unknown fields, writes only changes (8.5, 8.7).
func TestStructuredFieldEditPreservesComments(t *testing.T) {
	f := newFixture(t)
	name := f.createIdea("fields idea")
	file := filepath.Join(f.base, "drafts", "ideas", name, "index.md")
	original := "---\nslug: " + name + "\ntitle: Old   # keep me\ncustom: keep\n---\n\nBody.\n"
	if err := os.WriteFile(file, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	_, entry := f.do("GET", "/api/collections/blog/entries/"+name, nil)

	// No field changed: no rewrite at all.
	before := statMod(t, file)
	res, out := f.do("PUT", "/api/collections/blog/entries/"+name+"/frontmatter", map[string]any{
		"fields":  map[string]any{"title": "Old"},
		"modTime": entry["modTime"],
	})
	if res.StatusCode != 200 || out["changed"] == true {
		t.Fatalf("unchanged fields should not write: %d %v", res.StatusCode, out)
	}
	if statMod(t, file).Equal(before) != true {
		t.Error("unchanged fields rewrote the file")
	}

	// Title change: comment and custom field survive.
	res, out = f.do("PUT", "/api/collections/blog/entries/"+name+"/frontmatter", map[string]any{
		"fields":  map[string]any{"title": "New"},
		"modTime": entry["modTime"],
	})
	if res.StatusCode != 200 || out["changed"] != true {
		t.Fatalf("field save: %d %v", res.StatusCode, out)
	}
	data, _ := os.ReadFile(file)
	got := string(data)
	for _, want := range []string{"# keep me", "custom: keep", "title: New", "Body."} {
		if !strings.Contains(got, want) {
			t.Errorf("field edit lost %q:\n%s", want, got)
		}
	}
}

// TestRawEditRoundTrip verifies a raw frontmatter edit is reflected in the
// post and on the board (8.6) — including on read-only loose posts, which
// stay loose files in place (9.3).
func TestRawEditRoundTrip(t *testing.T) {
	f := newFixture(t)
	loose := filepath.Join(f.base, "src", "content", "blog", "loose-post.md")
	if err := os.WriteFile(loose, []byte("---\ntitle: Loose\n---\nOld body.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, entry := f.do("GET", "/api/collections/blog/entries/loose-post", nil)
	if entry["readOnly"] != true {
		t.Fatalf("loose post not read-only: %v", entry)
	}
	res, out := f.do("PUT", "/api/collections/blog/entries/loose-post/raw", map[string]any{
		"content": "---\ntitle: Loose Edited\n---\nNew body.\n",
		"modTime": entry["modTime"],
	})
	if res.StatusCode != 200 {
		t.Fatalf("raw save on loose post: %d %v", res.StatusCode, out)
	}
	data, _ := os.ReadFile(loose)
	if !strings.Contains(string(data), "Loose Edited") {
		t.Errorf("raw edit not reflected: %s", data)
	}

	// The structured body endpoint refuses loose posts: their editing path
	// is the raw text view (spec: "offers editing only through the raw text
	// view").
	_, entry2 := f.do("GET", "/api/collections/blog/entries/loose-post", nil)
	res2, out2 := f.do("PUT", "/api/collections/blog/entries/loose-post/body", map[string]any{
		"body":    "should be refused\n",
		"modTime": entry2["modTime"],
	})
	if res2.StatusCode != 403 {
		t.Errorf("loose body save: %d %v, want 403", res2.StatusCode, out2)
	}
	if _, err := os.Stat(filepath.Join(f.base, "drafts", "ideas", "loose-post")); err == nil {
		t.Error("refused body save created a folder")
	}
	if _, err := os.Stat(filepath.Join(f.base, "src", "content", "blog", "loose-post")); err == nil {
		t.Error("loose post was converted to a folder")
	}

	// And the board card shows the edited title (8.6).
	_, board := f.do("GET", "/api/collections/blog/board", nil)
	found := false
	for _, c := range board["columns"].(map[string]any)["published"].([]any) {
		if c.(map[string]any)["title"] == "Loose Edited" {
			found = true
		}
	}
	if !found {
		t.Error("board does not reflect the raw edit")
	}
}

// TestMoveEndpointAndPublishGate verifies drag-and-drop's backend: a move
// request runs the publish gate when the target is published (7.4, 4.4).
func TestMoveEndpointAndPublishGate(t *testing.T) {
	f := newFixture(t)
	name := f.createIdea("to be published")

	// Failing gate: no title/date.
	res, out := f.do("POST", "/api/collections/blog/entries/"+name+"/move", map[string]any{"to": "published"})
	if res.StatusCode != 422 {
		t.Fatalf("publish without metadata: %d %v", res.StatusCode, out)
	}
	if _, ok := out["problems"]; !ok {
		t.Error("gate failure does not carry named problems")
	}

	// Fix metadata, then the same request publishes.
	file := filepath.Join(f.base, "drafts", "ideas", name, "index.md")
	if err := os.WriteFile(file, []byte("---\nslug: "+name+"\ntitle: Ready\ndate: 2026-01-01\n---\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, out = f.do("POST", "/api/collections/blog/entries/"+name+"/move", map[string]any{"to": "published"})
	if res.StatusCode != 200 {
		t.Fatalf("publish: %d %v", res.StatusCode, out)
	}
	if _, err := os.Stat(filepath.Join(f.base, "src", "content", "blog", name, "index.md")); err != nil {
		t.Error("post not in the published directory")
	}
}

// TestUploadAsset verifies a pasted image lands in the post's own directory
// (8.8, 8.9 server side).
func TestUploadAsset(t *testing.T) {
	f := newFixture(t)
	name := f.createIdea("image holder")

	res, out := f.do("POST", "/api/collections/blog/entries/"+name+"/assets", map[string]any{
		"name": "diagram.png",
		"data": "aVNVTS1EQVRB",
	})
	if res.StatusCode != 201 {
		t.Fatalf("upload: %d %v", res.StatusCode, out)
	}
	if out["name"] != "diagram.png" {
		t.Errorf("asset name = %v", out["name"])
	}
	data, err := os.ReadFile(filepath.Join(f.base, "drafts", "ideas", name, "diagram.png"))
	if err != nil || len(data) == 0 {
		t.Errorf("asset not written into the post's directory: %v", err)
	}

	// A nameless paste gets a useful name instead of failing (8.9).
	res, out = f.do("POST", "/api/collections/blog/entries/"+name+"/assets", map[string]any{
		"name": "",
		"data": "aVNVTS1EQVRB",
	})
	if res.StatusCode != 201 {
		t.Fatalf("nameless upload: %d %v", res.StatusCode, out)
	}
	if !strings.HasPrefix(out["name"].(string), "pasted-") {
		t.Errorf("nameless upload named %v", out["name"])
	}
}

// TestFunnelEndpoint verifies current counts and recorded progression.
func TestFunnelEndpoint(t *testing.T) {
	f := newFixture(t)
	name := f.createIdea("funnel idea")
	f.do("POST", "/api/collections/blog/entries/"+name+"/move", map[string]any{"to": "wip"})

	res, funnel := f.do("GET", "/api/collections/blog/funnel", nil)
	if res.StatusCode != 200 {
		t.Fatalf("funnel: %d", res.StatusCode)
	}
	counts := funnel["counts"].(map[string]any)
	if counts["wip"].(float64) != 1 {
		t.Errorf("wip count = %v", counts["wip"])
	}
	advanced := funnel["advanced"].(map[string]any)
	if advanced["wip"].(float64) != 1 {
		t.Errorf("advanced into wip = %v", advanced["wip"])
	}
}

func statMod(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime()
}
