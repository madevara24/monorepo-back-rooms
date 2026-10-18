// Command api runs the MBR control-plane HTTP API.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/madevara24/monorepo-back-rooms/internal/api"
	"github.com/madevara24/monorepo-back-rooms/internal/store"
)

func main() {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://mbr:mbr@localhost:5432/mbr?sslmode=disable"
	}

	db, err := store.Open(databaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := store.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	mux := api.NewMux(&api.API{
		DB:           db,
		CookieSecure: os.Getenv("API_COOKIE_SECURE") == "true",
	})

	log.Printf("api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
