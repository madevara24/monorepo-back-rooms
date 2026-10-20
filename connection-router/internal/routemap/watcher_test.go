package routemap_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/routemap"
)

// writeAtomic mirrors control-panel-service's integration.WriteAtomic:
// a temp file in the same directory, then a rename over the target.
// The watcher must react to this rename, not just to in-place writes.
func writeAtomic(t *testing.T, path string, data []byte) {
	t.Helper()
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mbr-routes-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := tmp.Write(data); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		t.Fatalf("rename into place: %v", err)
	}
}

func awaitRouteCount(t *testing.T, w *routemap.Watcher, want int) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if len(w.Current()) == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for the watched map to have %d routes (has %d)", want, len(w.Current()))
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestWatcherReloadsOnAtomicRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mbr-routes.map")
	writeAtomic(t, path, []byte("alpha.db.mbr.madevara.com 10.0.0.1:21000;\n"))

	w, err := routemap.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	if addr, ok := w.Current().Lookup("alpha.db.mbr.madevara.com"); !ok || addr != "10.0.0.1:21000" {
		t.Fatalf("initial Current() = %q, %v", addr, ok)
	}

	writeAtomic(t, path, []byte(
		"alpha.db.mbr.madevara.com 10.0.0.1:21000;\n"+
			"beta.db.mbr.madevara.com 10.0.0.2:21001;\n"))
	awaitRouteCount(t, w, 2)

	if addr, ok := w.Current().Lookup("beta.db.mbr.madevara.com"); !ok || addr != "10.0.0.2:21001" {
		t.Fatalf("reloaded Current() missing beta: %q, %v", addr, ok)
	}
}

func TestWatcherKeepsOldMapOnParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mbr-routes.map")
	writeAtomic(t, path, []byte("alpha.db.mbr.madevara.com 10.0.0.1:21000;\n"))

	w, err := routemap.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	writeAtomic(t, path, []byte("this is not a valid route line\n"))

	// Give the watcher time to observe and reject the bad write, then
	// confirm the good map from startup is still what Current() serves.
	time.Sleep(200 * time.Millisecond)
	if addr, ok := w.Current().Lookup("alpha.db.mbr.madevara.com"); !ok || addr != "10.0.0.1:21000" {
		t.Fatalf("Current() after a bad reload = %q, %v, want the previous good map kept", addr, ok)
	}
}

func TestNewFailsOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := routemap.New(filepath.Join(dir, "does-not-exist.map")); err == nil {
		t.Fatal("New on a missing file: want error, got nil")
	}
}
