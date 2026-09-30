// Package project locates the Astro project astrogui is run against and
// resolves its content collections. Detection is heuristic by design: the
// project's collection configuration is TypeScript that imports astro:content,
// so it cannot be imported or reliably parsed (see the design's Non-Goals).
package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Project is a resolved Astro project root.
type Project struct {
	Root string
}

// ConfigFiles are the Astro configuration file names, in the directories they
// may occupy at the project root.
var rootConfigFiles = []string{
	"astro.config.mjs", "astro.config.js", "astro.config.ts",
	"astro.config.mts", "astro.config.cts",
}

// Find walks up from start to the nearest enclosing Astro project. Detection
// never modifies anything. When no project is found the error names the
// directory the search started from.
func Find(start string) (*Project, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	searched := dir
	for {
		if isAstroRoot(dir) {
			return &Project{Root: dir}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, fmt.Errorf("no Astro project found in or above %s", searched)
}

func isAstroRoot(dir string) bool {
	for _, name := range rootConfigFiles {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	if pkg, ok := readPackageJSON(dir); ok {
		deps := map[string]bool{}
		for _, m := range []map[string]string{pkg.Dependencies, pkg.DevDependencies} {
			for name := range m {
				deps[name] = true
			}
		}
		if deps["astro"] {
			return true
		}
	}
	return false
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func readPackageJSON(dir string) (packageJSON, bool) {
	var pkg packageJSON
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return pkg, false
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return pkg, false
	}
	return pkg, true
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Collection is a content collection astrogui can manage.
type Collection struct {
	// Name is the collection key in the project's collections config, used to
	// address the collection in the API.
	Name string
	// Dir is the collection's content directory, absolute.
	Dir string
	// Source records how Dir was determined.
	Source string
}

// collectionConfigCandidates are the files Astro looks for collection
// definitions in, relative to the project root.
var collectionConfigCandidates = []string{
	filepath.Join("src", "content.config.ts"),
	filepath.Join("src", "content.config.js"),
	filepath.Join("src", "content.config.mjs"),
	filepath.Join("src", "content", "config.ts"), // legacy layout
	filepath.Join("src", "content", "config.js"),
}

// DetectCollections finds the project's content collections. It reads
// collection config files as text and extracts glob() loaders' base
// directories and the names they are bound to; it never executes or imports
// the project's configuration. The legacy src/content/<name>/ directory-per-
// collection layout is recognized as a fallback.
func DetectCollections(root string) ([]Collection, error) {
	seen := map[string]bool{}
	var cols []Collection

	for _, rel := range collectionConfigCandidates {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		cols = append(cols, parseCollections(root, string(data), rel)...)
	}
	if len(cols) > 0 {
		for _, c := range cols {
			seen[c.Name] = true
		}
		return cols, nil
	}

	// Legacy fallback: src/content/<name>/ directories holding entries.
	contentDir := filepath.Join(root, "src", "content")
	entries, err := os.ReadDir(contentDir)
	if err != nil {
		return nil, fmt.Errorf("project: no content collections found under %s", root)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(contentDir, e.Name())
		if hasMarkdown(dir) {
			name := e.Name()
			if !seen[name] {
				seen[name] = true
				cols = append(cols, Collection{Name: name, Dir: dir, Source: "legacy layout"})
			}
		}
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("project: no content collections found under %s", root)
	}
	return cols, nil
}

func parseCollections(root, src, origin string) []Collection {
	var cols []Collection
	names := collectionNames(src)
	const undefined = "\x00"

	// Find each glob( loader call, then the nearest base: after it. A window
	// bounds the search so a base-less glob cannot swallow the next loader's
	// options; a loader with no base uses "." and points at the project root,
	// which is not a content collection worth managing.
	search := 0
	for {
		i := strings.Index(src[search:], "glob(")
		if i < 0 {
			break
		}
		i += search
		search = i + len("glob(")

		end := min(i+600, len(src))
		m := baseValueRe.FindStringSubmatch(src[i:end])
		if m == nil {
			continue
		}
		base := m[1]

		// The collection name is the nearest preceding defineCollection binding.
		name := undefined
		best := -1
		for at, n := range names {
			if at <= i && at > best {
				best = at
				name = n
			}
		}
		if name == undefined {
			name = filepath.Base(strings.TrimSuffix(filepath.ToSlash(base), "/"))
			if name == "." || name == "/" || name == "" {
				continue
			}
		}
		cols = append(cols, Collection{
			Name:   name,
			Dir:    resolveFrom(root, base),
			Source: origin,
		})
	}
	return cols
}

// collectionNames maps the end offset of each `NAME = defineCollection`
// binding to its name, so the loader block that follows can be attributed.
func collectionNames(src string) map[int]string {
	names := map[int]string{}
	for _, m := range defineCollectionRe.FindAllStringSubmatchIndex(src, -1) {
		names[m[1]] = src[m[2]:m[3]]
	}
	return names
}

var (
	defineCollectionRe = regexp.MustCompile(`(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*defineCollection`)
	baseValueRe        = regexp.MustCompile(`\bbase\s*:\s*["']([^"']+)["']`)
)

// resolveFrom resolves a config-file base path against the project root.
func resolveFrom(root, base string) string {
	// Forward-slash absolutes from the config are intent, not relative dirs
	// (filepath.IsAbs alone is false for "/srv/content" on Windows).
	if path.IsAbs(base) || filepath.IsAbs(base) {
		return filepath.Clean(filepath.FromSlash(base))
	}
	return filepath.Join(root, filepath.FromSlash(base))
}

func hasMarkdown(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && isMarkdown(e.Name()) {
			return true
		}
		// Folder posts: <dir>/<post>/index.md
		if e.IsDir() {
			if fileExists(filepath.Join(dir, e.Name(), "index.md")) {
				return true
			}
		}
	}
	return false
}

func isMarkdown(name string) bool {
	return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".mdx")
}

// ChooseCollection prompts for which collection to manage when several are
// present. It never guesses: without an explicit answer it returns an error
// and no collection is selected.
func ChooseCollection(cols []Collection, in io.Reader, out io.Writer) (Collection, error) {
	if len(cols) == 0 {
		return Collection{}, fmt.Errorf("project: no collections to choose from")
	}
	if len(cols) == 1 {
		return cols[0], nil
	}
	sort.Slice(cols, func(i, j int) bool { return cols[i].Name < cols[j].Name })
	fmt.Fprintln(out, "This project defines several content collections:")
	for i, c := range cols {
		fmt.Fprintf(out, "  %d) %-16s %s\n", i+1, c.Name, relDisplay(c.Dir))
	}
	fmt.Fprint(out, "Which collection should astrogui manage? [1-", len(cols), "] ")

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return Collection{}, fmt.Errorf("project: no collection selected")
	}
	line = strings.TrimSpace(line)
	n := 0
	if _, err := fmt.Sscanf(line, "%d", &n); err != nil || n < 1 || n > len(cols) {
		return Collection{}, fmt.Errorf("project: %q is not a collection choice; no collection selected", line)
	}
	return cols[n-1], nil
}

// ResolveCollection uses a valid configured collection preference, or falls
// back to an explicit choice when the preference is missing or stale. The
// boolean reports whether the preference was honored.
func ResolveCollection(cols []Collection, preferred string, in io.Reader, out io.Writer) (Collection, bool, error) {
	if preferred != "" {
		for _, c := range cols {
			if c.Name == preferred {
				return c, true, nil
			}
		}
	}
	c, err := ChooseCollection(cols, in, out)
	return c, false, err
}

// Managed returns the collection to manage for a project: a user-configured
// content directory takes precedence over the detected one; otherwise the
// detected (and possibly prompted-for) collection is used as-is.
func Managed(root, overrideDir string, detected Collection) Collection {
	if overrideDir == "" {
		return detected
	}
	dir := resolveFrom(root, overrideDir)
	name := detected.Name
	if name == "" {
		name = filepath.Base(dir)
	}
	return Collection{Name: name, Dir: dir, Source: "user override"}
}

func relDisplay(dir string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(dir, home) {
		return "~" + strings.TrimPrefix(dir, home)
	}
	return dir
}
