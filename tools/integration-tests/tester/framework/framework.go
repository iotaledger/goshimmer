// Package framework provides integration test functionality for GoShimmer with a Docker network.
// It effectively abstracts away all complexity with creating a custom Docker network per test,
// discovering peers, waiting for them to autopeer and offers easy access to the peers' web API and logs.
package framework

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/docker/docker/client"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/tools/genesis-snapshot/snapshotcreator"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
)

var (
	once     sync.Once
	instance *Framework
)

// Framework is a wrapper that provides the integration testing functionality.
type Framework struct {
	tester *DockerContainer
	docker *client.Client
}

// Instance returns the singleton Framework instance.
func Instance() (f *Framework, err error) {
	once.Do(func() {
		f, err = newFramework(context.Background())
		instance = f
	})

	return instance, err
}

// newFramework creates a new instance of Framework, creates a DockerClient
// and creates a DockerContainer for the tester container where the tests are running in.
func newFramework(ctx context.Context) (*Framework, error) {
	dockerClient, err := newDockerClient()
	if err != nil {
		return nil, err
	}

	// Since we are running within a container, the HOSTNAME environment variable defaults
	// to a shortened the container Id.
	tester, err := NewDockerContainerFromExisting(ctx, dockerClient, os.Getenv("HOSTNAME"))
	if err != nil {
		return nil, err
	}

	f := &Framework{
		docker: dockerClient,
		tester: tester,
	}
	return f, nil
}

// CfgAlterFunc is a function that alters the configuration for a given peer. To identify the peer the function gets
// called with the peer's index and its the master peer status. It should returned an updated config for the peer.
type CfgAlterFunc func(peerIndex int, isPeerMaster bool, cfg config.GoShimmer) config.GoShimmer

// SnapshotInfo stores the details about snapshots created for integration tests
type SnapshotInfo struct {
	// FilePath defines the file path of the snapshot, if specified, the snapshot will not be generated.
	FilePath string
	// MasterSeed is the seed of the PeerMaster node where the genesis pledge goes to.
	MasterSeed string
	// GenesisTokenAmount is the amount of tokens left on the Genesis, pledged to Peer Master.
	GenesisTokenAmount uint64
	// PeerSeedBase58 is a slice of Seeds encoded in Base58, one entry per peer.
	PeersSeedBase58 []string
	// PeersAmountsPledges is a slice of amounts to be pledged to the peers, one entry per peer.
	PeersAmountsPledged []uint64
}

// CreateNetwork creates and returns a network that contains numPeers GoShimmer peers.
// It blocks until all peers are connected.
func (f *Framework) CreateNetwork(ctx context.Context, name string, numPeers int, conf CreateNetworkConfig, cfgAlterFunc ...CfgAlterFunc) (*Network, error) {
	network, err := f.CreateNetworkNoAutomaticManualPeering(ctx, name, numPeers, conf, cfgAlterFunc...)
	if err == nil && !conf.Autopeering {
		err = network.DoManualPeering(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "manual peering failed")
		}
	}

	return network, err
}

func (f *Framework) CreateNetworkNoAutomaticManualPeering(ctx context.Context, name string, numPeers int, conf CreateNetworkConfig, cfgAlterFunc ...CfgAlterFunc) (*Network, error) {
	network, err := NewNetwork(ctx, f.docker, name, f.tester)
	if err != nil {
		return nil, err
	}

	errCreateSnapshots := createSnapshot(conf.Snapshot)
	if errCreateSnapshots != nil {
		return nil, errors.Wrap(errCreateSnapshots, "failed to create snapshot")
	}

	// an entry node is only required for autopeering
	if conf.Autopeering {
		if err = network.createEntryNode(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to create entry node")
		}
	}

	err = network.createPeers(ctx, numPeers, conf, cfgAlterFunc...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create peers")
	}

	// wrap peers with socat containers
	for i, peer := range network.peers {
		if _, err = network.createSocatContainer(ctx, peer, i); err != nil {
			return nil, errors.Wrap(err, "failed to create socat container")
		}
	}

	// wait for peering to complete
	if conf.Autopeering {
		err = network.WaitForAutopeering(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "autopeering failed")
		}
		err = network.WaitForPeerDiscovery(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "peer discovery failed")
		}
	}

	return network, nil
}

func createSnapshot(snapshotInfo SnapshotInfo) error {
	nodesToPledgeMap, err := createPledgeMap(snapshotInfo)
	if err != nil {
		return err
	}

	if len(nodesToPledgeMap) == 0 {
		return errors.Errorf("no nodes to pledge specified in SnapshotInfo")
	}

	masterSeed, err := base58.Decode(snapshotInfo.MasterSeed)
	if err != nil {
		return errors.Wrap(err, "failed to decode master seed")
	}

	// default to /assets/snapshot.bin
	if snapshotInfo.FilePath == "" {
		snapshotInfo.FilePath = "/assets/snapshot.bin"
	}

	err = snapshotcreator.CreateSnapshotForIntegrationTest(snapshotInfo.FilePath, snapshotInfo.GenesisTokenAmount, GenesisSeedBytes, masterSeed, nodesToPledgeMap)
	if err != nil {
		return err
	}

	return nil
}

// createPledgeMap creates a pledge map according to snapshotInfo
func createPledgeMap(snapshotInfo SnapshotInfo) (nodesToPledge map[[32]byte]uint64, err error) {
	nodesToPledge = make(map[[32]byte]uint64)

	for i, peerSeedBase58 := range snapshotInfo.PeersSeedBase58 {
		seedBytes, err := base58.Decode(peerSeedBase58)
		if err != nil {
			return nil, err
		}

		var seed [32]byte
		copy(seed[:], seedBytes)
		nodesToPledge[seed] = snapshotInfo.PeersAmountsPledged[i]
	}

	return nodesToPledge, nil
}

// CreateNetworkWithPartitions creates and returns a network that contains numPeers GoShimmer nodes
// distributed over numPartitions partitions. It blocks until all peers are connected.
func (f *Framework) CreateNetworkWithPartitions(ctx context.Context, name string, numPeers, numPartitions int, conf CreateNetworkConfig, cfgAlterFunc ...CfgAlterFunc) (*Network, error) {
	network, err := NewNetwork(ctx, f.docker, name, f.tester)
	if err != nil {
		return nil, err
	}

	// make sure that autopeering is on
	conf.Autopeering = true

	// Create Snapshot defined in the network configuration.
	errCreateSnapshots := createSnapshot(conf.Snapshot)
	if errCreateSnapshots != nil {
		return nil, errCreateSnapshots
	}

	// create an entry node with blocked traffic
	log.Println("Starting entry node...")
	if err = network.createEntryNode(ctx); err != nil {
		return nil, err
	}
	pumba, err := network.createPumba(ctx, network.entryNode, nil)
	if err != nil {
		return nil, err
	}
	// wait until pumba is started and the traffic is blocked
	time.Sleep(graceTimePumba)
	log.Println("Starting entry node... done")

	if err = network.createPeers(ctx, numPeers, conf, cfgAlterFunc...); err != nil {
		return nil, err
	}
	// wrap peers with socat containers
	for i, peer := range network.peers {
		if _, err = network.createSocatContainer(ctx, peer, i); err != nil {
			return nil, errors.Wrap(err, "failed to create socat container")
		}
	}

	log.Printf("Creating %d partitions for %d peers...", numPartitions, numPeers)
	chunkSize := numPeers / numPartitions
	for i := 0; i < numPeers; i += chunkSize {
		end := i + chunkSize
		if end > numPeers {
			end = numPeers
		}
		err = network.createPartition(ctx, network.peers[i:end])
		if err != nil {
			return nil, err
		}
	}
	// wait until pumba containers are started and block traffic between partitions
	time.Sleep(graceTimePumba)
	log.Println("Creating partitions... done")

	// delete pumba for entry node
	err = pumba.Stop(ctx)
	if err != nil {
		return nil, err
	}
	err = pumba.Remove(ctx)
	if err != nil {
		return nil, err
	}

	err = network.WaitForAutopeering(ctx)
	if err != nil {
		return nil, err
	}

	return network, nil
}
