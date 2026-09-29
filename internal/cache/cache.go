// Package cache stores the tool's derived data — first-seen timestamps and
// observed state transitions — under the tool's own directory, outside the
// project.
//
// The cache is disposable by design: the board is fully reconstructible from
// the filesystem, and deleting the cache loses only derived history (how long
// ago a post first appeared, which posts advanced between states), never a
// post or its content.
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Transition records one observed lifecycle move.
type Transition struct {
	Post string    `json:"post"`
	From string    `json:"from"`
	To   string    `json:"to"`
	At   time.Time `json:"at"`
}

// Entry is the derived data for one project.
type Entry struct {
	// FirstSeen maps post name -> first time the tool saw the post.
	FirstSeen map[string]time.Time `json:"firstSeen"`
	// Transitions are observed moves, oldest first.
	Transitions []Transition `json:"transitions"`
}

// Cache is the on-disk derived-data store, keyed by absolute project root.
type Cache struct {
	path     string
	mu       sync.Mutex
	Projects map[string]*Entry
}

// Open loads the cache from the tool's own directory, creating nothing until
// Save. A missing file yields an empty cache.
func Open() (*Cache, error) {
	dir, err := toolDir()
	if err != nil {
		return nil, err
	}
	c := &Cache{
		path:     filepath.Join(dir, "cache.json"),
		Projects: map[string]*Entry{},
	}
	data, err := os.ReadFile(c.path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cache: reading %s: %w", c.path, err)
	}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("cache: parsing %s: %w", c.path, err)
	}
	if c.Projects == nil {
		c.Projects = map[string]*Entry{}
	}
	return c, nil
}

// toolDir returns the tool's own directory (config and derived data), shared
// with the config package's location.
func toolDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cache: locating user config dir: %w", err)
	}
	return filepath.Join(base, "astrogui"), nil
}

// Path exposes the cache file location (used by tests and diagnostics).
func (c *Cache) Path() string { return c.path }

func (c *Cache) lock()   { c.mu.Lock() }
func (c *Cache) unlock() { c.mu.Unlock() }

// entry returns (creating if needed) the derived data for a project.
func (c *Cache) entry(project string) *Entry {
	e, ok := c.Projects[project]
	if !ok {
		e = &Entry{FirstSeen: map[string]time.Time{}}
		c.Projects[project] = e
	}
	if e.FirstSeen == nil {
		e.FirstSeen = map[string]time.Time{}
	}
	return e
}

// FirstSeen returns when the tool first saw the post, recording now when this
// is the first time. The second return is true when the value was newly
// recorded (and the cache is therefore dirty).
func (c *Cache) FirstSeen(project, post string, now time.Time) (time.Time, bool) {
	c.lock()
	defer c.unlock()
	e := c.entry(project)
	if t, ok := e.FirstSeen[post]; ok {
		return t, false
	}
	e.FirstSeen[post] = now
	return now, true
}

// RecordTransition appends an observed move between states.
func (c *Cache) RecordTransition(project, post, from, to string, at time.Time) {
	c.lock()
	defer c.unlock()
	e := c.entry(project)
	e.Transitions = append(e.Transitions, Transition{Post: post, From: from, To: to, At: at})
}

// Transitions returns the observed transitions for a project.
func (c *Cache) Transitions(project string) []Transition {
	c.lock()
	defer c.unlock()
	e, ok := c.Projects[project]
	if !ok {
		return nil
	}
	return append([]Transition(nil), e.Transitions...)
}

// Funnel reports, per state, how many posts currently occupy it and how many
// captured posts the tool has observed advancing into it.
type Funnel struct {
	Counts map[string]int `json:"counts"`
	// Advanced counts, per state, posts observed moving into that state.
	Advanced map[string]int `json:"advanced"`
}

// FunnelFor combines current counts (from a directory listing, so it needs no
// cache at all) with the recorded progression.
func (c *Cache) FunnelFor(project string, currentCounts map[string]int) Funnel {
	f := Funnel{
		Counts:   map[string]int{},
		Advanced: map[string]int{},
	}
	for state, n := range currentCounts {
		f.Counts[state] = n
	}
	for _, tr := range c.Transitions(project) {
		f.Advanced[tr.To]++
	}
	return f
}

// Save writes the cache atomically, creating the tool's own directory as
// needed. This is one of the tool's two writes outside the project, alongside
// its config.
func (c *Cache) Save() error {
	c.lock()
	data, err := json.MarshalIndent(c, "", "  ")
	c.unlock()
	if err != nil {
		return fmt.Errorf("cache: encoding: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return fmt.Errorf("cache: creating %s: %w", filepath.Dir(c.path), err)
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("cache: writing %s: %w", tmp, err)
	}
	return os.Rename(tmp, c.path)
}
