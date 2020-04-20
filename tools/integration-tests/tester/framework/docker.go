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

func NewDockerClient() *client.Client {
	c, err := client.NewClient(
		"unix:///var/run/docker.sock",
		"",
		nil,
		nil,
	)
	if err != nil {
		fmt.Println("Could not create docker CLI client.")
		panic(err)
	}

	return c
}

type DockerContainer struct {
	client *client.Client
	id     string
}

func NewDockerContainer(c *client.Client) *DockerContainer {
	return &DockerContainer{client: c}
}

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

func (d *DockerContainer) CreateGoShimmer(name string, entryNode string) {
	// TODO: identities?
	// configure GoShimmer container instance
	var containerConfig *container.Config
	if entryNode == "" {
		containerConfig = &container.Config{
			Image:        "iotaledger/goshimmer",
			ExposedPorts: nil,
			Cmd: strslice.StrSlice{
				"--node.disablePlugins=portcheck,spa,analysis,gossip,webapi,webapibroadcastdataendpoint,webapifindtransactionhashesendpoint,webapigetneighborsendpoint,webapigettransactionobjectsbyhashendpoint,webapigettransactiontrytesbyhashendpoint",
				"--autopeering.entryNodes=",
				"--autopeering.seed=uuDCzsjyLNQ17/7fWKPNMYmr4IWuaVRf7qKqRL0v/6c=",
			},
		}
	} else {
		containerConfig = &container.Config{
			Image: "iotaledger/goshimmer",
			ExposedPorts: nat.PortSet{
				nat.Port("8080/tcp"): {},
			},
			Cmd: strslice.StrSlice{
				"--node.disablePlugins=portcheck,spa,analysis",
				"--webapi.bindAddress=0.0.0.0:8080",
				fmt.Sprintf("--autopeering.entryNodes=X2cmCzYnZDjmsvdAH90Q7oKmhNeTdwJdj2FX84adLzo=@%s:14626", entryNode),
			},
		}
	}

	resp, err := d.client.ContainerCreate(context.Background(), containerConfig, nil, nil, name)
	if err != nil {
		panic(err)
	}

	d.id = resp.ID
}

func (d *DockerContainer) ConnectToNetwork(networkId string) {
	err := d.client.NetworkConnect(context.Background(), networkId, d.id, nil)
	if err != nil {
		panic(err)
	}
}

func (d *DockerContainer) DisconnectFromNetwork(networkId string) {
	err := d.client.NetworkDisconnect(context.Background(), networkId, d.id, true)
	if err != nil {
		panic(err)
	}
}

func (d *DockerContainer) Start() {
	if err := d.client.ContainerStart(context.Background(), d.id, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}
}

func (d *DockerContainer) Remove() {
	err := d.client.ContainerRemove(context.Background(), d.id, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		panic(err)
	}
}

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
		ShowStderr: false,
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
