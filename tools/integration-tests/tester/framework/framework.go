// Package framework provides integration test functionality for GoShimmer with a Docker network.
// It effectively abstracts away all complexity with creating a custom Docker network per test,
// discovering peers, waiting for them to autopeer and offers easy access to the peers' web API and logs.
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

// Framework is a wrapper that provides the integration testing functionality.
type Framework struct {
	tester       *DockerContainer
	dockerClient *client.Client
}

// Instance returns the singleton Framework instance.
func Instance() (f *Framework, err error) {
	once.Do(func() {
		f, err = newFramework()
		instance = f
	})

	return instance, err
}

// newFramework creates a new instance of Framework, creates a DockerClient
// and creates a DockerContainer for the tester container where the tests are running in.
func newFramework() (*Framework, error) {
	dockerClient, err := newDockerClient()
	if err != nil {
		return nil, err
	}

	tester, err := NewDockerContainerFromExisting(dockerClient, containerNameTester)
	if err != nil {
		return nil, err
	}

	f := &Framework{
		dockerClient: dockerClient,
		tester:       tester,
	}

	return f, nil
}

// CreateNetwork creates and returns a (Docker) Network that contains `peers` GoShimmer nodes.
// It waits for the peers to autopeer until the minimum neighbors criteria is met for every peer.
func (f *Framework) CreateNetwork(name string, peers int, minimumNeighbors int) (*Network, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), f.tester)
	if err != nil {
		return nil, err
	}

	err = network.createEntryNode()
	if err != nil {
		return nil, err
	}

	// create peers/GoShimmer nodes
	for i := 0; i < peers; i++ {
		_, err = network.CreatePeer()
		if err != nil {
			return nil, err
		}
	}

	// wait until containers are fully started
	time.Sleep(1 * time.Second)
	err = network.WaitForAutopeering(minimumNeighbors)
	if err != nil {
		return nil, err
	}

	return network, nil
}
