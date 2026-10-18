package store

import "time"

// User is a control-plane account.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// Session is a server-side session keyed by the SHA-256 hash of the opaque
// token the client carries in its cookie. The raw token is never stored.
type Session struct {
	TokenHash string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Database is a tenant database row. Slug, NodeID, Host, Port, and
// PasswordEncrypted are nil until the job worker provisions the row; Slug
// is assigned at creation time but the others wait on the agent's
// response.
type Database struct {
	ID                string
	UserID            string
	Name              string
	SizeGB            int
	Status            string
	Slug              *string
	NodeID            *string
	Host              *string
	Port              *int
	PasswordEncrypted []byte
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Node is a registered mbr-node agent. TokenHash is the SHA-256 hash of
// the bearer token the agent presents on every heartbeat; the raw token
// is never stored.
type Node struct {
	ID            string
	Name          string
	Address       string
	TokenHash     string
	Capacity      []byte
	Version       string
	LastHeartbeat *time.Time
	CreatedAt     time.Time
}

// NodeContainer mirrors one container a node reported in its latest
// heartbeat.
type NodeContainer struct {
	NodeID     string
	DatabaseID string
	Status     string
	Port       int
	UpdatedAt  time.Time
}

// DatabaseEvent is an audit log entry for a database's status
// transitions, written by the job worker.
type DatabaseEvent struct {
	ID         string
	DatabaseID string
	FromStatus *string
	ToStatus   string
	Reason     *string
	CreatedAt  time.Time
}

// Job is a unit of work for the job worker: a "provision" or "delete"
// request queued by the databases API and claimed with
// FOR UPDATE SKIP LOCKED.
type Job struct {
	ID        string
	Kind      string
	Payload   []byte
	Status    string
	Attempts  int
	RunAt     time.Time
	LockedAt  *time.Time
	LastError *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Invoice is a billing period for a user. Payment only flips the status;
// no money moves.
type Invoice struct {
	ID             string
	UserID         string
	Status         string
	TotalAmountIDR int64
	PeriodStart    time.Time
	PeriodEnd      time.Time
	CreatedAt      time.Time
	PaidAt         *time.Time
}
