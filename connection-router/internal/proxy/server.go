// Package proxy is the router's data path: per-connection handshake
// detection, SNI-based routing, and byte splicing. It never builds a
// *tls.Config and never sees a certificate or key — every byte a
// client sends during the TLS handshake is forwarded to the tenant
// unparsed beyond the SNI extension the router reads in order to
// decide where "unparsed" goes.
package proxy

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/pgwire"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/routemap"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/sni"
)

// Defaults match the task's operational requirements; cmd/router
// wires them in explicitly rather than having Server guess at zero
// values, so a test can set its own short timeouts without surprises.
const (
	DefaultDialTimeout      = 5 * time.Second
	DefaultIdleTimeout      = time.Hour
	DefaultHandshakeTimeout = 10 * time.Second
	DefaultMaxConnections   = 4096
)

// RouteSource is the live, hot-reloadable host map the router
// consults on every connection. *routemap.Watcher satisfies this.
type RouteSource interface {
	Current() routemap.Map
}

// Server is the protocol-aware router described in the task: it never
// terminates TLS, holds no certificate, and forwards every byte past
// the SNI it reads to decide a route.
type Server struct {
	// Routes is consulted fresh on every connection, so a map reload
	// takes effect immediately for new connections without a restart.
	Routes RouteSource
	// Metrics is updated as connections are classified and handled;
	// GET /healthz reports a snapshot of it.
	Metrics *Metrics

	DialTimeout      time.Duration
	IdleTimeout      time.Duration
	HandshakeTimeout time.Duration
	// MaxConnections caps concurrently active connections; beyond it,
	// new connections are accepted and immediately closed.
	MaxConnections int
}

// Serve accepts connections from ln and handles each in its own
// goroutine until ctx is canceled or Accept fails.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	sem := make(chan struct{}, s.MaxConnections)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	var wg sync.WaitGroup
	for {
		conn, err := ln.Accept()
		if err != nil {
			wg.Wait()
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		select {
		case sem <- struct{}{}:
		default:
			_ = conn.Close()
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			s.handleConn(conn)
		}()
	}
}

// handleConn runs the full handshake-detection-and-splice lifecycle
// for one accepted connection. Every return from the functions it
// calls closes the connection; there is no error path that leaves it
// open without either a route or an explicit protocol reply.
func (s *Server) handleConn(conn net.Conn) {
	s.Metrics.ActiveConnections.Add(1)
	defer s.Metrics.ActiveConnections.Add(-1)
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(s.HandshakeTimeout)); err != nil {
		return
	}

	br := bufio.NewReader(conn)
	s.negotiate(conn, br)
}

// negotiate implements the handshake decision tree: direct TLS, or
// the 8-byte length+code prefix that disambiguates SSLRequest,
// GSSENCRequest, CancelRequest, and a plaintext StartupMessage.
func (s *Server) negotiate(conn net.Conn, br *bufio.Reader) {
	for {
		first, err := br.Peek(1)
		if err != nil {
			return
		}
		if first[0] == pgwire.DirectTLSByte {
			s.handleDirectTLS(conn, br)
			return
		}

		var header [8]byte
		if _, err := io.ReadFull(br, header[:]); err != nil {
			return
		}
		length := binary.BigEndian.Uint32(header[0:4])
		code := binary.BigEndian.Uint32(header[4:8])

		switch {
		case code == pgwire.SSLRequestCode && length == 8:
			s.Metrics.SSLRequest.Add(1)
			s.handleSSLRequest(conn, br)
			return

		case code == pgwire.GSSENCRequestCode && length == 8:
			s.Metrics.GSSENCDeclined.Add(1)
			if _, err := conn.Write([]byte("N")); err != nil {
				return
			}
			continue // the client sends its real first message next

		case code == pgwire.CancelRequestCode && pgwire.ValidCancelRequestLength(length):
			// CancelRequest carries no SNI and arrives on a brand new
			// connection, so the router has nothing to route it by;
			// routing cancellation is explicitly out of scope. Close
			// so the client's cancel attempt fails fast rather than
			// hanging.
			s.Metrics.CancelClosed.Add(1)
			return

		case pgwire.LooksLikeStartupMessage(code):
			s.Metrics.PlaintextRejected.Add(1)
			_, _ = conn.Write(pgwire.ErrorResponse("direct TLS or SSLRequest required"))
			return

		default:
			s.Metrics.UnknownClosed.Add(1)
			return
		}
	}
}

// handleDirectTLS handles a client that skipped the plaintext
// SSLRequest and sent a TLS ClientHello as its very first bytes
// (sslnegotiation=direct). The tenant is expected to accept the same
// direct negotiation, so the raw ClientHello bytes are forwarded
// as-is with no SSLRequest primer.
func (s *Server) handleDirectTLS(conn net.Conn, br *bufio.Reader) {
	hello, err := sni.ReadClientHello(br)
	if err != nil {
		s.Metrics.MissingSNI.Add(1)
		return
	}

	backend := s.dial(hello.ServerName)
	if backend == nil {
		return
	}
	defer backend.Close()

	if err := forward(backend, hello.Raw, s.DialTimeout); err != nil {
		return
	}

	s.Metrics.DirectTLS.Add(1)
	s.splice(conn, br, backend)
}

// handleSSLRequest handles the common case: a client sent the
// plaintext SSLRequest, expecting the single-byte 'S'/'N' reply
// before it starts TLS. The router replies 'S' unconditionally (SNI
// routing requires a ClientHello) and re-issues the SSLRequest to the
// tenant on the client's behalf once it knows which tenant that is.
func (s *Server) handleSSLRequest(conn net.Conn, br *bufio.Reader) {
	if _, err := conn.Write([]byte("S")); err != nil {
		return
	}

	hello, err := sni.ReadClientHello(br)
	if err != nil {
		s.Metrics.MissingSNI.Add(1)
		return
	}

	backend := s.dial(hello.ServerName)
	if backend == nil {
		return
	}
	defer backend.Close()

	if err := backend.SetDeadline(time.Now().Add(s.DialTimeout)); err != nil {
		return
	}
	if _, err := backend.Write(pgwire.SSLRequest()); err != nil {
		return
	}
	var resp [1]byte
	if _, err := io.ReadFull(backend, resp[:]); err != nil || resp[0] != 'S' {
		return
	}
	if err := backend.SetDeadline(time.Time{}); err != nil {
		return
	}

	if err := forward(backend, hello.Raw, s.DialTimeout); err != nil {
		return
	}

	s.splice(conn, br, backend)
}

// dial looks serverName up in the current route map and connects to
// its tenant, logging and counting a miss or a dial failure. A nil
// return means the caller's connection has already been fully
// handled (closing it is still the caller's job).
func (s *Server) dial(serverName string) net.Conn {
	host := normalizeHost(serverName)
	addr, ok := s.Routes.Current().Lookup(host)
	if !ok {
		s.Metrics.UnknownHost.Add(1)
		return nil
	}

	backend, err := net.DialTimeout("tcp", addr, s.DialTimeout)
	if err != nil {
		s.Metrics.DialError.Add(1)
		log.Printf("proxy: dial %s for %s: %v", addr, host, err)
		return nil
	}
	return backend
}

// normalizeHost lowercases name and strips one trailing dot, matching
// how a ClientHello's server_name is compared against the map. There
// is no wildcard or suffix matching: only an exact hit routes.
func normalizeHost(name string) string {
	return strings.TrimSuffix(strings.ToLower(name), ".")
}

// forward writes buffered bytes to backend under a bounded deadline,
// then clears the deadline so a later splice isn't cut short by it.
func forward(backend net.Conn, buffered []byte, timeout time.Duration) error {
	if err := backend.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	if _, err := backend.Write(buffered); err != nil {
		return err
	}
	return backend.SetWriteDeadline(time.Time{})
}

// splice copies bytes in both directions between the client and the
// tenant until one side closes, half-closing the other side's write
// end so a client-initiated close propagates to the tenant (and vice
// versa) instead of leaking a half-open connection. Each direction's
// read deadline is refreshed on every read, so IdleTimeout measures
// silence, not total connection lifetime.
func (s *Server) splice(conn net.Conn, br *bufio.Reader, backend net.Conn) {
	_ = conn.SetDeadline(time.Time{})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(backend, &deadlineReader{r: br, conn: conn, timeout: s.IdleTimeout})
		halfClose(backend)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(conn, &deadlineReader{r: backend, conn: backend, timeout: s.IdleTimeout})
		halfClose(conn)
	}()
	wg.Wait()
}

// deadlineReader resets conn's read deadline before every Read, so a
// stream that is actively (if slowly) exchanging bytes never trips
// the deadline, but one that goes silent for timeout does.
type deadlineReader struct {
	r       io.Reader
	conn    net.Conn
	timeout time.Duration
}

func (d *deadlineReader) Read(p []byte) (int, error) {
	if err := d.conn.SetReadDeadline(time.Now().Add(d.timeout)); err != nil {
		return 0, err
	}
	return d.r.Read(p)
}

type closeWriter interface {
	CloseWrite() error
}

// halfClose shuts down c's write side if it supports TCP half-close,
// falling back to a full close for connections that don't (e.g. the
// net.Pipe conns the tests use).
func halfClose(c net.Conn) {
	if cw, ok := c.(closeWriter); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = c.Close()
}
