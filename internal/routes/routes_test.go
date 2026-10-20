package routes_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/internal/routes"
)

func TestRenderSortsAndFormatsLines(t *testing.T) {
	got := routes.Render([]routes.Route{
		{Slug: "zeta", Host: "10.0.0.2", Port: 21001},
		{Slug: "alpha", Host: "10.0.0.1", Port: 21000},
	})
	want := "alpha.db.mbr.madevara.com 10.0.0.1:21000;\n" +
		"zeta.db.mbr.madevara.com 10.0.0.2:21001;\n"
	if string(got) != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
}

func TestRenderEmpty(t *testing.T) {
	got := routes.Render(nil)
	if len(got) != 0 {
		t.Fatalf("Render(nil) = %q, want empty", got)
	}
}

func TestWriteAtomicWritesAndSkipsUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mbr-routes.map")

	changed, err := routes.WriteAtomic(path, []byte("one.db.mbr.madevara.com 10.0.0.1:21000;\n"))
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

	changed, err = routes.WriteAtomic(path, []byte("one.db.mbr.madevara.com 10.0.0.1:21000;\n"))
	if err != nil {
		t.Fatalf("WriteAtomic (unchanged): %v", err)
	}
	if changed {
		t.Fatalf("changed = true, want false when the content is identical")
	}

	changed, err = routes.WriteAtomic(path, []byte("one.db.mbr.madevara.com 10.0.0.1:21999;\n"))
	if err != nil {
		t.Fatalf("WriteAtomic (changed): %v", err)
	}
	if !changed {
		t.Fatalf("changed = false, want true when the content differs")
	}
}

func TestWriteAtomicMissingDirectoryErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist", "mbr-routes.map")
	if _, err := routes.WriteAtomic(path, []byte("x")); err == nil {
		t.Fatalf("WriteAtomic into a missing directory: want error, got nil")
	}
}

func TestConnectionString(t *testing.T) {
	got := routes.ConnectionString("demo-ab12", "s3cr3t")
	want := "postgres://postgres:s3cr3t@demo-ab12.db.mbr.madevara.com:5432/postgres?sslmode=verify-full&sslnegotiation=direct&sslrootcert=system"
	if got != want {
		t.Fatalf("ConnectionString = %q, want %q", got, want)
	}
}

func TestJDBCConnectionString(t *testing.T) {
	got := routes.JDBCConnectionString("demo-ab12")
	want := "jdbc:postgresql://demo-ab12.db.mbr.madevara.com:5432/postgres?sslmode=require&sslNegotiation=direct"
	if got != want {
		t.Fatalf("JDBCConnectionString = %q, want %q", got, want)
	}
}

func TestPublicHost(t *testing.T) {
	if got := routes.PublicHost("demo-ab12"); got != "demo-ab12.db.mbr.madevara.com" {
		t.Fatalf("PublicHost = %q", got)
	}
}
