package framework

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
)

// newDockerClient creates a Docker client that communicates via the Docker socket.
func newDockerClient() (*client.Client, error) {
	return client.NewClient(
		"unix:///var/run/docker.sock",
		"",
		nil,
		nil,
	)
}

// DockerContainer is a wrapper object for a Docker container.
type DockerContainer struct {
	name string
	id   string

	client *client.Client
}

// NewDockerContainer creates a new DockerContainer.
func NewDockerContainer(c *client.Client) *DockerContainer {
	return &DockerContainer{client: c}
}

// NewDockerContainerFromExisting creates a new DockerContainer from an already existing Docker container by name.
func NewDockerContainerFromExisting(ctx context.Context, c *client.Client, name string) (*DockerContainer, error) {
	containers, err := c.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	for _, cont := range containers {
		if cont.Names[0] == name {
			return &DockerContainer{
				name:   name,
				id:     cont.ID,
				client: c,
			}, nil
		}
	}

	return nil, fmt.Errorf("could not find container with name '%s'", name)
}

// CreateNode creates a new GoShimmer container.
func (d *DockerContainer) CreateNode(ctx context.Context, conf config.GoShimmer) error {
	cmd := strslice.StrSlice{
		"--skip-config=true",
		"--logger.level=debug",
	}
	cmd = append(cmd, conf.CreateFlags()...)

	// configure GoShimmer container instance
	containerConfig := &container.Config{
		Image: "iotaledger/goshimmer",
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/tcp", apiPort)):     {},
			nat.Port(fmt.Sprintf("%d/tcp", gossipPort)):  {},
			nat.Port(fmt.Sprintf("%d/udp", peeringPort)): {},
			nat.Port(fmt.Sprintf("%d/tcp", fpcPort)):     {},
		},
		Cmd: cmd,
	}
	log.Printf("Start %s %v", containerConfig.Image, containerConfig.Cmd)

	return d.CreateContainer(ctx, conf.Name, containerConfig, &container.HostConfig{
		Binds: []string{"goshimmer-testing-assets:/assets:rw"},
	})
}

// CreateDrandMember creates a new container with the drand configuration.
func (d *DockerContainer) CreateDrandMember(ctx context.Context, name string, goShimmerAPI string, leader bool) error {
	// configure drand container instance
	env := []string{}
	if leader {
		env = append(env, "LEADER=1")
	}
	env = append(env, "GOSHIMMER=http://"+goShimmerAPI)
	containerConfig := &container.Config{
		Image: "angelocapossele/drand:v1.1.4",
		ExposedPorts: nat.PortSet{
			nat.Port("8000/tcp"): {},
		},
		Env:        env,
		Entrypoint: strslice.StrSlice{"/data/client-script.sh"},
	}

	return d.CreateContainer(ctx, name, containerConfig)
}

// CreatePumba creates a new container with Pumba configuration blocking all traffic.
// This blocks all traffic of effectedContainer to targetIPs.
func (d *DockerContainer) CreatePumba(ctx context.Context, name string, effectedContainerName string, targetIPs []string) error {
	hostConfig := &container.HostConfig{
		Binds: strslice.StrSlice{"/var/run/docker.sock:/var/run/docker.sock:ro"},
	}

	cmd := strslice.StrSlice{
		"--log-level=debug",
		"netem",
		"--duration=100m",
	}

	for _, ip := range targetIPs {
		targetFlag := "--target=" + ip
		cmd = append(cmd, targetFlag)
	}

	slice := strslice.StrSlice{
		"--tc-image=gaiadocker/iproute2",
		"loss",
		"--percent=100",
		effectedContainerName,
	}
	cmd = append(cmd, slice...)

	containerConfig := &container.Config{
		Image: "gaiaadm/pumba:0.7.2",
		Cmd:   cmd,
	}

	return d.CreateContainer(ctx, name, containerConfig, hostConfig)
}

// CreateContainer creates a new container with the given configuration.
func (d *DockerContainer) CreateContainer(ctx context.Context, name string, containerConfig *container.Config, hostConfigs ...*container.HostConfig) error {
	var hostConfig *container.HostConfig
	if len(hostConfigs) > 0 {
		hostConfig = hostConfigs[0]
	}

	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, name)
	if err != nil {
		return err
	}

	d.id = resp.ID
	d.name = name
	return nil
}

// ConnectToNetwork connects a container to an existent network in the docker host.
func (d *DockerContainer) ConnectToNetwork(ctx context.Context, networkID string) error {
	return d.client.NetworkConnect(ctx, networkID, d.id, nil)
}

// DisconnectFromNetwork disconnects a container from an existent network in the docker host.
func (d *DockerContainer) DisconnectFromNetwork(ctx context.Context, networkID string) error {
	return d.client.NetworkDisconnect(ctx, networkID, d.id, true)
}

// Start sends a request to the docker daemon to start a container.
func (d *DockerContainer) Start(ctx context.Context) error {
	return d.client.ContainerStart(ctx, d.id, types.ContainerStartOptions{})
}

// Remove kills and removes a container from the docker host.
func (d *DockerContainer) Remove(ctx context.Context) error {
	return d.client.ContainerRemove(ctx, d.id, types.ContainerRemoveOptions{Force: true})
}

// Stop stops a container without terminating the process.
// The process is blocked until the container stops or the timeout expires.
func (d *DockerContainer) Stop(ctx context.Context, optionalTimeout ...time.Duration) error {
	duration := 3 * time.Minute
	if optionalTimeout != nil {
		duration = optionalTimeout[0]
	}
	return d.client.ContainerStop(ctx, d.id, &duration)
}

// ExitStatus returns the exit status according to the container information.
func (d *DockerContainer) ExitStatus(ctx context.Context) (int, error) {
	resp, err := d.client.ContainerInspect(ctx, d.id)
	if err != nil {
		return -1, err
	}

	return resp.State.ExitCode, nil
}

// IP returns the IP address according to the container information for the given network.
func (d *DockerContainer) IP(ctx context.Context, network string) (string, error) {
	resp, err := d.client.ContainerInspect(ctx, d.id)
	if err != nil {
		return "", err
	}

	for name, v := range resp.NetworkSettings.Networks {
		if name == network {
			return v.IPAddress, nil
		}
	}

	return "", fmt.Errorf("IP address in %s could not be determined", network)
}

// Logs returns the logs of the container as io.ReadCloser.
func (d *DockerContainer) Logs(ctx context.Context) (io.ReadCloser, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      "",
		Timestamps: false,
		Follow:     false,
		Tail:       "",
		Details:    false,
	}
	return d.client.ContainerLogs(ctx, d.id, options)
}

func (d *DockerContainer) Shutdown(ctx context.Context, optionalTimeout ...time.Duration) (exitStatus int, err error) {
	err = d.Stop(ctx, optionalTimeout...)
	if err != nil {
		return 0, err
	}
	logs, err := d.Logs(ctx)
	if err != nil {
		return 0, err
	}
	err = createLogFile(d.name, logs)
	if err != nil {
		return 0, err
	}
	return d.ExitStatus(ctx)
}
