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

// Database is a tenant database row. Everything past "provisioning" status
// is written by the data-plane agent, which this slice does not call.
type Database struct {
	ID        string
	UserID    string
	Name      string
	SizeGB    int
	Status    string
	CreatedAt time.Time
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
