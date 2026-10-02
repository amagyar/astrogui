// Package watch watches the managed directories and emits change events for
// create, modify, rename, and delete, so the board tracks the filesystem —
// including changes made outside the tool — while it runs.
package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Op names a kind of change.
type Op string

const (
	Create Op = "create"
	Modify Op = "modify"
	Rename Op = "rename"
	Delete Op = "delete"
)

// Event is one observed change under a watched directory.
type Event struct {
	Op   Op     `json:"op"`
	Path string `json:"path"`
	Dir  string `json:"dir"` // the watched root the path is under
}

// String renders the event for logs.
func (e Event) String() string {
	return fmt.Sprintf("%s %s", e.Op, e.Path)
}

// Watcher emits debounced change events from the managed directories. It is
// advisory by design: consumers re-read the filesystem rather than trusting
// event payloads, so no change is ever overwritten from stored state.
type Watcher struct {
	fw      *fsnotify.Watcher
	out     chan Event
	roots   []string
	mu      sync.Mutex
	closed  bool
	pending map[string]Op // path -> op, debounced
	timer   *time.Timer
}

// New watches the given directories recursively.
func New(dirs ...string) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("watch: %w", err)
	}
	w := &Watcher{
		fw:      fw,
		out:     make(chan Event, 64),
		roots:   append([]string(nil), dirs...),
		pending: map[string]Op{},
	}
	for _, dir := range dirs {
		if err := w.watchTree(dir); err != nil {
			fw.Close()
			return nil, err
		}
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) watchTree(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return w.fw.Add(path)
		}
		return nil
	})
}

// rootFor returns the watched root containing path. A path outside every
// root yields "", including siblings that share a prefix: filepath.Rel
// reports them as "../name", which is not containment.
func (w *Watcher) rootFor(path string) string {
	best := ""
	for _, root := range w.roots {
		if rel, err := filepath.Rel(root, path); err == nil && rel != ".." &&
			!strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if len(root) > len(best) {
				best = root
			}
		}
	}
	return best
}

func (w *Watcher) loop() {
	debounce := time.NewTimer(time.Hour)
	debounce.Stop()
	for {
		select {
		case ev, ok := <-w.fw.Events:
			if !ok {
				w.flush()
				w.closeOut()
				return
			}
			w.handle(ev)
			if !debounce.Stop() {
				select {
				case <-debounce.C:
				default:
				}
			}
			debounce.Reset(60 * time.Millisecond)
		case err, ok := <-w.fw.Errors:
			if !ok {
				w.flush()
				w.closeOut()
				return
			}
			_ = err // watching is advisory; errors must not stop the feed
		case <-debounce.C:
			w.flush()
		}
	}
}

// handle maps an fsnotify event onto a tool Op. New directories get watches
// so posts created after startup are covered, and their existing entries are
// recorded too: the entries may have appeared before the watch attached.
func (w *Watcher) handle(ev fsnotify.Event) {
	if ev.Has(fsnotify.Chmod) {
		// A metadata-only change (mtime, permissions) is a modify for this
		// feed: the board's time signals depend on modification times, and
		// consumers re-read the filesystem rather than event payloads.
		w.record(ev.Name, Modify)
		return
	}
	if ev.Has(fsnotify.Create) {
		w.record(ev.Name, Create)
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			_ = w.watchTree(ev.Name)
			if entries, err := os.ReadDir(ev.Name); err == nil {
				for _, e := range entries {
					w.record(filepath.Join(ev.Name, e.Name()), Create)
				}
			}
		}
		return
	}
	switch {
	case ev.Has(fsnotify.Remove):
		w.record(ev.Name, Delete)
	case ev.Has(fsnotify.Rename):
		w.record(ev.Name, Rename)
	default:
		w.record(ev.Name, Modify)
	}
}

// record keeps the strongest op per path for the current debounce window.
// A single os.WriteFile is an open followed by a write: fsnotify reports
// CREATE then WRITE for it, and both land in one window, so a weaker later
// op must never downgrade an earlier stronger one.
func (w *Watcher) record(path string, op Op) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	prev, ok := w.pending[path]
	if ok && prev == Delete && (op == Modify || op == Create) {
		op = Create // something replaced a deleted path
	} else if ok && opRank(prev) > opRank(op) {
		op = prev
	}
	w.pending[path] = op
}

// opRank orders ops by how much they tell consumers: a removal beats
// everything, a rename (path gone from the old name) beats a create, and a
// create beats a plain modify. The delete-then-replace case is handled
// separately in record.
func opRank(op Op) int {
	switch op {
	case Delete:
		return 3
	case Rename:
		return 2
	case Create:
		return 1
	default:
		return 0
	}
}

func (w *Watcher) flush() {
	w.mu.Lock()
	events := make([]Event, 0, len(w.pending))
	for path, op := range w.pending {
		root := w.rootFor(path)
		if root == "" {
			continue // outside the managed directories
		}
		events = append(events, Event{Op: op, Path: path, Dir: root})
	}
	w.pending = map[string]Op{}
	w.mu.Unlock()
	for _, e := range events {
		select {
		case w.out <- e:
		default: // drop rather than block: advisory feed
		}
	}
}

func (w *Watcher) closeOut() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	close(w.out)
}

// Events returns the change feed.
func (w *Watcher) Events() <-chan Event { return w.out }

// Close stops watching. The channel is closed after any remaining events are
// flushed.
func (w *Watcher) Close() error {
	return w.fw.Close()
}
