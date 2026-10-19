// Command api runs the MBR control-plane HTTP API.
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/madevara24/monorepo-back-rooms/internal/agentclient"
	"github.com/madevara24/monorepo-back-rooms/internal/api"
	"github.com/madevara24/monorepo-back-rooms/internal/credentials"
	"github.com/madevara24/monorepo-back-rooms/internal/jobs"
	"github.com/madevara24/monorepo-back-rooms/internal/routes"
	"github.com/madevara24/monorepo-back-rooms/internal/store"
)

// commit is stamped at build time with -ldflags "-X main.commit=<sha>".
var commit = "unknown"

// jobPollInterval is how often the worker checks for due jobs when the
// queue is empty.
const jobPollInterval = 2 * time.Second

// routeReconcileInterval is how often the SNI route map is rewritten from
// the current set of running databases, on top of the reconcile that
// already runs right after a provision or delete job completes.
const routeReconcileInterval = 10 * time.Second

// defaultRoutesPath is the SNI proxy's host map on KKB, bind-mounted
// read-write into the mbr-api container (see dokploy-compose.yml).
const defaultRoutesPath = "/etc/mbr/routes/mbr-routes.map"

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

	cipher, err := credentials.NewCipher(os.Getenv("CREDENTIAL_KEY"))
	if err != nil {
		log.Fatalf("configure credential cipher: %v", err)
	}

	agent, err := newAgentClient()
	if err != nil {
		log.Fatalf("configure agent client: %v", err)
	}

	routesPath := os.Getenv("MBR_ROUTES_PATH")
	if routesPath == "" {
		routesPath = defaultRoutesPath
	}
	routeReconciler := &routes.Reconciler{DB: db, Path: routesPath}

	worker := &jobs.Worker{
		DB: db, Agent: agent, Cipher: cipher, PollInterval: jobPollInterval, LeaseTimeout: 10 * time.Minute,
		Routes: routeReconciler,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go worker.Run(ctx)
	go routeReconciler.Run(ctx, routeReconcileInterval)

	mux := api.NewMux(&api.API{
		DB:           db,
		CookieSecure: os.Getenv("API_COOKIE_SECURE") == "true",
		Commit:       commit,
		Cipher:       cipher,
	})

	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("api listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// newAgentClient builds the mTLS client cmd/api's job worker uses to call
// mbr-node, from the base64-encoded PEM certificates in the environment.
func newAgentClient() (*agentclient.Client, error) {
	caPEM, err := decodeBase64Env("AGENT_CA_B64")
	if err != nil {
		return nil, err
	}
	certPEM, err := decodeBase64Env("AGENT_CLIENT_CERT_B64")
	if err != nil {
		return nil, err
	}
	keyPEM, err := decodeBase64Env("AGENT_CLIENT_KEY_B64")
	if err != nil {
		return nil, err
	}
	return agentclient.New(caPEM, certPEM, keyPEM)
}

func decodeBase64Env(key string) ([]byte, error) {
	v := os.Getenv(key)
	if v == "" {
		return nil, fmt.Errorf("%s is required", key)
	}
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", key, err)
	}
	return b, nil
}
