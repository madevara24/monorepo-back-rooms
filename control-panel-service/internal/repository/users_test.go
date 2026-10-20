package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
)

func TestCreateAndFetchUser(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	created, err := repository.CreateUser(ctx, db, "person@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.ID == "" {
		t.Fatal("CreateUser returned an empty id")
	}

	byEmail, err := repository.UserByEmail(ctx, db, "person@example.com")
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Fatalf("UserByEmail id = %q, want %q", byEmail.ID, created.ID)
	}

	byID, err := repository.UserByID(ctx, db, created.ID)
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

	if _, err := repository.CreateUser(ctx, db, "dup@example.com", "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err := repository.CreateUser(ctx, db, "dup@example.com", "hash2")
	if !errors.Is(err, repository.ErrEmailTaken) {
		t.Fatalf("CreateUser error = %v, want ErrEmailTaken", err)
	}
}

func TestUserByEmailNotFound(t *testing.T) {
	db := storetest.Open(t)

	_, err := repository.UserByEmail(context.Background(), db, "nobody@example.com")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UserByEmail error = %v, want sql.ErrNoRows", err)
	}
}
