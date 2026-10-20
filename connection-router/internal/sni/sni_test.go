package sni_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/sni"
)

// buildClientHello assembles a minimal (but structurally valid)
// ClientHello handshake body carrying serverName in its SNI
// extension, if serverName is non-empty.
func buildClientHello(serverName string) []byte {
	var body bytes.Buffer
	body.Write([]byte{0x03, 0x03})             // client_version: TLS 1.2
	body.Write(make([]byte, 32))               // random
	body.WriteByte(0)                          // session_id length
	body.Write([]byte{0x00, 0x02, 0x13, 0x01}) // cipher_suites: len 2, one suite
	body.Write([]byte{0x01, 0x00})             // compression_methods: len 1, null

	var extensions bytes.Buffer
	if serverName != "" {
		var sniExt bytes.Buffer
		var nameList bytes.Buffer
		nameList.WriteByte(0x00) // host_name
		nameList.Write([]byte{byte(len(serverName) >> 8), byte(len(serverName))})
		nameList.WriteString(serverName)

		sniExt.Write([]byte{byte(nameList.Len() >> 8), byte(nameList.Len())})
		sniExt.Write(nameList.Bytes())

		extensions.Write([]byte{0x00, 0x00}) // extension type: server_name
		extensions.Write([]byte{byte(sniExt.Len() >> 8), byte(sniExt.Len())})
		extensions.Write(sniExt.Bytes())
	}

	if extensions.Len() > 0 {
		body.Write([]byte{byte(extensions.Len() >> 8), byte(extensions.Len())})
		body.Write(extensions.Bytes())
	}

	var hs bytes.Buffer
	hs.WriteByte(0x01) // handshake type: ClientHello
	l := body.Len()
	hs.Write([]byte{byte(l >> 16), byte(l >> 8), byte(l)})
	hs.Write(body.Bytes())
	return hs.Bytes()
}

// wrapRecord wraps a handshake-protocol payload in one TLS record.
func wrapRecord(payload []byte) []byte {
	var rec bytes.Buffer
	rec.WriteByte(0x16) // handshake
	rec.Write([]byte{0x03, 0x01})
	rec.Write([]byte{byte(len(payload) >> 8), byte(len(payload))})
	rec.Write(payload)
	return rec.Bytes()
}

func TestReadClientHelloExtractsSNI(t *testing.T) {
	hs := buildClientHello("demo-ab12.db.mbr.madevara.com")
	record := wrapRecord(hs)

	got, err := sni.ReadClientHello(bytes.NewReader(record))
	if err != nil {
		t.Fatalf("ReadClientHello: %v", err)
	}
	if got.ServerName != "demo-ab12.db.mbr.madevara.com" {
		t.Fatalf("ServerName = %q", got.ServerName)
	}
	if !bytes.Equal(got.Raw, record) {
		t.Fatalf("Raw = %d bytes, want the exact %d bytes read", len(got.Raw), len(record))
	}
}

func TestReadClientHelloFragmentedAcrossRecords(t *testing.T) {
	hs := buildClientHello("fragmented.db.mbr.madevara.com")
	mid := len(hs) / 2
	var stream bytes.Buffer
	stream.Write(wrapRecord(hs[:mid]))
	stream.Write(wrapRecord(hs[mid:]))

	got, err := sni.ReadClientHello(bytes.NewReader(stream.Bytes()))
	if err != nil {
		t.Fatalf("ReadClientHello: %v", err)
	}
	if got.ServerName != "fragmented.db.mbr.madevara.com" {
		t.Fatalf("ServerName = %q", got.ServerName)
	}
	if !bytes.Equal(got.Raw, stream.Bytes()) {
		t.Fatalf("Raw does not match the exact bytes read across both records")
	}
}

func TestReadClientHelloMissingSNI(t *testing.T) {
	hs := buildClientHello("")
	record := wrapRecord(hs)

	_, err := sni.ReadClientHello(bytes.NewReader(record))
	if !errors.Is(err, sni.ErrNoServerName) {
		t.Fatalf("err = %v, want ErrNoServerName", err)
	}
}

func TestReadClientHelloMalformedTruncated(t *testing.T) {
	hs := buildClientHello("truncated.db.mbr.madevara.com")
	record := wrapRecord(hs)
	truncated := record[:len(record)-5]

	_, err := sni.ReadClientHello(bytes.NewReader(truncated))
	if err == nil {
		t.Fatal("ReadClientHello on a truncated stream: want error, got nil")
	}
}

func TestReadClientHelloNotAHandshakeRecord(t *testing.T) {
	var rec bytes.Buffer
	rec.WriteByte(0x17) // application_data, not handshake
	rec.Write([]byte{0x03, 0x01})
	rec.Write([]byte{0x00, 0x01})
	rec.WriteByte(0x00)

	_, err := sni.ReadClientHello(bytes.NewReader(rec.Bytes()))
	if !errors.Is(err, sni.ErrNotHandshake) {
		t.Fatalf("err = %v, want ErrNotHandshake", err)
	}
}

func TestReadClientHelloTooLarge(t *testing.T) {
	huge := make([]byte, 0xffff)
	var rec bytes.Buffer
	rec.WriteByte(0x16)
	rec.Write([]byte{0x03, 0x01})
	rec.Write([]byte{0xff, 0xff}) // record length 65535, over MaxClientHello
	rec.Write(huge)

	_, err := sni.ReadClientHello(bytes.NewReader(rec.Bytes()))
	if !errors.Is(err, sni.ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}
