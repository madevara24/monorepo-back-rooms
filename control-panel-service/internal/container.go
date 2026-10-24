// Package container is the control plane's composition root: it reads
// config.Config, connects and migrates the database, and wires
// repository, integration, usecase, and handler into a ready mux and
// background workers for cmd/api to run.
package container

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/config"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/handler"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/integration"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/credentials"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/migrations"
)

// jobPollInterval is how often the worker checks for due jobs when the
// queue is empty.
const jobPollInterval = 2 * time.Second

// jobLeaseTimeout is how long a job may sit in "running" before it is
// reclaimed from a crashed or killed worker.
const jobLeaseTimeout = 10 * time.Minute

// metricsRetentionDays is how long connection-pressure history is kept
// before it is pruned.
const metricsRetentionDays = 14

// Container holds every long-lived dependency cmd/api runs: the database
// handle, the HTTP router, and the background job worker, route
// reconciler, and metrics retention pruner.
type Container struct {
	DB               *sql.DB
	Mux              http.Handler
	Worker           *usecase.Worker
	Reconciler       *usecase.Reconciler
	MetricsRetention *usecase.MetricsRetention
	Demo             *usecase.DemoReset
}

// New connects to the database, applies migrations, and wires every
// layer together.
func New(cfg config.Config, commit string) (*Container, error) {
	db, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := migrations.Run(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	cipher, err := credentials.NewCipher(cfg.CredentialKeyB64)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure credential cipher: %w", err)
	}

	agent, err := newAgentClient(cfg)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure agent client: %w", err)
	}

	reconciler := &usecase.Reconciler{DB: db, Path: cfg.RoutesPath}
	metricsRetention := &usecase.MetricsRetention{DB: db, RetentionDays: metricsRetentionDays}

	worker := &usecase.Worker{
		DB: db, Agent: agent, Cipher: cipher,
		PollInterval: jobPollInterval, LeaseTimeout: jobLeaseTimeout,
		Routes: reconciler,
	}

	h := &handler.Handler{
		DB:           db,
		Auth:         &usecase.Auth{DB: db},
		CookieSecure: cfg.CookieSecure,
		Commit:       commit,
		Cipher:       cipher,
	}

	var demo *usecase.DemoReset
	if cfg.DemoMode {
		if _, err := usecase.SeedAdmin(context.Background(), db, cfg.DemoAdminEmail, cfg.DemoAdminPassword); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("seed demo admin: %w", err)
		}
		demo = &usecase.DemoReset{
			DB:         db,
			Agent:      agent,
			Reconciler: reconciler,
			Gate:       &sync.RWMutex{},
		}
		worker.Gate = demo.Gate
		worker.Paused = demo.Paused.Load
		h.Demo = demo
	}

	return &Container{
		DB:               db,
		Mux:              handler.NewMux(h),
		Worker:           worker,
		Reconciler:       reconciler,
		MetricsRetention: metricsRetention,
		Demo:             demo,
	}, nil
}

// newAgentClient builds the mTLS client the job worker uses to call
// mbr-node, from the base64-encoded PEM certificates in cfg.
func newAgentClient(cfg config.Config) (*integration.Client, error) {
	caPEM, err := decodeBase64("AGENT_CA_B64", cfg.AgentCAB64)
	if err != nil {
		return nil, err
	}
	certPEM, err := decodeBase64("AGENT_CLIENT_CERT_B64", cfg.AgentClientCertB64)
	if err != nil {
		return nil, err
	}
	keyPEM, err := decodeBase64("AGENT_CLIENT_KEY_B64", cfg.AgentClientKeyB64)
	if err != nil {
		return nil, err
	}
	return integration.New(caPEM, certPEM, keyPEM)
}

func decodeBase64(name, v string) ([]byte, error) {
	if v == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", name, err)
	}
	return b, nil
}
