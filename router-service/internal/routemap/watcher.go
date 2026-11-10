package routemap

import (
	"context"
	"log"
	"path/filepath"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

// Watcher holds the current Map and swaps it atomically whenever the
// underlying file changes. Readers call Current(); they never block on
// a reload and never see a partially-applied map.
//
// The control plane's reconciler writes the map file with a
// temp-file-then-rename (see integration.WriteAtomic), which emits a
// Rename/Create event on the directory rather than a Write on the
// file itself, so Watcher watches the file's directory and filters
// for its name.
type Watcher struct {
	path    string
	current atomic.Pointer[Map]
	fsw     *fsnotify.Watcher
}

// New loads path's initial Map and starts watching it for changes. It
// returns an error only if the initial load fails; a bad reload later
// is logged and the previous Map is kept.
func New(path string) (*Watcher, error) {
	m, err := ParseFile(path)
	if err != nil {
		return nil, err
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fsw.Add(filepath.Dir(path)); err != nil {
		_ = fsw.Close()
		return nil, err
	}

	w := &Watcher{path: path, fsw: fsw}
	w.current.Store(&m)
	return w, nil
}

// Current returns the most recently loaded Map.
func (w *Watcher) Current() Map {
	return *w.current.Load()
}

// Reload re-parses the map file and swaps it in on success. On
// failure it logs and leaves the current Map in place. It is exported
// so a SIGHUP handler can trigger a manual reload alongside the
// fsnotify-driven one.
func (w *Watcher) Reload() {
	m, err := ParseFile(w.path)
	if err != nil {
		log.Printf("routemap: reload %s: keeping previous map: %v", w.path, err)
		return
	}
	w.current.Store(&m)
	log.Printf("routemap: reloaded %s (%d routes)", w.path, len(m))
}

// Run watches for filesystem events affecting path until ctx is
// canceled, calling Reload on every relevant one. It is meant to run
// in its own goroutine.
func (w *Watcher) Run(ctx context.Context) {
	name := filepath.Base(w.path)
	defer func() { _ = w.fsw.Close() }()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if filepath.Base(event.Name) != name {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			w.Reload()
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			log.Printf("routemap: watch error: %v", err)
		}
	}
}
