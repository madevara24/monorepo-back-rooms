package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/internal/store"
	"github.com/madevara24/monorepo-back-rooms/internal/storetest"
)

func TestCreateAndFetchUser(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	created, err := store.CreateUser(ctx, db, "person@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.ID == "" {
		t.Fatal("CreateUser returned an empty id")
	}

	byEmail, err := store.UserByEmail(ctx, db, "person@example.com")
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Fatalf("UserByEmail id = %q, want %q", byEmail.ID, created.ID)
	}

	byID, err := store.UserByID(ctx, db, created.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if byID.Email != "person@example.com" {
		t.Fatalf("UserByID email = %q, want %q", byID.Email, "person@example.com")
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	if _, err := store.CreateUser(ctx, db, "dup@example.com", "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err := store.CreateUser(ctx, db, "dup@example.com", "hash2")
	if !errors.Is(err, store.ErrEmailTaken) {
		t.Fatalf("CreateUser error = %v, want ErrEmailTaken", err)
	}
}

func TestUserByEmailNotFound(t *testing.T) {
	db := storetest.Open(t)

	_, err := store.UserByEmail(context.Background(), db, "nobody@example.com")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UserByEmail error = %v, want sql.ErrNoRows", err)
	}
}
