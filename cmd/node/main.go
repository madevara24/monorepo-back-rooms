// Command node runs the MBR data-plane agent (mbr-node). It is the only
// process that is meant to talk to Docker; for now it only answers healthz.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/madevara24/monorepo-back-rooms/internal/healthz"
)

func main() {
	addr := os.Getenv("NODE_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz.NewHandler(""))

	log.Printf("node listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
