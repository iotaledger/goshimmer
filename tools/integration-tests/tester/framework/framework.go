// Package framework provides integration test functionality for GoShimmer with a Docker network.
// It effectively abstracts away all complexity with creating a custom Docker network per test,
// discovering peers, waiting for them to autopeer and offers easy access to the peers' web API and logs.
package framework

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	hive_ed25519 "github.com/iotaledger/hive.go/crypto/ed25519"
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
// The first peer automatically starts with the bootstrap plugin enabled.
func (f *Framework) CreateNetwork(name string, peers int, minimumNeighbors int, networkConfig ...NetworkConfig) (*Network, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), f.tester)
	if err != nil {
		return nil, err
	}

	err = network.createEntryNode()
	if err != nil {
		return nil, err
	}

	// configuration of bootstrap plugin
	bootstrapInitialIssuanceTimePeriodSec := -1
	if len(networkConfig) > 0 {
		bootstrapInitialIssuanceTimePeriodSec = networkConfig[0].BootstrapInitialIssuanceTimePeriodSec
	}

	// create peers/GoShimmer nodes
	for i := 0; i < peers; i++ {
		config := GoShimmerConfig{
			Bootstrap:                             i == 0,
			BootstrapInitialIssuanceTimePeriodSec: bootstrapInitialIssuanceTimePeriodSec,
		}
		if _, err = network.CreatePeer(config); err != nil {
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

// CreateNetworkWithPartitions creates and returns a partitioned network that contains `peers` GoShimmer nodes per partition.
// It waits for the peers to autopeer until the minimum neighbors criteria is met for every peer.
// The first peer automatically starts with the bootstrap plugin enabled.
func (f *Framework) CreateNetworkWithPartitions(name string, peers, partitions, minimumNeighbors int) (*Network, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), f.tester)
	if err != nil {
		return nil, err
	}

	err = network.createEntryNode()
	if err != nil {
		return nil, err
	}

	// block all traffic from/to entry node
	pumbaEntryNodeName := network.namePrefix(containerNameEntryNode) + containerNameSuffixPumba
	pumbaEntryNode, err := network.createPumba(
		pumbaEntryNodeName,
		network.namePrefix(containerNameEntryNode),
		strslice.StrSlice{},
	)
	if err != nil {
		return nil, err
	}
	// wait until pumba is started and blocks all traffic
	time.Sleep(5 * time.Second)

	// create peers/GoShimmer nodes
	for i := 0; i < peers; i++ {
		config := GoShimmerConfig{
			Bootstrap: i == 0,
		}
		if _, err = network.CreatePeer(config); err != nil {
			return nil, err
		}
	}
	// wait until containers are fully started
	time.Sleep(2 * time.Second)

	// create partitions
	chunkSize := peers / partitions
	var end int
	for i := 0; end < peers; i += chunkSize {
		end = i + chunkSize
		// last partitions takes the rest
		if i/chunkSize == partitions-1 {
			end = peers
		}
		_, err = network.createPartition(network.peers[i:end])
		if err != nil {
			return nil, err
		}
	}
	// wait until pumba containers are started and block traffic between partitions
	time.Sleep(5 * time.Second)

	// delete pumba for entry node
	err = pumbaEntryNode.Stop()
	if err != nil {
		return nil, err
	}
	logs, err := pumbaEntryNode.Logs()
	if err != nil {
		return nil, err
	}
	err = createLogFile(pumbaEntryNodeName, logs)
	if err != nil {
		return nil, err
	}
	err = pumbaEntryNode.Remove()
	if err != nil {
		return nil, err
	}

	err = network.WaitForAutopeering(minimumNeighbors)
	if err != nil {
		return nil, err
	}

	return network, nil
}

// CreateDRNGNetwork creates and returns a (Docker) Network that contains drand and `peers` GoShimmer nodes.
func (f *Framework) CreateDRNGNetwork(name string, members, peers, minimumNeighbors int) (*DRNGNetwork, error) {
	drng, err := newDRNGNetwork(f.dockerClient, strings.ToLower(name), f.tester)
	if err != nil {
		return nil, err
	}

	err = drng.network.createEntryNode()
	if err != nil {
		return nil, err
	}

	// create members/drand nodes
	for i := 0; i < members; i++ {
		leader := i == 0
		if _, err = drng.CreateMember(leader); err != nil {
			return nil, err
		}
	}

	// wait until containers are fully started
	time.Sleep(1 * time.Second)
	err = drng.WaitForDKG()
	if err != nil {
		return nil, err
	}

	// create GoShimmer identities
	pubKeys := make([]hive_ed25519.PublicKey, peers)
	privKeys := make([]hive_ed25519.PrivateKey, peers)
	var drngCommittee string

	for i := 0; i < peers; i++ {
		pubKeys[i], privKeys[i], err = hive_ed25519.GenerateKey()
		if err != nil {
			return nil, err
		}

		if i < members {
			if drngCommittee != "" {
				drngCommittee += fmt.Sprintf(",")
			}
			drngCommittee += pubKeys[i].String()
		}
	}

	config := GoShimmerConfig{
		DRNGInstance:  1,
		DRNGThreshold: 3,
		DRNGDistKey:   hex.EncodeToString(drng.distKey),
		DRNGCommittee: drngCommittee,
	}

	// create peers/GoShimmer nodes
	for i := 0; i < peers; i++ {
		config.Bootstrap = i == 0
		config.Seed = base64.StdEncoding.EncodeToString(ed25519.PrivateKey(privKeys[i].Bytes()).Seed())
		if _, err = drng.CreatePeer(config, pubKeys[i]); err != nil {
			return nil, err
		}
	}

	// wait until peers are fully started and connected
	time.Sleep(1 * time.Second)
	err = drng.network.WaitForAutopeering(minimumNeighbors)
	if err != nil {
		return nil, err
	}

	return drng, nil
}
