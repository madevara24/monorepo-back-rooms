// Package repository implements the agent's Docker and volume adapters:
// the only package that imports the Docker SDK or shells out to mount
// and mkfs.ext4.
package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerfilters "github.com/docker/docker/api/types/filters"
	dockerimage "github.com/docker/docker/api/types/image"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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
	// CollectConnectionMetrics runs one bounded, read-only query inside a
	// running tenant container over its local socket and trust auth, and
	// inspects the container's configured memory limit. It returns an
	// error if the container is not reachable or the query fails.
	CollectConnectionMetrics(ctx context.Context, containerID string) (entities.ConnectionMetrics, error)
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
		LogConfig: dockercontainer.LogConfig{
			Type: "json-file",
			Config: map[string]string{
				"max-size": "10m",
				"max-file": "3",
			},
		},
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

// connectionStatsTimeout bounds the whole collection (inspect + exec) for
// one container, so one slow or hung tenant cannot stall the agent's
// heartbeat tick.
const connectionStatsTimeout = 5 * time.Second

// connectionStatsStatementTimeoutMS is set via PGOPTIONS at connection
// startup, so the query below cannot itself run long on a wedged backend.
const connectionStatsStatementTimeoutMS = "2000"

// connectionStatsQuery counts backends by state and reads the two
// connection-limit GUCs in a single round trip: the one bounded query per
// tenant the task calls for. It runs as the postgres OS user over the
// container's local socket, which tenantHBAConf's "local all all trust"
// rule accepts without a password. It excludes its own backend
// (pg_backend_pid()), which would otherwise always count itself as one
// "active" connection and bias every snapshot upward by one.
const connectionStatsQuery = `SELECT
	count(*) FILTER (WHERE state = 'active'),
	count(*) FILTER (WHERE state = 'idle'),
	count(*) FILTER (WHERE state = 'idle in transaction'),
	(SELECT setting FROM pg_settings WHERE name = 'max_connections'),
	(SELECT setting FROM pg_settings WHERE name = 'superuser_reserved_connections')
FROM pg_stat_activity
WHERE pid <> pg_backend_pid();`

func (c *sdkClient) CollectConnectionMetrics(ctx context.Context, containerID string) (entities.ConnectionMetrics, error) {
	ctx, cancel := context.WithTimeout(ctx, connectionStatsTimeout)
	defer cancel()

	inspect, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return entities.ConnectionMetrics{}, fmt.Errorf("inspect container: %w", err)
	}
	var memoryLimitBytes int64
	if inspect.HostConfig != nil {
		memoryLimitBytes = inspect.HostConfig.Memory
	}

	created, err := c.cli.ContainerExecCreate(ctx, containerID, dockercontainer.ExecOptions{
		Cmd:          []string{"psql", "-U", "postgres", "-d", "postgres", "-At", "-F", ",", "-c", connectionStatsQuery},
		Env:          []string{"PGOPTIONS=-c statement_timeout=" + connectionStatsStatementTimeoutMS},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return entities.ConnectionMetrics{}, fmt.Errorf("create exec: %w", err)
	}

	attach, err := c.cli.ContainerExecAttach(ctx, created.ID, dockercontainer.ExecAttachOptions{})
	if err != nil {
		return entities.ConnectionMetrics{}, fmt.Errorf("attach exec: %w", err)
	}
	defer attach.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, attach.Reader); err != nil {
		return entities.ConnectionMetrics{}, fmt.Errorf("read exec output: %w", err)
	}

	inspectExec, err := c.cli.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return entities.ConnectionMetrics{}, fmt.Errorf("inspect exec: %w", err)
	}
	if inspectExec.ExitCode != 0 {
		return entities.ConnectionMetrics{}, fmt.Errorf("psql exited %d: %s", inspectExec.ExitCode, strings.TrimSpace(stderr.String()))
	}

	metrics, err := parseConnectionStats(stdout.String())
	if err != nil {
		return entities.ConnectionMetrics{}, err
	}
	metrics.MemoryLimitBytes = memoryLimitBytes
	return metrics, nil
}

// parseConnectionStats parses the one-line, comma-separated output of
// connectionStatsQuery.
func parseConnectionStats(out string) (entities.ConnectionMetrics, error) {
	line := strings.TrimSpace(out)
	fields := strings.Split(line, ",")
	if len(fields) != 5 {
		return entities.ConnectionMetrics{}, fmt.Errorf("unexpected connection stats output %q", line)
	}

	values := make([]int, len(fields))
	for i, f := range fields {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			return entities.ConnectionMetrics{}, fmt.Errorf("parse connection stats output %q: %w", line, err)
		}
		values[i] = n
	}

	return entities.ConnectionMetrics{
		Active:                       values[0],
		Idle:                         values[1],
		IdleInTransaction:            values[2],
		MaxConnections:               values[3],
		SuperuserReservedConnections: values[4],
	}, nil
}
