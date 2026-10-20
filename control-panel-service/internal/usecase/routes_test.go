package usecase_test

import (
	"testing"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

func TestRenderSortsAndFormatsLines(t *testing.T) {
	got := usecase.Render([]usecase.Route{
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
	got := usecase.Render(nil)
	if len(got) != 0 {
		t.Fatalf("Render(nil) = %q, want empty", got)
	}
}

func TestConnectionString(t *testing.T) {
	got := usecase.ConnectionString("demo-ab12", "s3cr3t")
	want := "postgres://postgres:s3cr3t@demo-ab12.db.mbr.madevara.com:5432/postgres?sslmode=verify-full&sslnegotiation=direct&sslrootcert=system"
	if got != want {
		t.Fatalf("ConnectionString = %q, want %q", got, want)
	}
}

func TestJDBCConnectionString(t *testing.T) {
	got := usecase.JDBCConnectionString("demo-ab12")
	want := "jdbc:postgresql://demo-ab12.db.mbr.madevara.com:5432/postgres?sslmode=require&sslNegotiation=direct"
	if got != want {
		t.Fatalf("JDBCConnectionString = %q, want %q", got, want)
	}
}

func TestPublicHost(t *testing.T) {
	if got := usecase.PublicHost("demo-ab12"); got != "demo-ab12.db.mbr.madevara.com" {
		t.Fatalf("PublicHost = %q", got)
	}
}
