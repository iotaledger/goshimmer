package framework

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// newDockerClient creates a Docker client that communicates via the Docker socket.
func newDockerClient() *client.Client {
	c, err := client.NewClient(
		"unix:///var/run/docker.sock",
		"",
		nil,
		nil,
	)
	if err != nil {
		panic(err)
	}

	return c
}

// Wrapper object for a Docker container.
type DockerContainer struct {
	client *client.Client
	id     string
}

// NewDockerContainer creates a new DockerContainer.
func NewDockerContainer(c *client.Client) *DockerContainer {
	return &DockerContainer{client: c}
}

// NewDockerContainerFromExisting creates a new DockerContainer from an already existing Docker container by name.
func NewDockerContainerFromExisting(c *client.Client, name string) *DockerContainer {
	containers, err := c.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, cont := range containers {
		if cont.Names[0] == name {
			return &DockerContainer{
				client: c,
				id:     cont.ID,
			}
		}
	}

	panic(fmt.Sprintf("Could not find container with name '%s'", name))
}

// CreateGoShimmerEntryNode creates a new container with the GoShimmer entry node's configuration.
func (d *DockerContainer) CreateGoShimmerEntryNode(name string, seed string) {
	containerConfig := &container.Config{
		Image:        "iotaledger/goshimmer",
		ExposedPorts: nil,
		Cmd: strslice.StrSlice{
			fmt.Sprintf("--node.disablePlugins=%s", disabledPluginsEntryNode),
			"--autopeering.entryNodes=",
			fmt.Sprintf("--autopeering.seed=%s", seed),
		},
	}

	d.CreateContainer(name, containerConfig)
}

// CreateGoShimmerPeer creates a new container with the GoShimmer peer's configuration.
func (d *DockerContainer) CreateGoShimmerPeer(name string, seed string, entryNodeHost string, entryNodePublicKey string) {
	// configure GoShimmer container instance
	containerConfig := &container.Config{
		Image: "iotaledger/goshimmer",
		ExposedPorts: nat.PortSet{
			nat.Port("8080/tcp"): {},
		},
		Cmd: strslice.StrSlice{
			fmt.Sprintf("--node.disablePlugins=%s", disabledPluginsPeer),
			"--webapi.bindAddress=0.0.0.0:8080",
			fmt.Sprintf("--autopeering.seed=%s", seed),
			fmt.Sprintf("--autopeering.entryNodes=%s@%s:14626", entryNodePublicKey, entryNodeHost),
		},
	}

	d.CreateContainer(name, containerConfig)
}

// CreateContainer creates a new container with the given configuration.
func (d *DockerContainer) CreateContainer(name string, containerConfig *container.Config) {
	resp, err := d.client.ContainerCreate(context.Background(), containerConfig, nil, nil, name)
	if err != nil {
		panic(err)
	}

	d.id = resp.ID
}

// ConnectToNetwork connects a container to an existent network in the docker host.
func (d *DockerContainer) ConnectToNetwork(networkId string) {
	err := d.client.NetworkConnect(context.Background(), networkId, d.id, nil)
	if err != nil {
		panic(err)
	}
}

// DisconnectFromNetwork disconnects a container from an existent network in the docker host.
func (d *DockerContainer) DisconnectFromNetwork(networkId string) {
	err := d.client.NetworkDisconnect(context.Background(), networkId, d.id, true)
	if err != nil {
		panic(err)
	}
}

// Start sends a request to the docker daemon to start a container.
func (d *DockerContainer) Start() {
	if err := d.client.ContainerStart(context.Background(), d.id, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}
}

// Remove kills and removes a container from the docker host.
func (d *DockerContainer) Remove() {
	err := d.client.ContainerRemove(context.Background(), d.id, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		panic(err)
	}
}

// Stop stops a container without terminating the process.
// The process is blocked until the container stops or the timeout expires.
func (d *DockerContainer) Stop() {
	duration := 10 * time.Second
	err := d.client.ContainerStop(context.Background(), d.id, &duration)
	if err != nil {
		panic(err)
	}
}

// Logs returns the logs of the container as io.ReadCloser.
func (d *DockerContainer) Logs() io.ReadCloser {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      "",
		Timestamps: false,
		Follow:     false,
		Tail:       "",
		Details:    false,
	}

	out, err := d.client.ContainerLogs(context.Background(), d.id, options)
	if err != nil {
		panic(err)
	}

	return out
}
