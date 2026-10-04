package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/astrogui/astrogui/internal/cache"
	"github.com/astrogui/astrogui/internal/config"
	"github.com/astrogui/astrogui/internal/lifecycle"
	"github.com/astrogui/astrogui/internal/project"
	"github.com/astrogui/astrogui/internal/safe"
	"github.com/astrogui/astrogui/internal/watch"
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
	trash := filepath.Join(base, "drafts", "trash")
	guard, err := safe.New(ideas, wip, content, trash)
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	t.Cleanup(func() { _ = guard.Close() })
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
		Manager:    lifecycle.New(ideas, wip, content, trash, guard, derived, base),
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
	if out["currentModTime"] == nil || out["current"] == nil {
		t.Fatalf("conflict did not include the reviewed disk version and modTime: %v", out)
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
	res, out = f.do("POST", "/api/collections/blog/entries/"+name+"/assets", map[string]any{
		"name": "diagram.png",
		"data": "aVNVTS1EQVRB",
	})
	if res.StatusCode != 201 || out["name"] != "diagram-2.png" {
		t.Errorf("duplicate upload = %d %v, want unique diagram-2.png", res.StatusCode, out)
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

	// A dangling symlink must not be treated as an unused name and followed
	// outside the managed post directory.
	outside := filepath.Join(f.base, "outside", "diagram.png")
	if err := os.MkdirAll(filepath.Dir(outside), 0o755); err != nil {
		t.Fatal(err)
	}
	postDir := filepath.Join(f.base, "drafts", "ideas", name)
	if err := os.Symlink(outside, filepath.Join(postDir, "escape.png")); err != nil {
		t.Fatal(err)
	}
	res, _ = f.do("POST", "/api/collections/blog/entries/"+name+"/assets", map[string]any{
		"name": "escape.png",
		"data": "aVNVTS1EQVRB",
	})
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("upload through dangling symlink status = %d, want 403", res.StatusCode)
	}
	if _, err := os.Stat(outside); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("upload created outside target: %v", err)
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

func TestGitStatusPreviewEndpoint(t *testing.T) {
	f := newFixture(t)
	if out, err := exec.Command("git", "init", f.base).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	name := "new file\nname.md"
	if runtime.GOOS == "windows" {
		// Newlines are not legal in Windows filenames; use a plain name and
		// keep the newline edge case to the in-memory parser tests.
		name = "new file name.md"
	}
	if err := os.WriteFile(filepath.Join(f.base, name), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, body := f.do("GET", "/api/collections/blog/git-status", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("git status: %d %v", res.StatusCode, body)
	}
	if body["clean"] != false {
		t.Fatalf("working tree unexpectedly clean: %v", body)
	}
	changes, ok := body["changes"].([]any)
	if !ok || len(changes) != 1 {
		t.Fatalf("changes = %v", body["changes"])
	}
	change := changes[0].(map[string]any)
	if change["status"] != "??" || change["path"] != name {
		t.Fatalf("path/status not preserved through API: %+v", change)
	}
}

func TestCommitFailureOutputSurvivesAPIResponse(t *testing.T) {
	f := newFixture(t)
	if out, err := exec.Command("git", "init", f.base).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", f.base, "config", "user.name", "test").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", f.base, "config", "user.email", "test@example.test").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(f.base, "newfile.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(f.base, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho commit-hook-output >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, body := f.do("POST", "/api/collections/blog/commit", map[string]any{"message": "will fail"})
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("commit response = %d %v, want 500", res.StatusCode, body)
	}
	if body["command"] != "git commit -m will fail" || !strings.Contains(fmt.Sprint(body["output"]), "commit-hook-output") {
		t.Fatalf("commit failure output lost: %v", body)
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

// TestModTimeOrNullSurvivesMissingFile is the regression for the post-save
// stat: a file removed or made unreadable the instant after a successful
// write must yield a null time, never a nil-dereference panic.
func TestModTimeOrNullSurvivesMissingFile(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("modTimeOrNull panicked: %v", r)
		}
	}()
	if got := modTimeOrNull(filepath.Join(t.TempDir(), "gone.md")); got != nil {
		t.Errorf("missing file modTime = %v, want nil", got)
	}
	live := filepath.Join(t.TempDir(), "live.md")
	if err := os.WriteFile(live, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if modTimeOrNull(live) == nil {
		t.Error("existing file modTime = nil, want the modification time")
	}
}

// TestEntryLookupErrorsAreHonest verifies a post that does not exist is 404
// while a post that exists but cannot be read is a 500 naming the read
// failure — an internal error is never disguised as absence.
func TestEntryLookupErrorsAreHonest(t *testing.T) {
	f := newFixture(t)

	// Absent post: not found.
	res, body := f.do("GET", "/api/collections/blog/entries/no-such-post", nil)
	if res.StatusCode != 404 {
		t.Fatalf("absent post = %d %v, want 404", res.StatusCode, body)
	}

	if runtime.GOOS == "windows" {
		t.Skip("permission-based unreadability is not available on windows")
	}
	// Existing post whose file cannot be read: server error naming the read.
	name := f.createIdea("unreadable idea")
	index := filepath.Join(f.base, "drafts", "ideas", name, "index.md")
	if err := os.Chmod(index, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(index, 0o644) })
	res, body = f.do("GET", "/api/collections/blog/entries/"+name, nil)
	if res.StatusCode != 500 {
		t.Fatalf("unreadable post = %d %v, want 500", res.StatusCode, body)
	}
	if errText := fmt.Sprint(body["error"]); !strings.Contains(errText, "reading") {
		t.Errorf("500 does not name the read failure: %v", errText)
	}
}

// TestLoosePostExposesNoAssetNamespace verifies a loose file — a bare
// markdown file with no directory of its own — cannot serve files from the
// collection directory through its asset path, mirroring the upload refusal.
func TestLoosePostExposesNoAssetNamespace(t *testing.T) {
	f := newFixture(t)
	content := filepath.Join(f.base, "src", "content", "blog")
	if err := os.WriteFile(filepath.Join(content, "loose.md"), []byte("---\ntitle: Loose\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A real file in the collection directory, outside any post's folder.
	if err := os.WriteFile(filepath.Join(content, "neighbor.png"), []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, body := f.do("GET", "/api/collections/blog/entries/loose/assets/neighbor.png", nil)
	if res.StatusCode != 404 {
		t.Fatalf("loose-post asset request = %d %v, want 404", res.StatusCode, body)
	}
}

// TestAssetResponsesAreInert verifies an asset served as a top-level
// document cannot execute in the tool's origin: sandbox policy and no type
// sniffing on every asset response.
func TestAssetResponsesAreInert(t *testing.T) {
	f := newFixture(t)
	name := f.createIdea("an idea with a diagram")
	postDir := filepath.Join(f.base, "drafts", "ideas", name)
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`
	if err := os.WriteFile(filepath.Join(postDir, "diagram.svg"), []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	res, body := f.do("GET", "/api/collections/blog/entries/"+name+"/assets/diagram.svg", nil)
	if res.StatusCode != 200 {
		t.Fatalf("svg asset = %d %v, want 200", res.StatusCode, body)
	}
	if got := res.Header.Get("Content-Security-Policy"); got != "default-src 'none'" {
		t.Errorf("Content-Security-Policy = %q, want default-src 'none'", got)
	}
	if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := res.Header.Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", got)
	}
}

// TestUnicodeTitlePinsSlugEndToEnd verifies a post created from a non-ASCII
// title gets a folder name carrying the title's letters and a slug pinned to
// that name, so the published URL depends only on the post's identity.
func TestUnicodeTitlePinsSlugEndToEnd(t *testing.T) {
	f := newFixture(t)
	res, body := f.do("POST", "/api/collections/blog/entries", map[string]any{"title": "こんにちは世界"})
	if res.StatusCode != 201 {
		t.Fatalf("create = %d %v, want 201", res.StatusCode, body)
	}
	name, _ := body["name"].(string)
	if name != "こんにちは世界" {
		t.Fatalf("folder name = %q, want the title's letters", name)
	}
	index := filepath.Join(f.base, "drafts", "ideas", name, "index.md")
	data, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "slug: "+name) {
		t.Errorf("frontmatter does not pin slug to %q:\n%s", name, data)
	}
}

// newWatchedFixture wires a real filesystem watcher into the fixture's app
// so the change feed has a source.
func newWatchedFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	w, err := watch.New(filepath.Join(f.base, "drafts", "ideas"), filepath.Join(f.base, "drafts", "wip"), f.app.Collection.Dir)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	f.app.Events = w.Events()
	f.srv.startFeed()
	return f
}

// openFeed opens the change feed carrying the token in the query string,
// the way EventSource must.
func (f *fixture) openFeed(ctx context.Context) *http.Response {
	f.t.Helper()
	req, err := http.NewRequestWithContext(ctx, "GET",
		f.baseURL+"/api/collections/blog/events?token="+f.srv.Token(), nil)
	if err != nil {
		f.t.Fatal(err)
	}
	res, err := f.ts.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

// feedLines streams a feed response body into a channel of lines.
type feedLines struct {
	lines chan string
	res   *http.Response
}

func (f *fixture) readFeed(res *http.Response) *feedLines {
	fl := &feedLines{lines: make(chan string, 64), res: res}
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			fl.lines <- sc.Text()
		}
		close(fl.lines)
	}()
	return fl
}

func (fl *feedLines) await(t *testing.T, needle string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case line, ok := <-fl.lines:
			if !ok {
				return false
			}
			if strings.Contains(line, needle) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// TestChangeFeedGuards verifies the feed obeys the same controls as every
// API request — token (header or, for EventSource, query) and Host — and
// that a valid request opens the stream.
func TestChangeFeedGuards(t *testing.T) {
	f := newWatchedFixture(t)

	get := func(url string, host string) *http.Response {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		if host != "" {
			req.Host = host
		}
		res, err := f.ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}

	if res := get(f.baseURL+"/api/collections/blog/events", ""); res.StatusCode != 401 {
		t.Errorf("no token = %d, want 401", res.StatusCode)
	}
	if res := get(f.baseURL+"/api/collections/blog/events?token=wrong", ""); res.StatusCode != 401 {
		t.Errorf("wrong query token = %d, want 401", res.StatusCode)
	}
	if res := get(f.baseURL+"/api/collections/blog/events?token="+f.srv.Token(), "rebind.example:9999"); res.StatusCode != 403 {
		t.Errorf("foreign Host = %d, want 403", res.StatusCode)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res := f.openFeed(ctx)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("valid query token = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if !f.readFeed(res).await(t, "event: ready", 2*time.Second) {
		t.Error("stream never announced readiness")
	}
}

// TestChangeFeedUnavailableWithoutWatcher verifies the feed is refused, not
// silently empty, when watching could not be established — the client's cue
// to fall back to periodic refresh.
func TestChangeFeedUnavailableWithoutWatcher(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", f.baseURL+"/api/collections/blog/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-AstroGUI-Token", f.srv.Token())
	res, err := f.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("feed without watcher = %d, want 503", res.StatusCode)
	}
}

// TestChangeFeedDeliversTicksAndSurvivesDisconnects verifies a change under
// a watched directory reaches connected clients, and that a client dropping
// off never blocks the others.
func TestChangeFeedDeliversTicksAndSurvivesDisconnects(t *testing.T) {
	f := newWatchedFixture(t)
	ideas := filepath.Join(f.base, "drafts", "ideas")
	postFile := filepath.Join(ideas, "tick-post", "index.md")
	if err := os.MkdirAll(filepath.Dir(postFile), 0o755); err != nil {
		t.Fatal(err)
	}

	ctxA, cancelA := context.WithCancel(context.Background())
	resA := f.openFeed(ctxA)
	a := f.readFeed(resA)
	if !a.await(t, "event: ready", 2*time.Second) {
		t.Fatal("client A never saw ready")
	}
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	resB := f.openFeed(ctxB)
	b := f.readFeed(resB)
	if !b.await(t, "event: ready", 2*time.Second) {
		t.Fatal("client B never saw ready")
	}

	if err := os.WriteFile(postFile, []byte("---\ntitle: Tick\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !a.await(t, "event: changed", 5*time.Second) {
		t.Error("client A never heard about the change")
	}
	if !b.await(t, "event: changed", 5*time.Second) {
		t.Error("client B never heard about the change")
	}

	// Client A drops off mid-stream.
	cancelA()
	resA.Body.Close()

	// Another change: client B still hears about it.
	if err := os.WriteFile(postFile, []byte("---\ntitle: Tick\n---\nbody grown"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !b.await(t, "event: changed", 5*time.Second) {
		t.Error("a disconnected client blocked the feed for others")
	}
}

// TestChangeFeedHeartbeat verifies the stream keeps itself alive with
// comment pings so dead connections surface as write errors.
func TestChangeFeedHeartbeat(t *testing.T) {
	f := newWatchedFixture(t)
	f.srv.heartbeat = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res := f.openFeed(ctx)
	defer res.Body.Close()
	if !f.readFeed(res).await(t, ": ping", 2*time.Second) {
		t.Error("heartbeat never arrived")
	}
}

// TestRenameAndDiscardRoutes verifies the management actions over the API:
// rename succeeds and reports the derived name, a taken name is a 409, and
// loose posts are refused 403 for both routes.
func TestRenameAndDiscardRoutes(t *testing.T) {
	f := newFixture(t)
	name := f.createIdea("rename me please")

	// Loose file for the refusal cases.
	content := filepath.Join(f.base, "src", "content", "blog")
	if err := os.WriteFile(filepath.Join(content, "loose.md"), []byte("---\ntitle: Loose\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Rename succeeds and reports the slugified name.
	res, body := f.do("POST", "/api/collections/blog/entries/"+name+"/rename", map[string]any{"name": "A Cleaner Name"})
	if res.StatusCode != 200 {
		t.Fatalf("rename = %d %v, want 200", res.StatusCode, body)
	}
	if body["name"] != "a-cleaner-name" {
		t.Errorf("renamed name = %v, want a-cleaner-name", body["name"])
	}
	if _, err := os.Stat(filepath.Join(f.base, "drafts", "ideas", "a-cleaner-name", "index.md")); err != nil {
		t.Fatalf("folder did not move: %v", err)
	}

	// Taken name: 409, both posts unchanged.
	f.createIdea("occupy the name") // becomes occupy-the-name
	res, body = f.do("POST", "/api/collections/blog/entries/a-cleaner-name/rename", map[string]any{"name": "occupy the name"})
	if res.StatusCode != 409 {
		t.Fatalf("taken name = %d %v, want 409", res.StatusCode, body)
	}

	// Loose posts are refused on both routes.
	res, body = f.do("POST", "/api/collections/blog/entries/loose/rename", map[string]any{"name": "elsewhere"})
	if res.StatusCode != 403 {
		t.Fatalf("loose rename = %d %v, want 403", res.StatusCode, body)
	}
	res, body = f.do("POST", "/api/collections/blog/entries/loose/discard", nil)
	if res.StatusCode != 403 {
		t.Fatalf("loose discard = %d %v, want 403", res.StatusCode, body)
	}

	// Discard succeeds: the folder lands in the trash, not deletion.
	res, body = f.do("POST", "/api/collections/blog/entries/a-cleaner-name/discard", nil)
	if res.StatusCode != 200 {
		t.Fatalf("discard = %d %v, want 200", res.StatusCode, body)
	}
	if _, err := os.Stat(filepath.Join(f.base, "drafts", "trash", "a-cleaner-name", "index.md")); err != nil {
		t.Fatalf("post not in trash: %v", err)
	}
	// And the board no longer lists it.
	res, body = f.do("GET", "/api/collections/blog/board", nil)
	if res.StatusCode != 200 {
		t.Fatalf("board: %d", res.StatusCode)
	}
	for _, card := range body["columns"].(map[string]any)["ideas"].([]any) {
		if card.(map[string]any)["name"] == "a-cleaner-name" {
			t.Error("discarded post still on the board")
		}
	}
}

// TestDevURLRoute verifies the dev-server bridge: URL shapes for default and
// configured bases, honest reachability both ways, and the standard request
// guards.
func TestDevURLRoute(t *testing.T) {
	f := newFixture(t)

	// Default shape: Astro's conventional base plus the slug.
	res, body := f.do("GET", "/api/collections/blog/dev-url?slug=my-post", nil)
	if res.StatusCode != 200 {
		t.Fatalf("dev-url = %d %v, want 200", res.StatusCode, body)
	}
	if body["url"] != "http://localhost:4321/my-post/" {
		t.Errorf("default url = %v", body["url"])
	}
	if _, ok := body["reachable"].(bool); !ok {
		t.Errorf("reachable = %v, want a boolean", body["reachable"])
	}

	// A configured base with a subpath and trailing slash joins cleanly.
	f.app.DevURL = "http://localhost:4321/posts/"
	res, body = f.do("GET", "/api/collections/blog/dev-url?slug=my-post", nil)
	if res.StatusCode != 200 || body["url"] != "http://localhost:4321/posts/my-post/" {
		t.Fatalf("subpath url = %d %v", res.StatusCode, body)
	}

	// Nothing listening: reachable is honestly false (port 1 refuses fast).
	f.app.DevURL = "http://127.0.0.1:1"
	res, body = f.do("GET", "/api/collections/blog/dev-url?slug=x", nil)
	if res.StatusCode != 200 {
		t.Fatalf("probe-false response = %d", res.StatusCode)
	}
	if body["reachable"] != false {
		t.Errorf("reachable = %v, want false", body["reachable"])
	}

	// Something listening: reachable is true.
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // any answer means running
	}))
	defer up.Close()
	f.app.DevURL = up.URL
	res, body = f.do("GET", "/api/collections/blog/dev-url?slug=x", nil)
	if res.StatusCode != 200 || body["reachable"] != true {
		t.Fatalf("reachable-true = %d %v", res.StatusCode, body)
	}

	// The route obeys the standard guards: token and Host.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", f.baseURL+"/api/collections/blog/dev-url?slug=x", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = f.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Errorf("no token = %d, want 401", res.StatusCode)
	}
	req, err = http.NewRequestWithContext(ctx, "GET", f.baseURL+"/api/collections/blog/dev-url?slug=x", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "rebind.example:9999"
	res, err = f.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Errorf("foreign Host = %d, want 403", res.StatusCode)
	}

	// A missing slug is a named client error.
	res, body = f.do("GET", "/api/collections/blog/dev-url", nil)
	if res.StatusCode != 400 {
		t.Fatalf("missing slug = %d %v, want 400", res.StatusCode, body)
	}
}
