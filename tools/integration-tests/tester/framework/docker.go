package framework

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
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
	client *client.Client
	id     string
}

// NewDockerContainer creates a new DockerContainer.
func NewDockerContainer(c *client.Client) *DockerContainer {
	return &DockerContainer{client: c}
}

// NewDockerContainerFromExisting creates a new DockerContainer from an already existing Docker container by name.
func NewDockerContainerFromExisting(c *client.Client, name string) (*DockerContainer, error) {
	containers, err := c.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	for _, cont := range containers {
		if cont.Names[0] == name {
			return &DockerContainer{
				client: c,
				id:     cont.ID,
			}, nil
		}
	}

	return nil, fmt.Errorf("could not find container with name '%s'", name)
}

// CreateGoShimmerEntryNode creates a new container with the GoShimmer entry node's configuration.
func (d *DockerContainer) CreateGoShimmerEntryNode(name string, seed string) error {
	containerConfig := &container.Config{
		Image:        "iotaledger/goshimmer",
		ExposedPorts: nil,
		Cmd: strslice.StrSlice{
			"--skip-config=true",
			"--logger.level=debug",
			fmt.Sprintf("--node.disablePlugins=%s", disabledPluginsEntryNode),
			"--autopeering.entryNodes=",
			fmt.Sprintf("--autopeering.seed=base58:%s", seed),
		},
	}

	return d.CreateContainer(name, containerConfig)
}

// CreateGoShimmerPeer creates a new container with the GoShimmer peer's configuration.
func (d *DockerContainer) CreateGoShimmerPeer(config GoShimmerConfig) error {
	// configure GoShimmer container instance
	containerConfig := &container.Config{
		Image: "iotaledger/goshimmer",
		ExposedPorts: nat.PortSet{
			nat.Port("8080/tcp"): {},
		},
		Cmd: strslice.StrSlice{
			"--skip-config=true",
			"--logger.level=debug",
			fmt.Sprintf("--valueLayer.fcob.averageNetworkDelay=%d", ParaFCoBAverageNetworkDelay),
			fmt.Sprintf("--node.disablePlugins=%s", config.DisabledPlugins),
			fmt.Sprintf("--pow.difficulty=%d", ParaPoWDifficulty),
			fmt.Sprintf("--faucet.powDifficulty=%d", ParaPoWFaucetDifficulty),
			fmt.Sprintf("--gracefulshutdown.waitToKillTime=%d", ParaWaitToKill),
			fmt.Sprintf("--node.enablePlugins=%s", func() string {
				var plugins []string
				if config.Faucet {
					plugins = append(plugins, "faucet")
				}
				if config.SyncBeacon {
					plugins = append(plugins, "SyncBeacon")
				}
				if config.SyncBeaconFollower {
					plugins = append(plugins, "SyncBeaconFollower")
				}
				return strings.Join(plugins[:], ",")
			}()),
			// define the faucet seed in case the faucet dApp is enabled
			func() string {
				if !config.Faucet {
					return ""
				}
				return fmt.Sprintf("--faucet.seed=%s", genesisSeedBase58)
			}(),
			fmt.Sprintf("--faucet.tokensPerRequest=%d", ParaFaucetTokensPerRequest),
			fmt.Sprintf("--valueLayer.snapshot.file=%s", config.SnapshotFilePath),
			"--webapi.bindAddress=0.0.0.0:8080",
			fmt.Sprintf("--autopeering.seed=base58:%s", config.Seed),
			fmt.Sprintf("--autopeering.entryNodes=%s@%s:14626", config.EntryNodePublicKey, config.EntryNodeHost),
			fmt.Sprintf("--fpc.roundInterval=%d", config.FPCRoundInterval),
			fmt.Sprintf("--fpc.listen=%v", config.FPCListen),
			fmt.Sprintf("--statement.writeStatement=%v", config.WriteStatement),
			fmt.Sprintf("--statement.waitForStatement=%d", config.WaitForStatement),
			fmt.Sprintf("--drng.custom.instanceId=%d", config.DRNGInstance),
			fmt.Sprintf("--drng.custom.threshold=%d", config.DRNGThreshold),
			fmt.Sprintf("--drng.custom.committeeMembers=%s", config.DRNGCommittee),
			fmt.Sprintf("--drng.custom.distributedPubKey=%s", config.DRNGDistKey),
			fmt.Sprintf("--drng.xteam.committeeMembers="),
			fmt.Sprintf("--drng.pollen.committeeMembers="),
			fmt.Sprintf("--syncbeaconfollower.followNodes=%s", config.SyncBeaconFollowNodes),
			fmt.Sprintf("--syncbeacon.broadcastInterval=%d", config.SyncBeaconBroadcastInterval),
			"--syncbeacon.startSynced=true",
			func() string {
				if config.SyncBeaconMaxTimeOfflineSec == 0 {
					return ""
				}
				return fmt.Sprintf("--syncbeaconfollower.maxTimeOffline=%d", config.SyncBeaconMaxTimeOfflineSec)
			}(),
		},
	}

	return d.CreateContainer(config.Name, containerConfig, &container.HostConfig{
		Binds: []string{"goshimmer-testing-assets:/assets:rw"},
	})
}

// CreateDrandMember creates a new container with the drand configuration.
func (d *DockerContainer) CreateDrandMember(name string, goShimmerAPI string, leader bool) error {
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

	return d.CreateContainer(name, containerConfig)
}

// CreatePumba creates a new container with Pumba configuration.
func (d *DockerContainer) CreatePumba(name string, containerName string, targetIPs []string) error {
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
		containerName,
	}
	cmd = append(cmd, slice...)

	containerConfig := &container.Config{
		Image: "gaiaadm/pumba:0.7.2",
		Cmd:   cmd,
	}

	return d.CreateContainer(name, containerConfig, hostConfig)
}

// CreateContainer creates a new container with the given configuration.
func (d *DockerContainer) CreateContainer(name string, containerConfig *container.Config, hostConfigs ...*container.HostConfig) error {
	var hostConfig *container.HostConfig
	if len(hostConfigs) > 0 {
		hostConfig = hostConfigs[0]
	}

	resp, err := d.client.ContainerCreate(context.Background(), containerConfig, hostConfig, nil, name)
	if err != nil {
		return err
	}

	d.id = resp.ID
	return nil
}

// ConnectToNetwork connects a container to an existent network in the docker host.
func (d *DockerContainer) ConnectToNetwork(networkID string) error {
	return d.client.NetworkConnect(context.Background(), networkID, d.id, nil)
}

// DisconnectFromNetwork disconnects a container from an existent network in the docker host.
func (d *DockerContainer) DisconnectFromNetwork(networkID string) error {
	return d.client.NetworkDisconnect(context.Background(), networkID, d.id, true)
}

// Start sends a request to the docker daemon to start a container.
func (d *DockerContainer) Start() error {
	return d.client.ContainerStart(context.Background(), d.id, types.ContainerStartOptions{})
}

// Remove kills and removes a container from the docker host.
func (d *DockerContainer) Remove() error {
	return d.client.ContainerRemove(context.Background(), d.id, types.ContainerRemoveOptions{Force: true})
}

// Stop stops a container without terminating the process.
// The process is blocked until the container stops or the timeout expires.
func (d *DockerContainer) Stop(optionalTimeout ...time.Duration) error {
	duration := 3 * time.Minute
	if optionalTimeout != nil {
		duration = optionalTimeout[0]
	}
	return d.client.ContainerStop(context.Background(), d.id, &duration)
}

// ExitStatus returns the exit status according to the container information.
func (d *DockerContainer) ExitStatus() (int, error) {
	resp, err := d.client.ContainerInspect(context.Background(), d.id)
	if err != nil {
		return -1, err
	}

	return resp.State.ExitCode, nil
}

// IP returns the IP address according to the container information for the given network.
func (d *DockerContainer) IP(network string) (string, error) {
	resp, err := d.client.ContainerInspect(context.Background(), d.id)
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
func (d *DockerContainer) Logs() (io.ReadCloser, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      "",
		Timestamps: false,
		Follow:     false,
		Tail:       "",
		Details:    false,
	}

	return d.client.ContainerLogs(context.Background(), d.id, options)
}
