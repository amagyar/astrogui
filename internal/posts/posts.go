// Package posts is the post model: reading post folders and loose files into
// a byte-exact representation, listing them per lifecycle state, and deriving
// the metadata a filesystem listing cannot show.
//
// A post managed by astrogui is a folder (index.md plus its images). Posts
// the tool did not create — loose .md files in the content directory — are
// listed too, marked read-only, and never rewritten or migrated.
package posts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/astrogui/astrogui/internal/safe"
	"github.com/goccy/go-yaml"
)

// Lifecycle states, in pipeline order. Each state is a directory.
const (
	StateIdeas     = "ideas"
	StateWIP       = "wip"
	StatePublished = "published"
)

// States lists the lifecycle states in order.
var States = []string{StateIdeas, StateWIP, StatePublished}

// Post is one post, folder-shaped or loose, read from disk verbatim.
type Post struct {
	// Name is the post's identity: the folder name for folder posts, the
	// file base name for loose files. It survives moves between states.
	Name string
	// State is the lifecycle state whose directory holds the post.
	State string
	// Dir is the post's absolute directory (the folder for folder posts; the
	// containing directory for loose files).
	Dir string
	// File is the absolute path of the post's markdown file.
	File string
	// Loose marks a post that is a bare .md file rather than a folder.
	Loose bool

	// The file's exact bytes, kept verbatim; fmStart:fmEnd delimit the
	// frontmatter block and bodyStart starts the body, so reassembly is
	// byte-identical by construction.
	raw       []byte
	fmStart   int
	fmEnd     int
	bodyStart int
	hasFM     bool
}

// HasFrontmatter reports whether the post's file opens with a frontmatter
// block.
func (p *Post) HasFrontmatter() bool { return p.hasFM }

// Frontmatter returns the raw frontmatter bytes (between the --- markers),
// or nil when the post has none. The bytes are the file's own.
func (p *Post) Frontmatter() []byte {
	if !p.hasFM {
		return nil
	}
	return p.raw[p.fmStart:p.fmEnd]
}

// FMStart and FMEnd delimit the frontmatter block's content within the file;
// BodyStart is where the body begins. Exposed for exact reassembly.
func (p *Post) FMStart() int   { return p.fmStart }
func (p *Post) FMEnd() int     { return p.fmEnd }
func (p *Post) BodyStart() int { return p.bodyStart }

// Body returns the body bytes verbatim, from just after the frontmatter
// block (or the whole file when there is none).
func (p *Post) Body() []byte { return p.raw[p.bodyStart:] }

// Bytes returns the post file's bytes exactly as read.
func (p *Post) Bytes() []byte { return p.raw }

// Read loads the markdown file at path into a Post.
func Read(path string) (*Post, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("posts: reading %s: %w", path, err)
	}
	p := &Post{
		File: path,
		Dir:  filepath.Dir(path),
		Name: postName(path),
		Loose: !strings.HasSuffix(filepath.Dir(path), string(filepath.Separator)) &&
			filepath.Base(path) != "index.md",
	}
	p.raw = data
	splitFrontmatter(p)
	return p, nil
}

func postName(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "index" {
		return filepath.Base(filepath.Dir(path))
	}
	return base
}

// splitFrontmatter locates a leading `---` delimited frontmatter block,
// recording byte offsets only; the raw bytes are never rewritten.
func splitFrontmatter(p *Post) {
	s := p.raw
	const opener = "---"
	if len(s) < len(opener) || string(s[:len(opener)]) != opener {
		return
	}
	pos := len(opener)
	if pos == len(s) {
		return // file is exactly "---"
	}
	// Line ending style of the opener line governs the whole scan.
	eol := 1
	if s[pos] == '\r' {
		eol = 2
		if len(s) < pos+2 || s[pos+1] != '\n' {
			return
		}
	} else if s[pos] != '\n' {
		return // characters on the delimiter line: not frontmatter
	}

	fmStart := pos + eol
	for lineStart := fmStart; lineStart <= len(s); {
		if lineStart == len(s) {
			return // no closing delimiter: treat whole file as body
		}
		nl := bytesIndexFrom(s, '\n', lineStart)
		var lineEnd int
		if nl < 0 {
			lineEnd = len(s)
		} else {
			lineEnd = nl
		}
		line := string(s[lineStart:lineEnd])
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" || trimmed == "..." {
			p.hasFM = true
			p.fmStart = fmStart
			p.fmEnd = lineStart
			if nl < 0 {
				p.bodyStart = len(s)
			} else {
				p.bodyStart = nl + 1
			}
			return
		}
		if nl < 0 {
			return
		}
		lineStart = nl + 1
	}
}

// bytesIndexFrom finds the first b in s at or after from.
func bytesIndexFrom(s []byte, b byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// bytesHasPrefixLine reports whether b starts with the given line content.
func bytesHasPrefixLine(b []byte, line string) bool {
	return len(b) >= len(line) && string(b[:len(line)]) == line
}

// FrontmatterMap parses the frontmatter into a generic map. Keys the tool
// does not recognise are simply present in the map; nothing is rejected.
func (p *Post) FrontmatterMap() map[string]any {
	out := map[string]any{}
	if !p.hasFM {
		return out
	}
	if err := yaml.Unmarshal(p.Frontmatter(), &out); err != nil {
		return out
	}
	return out
}

// Title returns the post's title: the frontmatter title when present, else
// the post name.
func (p *Post) Title() string {
	if v, ok := p.FrontmatterMap()["title"]; ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return p.Name
}

// Date returns the publication date from frontmatter, or the zero time.
func (p *Post) Date() time.Time {
	v, ok := p.FrontmatterMap()["date"]
	if !ok {
		return time.Time{}
	}
	switch d := v.(type) {
	case time.Time:
		return d
	case string:
		if t, err := time.Parse(time.RFC3339, d); err == nil {
			return t
		}
		if t, err := time.Parse("2006-01-02", d); err == nil {
			return t
		}
	}
	return time.Time{}
}

// DateMissing reports whether the publication date is absent or unparseable.
func (p *Post) DateMissing() bool { return p.Date().IsZero() }

// imageExtensions are the asset types astrogui manages with a post.
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".svg": true, ".avif": true,
}

// Assets lists the image files in the post's own directory.
func (p *Post) Assets() []string {
	entries, err := os.ReadDir(p.Dir)
	if err != nil {
		return nil
	}
	var assets []string
	for _, e := range entries {
		if e.IsDir() || !imageExtensions[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		assets = append(assets, e.Name())
	}
	sort.Strings(assets)
	return assets
}

var (
	markdownImageRe = regexp.MustCompile(`!\[[^\]]*\]\(\s*([^)\s]+)[^)]*\)`)
	htmlImageRe     = regexp.MustCompile(`<img[^>]*\bsrc\s*=\s*["']([^"']+)["']`)
)

// ImageRefs returns the image references found in the body, in order. Both
// markdown and HTML forms are recognised; the tool does not rewrite them, it
// only checks they resolve.
func (p *Post) ImageRefs() []string {
	body := string(p.Body())
	var refs []string
	for _, m := range markdownImageRe.FindAllStringSubmatch(body, -1) {
		refs = append(refs, m[1])
	}
	for _, m := range htmlImageRe.FindAllStringSubmatch(body, -1) {
		refs = append(refs, m[1])
	}
	return refs
}

// Meta is the derived per-post metadata the board shows.
type Meta struct {
	// FirstSeen is when the post first appeared (from the derived cache,
	// falling back to the filesystem).
	FirstSeen time.Time `json:"firstSeen"`
	// LastModified is the markdown file's modification time.
	LastModified time.Time `json:"lastModified"`
	// Size is the body length in bytes.
	Size int `json:"size"`
	// Images is the number of image references in the body.
	Images int `json:"images"`
	// Title is the display title.
	Title string `json:"title"`
	// Stalled is set by the board when LastModified is older than the
	// configured threshold.
	Stalled bool `json:"stalled"`
}

// DeriveMeta computes the derived metadata for a post. firstSeen comes from
// the derived cache when known; the fallback is the file's own timestamps,
// which keeps the board reconstructible when the cache is deleted.
func DeriveMeta(p *Post, firstSeen time.Time) Meta {
	info, err := os.Stat(p.File)
	now := time.Now()
	meta := Meta{
		FirstSeen: firstSeen,
		Size:      len(p.Body()),
		Images:    len(p.ImageRefs()),
		Title:     p.Title(),
	}
	if err == nil {
		meta.LastModified = info.ModTime()
		if firstSeen.IsZero() {
			meta.FirstSeen = info.ModTime() // best filesystem fallback
		}
	} else {
		meta.LastModified = now
		if firstSeen.IsZero() {
			meta.FirstSeen = now
		}
	}
	return meta
}

// Listing is the board's source of truth: the posts present in each state's
// directory, read from the filesystem with no stored state involved.
type Listing struct {
	Ideas     []*Post `json:"ideas"`
	WIP       []*Post `json:"wip"`
	Published []*Post `json:"published"`
}

// In returns the posts in a state.
func (l Listing) In(state string) []*Post {
	switch state {
	case StateIdeas:
		return l.Ideas
	case StateWIP:
		return l.WIP
	case StatePublished:
		return l.Published
	}
	return nil
}

// All returns every post across states.
func (l Listing) All() []*Post {
	var all []*Post
	for _, state := range States {
		all = append(all, l.In(state)...)
	}
	return all
}

// List reads every lifecycle directory. Folder posts (a directory containing
// index.md) are managed posts; loose .md files are listed read-only and are
// never rewritten.
func List(ideasDir, wipDir, contentDir string) (Listing, error) {
	var l Listing
	var err error
	if l.Ideas, err = listDir(StateIdeas, ideasDir); err != nil {
		return l, err
	}
	if l.WIP, err = listDir(StateWIP, wipDir); err != nil {
		return l, err
	}
	if l.Published, err = listDir(StatePublished, contentDir); err != nil {
		return l, err
	}
	return l, nil
}

func listDir(state, dir string) ([]*Post, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("posts: listing %s: %w", dir, err)
	}
	var out []*Post
	for _, e := range entries {
		if e.IsDir() {
			index := filepath.Join(dir, e.Name(), "index.md")
			if _, err := os.Stat(index); err != nil {
				continue // not a post folder
			}
			p, err := Read(index)
			if err != nil {
				continue // unreadable posts are skipped, not fatal
			}
			p.State = state
			p.Loose = false
			out = append(out, p)
			continue
		}
		if !isMarkdownFile(e.Name()) {
			continue
		}
		p, err := Read(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		p.State = state
		p.Loose = true
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ConflictError reports a save against a file that changed on disk after the
// editor loaded it. The on-disk version is preserved and surfaced; nothing is
// overwritten.
type ConflictError struct {
	Path       string
	Current    []byte    // the on-disk bytes that won
	CurrentMod time.Time // the on-disk modification time
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("posts: %s changed on disk after it was opened; the on-disk version is preserved", e.Path)
}

// SaveBody writes a new body into the post file, byte-exactly.
//
//   - When the body is unchanged, the file is not rewritten, so its
//     modification time is untouched.
//   - When the file changed on disk after it was loaded (expectedModTime
//     mismatch), the save is refused with a ConflictError carrying the
//     on-disk version; the tool never silently discards an external edit.
//
// The write is atomic (temp file plus rename in the same directory).
func SaveBody(path string, body []byte, expectedModTime time.Time) error {
	return SaveBodyGuarded(nil, path, body, expectedModTime)
}

// SaveBodyGuarded is SaveBody with replacement performed through guard's
// anchored managed-directory handle.
func SaveBodyGuarded(guard *safe.Guard, path string, body []byte, expectedModTime time.Time) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("posts: reading %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("posts: stating %s: %w", path, err)
	}
	if !expectedModTime.IsZero() && !info.ModTime().Equal(expectedModTime) {
		return &ConflictError{Path: path, Current: current, CurrentMod: info.ModTime()}
	}

	p, err := Read(path)
	if err != nil {
		return err
	}
	if bytesEqual(p.Body(), body) {
		return nil // unchanged: no rewrite, mtime untouched
	}

	out := append([]byte(nil), p.raw[:p.bodyStart]...)
	out = append(out, body...)
	return writeFileAtomic(guard, path, out)
}

// WriteFile is the guarded entry the server uses for raw whole-file saves
// (the raw frontmatter view). Same conflict semantics as SaveBody.
func WriteFile(path string, content, expected []byte, expectedModTime time.Time) error {
	return WriteFileGuarded(nil, path, content, expected, expectedModTime)
}

// WriteFileGuarded is WriteFile with replacement performed through guard's
// anchored managed-directory handle.
func WriteFileGuarded(guard *safe.Guard, path string, content, expected []byte, expectedModTime time.Time) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("posts: reading %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("posts: stating %s: %w", path, err)
	}
	if !expectedModTime.IsZero() && !info.ModTime().Equal(expectedModTime) {
		return &ConflictError{Path: path, Current: current, CurrentMod: info.ModTime()}
	}
	if bytesEqual(current, content) {
		return nil
	}
	return writeFileAtomic(guard, path, content)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// writeFileAtomic replaces path's content via a temp file in the same
// directory followed by a rename, so a crash never leaves a partial post.
func writeFileAtomic(guard *safe.Guard, path string, content []byte) error {
	if guard != nil {
		if err := guard.AtomicWriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("posts: atomically writing %s: %w", path, err)
		}
		return nil
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".astrogui-save-*")
	if err != nil {
		return fmt.Errorf("posts: creating temp file in %s: %w", dir, err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("posts: writing %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("posts: closing %s: %w", name, err)
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return fmt.Errorf("posts: chmod %s: %w", name, err)
	}
	return os.Rename(name, path)
}

func isMarkdownFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".mdx"
}
