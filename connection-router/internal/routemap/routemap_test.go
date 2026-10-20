package routemap_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/routemap"
)

func TestParseValid(t *testing.T) {
	input := "alpha.db.mbr.madevara.com 10.0.0.1:21000;\n" +
		"\n" +
		"zeta.db.mbr.madevara.com 10.0.0.2:21001;\n"

	m, err := routemap.Parse(bufio.NewReader(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("len(m) = %d, want 2", len(m))
	}
	if addr, ok := m.Lookup("alpha.db.mbr.madevara.com"); !ok || addr != "10.0.0.1:21000" {
		t.Fatalf("Lookup(alpha) = %q, %v", addr, ok)
	}
	if addr, ok := m.Lookup("zeta.db.mbr.madevara.com"); !ok || addr != "10.0.0.2:21001" {
		t.Fatalf("Lookup(zeta) = %q, %v", addr, ok)
	}
}

func TestParseEmpty(t *testing.T) {
	m, err := routemap.Parse(bufio.NewReader(strings.NewReader("")))
	if err != nil {
		t.Fatalf("Parse(empty): %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("len(m) = %d, want 0", len(m))
	}
}

func TestParseMalformedTooManyFields(t *testing.T) {
	_, err := routemap.Parse(bufio.NewReader(strings.NewReader("alpha.db.mbr.madevara.com 10.0.0.1:21000 extra;\n")))
	if err == nil {
		t.Fatal("Parse with 3 fields: want error, got nil")
	}
}

func TestParseMalformedMissingAddress(t *testing.T) {
	_, err := routemap.Parse(bufio.NewReader(strings.NewReader("alpha.db.mbr.madevara.com\n")))
	if err == nil {
		t.Fatal("Parse with missing address: want error, got nil")
	}
}

func TestLookupMissing(t *testing.T) {
	m, err := routemap.Parse(bufio.NewReader(strings.NewReader("alpha.db.mbr.madevara.com 10.0.0.1:21000;\n")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, ok := m.Lookup("unknown.db.mbr.madevara.com"); ok {
		t.Fatal("Lookup(unknown) = ok, want not found")
	}
}
