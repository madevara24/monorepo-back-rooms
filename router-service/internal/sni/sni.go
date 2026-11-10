// Package sni reads a TLS ClientHello far enough to extract the SNI
// server_name extension, and nothing else. It never builds a
// *tls.Conn and never sees a certificate or key: the router forwards
// the raw bytes it reads here on to the tenant unmodified, so the
// tenant performs the real TLS handshake.
package sni

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// MaxClientHello caps the total bytes read while assembling a
// ClientHello (across every TLS record fragment), so a malicious or
// broken client can't make the router buffer unbounded data.
const MaxClientHello = 16 * 1024

const (
	recordTypeHandshake      = 0x16
	handshakeTypeClientHello = 0x01
	extensionServerName      = 0x0000
	serverNameTypeHost       = 0x00
)

var (
	// ErrTooLarge is returned when assembling the ClientHello would
	// exceed MaxClientHello.
	ErrTooLarge = errors.New("sni: ClientHello exceeds size cap")
	// ErrNotHandshake is returned when the first TLS record is not a
	// handshake record, or the handshake message is not a ClientHello.
	ErrNotHandshake = errors.New("sni: not a TLS ClientHello")
	// ErrNoServerName is returned when a ClientHello parses cleanly
	// but carries no server_name extension.
	ErrNoServerName = errors.New("sni: no server_name extension")
)

// Result is a successfully parsed ClientHello: the SNI hostname it
// carried (already lowercased with a trailing dot stripped) and the
// raw bytes read from r to assemble it, which the caller must forward
// to the tenant byte-for-byte.
type Result struct {
	ServerName string
	Raw        []byte
}

// ReadClientHello reads a TLS ClientHello from r, which may be split
// across multiple TLS records (a client is free to fragment the
// handshake layer at any record boundary), and extracts its SNI
// server_name. It returns the exact bytes consumed from r so the
// caller can replay them to a backend verbatim.
func ReadClientHello(r io.Reader) (Result, error) {
	var raw bytes.Buffer
	var hs bytes.Buffer // assembled handshake-protocol stream

	for {
		record, err := readRecord(r, &raw)
		if err != nil {
			return Result{}, err
		}
		if record.typ != recordTypeHandshake {
			return Result{}, ErrNotHandshake
		}
		hs.Write(record.payload)

		if raw.Len() > MaxClientHello || hs.Len() > MaxClientHello {
			return Result{}, ErrTooLarge
		}

		if hs.Len() < 4 {
			continue // haven't even got the handshake header yet
		}
		hsBytes := hs.Bytes()
		if hsBytes[0] != handshakeTypeClientHello {
			return Result{}, ErrNotHandshake
		}
		bodyLen := int(hsBytes[1])<<16 | int(hsBytes[2])<<8 | int(hsBytes[3])
		if 4+bodyLen > MaxClientHello {
			return Result{}, ErrTooLarge
		}
		if hs.Len() < 4+bodyLen {
			continue // ClientHello body itself is still fragmented
		}

		name, err := parseServerName(hsBytes[4 : 4+bodyLen])
		if err != nil {
			return Result{}, err
		}
		return Result{ServerName: name, Raw: raw.Bytes()}, nil
	}
}

type tlsRecord struct {
	typ     byte
	payload []byte
}

// readRecord reads one TLS record from r (5-byte header: type, 2-byte
// version, 2-byte length, then length bytes of payload), appending
// every byte it consumes to raw.
func readRecord(r io.Reader, raw *bytes.Buffer) (tlsRecord, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return tlsRecord{}, fmt.Errorf("sni: read record header: %w", err)
	}
	raw.Write(header[:])

	length := int(header[3])<<8 | int(header[4])
	if length > MaxClientHello {
		return tlsRecord{}, ErrTooLarge
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return tlsRecord{}, fmt.Errorf("sni: read record payload: %w", err)
	}
	raw.Write(payload)

	return tlsRecord{typ: header[0], payload: payload}, nil
}

// parseServerName walks a ClientHello body (everything after the
// 4-byte handshake header) to find the server_name extension's first
// host_name entry.
func parseServerName(body []byte) (string, error) {
	c := cursor{buf: body}

	c.skip(2)  // client_version
	c.skip(32) // random
	sessionIDLen, ok := c.readU8()
	if !ok {
		return "", ErrNotHandshake
	}
	c.skip(int(sessionIDLen))

	cipherSuitesLen, ok := c.readU16()
	if !ok {
		return "", ErrNotHandshake
	}
	c.skip(int(cipherSuitesLen))

	compressionLen, ok := c.readU8()
	if !ok {
		return "", ErrNotHandshake
	}
	c.skip(int(compressionLen))

	if c.remaining() == 0 {
		// No extensions block at all: valid ClientHello, no SNI.
		return "", ErrNoServerName
	}
	extensionsLen, ok := c.readU16()
	if !ok {
		return "", ErrNotHandshake
	}
	extensions, ok := c.readN(int(extensionsLen))
	if !ok {
		return "", ErrNotHandshake
	}

	ec := cursor{buf: extensions}
	for ec.remaining() > 0 {
		extType, ok := ec.readU16()
		if !ok {
			return "", ErrNotHandshake
		}
		extLen, ok := ec.readU16()
		if !ok {
			return "", ErrNotHandshake
		}
		extData, ok := ec.readN(int(extLen))
		if !ok {
			return "", ErrNotHandshake
		}

		if extType == extensionServerName {
			return parseServerNameExtension(extData)
		}
	}
	return "", ErrNoServerName
}

// parseServerNameExtension parses the server_name extension's data:
// a 2-byte list length, then one or more (type, 2-byte length, name)
// entries. It returns the first host_name entry.
func parseServerNameExtension(data []byte) (string, error) {
	c := cursor{buf: data}
	listLen, ok := c.readU16()
	if !ok {
		return "", ErrNotHandshake
	}
	list, ok := c.readN(int(listLen))
	if !ok {
		return "", ErrNotHandshake
	}

	lc := cursor{buf: list}
	for lc.remaining() > 0 {
		nameType, ok := lc.readU8()
		if !ok {
			return "", ErrNotHandshake
		}
		nameLen, ok := lc.readU16()
		if !ok {
			return "", ErrNotHandshake
		}
		name, ok := lc.readN(int(nameLen))
		if !ok {
			return "", ErrNotHandshake
		}
		if nameType == serverNameTypeHost {
			return string(name), nil
		}
	}
	return "", ErrNoServerName
}

// cursor is a bounds-checked reader over an in-memory byte slice. Any
// read past the end of buf sets err and every subsequent call is a
// no-op, so callers can chain reads and check err once at the end.
type cursor struct {
	buf []byte
	pos int
	err bool
}

func (c *cursor) remaining() int { return len(c.buf) - c.pos }

func (c *cursor) skip(n int) {
	if c.err || n > c.remaining() {
		c.err = true
		return
	}
	c.pos += n
}

func (c *cursor) readN(n int) ([]byte, bool) {
	if c.err || n > c.remaining() {
		c.err = true
		return nil, false
	}
	out := c.buf[c.pos : c.pos+n]
	c.pos += n
	return out, true
}

func (c *cursor) readU8() (byte, bool) {
	b, ok := c.readN(1)
	if !ok {
		return 0, false
	}
	return b[0], true
}

func (c *cursor) readU16() (uint16, bool) {
	b, ok := c.readN(2)
	if !ok {
		return 0, false
	}
	return uint16(b[0])<<8 | uint16(b[1]), true
}
