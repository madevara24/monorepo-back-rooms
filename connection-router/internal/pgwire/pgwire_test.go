package pgwire_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/pgwire"
)

func TestLooksLikeStartupMessage(t *testing.T) {
	cases := []struct {
		name string
		code uint32
		want bool
	}{
		{"protocol 3.0", 196608, true},
		{"protocol 3.2", 3<<16 | 2, true},
		{"SSLRequest code", pgwire.SSLRequestCode, false},
		{"GSSENCRequest code", pgwire.GSSENCRequestCode, false},
		{"CancelRequest code", pgwire.CancelRequestCode, false},
		{"garbage", 0xDEADBEEF, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pgwire.LooksLikeStartupMessage(c.code); got != c.want {
				t.Fatalf("LooksLikeStartupMessage(%d) = %v, want %v", c.code, got, c.want)
			}
		})
	}
}

func TestValidCancelRequestLength(t *testing.T) {
	if !pgwire.ValidCancelRequestLength(16) {
		t.Fatal("ValidCancelRequestLength(16) = false, want true")
	}
	if pgwire.ValidCancelRequestLength(8) {
		t.Fatal("ValidCancelRequestLength(8) = true, want false")
	}
}

func TestSSLRequestWireFormat(t *testing.T) {
	got := pgwire.SSLRequest()
	if len(got) != 8 {
		t.Fatalf("len(SSLRequest()) = %d, want 8", len(got))
	}
	if binary.BigEndian.Uint32(got[0:4]) != 8 {
		t.Fatalf("length field = %d, want 8", binary.BigEndian.Uint32(got[0:4]))
	}
	if binary.BigEndian.Uint32(got[4:8]) != pgwire.SSLRequestCode {
		t.Fatalf("code field = %d, want %d", binary.BigEndian.Uint32(got[4:8]), pgwire.SSLRequestCode)
	}
}

func TestErrorResponseWireFormat(t *testing.T) {
	got := pgwire.ErrorResponse("direct TLS or SSLRequest required")

	if got[0] != 'E' {
		t.Fatalf("message type = %q, want 'E'", got[0])
	}
	length := binary.BigEndian.Uint32(got[1:5])
	if int(length) != len(got)-1 {
		t.Fatalf("length field = %d, want %d (len(got)-1)", length, len(got)-1)
	}
	if !bytes.Contains(got, []byte("FATAL\x00")) {
		t.Fatal("ErrorResponse does not contain severity FATAL")
	}
	if !bytes.Contains(got, []byte("08P01\x00")) {
		t.Fatal("ErrorResponse does not contain SQLSTATE 08P01")
	}
	if !bytes.Contains(got, []byte("direct TLS or SSLRequest required\x00")) {
		t.Fatal("ErrorResponse does not contain the message text")
	}
	if got[len(got)-1] != 0 {
		t.Fatal("ErrorResponse does not end with the field-list terminator")
	}
}
