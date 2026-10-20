package proxy_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/pgwire"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/proxy"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/routemap"
)

// staticRoutes adapts a plain routemap.Map to proxy.RouteSource for
// tests that don't need a live-reloading map.
type staticRoutes routemap.Map

func (s staticRoutes) Current() routemap.Map { return routemap.Map(s) }

func newTestServer(routes routemap.Map) (*proxy.Server, string, func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	s := &proxy.Server{
		Routes:           staticRoutes(routes),
		Metrics:          &proxy.Metrics{},
		DialTimeout:      2 * time.Second,
		IdleTimeout:      2 * time.Second,
		HandshakeTimeout: 2 * time.Second,
		MaxConnections:   16,
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = s.Serve(ctx, ln) }()
	return s, ln.Addr().String(), func() { cancel(); _ = ln.Close() }
}

// startBackend runs a fake tenant on 127.0.0.1:0 and returns its
// "host:port" dial address. onConn handles each accepted connection
// and must close it when done.
func startBackend(t *testing.T, onConn func(net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go onConn(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String()
}

// directTLSBackend expects the raw ClientHello bytes with no
// SSLRequest primer (the sslnegotiation=direct path), then echoes.
func directTLSBackend(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	_, _ = conn.Write(buf[:n])
	_, _ = io.Copy(conn, conn)
}

// sslPrimedBackend expects the router to re-issue an 8-byte
// SSLRequest, replies 'S', then echoes whatever arrives after.
func sslPrimedBackend(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	var hdr [8]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return
	}
	if binary.BigEndian.Uint32(hdr[4:8]) != pgwire.SSLRequestCode {
		return
	}
	if _, err := conn.Write([]byte("S")); err != nil {
		return
	}
	_, _ = io.Copy(conn, conn)
}

func buildClientHello(serverName string) []byte {
	var body bytes.Buffer
	body.Write([]byte{0x03, 0x03})
	body.Write(make([]byte, 32))
	body.WriteByte(0)
	body.Write([]byte{0x00, 0x02, 0x13, 0x01})
	body.Write([]byte{0x01, 0x00})

	var extensions bytes.Buffer
	if serverName != "" {
		var nameList bytes.Buffer
		nameList.WriteByte(0x00)
		nameList.Write([]byte{byte(len(serverName) >> 8), byte(len(serverName))})
		nameList.WriteString(serverName)

		var sniExt bytes.Buffer
		sniExt.Write([]byte{byte(nameList.Len() >> 8), byte(nameList.Len())})
		sniExt.Write(nameList.Bytes())

		extensions.Write([]byte{0x00, 0x00})
		extensions.Write([]byte{byte(sniExt.Len() >> 8), byte(sniExt.Len())})
		extensions.Write(sniExt.Bytes())
	}
	if extensions.Len() > 0 {
		body.Write([]byte{byte(extensions.Len() >> 8), byte(extensions.Len())})
		body.Write(extensions.Bytes())
	}

	var hs bytes.Buffer
	hs.WriteByte(0x01)
	l := body.Len()
	hs.Write([]byte{byte(l >> 16), byte(l >> 8), byte(l)})
	hs.Write(body.Bytes())

	var rec bytes.Buffer
	rec.WriteByte(0x16)
	rec.Write([]byte{0x03, 0x01})
	rec.Write([]byte{byte(hs.Len() >> 8), byte(hs.Len())})
	rec.Write(hs.Bytes())
	return rec.Bytes()
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestDirectTLSRoutesAndSplices(t *testing.T) {
	backendAddr := startBackend(t, directTLSBackend)
	_, addr, stop := newTestServer(routemap.Map{"tenant.db.mbr.madevara.com": backendAddr})
	defer stop()

	conn := dial(t, addr)
	hello := buildClientHello("tenant.db.mbr.madevara.com")
	if _, err := conn.Write(hello); err != nil {
		t.Fatalf("write ClientHello: %v", err)
	}

	got := make([]byte, len(hello))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if !bytes.Equal(got, hello) {
		t.Fatalf("echoed bytes do not match the ClientHello sent")
	}
}

func TestSSLRequestRoutesAndSplices(t *testing.T) {
	backendAddr := startBackend(t, sslPrimedBackend)
	_, addr, stop := newTestServer(routemap.Map{"tenant.db.mbr.madevara.com": backendAddr})
	defer stop()

	conn := dial(t, addr)
	var req [8]byte
	binary.BigEndian.PutUint32(req[0:4], 8)
	binary.BigEndian.PutUint32(req[4:8], pgwire.SSLRequestCode)
	if _, err := conn.Write(req[:]); err != nil {
		t.Fatalf("write SSLRequest: %v", err)
	}

	var resp [1]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		t.Fatalf("read SSLRequest reply: %v", err)
	}
	if resp[0] != 'S' {
		t.Fatalf("SSLRequest reply = %q, want 'S'", resp[0])
	}

	hello := buildClientHello("tenant.db.mbr.madevara.com")
	if _, err := conn.Write(hello); err != nil {
		t.Fatalf("write ClientHello: %v", err)
	}

	got := make([]byte, len(hello))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if !bytes.Equal(got, hello) {
		t.Fatalf("echoed bytes do not match the ClientHello sent")
	}
}

func TestGSSENCRequestDeclinedThenSSLRequestSucceeds(t *testing.T) {
	backendAddr := startBackend(t, sslPrimedBackend)
	_, addr, stop := newTestServer(routemap.Map{"tenant.db.mbr.madevara.com": backendAddr})
	defer stop()

	conn := dial(t, addr)
	var gssReq [8]byte
	binary.BigEndian.PutUint32(gssReq[0:4], 8)
	binary.BigEndian.PutUint32(gssReq[4:8], pgwire.GSSENCRequestCode)
	if _, err := conn.Write(gssReq[:]); err != nil {
		t.Fatalf("write GSSENCRequest: %v", err)
	}

	var resp [1]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		t.Fatalf("read GSSENCRequest reply: %v", err)
	}
	if resp[0] != 'N' {
		t.Fatalf("GSSENCRequest reply = %q, want 'N'", resp[0])
	}

	var sslReq [8]byte
	binary.BigEndian.PutUint32(sslReq[0:4], 8)
	binary.BigEndian.PutUint32(sslReq[4:8], pgwire.SSLRequestCode)
	if _, err := conn.Write(sslReq[:]); err != nil {
		t.Fatalf("write SSLRequest: %v", err)
	}
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		t.Fatalf("read SSLRequest reply: %v", err)
	}
	if resp[0] != 'S' {
		t.Fatalf("SSLRequest reply = %q, want 'S'", resp[0])
	}
}

func TestCancelRequestCloses(t *testing.T) {
	_, addr, stop := newTestServer(routemap.Map{})
	defer stop()

	conn := dial(t, addr)
	var req [16]byte
	binary.BigEndian.PutUint32(req[0:4], 16)
	binary.BigEndian.PutUint32(req[4:8], pgwire.CancelRequestCode)
	if _, err := conn.Write(req[:]); err != nil {
		t.Fatalf("write CancelRequest: %v", err)
	}

	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != io.EOF {
		t.Fatalf("read after CancelRequest = %v, want io.EOF", err)
	}
}

func TestPlaintextStartupMessageGetsErrorResponse(t *testing.T) {
	_, addr, stop := newTestServer(routemap.Map{})
	defer stop()

	conn := dial(t, addr)
	var req [8]byte
	binary.BigEndian.PutUint32(req[0:4], 41)     // plausible StartupMessage length
	binary.BigEndian.PutUint32(req[4:8], 196608) // protocol 3.0
	if _, err := conn.Write(req[:]); err != nil {
		t.Fatalf("write StartupMessage prefix: %v", err)
	}

	got, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read ErrorResponse: %v", err)
	}
	if len(got) == 0 || got[0] != 'E' {
		t.Fatalf("response = %q, want an ErrorResponse starting with 'E'", got)
	}
	if !bytes.Contains(got, []byte("FATAL")) {
		t.Fatalf("response does not mention FATAL: %q", got)
	}
}

func TestUnknownFirstMessageCloses(t *testing.T) {
	_, addr, stop := newTestServer(routemap.Map{})
	defer stop()

	conn := dial(t, addr)
	var req [8]byte
	binary.BigEndian.PutUint32(req[0:4], 8)
	binary.BigEndian.PutUint32(req[4:8], 0xDEADBEEF)
	if _, err := conn.Write(req[:]); err != nil {
		t.Fatalf("write garbage: %v", err)
	}

	got, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read after garbage: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("response = %q, want no reply for an unrecognized message", got)
	}
}

func TestSSLRequestUnknownHostCloses(t *testing.T) {
	_, addr, stop := newTestServer(routemap.Map{"known.db.mbr.madevara.com": "127.0.0.1:1"})
	defer stop()

	conn := dial(t, addr)
	var req [8]byte
	binary.BigEndian.PutUint32(req[0:4], 8)
	binary.BigEndian.PutUint32(req[4:8], pgwire.SSLRequestCode)
	if _, err := conn.Write(req[:]); err != nil {
		t.Fatalf("write SSLRequest: %v", err)
	}
	var resp [1]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		t.Fatalf("read SSLRequest reply: %v", err)
	}

	hello := buildClientHello("unknown.db.mbr.madevara.com")
	if _, err := conn.Write(hello); err != nil {
		t.Fatalf("write ClientHello: %v", err)
	}

	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != io.EOF {
		t.Fatalf("read for unknown host = %v, want io.EOF", err)
	}
}

func TestSSLRequestMissingSNICloses(t *testing.T) {
	_, addr, stop := newTestServer(routemap.Map{})
	defer stop()

	conn := dial(t, addr)
	var req [8]byte
	binary.BigEndian.PutUint32(req[0:4], 8)
	binary.BigEndian.PutUint32(req[4:8], pgwire.SSLRequestCode)
	if _, err := conn.Write(req[:]); err != nil {
		t.Fatalf("write SSLRequest: %v", err)
	}
	var resp [1]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		t.Fatalf("read SSLRequest reply: %v", err)
	}

	hello := buildClientHello("") // no server_name extension
	if _, err := conn.Write(hello); err != nil {
		t.Fatalf("write ClientHello: %v", err)
	}

	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != io.EOF {
		t.Fatalf("read for missing SNI = %v, want io.EOF", err)
	}
}
