// Package framework provides integration test functionality for GoShimmer with a Docker network.
// It effectively abstracts away all complexity with discovering peers,
// waiting for them to autopeer and offers easy access to the peers' web API
// and logs via Docker.
package framework

import (
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
)

var (
	once     sync.Once
	instance *Framework
)

// Framework is a wrapper encapsulating all peers
type Framework struct {
	// TODO: this list needs to be globally managed and properly cleaned up at shutdown
	peers        []*Peer
	tester       *DockerContainer
	dockerClient *client.Client
}

func Instance() *Framework {
	once.Do(func() {
		instance = newFramework()
	})

	return instance
}

// New creates a new instance of Framework, gets all available peers within the Docker network and
// waits for them to autopeer.
// Panics if no peer is found.
func newFramework() *Framework {
	dockerClient := NewDockerClient()
	f := &Framework{
		dockerClient: dockerClient,
		tester:       NewDockerContainerFromExisting(dockerClient, containerNameTester),
	}

	return f
}

func (f *Framework) CreateNetwork(name string, peers int) *Network {
	network := newNetwork(f.dockerClient, strings.ToLower(name), f.tester)
	// create entry_node
	network.createEntryNode()

	// create replicas
	for i := 0; i < peers; i++ {
		network.CreatePeer()
	}

	// wait until containers are fully started
	time.Sleep(1 * time.Second)
	network.WaitForAutopeering()

	return network
}
