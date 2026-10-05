// Package lifecycle implements the directory state machine: every transition
// is a single atomic filesystem move, and the move into the published state is
// gated by pre-flight checks. The design invariant: a move either fully
// succeeds or leaves both locations unchanged.
package lifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/astrogui/astrogui/internal/cache"
	"github.com/astrogui/astrogui/internal/fmedit"
	"github.com/astrogui/astrogui/internal/posts"
	"github.com/astrogui/astrogui/internal/safe"
	"github.com/goccy/go-yaml"
)

// ErrDestinationExists is returned when a move's destination already holds a
// post; both posts are left unchanged.
var ErrDestinationExists = errors.New("destination already exists")

// ErrCrossDevice is returned when a move would cross a filesystem boundary,
// where no atomic rename exists. The move is refused rather than degraded to
// a non-atomic copy.
var ErrCrossDevice = errors.New("move crosses a filesystem boundary; no atomic rename exists")

// CheckFailure reports that pre-flight checks blocked a publish.
type CheckFailure struct {
	Problems []Problem
}

func (e *CheckFailure) Error() string {
	var msgs []string
	for _, p := range e.Problems {
		msgs = append(msgs, p.Message)
	}
	return "pre-flight check failed: " + strings.Join(msgs, "; ")
}

// Problem is one named pre-flight failure.
type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Manager performs lifecycle operations for one project.
type Manager struct {
	Ideas   string
	WIP     string
	Content string
	Trash   string
	Guard   *safe.Guard
	Cache   *cache.Cache
	Project string // project root, the cache key

	// rename is the underlying move, swappable in tests (to simulate a
	// cross-device failure or an interrupted move).
	rename func(oldname, newname string) error
}

// New creates a Manager whose every write passes the guard.
func New(ideas, wip, content, trash string, guard *safe.Guard, c *cache.Cache, project string) *Manager {
	m := &Manager{
		Ideas:   ideas,
		WIP:     wip,
		Content: content,
		Trash:   trash,
		Guard:   guard,
		Cache:   c,
		Project: project,
		rename:  os.Rename,
	}
	return m
}

// validPostName reports whether name is a clean single path element — the
// only shape a post name may take. Every request-derived name funnels
// through FindIn (or through Slugify, whose output always satisfies this),
// so a name carrying separators or traversal can never reach a filesystem
// path. This is the read-side twin of the write guard.
func validPostName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, 0) {
		return false
	}
	return name == filepath.Base(name)
}

// DirFor returns the directory of a lifecycle state.
func (m *Manager) DirFor(state string) (string, error) {
	switch state {
	case posts.StateIdeas:
		return m.Ideas, nil
	case posts.StateWIP:
		return m.WIP, nil
	case posts.StatePublished:
		return m.Content, nil
	}
	return "", fmt.Errorf("lifecycle: unknown state %q", state)
}

// Find locates a post by name across all states, returning nil when absent.
// When the same name exists in several states the earliest state wins; Move
// resolves posts within their source state via FindIn.
func (m *Manager) Find(name string) (*posts.Post, error) {
	for _, state := range posts.States {
		if p, err := m.FindIn(name, state); err != nil {
			return nil, err
		} else if p != nil {
			return p, nil
		}
	}
	return nil, nil
}

// FindIn locates a post by name within one state's directory, returning nil
// when the state holds no post of that name. A name that is not a clean
// single path element is refused outright: request-derived names reach this
// choke point, and a separator or traversal inside one must never reach a
// filesystem path.
func (m *Manager) FindIn(name, state string) (*posts.Post, error) {
	if !validPostName(name) {
		return nil, fmt.Errorf("lifecycle: %q is not a post name", name)
	}
	dir, err := m.DirFor(state)
	if err != nil {
		return nil, err
	}
	if index, ok := posts.FolderIndex(filepath.Join(dir, name)); ok {
		p, err := posts.Read(index)
		if err != nil {
			return nil, err
		}
		p.State = state
		return p, nil
	}
	// Loose file with that name (read-only posts).
	for _, ext := range []string{".md", ".mdx"} {
		file := filepath.Join(dir, name+ext)
		if _, err := os.Stat(file); err == nil {
			p, err := posts.Read(file)
			if err != nil {
				return nil, err
			}
			p.State = state
			p.Loose = true
			return p, nil
		}
	}
	return nil, nil
}

// Move performs a lifecycle transition as one atomic rename. Loose posts are
// never moved (posts the tool did not create are read-only).
func (m *Manager) Move(name, from, to string) error {
	srcDir, err := m.DirFor(from)
	if err != nil {
		return err
	}
	dstDir, err := m.DirFor(to)
	if err != nil {
		return err
	}
	src := filepath.Join(srcDir, name)
	dst := filepath.Join(dstDir, name)

	p, err := m.FindIn(name, from)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("lifecycle: no post %q in state %q", name, from)
	}
	if p.Loose {
		return fmt.Errorf("lifecycle: %q is a loose file the tool did not create; it is read-only and never moved", name)
	}

	// Every write passes the containment guard, source and destination.
	if err := m.Guard.Check(src, dst); err != nil {
		return fmt.Errorf("lifecycle: refusing move: %w", err)
	}

	// Refuse when the destination is taken: both posts stay unchanged.
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("%w: a post already exists at %s; both posts are unchanged", ErrDestinationExists, dst)
	}

	// The publish move is gated by pre-flight checks.
	if to == posts.StatePublished {
		if problems := m.Preflight(p); len(problems) > 0 {
			return &CheckFailure{Problems: problems}
		}
	}

	if err := m.Guard.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("lifecycle: preparing destination: %w", err)
	}
	if err := m.rename(src, dst); err != nil {
		if isCrossDevice(err) {
			return fmt.Errorf("%w: %s -> %s; the post is unchanged — complete the move by hand", ErrCrossDevice, src, dst)
		}
		return fmt.Errorf("lifecycle: move failed, post left unchanged at %s: %w", src, err)
	}

	// Record the observed transition for the funnel (derived, disposable).
	if m.Cache != nil {
		m.Cache.RecordTransition(m.Project, name, from, to, time.Now())
	}
	return nil
}

// Preflight verifies a post can survive being seen by the blog's build. It is
// heuristic by design and never reads the project's collection schema. Every
// problem names the specific thing that failed.
func (m *Manager) Preflight(p *posts.Post) []Problem {
	var problems []Problem

	// The title must come from frontmatter: the board's display fallback to
	// the folder name is not a publishable title.
	titleProblem := func(msg string) {
		problems = append(problems, Problem{Code: "title", Message: msg})
	}
	if !p.HasFrontmatter() {
		titleProblem("no title: the post has no frontmatter")
	} else if v, ok := p.FrontmatterMap()["title"]; !ok {
		titleProblem("no title: the post's frontmatter has no title field")
	} else if s, ok := v.(string); !ok || strings.TrimSpace(s) == "" {
		titleProblem("no title: the title field is empty")
	}

	date := p.Date()
	if p.DateMissing() {
		problems = append(problems, Problem{Code: "date", Message: "publication date missing or unparseable in frontmatter"})
	} else if date.After(time.Now().Add(24 * time.Hour)) {
		problems = append(problems, Problem{Code: "date", Message: fmt.Sprintf("publication date %s is in the future", date.Format("2006-01-02"))})
	}

	if len(strings.TrimSpace(string(p.Body()))) == 0 {
		problems = append(problems, Problem{Code: "body", Message: "the post body is empty"})
	}

	// Every referenced image must resolve inside the post's own directory.
	problems = append(problems, m.checkImages(p)...)

	sort.Slice(problems, func(i, j int) bool { return problems[i].Code < problems[j].Code })
	return problems
}

func (m *Manager) checkImages(p *posts.Post) []Problem {
	var problems []Problem
	for _, ref := range p.ImageRefs() {
		if isRemoteRef(ref) {
			continue // remote references are the blog's problem, not the move's
		}
		resolved := resolveRef(p.Dir, ref)
		inside := sameOrUnder(p.Dir, resolved)
		if !inside {
			problems = append(problems, Problem{
				Code:    "asset-outside",
				Message: fmt.Sprintf("image %q (referenced at %s) resolves outside the post's own directory", ref, refLocation(p, ref)),
			})
			continue
		}
		if _, err := os.Stat(resolved); err != nil {
			problems = append(problems, Problem{
				Code:    "missing-image",
				Message: fmt.Sprintf("image %q is missing from the post's directory (referenced at %s)", ref, refLocation(p, ref)),
			})
		}
	}
	return problems
}

// refLocation finds the body line that carries a reference, so failures name
// where to look.
func refLocation(p *posts.Post, ref string) string {
	lines := strings.Split(string(p.Body()), "\n")
	for i, line := range lines {
		if strings.Contains(line, ref) {
			return fmt.Sprintf("body line %d", i+1)
		}
	}
	return "unknown location"
}

func isRemoteRef(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "//")
}

// resolveRef resolves a possibly-relative image reference against the post's
// own directory, cleaning traversal segments.
func resolveRef(postDir, ref string) string {
	ref = strings.TrimPrefix(strings.TrimPrefix(ref, "./"), ".\\")
	if filepath.IsAbs(ref) {
		return filepath.Clean(ref)
	}
	return filepath.Clean(filepath.Join(postDir, filepath.FromSlash(ref)))
}

func sameOrUnder(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// slugSeparator matches everything that is not a letter, number, or
// combining mark in any script: titles keep their non-ASCII letters instead
// of collapsing to a placeholder, while separator runs still normalize to -.
var slugSeparator = regexp.MustCompile(`[^\p{L}\p{N}\p{M}]+`)

// Slugify derives a URL-friendly, folder-safe name from a title: the title's
// own letters and numbers, whatever their script, lowercased and separated.
func Slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = slugSeparator.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "untitled"
	}
	return s
}

// UniqueName returns name, or name-2, name-3… when name is already taken in
// the state directory.
func (m *Manager) UniqueName(state, name string) string {
	dir, err := m.DirFor(state)
	if err != nil {
		return name
	}
	return uniqueName(dir, name)
}

// uniqueName returns name, or name-2, name-3… when dir already holds it.
func uniqueName(dir, name string) string {
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if _, err := os.Stat(filepath.Join(dir, candidate)); err != nil {
			return candidate
		}
	}
}

// Rename renames a folder post within its current state as a single atomic
// directory rename, keeping the pinned slug equal to the new name (the author
// changed the post's identity deliberately). Loose files are never renamed:
// posts the tool did not create are read-only.
func (m *Manager) Rename(name, state, newName string) (*posts.Post, error) {
	dir, err := m.DirFor(state)
	if err != nil {
		return nil, err
	}
	newName = Slugify(newName)
	// Slugify's output provably satisfies this (every separator-ish run
	// becomes "-"), but the check keeps the guarantee local to the sink.
	if !validPostName(newName) {
		return nil, fmt.Errorf("lifecycle: %q does not derive to a usable post name", newName)
	}

	p, err := m.FindIn(name, state)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("lifecycle: no post %q in state %q", name, state)
	}
	if p.Loose {
		return nil, fmt.Errorf("lifecycle: %q is a loose file the tool did not create; it is read-only and never renamed", name)
	}

	src := filepath.Join(dir, name)
	dst := filepath.Join(dir, newName)
	if src == dst {
		return p, nil // the name already is what it derives to
	}

	// Every write passes the containment guard, source and destination.
	if err := m.Guard.Check(src, dst); err != nil {
		return nil, fmt.Errorf("lifecycle: refusing rename: %w", err)
	}
	// Refuse when the destination is taken: both posts stay unchanged.
	if _, err := os.Lstat(dst); err == nil {
		return nil, fmt.Errorf("%w: a post already exists at %s; both posts are unchanged", ErrDestinationExists, dst)
	}

	if err := m.rename(src, dst); err != nil {
		if isCrossDevice(err) {
			return nil, fmt.Errorf("%w: %s -> %s; the post is unchanged — complete the rename by hand", ErrCrossDevice, src, dst)
		}
		return nil, fmt.Errorf("lifecycle: rename failed, post left unchanged at %s: %w", src, err)
	}

	index, ok := posts.FolderIndex(dst)
	if !ok {
		return nil, fmt.Errorf("lifecycle: renamed folder at %s holds no index file", dst)
	}
	renamed, err := posts.Read(index)
	if err != nil {
		return nil, err
	}
	renamed.State = state

	// The slug pin follows the name when the post carries frontmatter to pin
	// it in. Best effort by design: the folder rename is the atomic operation
	// and already succeeded, so a frontmatter edit that fails is reported by
	// nothing and rolls back nothing — the post keeps working, and the pin
	// still governs URL derivation from wherever it stands.
	if renamed.HasFrontmatter() {
		if updated, err := fmedit.Update(renamed.Frontmatter(), "slug", newName); err == nil {
			var out []byte
			out = append(out, renamed.Bytes()[:renamed.FMStart()]...)
			out = append(out, updated...)
			if len(updated) == 0 || updated[len(updated)-1] != '\n' {
				out = append(out, '\n')
			}
			out = append(out, renamed.Bytes()[renamed.FMEnd():]...)
			_ = m.Guard.AtomicWriteFile(renamed.File, out, 0o644)
		}
	}
	return renamed, nil
}

// Discard moves a folder post into the trash directory as a single atomic
// move, suffixing the name when the trash already holds a post of that name.
// No file is deleted: recovery is moving the folder back with any tool.
// Loose files are never discarded: posts the tool did not create are
// read-only.
func (m *Manager) Discard(name, state string) error {
	srcDir, err := m.DirFor(state)
	if err != nil {
		return err
	}
	p, err := m.FindIn(name, state)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("lifecycle: no post %q in state %q", name, state)
	}
	if p.Loose {
		return fmt.Errorf("lifecycle: %q is a loose file the tool did not create; it is read-only and never discarded", name)
	}

	src := filepath.Join(srcDir, name)
	dst := filepath.Join(m.Trash, uniqueName(m.Trash, name))

	if err := m.Guard.Check(src, dst); err != nil {
		return fmt.Errorf("lifecycle: refusing discard: %w", err)
	}
	if err := m.Guard.MkdirAll(m.Trash, 0o755); err != nil {
		return fmt.Errorf("lifecycle: preparing the trash directory: %w", err)
	}
	if err := m.rename(src, dst); err != nil {
		if isCrossDevice(err) {
			return fmt.Errorf("%w: %s -> %s; the post is unchanged — complete the move by hand", ErrCrossDevice, src, dst)
		}
		return fmt.Errorf("lifecycle: discard failed, post left unchanged at %s: %w", src, err)
	}
	return nil
}

// Create writes a new post folder in the given state. Frontmatter fields the
// tool pins (the slug, so the published URL depends only on the post's
// identity) are written once, here; the body is the author's bytes verbatim.
// No field that records workflow state is ever written.
func (m *Manager) Create(state, name string, frontmatter map[string]any, body []byte) (*posts.Post, error) {
	dir, err := m.DirFor(state)
	if err != nil {
		return nil, err
	}
	name = m.UniqueName(state, Slugify(name))
	postDir := filepath.Join(dir, name)
	index := filepath.Join(postDir, "index.md")

	if err := m.Guard.Check(index); err != nil {
		return nil, fmt.Errorf("lifecycle: refusing create: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("slug: " + name + "\n")
	keys := make([]string, 0, len(frontmatter))
	for k := range frontmatter {
		if k == "slug" {
			continue // the slug is pinned to the folder name, not caller-set
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		line, err := yamlLine(k, frontmatter[k])
		if err != nil {
			return nil, fmt.Errorf("lifecycle: encoding frontmatter field %q: %w", k, err)
		}
		sb.WriteString(line)
	}
	sb.WriteString("---\n")
	sb.Write(body)

	if err := m.Guard.MkdirAll(postDir, 0o755); err != nil {
		return nil, fmt.Errorf("lifecycle: creating %s: %w", postDir, err)
	}
	file, err := m.Guard.OpenFile(index, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("lifecycle: creating %s: %w", index, err)
	}
	if _, err := file.Write([]byte(sb.String())); err != nil {
		_ = file.Close()
		_ = m.Guard.Remove(index)
		return nil, fmt.Errorf("lifecycle: writing %s: %w", index, err)
	}
	if err := file.Close(); err != nil {
		_ = m.Guard.Remove(index)
		return nil, fmt.Errorf("lifecycle: writing %s: %w", index, err)
	}
	p, err := posts.Read(index)
	if err != nil {
		return nil, err
	}
	p.State = state
	return p, nil
}

// yamlLine renders one scalar frontmatter line.
func yamlLine(key string, value any) (string, error) {
	data, err := yaml.Marshal(map[string]any{key: value})
	if err != nil {
		return "", err
	}
	return string(data), nil
}
