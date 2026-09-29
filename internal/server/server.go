// Package server hosts the embedded interface and the local HTTP API, with
// the controls that make a file-writing localhost server safe to run:
// loopback-only binding, Host header validation, a per-run session token
// carried in the URL fragment, real-path confinement of asset requests, and
// post content treated as untrusted in the tool's own page.
package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/astrogui/astrogui/internal/cache"
	"github.com/astrogui/astrogui/internal/config"
	"github.com/astrogui/astrogui/internal/fmedit"
	"github.com/astrogui/astrogui/internal/lifecycle"
	"github.com/astrogui/astrogui/internal/posts"
	"github.com/astrogui/astrogui/internal/project"
	"github.com/astrogui/astrogui/internal/safe"
	"github.com/astrogui/astrogui/internal/vcs"
)

// App carries everything the server needs about the managed project.
type App struct {
	Project    string
	Collection project.Collection
	Config     config.Project
	Manager    *lifecycle.Manager
	Cache      *cache.Cache
	Guard      *safe.Guard
	Version    string
}

// Server is the HTTP host for one run.
type Server struct {
	app      *App
	token    string
	ui       fs.FS
	expected string // expected Host header value, e.g. 127.0.0.1:4190
	mux      *http.ServeMux
}

// New mints a session token and wires the routes. The token is delivered in
// the URL fragment of the announced address; every API call must carry it in
// the X-AstroGUI-Token header, which a cross-origin page cannot obtain.
func New(app *App, ui fs.FS) (*Server, error) {
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("server: minting session token: %w", err)
	}
	s := &Server{
		app:   app,
		token: hex.EncodeToString(secret),
		ui:    ui,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

// Token exposes the per-run session token (for the announced URL fragment).
func (s *Server) Token() string { return s.token }

// Serve runs the HTTP server on the listener until it is closed.
func (s *Server) Serve(ln net.Listener) error {
	httpServer := &http.Server{Handler: s.Handler(ln)}
	return httpServer.Serve(ln)
}

// Announce builds the URL the user should open, token in the fragment.
func (s *Server) Announce(host string) string {
	return "http://" + host + "/#token=" + s.token
}

// Listen binds loopback only. A busy preferred port is reported and a free
// port is selected; the caller announces the address actually served.
func Listen(preferredPort int) (net.Listener, string, error) {
	try := func(port int) (net.Listener, error) {
		return net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	}
	ln, err := try(preferredPort)
	if err == nil {
		return ln, "", nil
	}
	// Report the conflict, then take a free port rather than failing or
	// taking over anything.
	free, ferr := try(0)
	if ferr != nil {
		return nil, "", fmt.Errorf("server: preferred port %d is in use (%v) and no free port could be bound: %w", preferredPort, err, ferr)
	}
	return free, fmt.Sprintf("port %d is already in use; serving on %d instead", preferredPort, free.Addr().(*net.TCPAddr).Port), nil
}

// Handler returns the fully-wrapped handler for a listener.
func (s *Server) Handler(ln net.Listener) http.Handler {
	s.expected = ln.Addr().String() // e.g. 127.0.0.1:4190
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Control 1: loopback binding is by construction (Listen).
	// Control 2: the Host header must name our own origin, which defeats DNS
	// rebinding; anything else is refused before routing.
	if !s.hostOK(r.Host) {
		http.Error(w, "refused: unexpected Host header", http.StatusForbidden)
		return
	}
	// Control 3: every API call must carry the session token. Refusal
	// happens before any filesystem effect.
	if strings.HasPrefix(r.URL.Path, "/api/") && r.Header.Get("X-AstroGUI-Token") != s.token {
		http.Error(w, "refused: missing or invalid session token", http.StatusUnauthorized)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api/") {
		s.mux.ServeHTTP(w, r)
		return
	}
	s.serveUI(w, r)
}

// hostOK accepts only the literal loopback origin we serve on.
func (s *Server) hostOK(host string) bool {
	if s.expected == "" {
		return false
	}
	if host == s.expected {
		return true
	}
	// localhost spelled by name is accepted with our port.
	_, port, err := net.SplitHostPort(s.expected)
	if err != nil {
		return false
	}
	return host == net.JoinHostPort("localhost", port)
}

// serveUI serves the embedded interface with a Content-Security-Policy that
// forbids inline script, so post content rendered in the preview cannot
// execute even if the client-side sanitizer missed something.
func (s *Server) serveUI(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	data, err := fs.ReadFile(s.ui, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch filepath.Ext(name) {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Write(data)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/collections", s.handleCollections)
	s.mux.HandleFunc("GET /api/collections/{name}/board", s.guardCollection(s.handleBoard))
	s.mux.HandleFunc("GET /api/collections/{name}/entries", s.guardCollection(s.handleEntries))
	s.mux.HandleFunc("POST /api/collections/{name}/entries", s.guardCollection(s.handleCreate))
	s.mux.HandleFunc("GET /api/collections/{name}/entries/{post}", s.guardCollection(s.handleEntry))
	s.mux.HandleFunc("PUT /api/collections/{name}/entries/{post}/body", s.guardCollection(s.handleSaveBody))
	s.mux.HandleFunc("PUT /api/collections/{name}/entries/{post}/frontmatter", s.guardCollection(s.handleSaveFrontmatter))
	s.mux.HandleFunc("PUT /api/collections/{name}/entries/{post}/raw", s.guardCollection(s.handleSaveRaw))
	s.mux.HandleFunc("POST /api/collections/{name}/entries/{post}/move", s.guardCollection(s.handleMove))
	s.mux.HandleFunc("POST /api/collections/{name}/entries/{post}/assets", s.guardCollection(s.handleUploadAsset))
	s.mux.HandleFunc("GET /api/collections/{name}/entries/{post}/assets/{file}", s.guardCollection(s.handleAsset))
	s.mux.HandleFunc("GET /api/collections/{name}/funnel", s.guardCollection(s.handleFunnel))
	s.mux.HandleFunc("POST /api/collections/{name}/commit", s.guardCollection(s.handleCommit))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"version":    s.app.Version,
		"project":    s.app.Project,
		"collection": s.app.Collection.Name,
	})
}

func (s *Server) handleCollections(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"collections": []string{s.app.Collection.Name}})
}

// guardCollection ensures the request addresses the managed collection by
// name (a manager that hardcodes one collection is wrong the first time a
// project has two).
func (s *Server) guardCollection(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("name") != s.app.Collection.Name {
			writeErr(w, http.StatusNotFound, fmt.Errorf("unknown collection %q", r.PathValue("name")))
			return
		}
		next(w, r)
	}
}

// boardJSON is one card on the board.
type cardJSON struct {
	Name     string   `json:"name"`
	Title    string   `json:"title"`
	State    string   `json:"state"`
	Loose    bool     `json:"loose"`
	ReadOnly bool     `json:"readOnly"`
	Snippet  string   `json:"snippet"`
	Stalled  bool     `json:"stalled"`
	Meta     metaJSON `json:"meta"`
}

type metaJSON struct {
	FirstSeen   time.Time `json:"firstSeen"`
	LastChanged time.Time `json:"lastChanged"`
	Size        int       `json:"size"`
	Images      int       `json:"images"`
}

func (s *Server) cardFor(p *posts.Post, now time.Time) cardJSON {
	firstSeen, _ := s.app.Cache.FirstSeen(s.app.Project, p.Name, now)
	meta := posts.DeriveMeta(p, firstSeen)
	meta.Stalled = now.Sub(meta.LastModified) > s.app.Config.EffectiveStaleness()
	snippet := strings.Join(strings.Fields(string(p.Body())), " ")
	if len(snippet) > 90 {
		snippet = snippet[:90] + "…"
	}
	return cardJSON{
		Name:     p.Name,
		Title:    p.Title(),
		State:    p.State,
		Loose:    p.Loose,
		ReadOnly: p.Loose,
		Snippet:  snippet,
		Stalled:  meta.Stalled,
		Meta: metaJSON{
			FirstSeen:   meta.FirstSeen,
			LastChanged: meta.LastModified,
			Size:        meta.Size,
			Images:      meta.Images,
		},
	}
}

func (s *Server) listing() (posts.Listing, error) {
	return posts.List(s.app.Manager.Ideas, s.app.Manager.WIP, s.app.Manager.Content)
}

func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	l, err := s.listing()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	now := time.Now()
	board := map[string][]cardJSON{}
	for _, state := range posts.States {
		cards := []cardJSON{}
		for _, p := range l.In(state) {
			cards = append(cards, s.cardFor(p, now))
		}
		board[state] = cards
	}
	writeJSON(w, 200, map[string]any{
		"collection": s.app.Collection.Name,
		"states":     posts.States,
		"columns":    board,
		"staleness":  s.app.Config.EffectiveStaleness().String(),
	})
}

func (s *Server) handleEntries(w http.ResponseWriter, r *http.Request) {
	l, err := s.listing()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	now := time.Now()
	cards := []cardJSON{}
	for _, p := range l.All() {
		cards = append(cards, s.cardFor(p, now))
	}
	writeJSON(w, 200, map[string]any{"entries": cards})
}

func (s *Server) findPost(name string) (*posts.Post, error) {
	return s.app.Manager.Find(name)
}

func (s *Server) handleEntry(w http.ResponseWriter, r *http.Request) {
	p, err := s.findPost(r.PathValue("post"))
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	if p == nil {
		writeErr(w, 404, fmt.Errorf("no post %q", r.PathValue("post")))
		return
	}
	info, _ := os.Stat(p.File)
	mod := time.Time{}
	if info != nil {
		mod = info.ModTime()
	}
	firstSeen, _ := s.app.Cache.FirstSeen(s.app.Project, p.Name, time.Now())
	meta := posts.DeriveMeta(p, firstSeen)
	writeJSON(w, 200, map[string]any{
		"name":           p.Name,
		"title":          p.Title(),
		"state":          p.State,
		"loose":          p.Loose,
		"readOnly":       p.Loose,
		"frontmatter":    p.FrontmatterMap(),
		"frontmatterRaw": string(p.Frontmatter()),
		"hasFrontmatter": p.HasFrontmatter(),
		"body":           string(p.Body()),
		"modTime":        mod,
		"assets":         p.Assets(),
		"firstSeen":      meta.FirstSeen,
		"stalled":        time.Since(meta.LastModified) > s.app.Config.EffectiveStaleness(),
		"preflight":      s.app.Manager.Preflight(p),
	})
}

// handleCreate captures a new entry: a bare idea (a single line, no
// structured metadata required) or a titled post.
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Line  string `json:"line"`
		Title string `json:"title"`
		Body  string `json:"body"`
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, fmt.Errorf("invalid request body: %w", err))
		return
	}
	state := req.State
	if state == "" {
		state = posts.StateIdeas
	}
	if state != posts.StateIdeas && state != posts.StateWIP {
		writeErr(w, 400, fmt.Errorf("posts are created in %s or %s", posts.StateIdeas, posts.StateWIP))
		return
	}

	var p *posts.Post
	var err error
	if strings.TrimSpace(req.Line) != "" {
		// Idea capture: the line is the content; the name derives from it so
		// no structured metadata is required.
		p, err = s.app.Manager.Create(state, nameFromLine(req.Line), nil, []byte(req.Line+"\n"))
	} else if strings.TrimSpace(req.Title) != "" {
		body := req.Body
		if body != "" && !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		p, err = s.app.Manager.Create(state, req.Title, map[string]any{"title": req.Title}, []byte(body))
	} else {
		writeErr(w, 400, fmt.Errorf("provide a line to capture as an idea, or a title"))
		return
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 201, map[string]any{"name": p.Name, "state": p.State})
}

// nameFromLine derives a short, unique-enough folder name from an idea's
// first line.
func nameFromLine(line string) string {
	words := strings.Fields(line)
	if len(words) > 6 {
		words = words[:6]
	}
	return lifecycle.Slugify(strings.Join(words, " "))
}

func (s *Server) handleSaveBody(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Body    string    `json:"body"`
		ModTime time.Time `json:"modTime"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, fmt.Errorf("invalid request body: %w", err))
		return
	}
	p, err := s.findPost(r.PathValue("post"))
	if err != nil || p == nil {
		writeErr(w, 404, fmt.Errorf("no post %q", r.PathValue("post")))
		return
	}
	if p.Loose {
		// The spec's editing path for posts the tool did not create is the
		// raw text view; the structured body pane is not it.
		writeErr(w, 403, fmt.Errorf("loose posts are edited through the raw text view only"))
		return
	}
	if err := s.app.Guard.Check(p.File); err != nil {
		writeErr(w, 403, err)
		return
	}
	err = posts.SaveBody(p.File, []byte(req.Body), req.ModTime)
	var conflict *posts.ConflictError
	if errors.As(err, &conflict) {
		writeJSON(w, 409, map[string]any{
			"error":   err.Error(),
			"current": string(conflict.Current),
		})
		return
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	info, _ := os.Stat(p.File)
	writeJSON(w, 200, map[string]any{"saved": true, "modTime": info.ModTime()})
}

// handleSaveFrontmatter edits structured fields. Only fields that actually
// changed are written, through the comment- and order-preserving editor.
func (s *Server) handleSaveFrontmatter(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Fields  map[string]any `json:"fields"`
		ModTime time.Time      `json:"modTime"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, fmt.Errorf("invalid request body: %w", err))
		return
	}
	p, err := s.findPost(r.PathValue("post"))
	if err != nil || p == nil {
		writeErr(w, 404, fmt.Errorf("no post %q", r.PathValue("post")))
		return
	}
	if p.Loose {
		writeErr(w, 403, fmt.Errorf("loose posts are edited through the raw view only"))
		return
	}
	if err := s.app.Guard.Check(p.File); err != nil {
		writeErr(w, 403, err)
		return
	}

	// Nothing changed: no rewrite at all.
	changed := fmedit.Changed(p.Frontmatter(), req.Fields)
	if len(changed) == 0 {
		info, _ := os.Stat(p.File)
		writeJSON(w, 200, map[string]any{"saved": false, "changed": false, "modTime": info.ModTime()})
		return
	}

	// Apply the changed fields to the frontmatter block only.
	fm := p.Frontmatter()
	if len(fm) == 0 {
		fm = []byte("\n")
	}
	for field, value := range changed {
		fm, err = fmedit.Update(fm, field, value)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
	}

	// Reassemble: delimiters and body byte-exact, frontmatter with only the
	// changed lines rewritten.
	var out []byte
	if p.HasFrontmatter() {
		out = append(out, p.Bytes()[:p.FMStart()]...)
		out = append(out, fm...)
		if len(fm) == 0 || fm[len(fm)-1] != '\n' {
			out = append(out, '\n')
		}
		out = append(out, p.Bytes()[p.FMEnd():]...)
	} else {
		out = append(out, []byte("---\n")...)
		out = append(out, fm...)
		if len(fm) == 0 || fm[len(fm)-1] != '\n' {
			out = append(out, '\n')
		}
		out = append(out, []byte("---\n")...)
		out = append(out, p.Bytes()...)
	}

	err = posts.WriteFile(p.File, out, p.Bytes(), req.ModTime)
	var conflict *posts.ConflictError
	if errors.As(err, &conflict) {
		writeJSON(w, 409, map[string]any{"error": err.Error(), "current": string(conflict.Current)})
		return
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	info, _ := os.Stat(p.File)
	writeJSON(w, 200, map[string]any{"saved": true, "changed": true, "modTime": info.ModTime()})
}

// handleSaveRaw replaces the whole file from the raw view.
func (s *Server) handleSaveRaw(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string    `json:"content"`
		ModTime time.Time `json:"modTime"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, fmt.Errorf("invalid request body: %w", err))
		return
	}
	p, err := s.findPost(r.PathValue("post"))
	if err != nil || p == nil {
		writeErr(w, 404, fmt.Errorf("no post %q", r.PathValue("post")))
		return
	}
	if err := s.app.Guard.Check(p.File); err != nil {
		writeErr(w, 403, err)
		return
	}
	err = posts.WriteFile(p.File, []byte(req.Content), p.Bytes(), req.ModTime)
	var conflict *posts.ConflictError
	if errors.As(err, &conflict) {
		writeJSON(w, 409, map[string]any{"error": err.Error(), "current": string(conflict.Current)})
		return
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	info, _ := os.Stat(p.File)
	writeJSON(w, 200, map[string]any{"saved": true, "modTime": info.ModTime()})
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, fmt.Errorf("invalid request body: %w", err))
		return
	}
	p, err := s.findPost(r.PathValue("post"))
	if err != nil || p == nil {
		writeErr(w, 404, fmt.Errorf("no post %q", r.PathValue("post")))
		return
	}
	err = s.app.Manager.Move(p.Name, p.State, req.To)
	var checks *lifecycle.CheckFailure
	if errors.As(err, &checks) {
		writeJSON(w, 422, map[string]any{"error": err.Error(), "problems": checks.Problems})
		return
	}
	if errors.Is(err, lifecycle.ErrDestinationExists) || errors.Is(err, lifecycle.ErrCrossDevice) {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	if err := s.app.Cache.Save(); err != nil {
		// Derived data is disposable: a save failure does not fail the move.
		_ = err
	}
	writeJSON(w, 200, map[string]any{"moved": true, "to": req.To})
}

func (s *Server) handleFunnel(w http.ResponseWriter, r *http.Request) {
	l, err := s.listing()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	counts := map[string]int{}
	for _, state := range posts.States {
		counts[state] = len(l.In(state))
	}
	writeJSON(w, 200, s.app.Cache.FunnelFor(s.app.Project, counts))
}

func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
		Push    bool   `json:"push"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, fmt.Errorf("invalid request body: %w", err))
		return
	}
	var result *vcs.Result
	var err error
	if req.Push {
		result, err = vcs.CommitAndPush(s.app.Project, req.Message)
	} else {
		result, err = vcs.Commit(s.app.Project, req.Message)
	}
	if errors.Is(err, vcs.ErrUnavailable) {
		writeErr(w, 503, err)
		return
	}
	if err != nil && result == nil {
		writeErr(w, 500, err)
		return
	}
	status := 200
	if err != nil {
		status = 500 // failure output surfaces verbatim, never as success
	}
	writeJSON(w, status, map[string]any{
		"command": result.Command,
		"output":  result.Output,
		"ok":      err == nil,
	})
}

// handleUploadAsset stores a pasted image in the post's own directory.
func (s *Server) handleUploadAsset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		DataB64 string `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 20<<20)).Decode(&req); err != nil {
		writeErr(w, 400, fmt.Errorf("invalid request body: %w", err))
		return
	}
	p, err := s.findPost(r.PathValue("post"))
	if err != nil || p == nil {
		writeErr(w, 404, fmt.Errorf("no post %q", r.PathValue("post")))
		return
	}
	if p.Loose {
		writeErr(w, 403, fmt.Errorf("loose posts hold no assets of their own"))
		return
	}

	name := lifecycle.Slugify(strings.TrimSuffix(req.Name, filepath.Ext(req.Name)))
	ext := strings.ToLower(filepath.Ext(req.Name))
	if ext == "" {
		ext = ".png"
	}
	if name == "" || name == "untitled" {
		name = "pasted-" + time.Now().Format("20060102-150405")
	}
	// A useful, unique name: never fail because the paste had no name.
	final := name + ext
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(p.Dir, final)); err != nil {
			break
		}
		final = fmt.Sprintf("%s-%d%s", name, i, ext)
	}

	data, err := decodeBase64(req.DataB64)
	if err != nil {
		writeErr(w, 400, fmt.Errorf("invalid image data: %w", err))
		return
	}
	target := filepath.Join(p.Dir, final)
	if err := s.app.Guard.Check(target); err != nil {
		writeErr(w, 403, err)
		return
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 201, map[string]any{"name": final})
}

// handleAsset serves a post's own asset, confined by real path after symlink
// resolution to the post's directory.
func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	p, err := s.findPost(r.PathValue("post"))
	if err != nil || p == nil {
		writeErr(w, 404, fmt.Errorf("no post %q", r.PathValue("post")))
		return
	}
	file := r.PathValue("file")

	// Control 4: confinement by resolved real path against the post's own
	// directory. The refusal discloses nothing about the path.
	target := filepath.Join(p.Dir, filepath.FromSlash(file))
	real, err := filepath.EvalSymlinks(target)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	realDir, err := filepath.EvalSymlinks(p.Dir)
	if err != nil || !under(realDir, real) {
		http.Error(w, "refused", http.StatusForbidden)
		return
	}
	data, err := os.ReadFile(real)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	switch strings.ToLower(filepath.Ext(real)) {
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".avif":
		w.Header().Set("Content-Type", "image/avif")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Write(data)
}

// under reports whether path is dir itself or inside it.
func under(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(strings.TrimSpace(s))
}
