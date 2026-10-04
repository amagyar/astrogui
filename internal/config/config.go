// Package config defines the tool's own configuration, stored outside the
// project it manages (the write boundary allows the tool's own config and
// derived data, and nothing else, outside the project).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"
)

// DefaultStaleness is how long a post may go unchanged before the board marks
// it stalled.
const DefaultStaleness = 30 * 24 * time.Hour

// DefaultDevURL is the dev-server base URL used when none is configured:
// Astro's conventional dev-server address.
const DefaultDevURL = "http://localhost:4321"

// Project is the per-project configuration. The zero value plus FillDefaults
// yields the conventional layout; persisted fields are overrides only.
type Project struct {
	// IdeasDir is the ideas draft directory, relative to the project root
	// unless absolute. Default "drafts/ideas".
	IdeasDir string `json:"ideasDir,omitempty"`
	// WipDir is the in-progress draft directory. Default "drafts/wip".
	WipDir string `json:"wipDir,omitempty"`
	// TrashDir is the discard directory: posts removed from the board move
	// here instead of being deleted. Default "drafts/trash".
	TrashDir string `json:"trashDir,omitempty"`
	// ContentDir overrides the detected content collection directory.
	// Empty means "use the detected directory".
	ContentDir string `json:"contentDir,omitempty"`
	// Staleness is the stalled-post threshold. Zero means DefaultStaleness.
	Staleness time.Duration `json:"staleness,omitempty"`
	// Collection names the managed collection when more than one exists.
	Collection string `json:"collection,omitempty"`
	// DevURL overrides the dev-server base URL for the editor's dev-server
	// bridge. Empty means Astro's conventional address.
	DevURL string `json:"devUrl,omitempty"`
}

// File is the on-disk configuration file: per-project overrides keyed by
// absolute project root.
type File struct {
	Projects map[string]Project `json:"projects"`
}

// FillDefaults returns p with unset fields replaced by their defaults.
func (p Project) FillDefaults() Project {
	if p.IdeasDir == "" {
		p.IdeasDir = "drafts/ideas"
	}
	if p.WipDir == "" {
		p.WipDir = "drafts/wip"
	}
	if p.TrashDir == "" {
		p.TrashDir = "drafts/trash"
	}
	if p.Staleness <= 0 {
		p.Staleness = DefaultStaleness
	}
	return p
}

// EffectiveStaleness returns the stalled threshold for p.
func (p Project) EffectiveStaleness() time.Duration {
	if p.Staleness <= 0 {
		return DefaultStaleness
	}
	return p.Staleness
}

// EffectiveDevURL returns the dev-server base URL for p, defaulting to
// Astro's conventional address when none is configured.
func (p Project) EffectiveDevURL() string {
	if p.DevURL == "" {
		return DefaultDevURL
	}
	return p.DevURL
}

// ResolveDir resolves a configured directory against the project root,
// treating absolute paths as already resolved.
func ResolveDir(root, dir string) string {
	// Config files may carry forward-slash absolute paths; on Windows those
	// are not filepath.IsAbs, but they are unambiguous user intent, never a
	// project-relative dir.
	if path.IsAbs(dir) || filepath.IsAbs(dir) {
		return filepath.Clean(filepath.FromSlash(dir))
	}
	return filepath.Join(root, dir)
}

// IdeasPath returns the absolute ideas directory for project root.
func (p Project) IdeasPath(root string) string {
	return ResolveDir(root, p.FillDefaults().IdeasDir)
}

// WipPath returns the absolute in-progress directory for project root.
func (p Project) WipPath(root string) string {
	return ResolveDir(root, p.FillDefaults().WipDir)
}

// TrashPath returns the absolute trash directory for project root.
func (p Project) TrashPath(root string) string {
	return ResolveDir(root, p.FillDefaults().TrashDir)
}

// ContentPath returns the absolute content directory override for project
// root, or "" when the detected directory should be used.
func (p Project) ContentPath(root string) string {
	if p.ContentDir == "" {
		return ""
	}
	return ResolveDir(root, p.ContentDir)
}

// Path returns the location of the tool's configuration file.
func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: locating user config dir: %w", err)
	}
	return filepath.Join(base, "astrogui", "config.json"), nil
}

// Dir returns the tool's own directory (config plus derived data).
func Dir() (string, error) {
	p, err := Path()
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

// Load reads the configuration file. A missing file is not an error: it yields
// an empty File whose per-project lookups fall back to defaults.
func Load() (File, error) {
	var f File
	p, err := Path()
	if err != nil {
		return f, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, fmt.Errorf("config: reading %s: %w", p, err)
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("config: parsing %s: %w", p, err)
	}
	if f.Projects == nil {
		f.Projects = map[string]Project{}
	}
	return f, nil
}

// For returns the configuration for a project root, with defaults filled.
func (f File) For(root string) Project {
	if f.Projects == nil {
		f.Projects = map[string]Project{}
	}
	return f.Projects[root].FillDefaults()
}

// Set records the configuration for a project root.
func (f *File) Set(root string, p Project) {
	if f.Projects == nil {
		f.Projects = map[string]Project{}
	}
	f.Projects[root] = p
}

// Save writes the configuration file atomically, creating the tool's own
// directory as needed.
func (f File) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("config: creating %s: %w", filepath.Dir(p), err)
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("config: encoding: %w", err)
	}
	data = append(data, '\n')
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("config: writing %s: %w", tmp, err)
	}
	return os.Rename(tmp, p)
}
