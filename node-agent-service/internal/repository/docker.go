// Package repository implements the agent's Docker and volume adapters:
// the only package that imports the Docker SDK or shells out to mount
// and mkfs.ext4.
package repository

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerfilters "github.com/docker/docker/api/types/filters"
	dockerimage "github.com/docker/docker/api/types/image"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/entities"
)

// Labels identify and describe tenant containers so the agent can find
// them again across restarts without a local database.
const (
	LabelTenant     = "mbr.tenant"
	LabelDatabaseID = "mbr.database_id"
	LabelPort       = "mbr.port"
)

// postgresContainerPort is the port Postgres listens on inside the
// container; it is always published to a host port from the agent's range.
const postgresContainerPort = "5432/tcp"

// Paths the wildcard TLS cert and key are mounted at inside every tenant
// container, regardless of where the agent found them on the host.
//
// Postgres refuses to start if ssl_key_file is readable by anyone but the
// server: the host-side key must be owned by root or by the container's
// postgres user (uid 999 in postgres:17) with mode 0600, or owned by root
// with mode 0640 and a group the postgres process belongs to (the image
// ships a "ssl-cert" group, gid 101, for exactly this). Bind mounts carry
// the host file's numeric uid/gid as-is, so infra must set one of those
// combinations when it places the wildcard key on the host; a wrong mode
// or owner fails every tenant creation with a clear error in the
// container's logs ("private key file ... must be owned by ..." or "...
// has group or world access"), not a silent one.
const (
	tenantTLSCertMountPath = "/run/mbr-node/tenant.crt"
	tenantTLSKeyMountPath  = "/run/mbr-node/tenant.key"
)

// tenantHBAConfName is the pg_hba.conf override written next to PGDATA
// inside the host data dir (so it survives inside the existing bind mount
// without a separate one) and passed to Postgres via hba_file.
const tenantHBAConfName = "pg_hba_custom.conf"

// tenantHBAConf returns a pg_hba.conf that accepts network connections
// over TLS only (hostssl, no plaintext host rule) while keeping the local
// socket trust rule the image's init scripts rely on and the existing
// password auth method for remote connections.
func tenantHBAConf() string {
	return "local all all trust\n" +
		"hostssl all all all scram-sha-256\n"
}

// DockerClient is the subset of the Docker API the agent needs. Handlers
// depend on this interface rather than the SDK client directly, so they
// can be tested against a fake.
type DockerClient interface {
	// ListTenantContainers returns every container labeled
	// mbr.tenant=true, regardless of state.
	ListTenantContainers(ctx context.Context) ([]entities.Container, error)
	// UsedHostPorts returns the host ports currently published by any
	// container, tenant or not.
	UsedHostPorts(ctx context.Context) (map[int]bool, error)
	// CreateTenantContainer pulls the image if needed and starts a new
	// labeled tenant container.
	CreateTenantContainer(ctx context.Context, spec entities.ContainerSpec) (entities.Container, error)
	// RemoveTenantContainer stops and force-removes a container.
	RemoveTenantContainer(ctx context.Context, containerID string) error
}

// sdkClient adapts the Docker SDK's *client.Client to DockerClient.
type sdkClient struct {
	cli *dockerclient.Client
}

// NewDockerClient wraps a Docker SDK client as a DockerClient.
func NewDockerClient(cli *dockerclient.Client) DockerClient {
	return &sdkClient{cli: cli}
}

func tenantFilter() dockerfilters.Args {
	return dockerfilters.NewArgs(dockerfilters.Arg("label", LabelTenant+"=true"))
}

func toContainer(s dockertypes.Container) entities.Container {
	port, _ := strconv.Atoi(s.Labels[LabelPort])
	return entities.Container{
		ContainerID: s.ID,
		DatabaseID:  s.Labels[LabelDatabaseID],
		Port:        port,
		Status:      s.State,
	}
}

func (c *sdkClient) ListTenantContainers(ctx context.Context) ([]entities.Container, error) {
	summaries, err := c.cli.ContainerList(ctx, dockercontainer.ListOptions{All: true, Filters: tenantFilter()})
	if err != nil {
		return nil, fmt.Errorf("list tenant containers: %w", err)
	}
	out := make([]entities.Container, len(summaries))
	for i, s := range summaries {
		out[i] = toContainer(s)
	}
	return out, nil
}

func (c *sdkClient) UsedHostPorts(ctx context.Context) (map[int]bool, error) {
	summaries, err := c.cli.ContainerList(ctx, dockercontainer.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	used := make(map[int]bool)
	for _, s := range summaries {
		for _, p := range s.Ports {
			if p.PublicPort != 0 {
				used[int(p.PublicPort)] = true
			}
		}
	}
	return used, nil
}

// buildContainerConfig builds the Docker config and host config for a
// tenant container from spec. It is pure (no Docker or filesystem calls)
// so the resulting mounts and Postgres TLS flags can be tested directly.
func buildContainerConfig(spec entities.ContainerSpec) (*dockercontainer.Config, *dockercontainer.HostConfig) {
	containerPort := nat.Port(postgresContainerPort)
	hostPort := strconv.Itoa(spec.Port)

	config := &dockercontainer.Config{
		Image: spec.Image,
		Env: []string{
			"POSTGRES_PASSWORD=" + spec.Password,
			"POSTGRES_DB=postgres",
			// The ext4 loop volume root contains lost+found, so Postgres
			// must initialize in a subdirectory instead of the mount root.
			"PGDATA=/var/lib/postgresql/data/pgdata",
		},
		Cmd: []string{
			"postgres",
			"-c", "ssl=on",
			"-c", "ssl_cert_file=" + tenantTLSCertMountPath,
			"-c", "ssl_key_file=" + tenantTLSKeyMountPath,
			"-c", "hba_file=/var/lib/postgresql/data/" + tenantHBAConfName,
		},
		ExposedPorts: nat.PortSet{containerPort: struct{}{}},
		Labels: map[string]string{
			LabelTenant:     "true",
			LabelDatabaseID: spec.DatabaseID,
			LabelPort:       hostPort,
		},
	}
	hostConfig := &dockercontainer.HostConfig{
		Binds: []string{
			spec.HostDataDir + ":/var/lib/postgresql/data",
			spec.TLSCertPath + ":" + tenantTLSCertMountPath + ":ro",
			spec.TLSKeyPath + ":" + tenantTLSKeyMountPath + ":ro",
		},
		PortBindings: nat.PortMap{
			containerPort: []nat.PortBinding{{HostPort: hostPort}},
		},
		RestartPolicy: dockercontainer.RestartPolicy{Name: dockercontainer.RestartPolicyUnlessStopped},
		Resources: dockercontainer.Resources{
			Memory:   spec.MemoryBytes,
			NanoCPUs: spec.NanoCPUs,
		},
	}
	return config, hostConfig
}

func (c *sdkClient) CreateTenantContainer(ctx context.Context, spec entities.ContainerSpec) (entities.Container, error) {
	if _, _, err := c.cli.ImageInspectWithRaw(ctx, spec.Image); err != nil {
		rc, pullErr := c.cli.ImagePull(ctx, spec.Image, dockerimage.PullOptions{})
		if pullErr != nil {
			return entities.Container{}, fmt.Errorf("pull image %s: %w", spec.Image, pullErr)
		}
		_, _ = io.Copy(io.Discard, rc)
		_ = rc.Close()
	}

	if err := os.WriteFile(filepath.Join(spec.HostDataDir, tenantHBAConfName), []byte(tenantHBAConf()), 0o644); err != nil {
		return entities.Container{}, fmt.Errorf("write pg_hba.conf override: %w", err)
	}

	config, hostConfig := buildContainerConfig(spec)

	name := "mbr-" + spec.DatabaseID
	created, err := c.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, name)
	if err != nil {
		return entities.Container{}, fmt.Errorf("create container: %w", err)
	}
	if err := c.cli.ContainerStart(ctx, created.ID, dockercontainer.StartOptions{}); err != nil {
		if removeErr := c.cli.ContainerRemove(ctx, created.ID, dockercontainer.RemoveOptions{Force: true}); removeErr != nil {
			log.Printf("node: docker: remove container %s after failed start: %v", created.ID, removeErr)
		}
		return entities.Container{}, fmt.Errorf("start container: %w", err)
	}

	return entities.Container{ContainerID: created.ID, DatabaseID: spec.DatabaseID, Port: spec.Port, Status: "running"}, nil
}

func (c *sdkClient) RemoveTenantContainer(ctx context.Context, containerID string) error {
	if err := c.cli.ContainerStop(ctx, containerID, dockercontainer.StopOptions{}); err != nil && !dockerclient.IsErrNotFound(err) {
		return fmt.Errorf("stop container: %w", err)
	}
	if err := c.cli.ContainerRemove(ctx, containerID, dockercontainer.RemoveOptions{Force: true}); err != nil && !dockerclient.IsErrNotFound(err) {
		return fmt.Errorf("remove container: %w", err)
	}
	return nil
}
