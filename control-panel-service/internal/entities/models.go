// Package entities holds the control plane's domain models: the row
// shapes repository reads and writes, and handler and usecase pass
// around.
package entities

import "time"

// User is a control-plane account. IsAdmin gates every /api/admin/* route;
// it is set by a one-time SQL update, not an endpoint.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	IsAdmin      bool
	IsSeed       bool
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
// response. PlanID and PriceIDRMonth are nil for rows created before
// plans existed, and for any row created without picking one.
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
	PlanID            *string
	PriceIDRMonth     *int64
}

// Plan is a purchasable database package: a fixed size and a monthly
// price. Deactivating a plan (Active = false) hides it from GET /api/plans
// and blocks new purchases without touching databases that already
// reference it. DatabaseCount is only populated by the admin listing
// (repository.ListAllPlans); every other reader leaves it zero.
type Plan struct {
	ID            string
	Slug          string
	Name          string
	SizeGB        int
	PriceIDRMonth int64
	Tagline       string
	Badge         string
	SortOrder     int
	Active        bool
	Purchasable   bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DatabaseCount int
}

// DemoState is the singleton row that drives the demo reset: the
// current generation, whether the demo is in maintenance, and the next
// wall-clock boundary.
type DemoState struct {
	Generation  int64
	Status      string
	Maintenance bool
	StartedAt   *time.Time
	FinishedAt  *time.Time
	NextResetAt time.Time
	LastError   *string
}

// ResetRun is one demo reset attempt: what triggered it, how long it
// took, and what the agent reported deleting.
type ResetRun struct {
	ID                string
	Trigger           string
	StartedAt         time.Time
	FinishedAt        *time.Time
	ContainersDeleted int
	VolumesDeleted    int
	OrphansDeleted    int
	Error             *string
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

// DatabaseMetric is one connection-pressure snapshot the agent collected
// from a tenant container, keyed by when the agent collected it (not when
// the control plane received it).
type DatabaseMetric struct {
	ID                           int64
	DatabaseID                   string
	Active                       int
	Idle                         int
	IdleInTransaction            int
	MaxConnections               int
	SuperuserReservedConnections int
	MemoryLimitBytes             int64
	CollectedAt                  time.Time
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
