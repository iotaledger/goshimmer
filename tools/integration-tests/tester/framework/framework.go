// Package framework provides integration test functionality for GoShimmer with a Docker network.
// It effectively abstracts away all complexity with creating a custom Docker network per test,
// discovering peers, waiting for them to autopeer and offers easy access to the peers' web API and logs.
package framework

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/mr-tron/base58"
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

// Removes the tester container and close the docker client
func (f *Framework) DestroyFramework() error {
	err := f.tester.Remove()
	if err != nil {
		return err
	}
	return f.dockerClient.Close()
}

// CreateNetwork creates and returns a (Docker) Network that contains `peers` GoShimmer nodes.
// It waits for the peers to autopeer until the minimum neighbors criteria is met for every peer.
// The first peer automatically starts with the bootstrap plugin enabled.
func (f *Framework) CreateNetwork(name string, peers int, config CreateNetworkConfig) (*Network, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), f.tester)
	if err != nil {
		return nil, err
	}

	if err := network.createEntryNode(); err != nil {
		return nil, err
	}

	// create peers/GoShimmer nodes
	for i := 0; i < peers; i++ {
		config := GoShimmerConfig{
			ActivityPlugin: func(i int) bool {
				if ParaActivityPluginOnEveryNode {
					return true
				}
				return i == 0
			}(i),
			ActivityInterval: func(i int) int {
				broadcastInterval := 3
				if i == 0 {
					broadcastInterval = 1
				}
				return broadcastInterval
			}(i),
			Seed: func(i int) string {
				if i == 0 {
					return syncBeaconSeed
				}
				return ""
			}(i),
			Faucet: config.Faucet && i == 0,
			Mana: func(i int) bool {
				if ParaManaOnEveryNode {
					return true
				}
				return config.Mana && i == 0
			}(i),
			StartSynced:                config.StartSynced,
			FPCRoundInterval:           ParaFPCRoundInterval,
			FPCTotalRoundsFinalization: ParaFPCTotalRoundsFinalization,
			WaitForStatement:           ParaWaitForStatement,
			FPCListen:                  ParaFPCListen,
			WriteStatement:             ParaWriteStatement,
			WriteManaThreshold:         ParaWriteManaThreshold,
			ReadManaThreshold:          ParaReadManaThreshold,
			SnapshotResetTime:          ParaSnapshotResetTime,
		}
		if _, err := network.CreatePeer(config); err != nil {
			return nil, err
		}
	}
	// wait until containers are fully started
	time.Sleep(5 * time.Second)
	if err := network.DoManualPeeringAndWait(); err != nil {
		return nil, errors.WithStack(err)
	}

	return network, nil
}

// CreateNetworkWithPartitions creates and returns a partitioned network that contains `peers` GoShimmer nodes per partition.
// It waits for the peers to autopeer until the minimum neighbors criteria is met for every peer.
// The first peer automatically starts with the bootstrap plugin enabled.
func (f *Framework) CreateNetworkWithPartitions(name string, peers, partitions, minimumNeighbors int, config CreateNetworkConfig) (*Network, error) {
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
			ActivityPlugin: func(i int) bool {
				if ParaActivityPluginOnEveryNode {
					return true
				}
				return i == 0
			}(i),
			ActivityInterval: func(i int) int {
				broadcastInterval := 3
				if i == 0 {
					broadcastInterval = 1
				}
				return broadcastInterval
			}(i),
			Seed: func(i int) string {
				if i == 0 {
					return syncBeaconSeed
				}
				return ""
			}(i),
			Faucet:                     config.Faucet && i == 0,
			Mana:                       config.Mana,
			FPCRoundInterval:           ParaFPCRoundInterval,
			FPCTotalRoundsFinalization: ParaFPCTotalRoundsFinalization,
			WaitForStatement:           ParaWaitForStatement,
			FPCListen:                  ParaFPCListen,
			WriteStatement:             ParaWriteStatement,
			WriteManaThreshold:         ParaWriteManaThreshold,
			ReadManaThreshold:          ParaReadManaThreshold,
			EnableAutopeeringForGossip: true,
			SnapshotResetTime:          ParaSnapshotResetTime,
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
func (f *Framework) CreateDRNGNetwork(name string, members, peers int) (*DRNGNetwork, error) {
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
	pubKeys := make([]ed25519.PublicKey, peers)
	privKeys := make([]ed25519.PrivateKey, peers)
	var drngCommittee string

	for i := 0; i < peers; i++ {
		pubKeys[i], privKeys[i], err = ed25519.GenerateKey()
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
		DRNGInstance:  111,
		DRNGThreshold: 3,
		DRNGDistKey:   hex.EncodeToString(drng.distKey),
		DRNGCommittee: drngCommittee,
		Mana:          true,
		StartSynced:   true,
	}

	// create peers/GoShimmer nodes
	for i := 0; i < peers; i++ {
		config.Seed = privKeys[i].Seed().String()
		if _, e := drng.CreatePeer(func() GoShimmerConfig {
			if i == 0 {
				faucetConfig := config
				faucetConfig.Faucet = true
				return faucetConfig
			}
			return config
		}(), pubKeys[i]); e != nil {
			return nil, e
		}
	}

	// wait until peers are fully started and connected
	time.Sleep(5 * time.Second)
	err = drng.network.DoManualPeeringAndWait(drng.network.peers...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// get mana from faucet
	for i := 1; i < peers; i++ {
		peer := drng.network.peers[i]
		addr := peer.Seed.Address(uint64(0)).Address()
		ID := base58.Encode(peer.ID().Bytes())
		_, err := drng.network.peers[0].SendFaucetRequest(addr.Base58(), ParaPoWFaucetDifficulty, ID, ID)
		if err != nil {
			return nil, fmt.Errorf("faucet request failed on peer %s: %w", peer.ID(), err)
		}
		time.Sleep(2 * time.Second)
	}
	err = drng.network.WaitForMana(drng.network.peers[1:]...)
	if err != nil {
		return nil, err
	}

	return drng, nil
}

// CreateNetworkWithMana creates and returns a (Docker) Network that contains peers that all have some mana.
// Mana is gotten by sending faucet requests.
func (f *Framework) CreateNetworkWithMana(name string, peers int, config CreateNetworkConfig) (*Network, error) {
	if !config.Faucet {
		return nil, fmt.Errorf("faucet is required")
	}
	if !config.Mana {
		return nil, fmt.Errorf("mana plugin is required to load mana snapshot")
	}

	n, err := f.CreateNetwork(name, peers, config)
	if err != nil {
		return nil, err
	}

	for i := 1; i < len(n.peers); i++ {
		peer := n.peers[i]
		addr := peer.Seed.Address(uint64(0)).Address()
		ID := base58.Encode(peer.ID().Bytes())
		_, err := n.peers[0].SendFaucetRequest(addr.Base58(), ParaPoWFaucetDifficulty, ID, ID)
		if err != nil {
			return nil, fmt.Errorf("faucet request failed on peer %s: %w", peer.ID(), err)
		}
	}
	err = n.WaitForMana(n.peers[1:]...)
	if err != nil {
		return nil, err
	}
	return n, nil
}
