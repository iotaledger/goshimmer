package framework

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
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
	Name string
	Id   string

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
		if cont.Names[0] == name || strings.HasPrefix(cont.ID, name) {
			return &DockerContainer{
				Name:   cont.Names[0],
				Id:     cont.ID,
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

	if conf.Image == "" {
		return fmt.Errorf("docker image must be provided as part of the configuration")
	}

	// configure GoShimmer container instance
	containerConfig := &container.Config{
		Image: conf.Image,
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/tcp", apiPort)):       {},
			nat.Port(fmt.Sprintf("%d/tcp", dashboardPort)): {},
			nat.Port(fmt.Sprintf("%d/tcp", dagVizPort)):    {},
			nat.Port(fmt.Sprintf("%d/tcp", gossipPort)):    {},
			nat.Port(fmt.Sprintf("%d/udp", peeringPort)):   {},
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
	var env []string
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

func (d *DockerContainer) createSocatContainer(ctx context.Context, name string, targetContainer string, portMapping map[int]config.GoShimmerPort) error {
	// create host configs
	portBindings := make(nat.PortMap, len(portMapping))
	exposedPorts := make(nat.PortSet, len(portMapping))
	for srcPort, targetPort := range portMapping {
		port, err := nat.NewPort("tcp", strconv.Itoa(int(targetPort)))
		if err != nil {
			return err
		}

		portBindings[port] = []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: strconv.Itoa(srcPort),
			},
		}

		exposedPorts[port] = struct{}{}
	}

	hostConfig := &container.HostConfig{
		Binds:        strslice.StrSlice{"/var/run/docker.sock:/var/run/docker.sock:ro"},
		PortBindings: portBindings,
	}

	cmd := ""
	for _, targetPort := range portMapping {
		cmd += fmt.Sprintf("socat tcp-listen:%d,fork,reuseaddr tcp-connect:%s:%d & ", targetPort, targetContainer, targetPort)
	}

	// create container configs
	containerConfig := &container.Config{
		Image:        "alpine/socat:1.7.4.3-r0",
		Entrypoint:   []string{"/bin/sh"},
		Cmd:          []string{"-c", cmd + "wait"},
		ExposedPorts: exposedPorts,
	}

	return d.CreateContainer(ctx, name, containerConfig, hostConfig)

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

	d.Id = resp.ID
	d.Name = name
	return nil
}

// ConnectToNetwork connects a container to an existent network in the docker host.
func (d *DockerContainer) ConnectToNetwork(ctx context.Context, networkID string) error {
	return d.client.NetworkConnect(ctx, networkID, d.Id, nil)
}

// DisconnectFromNetwork disconnects a container from an existent network in the docker host.
func (d *DockerContainer) DisconnectFromNetwork(ctx context.Context, networkID string) error {
	return d.client.NetworkDisconnect(ctx, networkID, d.Id, true)
}

// Start sends a request to the docker daemon to start a container.
func (d *DockerContainer) Start(ctx context.Context) error {
	return d.client.ContainerStart(ctx, d.Id, types.ContainerStartOptions{})
}

// Stop stops the container without terminating the process.
// The process is blocked until the container stops or the timeout expires.
func (d *DockerContainer) Stop(ctx context.Context, optionalTimeout ...time.Duration) error {
	duration := 3 * time.Minute
	if optionalTimeout != nil {
		duration = optionalTimeout[0]
	}
	return d.client.ContainerStop(ctx, d.Id, &duration)
}

// Kill sends a signal to the container
func (d *DockerContainer) Kill(ctx context.Context, signal string) error {
	return d.client.ContainerKill(ctx, d.Id, signal)
}

// ExitStatus returns the exit status according to the container information.
func (d *DockerContainer) ExitStatus(ctx context.Context) (int, error) {
	resp, err := d.client.ContainerInspect(ctx, d.Id)
	if err != nil {
		return -1, err
	}

	return resp.State.ExitCode, nil
}

// Shutdown stops the container and stores its log. It returns the exit status of the process.
func (d *DockerContainer) Shutdown(ctx context.Context, optionalTimeout ...time.Duration) (exitStatus int, err error) {
	err = d.Stop(ctx, optionalTimeout...)
	if err != nil {
		return 0, err
	}
	logs, err := d.Logs(ctx)
	if err != nil {
		return 0, err
	}
	err = createLogFile(d.Name, logs)
	if err != nil {
		return 0, err
	}
	return d.ExitStatus(ctx)
}

// Remove kills and removes a container from the docker host.
func (d *DockerContainer) Remove(ctx context.Context) error {
	return d.client.ContainerRemove(ctx, d.Id, types.ContainerRemoveOptions{Force: true})
}

// IP returns the IP address according to the container information for the given network.
func (d *DockerContainer) IP(ctx context.Context, network string) (string, error) {
	resp, err := d.client.ContainerInspect(ctx, d.Id)
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
	return d.client.ContainerLogs(ctx, d.Id, options)
}
