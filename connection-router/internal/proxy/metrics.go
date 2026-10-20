package proxy

import "sync/atomic"

// Metrics counts how connections were handled, broken down by the
// handshake path they took. GET /healthz reports these so an operator
// can confirm, from outside, that clients are actually negotiating
// the way the router expects (see the migration runbook in
// connection-router/README.md).
type Metrics struct {
	DirectTLS         atomic.Int64
	SSLRequest        atomic.Int64
	GSSENCDeclined    atomic.Int64
	CancelClosed      atomic.Int64
	PlaintextRejected atomic.Int64
	UnknownClosed     atomic.Int64
	MissingSNI        atomic.Int64
	UnknownHost       atomic.Int64
	DialError         atomic.Int64
	ActiveConnections atomic.Int64
}

// Snapshot is a point-in-time, JSON-friendly copy of Metrics.
type Snapshot struct {
	DirectTLS         int64 `json:"direct_tls"`
	SSLRequest        int64 `json:"ssl_request"`
	GSSENCDeclined    int64 `json:"gssenc_declined"`
	CancelClosed      int64 `json:"cancel_closed"`
	PlaintextRejected int64 `json:"plaintext_rejected"`
	UnknownClosed     int64 `json:"unknown_closed"`
	MissingSNI        int64 `json:"missing_sni"`
	UnknownHost       int64 `json:"unknown_host"`
	DialError         int64 `json:"dial_error"`
	ActiveConnections int64 `json:"active_connections"`
}

// Snapshot reads every counter into a Snapshot.
func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{
		DirectTLS:         m.DirectTLS.Load(),
		SSLRequest:        m.SSLRequest.Load(),
		GSSENCDeclined:    m.GSSENCDeclined.Load(),
		CancelClosed:      m.CancelClosed.Load(),
		PlaintextRejected: m.PlaintextRejected.Load(),
		UnknownClosed:     m.UnknownClosed.Load(),
		MissingSNI:        m.MissingSNI.Load(),
		UnknownHost:       m.UnknownHost.Load(),
		DialError:         m.DialError.Load(),
		ActiveConnections: m.ActiveConnections.Load(),
	}
}
