// Package node implements the MBR data-plane agent (mbr-node): the only
// process on a data-plane host that is meant to talk to Docker.
package node

import (
	"context"
	"fmt"
	"io"
	"strconv"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerfilters "github.com/docker/docker/api/types/filters"
	dockerimage "github.com/docker/docker/api/types/image"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
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

// Container is the subset of a tenant container's Docker state the agent
// exposes over its API.
type Container struct {
	ContainerID string
	DatabaseID  string
	Port        int
	Status      string
}

// ContainerSpec describes the tenant container to create.
type ContainerSpec struct {
	DatabaseID  string
	Image       string
	Password    string
	Port        int
	HostDataDir string
	MemoryBytes int64
	NanoCPUs    int64
}

// DockerClient is the subset of the Docker API the agent needs. Handlers
// depend on this interface rather than the SDK client directly, so they
// can be tested against a fake.
type DockerClient interface {
	// ListTenantContainers returns every container labeled
	// mbr.tenant=true, regardless of state.
	ListTenantContainers(ctx context.Context) ([]Container, error)
	// UsedHostPorts returns the host ports currently published by any
	// container, tenant or not.
	UsedHostPorts(ctx context.Context) (map[int]bool, error)
	// CreateTenantContainer pulls the image if needed and starts a new
	// labeled tenant container.
	CreateTenantContainer(ctx context.Context, spec ContainerSpec) (Container, error)
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

func toContainer(s dockertypes.Container) Container {
	port, _ := strconv.Atoi(s.Labels[LabelPort])
	return Container{
		ContainerID: s.ID,
		DatabaseID:  s.Labels[LabelDatabaseID],
		Port:        port,
		Status:      s.State,
	}
}

func (c *sdkClient) ListTenantContainers(ctx context.Context) ([]Container, error) {
	summaries, err := c.cli.ContainerList(ctx, dockercontainer.ListOptions{All: true, Filters: tenantFilter()})
	if err != nil {
		return nil, fmt.Errorf("list tenant containers: %w", err)
	}
	out := make([]Container, len(summaries))
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

func (c *sdkClient) CreateTenantContainer(ctx context.Context, spec ContainerSpec) (Container, error) {
	if _, _, err := c.cli.ImageInspectWithRaw(ctx, spec.Image); err != nil {
		rc, pullErr := c.cli.ImagePull(ctx, spec.Image, dockerimage.PullOptions{})
		if pullErr != nil {
			return Container{}, fmt.Errorf("pull image %s: %w", spec.Image, pullErr)
		}
		_, _ = io.Copy(io.Discard, rc)
		_ = rc.Close()
	}

	containerPort := nat.Port(postgresContainerPort)
	hostPort := strconv.Itoa(spec.Port)

	config := &dockercontainer.Config{
		Image: spec.Image,
		Env: []string{
			"POSTGRES_PASSWORD=" + spec.Password,
			"POSTGRES_DB=postgres",
		},
		ExposedPorts: nat.PortSet{containerPort: struct{}{}},
		Labels: map[string]string{
			LabelTenant:     "true",
			LabelDatabaseID: spec.DatabaseID,
			LabelPort:       hostPort,
		},
	}
	hostConfig := &dockercontainer.HostConfig{
		Binds: []string{spec.HostDataDir + ":/var/lib/postgresql/data"},
		PortBindings: nat.PortMap{
			containerPort: []nat.PortBinding{{HostPort: hostPort}},
		},
		RestartPolicy: dockercontainer.RestartPolicy{Name: dockercontainer.RestartPolicyUnlessStopped},
		Resources: dockercontainer.Resources{
			Memory:   spec.MemoryBytes,
			NanoCPUs: spec.NanoCPUs,
		},
	}

	name := "mbr-" + spec.DatabaseID
	created, err := c.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, name)
	if err != nil {
		return Container{}, fmt.Errorf("create container: %w", err)
	}
	if err := c.cli.ContainerStart(ctx, created.ID, dockercontainer.StartOptions{}); err != nil {
		return Container{}, fmt.Errorf("start container: %w", err)
	}

	return Container{ContainerID: created.ID, DatabaseID: spec.DatabaseID, Port: spec.Port, Status: "running"}, nil
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
