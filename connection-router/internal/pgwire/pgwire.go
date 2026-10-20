// Package pgwire implements just enough of the Postgres wire protocol's
// startup phase for the router to tell apart the handful of first
// messages a client can send, without ever becoming a Postgres server
// itself: no auth, no query protocol, no TLS termination.
package pgwire

import (
	"bytes"
	"encoding/binary"
)

// Special request codes a client may send as the second 4 bytes of its
// first message, in place of a protocol version. See the Postgres
// protocol docs, "Protocol: Message Formats, StartupMessage".
const (
	SSLRequestCode    = 80877103
	GSSENCRequestCode = 80877104
	CancelRequestCode = 80877102
)

// DirectTLSByte is the first byte of a TLS record (handshake content
// type). A client using sslnegotiation=direct sends a TLS ClientHello
// as its very first bytes, with no SSLRequest wrapper, so seeing this
// byte before anything else identifies that path.
const DirectTLSByte = 0x16

// cancelRequestLen is CancelRequest's fixed total length: the 8-byte
// header plus a 4-byte backend PID and a 4-byte cancel secret.
const cancelRequestLen = 16

// protocolMajorVersion3 is the high 16 bits of every StartupMessage
// protocol version Postgres has shipped (3.0 through 3.2 so far). A
// code whose top half matches this, and isn't one of the special
// request codes above, is treated as a real StartupMessage; anything
// else is just noise and gets dropped without a reply.
const protocolMajorVersion3 = 3

// LooksLikeStartupMessage reports whether code (the second 4 bytes of
// a client's first message, after length) looks like a StartupMessage
// protocol version rather than garbage. It does not match any of the
// special request codes; callers must check those first.
func LooksLikeStartupMessage(code uint32) bool {
	return code>>16 == protocolMajorVersion3
}

// ValidCancelRequestLength reports whether length (the first 4 bytes
// of a client's first message) matches a real CancelRequest.
func ValidCancelRequestLength(length uint32) bool {
	return length == cancelRequestLen
}

// SSLRequest is the 8-byte message the router sends to a tenant to
// re-request SSL on its behalf, mirroring what a direct-negotiation
// client would have sent.
func SSLRequest() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], 8)
	binary.BigEndian.PutUint32(buf[4:8], SSLRequestCode)
	return buf
}

// ErrorResponse builds a minimal Postgres ErrorResponse message:
// severity FATAL, SQLSTATE 08P01 (protocol_violation), and message.
// It is the only backend message the router ever originates; sending
// it is the one case where the router must look enough like a
// Postgres server to let a plaintext client print a clean error
// instead of just seeing the connection drop.
func ErrorResponse(message string) []byte {
	var body bytes.Buffer
	body.WriteByte('S')
	body.WriteString("FATAL\x00")
	body.WriteByte('C')
	body.WriteString("08P01\x00")
	body.WriteByte('M')
	body.WriteString(message)
	body.WriteByte(0)
	body.WriteByte(0) // terminates the field list

	out := make([]byte, 0, 5+body.Len())
	out = append(out, 'E')
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(4+body.Len())) //nolint:gosec // body is a few dozen bytes
	out = append(out, lenBuf[:]...)
	out = append(out, body.Bytes()...)
	return out
}
