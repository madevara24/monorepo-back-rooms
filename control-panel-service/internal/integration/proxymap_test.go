package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/integration"
)

func TestWriteAtomicWritesAndSkipsUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mbr-routes.map")

	changed, err := integration.WriteAtomic(path, []byte("one.db.mbr.madevara.com 10.0.0.1:21000;\n"))
	if err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	if !changed {
		t.Fatalf("changed = false, want true for the first write")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != "one.db.mbr.madevara.com 10.0.0.1:21000;\n" {
		t.Fatalf("file contents = %q", got)
	}

	// No temp files should be left behind in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("dir entries = %d, want 1 (no leftover temp files): %v", len(entries), entries)
	}

	changed, err = integration.WriteAtomic(path, []byte("one.db.mbr.madevara.com 10.0.0.1:21000;\n"))
	if err != nil {
		t.Fatalf("WriteAtomic (unchanged): %v", err)
	}
	if changed {
		t.Fatalf("changed = true, want false when the content is identical")
	}

	changed, err = integration.WriteAtomic(path, []byte("one.db.mbr.madevara.com 10.0.0.1:21999;\n"))
	if err != nil {
		t.Fatalf("WriteAtomic (changed): %v", err)
	}
	if !changed {
		t.Fatalf("changed = false, want true when the content differs")
	}
}

func TestWriteAtomicMissingDirectoryErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist", "mbr-routes.map")
	if _, err := integration.WriteAtomic(path, []byte("x")); err == nil {
		t.Fatalf("WriteAtomic into a missing directory: want error, got nil")
	}
}
