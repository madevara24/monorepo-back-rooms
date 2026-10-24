// Package container is the agent's composition root: it reads
// config.Config, connects to Docker, and wires repository and usecase
// into a ready usecase.Node and HTTP mux for cmd/node to run.
package container

import (
	"fmt"
	"net/http"

	dockerclient "github.com/docker/docker/client"

	"github.com/madevara24/monorepo-back-rooms/node-agent-service/config"
	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/handler"
	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/usecase"
)

// Container holds every long-lived dependency cmd/node runs: the
// usecase and the HTTP mux built from it.
type Container struct {
	Node *usecase.Node
	Mux  *http.ServeMux
}

// New connects to Docker and wires every layer together. commit is the
// build's git SHA, reported by GET /v1/node.
func New(cfg config.Config, commit string) (*Container, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("connect to docker: %w", err)
	}

	n := &usecase.Node{
		Config: entities.Config{
			NodeID:            cfg.NodeID,
			HostAddress:       cfg.HostAddress,
			Image:             cfg.Image,
			PortMin:           cfg.PortMin,
			PortMax:           cfg.PortMax,
			Version:           commit,
			DemoMode:          cfg.DemoMode,
			TenantTLSCertPath: cfg.TenantTLSCert,
			TenantTLSKeyPath:  cfg.TenantTLSKey,
		},
		Docker:  repository.NewDockerClient(cli),
		Volumes: repository.NewLoopVolumeManager(cfg.DataDir),
	}

	return &Container{Node: n, Mux: handler.NewMux(n)}, nil
}
