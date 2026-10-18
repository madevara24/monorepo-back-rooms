// Command api runs the MBR control-plane HTTP API.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/madevara24/monorepo-back-rooms/internal/healthz"
)

func main() {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz.Handler)

	log.Printf("api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
