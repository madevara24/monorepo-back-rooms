package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
)

func TestSessionLifecycle(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	user, err := repository.CreateUser(ctx, db, "session@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = repository.CreateSession(ctx, db, "token-hash-1", user.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	session, err := repository.SessionByTokenHash(ctx, db, "token-hash-1")
	if err != nil {
		t.Fatalf("SessionByTokenHash: %v", err)
	}
	if session.UserID != user.ID {
		t.Fatalf("SessionByTokenHash user id = %q, want %q", session.UserID, user.ID)
	}

	if err := repository.DeleteSession(ctx, db, "token-hash-1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, err = repository.SessionByTokenHash(ctx, db, "token-hash-1")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SessionByTokenHash after delete error = %v, want sql.ErrNoRows", err)
	}
}

func TestSessionByTokenHashExpired(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	user, err := repository.CreateUser(ctx, db, "expired@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = repository.CreateSession(ctx, db, "expired-token-hash", user.ID, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	_, err = repository.SessionByTokenHash(ctx, db, "expired-token-hash")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SessionByTokenHash for expired session error = %v, want sql.ErrNoRows", err)
	}
}
