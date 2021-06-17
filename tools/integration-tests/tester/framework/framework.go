// Package framework provides integration test functionality for GoShimmer with a Docker network.
// It effectively abstracts away all complexity with creating a custom Docker network per test,
// discovering peers, waiting for them to autopeer and offers easy access to the peers' web API and logs.
package framework

import (
	"context"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/docker/docker/client"
	"github.com/iotaledger/hive.go/crypto/ed25519"

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

	tester, err := NewDockerContainerFromExisting(ctx, dockerClient, containerNameTester)
	if err != nil {
		return nil, err
	}

	f := &Framework{
		docker: dockerClient,
		tester: tester,
	}
	return f, nil
}

func (f *Framework) CreateNetwork(ctx context.Context, name string, numPeers int, conf CreateNetworkConfig) (*Network, error) {
	network, err := NewNetwork(ctx, f.docker, name, f.tester)
	if err != nil {
		return nil, err
	}

	// an entry node is only required for autopeering
	if conf.Autopeering {
		err = network.createEntryNode(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create entry node")
		}
	}

	err = network.createPeers(ctx, numPeers, conf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create peers")
	}

	// use autopeering
	if conf.Autopeering {
		err = network.WaitForAutopeering(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "autopeering failed")
		}
		return network, nil
	}

	// use manual peering
	if err := network.DoManualPeering(ctx); err != nil {
		return nil, errors.Wrap(err, "manual peering failed")
	}
	return network, nil
}

// CreateNetworkWithPartitions creates and returns a partitioned network that contains `peers` GoShimmer nodes per partition.
// It waits for the peers to autopeer until the minimum neighbors criteria is met for every peer.
// The first peer automatically starts with the bootstrap plugin enabled.
func (f *Framework) CreateNetworkWithPartitions(ctx context.Context, name string, numPeers, partitions int, conf CreateNetworkConfig) (*Network, error) {
	network, err := NewNetwork(ctx, f.docker, name, f.tester)
	if err != nil {
		return nil, err
	}

	// make sure that autopeering is on
	conf.Autopeering = true

	// create an entry node with blocked traffic
	log.Println("Starting entry node...")
	err = network.createEntryNode(ctx)
	if err != nil {
		return nil, err
	}
	pumba, err := network.createPumba(ctx, network.entryNode, nil)
	if err != nil {
		return nil, err
	}
	// wait until pumba is started and the traffic is blocked
	time.Sleep(2 * time.Second)
	log.Println("Starting entry node... done")

	err = network.createPeers(ctx, numPeers, conf)
	if err != nil {
		return nil, err
	}

	log.Printf("Creating %d partitions for %d peers...", partitions, numPeers)
	chunkSize := numPeers / partitions
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
	time.Sleep(5 * time.Second)
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

func (f *Framework) CreateDRNGNetwork(ctx context.Context, name string, members, peers int) (*DRNGNetwork, error) {
	drng, err := newDRNGNetwork(ctx, f.docker, name, f.tester)
	if err != nil {
		return nil, err
	}

	// create members/drand nodes
	for i := 0; i < members; i++ {
		leader := i == 0
		if _, err = drng.CreateMember(ctx, leader); err != nil {
			return nil, err
		}
	}

	err = drng.WaitForDKG(ctx)
	if err != nil {
		return nil, err
	}

	// create GoShimmer identities
	pubKeys := make([]ed25519.PublicKey, peers)
	privKeys := make([]ed25519.PrivateKey, peers)
	var drngCommittee []string
	for i := 0; i < peers; i++ {
		pubKeys[i], privKeys[i], err = ed25519.GenerateKey()
		if err != nil {
			return nil, err
		}

		if i < members {
			drngCommittee = append(drngCommittee, pubKeys[i].String())
		}
	}

	conf := PeerConfig
	conf.DRNG = config.DRNG{
		Enabled: true,
		Custom: struct {
			InstanceId        int
			Threshold         int
			DistributedPubKey string
			CommitteeMembers  []string
		}{111, 3, hex.EncodeToString(drng.distKey), drngCommittee},
	}
	conf.MessageLayer.StartSynced = true

	// create peers/GoShimmer nodes
	for i := 0; i < peers; i++ {
		conf.Seed = privKeys[i].Seed().Bytes()
		if _, e := drng.CreatePeer(ctx, conf); e != nil {
			return nil, e
		}
	}

	err = drng.DoManualPeering(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "manual peering failed")
	}
	return drng, nil
}
